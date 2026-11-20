// Package specindex 同步 .sunshine/spec 下 Markdown 文件头上的 ## 索引节。
//
// 索引是导航，不是替代：只列各 ## 节的位置，agent 先读索引定位，再按需读
// 正文；拿不准就读正文。索引由标题生成，不花 token、不调模型，
// 因此可以由命令自动维护（sunshine spec index），不用人耗心智去对。
package specindex

import (
	"bytes"
	"fmt"
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

// SyncContent 同步单个文件内容的索引节与适用标签，返回新内容与是否改动。
// 没有索引节时在文件标题（# ）后插入；有则按当前标题重生成并替换，
// 旧索引里的描述（括号部分）保留，新节只写标题。
// 缺适用标签时按通用补（文件名未知时无法按名推断）；SyncFile 会传真实文件名。
func SyncContent(content string) (string, bool) {
	return SyncContentFor(content, "")
}

// SyncContentFor 与 SyncContent 相同，另按 filename 推断缺失的适用标签。
func SyncContentFor(content, filename string) (string, bool) {
	updated, tagChanged := EnsureApplicability(content, filename)
	content = updated
	changed := tagChanged
	lines := strings.Split(content, "\n")
	sections := ParseSections(content)
	if len(sections) == 0 {
		return content, changed // 没有 ## 节，不建索引；标签已补
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
			return content, changed // 没有标题，不处理
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
		return newContent, changed || newContent != content
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
	return newContent, changed || newContent != content
}

// writeFileAtomic 原子替换文件内容：先在同目录写临时文件，再 rename
// 覆盖目标。同目录保证 rename 在同一文件系统内、是原子操作，读者不会
// 读到写了一半的内容。perm 为目标文件应有的权限（保留原权限）。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".specindex-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后该路径已不存在，Remove 无害
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// SyncFile 同步单个 spec 文件的索引节与适用标签，返回是否改动。
// 写盘走同目录临时文件 + rename 原子替换，并保留原文件权限；
// 写前再比对一次内容：读盘后文件被别人改过（两次 Edit 快速连续、
// SessionStart 全量同步与 Edit 并发）时重读重算，不拿旧内容覆盖新改动。
func SyncFile(path string) (bool, error) {
	for attempt := 0; attempt < 5; attempt++ {
		info, err := os.Stat(path)
		if err != nil {
			return false, err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		updated, changed := SyncContentFor(string(raw), filepath.Base(path))
		if !changed {
			return false, nil
		}
		cur, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(cur, raw) {
			continue // 期间被改过：重读重算，别覆盖新改动
		}
		if err := writeFileAtomic(path, []byte(updated), info.Mode().Perm()); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, fmt.Errorf("spec 文件在同步期间持续变化，已放弃写入：%s", path)
}

// CheckFile 只检查单个 spec 文件的索引与适用标签是否同步，不写盘。
func CheckFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	_, changed := SyncContentFor(string(raw), filepath.Base(path))
	return changed, nil
}

// SpecFiles 列出 specDir 下全部待同步的 Markdown 文件（递归，跳过 guides/）。
func SpecFiles(specDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(specDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// 单个坏条目（悬空软链接、无权限目录等）跳过继续，
			// 不让一个坏文件 abort 整个目录的同步；根目录本身
			// 读不了才是真错误。
			if path == specDir {
				return err
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
// 单个文件失败（悬空软链接、不可读等）跳过继续同步其余文件，
// 最后汇总返回一个带跳过文件清单的错误，不中断全量同步。
func SyncDir(specDir string) ([]string, error) {
	files, err := SpecFiles(specDir)
	if err != nil {
		return nil, err
	}
	var changed []string
	var skipped []string
	for _, f := range files {
		ok, err := SyncFile(f)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", f, err))
			continue
		}
		if ok {
			changed = append(changed, f)
		}
	}
	if len(skipped) > 0 {
		return changed, fmt.Errorf("同步 spec 索引跳过 %d 个文件：%s", len(skipped), strings.Join(skipped, "; "))
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
