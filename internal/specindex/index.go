// Package specindex 同步 .sunshine/spec 下 Markdown 文件头上的 ## 索引节。
//
// 索引是导航，不是替代：只列各 ## 节的位置，agent 先读索引定位，再按需读
// 正文；拿不准就读正文。索引由标题生成，不花 token、不调模型，
// 因此可以由命令自动维护（sunshine spec index），不用人耗心智去对。
package specindex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Section 是 spec 文件里的一个 ## 标题。
type Section struct {
	Title string // 标题文本（去掉 ## 和前后空格）
}

// ParseSections 解析 Markdown 文本，返回所有 ## 标题（跳过 ## 索引本身）。
// 只取 ##，不取 ###——索引定位到大节就够了，子节由正文自己展开。
func ParseSections(content string) []Section {
	var out []Section
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		if title == "" || title == "索引" {
			continue
		}
		out = append(out, Section{Title: title})
	}
	return out
}

// ParseIndex 解析已有的索引表，返回标题 -> 描述。
// 索引表格式：| 内容 | 位置 |，内容形如“标题（描述）”或“标题”，
// 位置形如“`## 标题`”。没有括号时描述为空字符串。
func ParseIndex(content string) map[string]string {
	preserved := map[string]string{}
	inIndex := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")) == "索引" {
				inIndex = true
			} else {
				inIndex = false
			}
			continue
		}
		if !inIndex || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		parts := strings.Split(trimmed, "|")
		if len(parts) < 3 {
			continue
		}
		contentCell := strings.TrimSpace(parts[1])
		locCell := strings.TrimSpace(parts[2])
		if contentCell == "内容" || strings.HasPrefix(contentCell, "---") {
			continue // 表头与分隔行
		}
		loc := strings.TrimSpace(strings.TrimPrefix(strings.Trim(locCell, "`"), "## "))
		if loc == "" {
			continue
		}
		desc := ""
		if idx := strings.Index(contentCell, "（"); idx >= 0 {
			if end := strings.Index(contentCell[idx:], "）"); end >= 0 {
				desc = contentCell[idx+len("（") : idx+end]
			}
		}
		preserved[loc] = desc
	}
	return preserved
}

// BuildIndex 生成索引节的 Markdown 文本（末尾带一个换行，不含节后空行）。
func BuildIndex(sections []Section, preserved map[string]string) string {
	var b strings.Builder
	b.WriteString("## 索引\n\n")
	b.WriteString("索引是导航，不是替代：先读索引定位，再按需读正文；拿不准就读正文。\n\n")
	b.WriteString("| 内容 | 位置 |\n")
	b.WriteString("| --- | --- |\n")
	for _, s := range sections {
		content := s.Title
		if desc := preserved[s.Title]; desc != "" {
			content += "（" + desc + "）"
		}
		b.WriteString("| " + content + " | `## " + s.Title + "` |\n")
	}
	return b.String()
}

// indexSpan 定位索引节：返回起始行与结束行（结束行是下一节 ## 标题的行号，
// 索引是文件最后一节时为文件总行数）。没有索引节时起始行为 -1。
func indexSpan(lines []string) (start, end int) {
	start, end = -1, len(lines)
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		if title == "索引" && start < 0 {
			start = i
		} else if start >= 0 {
			end = i
			break
		}
	}
	return start, end
}

// SyncContent 同步单个文件内容的索引节，返回新内容与是否改动。
// 没有索引节时在文件标题（# ）后插入；有则按当前标题重生成并替换，
// 旧索引里的描述（括号部分）保留，新节只写标题。
func SyncContent(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	sections := ParseSections(content)
	if len(sections) == 0 {
		return content, false // 没有 ## 节，不用建索引
	}
	preserved := ParseIndex(content)
	indexText := strings.TrimSuffix(BuildIndex(sections, preserved), "\n")
	indexLines := strings.Split(indexText, "\n")

	start, end := indexSpan(lines)
	if start < 0 {
		// 没有索引节：在文件标题后插入。
		titleLine := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "# ") {
				titleLine = i
				break
			}
		}
		if titleLine < 0 {
			return content, false // 没有标题，不处理
		}
		rest := lines[titleLine+1:]
		for len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
			rest = rest[1:] // 标题后原有的空行只留一个
		}
		var out []string
		out = append(out, lines[:titleLine+1]...)
		out = append(out, "")
		out = append(out, indexLines...)
		out = append(out, "")
		out = append(out, rest...)
		newContent := strings.Join(out, "\n")
		return newContent, newContent != content
	}
	// 有索引节：替换。
	var out []string
	out = append(out, lines[:start]...)
	out = append(out, indexLines...)
	if end < len(lines) {
		out = append(out, "")
		out = append(out, lines[end:]...)
	}
	newContent := strings.Join(out, "\n")
	return newContent, newContent != content
}

// SyncFile 同步单个 spec 文件的索引节，返回是否改动。
func SyncFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, changed := SyncContent(string(raw))
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// CheckFile 只检查单个 spec 文件的索引是否同步，不写盘。
func CheckFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	_, changed := SyncContent(string(raw))
	return changed, nil
}

// SpecFiles 列出 specDir 下全部待同步的 Markdown 文件（递归，跳过 guides/）。
func SpecFiles(specDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(specDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "guides" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// SyncDir 同步 specDir 下全部 spec 文件的索引，返回改动的文件列表。
func SyncDir(specDir string) ([]string, error) {
	files, err := SpecFiles(specDir)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, f := range files {
		ok, err := SyncFile(f)
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, f)
		}
	}
	return changed, nil
}

// CheckDir 只检查 specDir 下全部 spec 文件的索引是否同步，不写盘，
// 返回不同步的文件列表。
func CheckDir(specDir string) ([]string, error) {
	files, err := SpecFiles(specDir)
	if err != nil {
		return nil, err
	}
	var stale []string
	for _, f := range files {
		ok, err := CheckFile(f)
		if err != nil {
			return stale, err
		}
		if ok {
			stale = append(stale, f)
		}
	}
	return stale, nil
}
