// PostToolUse 会话去重（2026-11-19，注入顺序与会话去重第二档）。
//
// 机制：Claude Code 的 PostToolUse hook 可返回
// hookSpecificOutput.updatedToolOutput 替换工具结果（值须匹配工具
// 输出形状；Bash 是 {stdout, stderr, interrupted, isImage}；
// PostToolUseFailure 不能改写，失败原样过）。同一会话里 agent 重
// 读模型已经有的东西（没变过的命令输出、已读文件的某段行范围）时，
// 不再贴全文，只给一个短引用——原文已在会话上下文里，引用能找回它。
//
// 去重表：.sunshine/.runtime/dedup-<session>.jsonl，一行一条
// {seq, tool, key, hash}，只记哈希不存原文；上限 1000 行，超了截断
// 留最近（activity.go 同款追加 + 截断）。命中条件：同一会话（无
// session_id 不去重，跨会话去重是错的）、同一键、输出哈希完全相同、
// 输出不小于引用、Bash 要求成功。v1 只做 Claude（updatedToolOutput
// 是 Claude Code 的契约；其它平台的输出替换契约未核实）。
//
// 铁律：任何失败静默丢弃，绝不阻断 agent 会话；并发触发时读-查-写
// 竞态的失败模式是良性的（漏一次去重或重复记一条）。
package hook

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MaxDedupLines 是去重表的保留上限：写盘时顺手截断，只留最近这么多行。
const MaxDedupLines = 1000

// dedupRecord 是去重表里的一行。Seq 是行内序号（截断后仍单调递增，
// 引用里的“第 N 次”指它）；Hash 是输出全文的 SHA-256，只记哈希，
// 不存原文。
type dedupRecord struct {
	Seq  int    `json:"seq"`
	Tool string `json:"tool"`
	Key  string `json:"key"`
	Hash string `json:"hash"`
}

// dedupFilePath 返回会话去重表的文件路径。
func dedupFilePath(root, sessionID string) string {
	return filepath.Join(root, ".sunshine", ".runtime", "dedup-"+sanitizeKey(sessionID)+".jsonl")
}

// dedupKey 给工具调用算去重键。Bash 用去空白后的命令串；Read 用
// file_path + offset + limit。其它工具不支持，返回 ok=false。
func dedupKey(toolName string, toolInput map[string]any) (string, bool) {
	switch toolName {
	case "Bash":
		cmd, _ := toolInput["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			return "", false
		}
		return cmd, true
	case "Read":
		path, _ := toolInput["file_path"].(string)
		if strings.TrimSpace(path) == "" {
			return "", false
		}
		return path + "\x00" + dedupNumField(toolInput, "offset") + "\x00" + dedupNumField(toolInput, "limit"), true
	default:
		return "", false
	}
}

