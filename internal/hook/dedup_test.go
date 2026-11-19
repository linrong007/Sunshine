package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dedupInput 组装一条 PostToolUse 输入；sessionID 为空时不带 session_id。
func dedupInput(t *testing.T, root, sessionID, toolName string, toolInput map[string]any, toolResponse any) string {
	t.Helper()
	m := map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       toolName,
		"tool_input":      toolInput,
		"tool_response":   toolResponse,
		"cwd":             root,
	}
	if sessionID != "" {
		m["session_id"] = sessionID
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runPostToolUseOut(t *testing.T, input string) string {
	t.Helper()
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunPostToolUse(in, o) }, input)
	return out
}

// dedupLines 读去重表行数（文件不存在返回 0）。
func dedupLines(t *testing.T, root, sessionID string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".sunshine", ".runtime", "dedup-"+sessionID+".jsonl"))
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

// updatedToolOutput 从 hook 输出里取 updatedToolOutput；无输出返回 nil。
func updatedToolOutput(t *testing.T, out string) any {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		return nil
	}
	data := mustJSON(t, out)
	hs, ok := data["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("输出缺少 hookSpecificOutput：%s", out)
	}
	return hs["updatedToolOutput"]
}

func bashResponse(stdout, stderr string) map[string]any {
	return map[string]any{"stdout": stdout, "stderr": stderr, "interrupted": false, "isImage": false}
}

func TestDedup_BashRepeat(t *testing.T) {
	root, _ := makeProject(t)
	outText := strings.Repeat("M src/foo.go\n", 20) // 约 260 字节，比引用长
	in := dedupInput(t, root, "sess-1", "Bash",
		map[string]any{"command": "git status --porcelain"},
		bashResponse(outText, ""))
	if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换输出，实际输出：%s", got)
	}
	if n := dedupLines(t, root, "sess-1"); n != 1 {
		t.Fatalf("首次调用后去重表应有 1 行，实际 %d 行", n)
	}
	out2 := runPostToolUseOut(t, in)
	repl := updatedToolOutput(t, out2)
	m, ok := repl.(map[string]any)
	if !ok {
		t.Fatalf("Bash 替换值应为对象形，实际：%v", repl)
	}
	want := "[Sunshine: 与本会话第 1 次工具调用的输出相同，已在上下文中，不再重复]"
	if m["stdout"] != want {
		t.Errorf("替换后的 stdout 应为引用，实际：%v", m["stdout"])
	}
	if m["stderr"] != "" || m["interrupted"] != false || m["isImage"] != false {
		t.Errorf("替换后 stderr 应置空、interrupted/isImage 应原样保留，实际：%v", m)
	}
	if n := dedupLines(t, root, "sess-1"); n != 1 {
		t.Fatalf("命中后不应再记一条，表应仍为 1 行，实际 %d 行", n)
	}
}

func TestDedup_BashChangedOutput(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "git status --porcelain"}
	a := strings.Repeat("M src/foo.go\n", 20)
	b := strings.Repeat("M src/bar.go\n", 20)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	// 同命令、不同输出：不命中，原样放过，并记第二条。
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(b, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("输出变了不应替换，实际：%s", got)
	}
	if n := dedupLines(t, root, "sess-1"); n != 2 {
		t.Fatalf("两次不同输出后表应有 2 行，实际 %d 行", n)
	}
	// 再跑第一次的输出：命中第一条，引用第 1 次。
	out3 := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a, "")))
	repl := updatedToolOutput(t, out3)
	m, ok := repl.(map[string]any)
	if !ok || !strings.Contains(m["stdout"].(string), "第 1 次") {
		t.Fatalf("重跑第一次的输出应引用第 1 次，实际：%v", repl)
	}
}

