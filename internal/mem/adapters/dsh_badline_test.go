package adapters

import (
	"context"
	"testing"

	"github.com/linrong007/Sunshine/internal/mem"
)

// TestDSHSessions_BadLineKeepsSession 是 D3 回归测试：
// 会话文件里有一行撕裂的坏行（崩溃中途写盘）时，会话仍应出现在
// 列表里，坏行只记警告、不丢整个会话。
func TestDSHSessions_BadLineKeepsSession(t *testing.T) {
	home := newTempHome(t)
	writeDshFixture(t, dshSessionPath(home, "--home-test-proj--", "bad1"),
		`{"type":"session","version":4,"id":"session-bad1","createdAt":1794876185597,"cwd":"/home/test/proj","isSeeded":false,"delegationDepth":0}`,
		`{"type":"user/message","seq":9,"time":1794876190890,"data":{"content":[{"type":"text","text":"hello bad line"}],"id":"m1","role":"user","source":{"kind":"user"}}}`,
		`{"type":"user/message","seq":10,"time":1794876190990,"data":{"content":[{"type":"text","text":"torn`,
		`{"type":"assistant/message","seq":20,"time":1794876193000,"data":{"message":{"content":[{"type":"text","text":"done"}],"id":"m2","role":"assistant","source":{"kind":"model"}}}}`,
	)
	ctx := context.Background()
	sessions, warnings, err := DshAdapter{}.Sessions(ctx, home)
	if err != nil {
		t.Fatalf("Sessions 不应返回错误：%v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("含坏行的会话仍应出现在列表里，会话数 = %d，想要 1（warnings=%v）", len(sessions), warnings)
	}
	if sessions[0].ID != "session-bad1" || sessions[0].Platform != mem.MemSourceDsh {
		t.Errorf("会话 = %+v，想要 id=session-bad1", sessions[0])
	}
	if sessions[0].Title != "hello bad line" {
		t.Errorf("标题应取首条真人消息，实际 %q", sessions[0].Title)
	}
	mustContainWarning(t, warnings, "跳过")
}
