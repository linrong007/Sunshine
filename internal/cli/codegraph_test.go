package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// installedPlatformIDs 按平台配置目录判定已装平台，顺序与注册表一致。
func TestInstalledPlatformIDs(t *testing.T) {
	dir := t.TempDir()
	if got := installedPlatformIDs(dir); len(got) != 0 {
		t.Fatalf("空目录不应有已装平台，得到 %v", got)
	}
	for _, d := range []string{".codex", ".claude"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := installedPlatformIDs(dir)
	if len(got) != 2 || got[0] != "claude-code" || got[1] != "codex" {
		t.Errorf("应按注册顺序返回 [claude-code codex]，得到 %v", got)
	}
}
