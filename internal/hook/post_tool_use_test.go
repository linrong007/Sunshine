package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runPostToolUse(t *testing.T, input string) {
	t.Helper()
	_, _ = runWithInput(t, func(in *strings.Reader, o Options) error { return RunPostToolUse(in, o) }, input)
}

func specContent(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".sunshine", "spec", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestPostToolUse_EditSpecWithIndex(t *testing.T) {
	root, _ := makeProject(t)
	p := filepath.Join(root, ".sunshine", "spec", "backend.md")
	mustWrite(t, p, "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n| --- | --- |\n| 接口 | `## 接口` |\n\n## 接口\n\n## 新增节\n\n正文。\n")
	runPostToolUse(t, `{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":`+strconv_quot(p)+`},"cwd":`+strconv_quot(root)+`}`)
	if !strings.Contains(specContent(t, root, "backend.md"), "| 新增节 |") {
		t.Fatal("Edit 改有索引节的 spec 文件后，索引表里应自动同步出新增节")
	}
}

func TestPostToolUse_EditSpecWithoutIndexGetsIndex(t *testing.T) {
	root, _ := makeProject(t)
	p := filepath.Join(root, ".sunshine", "spec", "frontend.md")
	mustWrite(t, p, "# 前端规范\n\n## 技术栈\n\n- Vue。\n")
	runPostToolUse(t, `{"hook_event_name":"PostToolUse","tool_name":"Write","tool_input":{"file_path":`+strconv_quot(p)+`},"cwd":`+strconv_quot(root)+`}`)
	// 索引是强制的：无索引节的文件被改后也要补一个。
	if got := specContent(t, root, "frontend.md"); !strings.Contains(got, "## 索引") || !strings.Contains(got, "| 技术栈 |") {
		t.Fatalf("无索引节的 spec 文件被改后应补索引，实际：\n%s", got)
	}
}

func TestPostToolUse_NonSpecFileIgnored(t *testing.T) {
	root, _ := makeProject(t)
	p := filepath.Join(root, ".sunshine", "spec", "backend.md")
	orig := "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n| --- | --- |\n| 接口 | `## 接口` |\n\n## 接口\n\n## 新增节\n"
	mustWrite(t, p, orig)
	other := filepath.Join(root, "README.md")
	mustWrite(t, other, "# 项目\n")
	runPostToolUse(t, `{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":`+strconv_quot(other)+`},"cwd":`+strconv_quot(root)+`}`)
	if got := specContent(t, root, "backend.md"); got != orig {
		t.Fatalf("改非 spec 文件时，spec 文件应原样不动，实际：\n%s", got)
	}
}

func TestPostToolUse_BashSyncsDir(t *testing.T) {
	root, _ := makeProject(t)
	p := filepath.Join(root, ".sunshine", "spec", "backend.md")
	mustWrite(t, p, "# 后端规范\n\n## 索引\n\n| 内容 | 位置 |\n\n## 接口\n\n## 新增节\n")
	// Bash 改 spec（sed/heredoc 等）：file_path 不可靠，走整目录同步。
	runPostToolUse(t, `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"sed -i 's/旧/新/' .sunshine/spec/backend.md"},"cwd":`+strconv_quot(root)+`}`)
	if !strings.Contains(specContent(t, root, "backend.md"), "| 新增节 |") {
		t.Fatal("Bash 后应整目录同步，有索引节的 spec 文件索引表里应有新增节")
	}
}

func TestPostToolUse_BadInputNeverErrors(t *testing.T) {
	root, _ := makeProject(t)
	// 非 JSON、非 Edit/Write/Bash、缺 file_path：一律静默返回 nil。
	for _, input := range []string{
		`不是 JSON`,
		`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"x.md"},"cwd":` + strconv_quot(root) + `}`,
		`{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{},"cwd":` + strconv_quot(root) + `}`,
		`{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":"/tmp/不存在.md"},"cwd":` + strconv_quot(root) + `}`,
	} {
		runPostToolUse(t, input)
	}
}
