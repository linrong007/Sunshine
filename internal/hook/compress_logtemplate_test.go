package hook

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogTemplate_BasicFold(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "request req-%d completed in %dms\n", i, 100+i%50)
	}
	out := compressStream(b.String())
	if !strings.Contains(out, "[×300] request {} completed in {}  (e.g. request req-0 completed in 100ms)") {
		t.Fatalf("300 行同模板应折成一行标记，实际：\n%s", out)
	}
	if !strings.Contains(out, "原 300 行") {
		t.Fatalf("截断说明应写原行数，实际首行：%q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Contains(out, "req-1 completed") {
		t.Fatalf("其余出现应直接丢弃，不应残留 req-1 行，实际：\n%s", out)
	}
}

func TestLogTemplate_VariableClasses(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("request req-%d completed in %dms", i, 100+i))
	}
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("user=alice%d action=login", i))
	}
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("trace deadbeef%04d done", i))
	}
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("release v1.24.0 took %dms", 200+i))
	}
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("build 1.2.3 took %ds", 5+i%10))
	}
	// 另加数字形与 IP：时间戳与 IP 应被换成 {}。
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("conn 192.168.0.%d opened at 12:30:%02d", i, i%60))
	}
	got := foldLogTemplates(lines)
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"[×50] request {} completed in {}",
		"[×50] user={} action={}",
		"[×50] trace {} done",
		"[×50] release v1.24.0 took {}",
		"[×50] build 1.2.3 took {}",
		"[×50] conn {} opened at {}",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("应折叠出 %q，实际：\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "build {} took") {
		t.Errorf("版本号 1.2.3 应保留在模板里，不应被换成 {}：\n%s", joined)
	}
	if len(got) != 6 {
		t.Fatalf("300 行应折成 6 行标记，实际 %d 行：\n%s", len(got), joined)
	}
}

func TestLogTemplate_Threshold(t *testing.T) {
	// 模板只出现 2 次：不折叠，两行原样保留。
	lines := []string{
		"request req-1 completed in 100ms",
		"request req-2 completed in 200ms",
		"plain line one",
		"plain line two",
	}
	got := foldLogTemplates(lines)
	if len(got) != 4 {
		t.Fatalf("出现 2 次不应折叠，实际 %d 行：%v", len(got), got)
	}
	if got[0] != lines[0] || got[1] != lines[1] {
		t.Fatalf("原样保留的行不对：%v", got)
	}
	// 出现 3 次：折叠。
	lines3 := append(append([]string{}, lines[:2]...), "request req-3 completed in 300ms")
	got3 := foldLogTemplates(lines3)
	if len(got3) != 1 || !strings.HasPrefix(got3[0], "[×3] request {} completed in {}") {
		t.Fatalf("出现 3 次应折叠，实际：%v", got3)
	}
}

func TestLogTemplate_FailureLinesNeverFold(t *testing.T) {
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("ERROR req-%d failed after %dms", i, 100+i))
	}
	got := foldLogTemplates(lines)
	if len(got) != 5 {
		t.Fatalf("失败行永不折叠，应原样保留 5 行，实际：%v", got)
	}
	for i, l := range lines {
		if got[i] != l {
			t.Fatalf("失败行应原样保留，got[%d]=%q want %q", i, got[i], l)
		}
	}
}

func TestLogTemplate_SampleTruncation(t *testing.T) {
	first := "request req-0 completed in 100ms trace=" + strings.Repeat("a", 120)
	lines := []string{
		first,
		"request req-1 completed in 200ms trace=b",
		"request req-2 completed in 300ms trace=c",
	}
	got := foldLogTemplates(lines)
	if len(got) != 1 {
		t.Fatalf("应折成 1 行，实际：%v", got)
	}
	marker := got[0]
	if !strings.Contains(marker, "…") {
		t.Fatalf("超 100 字符的样本应截断加 …，实际：%q", marker)
	}
	// 样本部分（e.g. 之后）不应超过 100 字符 + …。
	idx := strings.Index(marker, "(e.g. ")
	sample := strings.TrimSuffix(marker[idx+len("(e.g. "):], ")")
	if len([]rune(sample)) != 101 { // 100 字符 + …
		t.Fatalf("样本应为 100 字符 + …，实际 %d 字符：%q", len([]rune(sample)), sample)
	}
}

func TestLogTemplate_OrderPreserved(t *testing.T) {
	lines := []string{
		"aaa bbb",
		"request req-1 completed in 100ms",
		"ccc ddd",
		"request req-2 completed in 200ms",
		"request req-3 completed in 300ms",
	}
	got := foldLogTemplates(lines)
	if len(got) != 3 {
		t.Fatalf("应剩 3 行，实际：%v", got)
	}
	if got[0] != "aaa bbb" || got[2] != "ccc ddd" {
		t.Fatalf("折叠行应在首次出现位置保序，实际：%v", got)
	}
	if !strings.HasPrefix(got[1], "[×3] request {} completed in {}") {
		t.Fatalf("中间应为折叠标记，实际：%q", got[1])
	}
}

func TestLogTemplate_NoVariableNoFold(t *testing.T) {
	// 无变量的逐字重复行不归 log-template 管（留给连续重复折叠）。
	lines := []string{"same words here", "same words here", "same words here"}
	got := foldLogTemplates(lines)
	if len(got) != 3 {
		t.Fatalf("无 {} 槽位的模板不应折叠，实际：%v", got)
	}
}

func TestLogTemplateToken(t *testing.T) {
	cases := []struct {
		tok  string
		want string
	}{
		{"12:30:45", "{}"},
		{"2026-06-21T10:30:45", "{}"},
		{"192.168.0.1", "{}"},
		{"3.14", "{}"},
		{"200ms", "{}"},
		{"5s", "{}"},
		{"3.2KB", "{}"},
		{"50%", "{}"},
		{"1.5x", "{}"},
		{"deadbeef1234", "{}"},
		{"550e8400-e29b-41d4-a716-446655440000", "{}"},
		{"req-1", "{}"},
		{"user_123", "{}"},
		{"user=alice", "user={}"},
		{"key=", "key="}, // 值为空，不换
		{"v1.24.0", "v1.24.0"},
		{"1.2.3", "1.2.3"},
		{"ERROR", "ERROR"},
		{"INFO", "INFO"},
		{"hello", "hello"},
	}
	for _, c := range cases {
		if got := logTemplateToken(c.tok); got != c.want {
			t.Errorf("logTemplateToken(%q) = %q, want %q", c.tok, got, c.want)
		}
	}
}
