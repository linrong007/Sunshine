package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/linrong007/Sunshine/internal/channel/store"
)

// DSH worker 适配器：`dsh --profile headless` 一次一任务，
// --json 事件流解析成频道事件，--session-id 复用会话模拟常驻 worker。
//
// 与 claude/codex 的差别（以 dsh-headless README 为准，
// @deepseek-ai/dsh-headless@0.2.2-rc.1）：
//   - headless 是一次性进程：一个任务跑完就退出，没有交互式 stdin 协议；
//     因此 Send 是异步的——拉起进程即返回，回合结束由进程退出的
//     final 事件驱动（done/error），而不是写 stdin 等回包；
//   - 系统提示词没有 --system-prompt 旗标：首次 Send 时拼进任务文本；
//   - 模型用 profile 默认值，WorkerSpec.Model 对 DSH 无效；
//   - 中断 = 杀掉当前进程（headless README：中断后须等进程退出再续跑
//     同一会话）；替换指令无法写进已死的进程，随下一次 Send 一起发；
//   - --permission-mode workspace-write + --approval-policy never 写死
//     （与 2026-11-17 上午 ~/dsh-e2e 实跑调用一致，无人值守 worker
//     不弹审批）；cwd 下有 .git 时再加 --add-dir <cwd>/.git。

// dshJSONEvent 是 dsh headless --json 流里的一行事件。
// 字段按 dsh-headless 的 json-stream 投影核实：
// session{sessionId,cwd}、status{phase,turn,step,usage?,reason?}、
// thinking{text}、text{text}、tool_call{callId,tool,input}、
// tool_result{callId,status,result,error?}、final{sessionId,reason,text}、
// error{message,code?}。
type dshJSONEvent struct {
	Type      string         `json:"type"`
	SessionID string         `json:"sessionId"`
	Cwd       string         `json:"cwd"`
	Text      string         `json:"text"`
	Phase     string         `json:"phase"`
	Turn      int            `json:"turn"`
	Step      int            `json:"step"`
	CallID    string         `json:"callId"`
	Tool      string         `json:"tool"`
	Input     any            `json:"input"`
	Status    string         `json:"status"`
	Result    string         `json:"result"`
	Error     any            `json:"error"`
	Reason    *dshTurnReason `json:"reason"`
	Usage     any            `json:"usage"`
	Message   string         `json:"message"`
	Code      string         `json:"code"`
}

