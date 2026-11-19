// 模糊去重（squeez 增量机制，2026-11-19）：PostToolUse 跨调用去重的
// 第二档。精确命中先跑，模糊只在精确未命中时跑。
//
// 素描：输出全文按空白分词，取连续三元组（trigram），每组 FNV-1a-64
// 哈希，取最小的 96 个（bottom-k MinHash），排序去重；Jaccard 在
// 两个有序去重集合上算。只记素描不存原文。
//
// 命中条件（全满足）：同会话 + 同键 + 输出 ≥6 行 + 长度比 min/max
// ≥ 0.80 + trigram Jaccard ≥ 0.85 + 只查最近 16 条 + 净省 ≥24 token
// （约 96 字符）。Read 模糊命中须文件 mtime 与记录时一致；取不到
// mtime 时 Read 只允许精确命中。MCP 工具不做模糊去重（dedupKey
// 只接 Bash/Read，MCP 到不了这里）。
//
// 铁律：任何失败静默原样放过，绝不阻断 agent 会话。
package hook

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// 模糊去重阈值（PLAN.md squeez 一节）。
const (
	fuzzySketchK        = 96   // bottom-k MinHash 保留的哈希个数
	fuzzyMinLines       = 6    // 输出至少这么多行才记素描、才做模糊
	fuzzyMinLengthRatio = 0.80 // 长度比 min/max 下限
	fuzzyMinJaccard     = 0.85 // trigram Jaccard 下限
	fuzzyRecentWindow   = 16   // 只查最近这么多条记录
	fuzzyMinNetSaving   = 96   // 净省下限（字符数，约 24 token）
)

// fuzzySketch 算输出全文的 bottom-k MinHash 素描：空白 token 的
// 连续三元组，每组 FNV-1a-64，取最小的 96 个，排序去重。token
// 不足 3 个时返回空（无法做模糊）。
func fuzzySketch(text string) []uint64 {
	tokens := strings.Fields(text)
	if len(tokens) < 3 {
		return nil
	}
	hashes := make([]uint64, 0, len(tokens)-2)
	for i := 0; i+2 < len(tokens); i++ {
		h := fnv.New64a()
		_, _ = h.Write([]byte(tokens[i]))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(tokens[i+1]))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(tokens[i+2]))
		hashes = append(hashes, h.Sum64())
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })
	// 去重后取最小的 96 个。
	uniq := hashes[:0]
	for _, v := range hashes {
		if len(uniq) == 0 || uniq[len(uniq)-1] != v {
			uniq = append(uniq, v)
		}
	}
	if len(uniq) > fuzzySketchK {
		uniq = uniq[:fuzzySketchK]
	}
	return uniq
}

// fuzzyJaccard 算两个有序去重素描集合的 Jaccard 相似度。
func fuzzyJaccard(a, b []uint64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	i, j, inter := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			inter++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// fuzzyLineCount 数输出行数（末尾换行不另算一行）。
func fuzzyLineCount(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSuffix(text, "\n"), "\n"))
}

// fuzzyReference 生成模糊命中的替换文本。N 是首次记录的 seq，
// pct 是 Jaccard 百分比取整。
func fuzzyReference(seq int, pct int) string {
	return "[Sunshine: 与本会话第 " + strconv.Itoa(seq) + " 次工具调用的输出约 " + strconv.Itoa(pct) + "% 相似，已在上下文中，不再重复，如需确认请重读]"
}

// dedupReadMtime 取 Read 目标文件的 mtime（Unix 纳秒）。路径为
// 相对路径时按 hook 输入的 cwd 解析；取不到返回 0（调用方把 0
// 当作“无 mtime”，Read 只做精确命中）。
func dedupReadMtime(input HookInput, toolInput map[string]any) int64 {
	path, _ := toolInput["file_path"].(string)
	if strings.TrimSpace(path) == "" {
		return 0
	}
	if !filepath.IsAbs(path) {
		cwd := GetString(input, "cwd")
		if cwd == "" {
			if wd, err := os.Getwd(); err == nil {
				cwd = wd
			} else {
				return 0
			}
		}
		path = filepath.Join(cwd, path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}

// fuzzyLookup 在最近 16 条同工具同键的记录里找模糊命中，返回
// （首次记录 seq，Jaccard）。记录无素描或无长度时跳过（旧行
// 没有这些字段，只做精确命中）；Read 命中须 mtime 与记录时
// 一致（记录 mtime 为 0 或当前取不到时跳过）；按 Jaccard 从高
// 到低依次验长度比与净省，首个全过者命中。
func fuzzyLookup(records []dedupRecord, tool, key, text string, sketch []uint64, mtime int64) (int, float64, bool) {
	if len(sketch) == 0 || fuzzyLineCount(text) < fuzzyMinLines {
		return 0, 0, false
	}
	textLen := len([]rune(text))
	// 只查最近 16 条。
	start := 0
	if len(records) > fuzzyRecentWindow {
		start = len(records) - fuzzyRecentWindow
	}
	type candidate struct {
		seq    int
		score  float64
		recLen int
	}
	var cands []candidate
	for _, rec := range records[start:] {
		if rec.Tool != tool || rec.Key != key || len(rec.Sketch) == 0 || rec.Len <= 0 {
			continue
		}
		if tool == "Read" {
			// mtime 护栏：记录时或命中时取不到 mtime，只允许精确命中。
			if rec.Mtime == 0 || mtime == 0 || rec.Mtime != mtime {
				continue
			}
		}
		j := fuzzyJaccard(sketch, rec.Sketch)
		if j < fuzzyMinJaccard {
			continue
		}
		// 长度与 seq 一起带上，不再按 seq 二次查表（并发下 seq 可能重复）。
		cands = append(cands, candidate{seq: rec.Seq, score: j, recLen: rec.Len})
	}
	// 按 Jaccard 从高到低，对每个依次验长度比与净省。
	sort.Slice(cands, func(i, k int) bool { return cands[i].score > cands[k].score })
	for _, c := range cands {
		// 长度比：新旧输出字符数 min/max ≥ 0.80。
		ratio := float64(textLen) / float64(c.recLen)
		if c.recLen < textLen {
			ratio = float64(c.recLen) / float64(textLen)
		}
		if ratio < fuzzyMinLengthRatio {
			continue
		}
		// 净省：新输出字符数 - 引用文本字符数 ≥ 96。
		ref := fuzzyReference(c.seq, int(c.score*100))
		if textLen-len([]rune(ref)) < fuzzyMinNetSaving {
			continue
		}
		return c.seq, c.score, true
	}
	return 0, 0, false
}
