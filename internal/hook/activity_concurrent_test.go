package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestAppendActivityEvent_ConcurrentNoLoss 是 D1 回归测试：
// 预置 999 行接近上限，200 个 goroutine 并发追加，截断的 rename
// 不应吞掉窗口期内其他 goroutine 的追加。最终保留的最近 1000 行
// 里必须包含全部 200 条并发事件（被挤掉的只能是预置的旧行）。
func TestAppendActivityEvent_ConcurrentNoLoss(t *testing.T) {
	root := t.TempDir()
	path := ActivityFilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for i := 0; i < 999; i++ {
		fmt.Fprintf(&sb, "{\"ts\":\"2026-11-20T00:00:00Z\",\"kind\":\"subagent_started\",\"agent\":\"implement\",\"task\":\"preset\",\"session_id\":\"preset-%d\",\"source\":\"hook\"}\n", i)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			AppendActivityEvent(root, ActivityEvent{
				Kind:      ActivityKindStopped,
				Agent:     "implement",
				Task:      "race-task",
				SessionID: fmt.Sprintf("race-%d", i),
			})
		}(i)
	}
	wg.Wait()
	events := readActivity(t, root)
	if len(events) != MaxActivityLines {
		t.Fatalf("截断后应保留 %d 行，实际 %d", MaxActivityLines, len(events))
	}
	seen := make(map[string]bool)
	for _, ev := range events {
		if strings.HasPrefix(ev.SessionID, "race-") {
			seen[ev.SessionID] = true
		}
	}
	if len(seen) != n {
		var missing []string
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("race-%d", i)
			if !seen[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) > 10 {
			missing = missing[:10]
		}
		t.Fatalf("并发事件应全部保留：期望 %d 条 race 事件，实际 %d 条，丢失示例 %v", n, len(seen), missing)
	}
}