// dedupNumField 取 tool_input 里的数字字段（JSON 数或字符串两种写法都接）。
func dedupNumField(toolInput map[string]any, key string) string {
	switch v := toolInput[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.Itoa(int(v))
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// dedupOutput 从 tool_response 里取可去重的输出全文。返回（全文，是否
// 有资格去重）。Bash 要求成功：stderr 为空；有 exitCode 字段时要求
// 为 0（字符串形的响应没有可分的 stderr / exitCode，以全文哈希为准）。
// Read 接字符串形与含 content / text 字段的对象形，形状不符不去重。
func dedupOutput(toolName string, toolResponse any) (string, bool) {
	switch toolName {
	case "Bash":
		switch resp := toolResponse.(type) {
		case string:
			return resp, true
		case map[string]any:
			stdout, _ := resp["stdout"].(string)
			stderr, _ := resp["stderr"].(string)
			if stderr != "" {
				return "", false
			}
			if code, ok := resp["exitCode"].(float64); ok && int(code) != 0 {
				return "", false
			}
			return stdout, true
		default:
			return "", false
		}
	case "Read":
		switch resp := toolResponse.(type) {
		case string:
			return resp, true
		case map[string]any:
			if content, _ := resp["content"].(string); content != "" {
				return content, true
			}
			if text, _ := resp["text"].(string); text != "" {
				return text, true
			}
			return "", false
		default:
			return "", false
		}
	default:
		return "", false
	}
}

// dedupHash 算输出全文的 SHA-256（hex）。
func dedupHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// dedupReference 生成替换用的短引用。N 是首次记录的 seq（行内序号）。
func dedupReference(seq int) string {
	return fmt.Sprintf("[Sunshine: 与本会话第 %d 次工具调用的输出相同，已在上下文中，不再重复]", seq)
}

// dedupReadRecords 读去重表（不存在返回空切片，不报错）；解析失败的行跳过。
func dedupReadRecords(root, sessionID string) []dedupRecord {
	f, err := os.Open(dedupFilePath(root, sessionID))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []dedupRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec dedupRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Tool == "" || rec.Key == "" || rec.Hash == "" {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// dedupNextSeq 返回下一条记录的 seq（末条 seq + 1，空表为 1）。
func dedupNextSeq(records []dedupRecord) int {
	if len(records) == 0 {
		return 1
	}
	return records[len(records)-1].Seq + 1
}

// dedupLookup 在表里找 tool + key + hash 全相同的记录，返回其 seq。
func dedupLookup(records []dedupRecord, tool, key, hash string) (int, bool) {
	for _, rec := range records {
		if rec.Tool == tool && rec.Key == key && rec.Hash == hash {
			return rec.Seq, true
		}
	}
	return 0, false
}

// dedupAppend 追加一条记录；失败静默丢弃。写盘后若行数超过
// MaxDedupLines，截断只留最近 MaxDedupLines 行（seq 保持原值）。
func dedupAppend(root, sessionID string, rec dedupRecord) {
	if root == "" || sessionID == "" {
		return
	}
	path := dedupFilePath(root, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, err = f.Write(append(line, '\n'))
	_ = f.Close()
	if err != nil {
		return
	}
	dedupTruncate(path)
}

// dedupTruncate 在行数超限时重写文件只留最近 MaxDedupLines 行；
// 任何失败都静默（文件多长一点，下次写盘再试）。
func dedupTruncate(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= MaxDedupLines {
		return
	}
	keep := lines[len(lines)-MaxDedupLines:]
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// dedupReplacement 按工具输出形状造 updatedToolOutput 的值。Bash
// 对象形保留 interrupted / isImage、stdout 换成引用、stderr 置空；
// 字符串形与 Read 字符串形直接换成引用；Read 对象形替换 content /
// text 字段（原字段名保留）。形状不符返回 ok=false（原样放过）。
func dedupReplacement(toolName string, toolResponse any, ref string) (any, bool) {
	switch toolName {
	case "Bash":
		switch resp := toolResponse.(type) {
		case string:
			return ref, true
		case map[string]any:
			out := map[string]any{
				"stdout":      ref,
				"stderr":      "",
				"interrupted": resp["interrupted"],
				"isImage":     resp["isImage"],
			}
			if _, ok := resp["interrupted"]; !ok {
				out["interrupted"] = false
			}
			if _, ok := resp["isImage"]; !ok {
				out["isImage"] = false
			}
			return out, true
		default:
			return nil, false
		}
	case "Read":
		switch resp := toolResponse.(type) {
		case string:
			return ref, true
		case map[string]any:
			out := make(map[string]any, len(resp))
			for k, v := range resp {
				out[k] = v
			}
			if _, ok := resp["content"].(string); ok {
				out["content"] = ref
				return out, true
			}
			if _, ok := resp["text"].(string); ok {
				out["text"] = ref
				return out, true
			}
			return nil, false
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

// dedupPostToolUse 跑一次会话去重：命中时写出 updatedToolOutput 替换
// 输出并返回 true；未命中时记一条记录、返回 false；不适用时直接
// 返回 false（不写任何输出）。任何失败都静默返回 false。
func dedupPostToolUse(input HookInput, root string, opts Options) bool {
	// v1 只做 Claude：updatedToolOutput 是 Claude Code 的契约。
	if DetectPlatform(input) != "claude" {
		return false
	}
	sessionID := GetString(input, "session_id", "sessionId", "sessionID")
	if sessionID == "" {
		return false
	}
	toolName := GetString(input, "tool_name")
	toolInput, _ := input["tool_input"].(map[string]any)
	key, ok := dedupKey(toolName, toolInput)
	if !ok {
		return false
	}
	toolResponse, hasResp := input["tool_response"]
	if !hasResp {
		return false
	}
	text, eligible := dedupOutput(toolName, toolResponse)
	if !eligible {
		return false
	}
	hash := dedupHash(text)
	records := dedupReadRecords(root, sessionID)
	if seq, found := dedupLookup(records, toolName, key, hash); found {
		ref := dedupReference(seq)
		// 输出比引用还短时，换引用反而更长，不换。
		if len(text) < len(ref) {
			return false
		}
		repl, ok := dedupReplacement(toolName, toolResponse, ref)
		if !ok {
			return false
		}
		WriteJSON(opts.out(), map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PostToolUse",
				"updatedToolOutput": repl,
			},
		})
		return true
	}
	dedupAppend(root, sessionID, dedupRecord{Seq: dedupNextSeq(records), Tool: toolName, Key: key, Hash: hash})
	return false
}
