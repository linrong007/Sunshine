package platforms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRootHijack(t *testing.T) {
	// 祖先带 .git 的场景：家目录即 git 根（2026-11-17 demo12 实测）。
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(home, "demo", "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	ancestor, hijacked := GitRootHijack(proj)
	if !hijacked {
		t.Fatal("祖先有 .git 时应报劫持")
	}
	if ancestor != home {
		t.Fatalf("劫持根应为 %s，实际 %s", home, ancestor)
	}

	// 项目自己有 .git（目录）时不报。
	if _, hijacked := GitRootHijack(home); hijacked {
		t.Fatal("项目自己有 .git 时不应报劫持")
	}

	// 项目自己有 .git（worktree 的 gitdir 文件）时不报。
	worktree := filepath.Join(home, "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, hijacked := GitRootHijack(worktree); hijacked {
		t.Fatal("项目自己有 .git 文件时不应报劫持")
	}

	// 祖先的 .git 是文件时也报（submodule/worktree 的父级）。
	sub := filepath.Join(home, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 注意 sub 自己有 .git 文件，不报；它的子目录才报。
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	ancestor, hijacked = GitRootHijack(deep)
	if !hijacked || ancestor != sub {
		t.Fatalf("祖先 .git 为文件时应报劫持到 %s，实际 hijacked=%v ancestor=%s", sub, hijacked, ancestor)
	}
}

func TestGitRootHijackNoGitAnywhere(t *testing.T) {
	// /tmp 之上无 .git（沙箱与常规机器都成立）：往上找不到时不报。
	proj := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, hijacked := GitRootHijack(proj); hijacked {
		t.Fatal("往上无 .git 时不应报劫持（DSH 回退到 cwd）")
	}
}

func TestDshGitRootHint(t *testing.T) {
	hint := DshGitRootHint("/Users/ethan")
	for _, want := range []string{"/Users/ethan", "git init", "unknown", "dsh web"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("警告应包含 %q，实际：%s", want, hint)
		}
	}
	if strings.Contains(hint, "自动") {
		t.Fatal("警告不应提自动建仓库：本功能只检测提醒")
	}
}
