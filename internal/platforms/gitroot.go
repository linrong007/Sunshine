package platforms

import (
	"os"
	"path/filepath"
)

// GitRootHijack 检测项目目录是否被上层目录的 .git 劫持了 git 根：
// 从 dir 往上找最近一个含 .git 的祖先；找到的根不是 dir 本身时
// 返回 (根目录, true)。dir 自己有 .git（目录或 worktree 的 gitdir
// 文件都算），或往上到根都找不到 .git 时返回 ("", false)。
//
// 背景：DSH 按“最近一个含 .git 的祖先”定项目根，再扫该根下的
// .dsh/skills 与 .agents/skills；项目本身没有 .git 时，上层目录
// （例如家目录）的 .git 会把根劫持走，项目 skill 全部报 unknown，
// 而 agent 的文件工具仍在项目目录里正常工作，很容易误判成
// DSH 的 bug。只检测、不自动建仓库，见 DshGitRootHint。
func GitRootHijack(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	if hasGitDir(abs) {
		return "", false
	}
	// 往上逐级找：到文件系统根（Dir(根)==根）停，否则 Dir("/") 恒为
	// "/" 会死循环——条件绝不能写成 parent != abs。
	dir = filepath.Dir(abs)
	for {
		if hasGitDir(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// hasGitDir 报告目录下是否存在 .git（目录或文件都算；worktree 与
// submodule 的 .git 是指向 gitdir 的文件）。读不到一律算不存在。
func hasGitDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// DshGitRootHint 返回 init --dsh 完成后打印的 git 根劫持警告（中文）。
// 只提醒、不自动建仓库：项目可能是有意放在父仓库里的，自动
// git init 会打扰这种安排；修复命令写在警告里，用户自己跑一句。
func DshGitRootHint(ancestor string) string {
	return "警告：上层目录 " + ancestor + " 是 git 仓库根目录，当前项目没有自己的 .git；" +
		"DSH 会把项目根判定为 " + ancestor + "，项目 skill 将无法被发现（报 unknown）。" +
		"修复：在项目目录运行 git init（只新建空仓库，不动上层仓库），然后重启 dsh web 并开新会话。"
}
