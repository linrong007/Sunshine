package adapters

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/linrong007/Sunshine/internal/mem"
)

// writeDshFixture 把 JSONL 行压成 zstd，写到 DSH 会话文件位置。
func writeDshFixture(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建夹具目录失败：%v", err)
	}
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("建 zstd writer 失败：%v", err)
	}
	for _, line := range lines {
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("写夹具失败：%v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关 zstd writer 失败：%v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("写夹具文件失败：%v", err)
	}
}

func dshSessionPath(home, slug, id string) string {
	return filepath.Join(home, ".dsh", "sessions", slug, "session-"+id, "session.v4.jsonl.zstd")
}

// TestDshSessions：两条会话，一条有 LLM 标题，一条无标题走首条真人消息 fallback。
func TestDshSessions(t *testing.T) {
	home := newTempHome(t)
	writeDshFixture(t, dshSessionPath(home, "--home-test-proj--", "aaa"),
		`{"type":"session","version":4,"id":"session-aaa","createdAt":1794876185597,"cwd":"/home/test/proj","isSeeded":false,"delegationDepth":0}`,
		`{"type":"user/message","seq":9,"time":1794876190890,"data":{"content":[{"type":"text","text":"给 calc 加 subtract"}],"id":"m1","role":"user","source":{"kind":"user"}}}`,
		`{"type":"session/title","seq":14,"time":1794876190899,"data":{"messageSeqs":[9],"source":{"kind":"fallback"},"title":"给 calc 加 subtract"}}`,
		`{"type":"session/title","seq":18,"time":1794876192675,"data":{"messageSeqs":[9],"source":{"kind":"provider"},"title":"Sunshine task for subtract"}}`,
		`{"type":"assistant/message","seq":20,"time":1794876193000,"data":{"message":{"content":[{"type":"text","text":"好的"}],"id":"m2","role":"assistant","source":{"kind":"model"}}}}`,
	)
	writeDshFixture(t, dshSessionPath(home, "--home-test-other--", "bbb"),
		`{"type":"session","version":4,"id":"session-bbb","createdAt":1794876300000,"cwd":"/home/test/other","isSeeded":false,"delegationDepth":0}`,
		`{"type":"user/message","seq":9,"time":1794876301000,"data":{"content":[{"type":"text","text":"这是一个很长的用户问题，用来验证无标题时取首条真人消息做标题 fallback 的截断行为是否正常工作"}],"id":"m3","role":"user","source":{"kind":"user"}}}`,
	)

	ctx := context.Background()
	sessions, warnings, err := DshAdapter{}.Sessions(ctx, home)
	if err != nil {
		t.Fatalf("Sessions 不应返回错误：%v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("不应有警告，实际：%v", warnings)
	}
	if len(sessions) != 2 {
		t.Fatalf("会话数 = %d，想要 2", len(sessions))
	}
	first := sessions[0]
	if first.Platform != mem.MemSourceDsh || first.ID != "session-bbb" || first.CWD != "/home/test/other" {
		t.Errorf("第一条会话 = %+v", first)
	}
	if first.Title != "这是一个很长的用户问题，用来验证无标题时取首条真人消息做标题 fallback 的截断行为是否正常工作" {
		t.Errorf("无标题会话应取首条真人消息做标题，实际 %q", first.Title)
	}
	if first.Created != "2026-11-17T00:45:00.000Z" {
		t.Errorf("Created = %q，想要 2026-11-17T00:45:00.000Z", first.Created)
	}
	if first.Updated != "2026-11-17T00:45:01.000Z" {
		t.Errorf("Updated = %q，想要 2026-11-17T00:45:01.000Z", first.Updated)
	}
	second := sessions[1]
	if second.ID != "session-aaa" || second.Title != "Sunshine task for subtract" {
		t.Errorf("第二条会话 = %+v，想要 id=session-aaa、标题取最后一条 session/title", second)
	}
	if second.Created != "2026-11-17T00:43:05.597Z" {
		t.Errorf("Created = %q，想要 2026-11-17T00:43:05.597Z", second.Created)
	}
}

// TestDshDialogue：注入消息、思考块、工具结果都不进对话；
// 只有真人消息与助手 text 块是轮次。
func TestDshDialogue(t *testing.T) {
	home := newTempHome(t)
	path := dshSessionPath(home, "--home-test-proj--", "aaa")
	writeDshFixture(t, path,
		`{"type":"session","version":4,"id":"session-aaa","createdAt":1794876185597,"cwd":"/home/test/proj","isSeeded":false,"delegationDepth":0}`,
		`{"type":"user/message","seq":9,"time":1794876190890,"data":{"content":[{"type":"text","text":"给 calc 加 subtract"}],"id":"m1","role":"user","source":{"kind":"user"}}}`,
		`{"type":"user/message","seq":10,"time":1794876190891,"data":{"content":[{"type":"text","text":"Current runtime context."}],"id":"m2","role":"user","source":{"kind":"runtime-context","form":"snapshot"}}}`,
		`{"type":"user/message","seq":11,"time":1794876190892,"data":{"content":[{"type":"text","text":"<system-reminder>\nA skill is available.\n</system-reminder>"}],"id":"m3","role":"user","source":{"kind":"skill-catalog","form":"catalog"}}}`,
		`{"type":"system/message","seq":12,"time":1794876190893,"data":{"message":{"content":[{"type":"text","text":"You are an AI agent."}],"role":"system"}}}`,
		`{"type":"assistant/message","seq":20,"time":1794876193000,"data":{"message":{"content":[{"type":"reasoning","text":"先读 skill"},{"type":"text","text":"我先读 skill。"}],"id":"m4","role":"assistant","source":{"kind":"model"}}}}`,
		`{"type":"tool/result","seq":21,"time":1794876193100,"data":{"message":{"content":[{"type":"text","text":"skill 内容"}],"id":"m5","role":"tool","source":{"callId":"c1","kind":"tool"}}}}`,
		`{"type":"assistant/message","seq":22,"time":1794876193200,"data":{"message":{"content":[{"type":"text","text":"做完了。"}],"id":"m6","role":"assistant","source":{"kind":"model"}}}}`,
	)

	ctx := context.Background()
	s := mem.MemSessionInfo{Platform: mem.MemSourceDsh, ID: "session-aaa", FilePath: path}
	turns, warnings, err := DshAdapter{}.Dialogue(ctx, s)
	if err != nil {
		t.Fatalf("Dialogue 不应返回错误：%v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("不应有警告，实际：%v", warnings)
	}
	if len(turns) != 3 {
		t.Fatalf("轮次数 = %d，想要 3：%+v", len(turns), turns)
	}
	if turns[0] != (mem.DialogueTurn{Role: "user", Text: "给 calc 加 subtract"}) {
		t.Errorf("第 0 轮 = %+v", turns[0])
	}
	if turns[1] != (mem.DialogueTurn{Role: "assistant", Text: "我先读 skill。"}) {
		t.Errorf("第 1 轮 = %+v，想要助手 text 块（reasoning 不进对话）", turns[1])
	}
	if turns[2] != (mem.DialogueTurn{Role: "assistant", Text: "做完了。"}) {
		t.Errorf("第 2 轮 = %+v", turns[2])
	}
	for _, turn := range turns {
		if strings.Contains(turn.Text, "runtime context") || strings.Contains(turn.Text, "skill is available") || strings.Contains(turn.Text, "AI agent") {
			t.Errorf("注入文本不应进对话：%q", turn.Text)
		}
	}

	collectTurns, events, _, err := DshAdapter{}.Collect(ctx, s)
	if err != nil {
		t.Fatalf("Collect 不应返回错误：%v", err)
	}
	assertSameTurns(t, collectTurns, turns)
	if len(events) != 0 {
		t.Errorf("本夹具无任务命令，事件应为空，实际 %+v", events)
	}
}

// TestDshCollectEvents：bash tool/call 里的 create/start 抽成事件；
// 含 sunshine task 字样的 grep 不算；turnIndex 是调用时已入池的轮次数。
func TestDshCollectEvents(t *testing.T) {
	home := newTempHome(t)
	path := dshSessionPath(home, "--home-test-proj--", "aaa")
	writeDshFixture(t, path,
		`{"type":"session","version":4,"id":"session-aaa","createdAt":1794876185597,"cwd":"/home/test/proj","isSeeded":false,"delegationDepth":0}`,
		`{"type":"user/message","seq":9,"time":1794876190890,"data":{"content":[{"type":"text","text":"加 subtract"}],"id":"m1","role":"user","source":{"kind":"user"}}}`,
		`{"type":"assistant/message","seq":20,"time":1794876193000,"data":{"message":{"content":[{"type":"text","text":"开始。"}],"id":"m2","role":"assistant","source":{"kind":"model"}}}}`,
		`{"type":"tool/call","seq":26,"time":1794876195412,"data":{"arguments":"{\"command\": \"export PATH=\\\"/x/bin:$PATH\\\"; sunshine task create \\\"为 calc 添加 subtract\\\" --slug calc-subtract --description \\\"d\\\"\"}","callId":"c1","name":"bash","step":2,"turn":1}}`,
		`{"type":"tool/call","seq":30,"time":1794876195500,"data":{"arguments":"{\"command\": \"grep -rn \\\"sunshine task\\\" .dsh --include=*.md\"}","callId":"c2","name":"bash","step":2,"turn":1}}`,
		`{"type":"assistant/message","seq":40,"time":1794876196000,"data":{"message":{"content":[{"type":"text","text":"任务已建。"}],"id":"m3","role":"assistant","source":{"kind":"model"}}}}`,
		`{"type":"tool/call","seq":46,"time":1794876245249,"data":{"arguments":"{\"command\": \"sunshine task start .sunshine/tasks/11-17-calc-subtract 2>&1\"}","callId":"c3","name":"bash","step":3,"turn":1}}`,
	)

	ctx := context.Background()
	s := mem.MemSessionInfo{Platform: mem.MemSourceDsh, ID: "session-aaa", FilePath: path}
	turns, events, _, err := DshAdapter{}.Collect(ctx, s)
	if err != nil {
		t.Fatalf("Collect 不应返回错误：%v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("轮次数 = %d，想要 3", len(turns))
	}
	if len(events) != 2 {
		t.Fatalf("事件数 = %d，想要 2（grep 那条不算）：%+v", len(events), events)
	}
	assertEvent(t, events[0], "create", 2, "calc-subtract", "", "2026-11-17T00:43:15.412Z")
	assertEvent(t, events[1], "start", 3, "", ".sunshine/tasks/11-17-calc-subtract", "2026-11-17T00:44:05.249Z")
}

// TestDshCorruptFile：坏的 zstd 文件在 Sessions 里跳过、Dialogue 里给警告，均不报错。
func TestDshCorruptFile(t *testing.T) {
	home := newTempHome(t)
	path := dshSessionPath(home, "--home-test-proj--", "aaa")
	writeFixture(t, path, "这不是 zstd 数据")

	ctx := context.Background()
	sessions, _, err := DshAdapter{}.Sessions(ctx, home)
	if err != nil {
		t.Fatalf("Sessions 不应返回错误：%v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("坏文件应被跳过，会话数 = %d", len(sessions))
	}
	s := mem.MemSessionInfo{Platform: mem.MemSourceDsh, ID: "session-aaa", FilePath: path}
	turns, warnings, err := DshAdapter{}.Dialogue(ctx, s)
	if err != nil {
		t.Fatalf("Dialogue 不应返回错误：%v", err)
	}
	if len(turns) != 0 {
		t.Errorf("坏文件对话应为空，实际 %d 轮", len(turns))
	}
	// zstd 解码是惰性的：垃圾输入在首读才报错，落到“跳过行”警告。
	mustContainWarning(t, warnings, "跳过")
}

// TestDshNoSessionHeader：能解压但无 session 头的文件跳过。
func TestDshNoSessionHeader(t *testing.T) {
	home := newTempHome(t)
	writeDshFixture(t, dshSessionPath(home, "--home-test-proj--", "aaa"),
		`{"type":"user/message","seq":9,"time":1794876190890,"data":{"content":[{"type":"text","text":"hi"}],"id":"m1","role":"user","source":{"kind":"user"}}}`,
	)

	ctx := context.Background()
	sessions, _, err := DshAdapter{}.Sessions(ctx, home)
	if err != nil {
		t.Fatalf("Sessions 不应返回错误：%v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("无 session 头的文件应被跳过，会话数 = %d", len(sessions))
	}
}
