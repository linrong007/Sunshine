// RunPreToolUse 实现 sunshine hook pre-tool-use，对应 Claude Code
// 的 PreToolUse 事件（settings.json 里 matcher 只接 Bash）：
// 白名单命令在执行前改写，省 token；不命中时无输出、退出 0。
//
// 改写分两档：
//  1. git log / git diff 直接加 flag（--oneline / --stat），不改命令结构；
//  2. 测试 / lint / 构建类吵闹命令包一层
//     `sunshine hook compress -- <原命令>`，由 compress 子命令跑原命令、
//     压缩输出、原样返回退出码。
//
// 契约（Claude Code 官方 hooks 文档，2026-11-19 核）：
// hookSpecificOutput.updatedInput 替换整个 tool_input，必须把原字段
// 原样带上、只改 command；不带 permissionDecision 时 auto 模式可能
// 直接拒绝改写后的调用，所以改写时一律带 permissionDecision "allow"
// （deny/ask 规则仍照常评估；改写是原命令的严格收窄，不增加新能力）。
//
// 铁律：任何情况下返回 nil（退出码 0）；解析失败一律原样放过，
// 绝不阻断 agent 会话。
package hook

import (
	"io"
	"os"
	"strconv"
	"strings"
)

// RunPreToolUse 处理 PreToolUse 事件：只对 Claude 平台做改写，
// 其余一律静默放过。按 tool_name 分发：Bash 走现有白名单改写，
// Read/Grep 走限流注入（缺 limit/head_limit 时补默认值）。
func RunPreToolUse(r io.Reader, opts Options) error {
	if HooksDisabled() {
		return nil
	}
	input := ReadInput(r)
	if len(input) == 0 {
		return nil
	}
	// v1 只做 Claude：Codex 的改写契约未核实，DSH 有无对应事件未核实。
	if DetectPlatform(input) != "claude" {
		return nil
	}
	switch GetString(input, "tool_name") {
	case "Bash":
		return runPreToolUseBash(input, opts)
	case "Read", "Grep":
		return runPreToolUseLimit(input, opts)
	default:
		return nil
	}
}

// runPreToolUseBash 是原有的 Bash 白名单改写分支。
func runPreToolUseBash(input HookInput, opts Options) error {
	toolInput, _ := input["tool_input"].(map[string]any)
	if toolInput == nil {
		return nil
	}
	command, _ := toolInput["command"].(string)
	if strings.TrimSpace(command) == "" {
		return nil
	}
	// 改写后的命令里用二进制的绝对路径，不依赖执行时的 PATH。
	bin, err := os.Executable()
	if err != nil || strings.TrimSpace(bin) == "" {
		bin = "sunshine"
	}
	rewritten, note := rewriteBashCommand(command, bin, timeoutOf(toolInput))
	if rewritten == "" || rewritten == command {
		return nil
	}
	// updatedInput 替换整个 tool_input：原字段原样带上，只改 command。
	updated := make(map[string]any, len(toolInput)+1)
	for k, v := range toolInput {
		updated[k] = v
	}
	updated["command"] = rewritten
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"permissionDecision": "allow",
			"updatedInput":       updated,
			"additionalContext":  note,
		},
	}
	WriteJSON(opts.out(), out)
	return nil
}

// 限流注入的默认值（squeez 增量机制，read_max_lines /
// grep_max_results）：只补缺失键，永不覆盖调用方显式值，
// 不设上限（传 5000 也原样放过）；Glob 不注入。
const (
	readDefaultLimit     = 300
	grepDefaultHeadLimit = 100
)

// runPreToolUseLimit 给 Read/Grep 补默认限流：Read 缺 limit 时
// 注入 300，Grep 缺 head_limit 时注入 100。键已存在（任何值，
// 含显式 0）时不动；解析失败原样放过。返回全量 updatedInput +
// permissionDecision "allow"，口径与 Bash 改写一致。
func runPreToolUseLimit(input HookInput, opts Options) error {
	toolName := GetString(input, "tool_name")
	toolInput, _ := input["tool_input"].(map[string]any)
	if toolInput == nil {
		return nil
	}
	var key string
	var value int
	var note string
	switch toolName {
	case "Read":
		key, value = "limit", readDefaultLimit
		note = "Sunshine 已补默认限流：Read 缺 limit，已注入 limit=300（只补缺失键，不改显式值）。"
	case "Grep":
		key, value = "head_limit", grepDefaultHeadLimit
		note = "Sunshine 已补默认限流：Grep 缺 head_limit，已注入 head_limit=100（只补缺失键，不改显式值）。"
	default:
		return nil
	}
	if v, ok := toolInput[key]; ok && v != nil {
		return nil
	}
	updated := make(map[string]any, len(toolInput)+1)
	for k, v := range toolInput {
		updated[k] = v
	}
	updated[key] = value
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"permissionDecision": "allow",
			"updatedInput":       updated,
			"additionalContext":  note,
		},
	}
	WriteJSON(opts.out(), out)
	return nil
}

