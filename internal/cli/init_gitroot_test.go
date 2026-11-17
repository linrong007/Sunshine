package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDshGitRootWarning(t *testing.T) {
	// 家目录即 git 根（2026-11-17 demo12 实测），项目里装了 DSH：应警告。
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(home, "demo", "proj")
	if err := os.MkdirAll(filepath.Join(proj, ".dsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	hint := dshGitRootWarning(proj)
	if hint == "" {
		t.Fatal("DSH 已装且根被劫持时应返回警告")
	}
	for _, want := range []string{home, "git init", "unknown"} {
		if !strings.Contains(hint, want) {
			t.Errorf("警告应包含 %q，实际：%s", want, hint)
		}
	}

	// 项目自己有 .git：不警告（先给 home 也装上 DSH，免得用“没装”过关）。
	if err := os.Mkdir(filepath.Join(home, ".dsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := dshGitRootWarning(home); got != "" {
		t.Errorf("项目自己有 .git 时不应警告，实际：%s", got)
	}

	// 项目没装 DSH（无 .dsh 目录）：不警告，哪怕根被劫持。
	bare := filepath.Join(home, "bare")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := dshGitRootWarning(bare); got != "" {
		t.Errorf("项目没装 DSH 时不应警告，实际：%s", got)
	}

	// DSH 已装、往上无 .git：不警告。
	plain := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(plain, ".dsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := dshGitRootWarning(plain); got != "" {
		t.Errorf("往上无 .git 时不应警告，实际：%s", got)
	}
}

func TestInitRerunWarnsDshGitRootHijack(t *testing.T) {
	// 端到端：增量重跑 init 也要警告（v0.5.21 只查本次安装列表，
	// 增量重跑直接 early return，警告一次都出不来——用户实测报障）。
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(home, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	// 第 1 次：全新安装 DSH，应警告。
	out, code := runInitBinary(t, proj, "--dsh", "-y", "-u", "测试")
	if code != 0 {
		t.Fatalf("第 1 次 init 退出码 %d，输出：%s", code, out)
	}
	if !strings.Contains(out, "警告") || !strings.Contains(out, home) {
		t.Errorf("第 1 次 init 应警告劫持根 %s，实际：\n%s", home, out)
	}
	// 第 2 次：增量重跑（DSH 已装），照样警告。
	out, code = runInitBinary(t, proj, "--dsh", "-y", "-u", "测试")
	if code != 0 {
		t.Fatalf("第 2 次 init 退出码 %d，输出：%s", code, out)
	}
	if !strings.Contains(out, "警告") || !strings.Contains(out, home) {
		t.Errorf("增量重跑 init 也应警告劫持根 %s，实际：\n%s", home, out)
	}
	// 第 3 次：增量装 codex（DSH 早已在项目里），照样警告。
	out, code = runInitBinary(t, proj, "--codex", "-y", "-u", "测试")
	if code != 0 {
		t.Fatalf("第 3 次 init 退出码 %d，输出：%s", code, out)
	}
	if !strings.Contains(out, "警告") || !strings.Contains(out, home) {
		t.Errorf("增量装别的平台时也应警告 DSH 劫持，实际：\n%s", out)
	}
}
