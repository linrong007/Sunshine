package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSessionStartCtx(t *testing.T, root, source, sessionID string) string {
	t.Helper()
	input := `{"session_id":` + strconv_quot(sessionID) + `,"cwd":` + strconv_quot(root)
	if source != "" {
		input += `,"source":` + strconv_quot(source)
	}
	input += `}`
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunSessionStart(in, o) }, input)
	if strings.TrimSpace(out) == "" {
		t.Fatalf("source=%q 时 session-start 应有输出", source)
	}
	return additionalContext(t, out)
}

func TestCompactResume_OnlyCompact(t *testing.T) {
	root, _ := makeProject(t)
	ctx := runSessionStartCtx(t, root, "compact", "sess-1")
	if !strings.Contains(ctx, "<compact-resume>") {
		t.Fatalf("source=compact 时应追加 <compact-resume> 块，实际：\n%s", ctx)
	}
	if !strings.Contains(ctx, "演示任务") || !strings.Contains(ctx, "下一步") {
		t.Fatalf("指针块应含当前任务与下一步，实际：\n%s", ctx)
	}
	for _, source := range []string{"startup", "resume", "clear", "fork"} {
		ctx := runSessionStartCtx(t, root, source, "sess-1")
		if strings.Contains(ctx, "<compact-resume>") {
			t.Fatalf("source=%s 时不应追加指针块，实际：\n%s", source, ctx)
		}
	}
}

func TestCompactResume_NoSource(t *testing.T) {
	root, _ := makeProject(t)
	ctx := runSessionStartCtx(t, root, "", "sess-1")
	if strings.Contains(ctx, "<compact-resume>") {
		t.Fatalf("取不到 source 时应静默不注，实际：\n%s", ctx)
	}
}

func TestCompactResume_AcceptanceCriteria(t *testing.T) {
	root, taskRef := makeProject(t)
	long := strings.Repeat("长", 120)
	mustWrite(t, filepath.Join(root, taskRef, "prd.md"),
		"# 需求\n做一个演示功能。\n\n## 验收标准\n\n- 第一条：接口响应在 100ms 内\n- 第二条：覆盖全部用例\n- 第三条：文档同步更新\n- 第四条：不应出现在指针里\n- 第五条："+long+"\n")
	ctx := runSessionStartCtx(t, root, "compact", "sess-1")
	for _, want := range []string{"第一条：接口响应在 100ms 内", "第二条：覆盖全部用例", "第三条：文档同步更新"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("指针块应含验收标准 %q，实际：\n%s", want, ctx)
		}
	}
	if strings.Contains(ctx, "第四条") {
		t.Errorf("指针块最多取 3 条验收标准，不应含第四条，实际：\n%s", ctx)
	}
	// 直接验截断：一条 120 字符的验收标准应截到 80。
	mustWrite(t, filepath.Join(root, taskRef, "prd.md"),
		"# 需求\n\n## 验收标准\n\n- "+long+"\n")
	got := compactAcceptanceCriteria(filepath.Join(root, taskRef, "prd.md"))
	if len(got) != 1 || len([]rune(got[0])) != 80 {
		t.Fatalf("单条验收标准应截到 80 字符，实际 %d 字符：%v", len([]rune(got[0])), got)
	}
}

func TestCompactResume_MaxChars(t *testing.T) {
	root, taskRef := makeProject(t)
	mustWrite(t, filepath.Join(root, taskRef, "task.json"),
		`{"id": "11-09-demo", "title": "`+strings.Repeat("长", 5000)+`", "status": "planning"}`+"\n")
	ctx := runSessionStartCtx(t, root, "compact", "sess-1")
	idx := strings.Index(ctx, "<compact-resume>")
	if idx < 0 {
		t.Fatal("应含 <compact-resume> 块")
	}
	block := ctx[idx:]
	if len([]rune(block)) > compactResumeMaxChars {
		t.Fatalf("指针块应 ≤%d 字符，实际 %d 字符", compactResumeMaxChars, len([]rune(block)))
	}
	if !strings.HasSuffix(strings.TrimSpace(block), "</compact-resume>") {
		t.Fatal("超长截断后闭合标签也应在位")
	}
}

func TestCompactResume_DeletesDedupTable(t *testing.T) {
	root, _ := makeProject(t)
	dedupPath := filepath.Join(root, ".sunshine", ".runtime", "dedup-sess-1.jsonl")
	mustWrite(t, dedupPath, "{\"seq\":1,\"tool\":\"Bash\",\"key\":\"git status\",\"hash\":\"abc\"}\n")
	_ = runSessionStartCtx(t, root, "compact", "sess-1")
	if _, err := os.Stat(dedupPath); !os.IsNotExist(err) {
		t.Fatalf("source=compact 时应删本会话去重表，文件仍在：%s", dedupPath)
	}
	// 非 compact 时不删。
	mustWrite(t, dedupPath, "{\"seq\":1,\"tool\":\"Bash\",\"key\":\"git status\",\"hash\":\"abc\"}\n")
	_ = runSessionStartCtx(t, root, "startup", "sess-1")
	if _, err := os.Stat(dedupPath); err != nil {
		t.Fatalf("source=startup 时不应删去重表，err=%v", err)
	}
}

func TestCompactResume_NextStepFromActivity(t *testing.T) {
	root, taskRef := makeProject(t)
	mustWrite(t, filepath.Join(root, taskRef, "task.json"),
		`{"id": "11-09-demo", "title": "演示任务", "status": "in_progress"}`+"\n")
	// 最近事件是 check 停止：下一步应为复核检查结果。
	mustWrite(t, filepath.Join(root, ".sunshine", ".runtime", "activity.jsonl"),
		"{\"ts\":\"2026-11-19T10:00:00Z\",\"kind\":\"subagent_started\",\"agent\":\"implement\",\"task\":\"11-09-demo\",\"source\":\"hook\"}\n"+
			"{\"ts\":\"2026-11-19T10:05:00Z\",\"kind\":\"subagent_stopped\",\"agent\":\"check\",\"task\":\"11-09-demo\",\"source\":\"hook\"}\n")
	ctx := runSessionStartCtx(t, root, "compact", "sess-1")
	if !strings.Contains(ctx, "复核检查结果") {
		t.Fatalf("最近事件为 check 停止时，下一步应为复核检查结果，实际：\n%s", ctx)
	}
	// 最近事件是 implement 启动：下一步应为等待 implement 完成。
	mustWrite(t, filepath.Join(root, ".sunshine", ".runtime", "activity.jsonl"),
		"{\"ts\":\"2026-11-19T10:00:00Z\",\"kind\":\"subagent_started\",\"agent\":\"implement\",\"task\":\"11-09-demo\",\"source\":\"hook\"}\n")
	ctx = runSessionStartCtx(t, root, "compact", "sess-1")
	if !strings.Contains(ctx, "等待 implement 子代理完成") {
		t.Fatalf("最近事件为 implement 启动时，下一步应为等待其完成，实际：\n%s", ctx)
	}
}

func TestCompactResume_NoTask(t *testing.T) {
	root, _ := makeProject(t)
	ctx := runSessionStartCtx(t, root, "compact", "nobody")
	if !strings.Contains(ctx, "<compact-resume>") {
		t.Fatalf("无任务时 compact 也应注指针块，实际：\n%s", ctx)
	}
	if !strings.Contains(ctx, "当前任务：无") {
		t.Fatalf("无任务时指针块应写当前任务无，实际：\n%s", ctx)
	}
}
