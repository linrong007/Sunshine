// RunPostToolUse 实现 sunshine hook post-tool-use，对应 Claude Code
// 的 PostToolUse 事件（settings.json 里 matcher 接 Edit|Write|Bash|Read）
// 与 Codex 的 PostToolUse 事件（hooks.json 里 matcher 接
// apply_patch|Bash）：文件写盘后立即把 spec 索引同步一次，agent
// 不用再记得跑 `sunshine spec index`，同 turn 内改完 spec 再派子代理
// 也能读到新鲜索引；Bash / Read 的重复输出走会话去重
// （dedup.go，命中时用 updatedToolOutput 换成短引用）。
//
// 索引是强制的（2026-11-16 用户定：为省 token，.sunshine/spec/ 下
// 全部文件都要带索引）：同步用完整 SyncFile/SyncDir，无索引节的
// 文件也补一个；只处理同时满足两条的改动：
//  1. 工具是 Edit、Write、Bash 或 apply_patch（改文件的工具；matcher
//     已过滤，这里再验一次，防止被手动或从别的平台以别的 payload
//     调到；apply_patch 是 Codex 的文件编辑工具，对应 Claude 的
//     Edit/Write）；
//  2. Edit/Write：file_path 在项目 `.sunshine/spec/` 下且以 .md 结尾
//     （单文件同步）；Bash/apply_patch：无法从命令串或 patch 文本可靠
//     判断改了哪个文件，改走整目录同步（只写真正不同步的文件）。
//
// 铁律：任何情况下返回 nil（退出码 0）；写盘失败静默丢弃，绝不阻断
// agent 会话。
package hook

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/linrong007/Sunshine/internal/specindex"
)

func RunPostToolUse(r io.Reader, opts Options) error {
	if HooksDisabled() {
		return nil
	}
	input := ReadInput(r)
	if len(input) == 0 {
		return nil
	}
	toolName := GetString(input, "tool_name")
	switch toolName {
	case "Edit", "Write":
		// 单文件：只同步 file_path 指向的 spec 文件。
		toolInput, _ := input["tool_input"].(map[string]any)
		if toolInput == nil {
			return nil
		}
		filePath, _ := toolInput["file_path"].(string)
		if strings.TrimSpace(filePath) == "" {
			return nil
		}
		root := ResolveProjectRoot(input)
		if root == "" {
			return nil
		}
		abs := filePath
		if !filepath.IsAbs(abs) {
			cwd := GetString(input, "cwd")
			if cwd == "" {
				if wd, err := os.Getwd(); err == nil {
					cwd = wd
				} else {
					return nil
				}
			}
			abs = filepath.Join(cwd, filePath)
		}
		abs = filepath.Clean(abs)
		specDir := filepath.Join(root, ".sunshine", "spec") + string(os.PathSeparator)
		if !strings.HasPrefix(abs, specDir) {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(abs), ".md") {
			return nil
		}
		// 索引是强制的：无索引节也补一个，与手动 sunshine spec index 一致。
		_, _ = specindex.SyncFile(abs)
	case "Bash", "apply_patch":
		// Bash 改了哪个文件从命令串判断不可靠（sed、python、heredoc
		// 都可能改 spec）；apply_patch 同理，patch 文本解析也不可靠。
		// 改走整目录同步：只写真正不同步的文件。
		root := ResolveProjectRoot(input)
		if root == "" {
			return nil
		}
		_, _ = specindex.SyncDir(filepath.Join(root, ".sunshine", "spec"))
	case "Read":
		// Read 不改文件，不同步索引；只走下面的会话去重。
	default:
		return nil
	}
	// 会话去重（2026-11-19）：只对 Bash / Read 生效，其它工具在
	// dedupKey 里拒绝；命中时写出 updatedToolOutput 替换输出。
	if root := ResolveProjectRoot(input); root != "" {
		dedupPostToolUse(input, root, opts)
	}
	return nil
}
