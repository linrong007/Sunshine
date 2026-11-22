package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const testBin = "/usr/local/bin/sunshine"

func TestRewriteBashCommand(t *testing.T) {
	wrap := func(cmd string) string {
		return "'" + testBin + "' hook compress -- '" + cmd + "'"
	}
	tests := []struct {
		name    string
		command string
		want    string // "" 表示不改写
	}{
		{"git log", "git log", "git log --oneline"},
		{"git log 已带 oneline", "git log --oneline", ""},
		{"git log -p", "git log -p", ""},
		{"git log --format=", "git log --format=%s", ""},
		{"git log 带管道", "git log | head -20", ""},
		{"git log -5", "git log -5", "git log --oneline -5"},
		{"git diff", "git diff", "git diff --stat"},
		{"git diff --cached", "git diff --cached", "git diff --stat --cached"},
		{"git diff 已带 stat", "git diff --stat", ""},
		{"git diff 带范围与路径", "git diff HEAD~1 -- src/a.go", "git diff --stat HEAD~1 -- src/a.go"},
		{"git status", "git status", ""},
		{"go test", "go test ./...", wrap("go test ./...")},
		{"cd 后 go test", "cd /tmp && go test ./...", "cd /tmp && " + wrap("go test ./...")},
		{"go test 带管道", "go test ./... | grep FAIL", ""},
		{"go test 后接 echo", "go test ./... && echo done", wrap("go test ./...") + " && echo done"},
		{"引号里的 && 不切分", `echo "a && b" && go test ./...`, `echo "a && b" && ` + wrap("go test ./...")},
		{"引号不平衡", "git log --author='x", ""},
		{"普通命令", "echo hello", ""},
		{"npm test", "npm test", wrap("npm test")},
		{"make test", "make test", wrap("make test")},
		{"pytest", "pytest tests/", wrap("pytest tests/")},
		{"python -m pytest", "python -m pytest -q", wrap("python -m pytest -q")},
		{"已包过", "'" + testBin + "' hook compress -- 'go test ./...'", ""},
		{"sudo 开头", "sudo go test ./...", ""},
		{"空命令", "", ""},
		{"go test 2>&1", "go test ./... 2>&1", wrap("go test ./... 2>&1")},
		{"go test 后台 &", "go test ./... &", ""},
		{"分号两段都改", "go test ./...; git log", wrap("go test ./...") + "; git log --oneline"},
		{"go vet", "go vet ./...", wrap("go vet ./...")},
		{"cargo test", "cargo test", wrap("cargo test")},
		{"npx tsc", "npx tsc --noEmit", wrap("npx tsc --noEmit")},
		{"go test || 回退", "go test ./... || go test ./pkg/...", wrap("go test ./...") + " || " + wrap("go test ./pkg/...")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, note := rewriteBashCommand(tc.command, testBin, 0)
			if got != tc.want {
				t.Fatalf("rewriteBashCommand(%q) = %q, want %q", tc.command, got, tc.want)
			}
			if tc.want != "" && note == "" {
				t.Fatalf("rewriteBashCommand(%q) 改写了但说明为空", tc.command)
			}
			if tc.want == "" && note != "" {
				t.Fatalf("rewriteBashCommand(%q) 没改写但说明非空: %q", tc.command, note)
			}
		})
	}
}

func TestRewriteBashCommandTimeout(t *testing.T) {
	got, _ := rewriteBashCommand("go test ./...", testBin, 120000)
	want := "'" + testBin + "' hook compress --timeout 120000 -- 'go test ./...'"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitCommand(t *testing.T) {
	segs, seps, ok := splitCommand("cd /tmp && go test ./...; echo done")
	if !ok {
		t.Fatal("splitCommand 返回 ok=false")
	}
	if len(segs) != 3 || len(seps) != 3 {
		t.Fatalf("segs=%v seps=%v", segs, seps)
	}
	if seps[0] != "&&" || seps[1] != ";" || seps[2] != "" {
		t.Fatalf("seps=%v", seps)
	}
	// 拼回即原命令。
	var b strings.Builder
	for i := range segs {
		b.WriteString(segs[i])
		b.WriteString(seps[i])
	}
	if b.String() != "cd /tmp && go test ./...; echo done" {
		t.Fatalf("拼回 %q", b.String())
	}
	if _, _, ok := splitCommand("echo 'unbalanced"); ok {
		t.Fatal("引号不平衡应返回 ok=false")
	}
}

func TestRunPreToolUse(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"sess-1","cwd":"/tmp","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git log","description":"Show log","timeout":120000,"run_in_background":false}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("RunPreToolUse 返回错误: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("git log 应被改写，但无输出")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	specific, _ := decoded["hookSpecificOutput"].(map[string]any)
	if specific == nil {
		t.Fatal("缺少 hookSpecificOutput")
	}
	if specific["permissionDecision"] != "allow" {
		t.Fatalf("permissionDecision = %v, want allow", specific["permissionDecision"])
	}
	if specific["hookEventName"] != "PreToolUse" {
		t.Fatalf("hookEventName = %v, want PreToolUse（新版 Claude Code 严格校验，缺了 hook 报错）", specific["hookEventName"])
	}
	updated, _ := specific["updatedInput"].(map[string]any)
	if updated == nil {
		t.Fatal("缺少 updatedInput")
	}
	if updated["command"] != "git log --oneline" {
		t.Fatalf("updatedInput.command = %v", updated["command"])
	}
	// 原字段原样带上（updatedInput 替换整个对象，漏了就没了）。
	if updated["description"] != "Show log" {
		t.Fatalf("description 丢了: %v", updated["description"])
	}
	if updated["timeout"] != float64(120000) {
		t.Fatalf("timeout 丢了: %v", updated["timeout"])
	}
	if updated["run_in_background"] != false {
		t.Fatalf("run_in_background 丢了: %v", updated["run_in_background"])
	}
	if ctx, _ := specific["additionalContext"].(string); !strings.Contains(ctx, "git log") {
		t.Fatalf("additionalContext 缺少说明: %q", ctx)
	}
}

func TestRunPreToolUseNoRewrite(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	cases := []string{
		`{"session_id":"s","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":"echo hi"}}`,
		`{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x","limit":50}}`,
		`{"session_id":"s","cwd":"/tmp","tool_name":"Grep","tool_input":{"pattern":"foo","head_limit":10}}`,
		`{"session_id":"s","cwd":"/tmp","tool_name":"Glob","tool_input":{"pattern":"*.go"}}`,
		`{"session_id":"s","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":""}}`,
		`not json`,
		``,
	}
	for _, in := range cases {
		var out bytes.Buffer
		if err := RunPreToolUse(strings.NewReader(in), Options{Stdout: &out}); err != nil {
			t.Fatalf("输入 %q 返回错误: %v", in, err)
		}
		if out.Len() != 0 {
			t.Fatalf("输入 %q 应无输出，实际 %q", in, out.String())
		}
	}
}

func TestRunPreToolUseNonClaude(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "codex")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":"git log"}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("返回错误: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("非 Claude 平台应无输出，实际 %q", out.String())
	}
}

func TestRunPreToolUseDisabled(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	t.Setenv("SUNSHINE_HOOKS", "0")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":"git log"}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("返回错误: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("禁用时应无输出，实际 %q", out.String())
	}
}
