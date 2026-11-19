// log-template 折叠（squeez 增量机制，2026-11-19）：只在 Bash
// 输出压缩管道（compress.go）内做，不碰 Read/Grep 结果。
//
// 位置：压缩路径里连续重复折叠之后、首 10 + 失败行 + 末 80 行
// 截断之前。失败特征行永不折叠。
//
// 推导：整行按空白分词，逐 token 判变量；`key=value` 只把值换成
// `{}`、键保留。变量四类：①纯数字形（仅数字 + `.:-/T+` 分隔符，
// 如 12:30:45、192.168.0.1、3.14；三段式版本号 1.2.3 除外，按
// 规范保留）；②数字+单位（1ms、200ms、5s、3.2KB、50%、1.5x，
// 后缀 ≤4 字母/%）；③hex/UUID（长度 ≥8、≥2 个数字、全 hex 或
// `-`）；④复合 ID（带数字且含 `-`/`_` 分隔符，如 req-1）。
// 纯单词（含 ERROR/INFO）是结构，不替换。
//
// 阈值：模板出现次数 ≥3 才折叠；只有含 `{}` 槽位的模板才折叠，
// 无变量的逐字重复行留给连续重复折叠。标记：
// `[×N] <模板>  (e.g. <首行原文样本>)`，样本为首次出现行原文，
// trim 后超 100 字符截断加 `…`；折叠行发在首次出现位置保序，
// 其余出现直接丢弃。
package hook

import (
	"fmt"
	"strings"
)

// logTemplateMinCount 是模板折叠的出现次数阈值。
const logTemplateMinCount = 3

// logTemplateSampleMax 是标记里原文样本的最大字符数。
const logTemplateSampleMax = 100

// foldLogTemplates 折叠重复出现的日志模板行：同一模板出现 ≥3 次
// 时只在首次出现位置留一行标记，其余丢弃；失败行与出现不足
// 3 次的模板原样保留。
func foldLogTemplates(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	type occurrence struct {
		firstIdx int
		first    string
		count    int
	}
	templates := map[string]*occurrence{}
	order := []string{}
	for i, line := range lines {
		if isFailureLine(line) {
			continue
		}
		tmpl := logTemplateOf(line)
		if !strings.Contains(tmpl, "{}") {
			continue
		}
		occ, ok := templates[tmpl]
		if !ok {
			occ = &occurrence{firstIdx: i, first: line}
			templates[tmpl] = occ
			order = append(order, tmpl)
		}
		occ.count++
	}
	foldable := map[string]bool{}
	for _, tmpl := range order {
		if templates[tmpl].count >= logTemplateMinCount {
			foldable[tmpl] = true
		}
	}
	if len(foldable) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if isFailureLine(line) {
			out = append(out, line)
			continue
		}
		tmpl := logTemplateOf(line)
		if !foldable[tmpl] {
			out = append(out, line)
			continue
		}
		occ := templates[tmpl]
		if i != occ.firstIdx {
			continue // 其余出现直接丢弃
		}
		sample := strings.TrimSpace(occ.first)
		if r := []rune(sample); len(r) > logTemplateSampleMax {
			sample = string(r[:logTemplateSampleMax]) + "…"
		}
		out = append(out, fmt.Sprintf("[×%d] %s  (e.g. %s)", occ.count, tmpl, sample))
	}
	return out
}

// logTemplateOf 把一行推导成模板：逐 token 判变量，变量换成 `{}`，
// 其余原样保留；token 间用单个空白连接。
func logTemplateOf(line string) string {
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, len(tokens))
	for i, tok := range tokens {
		parts[i] = logTemplateToken(tok)
	}
	return strings.Join(parts, " ")
}

// logTemplateToken 判断单个 token：变量换成 `{}`（key=value 只换
// 值），结构原样保留。
func logTemplateToken(tok string) string {
	if eq := strings.Index(tok, "="); eq > 0 && eq < len(tok)-1 {
		return tok[:eq] + "={}"
	}
	if isLogVariable(tok) {
		return "{}"
	}
	return tok
}

// isLogVariable 判断 token 是否属于四类变量之一。
func isLogVariable(tok string) bool {
	if tok == "" {
		return false
	}
	if isNumericForm(tok) || isNumberWithUnit(tok) || isHexOrUUID(tok) || isCompositeID(tok) {
		return true
	}
	return false
}

// isNumericForm 是纯数字形：仅数字 + `.:-/T+` 分隔符，至少含
// 一个数字；三段式版本号（1.2.3）按规范保留，不算变量。
func isNumericForm(tok string) bool {
	hasDigit := false
	for _, r := range tok {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.' || r == ':' || r == '-' || r == '/' || r == 'T' || r == '+':
		default:
			return false
		}
	}
	if !hasDigit {
		return false
	}
	if isVersionForm(tok) {
		return false
	}
	return true
}

// isVersionForm 判断三段式版本号 1.2.3（恰好三段纯数字，点分隔）。
func isVersionForm(tok string) bool {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// isNumberWithUnit 是数字+单位：数字（可选一位小数）+ 后缀
// （≤4 字母或 %），如 200ms、5s、3.2KB、50%、1.5x。
func isNumberWithUnit(tok string) bool {
	if tok == "" || tok[0] < '0' || tok[0] > '9' {
		return false
	}
	i := 0
	dots := 0
	for i < len(tok) && ((tok[i] >= '0' && tok[i] <= '9') || tok[i] == '.') {
		if tok[i] == '.' {
			dots++
		}
		i++
	}
	if i == 0 || dots > 1 || i == len(tok) {
		return false
	}
	// 数字部分不能以点开头或结尾。
	if tok[0] == '.' || tok[i-1] == '.' {
		return false
	}
	suffix := tok[i:]
	if len(suffix) > 4 {
		return false
	}
	for _, r := range suffix {
		if r == '%' {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}

// isHexOrUUID 是 hex/UUID：长度 ≥8、≥2 个数字、全 hex 或 `-`。
func isHexOrUUID(tok string) bool {
	if len([]rune(tok)) < 8 {
		return false
	}
	digits := 0
	for _, r := range tok {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F'):
		case r == '-':
		default:
			return false
		}
	}
	return digits >= 2
}

// isCompositeID 是复合 ID：带数字且含 `-` 或 `_` 分隔符。
func isCompositeID(tok string) bool {
	hasDigit, hasSep := false, false
	for _, r := range tok {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '-' || r == '_':
			hasSep = true
		}
	}
	return hasDigit && hasSep
}
