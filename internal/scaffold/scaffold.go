// Package scaffold 实现 sunshine init：生成 .sunshine/ 工程骨架。
package scaffold

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/linrong007/Sunshine/internal/projecttype"
	"github.com/linrong007/Sunshine/templates"
)

// RecordHashes 把 files（相对项目根的路径，如 ".claude/settings.json"）的
// SHA-256 合并进 .sunshine/.template-hashes.json，保留已有条目。
// 清单格式与 Init 写入的一致：{"__version": 2, "files": {...}}。
// 清单不存在时新建一份；任一文件不存在或清单损坏时返回错误。
func RecordHashes(cwd string, files []string) error {
	manifestPath := filepath.Join(cwd, ".sunshine", ".template-hashes.json")
	hashes := make(map[string]string)
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest struct {
			Version int               `json:"__version"`
			Files   map[string]string `json:"files"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("读取模板哈希清单失败：%w", err)
		}
		for k, v := range manifest.Files {
			hashes[k] = v
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取模板哈希清单失败：%w", err)
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(cwd, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("计算 %s 的哈希失败：%w", rel, err)
		}
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"__version": 2,
		"files":     hashes,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, append(manifest, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入模板哈希清单失败：%w", err)
	}
	return nil
}

var hashExcluded = map[string]bool{
	".template-hashes.json": true,
	".version":              true,
	".developer":            true,
}

// InitOptions 是 InitWithOptions 的参数。
type InitOptions struct {
	// Force 为真时覆盖已有的骨架：删除前先把旧 .sunshine/ 改名备份到
	// 同级 .sunshine.backup-<时间戳> 目录，并打印备份路径。
	Force bool
	// SkipSpec 为真时不写默认 spec 模板（--registry 将从模板市场装 spec，
	// 装完再由调用方记入哈希清单；对应上游 init 的 skipSpecTemplates）。
	SkipSpec bool
	// SpecType 是本次 init 的项目类型：Frontend 只写前端 spec，
	// Backend 只写后端 spec，Fullstack 或 Unknown（零值）三份全写；
	// Design 与 Fullstack 一样三份全写，另写 doc/ 设计文档模板；
	// system.md 与类型无关，永远写。
	SpecType projecttype.Type
	// Version 为空时 .version 沿用模板文件内容；非空时写入该值
	//（CLI 的 init 传 cli.version，保证 .version 与 --version 同源，
	// 不另留一份版本常量）。
	Version string
}

// Init 在 cwd 下生成 .sunshine/ 工程骨架。force 为真时覆盖已有的骨架。
func Init(cwd, user string, force bool) error {
	return InitWithOptions(cwd, user, InitOptions{Force: force})
}

// InitWithOptions 在 cwd 下生成 .sunshine/ 工程骨架。
func InitWithOptions(cwd, user string, opts InitOptions) error {
	home, err := os.UserHomeDir()
	if err == nil && cwd == home && os.Getenv("SUNSHINE_ALLOW_HOMEDIR") != "1" {
		return errors.New("不能在用户主目录执行初始化（设置 SUNSHINE_ALLOW_HOMEDIR=1 可绕过）")
	}
	target := filepath.Join(cwd, ".sunshine")
	if st, err := os.Stat(target); err == nil && st.IsDir() {
		if !opts.Force {
			return errors.New("当前目录已经初始化过（.sunshine/ 已存在），如需重新初始化请使用 --force")
		}
		// Force 重建前先把旧 .sunshine/ 整体改名备份到同级带时间戳目录，
		// 不再直接 RemoveAll：任务/journal/memory 等用户数据不可逆丢失，
		// 且重建中途失败时旧数据无处可寻。备份失败则中止，旧目录不动。
		// 命名与 internal/update 的备份先例同风格（时间戳目录）：
		// .sunshine.backup-YYYYMMDD-HHMMSS，同秒冲突时加 -1、-2 后缀。
		stamp := time.Now().Format("20060102-150405")
		backupPath := filepath.Join(cwd, ".sunshine.backup-"+stamp)
		for suffix := 1; ; suffix++ {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				break
			} else if err != nil {
				return fmt.Errorf("检查备份目录 %s 失败：%w", backupPath, err)
			}
			backupPath = filepath.Join(cwd, fmt.Sprintf(".sunshine.backup-%s-%d", stamp, suffix))
		}
		if err := os.Rename(target, backupPath); err != nil {
			return fmt.Errorf("备份旧的 .sunshine/ 到 %s 失败（旧数据未动）：%w", backupPath, err)
		}
		fmt.Printf("已备份旧 .sunshine/ 到：%s\n", backupPath)
	}
	hashes := make(map[string]string)
	err = fs.WalkDir(templates.Sunshine, "sunshine", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "sunshine/")
		if opts.SkipSpec && strings.HasPrefix(rel, "spec/") {
			return nil
		}
		// 按项目类型跳过不匹配的 spec 文件（跳过的文件不写盘，
		// 也不进入哈希清单）；system.md 与类型无关，永远写。
		// 产品设计先行也按全栈写三份规范（另写 doc/ 模板）。
		if rel == "spec/frontend.md" && opts.SpecType == projecttype.Backend {
			return nil
		}
		if rel == "spec/backend.md" && opts.SpecType == projecttype.Frontend {
			return nil
		}
		data, err := templates.Sunshine.ReadFile(path)
		if err != nil {
			return err
		}
		// .version 与 CLI 版本同源：调用方传了 Version 时以它为准，
		// 不沿用模板文件里的旧版本号（模板文件只作未传版本时的 fallback）。
		if rel == ".version" && opts.Version != "" {
			data = []byte(opts.Version + "\n")
		}
		dest := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		if !hashExcluded[rel] {
			sum := sha256.Sum256(data)
			// 清单键统一为相对项目根的 POSIX 路径（与 Trellis 的
			// `.template-hashes.json` 约定一致），不要用相对 .sunshine/ 的路径。
			hashes[filepath.ToSlash(filepath.Join(".sunshine", rel))] = hex.EncodeToString(sum[:])
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("写入工程骨架失败：%w", err)
	}
	// workflow.md 用完整版 native 模板（与 `sunshine workflow --template native`
	// 同源；2026-11-09 用户确认 init 改用完整版，不再用 28 行简版）。
	nativeWorkflow, err := templates.Workflows.ReadFile("workflows/native/workflow.md")
	if err != nil {
		return fmt.Errorf("读取原生工作流模板失败：%w", err)
	}
	if err := os.WriteFile(filepath.Join(target, "workflow.md"), nativeWorkflow, 0o644); err != nil {
		return fmt.Errorf("写入 workflow.md 失败：%w", err)
	}
	sum := sha256.Sum256(nativeWorkflow)
	hashes[".sunshine/workflow.md"] = hex.EncodeToString(sum[:])
	// 开发者身份文件（不进入哈希清单）。
	if err := os.WriteFile(filepath.Join(target, ".developer"), []byte(user+"\n"), 0o644); err != nil {
		return fmt.Errorf("写入开发者身份失败：%w", err)
	}
	// 模板哈希清单，供后续 update 做漂移检测。
	manifest, err := json.MarshalIndent(map[string]any{
		"__version": 2,
		"files":     hashes,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, ".template-hashes.json"), append(manifest, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入模板哈希清单失败：%w", err)
	}
	// 产品设计先行：写 doc/ 设计文档模板（不在 .sunshine/ 里，
	// 不进哈希清单；已填过的内容绝不覆盖，-f 重建也不覆盖）。
	if opts.SpecType == projecttype.Design {
		if err := writeDesignTemplates(cwd); err != nil {
			return err
		}
	}
	// 仓库根 .gitignore：只在不存在时写（已有内容绝不覆盖，-f 也不覆盖）。
	if err := writeRootGitignore(cwd); err != nil {
		return err
	}
	return nil
}

// rootGitignore 是 init 生成的仓库根 .gitignore 内容。Sunshine 自身
// 生成的文件都在 .sunshine/ 里（由 .sunshine/.gitignore 管），根目录
// 没有 Sunshine 自有项；这里只放真实踩过的坑与通用项，不多加：
// node_modules/ 是奶茶那轮差点进库的一项，.env 防密钥进库，
// .DS_Store 是 macOS 通用项。
const rootGitignore = `# Sunshine：init 生成的项目根忽略；文件已存在时绝不覆盖。
node_modules/
.env
.DS_Store
`

// writeRootGitignore 在项目根写 .gitignore，只在文件不存在时写：
// 已有内容绝不覆盖，-f 重建也不覆盖（与 writeDesignTemplates 同规矩）。
func writeRootGitignore(cwd string) error {
	dest := filepath.Join(cwd, ".gitignore")
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 .gitignore 失败：%w", err)
	}
	if err := os.WriteFile(dest, []byte(rootGitignore), 0o644); err != nil {
		return fmt.Errorf("写入 .gitignore 失败：%w", err)
	}
	return nil
}

// designTemplates 是产品设计文档模板：内嵌源路径 → 项目内目标路径。
// 需求列表与原型是设计阶段的活文档；其余七份是交付文档占位
// （七份都在 doc/delivery/），只有标题与
// 章节骨架，项目收尾时经用户确认才生成内容。
// 原型按页分文件：index.html 是导航入口，每页一个文件。
var designTemplates = []struct{ src, dst string }{
	{"design/requirements.md", "doc/design/需求列表.md"},
	{"design/outline.md", "doc/delivery/概要设计.md"},
	{"design/prototype/index.html", "doc/prototype/index.html"},
	{"design/prototype/示例页.html", "doc/prototype/示例页.html"},
	{"design/detailed.md", "doc/delivery/详细设计.md"},
	{"design/database.md", "doc/delivery/数据库设计.md"},
	{"design/api.md", "doc/delivery/接口文档.md"},
	{"design/test-report.md", "doc/delivery/测试报告.md"},
	{"design/manual.md", "doc/delivery/用户手册.md"},
	{"design/deployment.md", "doc/delivery/部署说明.md"},
}

// writeDesignTemplates 把产品设计文档模板写进项目 doc/。
// 只在目标文件不存在时写：设计人员已填的内容绝不覆盖。
func writeDesignTemplates(cwd string) error {
	for _, t := range designTemplates {
		data, err := templates.Design.ReadFile(t.src)
		if err != nil {
			return fmt.Errorf("读取产品设计文档模板 %s 失败：%w", t.src, err)
		}
		dest := filepath.Join(cwd, filepath.FromSlash(t.dst))
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("检查 %s 失败：%w", t.dst, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("写入 %s 失败：%w", t.dst, err)
		}
	}
	return nil
}
