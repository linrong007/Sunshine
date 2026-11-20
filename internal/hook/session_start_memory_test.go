package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMemoryIndex 在夹具项目里写 .sunshine/memory/index.md。
func writeMemoryIndex(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".sunshine", "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const memIndexHead = "# 记忆索引\n\n| 标题 | 摘要 | 文件 |\n|------|------|------|\n"

func TestCollectMemoryIndex_Rows(t *testing.T) {
	root := t.TempDir()
	writeMemoryIndex(t, root, memIndexHead+
		"| 优惠券并发坑 | 库存扣减必须走行锁，裸更新会超卖 | 2026-11-01-coupon-lock.md |\n"+
		"| 支付回调幂等 | 回调可能重复投递，按单号去重 | 2026-11-02-pay-callback.md |\n")
	got := collectMemoryIndex(root)
	for _, want := range []string{"优惠券并发坑", "支付回调幂等", "2026-11-01-coupon-lock.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("索引注入缺少 %q\n实际：\n%s", want, got)
		}
	}
}

func TestCollectMemoryIndex_Empty(t *testing.T) {
	root := t.TempDir()
	// 只有模板表头、没有数据行 → 不注入。
	writeMemoryIndex(t, root, memIndexHead)
	if got := collectMemoryIndex(root); got != "" {
		t.Errorf("空索引应返回空串，实际：%q", got)
	}
	// 连文件都没有 → 不注入。
	if got := collectMemoryIndex(t.TempDir()); got != "" {
		t.Errorf("无索引文件应返回空串，实际：%q", got)
	}
}

func TestCollectMemoryIndex_Truncate(t *testing.T) {
	root := t.TempDir()
	var sb strings.Builder
	sb.WriteString(memIndexHead)
	for i := 0; i < 80; i++ {
		sb.WriteString("| 条目" + strings.Repeat("长", 40) + " | " + strings.Repeat("摘要内容", 20) + " | f.md |\n")
	}
	writeMemoryIndex(t, root, sb.String())
	got := collectMemoryIndex(root)
	if !strings.Contains(got, "已截断") {
		t.Errorf("超长索引应带截断注记")
	}
	if len(got) > memoryIndexMaxBytes+512 {
		t.Errorf("截断后长度 %d 超出上限太多", len(got))
	}
}

func TestSessionStart_InjectsMemoryIndex(t *testing.T) {
	root, _ := makeProject(t)
	writeMemoryIndex(t, root, memIndexHead+
		"| 优惠券并发坑 | 库存扣减必须走行锁 | 2026-11-01-coupon-lock.md |\n")
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunSessionStart(in, o) },
		`{"session_id":"sess-1","cwd":`+strconv_quot(root)+`}`)
	ctx := additionalContext(t, out)
	for _, want := range []string{"长期记忆索引", "优惠券并发坑", "以规范为准"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("session-start 上下文缺少 %q", want)
		}
	}
}

func TestSessionStart_NoMemoryNoInject(t *testing.T) {
	root, _ := makeProject(t)
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunSessionStart(in, o) },
		`{"session_id":"sess-1","cwd":`+strconv_quot(root)+`}`)
	ctx := additionalContext(t, out)
	if strings.Contains(ctx, "长期记忆索引") {
		t.Errorf("没有记忆条目时不应注入长期记忆索引节")
	}
}