// rewriteBashCommand 改写一条 Bash 命令，返回改写后的命令与一句说明；
// 不命中返回 ""（调用方原样放过）。
//
// 规则：
//   - 含管道 `|`（引号外）：agent 已在自己管输出，整条不动；
//   - 复合命令按 && / || / ; / 换行切段，每段独立判断、命中只改该段；
//   - 引号不平衡：整条不动；
//   - 含单独的 `&`（后台操作符）的段：不动（包一层会打乱后台语义）；
//   - 已含 `sunshine hook compress`：不重复包。
func rewriteBashCommand(command, sunshineBin string, timeoutMs int) (string, string) {
	segs, seps, ok := splitCommand(command)
	if !ok {
		return "", ""
	}
	for _, s := range seps {
		if s == "|" {
			return "", ""
		}
	}
	var notes []string
	changed := false
	for i, seg := range segs {
		lead, core, trail := splitAround(seg)
		if core == "" || hasLoneAmpersand(core) || strings.Contains(core, "hook compress") {
			continue
		}
		words := shellWords(core)
		if len(words) == 0 {
			continue
		}
		if nw, desc := rewriteGitSegment(core, words); nw != "" {
			segs[i] = lead + nw + trail
			notes = append(notes, desc)
			changed = true
			continue
		}
		if nw, desc := rewriteWrapSegment(core, words, sunshineBin, timeoutMs); nw != "" {
			segs[i] = lead + nw + trail
			notes = append(notes, desc)
			changed = true
		}
	}
	if !changed {
		return "", ""
	}
	var b strings.Builder
	for i, seg := range segs {
		b.WriteString(seg)
		b.WriteString(seps[i])
	}
	return b.String(), "Sunshine 已改写此命令以节省 token：" + strings.Join(notes, "；") + "。"
}

// gitFlagRules 是 git log / git diff 的改写规则：子命令、已带等价
// flag 时跳过、插入的 flag。
var gitFlagRules = []struct {
	subcmd string
	skip   []string
	insert string
}{
	{"log", []string{"--oneline", "--format", "-p", "--patch", "--stat", "--name-only", "--name-status", "--graph"}, "--oneline"},
	{"diff", []string{"--stat", "--name-only", "--name-status", "--numstat", "-p", "--patch", "--check"}, "--stat"},
}

// rewriteGitSegment 改写 git log / git diff 段：命中时在子命令词后
// 插入 flag。words 是 core 的 shell 词（含起止偏移）。
func rewriteGitSegment(core string, words []shellWord) (string, string) {
	if len(words) < 2 || words[0].text != "git" {
		return "", ""
	}
	for _, r := range gitFlagRules {
		if words[1].text != r.subcmd {
			continue
		}
		for _, w := range words[2:] {
			if hasGitFlag(w.text, r.skip) {
				return "", ""
			}
		}
		// 在子命令词后插入 flag。
		pos := words[1].end
		return core[:pos] + " " + r.insert + core[pos:], "git " + r.subcmd + " → git " + r.subcmd + " " + r.insert
	}
	return "", ""
}

// hasGitFlag 判断词 w 是否等价于 skip 列表里的 flag：
// 长 flag 精确匹配或 --flag=值 前缀匹配，短 flag 精确匹配。
func hasGitFlag(w string, skip []string) bool {
	for _, f := range skip {
		if w == f {
			return true
		}
		if strings.HasPrefix(f, "--") && strings.HasPrefix(w, f+"=") {
			return true
		}
	}
	return false
}

// wrapHeads 是包一层命令的白名单命令头（词序列）。
var wrapHeads = [][]string{
	{"go", "test"}, {"go", "vet"}, {"go", "build"},
	{"npm", "test"}, {"npm", "run", "test"},
	{"npx", "tsc"}, {"tsc"}, {"eslint"},
	{"cargo", "test"}, {"cargo", "clippy"},
	{"pytest"}, {"python", "-m", "pytest"},
	{"make", "test"}, {"make", "check"},
}

