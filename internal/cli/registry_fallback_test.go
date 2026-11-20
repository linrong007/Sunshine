package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linrong007/Sunshine/internal/registry"
)

// TestRegistryInstallFailureFallback_C1_KeepsExisting 是 C1 端到端回归：
// 已初始化项目（spec 里有手写内容）上装市场模板，探测成功但安装
// 失败（index.json 里模板 path 指向不存在的路径）时走回退，回退
// 不能再把手写规范覆盖成内置空白模板，也不能把手写文件记成模板哈希。
func TestRegistryInstallFailureFallback_C1_KeepsExisting(t *testing.T) {
	// 本地假市场：index.json 可探测，模板 path 不存在 → 安装阶段失败。
	market := filepath.Join(t.TempDir(), "market")
	initTestGitRepo(t, market, map[string]string{
		"index.json": `{"version": 1, "templates": [
			{"id": "bad-spec", "type": "spec", "name": "坏模板", "path": "specs/does-not-exist"}
		]}`,
		"README.md": "# 假市场\n",
	})

	// 已初始化项目：三份 spec 都是手写内容。
	proj := t.TempDir()
	specDir := filepath.Join(proj, ".sunshine", "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"system.md", "frontend.md", "backend.md"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(specDir, name), []byte("MUST-STAY "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	target, err := registry.ResolveTarget(market, "bad-spec")
	if err != nil {
		t.Fatalf("ResolveTarget 应探测成功：%v", err)
	}
	defer target.Close()
	if !target.WritesSpec() {
		t.Fatal("目标应会写 spec（回退分支的前提）")
	}

	oldRegistry, oldTemplate := initRegistry, initTemplate
	initRegistry, initTemplate = market, "bad-spec"
	defer func() { initRegistry, initTemplate = oldRegistry, oldTemplate }()

	installRegistryTemplates(proj, target, registry.StrategyAppend)

	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "MUST-STAY "+name+"\n" {
			t.Errorf("回退后手写 %s 被覆盖：%q", name, data)
		}
	}
	// 哈希清单里不应出现手写 spec（回退没有新写任何 spec 文件）。
	if data, err := os.ReadFile(filepath.Join(proj, ".sunshine", ".template-hashes.json")); err == nil {
		if strings.Contains(string(data), ".sunshine/spec/") {
			t.Errorf("哈希清单不应记手写 spec：%s", data)
		}
	}
}
