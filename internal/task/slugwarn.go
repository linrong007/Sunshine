package task

import "strings"

// genericSlugTokens 是命名里区分度最低的通用词，近似判断时先剥掉：
// 两个名字只差这类词，口头指代时照样分不清。
var genericSlugTokens = map[string]bool{
	"management": true, "mgmt": true, "page": true, "list": true,
	"detail": true, "details": true, "view": true, "module": true,
	"admin": true, "system": true, "feature": true, "info": true,
}

func slugTokens(slug string) []string {
	var out []string
	for _, p := range strings.Split(strings.ToLower(slug), "-") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stripGenericSlugTokens(tokens []string) []string {
	var out []string
	for _, tok := range tokens {
		if !genericSlugTokens[tok] {
			out = append(out, tok)
		}
	}
	return out
}

// tokenSubseq 判断 short 是否为 long 的子序列（保序、可不连续）。
func tokenSubseq(short, long []string) bool {
	i := 0
	for _, tok := range long {
		if i < len(short) && short[i] == tok {
			i++
		}
	}
	return i == len(short)
}

// SimilarSlug 判断两个 slug 是否近似到容易混淆：
//  1. 原始 slug 互为子串（如「额度管理」与「假期额度管理」）；
//  2. 剥掉通用词后，一方的词序列是另一方的子序列（含相等，即两名
//     只差通用词，如 leave-management 与 leave-request-mgmt 剥后为
//     [leave] 与 [leave request]）。
//
// 两边剥后都为空（名字全是通用词）时不判近似，避免噪声。
func SimilarSlug(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	ta := stripGenericSlugTokens(slugTokens(a))
	tb := stripGenericSlugTokens(slugTokens(b))
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	return tokenSubseq(ta, tb) || tokenSubseq(tb, ta)
}

// DirSlug 从任务目录名（MM-DD-slug；00- 系统任务的 slug 从第二段起）
// 取出 slug 部分；不是任务目录名时返回空串。
func DirSlug(dirName string) string {
	parts := strings.SplitN(dirName, "-", 3)
	if len(parts) < 3 {
		return ""
	}
	if parts[0] == "00" {
		return parts[1] + "-" + parts[2]
	}
	return parts[2]
}