// dshTurnReason 是 final/status 事件里的回合终态原因。
// 完成时 kind=completed；未完成时 kind 如 error/aborted，
// error 字段带 {code, message}（有则用）。
type dshTurnReason struct {
	Kind  string `json:"kind"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (r *dshTurnReason) message() string {
	if r == nil {
		return "dsh headless 结束但无终态原因"
	}
	kind := r.Kind
	if kind == "" {
		kind = "未知"
	}
	if r.Error != nil && strings.TrimSpace(r.Error.Message) != "" {
		if r.Error.Code != "" {
			return fmt.Sprintf("dsh 任务未完成（%s/%s）：%s", kind, r.Error.Code, r.Error.Message)
		}
		return fmt.Sprintf("dsh 任务未完成（%s）：%s", kind, r.Error.Message)
	}
	return fmt.Sprintf("dsh 任务未完成：%s", kind)
}

// ParseDshLine 解析一行 dsh headless --json 事件，返回频道事件与副作用。
// 纯函数，供 __parse-trace 与单测直接调用。
func ParseDshLine(line string) ParseResult {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ParseResult{}
	}
	var ev dshJSONEvent
	if err := json.Unmarshal([]byte(trimmed), &ev); err != nil {
		return ParseResult{Events: []AdapterEvent{{
			Kind: store.KindError,
			Payload: map[string]any{
				"message":     "解析 DSH 输出行失败",
				"raw_excerpt": firstRunes(trimmed, 200),
			},
		}}}
	}
	switch ev.Type {
	case "session":
		// 开场事件：本次 run 用的会话 id，持久化供下次 --session-id 复用。
		if ev.SessionID == "" {
			return ParseResult{}
		}
		return ParseResult{Side: &SideEffect{PersistSessionID: ev.SessionID}}
	case "text":
		if strings.TrimSpace(ev.Text) == "" {
			return ParseResult{}
		}
		return ParseResult{Events: []AdapterEvent{{
			Kind:    store.KindMessage,
			Payload: map[string]any{"text": ev.Text},
		}}}
	case "thinking":
		// 跳过（只在详细模式才需要，与 claude 的 thinking 块一致）。
		return ParseResult{}
	case "tool_call":
		return ParseResult{Events: []AdapterEvent{{
			Kind: store.KindProgress,
			Payload: map[string]any{"detail": map[string]any{
				"tool":          ev.Tool,
				"input_summary": summarizeInput(ev.Input, 120),
			}},
		}}}
	case "tool_result":
		detail := map[string]any{
			"tool_result":    true,
			"call_id":        ev.CallID,
			"output_summary": summarizeInput(ev.Result, 200),
		}
		if ev.Status == "error" {
			detail["is_error"] = true
		}
		return ParseResult{Events: []AdapterEvent{{
			Kind:    store.KindProgress,
			Payload: map[string]any{"detail": detail},
		}}}
	case "status":
		// 只有 step_end 带用量时才发 progress（与 codex 的
		// thread/tokenUsage/updated 通知同款处理）。
		if ev.Phase == "step_end" && ev.Usage != nil {
			return ParseResult{Events: []AdapterEvent{{
				Kind: store.KindProgress,
				Payload: map[string]any{"detail": map[string]any{
					"kind":  "token_usage",
					"usage": ev.Usage,
				}},
			}}}
		}
		return ParseResult{}
	case "final":
		// 注意：不把 final.text 复制进 done——final 文本已作为 text 事件
		// 发出（与 claude 的 result 行同款处理），再复制会渲染两遍。
		if ev.Reason != nil && ev.Reason.Kind == "completed" {
			return ParseResult{Events: []AdapterEvent{{
				Kind:    store.KindDone,
				Payload: map[string]any{},
			}}}
		}
		return ParseResult{Events: []AdapterEvent{{
			Kind:    store.KindError,
			Payload: map[string]any{"message": ev.Reason.message()},
		}}}
	case "error":
		msg := strings.TrimSpace(ev.Message)
		if msg == "" {
			msg = "dsh headless 报错"
		}
		if ev.Code != "" {
			msg = ev.Code + "：" + msg
		}
		return ParseResult{Events: []AdapterEvent{{
			Kind:    store.KindError,
			Payload: map[string]any{"message": msg},
		}}}
	default:
		return ParseResult{}
	}
}

// dshAdapter 是 DSH 的 Adapter 接口实现：启动参数与逐行解析。
// 没有 stdin 协议：EncodeUserMessage / EncodeInterruptMessage 报错，
// dshRuntime 不经过它们（Send 直接拉起一次 headless 进程）。
type dshAdapter struct{}

func (dshAdapter) Provider() Provider { return ProviderDsh }

// BuildArgs 生成一次 headless 调用的 argv（不含二进制名本身）。
// view.Resume 是已复用的会话 id（无则新开会话）；view.Cwd 用于判断
// 是否加 --add-dir <cwd>/.git。任务文本走 stdin（末尾的 "-"），
// 不占 argv，避免超长提示词触 argv 上限。
func (dshAdapter) BuildArgs(view SupervisorView) []string {
	args := []string{"--profile", "headless", "--json"}
	if strings.TrimSpace(view.Resume) != "" {
		args = append(args, "--session-id", strings.TrimSpace(view.Resume))
	}
	args = append(args, "--permission-mode", "workspace-write", "--approval-policy", "never")
	if strings.TrimSpace(view.Cwd) != "" {
		if gitDir := filepath.Join(view.Cwd, ".git"); dirExists(gitDir) {
			args = append(args, "--add-dir", gitDir)
		}
	}
	return append(args, "-")
}

func (dshAdapter) CreateCtx() any { return nil }

func (dshAdapter) Handshake(ctx context.Context, stdin StdinWriter, c any, view SupervisorView) error {
	return nil
}

func (dshAdapter) IsReady(c any) bool { return true }

func (dshAdapter) ParseLine(line string, c any) ParseResult {
	return ParseDshLine(line)
}

func (dshAdapter) EncodeUserMessage(text string, c any) (string, error) {
	return "", fmt.Errorf("DSH 不支持 stdin 消息编码：每次 Send 是一次 headless 调用")
}

func (dshAdapter) EncodeInterruptMessage(text string, c any) (string, error) {
	return "", fmt.Errorf("DSH 不支持 stdin 中断编码：中断即杀掉当前 headless 进程")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// dshRuntime 是 DSH 的 WorkerRuntime 实现：一次 Send 拉起一个
// headless 进程，异步跑完；进程退出即本回合结束。
// 没有常驻进程：Wait 只在 Stop 时返回；ExitState 只在 Stop 后报已退出。
type dshRuntime struct {
	sink EventSink
	log  io.Writer

	mu       sync.Mutex
	started  bool
	stopped  bool
	done     chan struct{} // Stop 时关闭
	stopOnce sync.Once

	binary       string
	channel      string
	worker       string
	cwd          string
	env          map[string]string
	sessionID    string
	systemPrompt string
	systemSent   bool
	pendingText  string // 中断的替换指令，随下一次 Send 一起发

	cmd         *exec.Cmd     // 当前进行中的 headless 进程
	runDone     chan struct{} // 当前 run 结束时关闭
	killCh      chan struct{} // 置 runKilled 时关闭（等泵排空的短超时信号）
	killClosed  bool          // killCh 是否已关闭
	runKilled   bool          // 当前 run 是被 Interrupt/Stop 杀掉的
	runFinished bool          // 当前 run 已出终态事件（done/error）
}

// NewDshWorkerRuntime 建一个 DSH worker runtime。
func NewDshWorkerRuntime(sink EventSink, log io.Writer) WorkerRuntime {
	return &dshRuntime{sink: sink, log: log}
}

// Start 只记配置、不拉进程：headless 进程在每次 Send 时现拉。
// PID 恒为 0（supervisor 只在 PID>0 时写 worker-pid 文件）。
func (r *dshRuntime) Start(ctx context.Context, opts StartOptions) (StartResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return StartResult{}, fmt.Errorf("worker runtime 已启动")
	}
	r.started = true
	r.done = make(chan struct{})
	r.binary = opts.Binary
	if r.binary == "" {
		r.binary = string(ProviderDsh)
	}
	r.channel = opts.Channel
	r.worker = opts.Worker
	r.cwd = opts.Cwd
	r.env = opts.Env
	r.sessionID = opts.Resume
	r.systemPrompt = opts.SystemPrompt
	return StartResult{Provider: ProviderDsh, PID: 0, StartedAt: time.Now()}, nil
}

// Send 异步拉起一次 headless 进程并立即返回：
// 任务文本经 stdin 传，--json 事件流进 stdout 泵；
// 进程退出时按 final 事件出 done/error，终态由 sink 侧收尾回合。
// 收件箱已保证一次只跑一个回合；重复 Send 直接报错。
func (r *dshRuntime) Send(ctx context.Context, text string) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return fmt.Errorf("worker 尚未启动")
	}
	if r.stopped {
		r.mu.Unlock()
		return fmt.Errorf("worker 已停止")
	}
	if r.cmd != nil {
		r.mu.Unlock()
		return fmt.Errorf("已有进行中的任务，请等它结束")
	}
	// 中断的替换指令随下一次 Send 一起发（进程已死，无法单独写进）。
	if r.pendingText != "" {
		text = r.pendingText + "\n\n" + text
		r.pendingText = ""
	}
	// 系统提示词没有旗标可传：首次 Send 时拼进任务文本。
	if !r.systemSent && strings.TrimSpace(r.systemPrompt) != "" {
		text = r.systemPrompt + "\n\n" + text
		r.systemSent = true
	}
	view := SupervisorView{Resume: r.sessionID, Cwd: r.cwd}
	args := dshAdapter{}.BuildArgs(view)
	binary := r.binary
	channel := r.channel
	worker := r.worker
	cwd := r.cwd
	env := r.env
	r.mu.Unlock()

	cmd := exec.Command(binary, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmdEnv := os.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	// 关掉 worker 内的 hooks，避免 worker 自己的会话再触发注入；
	// 同时把频道身份桥接给子进程（与 processRuntime 一致）。
	cmdEnv = append(cmdEnv,
		"SUNSHINE_HOOKS=0",
		"SUNSHINE_CHANNEL="+channel,
		"SUNSHINE_CHANNEL_AS="+worker,
	)
	cmd.Env = cmdEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("建 worker stdin 管道失败：%w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("建 worker stdout 管道失败：%w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("建 worker stderr 管道失败：%w", err)
	}

	r.logf("[supervisor] 启动 %s：%s %s\n", ProviderDsh, binary, strings.Join(args, " "))
	// 泵在 Start 之前就启动：退得快的进程可能在写完行后立刻退出，
	// 泵晚启动会错过调度间隙、丢掉未读的行。
	pumpDone := make(chan struct{})
	go r.pumpStdout(stdout, pumpDone)
	// stderr 直接进日志文件，不解析。
	go func() {
		_, _ = io.Copy(r.logWriter(), stderr)
	}()
	// 锁内拉起并登记：避免两个并发 Send 同时通过 cmd==nil 检查。
	r.mu.Lock()
	if r.cmd != nil || r.stopped {
		r.mu.Unlock()
		_ = stdout.Close()
		_ = stderr.Close()
		_ = stdin.Close()
		return fmt.Errorf("已有进行中的任务，请等它结束")
	}
	if err := cmd.Start(); err != nil {
		r.mu.Unlock()
		_ = stdout.Close()
		_ = stderr.Close()
		_ = stdin.Close()
		return fmt.Errorf("拉起 %s worker 失败：%w", ProviderDsh, err)
	}
	r.cmd = cmd
	runDone := make(chan struct{})
	r.runDone = runDone
	r.killCh = make(chan struct{})
	r.killClosed = false
	r.runKilled = false
	r.runFinished = false
	r.mu.Unlock()

	// 任务文本写进 stdin 后关闭（headless 读 "-" 即 stdin 任务）。
	go func() {
		_, _ = io.WriteString(stdin, text)
		_ = stdin.Close()
	}()

	// 等待 goroutine：先等 stdout 泵排空（final 事件已落实），再 Wait 收尸——
	// 反过来会让 Wait 关掉管道，丢掉泵还没读到的行。
	// 但泵可能等不到 EOF（子进程的孙子进程继承了 stdout，如 sleep）：
	// 被杀掉的 run 只等 2 秒，正常 run 等 10 秒，超时也继续收尾，
	// 免得 Interrupt/Stop 被一个孤儿进程拖住。
	go func() {
		r.mu.Lock()
		killCh := r.killCh
		r.mu.Unlock()
		select {
		case <-pumpDone:
		case <-killCh:
			// 被杀：再等 2 秒让泵排空已到的行，然后继续收尾。
			select {
			case <-pumpDone:
			case <-time.After(2 * time.Second):
				r.logf("[supervisor] dsh stdout 泵被杀后 2 秒未排空，继续收尾\n")
			}
		case <-time.After(10 * time.Second):
			r.logf("[supervisor] dsh stdout 泵 10 秒未排空，继续收尾\n")
		}
		waitErr := cmd.Wait()
		r.mu.Lock()
		killed := r.runKilled
		finished := r.runFinished
		r.cmd = nil
		close(runDone)
		r.mu.Unlock()
		if !killed && !finished {
			msg := "dsh headless 异常结束：未返回终态事件"
			if waitErr != nil {
				msg = fmt.Sprintf("dsh headless 异常结束：%v", waitErr)
			}
			r.emit(store.KindError, map[string]any{"message": msg})
		}
	}()
	return nil
}

// Interrupt 杀掉当前 headless 进程并等它退出（headless README：
// 中断后须等进程退出再续跑同一会话）。替换指令存下来，
// 随下一次 Send 一起发；无进行中的任务时只存指令。
func (r *dshRuntime) Interrupt(ctx context.Context, text string) error {
	r.mu.Lock()
	if strings.TrimSpace(text) != "" {
		if r.pendingText != "" {
			r.pendingText += "\n\n" + text
		} else {
			r.pendingText = text
		}
	}
	cmd := r.cmd
	runDone := r.runDone
	if cmd == nil || cmd.Process == nil {
		r.mu.Unlock()
		return nil
	}
	r.markKilledLocked()
	proc := cmd.Process
	r.mu.Unlock()
	_ = signalTerm(proc)
	select {
	case <-runDone:
	case <-time.After(ShutdownGrace):
		_ = signalKill(proc)
		<-runDone
	}
	return nil
}

// markKilledLocked 标记当前 run 被杀并关闭 killCh（调用方须持锁，只关一次）。
func (r *dshRuntime) markKilledLocked() {
	r.runKilled = true
	if !r.killClosed && r.killCh != nil {
		close(r.killCh)
		r.killClosed = true
	}
}

// Stop 关停 worker：杀掉进行中的进程（如有），关闭 done，
// Wait 随之返回。反复调用只生效一次。
func (r *dshRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return nil
	}
	r.stopped = true
	cmd := r.cmd
	runDone := r.runDone
	if cmd != nil && cmd.Process != nil {
		r.markKilledLocked()
	}
	done := r.done
	r.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		proc := cmd.Process
		_ = signalTerm(proc)
		select {
		case <-runDone:
		case <-time.After(ShutdownGrace):
			_ = signalKill(proc)
			<-runDone
		}
	}
	if done != nil {
		r.stopOnce.Do(func() { close(done) })
	}
	return nil
}

// Wait 等 worker 停止（Stop 关闭 done 后返回）；
// 进行中的 headless 进程退出不会让 Wait 返回。
func (r *dshRuntime) Wait() error {
	r.mu.Lock()
	done := r.done
	r.mu.Unlock()
	if done == nil {
		return fmt.Errorf("worker 尚未启动")
	}
	<-done
	return nil
}

// Ready 判断：Start 后、Stop 前即就绪（无握手）。
func (r *dshRuntime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started && !r.stopped
}

// ExitState 返回退出状态：只有 Stop 后才报已退出。
func (r *dshRuntime) ExitState() (code int, signal string, exited bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		return -1, "", false
	}
	return 0, "", true
}

// pumpStdout 逐行读 headless 的 --json 事件流，交给适配器解析，
// 再把事件与副作用落实；读完关闭 pumpDone。
func (r *dshRuntime) pumpStdout(stdout io.Reader, pumpDone chan struct{}) {
	defer close(pumpDone)
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) != "" {
			r.handleLine(trimmed)
		}
		if err != nil {
			return
		}
	}
}

// handleLine 处理一行 --json 事件：解析 → 发事件 → 落副作用。
func (r *dshRuntime) handleLine(line string) {
	r.logf("%s\n", line)
	result := ParseDshLine(line)
	r.applyParseResult(result)
}

// applyParseResult 把解析结果落实：事件经 sink 发出；
// 终态（done/error）标记本 run 已结束；会话 id 持久化并记下供下次复用。
func (r *dshRuntime) applyParseResult(result ParseResult) {
	for _, ev := range result.Events {
		r.emit(ev.Kind, ev.Payload)
		if ev.Kind == store.KindDone || ev.Kind == store.KindError {
			r.mu.Lock()
			r.runFinished = true
			r.mu.Unlock()
		}
	}
	if result.Side == nil || r.sink == nil {
		return
	}
	if result.Side.PersistSessionID != "" {
		r.mu.Lock()
		r.sessionID = result.Side.PersistSessionID
		r.mu.Unlock()
		if err := r.sink.PersistSessionID(result.Side.PersistSessionID); err != nil {
			r.logf("[supervisor] 会话 id 持久化失败：%v\n", err)
		}
	}
	if result.Side.PersistThreadID != "" {
		if err := r.sink.PersistThreadID(result.Side.PersistThreadID); err != nil {
			r.logf("[supervisor] 线程 id 持久化失败：%v\n", err)
		}
	}
}

// emit 经 sink 发一个事件；sink 为空时丢弃（测试可不接 sink）。
func (r *dshRuntime) emit(kind store.ChannelEventKind, payload map[string]any) {
	if r.sink == nil {
		return
	}
	if err := r.sink.Emit(kind, r.worker, payload); err != nil {
		r.logf("[supervisor] 事件追加失败：%v\n", err)
	}
}

func (r *dshRuntime) logf(format string, args ...any) {
	w := r.logWriter()
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, format, args...)
}

func (r *dshRuntime) logWriter() io.Writer {
	if r.log == nil {
		return io.Discard
	}
	return r.log
}
