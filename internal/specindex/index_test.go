package specindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSections(t *testing.T) {
	content := "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n\n## 接口总览\n\n正文\n\n### 子节不取\n\n## 数据模型\n"
	got := ParseSections(content)
	if len(got) != 2 || got[0].Title != "接口总览" || got[1].Title != "数据模型" {
		t.Fatalf("ParseSections = %+v", got)
	}
}

func TestParseIndex(t *testing.T) {
	content := "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n| --- | --- |\n| 接口总览（健康检查、商品） | `## 接口总览` |\n| 数据模型 | `## 数据模型` |\n\n## 接口总览\n"
	got := ParseIndex(content)
	if got["接口总览"] != "健康检查、商品" {
		t.Fatalf("接口总览描述 = %q", got["接口总览"])
	}
	if _, ok := got["数据模型"]; !ok {
		t.Fatalf("数据模型缺失：%v", got)
	}
	if got["数据模型"] != "" {
		t.Fatalf("数据模型描述应为空，实际 %q", got["数据模型"])
	}
}

func TestSyncContentInsert(t *testing.T) {
	content := "# 前端规范\n\n## 技术栈\n\n- Vue。\n\n## 约束\n\n- 防抖。\n"
	updated, changed := SyncContent(content)
	if !changed {
		t.Fatal("应改动")
	}
	if !strings.Contains(updated, "## 索引\n") {
		t.Fatalf("缺索引节：\n%s", updated)
	}
	if !strings.Contains(updated, "| 技术栈 | `## 技术栈` |") {
		t.Fatalf("缺技术栈行：\n%s", updated)
	}
	if !strings.Contains(updated, "| 约束 | `## 约束` |") {
		t.Fatalf("缺约束行：\n%s", updated)
	}
	// 幂等：再同步一次不再改动。
	if _, again := SyncContent(updated); again {
		t.Fatal("第二次同步应无改动")
	}
}

func TestSyncContentReplacePreservesDesc(t *testing.T) {
	content := "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n| --- | --- |\n| 接口总览（健康检查、商品） | `## 接口总览` |\n| 已删节 | `## 已删节` |\n\n## 接口总览\n\n正文\n\n## 数据模型\n\n正文\n"
	updated, changed := SyncContent(content)
	if !changed {
		t.Fatal("应改动")
	}
	if !strings.Contains(updated, "| 接口总览（健康检查、商品） | `## 接口总览` |") {
		t.Fatalf("应保留旧描述：\n%s", updated)
	}
	if strings.Contains(updated, "已删节") {
		t.Fatalf("已删节应被移除：\n%s", updated)
	}
	if !strings.Contains(updated, "| 数据模型 | `## 数据模型` |") {
		t.Fatalf("应新增数据模型行：\n%s", updated)
	}
	if _, again := SyncContent(updated); again {
		t.Fatal("第二次同步应无改动")
	}
}

func TestSyncContentNoSections(t *testing.T) {
	if _, changed := SyncContent("# 空\n\n只有正文，没有二级标题。\n"); changed {
		t.Fatal("无 ## 节时不应改动")
	}
}

func TestSyncDirSkipsGuides(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "backend.md"), []byte("# 后端规范\n\n## 接口\n\n正文。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "guides", "check.md"), []byte("# 检查清单\n\n## 事项\n\n正文。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncDir(specDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || filepath.Base(changed[0]) != "backend.md" {
		t.Fatalf("changed = %v", changed)
	}
	guides, err := os.ReadFile(filepath.Join(specDir, "guides", "check.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(guides), "## 索引") {
		t.Fatal("guides/ 下的文件不应被同步")
	}
}
