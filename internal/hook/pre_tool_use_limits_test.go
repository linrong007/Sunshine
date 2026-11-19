package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runPreToolUseLimitOut(t *testing.T, input string) (string, map[string]any) {
	t.Helper()
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("RunPreToolUse 返回错误：%v", err)
	}
	if out.Len() == 0 {
		return "", nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("输出不是合法 JSON：%v", err)
	}
	specific, _ := decoded["hookSpecificOutput"].(map[string]any)
	if specific == nil {
		t.Fatalf("缺少 hookSpecificOutput：%s", out.String())
	}
	updated, _ := specific["updatedInput"].(map[string]any)
	return out.String(), map[string]any{"specific": specific, "updated": updated}
}

func TestPreToolUseLimit_ReadInject(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x.go","offset":10}}`
	_, parts := runPreToolUseLimitOut(t, input)
	if parts == nil {
		t.Fatal("Read 缺 limit 应注入，但无输出")
	}
	specific := parts["specific"].(map[string]any)
	updated := parts["updated"].(map[string]any)
	if specific["permissionDecision"] != "allow" {
		t.Fatalf("permissionDecision = %v, want allow", specific["permissionDecision"])
	}
	if updated["limit"] != float64(300) {
		t.Fatalf("limit = %v, want 300", updated["limit"])
	}
	if updated["file_path"] != "/tmp/x.go" || updated["offset"] != float64(10) {
		t.Fatalf("原字段应原样保留，实际：%v", updated)
	}
	if ctx, _ := specific["additionalContext"].(string); !strings.Contains(ctx, "limit=300") {
		t.Fatalf("additionalContext 应说明注入了 limit=300，实际：%q", ctx)
	}
}

func TestPreToolUseLimit_ReadExplicitKept(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	for _, in := range []string{
		`{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x.go","limit":5000}}`,
		`{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x.go","limit":0}}`,
	} {
		var out bytes.Buffer
		if err := RunPreToolUse(strings.NewReader(in), Options{Stdout: &out}); err != nil {
			t.Fatalf("输入 %q 返回错误：%v", in, err)
		}
		if out.Len() != 0 {
			t.Fatalf("已有显式 limit 时不应改写，输入 %q 实际输出 %q", in, out.String())
		}
	}
}

func TestPreToolUseLimit_ReadNullLimitInject(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x.go","limit":null}}`
	_, parts := runPreToolUseLimitOut(t, input)
	if parts == nil {
		t.Fatal("limit 为 null 应视为缺失并注入，但无输出")
	}
	if parts["updated"].(map[string]any)["limit"] != float64(300) {
		t.Fatalf("limit 应注入 300，实际：%v", parts["updated"])
	}
}

func TestPreToolUseLimit_GrepInject(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Grep","tool_input":{"pattern":"TODO","path":"/tmp"}}`
	_, parts := runPreToolUseLimitOut(t, input)
	if parts == nil {
		t.Fatal("Grep 缺 head_limit 应注入，但无输出")
	}
	specific := parts["specific"].(map[string]any)
	updated := parts["updated"].(map[string]any)
	if specific["permissionDecision"] != "allow" {
		t.Fatalf("permissionDecision = %v, want allow", specific["permissionDecision"])
	}
	if updated["head_limit"] != float64(100) {
		t.Fatalf("head_limit = %v, want 100", updated["head_limit"])
	}
	if updated["pattern"] != "TODO" || updated["path"] != "/tmp" {
		t.Fatalf("原字段应原样保留，实际：%v", updated)
	}
	if ctx, _ := specific["additionalContext"].(string); !strings.Contains(ctx, "head_limit=100") {
		t.Fatalf("additionalContext 应说明注入了 head_limit=100，实际：%q", ctx)
	}
}

func TestPreToolUseLimit_GrepExplicitKept(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Grep","tool_input":{"pattern":"TODO","head_limit":50}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("返回错误：%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("已有显式 head_limit 时不应改写，实际：%q", out.String())
	}
}

func TestPreToolUseLimit_GlobNoInject(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "claude")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Glob","tool_input":{"pattern":"*.go"}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("返回错误：%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Glob 不注入，应无输出，实际：%q", out.String())
	}
}

func TestPreToolUseLimit_NonClaude(t *testing.T) {
	t.Setenv("SUNSHINE_HOOK_PLATFORM", "codex")
	input := `{"session_id":"s","cwd":"/tmp","tool_name":"Read","tool_input":{"file_path":"/tmp/x.go"}}`
	var out bytes.Buffer
	if err := RunPreToolUse(strings.NewReader(input), Options{Stdout: &out}); err != nil {
		t.Fatalf("返回错误：%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("非 Claude 平台不应注入，实际：%q", out.String())
	}
}
