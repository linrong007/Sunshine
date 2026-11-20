package specindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseApplicability(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"---\n适用：前端\n---\n\n# 前端规范\n", "前端"},
		{"---\n适用: 后端\n---\n\n# 后端规范\n", "后端"},
		{"---\ntitle: 演示\n适用：通用\n---\n\n# 演示\n", "通用"},
		{"---\n适用：\"前端\"\n---\n\n# 前端规范\n", "前端"},
		{"# 没有 frontmatter\n\n## 节\n", ""},
		{"---\ntitle: 演示\n---\n\n# 演示\n", ""},
		{"---\n适用：移动端\n---\n\n# 演示\n", "移动端"},
	}
	for _, c := range cases {
		if got := ParseApplicability(c.content); got != c.want {
			t.Errorf("ParseApplicability(%q) = %q，想得到 %q", c.content, got, c.want)
		}
	}
}

func TestInferApplicability(t *testing.T) {
	cases := map[string]string{
		"frontend.md":   "前端",
		"FRONTEND-x.md": "前端",
		"backend.md":    "后端",
		"system.md":     "通用",
		"api.md":        "通用",
		"ui.md":         "通用",
		"":              "通用",
	}
	for name, want := range cases {
		if got := InferApplicability(name); got != want {
			t.Errorf("InferApplicability(%q) = %q，想得到 %q", name, got, want)
		}
	}
}

func TestEnsureApplicability(t *testing.T) {
	// 无 frontmatter：新建一块，按文件名推断。
	got, changed := EnsureApplicability("# 前端规范\n\n## 技术栈\n", "frontend.md")
	if !changed || !strings.HasPrefix(got, "---\n适用：前端\n---\n\n# 前端规范\n") {
		t.Fatalf("应新建前端标签：\n%s", got)
	}
	// 已有 frontmatter 无标签：插进块里。
	got, changed = EnsureApplicability("---\ntitle: 演示\n---\n\n# 演示\n", "api.md")
	if !changed || !strings.Contains(got, "适用：通用\ntitle: 演示\n") {
		t.Fatalf("应插进已有 frontmatter：\n%s", got)
	}
	// 已有标签不动（作者的为准，哪怕与文件名推断不一致）。
	got, changed = EnsureApplicability("---\n适用：前端\n---\n\n# 接口规范\n", "api.md")
	if changed || !strings.Contains(got, "适用：前端") {
		t.Fatalf("已有标签不应改动：\n%s", got)
	}
}

func TestSyncContentForTagAndIndexCoexist(t *testing.T) {
	// 标签与索引共存：标签在文件最前面，索引在标题后；幂等。
	content := "# 前端规范\n\n## 技术栈\n\n- Vue。\n"
	updated, changed := SyncContentFor(content, "frontend.md")
	if !changed {
		t.Fatal("应改动")
	}
	if !strings.HasPrefix(updated, "---\n适用：前端\n---\n\n# 前端规范\n") {
		t.Fatalf("标签应在文件最前面：\n%s", updated)
	}
	if !strings.Contains(updated, "## 索引\n") {
		t.Fatalf("缺索引节：\n%s", updated)
	}
	if _, again := SyncContentFor(updated, "frontend.md"); again {
		t.Fatal("第二次同步应无改动")
	}
	// 已有标签 + 已有索引：替换索引时标签不动。
	content2 := "---\n适用：后端\n---\n\n# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n| --- | --- |\n| 旧节 | `## 旧节` |\n\n## 接口\n\n正文\n"
	updated2, changed2 := SyncContentFor(content2, "backend.md")
	if !changed2 {
		t.Fatal("应改动（索引过期）")
	}
	if !strings.HasPrefix(updated2, "---\n适用：后端\n---\n\n# 后端规范\n") {
		t.Fatalf("标签不应被索引替换动到：\n%s", updated2)
	}
	if strings.Contains(updated2, "旧节") {
		t.Fatalf("已删节应被移除：\n%s", updated2)
	}
}

