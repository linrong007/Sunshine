package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProjectConfig 在临时项目里写一份带注释的 config.yaml。
func writeProjectConfig(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".sunshine")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readConfig(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".sunshine", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestCodegraphDeclinedRoundTrip 拒绝记录的写、读、清全程：
// 写入保住原有注释与其它节，清除只删那一行。
func TestCodegraphDeclinedRoundTrip(t *testing.T) {
	root := writeProjectConfig(t, "# 项目配置\nsession_commit_message: \"x\"\n\nupdate:\n  skip:\n    - .claude/commands\n")
	if CodegraphDeclined(root) {
		t.Fatal("初始不应有拒绝记录")
	}
	if err := SetCodegraphDeclined(root, true); err != nil {
		t.Fatal(err)
	}
	if !CodegraphDeclined(root) {
		t.Fatal("写入后应读到拒绝记录")
	}
	content := readConfig(t, root)
	if !strings.Contains(content, "# 项目配置") || !strings.Contains(content, "- .claude/commands") {
		t.Errorf("原有内容与注释应保留：\n%s", content)
	}
	if !strings.Contains(content, "codegraph:\n  install_declined: true") {
		t.Errorf("应追加 codegraph 节：\n%s", content)
	}
	// 重复写入应幂等，不产生第二行。
	if err := SetCodegraphDeclined(root, true); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(readConfig(t, root), "install_declined"); n != 1 {
		t.Errorf("重复写入应只有一行记录，实际 %d 行", n)
	}
	if err := SetCodegraphDeclined(root, false); err != nil {
		t.Fatal(err)
	}
	if CodegraphDeclined(root) {
		t.Error("清除后不应再读到拒绝记录")
	}
	if strings.Contains(readConfig(t, root), "install_declined") {
		t.Error("清除后文件里不应残留该行")
	}
}

// TestCodegraphDeclinedExistingSection 已有 codegraph 节时把键插进节内，
// 且 false 值不算拒绝。
func TestCodegraphDeclinedExistingSection(t *testing.T) {
	root := writeProjectConfig(t, "codegraph:\n  install_declined: false\n")
	if CodegraphDeclined(root) {
		t.Error("false 不应算拒绝")
	}
	if err := SetCodegraphDeclined(root, true); err != nil {
		t.Fatal(err)
	}
	if !CodegraphDeclined(root) {
		t.Error("改写 false 为 true 后应读到拒绝")
	}
	if n := strings.Count(readConfig(t, root), "install_declined"); n != 1 {
		t.Errorf("应原地改值而非新增行，实际 %d 行", n)
	}
}

// TestCodegraphDeclinedNoFile 文件不存在时读为未拒绝，清除是无操作。
func TestCodegraphDeclinedNoFile(t *testing.T) {
	root := t.TempDir()
	if CodegraphDeclined(root) {
		t.Error("无文件应按未拒绝处理")
	}
	if err := SetCodegraphDeclined(root, false); err != nil {
		t.Fatal(err)
	}
}
