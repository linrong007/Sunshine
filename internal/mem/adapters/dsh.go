package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/linrong007/Sunshine/internal/mem"
)

// DSH 会话布局：~/.dsh/sessions/--<项目路径，/ 换 ->--/
// session-<uuid>/session.v4.jsonl.zstd（zstd 压的 JSONL v4，
// 2026-11-17 上午 ~/dsh-e2e 两次 headless 实跑会话核实）。
//
// 条目形态（v4）：
//   - 首行 type=session 的头：id、cwd、createdAt（毫秒时间戳）、version=4；
//   - user/message：data 直挂 content/role/source，source.kind=user 的
//     才是真人消息（runtime-context 快照、skill-catalog、
//     agent-instructions 都是注入，跳过）；
//   - assistant/message：data.message.content 里只取 type=text 的块，
//     reasoning（思考）与 tool-call 不进对话；
//   - system/message 与 tool/result 不进对话；
//   - tool/call：data.name=bash 时 arguments（JSON 字符串）里的 command
//     含 `sunshine task create|start`，抽成任务命令事件；
//   - session/title：data.title，取最后一条（LLM 标题优先于 fallback 截断）；
//   - 其余条目（step/turn/job/todo/request/session-log 等）忽略。
// 子代理会话（delegationDepth>0）与主会话同等列出，头里无父会话链接，
// 故不建 parent_id 关联。

func dshSessions(ctx context.Context, home string) ([]mem.MemSessionInfo, []string, error) {
	files := dshSessionFiles(home)
	if len(files) == 0 {
		return nil, []string{fmt.Sprintf("未找到 DSH 会话存储（%s），已跳过", DshSessionsDir(home))}, nil
	}
	var out []mem.MemSessionInfo
	for _, filePath := range files {
		if ctx.Err() != nil {
			return out, nil, ctx.Err()
		}
		info, warnings := dshSessionMeta(filePath)
		if len(warnings) > 0 {
			// 解不开的文件只警告、跳过，不让一个坏文件挡住整批。
			continue
		}
		if info == nil {
			continue
		}
		out = append(out, *info)
	}
	return out, nil, nil
}

// dshSessionFiles 列出 DSH 会话根下的全部 session.v4.jsonl.zstd（排序）。
func dshSessionFiles(home string) []string {
	root := DshSessionsDir(home)
	if !dirExists(root) {
		return nil
	}
	var out []string
	for _, file := range WalkDir(root) {
		if filepath.Base(file) == "session.v4.jsonl.zstd" {
			out = append(out, file)
		}
	}
	return out
}

// dshSessionMeta 一次流式遍历取会话元数据：头里的 id/cwd/createdAt、
// 最后一条 session/title、最大 time、首条真人消息（标题 fallback）。
// 解压或解析失败时返回警告（调用方跳过该文件）。
func dshSessionMeta(filePath string) (*mem.MemSessionInfo, []string) {
	var header map[string]any
	var title string
	var firstUserText string
	var maxTimeMs *int64
	warnings := dshReadSession(filePath, func(entry map[string]any) bool {
		if ms := dshEntryTimeMs(entry); ms != nil && (maxTimeMs == nil || *ms > *maxTimeMs) {
			maxTimeMs = ms
		}
		switch objStr(entry, "type") {
		case "session":
			if header == nil {
				header = entry
			}
		case "session/title":
			if t := strings.TrimSpace(objStr(objMap(entry, "data"), "title")); t != "" {
				title = t
			}
		case "user/message":
			if firstUserText == "" {
				if turn := dshUserTurn(entry); turn != nil {
					firstUserText = turn.Text
				}
			}
		}
		return true
	})
	if len(warnings) > 0 {
		return nil, warnings
	}
	if header == nil {
		return nil, []string{fmt.Sprintf("文件 %s：无 session 头，已跳过", filePath)}
	}
	id := objStr(header, "id")
	if id == "" {
		id = filepath.Base(filepath.Dir(filePath))
	}
	if title == "" {
		title = dshTruncateRunes(firstUserText, 60)
	}
	created := ""
	if ms := dshNumberMs(header["createdAt"]); ms != nil {
		created = formatISOTime(time.UnixMilli(*ms))
	}
	updated := ""
	if maxTimeMs != nil {
		updated = formatISOTime(time.UnixMilli(*maxTimeMs))
	} else if info, err := os.Stat(filePath); err == nil {
		updated = formatISOTime(info.ModTime())
	}
	return &mem.MemSessionInfo{
		Platform: mem.MemSourceDsh,
		ID:       id,
		Title:    title,
		CWD:      objStr(header, "cwd"),
		Created:  created,
		Updated:  updated,
		FilePath: filePath,
	}, nil
}

func dshDialogue(ctx context.Context, s mem.MemSessionInfo) ([]mem.DialogueTurn, []string, error) {
	turns, _, warnings, err := dshCollect(ctx, s)
	return turns, warnings, err
}