func TestDedup_ReadRepeat(t *testing.T) {
	root, _ := makeProject(t)
	content := strings.Repeat("package demo\n", 30) // 约 390 字节
	in := dedupInput(t, root, "sess-1", "Read",
		map[string]any{"file_path": "src/a.go"},
		content)
	if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
		t.Fatalf("首次读不应替换，实际：%s", got)
	}
	out2 := runPostToolUseOut(t, in)
	repl := updatedToolOutput(t, out2)
	s, ok := repl.(string)
	if !ok || !strings.Contains(s, "第 1 次") {
		t.Fatalf("重读同一文件应换成字符串形引用，实际：%v", repl)
	}
}

func TestDedup_ReadRangeKey(t *testing.T) {
	root, _ := makeProject(t)
	content := strings.Repeat("package demo\n", 30)
	base := map[string]any{"file_path": "src/a.go"}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", base, content)); strings.TrimSpace(got) != "" {
		t.Fatalf("首次读不应替换，实际：%s", got)
	}
	// 同文件、不同行范围：键不同，不命中。
	ranged := map[string]any{"file_path": "src/a.go", "offset": 10, "limit": 20}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", ranged, content)); strings.TrimSpace(got) != "" {
		t.Fatalf("行范围不同不应替换，实际：%s", got)
	}
}

func TestDedup_NoSessionID(t *testing.T) {
	root, _ := makeProject(t)
	outText := strings.Repeat("M src/foo.go\n", 20)
	in := dedupInput(t, root, "", "Bash",
		map[string]any{"command": "git status --porcelain"},
		bashResponse(outText, ""))
	for i := 0; i < 2; i++ {
		if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
			t.Fatalf("无 session_id 时不应替换，实际：%s", got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".sunshine", ".runtime")); err == nil {
		if entries, _ := os.ReadDir(filepath.Join(root, ".sunshine", ".runtime")); len(entries) > 0 {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "dedup-") {
					t.Fatalf("无 session_id 时不应建去重表，实际建了 %s", e.Name())
				}
			}
		}
	}
}

func TestDedup_SmallOutput(t *testing.T) {
	root, _ := makeProject(t)
	in := dedupInput(t, root, "sess-1", "Bash",
		map[string]any{"command": "echo ok"},
		bashResponse("ok\n", ""))
	for i := 0; i < 2; i++ {
		if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
			t.Fatalf("输出比引用短时不应替换，实际：%s", got)
		}
	}
}

func TestDedup_NonClaude(t *testing.T) {
	root, _ := makeProject(t)
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "codex")
	outText := strings.Repeat("M src/foo.go\n", 20)
	in := dedupInput(t, root, "sess-1", "Bash",
		map[string]any{"command": "git status --porcelain"},
		bashResponse(outText, ""))
	for i := 0; i < 2; i++ {
		if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
			t.Fatalf("非 Claude 平台不应替换，实际：%s", got)
		}
	}
	if n := dedupLines(t, root, "sess-1"); n != 0 {
		t.Fatalf("非 Claude 平台不应记表，实际 %d 行", n)
	}
}

func TestDedup_BashStderr(t *testing.T) {
	root, _ := makeProject(t)
	outText := strings.Repeat("boom\n", 30)
	in := dedupInput(t, root, "sess-1", "Bash",
		map[string]any{"command": "make test"},
		bashResponse(outText, "error: failed"))
	for i := 0; i < 2; i++ {
		if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
			t.Fatalf("stderr 非空（失败）时不应替换，实际：%s", got)
		}
	}
	if n := dedupLines(t, root, "sess-1"); n != 0 {
		t.Fatalf("失败输出不应记表，实际 %d 行", n)
	}
}

func TestDedup_BashExitCode(t *testing.T) {
	root, _ := makeProject(t)
	outText := strings.Repeat("boom\n", 30)
	resp := bashResponse(outText, "")
	resp["exitCode"] = float64(1)
	in := dedupInput(t, root, "sess-1", "Bash",
		map[string]any{"command": "make test"}, resp)
	if got := runPostToolUseOut(t, in); strings.TrimSpace(got) != "" {
		t.Fatalf("exitCode 非 0 时不应替换，实际：%s", got)
	}
	if n := dedupLines(t, root, "sess-1"); n != 0 {
		t.Fatalf("exitCode 非 0 的输出不应记表，实际 %d 行", n)
	}
}
