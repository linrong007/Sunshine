package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWithOptionsSkipSpec(t *testing.T) {
	dir := t.TempDir()
	if err := InitWithOptions(dir, "tester", InitOptions{SkipSpec: true}); err != nil {
		t.Fatalf("InitWithOptions 失败：%v", err)
	}
	// 骨架其他文件应存在。
	if _, err := os.Stat(filepath.Join(dir, ".sunshine", "config.yaml")); err != nil {
		t.Errorf("config.yaml 应存在：%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".sunshine", "workflow.md")); err != nil {
		t.Errorf("workflow.md 应存在：%v", err)
	}
	// spec 模板应被跳过（目录不存在或为空）。
	specDir := filepath.Join(dir, ".sunshine", "spec")
	entries, err := os.ReadDir(specDir)
	if err == nil && len(entries) != 0 {
		t.Errorf("SkipSpec 时 spec 目录应为空，得到 %v", entries)
	}
	// 哈希清单里不应有 spec 条目。
	data, err := os.ReadFile(filepath.Join(dir, ".sunshine", ".template-hashes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), ".sunshine/spec/") {
		t.Error("哈希清单不应含 spec 条目")
	}
}

func TestInitDefaultWritesSpec(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "tester", false); err != nil {
		t.Fatalf("Init 失败：%v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".sunshine", "spec"))
	if err != nil || len(entries) == 0 {
		t.Errorf("默认 Init 应写 spec 模板：%v, %v", entries, err)
	}
}

func TestWriteDefaultSpec(t *testing.T) {
	dir := t.TempDir()
	if err := InitWithOptions(dir, "tester", InitOptions{SkipSpec: true}); err != nil {
		t.Fatalf("InitWithOptions 失败：%v", err)
	}
	files, err := WriteDefaultSpec(dir)
	if err != nil {
		t.Fatalf("WriteDefaultSpec 失败：%v", err)
	}
	if len(files) == 0 {
		t.Fatal("应写出 spec 文件")
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("文件 %s 不存在：%v", f, err)
		}
		if !strings.Contains(f, ".sunshine/spec/") {
			t.Errorf("文件路径 %q 应在 .sunshine/spec/ 下", f)
		}
	}
}

// TestWriteDefaultSpec_KeepsExisting 是 C1 回归：回退写默认 spec 时
// 只补缺失，已存在的手写规范原样保留、且不进返回清单（不被记哈希）。
func TestWriteDefaultSpec_KeepsExisting(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hand := "# 手写系统规范\nMUST-STAY\n"
	if err := os.WriteFile(filepath.Join(specDir, "system.md"), []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := WriteDefaultSpec(dir)
	if err != nil {
		t.Fatalf("WriteDefaultSpec 失败：%v", err)
	}
	data, err := os.ReadFile(filepath.Join(specDir, "system.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != hand {
		t.Errorf("手写 system.md 被覆盖：%q", data)
	}
	for _, f := range files {
		if f == ".sunshine/spec/system.md" {
			t.Error("已存在的手写 system.md 不应出现在返回清单里（会被记成模板哈希）")
		}
	}
	// 缺失的两份应被补齐并出现在清单里。
	for _, want := range []string{".sunshine/spec/frontend.md", ".sunshine/spec/backend.md"} {
		found := false
		for _, f := range files {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("缺失的 %s 应被补写并出现在清单里，files=%v", want, files)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want))); err != nil {
			t.Errorf("%s 应存在：%v", want, err)
		}
	}
	// 全部已存在时：内容全保留，返回清单为空。
	dir2 := t.TempDir()
	specDir2 := filepath.Join(dir2, ".sunshine", "spec")
	if err := os.MkdirAll(specDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"system.md", "frontend.md", "backend.md"} {
		if err := os.WriteFile(filepath.Join(specDir2, name), []byte("MUST-STAY "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files2, err := WriteDefaultSpec(dir2)
	if err != nil {
		t.Fatalf("WriteDefaultSpec 失败：%v", err)
	}
	if len(files2) != 0 {
		t.Errorf("全部已存在时返回清单应为空，得到 %v", files2)
	}
	for _, name := range []string{"system.md", "frontend.md", "backend.md"} {
		data, err := os.ReadFile(filepath.Join(specDir2, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "MUST-STAY "+name+"\n" {
			t.Errorf("%s 被覆盖：%q", name, data)
		}
	}
}
