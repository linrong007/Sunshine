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
	// 无 ## 节时不建索引，但适用标签照样补。
	updated, changed := SyncContent("# 空\n\n只有正文，没有二级标题。\n")
	if !changed {
		t.Fatal("缺适用标签时应改动")
	}
	if strings.Contains(updated, "## 索引") {
		t.Fatal("无 ## 节时不应建索引")
	}
	if !strings.HasPrefix(updated, "---\n适用：通用\n---\n\n") {
		t.Fatalf("应补通用标签：\n%s", updated)
	}
	if _, again := SyncContent(updated); again {
		t.Fatal("第二次同步应无改动")
	}
}

func TestSyncFile_AtomicNoOverwrite(t *testing.T) {
	// A1 回归：写盘走同目录临时文件 + rename 原子替换，
	// 保留原文件权限（旧实现写死 0644），且不留临时文件残骸。
	dir := t.TempDir()
	path := filepath.Join(dir, "backend.md")
	if err := os.WriteFile(path, []byte("# 后端规范\n\n## 接口\n\n正文。\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("应改动")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## 索引") || !strings.Contains(string(got), "适用：后端") {
		t.Fatalf("同步后内容不对：\n%s", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("原文件权限应保留为 0600，实际 %v", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "backend.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("目录里不应留临时文件，实际：%v", names)
	}
	// 幂等：再同步一次无改动。
	if changed, err := SyncFile(path); err != nil || changed {
		t.Fatalf("第二次同步应无改动，changed=%v err=%v", changed, err)
	}
}

func TestSyncDir_SkipsBadFile(t *testing.T) {
	// A2 回归（SyncDir 侧）：悬空软链接在场时其余文件仍同步，
	// 错误汇总返回且带上跳过的文件名，而不是首错即中断。
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "backend.md"), []byte("# 后端规范\n\n## 接口\n\n正文。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(specDir, "bad.md")
	if err := os.Symlink(filepath.Join(specDir, "不存在的目标.md"), bad); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncDir(specDir)
	if err == nil {
		t.Fatal("有坏文件时 SyncDir 应汇总返回错误")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("错误信息应带跳过文件清单，实际：%v", err)
	}
	if len(changed) != 1 || filepath.Base(changed[0]) != "backend.md" {
		t.Fatalf("好文件仍应同步，changed = %v", changed)
	}
	got, err := os.ReadFile(filepath.Join(specDir, "backend.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## 索引") {
		t.Fatalf("好文件应已同步索引：\n%s", got)
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