// dshCollect 一次遍历同时产出清理后的对话轮次与任务命令事件：
// 条目按文件顺序走，事件的 turnIndex 是该 tool/call 出现时已入池的轮次数。
func dshCollect(ctx context.Context, s mem.MemSessionInfo) ([]mem.DialogueTurn, []mem.MemEvent, []string, error) {
	var turns []mem.DialogueTurn
	var events []mem.MemEvent
	warnings := dshReadSession(s.FilePath, func(entry map[string]any) bool {
		if ctx.Err() != nil {
			return false
		}
		switch objStr(entry, "type") {
		case "user/message":
			if turn := dshUserTurn(entry); turn != nil {
				turns = append(turns, *turn)
			}
		case "assistant/message":
			if turn := dshAssistantTurn(entry); turn != nil {
				turns = append(turns, *turn)
			}
		case "tool/call":
			events = append(events, dshTaskEvents(entry, len(turns))...)
		}
		return true
	})
	return turns, events, warnings, nil
}

// dshUserTurn 取 user/message 的轮次：只有 source.kind=user 的真人消息才算；
// 注入（runtime-context / skill-catalog / agent-instructions）跳过。
// 文本走注入标签剥离与启动注入识别，与 piBuildTurn 同规则。
func dshUserTurn(entry map[string]any) *mem.DialogueTurn {
	data := objMap(entry, "data")
	if data == nil {
		return nil
	}
	if source := objMap(data, "source"); source != nil && objStr(source, "kind") != "user" {
		return nil
	}
	return dshBuildTurn("user", objList(data, "content"))
}

// dshAssistantTurn 取 assistant/message 的轮次：只取 text 块，
// reasoning 与 tool-call 不进对话。
func dshAssistantTurn(entry map[string]any) *mem.DialogueTurn {
	msg := objMap(objMap(entry, "data"), "message")
	if msg == nil {
		return nil
	}
	return dshBuildTurn("assistant", objList(msg, "content"))
}

// dshBuildTurn 从内容块数组里取文本：只取 type 为 text 的块，
// 注入标签剥离，启动注入轮次丢弃。
func dshBuildTurn(role string, blocks []any) *mem.DialogueTurn {
	var parts []string
	totalRaw := 0
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		if !ok || objStr(block, "type") != "text" {
			continue
		}
		text, ok := block["text"].(string)
		if !ok {
			continue
		}
		totalRaw += len(text)
		if cleaned := stripInjectionTags(text); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	merged := strings.Join(parts, "\n\n")
	if isBootstrapTurn(merged, totalRaw) {
		return nil
	}
	return &mem.DialogueTurn{Role: role, Text: merged}
}

// dshTaskEvents 从一条 tool/call 里抽任务命令事件：只看 name=bash 的调用，
// arguments 是 JSON 字符串，取其中的 command 字段解析。
// 时间戳取条目的 time（毫秒）。
func dshTaskEvents(entry map[string]any, turnIndex int) []mem.MemEvent {
	data := objMap(entry, "data")
	if data == nil || objStr(data, "name") != "bash" {
		return nil
	}
	argsStr := objStr(data, "arguments")
	if argsStr == "" {
		return nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		return nil
	}
	command, _ := args["command"].(string)
	if command == "" {
		return nil
	}
	ts := ""
	if ms := dshEntryTimeMs(entry); ms != nil {
		ts = formatISOTime(time.UnixMilli(*ms))
	}
	return mem.EventsFromParsed(mem.ParseTaskCommandsAll(command), ts, turnIndex)
}

// dshEntryTimeMs 取条目的 time 字段（毫秒时间戳）。
func dshEntryTimeMs(entry map[string]any) *int64 {
	return dshNumberMs(entry["time"])
}

// dshNumberMs 把数字（毫秒）转成 int64；非数字或非法值返回 nil。
func dshNumberMs(value any) *int64 {
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return nil
		}
		ms := int64(v)
		return &ms
	case int64:
		if v <= 0 {
			return nil
		}
		return &v
	}
	return nil
}

// dshTruncateRunes 按字数截断（标题 fallback 用），超长加省略号。
func dshTruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// dshReadSession 打开一个 DSH 会话文件（zstd 压的 JSONL），
// 解压后逐行解析并回调；打不开或解压失败时返回一条警告，不报错。
func dshReadSession(path string, onLine func(obj map[string]any) bool) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{fmt.Sprintf("无法打开会话文件（%s）：%v，已跳过", path, err)}
	}
	defer f.Close()
	dec, err := zstd.NewReader(f)
	if err != nil {
		return []string{fmt.Sprintf("无法解压 DSH 会话文件（%s）：%v，已跳过", path, err)}
	}
	defer dec.Close()
	return ReadJSONLReader(dec, path, onLine)
}