// rewriteWrapSegment 把白名单命令头开头的段包一层
// `sunshine hook compress [--timeout <毫秒>] -- '<原段>'`。
func rewriteWrapSegment(core string, words []shellWord, sunshineBin string, timeoutMs int) (string, string) {
	for _, head := range wrapHeads {
		if len(words) < len(head) {
			continue
		}
		match := true
		for i, h := range head {
			if words[i].text != h {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		wrapped := shellQuote(sunshineBin) + " hook compress"
		if timeoutMs > 0 {
			wrapped += " --timeout " + strconv.Itoa(timeoutMs)
		}
		wrapped += " -- " + shellQuote(core)
		return wrapped, strings.Join(head, " ") + " 已包一层压缩输出（只留失败项与摘要）"
	}
	return "", ""
}

// splitAround 把段拆成前导空白、核心、尾随空白。
func splitAround(seg string) (lead, core, trail string) {
	start := 0
	for start < len(seg) && (seg[start] == ' ' || seg[start] == '\t' || seg[start] == '\n') {
		start++
	}
	end := len(seg)
	for end > start && (seg[end-1] == ' ' || seg[end-1] == '\t' || seg[end-1] == '\n') {
		end--
	}
	return seg[:start], seg[start:end], seg[end:]
}

// hasLoneAmpersand 判断段里是否有单独的 `&`（后台操作符）：
// `&&`、`>&`、`&>` 里的 & 不算。
func hasLoneAmpersand(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			continue
		}
		if i > 0 && (s[i-1] == '&' || s[i-1] == '>') {
			continue
		}
		if i+1 < len(s) && (s[i+1] == '&' || s[i+1] == '>') {
			continue
		}
		return true
	}
	return false
}

// shellWord 是 shell 词：去引号后的文本与在原文中的起止偏移。
type shellWord struct {
	text  string
	start int
	end   int
}

// shellWords 按空白切分 shell 词（引号感知，词内引号去掉，
// 用于命令头匹配与 flag 检查；不要求引号平衡——调用前
// splitCommand 已保证平衡）。
func shellWords(s string) []shellWord {
	var words []shellWord
	var cur strings.Builder
	start := -1
	inSingle, inDouble := false, false
	flush := func(end int) {
		if start >= 0 {
			words = append(words, shellWord{text: cur.String(), start: start, end: end})
			cur.Reset()
			start = -1
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inDouble:
			if start < 0 {
				start = i
			}
			inSingle = !inSingle
		case c == '"' && !inSingle:
			if start < 0 {
				start = i
			}
			inDouble = !inDouble
		case c == '\\' && !inSingle && i+1 < len(s):
			if start < 0 {
				start = i
			}
			i++
			cur.WriteByte(s[i])
		case (c == ' ' || c == '\t' || c == '\n') && !inSingle && !inDouble:
			flush(i)
		default:
			if start < 0 {
				start = i
			}
			cur.WriteByte(c)
		}
	}
	flush(len(s))
	return words
}

// splitCommand 按 shell 运算符切分命令：返回段与分隔符，
// seps[i] 是 segs[i] 之后的分隔符（最后一段为 ""）。
// 段保留原文（含首尾空白），拼回即原命令。
// 引号不平衡时 ok=false。`|`、`&&`、`||`、`;`、换行都切分，
// 但 `|` 出现在段内时调用方会整条跳过（见 rewriteBashCommand）。
func splitCommand(cmd string) (segs []string, seps []string, ok bool) {
	var cur strings.Builder
	inSingle, inDouble := false, false
	flush := func(sep string) {
		segs = append(segs, cur.String())
		seps = append(seps, sep)
		cur.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case c == '\\' && !inSingle && i+1 < len(cmd):
			cur.WriteByte(c)
			i++
			cur.WriteByte(cmd[i])
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			cur.WriteByte(c)
		case c == '"' && !inSingle:
			inDouble = !inDouble
			cur.WriteByte(c)
		case !inSingle && !inDouble && c == '\n':
			flush("\n")
		case !inSingle && !inDouble && c == ';':
			flush(";")
		case !inSingle && !inDouble && c == '|':
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				flush("||")
				i++
			} else {
				flush("|")
			}
		case !inSingle && !inDouble && c == '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				flush("&&")
				i++
			} else {
				cur.WriteByte(c)
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inSingle || inDouble {
		return nil, nil, false
	}
	flush("")
	return segs, seps, true
}

// timeoutOf 从 tool_input 取 timeout（毫秒），非法或缺失返回 0。
func timeoutOf(toolInput map[string]any) int {
	if v, ok := toolInput["timeout"].(float64); ok && v > 0 {
		return int(v)
	}
	return 0
}
