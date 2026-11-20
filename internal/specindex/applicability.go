// Package specindex 的适用标签与查重：每个 spec 文件头有一行
// YAML frontmatter 适用标签（前端 / 后端 / 通用），缺了按文件名
// 自动补；CheckDuplicates 比对各文件索引章节，找出重复内容。
package specindex

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 适用范围：spec 文件头 frontmatter 里的 适用 标签。
const (
	ScopeGeneral  = "通用"
	ScopeFrontend = "前端"
	ScopeBackend  = "后端"
)

// NormalizeScope 把适用值归一到三档；未知值算通用。
func NormalizeScope(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "前端", "frontend":
		return ScopeFrontend
	case "后端", "backend":
		return ScopeBackend
	case "通用", "general", "system":
		return ScopeGeneral
	default:
		return ScopeGeneral
	}
}

// InferApplicability 按文件名推断适用：含 frontend→前端，
// backend→后端，其余（system 与其他）→通用。
func InferApplicability(filename string) string {
	base := strings.ToLower(filepath.Base(filename))
	switch {
	case strings.Contains(base, "frontend"):
		return ScopeFrontend
	case strings.Contains(base, "backend"):
		return ScopeBackend
	default:
		return ScopeGeneral
	}
}

// splitScopeKV 把 frontmatter 里的一行按第一个冒号（中文或英文）切成键值。
func splitScopeKV(line string) (key, val string, ok bool) {
	for _, sep := range []string{"：", ":"} {
		if idx := strings.Index(line, sep); idx >= 0 {
			return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+len(sep):]), true
		}
	}
	return "", "", false
}

// frontmatterSpan 定位文件头 YAML frontmatter 块：返回开块与闭块的行号
// （不含 --- 本身的行号区间为内容行）。文件不是以 --- 开头、或 100 行
// 内找不到闭块时返回 -1, -1。
func frontmatterSpan(lines []string) (start, end int) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1, -1
	}
	for i := 1; i < len(lines) && i < 100; i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return 0, i
		}
	}
	return -1, -1
}

// ParseApplicability 读文件头 YAML frontmatter 里的 适用 行，没有返回 ""。
// 接受中文冒号与英文冒号，值两侧的引号去掉；不做归一（归一由调用方做）。
func ParseApplicability(content string) string {
	lines := strings.Split(content, "\n")
	start, end := frontmatterSpan(lines)
	if start < 0 {
		return ""
	}
	for _, line := range lines[start+1 : end] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, val, ok := splitScopeKV(trimmed)
		if !ok || key != "适用" {
			continue
		}
		return strings.Trim(val, `"'`)
	}
	return ""
}

// EnsureApplicability：已有适用行不动；缺了按文件名推断，写进文件头
// frontmatter（已有 frontmatter 时插进块里，否则在最前面新建一块）。
func EnsureApplicability(content, filename string) (string, bool) {
	if ParseApplicability(content) != "" {
		return content, false
	}
	scope := InferApplicability(filename)
	lines := strings.Split(content, "\n")
	if start, end := frontmatterSpan(lines); start >= 0 {
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:1]...)
		out = append(out, "适用："+scope)
		out = append(out, lines[1:end+1]...)
		out = append(out, lines[end+1:]...)
		return strings.Join(out, "\n"), true
	}
	return "---\n适用：" + scope + "\n---\n\n" + content, true
}

// SpecEntry 是规范地图里的一行：spec 文件的路径、适用与索引章节。
type SpecEntry struct {
	Path     string   // 相对项目根的 POSIX 路径，如 .sunshine/spec/api.md
	Scope    string   // 前端 / 后端 / 通用
	Sections []string // 索引章节名（文件顺序；无 ## 节时为空）
}

