package adapters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linrong007/Sunshine/internal/channel/store"
)

func TestParseProviderDsh(t *testing.T) {
	p, err := ParseProvider("dsh")
	if err != nil || p != ProviderDsh {
		t.Fatalf("ParseProvider(dsh) = (%q, %v)，想要 (dsh, nil)", p, err)
	}
	if !IsProvider("dsh") {
		t.Error("IsProvider(dsh) 应为 true")
	}
	found := false
	for _, p := range ListProviders() {
		if p == ProviderDsh {
			found = true
		}
	}
	if !found {
		t.Error("ListProviders 应包含 dsh")
	}
	if _, err := GetAdapter(ProviderDsh); err != nil {
		t.Fatalf("GetAdapter(dsh) 失败：%v", err)
	}
}

func TestParseDshLine(t *testing.T) {
	t.Run("session 事件持久化会话 id", func(t *testing.T) {
		res := ParseDshLine(`{"type":"session","sessionId":"session-abc","cwd":"/tmp/proj"}`)
		if len(res.Events) != 0 || res.Side == nil || res.Side.PersistSessionID != "session-abc" {
			t.Fatalf("应只出副作用 PersistSessionID=session-abc，实际 %+v", res)
		}
	})
	t.Run("text 事件转为 message", func(t *testing.T) {
		res := ParseDshLine(`{"type":"text","text":"做完了。"}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindMessage || res.Events[0].Payload["text"] != "做完了。" {
			t.Fatalf("应出 message 事件，实际 %+v", res)
		}
	})
	t.Run("thinking 跳过", func(t *testing.T) {
		if res := ParseDshLine(`{"type":"thinking","text":"想想"}`); len(res.Events) != 0 {
			t.Fatalf("thinking 应跳过，实际 %+v", res)
		}
	})
	t.Run("tool_call 转 progress", func(t *testing.T) {
		res := ParseDshLine(`{"type":"tool_call","callId":"c1","tool":"bash","input":{"command":"node calc.test.js"}}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindProgress {
			t.Fatalf("应出 progress 事件，实际 %+v", res)
		}
		detail, _ := res.Events[0].Payload["detail"].(map[string]any)
		if detail["tool"] != "bash" {
			t.Fatalf("detail.tool = %v，想要 bash", detail["tool"])
		}
	})
	t.Run("tool_result 完成与报错都转 progress 摘要", func(t *testing.T) {
		res := ParseDshLine(`{"type":"tool_result","callId":"c1","status":"completed","result":"calc test ok"}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindProgress {
			t.Fatalf("应出 progress 事件，实际 %+v", res)
		}
		detail, _ := res.Events[0].Payload["detail"].(map[string]any)
		if detail["is_error"] != nil {
			t.Fatalf("完成的结果不应带 is_error，实际 %+v", detail)
		}
		res = ParseDshLine(`{"type":"tool_result","callId":"c2","status":"error","result":"exit 1","error":{"name":"x","code":"E1"}}`)
		detail, _ = res.Events[0].Payload["detail"].(map[string]any)
		if detail["is_error"] != true {
			t.Fatalf("报错的结果应带 is_error=true，实际 %+v", detail)
		}
	})
	t.Run("step_end 用量转 token_usage progress", func(t *testing.T) {
		res := ParseDshLine(`{"type":"status","phase":"step_end","turn":1,"step":2,"usage":{"input_tokens":10}}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindProgress {
			t.Fatalf("应用量 progress 事件，实际 %+v", res)
		}
		detail, _ := res.Events[0].Payload["detail"].(map[string]any)
		if detail["kind"] != "token_usage" {
			t.Fatalf("detail.kind = %v，想要 token_usage", detail["kind"])
		}
		if res := ParseDshLine(`{"type":"status","phase":"turn_start","turn":1}`); len(res.Events) != 0 {
			t.Fatalf("turn_start 应跳过，实际 %+v", res)
		}
	})
	t.Run("final 完成转 done，不复制文本", func(t *testing.T) {
		res := ParseDshLine(`{"type":"final","sessionId":"s1","reason":{"kind":"completed"},"text":"答案"}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindDone {
			t.Fatalf("应出 done 事件，实际 %+v", res)
		}
		if _, ok := res.Events[0].Payload["text"]; ok {
			t.Fatalf("done 不应复制 final 文本，实际 %+v", res.Events[0].Payload)
		}
	})
	t.Run("final 未完成转 error，带原因", func(t *testing.T) {
		res := ParseDshLine(`{"type":"final","sessionId":"s1","reason":{"kind":"error","error":{"code":"E2","message":"模型报错"}},"text":""}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindError {
			t.Fatalf("应出 error 事件，实际 %+v", res)
		}
		msg, _ := res.Events[0].Payload["message"].(string)
		if !strings.Contains(msg, "error") || !strings.Contains(msg, "模型报错") {
			t.Fatalf("error 消息应含 kind 与原因，实际 %q", msg)
		}
		res = ParseDshLine(`{"type":"final","sessionId":"s1","reason":null,"text":""}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindError {
			t.Fatalf("无原因的 final 也应出 error，实际 %+v", res)
		}
	})
	t.Run("error 事件转 error", func(t *testing.T) {
		res := ParseDshLine(`{"type":"error","code":"E3","message":"用法错误"}`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindError {
			t.Fatalf("应出 error 事件，实际 %+v", res)
		}
		if msg, _ := res.Events[0].Payload["message"].(string); !strings.Contains(msg, "E3") || !strings.Contains(msg, "用法错误") {
			t.Fatalf("error 消息应含 code 与 message，实际 %q", msg)
		}
	})
	t.Run("未知类型与空行跳过，非 JSON 报 error", func(t *testing.T) {
		if res := ParseDshLine(`{"type":"bogus"}`); len(res.Events) != 0 {
			t.Fatalf("未知类型应跳过，实际 %+v", res)
		}
		if res := ParseDshLine("   "); len(res.Events) != 0 {
			t.Fatalf("空行应跳过，实际 %+v", res)
		}
		res := ParseDshLine(`不是 JSON`)
		if len(res.Events) != 1 || res.Events[0].Kind != store.KindError {
			t.Fatalf("非 JSON 行应出 error，实际 %+v", res)
		}
	})
}

func TestDshAdapterBuildArgs(t *testing.T) {
	a := dshAdapter{}
	t.Run("默认参数", func(t *testing.T) {
		args := a.BuildArgs(SupervisorView{})
		want := []string{"--profile", "headless", "--json", "--permission-mode", "workspace-write", "--approval-policy", "never", "-"}
		if len(args) != len(want) {
			t.Fatalf("参数 = %v，想要 %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("参数 = %v，想要 %v", args, want)
			}
		}
	})
	t.Run("带 resume 会话", func(t *testing.T) {
		args := a.BuildArgs(SupervisorView{Resume: "session-xyz"})
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--session-id session-xyz") {
			t.Fatalf("应含 --session-id session-xyz，实际 %v", args)
		}
	})
	t.Run("cwd 下有 .git 时加 --add-dir", func(t *testing.T) {
		cwd := t.TempDir()
		if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		args := a.BuildArgs(SupervisorView{Cwd: cwd})
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--add-dir "+filepath.Join(cwd, ".git")) {
			t.Fatalf("应含 --add-dir <cwd>/.git，实际 %v", args)
		}
		if args := a.BuildArgs(SupervisorView{Cwd: t.TempDir()}); strings.Contains(strings.Join(args, " "), "--add-dir") {
			t.Fatalf("无 .git 时不应加 --add-dir，实际 %v", args)
		}
	})
	t.Run("EncodeUserMessage 与 EncodeInterruptMessage 报错", func(t *testing.T) {
		if _, err := a.EncodeUserMessage("hi", nil); err == nil {
			t.Error("DSH 无 stdin 协议，EncodeUserMessage 应报错")
		}
		if _, err := a.EncodeInterruptMessage("hi", nil); err == nil {
			t.Error("DSH 无 stdin 协议，EncodeInterruptMessage 应报错")
		}
		if !a.IsReady(nil) {
			t.Error("DSH 无握手，应直接就绪")
		}
	})
}

// fakeDshScript 是一个模拟 dsh headless 二进制的 shell 脚本：
// 把 argv 与 stdin 分别追加记日志，再输出一串 canned --json 事件并退出 0。
// FAKE_DSH_LOG 与 FAKE_DSH_STDIN 由调用方经环境变量传入。
const fakeDshScript = `#!/bin/sh
echo "$@" >> "$FAKE_DSH_LOG"
echo "---run---" >> "$FAKE_DSH_STDIN"
cat >> "$FAKE_DSH_STDIN"
echo '{"type":"session","sessionId":"stub-session-1","cwd":"/tmp"}'
echo '{"type":"text","text":"收到任务。"}'
echo '{"type":"tool_call","callId":"c1","tool":"bash","input":{"command":"node calc.test.js"}}'
echo '{"type":"tool_result","callId":"c1","status":"completed","result":"calc test ok"}'
echo '{"type":"final","sessionId":"stub-session-1","reason":{"kind":"completed"},"text":"收到任务。"}'
`

// fakeDshSleepScript 只发 session 事件然后睡 30 秒（供中断测试）。
const fakeDshSleepScript = `#!/bin/sh
echo "$@" >> "$FAKE_DSH_LOG"
echo "---run---" >> "$FAKE_DSH_STDIN"
cat >> "$FAKE_DSH_STDIN"
echo '{"type":"session","sessionId":"stub-session-1","cwd":"/tmp"}'
sleep 30
`

// fakeDshNoFinalScript 只发一条 text 就以退出码 1 结束（供无终态测试）。
const fakeDshNoFinalScript = `#!/bin/sh
echo '{"type":"text","text":"做到一半。"}'
exit 1
`

func writeFakeDsh(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-dsh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("写模拟二进制失败：%v", err)
	}
	return path
}

func TestDshRuntimeFlow(t *testing.T) {
	sink := &recordingSink{}
	rt := NewDshWorkerRuntime(sink, nil)
	bin := writeFakeDsh(t, fakeDshScript)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	argvLog := filepath.Join(t.TempDir(), "argv.log")
	stdinLog := filepath.Join(t.TempDir(), "stdin.log")
	ctx := context.Background()
	res, err := rt.Start(ctx, StartOptions{
		Provider: ProviderDsh, Binary: bin, Worker: "implement",
		Channel: "demo", Cwd: cwd, SystemPrompt: "你是 worker。",
		Env: map[string]string{"FAKE_DSH_LOG": argvLog, "FAKE_DSH_STDIN": stdinLog},
	})
	if err != nil {
		t.Fatalf("Start 失败：%v", err)
	}
	if res.PID != 0 {
		t.Fatalf("DSH 无常驻进程，PID 应为 0，实际 %+v", res)
	}
	if res.Provider != ProviderDsh {
		t.Fatalf("Provider 应为 dsh，实际 %+v", res)
	}
	if !rt.Ready() {
		t.Fatal("DSH Start 后应就绪")
	}

	if err := rt.Send(ctx, "做任务"); err != nil {
		t.Fatalf("Send 失败：%v", err)
	}
	sink.waitKind(t, store.KindDone, 5*time.Second)
	if sink.sessionID != "stub-session-1" {
		t.Fatalf("应持久化 session_id，实际 %q", sink.sessionID)
	}
	kinds := sink.kinds()
	wantKinds := []store.ChannelEventKind{store.KindMessage, store.KindProgress, store.KindProgress, store.KindDone}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("事件 = %v，想要 %v", kinds, wantKinds)
	}
	for i := range wantKinds {
		if kinds[i] != wantKinds[i] {
			t.Fatalf("事件 = %v，想要 %v", kinds, wantKinds)
		}
	}
	argvLines := readLines(t, argvLog)
	if len(argvLines) != 1 {
		t.Fatalf("应调用一次 fake dsh，实际 %d 次：%v", len(argvLines), argvLines)
	}
	first := argvLines[0]
	for _, want := range []string{"--profile headless", "--json", "--permission-mode workspace-write", "--approval-policy never", "--add-dir " + filepath.Join(cwd, ".git")} {
		if !strings.Contains(first, want) {
			t.Errorf("首次调用 argv 应含 %q，实际 %q", want, first)
		}
	}
	if strings.Contains(first, "--session-id") {
		t.Errorf("首次调用不应带 --session-id（新会话），实际 %q", first)
	}
	stdin := readFile(t, stdinLog)
	if !strings.Contains(stdin, "你是 worker。") || !strings.Contains(stdin, "做任务") {
		t.Errorf("首次任务应含系统提示词 + 任务文本，实际 %q", stdin)
	}

	// 第二次 Send：复用会话 id，系统提示词不再重复拼。
	if err := rt.Send(ctx, "继续做"); err != nil {
		t.Fatalf("第二次 Send 失败：%v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(sink.kinds()) >= 8 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(sink.kinds()) != 8 {
		t.Fatalf("两次 run 应共 8 个事件，实际 %v", sink.kinds())
	}
	argvLines = readLines(t, argvLog)
	if len(argvLines) != 2 {
		t.Fatalf("应调用两次 fake dsh，实际 %d 次", len(argvLines))
	}
	if !strings.Contains(argvLines[1], "--session-id stub-session-1") {
		t.Errorf("第二次调用应带 --session-id stub-session-1，实际 %q", argvLines[1])
	}
	stdin = readFile(t, stdinLog)
	if strings.Count(stdin, "你是 worker。") != 1 {
		t.Errorf("系统提示词只应拼一次，实际 stdin 日志：%q", stdin)
	}
	if !strings.Contains(stdin, "继续做") {
		t.Errorf("第二次任务文本应进 stdin，实际 %q", stdin)
	}

	if err := rt.Stop(ctx); err != nil {
		t.Fatalf("Stop 失败：%v", err)
	}
	if err := rt.Wait(); err != nil {
		t.Fatalf("Wait 应返回 nil，实际 %v", err)
	}
	code, _, exited := rt.ExitState()
	if !exited || code != 0 {
		t.Fatalf("退出状态应为 code=0，实际 code=%d exited=%v", code, exited)
	}
	if rt.Ready() {
		t.Fatal("Stop 后不应就绪")
	}
	if err := rt.Send(ctx, "再来"); err == nil {
		t.Error("Stop 后 Send 应报错")
	}
}

func TestDshRuntimeNoFinalEmitsError(t *testing.T) {
	sink := &recordingSink{}
	rt := NewDshWorkerRuntime(sink, nil)
	bin := writeFakeDsh(t, fakeDshNoFinalScript)
	ctx := context.Background()
	if _, err := rt.Start(ctx, StartOptions{
		Provider: ProviderDsh, Binary: bin, Worker: "w", Cwd: t.TempDir(),
	}); err != nil {
		t.Fatalf("Start 失败：%v", err)
	}
	if err := rt.Send(ctx, "做任务"); err != nil {
		t.Fatalf("Send 失败：%v", err)
	}
	sink.waitKind(t, store.KindError, 5*time.Second)
	kinds := sink.kinds()
	if len(kinds) != 2 || kinds[0] != store.KindMessage || kinds[1] != store.KindError {
		t.Fatalf("事件应为 message → error，实际 %v", kinds)
	}
	msg, _ := sink.events[1].Payload["message"].(string)
	if !strings.Contains(msg, "异常结束") {
		t.Fatalf("error 消息应说明无终态，实际 %q", msg)
	}
	_ = rt.Stop(ctx)
}

func TestDshRuntimeInterruptKillsRun(t *testing.T) {
	sink := &recordingSink{}
	rt := NewDshWorkerRuntime(sink, nil)
	bin := writeFakeDsh(t, fakeDshSleepScript)
	argvLog := filepath.Join(t.TempDir(), "argv.log")
	stdinLog := filepath.Join(t.TempDir(), "stdin.log")
	ctx := context.Background()
	if _, err := rt.Start(ctx, StartOptions{
		Provider: ProviderDsh, Binary: bin, Worker: "w", Cwd: t.TempDir(),
		Env: map[string]string{"FAKE_DSH_LOG": argvLog, "FAKE_DSH_STDIN": stdinLog},
	}); err != nil {
		t.Fatalf("Start 失败：%v", err)
	}
	if err := rt.Send(ctx, "慢慢做"); err != nil {
		t.Fatalf("Send 失败：%v", err)
	}
	time.Sleep(300 * time.Millisecond) // 等进程跑起来
	start := time.Now()
	if err := rt.Interrupt(ctx, "改一下"); err != nil {
		t.Fatalf("Interrupt 失败：%v", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("Interrupt 应在宽限内杀掉进程，实际耗时 %v", took)
	}
	// 中断后能立即再 Send（run 已清掉），且替换指令随下一次 Send 一起发。
	if err := rt.Send(ctx, "新任务"); err != nil {
		t.Fatalf("中断后 Send 失败：%v", err)
	}
	time.Sleep(300 * time.Millisecond)
	stdin := readFile(t, stdinLog)
	if !strings.Contains(stdin, "改一下") || !strings.Contains(stdin, "新任务") {
		t.Errorf("替换指令应随下一次 Send 一起发，实际 stdin 日志：%q", stdin)
	}
	// 被中断的 run 不应补 error 事件（watcher 已写 interrupted）。
	for _, k := range sink.kinds() {
		if k == store.KindError {
			t.Errorf("被中断的 run 不应补 error 事件，实际 %v", sink.kinds())
		}
	}
	_ = rt.Stop(ctx)
}

func TestDshRuntimeMissingBinary(t *testing.T) {
	sink := &recordingSink{}
	rt := NewDshWorkerRuntime(sink, nil)
	ctx := context.Background()
	// DSH 在 Start 时不拉进程，故 Start 成功；失败在 Send 时才暴露。
	if _, err := rt.Start(ctx, StartOptions{
		Provider: ProviderDsh, Binary: "/nonexistent/dsh", Worker: "w", Cwd: t.TempDir(),
	}); err != nil {
		t.Fatalf("Start 不应失败（进程在 Send 时才拉）：%v", err)
	}
	if err := rt.Send(ctx, "做任务"); err == nil {
		t.Fatal("二进制缺失时 Send 应报错")
	}
	_ = rt.Stop(ctx)
}

func TestDshRuntimeDoubleStart(t *testing.T) {
	rt := NewDshWorkerRuntime(&recordingSink{}, nil)
	ctx := context.Background()
	opts := StartOptions{Provider: ProviderDsh, Binary: "dsh", Worker: "w", Cwd: t.TempDir()}
	if _, err := rt.Start(ctx, opts); err != nil {
		t.Fatalf("首次 Start 失败：%v", err)
	}
	if _, err := rt.Start(ctx, opts); err == nil {
		t.Error("重复 Start 应报错")
	}
	_ = rt.Stop(ctx)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data := readFile(t, path)
	var out []string
	for _, line := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件 %s 失败：%v", path, err)
	}
	return string(data)
}
