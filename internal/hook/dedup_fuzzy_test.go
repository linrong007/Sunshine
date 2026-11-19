package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fuzzyToken 生成定长 token（15 字符），用于构造长度可控的模糊测试输出。
func fuzzyToken(i int) string {
	return fmt.Sprintf("token%08dxx", i)
}

// fuzzyOutput 生成 n 行输出，每行一个 token；lastChanged >= 0 时
// 把最后一行换成另一个 token（用于构造近似输出）。
func fuzzyOutput(n int, lastChanged int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		tok := fuzzyToken(i)
		if i == n-1 && lastChanged >= 0 {
			tok = fuzzyToken(lastChanged)
		}
		fmt.Fprintf(&b, "%s\n", tok)
	}
	return b.String()
}

func TestFuzzy_BashHit(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "git status --porcelain"}
	a := fuzzyOutput(20, -1)
	// 只改最后一行 token：18 个 trigram 里只变 1 个，
	// Jaccard = 17/19 ≈ 0.894 ≥ 0.85，应模糊命中。
	b := fuzzyOutput(20, 99999999)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	out2 := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(b, "")))
	repl := updatedToolOutput(t, out2)
	m, ok := repl.(map[string]any)
	if !ok {
		t.Fatalf("Bash 替换值应为对象形，实际：%v", repl)
	}
	stdout, _ := m["stdout"].(string)
	if !strings.Contains(stdout, "第 1 次") || !strings.Contains(stdout, "约 89%") || !strings.Contains(stdout, "相似") {
		t.Fatalf("模糊命中应换成相似引用（含第 1 次、约 89 百分比），实际：%q", stdout)
	}
	if !strings.Contains(stdout, "如需确认请重读") {
		t.Fatalf("模糊引用应写明可重读，实际：%q", stdout)
	}
	if n := dedupLines(t, root, "sess-1"); n != 1 {
		t.Fatalf("模糊命中后不应再记一条，表应仍为 1 行，实际 %d 行", n)
	}
}

func TestFuzzy_BelowThreshold(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "git status --porcelain"}
	// 20 个 token，只改第 2 个：trigram (0,1,2) 与 (1,2,3) 都变，
	// Jaccard = 16/20 = 0.80 < 0.85，不应命中。
	var b strings.Builder
	for i := 0; i < 20; i++ {
		tok := fuzzyToken(i)
		if i == 1 {
			tok = fuzzyToken(99999999)
		}
		fmt.Fprintf(&b, "%s\n", tok)
	}
	a := fuzzyOutput(20, -1)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(b.String(), ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("Jaccard 0.80 不应模糊命中，实际：%s", got)
	}
	if n := dedupLines(t, root, "sess-1"); n != 2 {
		t.Fatalf("未命中应记第二条，表应为 2 行，实际 %d 行", n)
	}
}

func TestFuzzy_LengthRatioGuard(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "git status --porcelain"}
	a := fuzzyOutput(20, -1)
	// B = A + 一行 100 字符的长行：trigram Jaccard = 18/19 ≈ 0.947，
	// 但长度比 320/421 ≈ 0.76 < 0.80，不应命中。
	b := a + strings.Repeat("z", 100) + "\n"
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(b, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("长度比不足不应模糊命中，实际：%s", got)
	}
}

func TestFuzzy_NetSavingGate(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "git status --porcelain"}
	// 20 行短 token（每行 2-3 字符）：Jaccard 17/19 ≈ 0.894，
	// 但全文仅约 70 字符，净省远不到 96 字符，不应命中。
	var a, b strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&a, "t%d\n", i)
		if i == 19 {
			fmt.Fprintf(&b, "z9\n")
		} else {
			fmt.Fprintf(&b, "t%d\n", i)
		}
	}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(a.String(), ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(b.String(), ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("净省不足不应模糊命中，实际：%s", got)
	}
}