func TestCollectSpecMap(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	write := func(rel, content string) {
		p := filepath.Join(specDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("frontend.md", "# 前端规范\n\n## 技术栈\n\n## 路由\n")
	write("api.md", "---\n适用：后端\n---\n\n# 接口规范\n\n## 认证\n")
	write("system.md", "# 系统规范\n\n## 总原则\n")
	write("pkg/deep.md", "# 深层\n\n## 细节\n")
	write("guides/index.md", "# 指南\n\n## 入门\n")
	write("guides/other.md", "# 其他指南\n\n## 事项\n")
	entries, err := CollectSpecMap(specDir)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]SpecEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	// 平铺文件在列，适用按标签/文件名取；子目录文件在列；guides 只留 index.md。
	cases := map[string]struct {
		scope    string
		sections []string
	}{
		".sunshine/spec/frontend.md":     {"前端", []string{"技术栈", "路由"}},
		".sunshine/spec/api.md":          {"后端", []string{"认证"}},
		".sunshine/spec/system.md":       {"通用", []string{"总原则"}},
		".sunshine/spec/pkg/deep.md":     {"通用", []string{"细节"}},
		".sunshine/spec/guides/index.md": {"通用", []string{"入门"}},
	}
	if len(entries) != len(cases) {
		t.Fatalf("entries = %v，想得到 %d 个", entries, len(cases))
	}
	for path, want := range cases {
		e, ok := byPath[path]
		if !ok {
			t.Errorf("缺 %s", path)
			continue
		}
		if e.Scope != want.scope {
			t.Errorf("%s 适用 = %q，想得到 %q", path, e.Scope, want.scope)
		}
		if strings.Join(e.Sections, ",") != strings.Join(want.sections, ",") {
			t.Errorf("%s 章节 = %v，想得到 %v", path, e.Sections, want.sections)
		}
	}
	if got := FormatSpecMap(entries); !strings.Contains(got, "### 通用组\n- .sunshine/spec/guides/index.md — 入门\n- .sunshine/spec/pkg/deep.md — 细节\n- .sunshine/spec/system.md — 总原则") ||
		!strings.Contains(got, "### 前端组\n- .sunshine/spec/frontend.md — 技术栈、路由") ||
		!strings.Contains(got, "### 后端组\n- .sunshine/spec/api.md — 认证") {
		t.Fatalf("地图分组不对：\n%s", got)
	}
}

func TestCollectSpecMap_SkipsBadFile(t *testing.T) {
	// A2 回归：spec/ 下有悬空软链接时，规范地图不能整体为空，
	// 其余文件仍应进地图，且不返回错误。
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "backend.md"), []byte("# 后端规范\n\n## 接口\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "frontend.md"), []byte("# 前端规范\n\n## 技术栈\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(specDir, "不存在的目标.md"), filepath.Join(specDir, "bad.md")); err != nil {
		t.Fatal(err)
	}
	entries, err := CollectSpecMap(specDir)
	if err != nil {
		t.Fatalf("单个坏文件不应让 CollectSpecMap 报错：%v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("坏文件应被跳过、其余 2 个文件仍进地图，实际 entries = %v", entries)
	}
	got := FormatSpecMap(entries)
	if !strings.Contains(got, ".sunshine/spec/backend.md") || !strings.Contains(got, ".sunshine/spec/frontend.md") {
		t.Fatalf("地图应含好文件：\n%s", got)
	}
	if strings.Contains(got, "bad.md") {
		t.Fatalf("坏文件不应进地图：\n%s", got)
	}
}

func TestCheckDuplicates(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, ".sunshine", "spec")
	write := func(rel, content string) {
		p := filepath.Join(specDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 标题撞车但内容不同。
	write("a.md", "# A\n\n## 认证\n\n用 Token。\n\n## 配色\n\n红。\n")
	write("b.md", "# B\n\n## 认证\n\n用 Cookie。\n\n## 部署\n\n手动。\n")
	// 标题相同且内容相同。
	write("c.md", "# C\n\n## 认证\n\n用 Token。\n")
	// 只有一份文件里有的标题不算重复。
	write("d.md", "# D\n\n## 部署\n\n自动。\n")
	dups, err := CheckDuplicates(specDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 2 {
		t.Fatalf("dups = %+v，想得到 2 处", dups)
	}
	byTitle := map[string]Duplicate{}
	for _, d := range dups {
		byTitle[d.Title] = d
	}
	// “认证”在 a/b/c 三处，内容不全相同。
	auth, ok := byTitle["认证"]
	if !ok || len(auth.Files) != 3 || auth.Identical {
		t.Errorf("认证重复不对：%+v", auth)
	}
	// “部署”在 b/d 两处，内容不同。
	deploy, ok := byTitle["部署"]
	if !ok || len(deploy.Files) != 2 || deploy.Identical {
		t.Errorf("部署重复不对：%+v", deploy)
	}
	// “配色”只有一份，不应列出。
	if _, ok := byTitle["配色"]; ok {
		t.Error("配色只有一份，不应列出")
	}
	// 内容完全相同的两处标 Identical。
	dir2 := t.TempDir()
	specDir2 := filepath.Join(dir2, ".sunshine", "spec")
	if err := os.MkdirAll(specDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"x.md", "y.md"} {
		if err := os.WriteFile(filepath.Join(specDir2, f), []byte("# T\n\n## 约定\n\n同一句话。\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dups2, err := CheckDuplicates(specDir2)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups2) != 1 || !dups2[0].Identical || dups2[0].Title != "约定" {
		t.Fatalf("内容相同的重复应标 Identical：%+v", dups2)
	}
	// 无重叠返回空。
	dir3 := t.TempDir()
	specDir3 := filepath.Join(dir3, ".sunshine", "spec")
	if err := os.MkdirAll(specDir3, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir3, "only.md"), []byte("# T\n\n## 甲\n\n## 乙\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dups3, err := CheckDuplicates(specDir3)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups3) != 0 {
		t.Fatalf("无重叠应返回空，得到 %+v", dups3)
	}
}