// CollectSpecMap 列出 specDir 下全部 spec 文件（递归），供规范地图用。
// 与 SpecFiles 不同：guides/ 下只保留 guides/index.md 本身，
// 其余文件不进地图；隐藏目录/文件跳过。适用取文件头标签，
// 缺了按文件名推断；章节按文件内 ## 标题顺序取。
func CollectSpecMap(specDir string) ([]SpecEntry, error) {
	var out []SpecEntry
	err := filepath.WalkDir(specDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// 单个坏条目（悬空软链接、无权限目录等）跳过继续，
			// 不能让一个坏文件把整个规范地图清空；根目录本身
			// 读不了才是真错误。
			if path == specDir {
				return err
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(specDir, path)
		if err != nil {
			return nil // 单个条目相对路径算不出：跳过该条目继续
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if strings.HasPrefix(relSlash, "guides/") && relSlash != "guides/index.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // 单文件读失败（悬空软链接/不可读）：跳过该文件继续
		}
		content := string(data)
		scope := ParseApplicability(content)
		if scope == "" {
			scope = InferApplicability(d.Name())
		} else {
			scope = NormalizeScope(scope)
		}
		var sections []string
		for _, s := range ParseSections(content) {
			sections = append(sections, s.Title)
		}
		out = append(out, SpecEntry{
			Path:     filepath.ToSlash(filepath.Join(".sunshine/spec", relSlash)),
			Scope:    scope,
			Sections: sections,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// FormatSpecMap 把地图条目按适用分组写成文本：通用组、前端组、后端组，
// 每行 `路径 — 章节1、章节2`，无索引节只列路径。空地图返回 ""。
func FormatSpecMap(entries []SpecEntry) string {
	groups := []string{ScopeGeneral, ScopeFrontend, ScopeBackend}
	byScope := map[string][]SpecEntry{}
	for _, e := range entries {
		byScope[NormalizeScope(e.Scope)] = append(byScope[NormalizeScope(e.Scope)], e)
	}
	var b strings.Builder
	for _, g := range groups {
		list := byScope[g]
		if len(list) == 0 {
			continue
		}
		b.WriteString("### " + g + "组\n")
		for _, e := range list {
			b.WriteString("- " + e.Path)
			if len(e.Sections) > 0 {
				b.WriteString(" — " + strings.Join(e.Sections, "、"))
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Duplicate 是一处跨文件重复：同一章节标题出现在多个文件里。
type Duplicate struct {
	Title     string   // 撞车的章节标题
	Files     []string // 相对项目根的 POSIX 路径（已排序）
	Identical bool     // 各文件该节正文是否完全相同
}

// indexTitles 取文件索引章节：有索引节时用索引表里的标题，
// 否则按 ## 标题现算。
func indexTitles(content string) []string {
	if idx := ParseIndex(content); len(idx) > 0 {
		titles := make([]string, 0, len(idx))
		for t := range idx {
			titles = append(titles, t)
		}
		sort.Strings(titles)
		return titles
	}
	var titles []string
	for _, s := range ParseSections(content) {
		titles = append(titles, s.Title)
	}
	return titles
}

// sectionBodies 按 ## 标题切出各节正文（标题行本身不计，首尾空白去掉）。
func sectionBodies(content string) map[string]string {
	bodies := map[string]string{}
	lines := strings.Split(content, "\n")
	cur := ""
	var buf []string
	flush := func() {
		if cur != "" {
			bodies[cur] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			cur = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			buf = nil
			continue
		}
		if cur != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return bodies
}

// CheckDuplicates 扫 specDir 下全部 .md（与 SpecFiles 同范围，跳过 guides/），
// 比对各文件索引章节：同一标题出现在多个文件里即列出；
// 各文件该节正文完全相同时再标内容相同。无重叠返回空。
func CheckDuplicates(specDir string) ([]Duplicate, error) {
	files, err := SpecFiles(specDir)
	if err != nil {
		return nil, err
	}
	type occurrence struct {
		file string
		hash string
	}
	byTitle := map[string][]occurrence{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		content := string(data)
		rel, err := filepath.Rel(specDir, f)
		if err != nil {
			return nil, err
		}
		relPath := filepath.ToSlash(filepath.Join(".sunshine/spec", rel))
		bodies := sectionBodies(content)
		for _, title := range indexTitles(content) {
			sum := sha256.Sum256([]byte(bodies[title]))
			byTitle[title] = append(byTitle[title], occurrence{file: relPath, hash: hex.EncodeToString(sum[:])})
		}
	}
	var out []Duplicate
	for title, occ := range byTitle {
		seen := map[string]bool{}
		var uniq []string
		for _, o := range occ {
			if !seen[o.file] {
				seen[o.file] = true
				uniq = append(uniq, o.file)
			}
		}
		if len(uniq) < 2 {
			continue
		}
		identical := true
		for i := 1; i < len(occ); i++ {
			if occ[i].hash != occ[0].hash {
				identical = false
				break
			}
		}
		sort.Strings(uniq)
		out = append(out, Duplicate{Title: title, Files: uniq, Identical: identical})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}
