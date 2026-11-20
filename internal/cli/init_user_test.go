package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// 重跑 init 时开发者名优先沿用项目里已记下的 .developer，
// 不被 git 配置顶掉；-u 显式指定永远最高。
func TestResolveUserPrefersExistingDeveloper(t *testing.T) {
	dir := t.TempDir()
	devDir := filepath.Join(dir, ".sunshine")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, ".developer"), []byte("linrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveUser("", dir); got != "linrong" {
		t.Errorf("无 -u 时应沿用已有 .developer，得到 %q", got)
	}
	if got := resolveUser("alice", dir); got != "alice" {
		t.Errorf("-u 应优先于已有 .developer，得到 %q", got)
	}
	// 没有 .developer 的目录不得凭空返回已记的名字。
	empty := t.TempDir()
	if got := resolveUser("", empty); got == "linrong" {
		t.Errorf("无 .developer 的目录不应返回 linrong，得到 %q", got)
	}
}