func TestFuzzy_ShortOutputNoSketch(t *testing.T) {
	root, _ := makeProject(t)
	cmd := map[string]any{"command": "echo hi"}
	out := "line one\nline two\nline three\n"
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd, bashResponse(out, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	data, err := os.ReadFile(filepath.Join(root, ".sunshine", ".runtime", "dedup-sess-1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sketch") {
		t.Fatalf("不足 6 行的输出不应记素描，实际行：%s", string(data))
	}
	// ≥6 行的输出应记素描。
	cmd2 := map[string]any{"command": "git status --porcelain"}
	long := fuzzyOutput(20, -1)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Bash", cmd2, bashResponse(long, ""))); strings.TrimSpace(got) != "" {
		t.Fatalf("首次调用不应替换，实际：%s", got)
	}
	data, err = os.ReadFile(filepath.Join(root, ".sunshine", ".runtime", "dedup-sess-1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sketch") {
		t.Fatalf("≥6 行的输出应记素描，实际行：%s", string(data))
	}
}

func TestFuzzy_ReadMtimeChanged(t *testing.T) {
	root, _ := makeProject(t)
	rel := "src/a.go"
	abs := filepath.Join(root, rel)
	a := fuzzyOutput(20, -1)
	b := fuzzyOutput(20, 99999999)
	mustWrite(t, abs, a)
	toolInput := map[string]any{"file_path": rel}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, a)); strings.TrimSpace(got) != "" {
		t.Fatalf("首次读不应替换，实际：%s", got)
	}
	// 改文件内容（mtime 变了）：近似输出也不应模糊命中。
	mustWrite(t, abs, b)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, b)); strings.TrimSpace(got) != "" {
		t.Fatalf("Read 文件 mtime 变了，不应模糊命中，实际：%s", got)
	}
	// 同内容再读：精确命中仍生效。
	out := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, b))
	repl := updatedToolOutput(t, out)
	s, ok := repl.(string)
	if !ok || !strings.Contains(s, "第 2 次") {
		t.Fatalf("mtime 变后精确命中仍应生效，实际：%v", repl)
	}
}

func TestFuzzy_ReadMtimeSame(t *testing.T) {
	root, _ := makeProject(t)
	rel := "src/a.go"
	abs := filepath.Join(root, rel)
	a := fuzzyOutput(20, -1)
	b := fuzzyOutput(20, 99999999)
	mustWrite(t, abs, a)
	toolInput := map[string]any{"file_path": rel}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, a)); strings.TrimSpace(got) != "" {
		t.Fatalf("首次读不应替换，实际：%s", got)
	}
	// 改内容后把 mtime 恢复成记录时的值：模糊命中应生效。
	st, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	mtime := st.ModTime()
	mustWrite(t, abs, b)
	if err := os.Chtimes(abs, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if st2, err := os.Stat(abs); err != nil || !st2.ModTime().Equal(mtime) {
		t.Skip("文件系统 mtime 精度不足，跳过同 mtime 模糊命中用例")
	}
	out := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, b))
	repl := updatedToolOutput(t, out)
	s, ok := repl.(string)
	if !ok || !strings.Contains(s, "第 1 次") || !strings.Contains(s, "约 89%") {
		t.Fatalf("mtime 一致时 Read 近似输出应模糊命中，实际：%v", repl)
	}
}

func TestFuzzy_ReadNoMtimeOnlyExact(t *testing.T) {
	root, _ := makeProject(t)
	// 文件不存在（取不到 mtime）：近似输出不应模糊命中。
	toolInput := map[string]any{"file_path": "src/missing.go"}
	a := fuzzyOutput(20, -1)
	b := fuzzyOutput(20, 99999999)
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, a)); strings.TrimSpace(got) != "" {
		t.Fatalf("首次读不应替换，实际：%s", got)
	}
	if got := runPostToolUseOut(t, dedupInput(t, root, "sess-1", "Read", toolInput, b)); strings.TrimSpace(got) != "" {
		t.Fatalf("取不到 mtime 时 Read 只应精确命中，实际：%s", got)
	}
	if n := dedupLines(t, root, "sess-1"); n != 2 {
		t.Fatalf("未命中应记两条，实际 %d 行", n)
	}
}

func TestFuzzySketch_Jaccard(t *testing.T) {
	a := fuzzySketch("one two three four five six seven eight")
	b := fuzzySketch("one two three four five six seven eight")
	if j := fuzzyJaccard(a, b); j != 1 {
		t.Fatalf("相同文本 Jaccard 应为 1，实际 %v", j)
	}
	c := fuzzySketch("one two three four five six seven nine")
	// 6 个 trigram 里只变最后 1 个：交 5 并 7。
	if j := fuzzyJaccard(a, c); j < 0.71 || j > 0.72 {
		t.Fatalf("Jaccard 应约为 5/7，实际 %v", j)
	}
	if len(a) > fuzzySketchK {
		t.Fatalf("素描不应超过 %d 个，实际 %d", fuzzySketchK, len(a))
	}
}

func TestFuzzySketch_TooFewTokens(t *testing.T) {
	if s := fuzzySketch("only two"); len(s) != 0 {
		t.Fatalf("不足 3 个 token 应无素描，实际 %d 个", len(s))
	}
}
