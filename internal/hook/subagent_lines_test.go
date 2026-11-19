package hook

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestParseLineRange：lines 字段 "120-180"（1-based 闭区间）。
func TestParseLineRange(t *testing.T) {
	cases := []struct {
		in    string
		start int
		end   int
		ok    bool
	}{
		{"120-180", 120, 180, true},
		{"1-1", 1, 1, true},
		{" 120 - 180 ", 120, 180, true},
		{"", 0, 0, false},
		{"180-120", 0, 0, false},
		{"0-10", 0, 0, false},
		{"120", 0, 0, false},
		{"abc", 0, 0, false},
		{"120-180-200", 0, 0, false},
		{"12a-180", 0, 0, false},
	}
	for _, c := range cases {
		start, end, ok := parseLineRange(c.in)
		if ok != c.ok || start != c.start || end != c.end {
			t.Errorf("parseLineRange(%q) = (%d, %d, %v)，想要 (%d, %d, %v)", c.in, start, end, ok, c.start, c.end, c.ok)
		}
	}
}

// TestSliceLines：按行切片（末尾空行不计入总行数）。
func TestSliceLines(t *testing.T) {
	data := []byte("l1\nl2\nl3\nl4\nl5\n")
	content, total, end := sliceLines(data, 2, 4)
	if content != "l2\nl3\nl4" || total != 5 || end != 4 {
		t.Errorf("sliceLines(2-4) = (%q, %d, %d)，想要 (\"l2\\nl3\\nl4\", 5, 4)", content, total, end)
	}
	// end 越界截到文件末尾。
	content, total, end = sliceLines(data, 4, 99)
	if content != "l4\nl5" || total != 5 || end != 5 {
		t.Errorf("sliceLines(4-99) = (%q, %d, %d)，想要 (\"l4\\nl5\", 5, 5)", content, total, end)
	}
	// 单行。
	content, _, end = sliceLines(data, 3, 3)
	if content != "l3" || end != 3 {
		t.Errorf("sliceLines(3-3) = (%q, %d)，想要 (\"l3\", 3)", content, end)
	}
	// start 越界：空内容、末行 0。
	content, total, end = sliceLines(data, 20, 30)
	if content != "" || total != 5 || end != 0 {
		t.Errorf("sliceLines(20-30) = (%q, %d, %d)，想要 (\"\", 5, 0)", content, total, end)
	}
}

// tenLineSpecFile 在规范目录下写一份 10 行的文件，每行可单独断言。
func tenLineSpecFile(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		b.WriteString("第" + strconv.Itoa(i) + "行-规范内容\n")
	}
	rel := ".sunshine/spec/backend.md"
	mustWrite(t, filepath.Join(root, rel), b.String())
	return rel
}

func TestMaterializeFile_Lines(t *testing.T) {
	root, _ := makeProject(t)
	rel := tenLineSpecFile(t, root)
	limits := limitsFromConfig(LoadConfig(root))
	b := &budget{maxTotal: 0}
	block := materializeFile(root, rel, "后端规范：接口定义", "3-5", limits, b)
	for _, want := range []string{"第3行-规范内容", "第4行-规范内容", "第5行-规范内容", "(lines 3-5 of 10)"} {
		if !strings.Contains(block, want) {
			t.Errorf("切片注入缺少 %q\n全文：\n%s", want, block)
		}
	}
	for _, notWant := range []string{"第1行-规范内容", "第2行-规范内容", "第6行-规范内容", "第10行-规范内容"} {
		if strings.Contains(block, notWant) {
			t.Errorf("切片注入不应含 %q\n全文：\n%s", notWant, block)
		}
	}
}

func TestMaterializeFile_LinesEndClamped(t *testing.T) {
	root, _ := makeProject(t)
	rel := tenLineSpecFile(t, root)
	limits := limitsFromConfig(LoadConfig(root))
	b := &budget{maxTotal: 0}
	block := materializeFile(root, rel, "后端规范：接口定义", "8-99", limits, b)
	if !strings.Contains(block, "(lines 8-10 of 10)") {
		t.Errorf("end 越界应截到末尾并注明实际行号，得到：\n%s", block)
	}
	if !strings.Contains(block, "第10行-规范内容") || strings.Contains(block, "第7行-规范内容") {
		t.Errorf("截断后的切片内容不对：\n%s", block)
	}
}

func TestMaterializeFile_LinesStartOutOfRange(t *testing.T) {
	root, _ := makeProject(t)
	rel := tenLineSpecFile(t, root)
	limits := limitsFromConfig(LoadConfig(root))
	b := &budget{maxTotal: 0}
	block := materializeFile(root, rel, "后端规范：接口定义", "20-30", limits, b)
	if !strings.Contains(block, "超出文件总行数") {
		t.Errorf("start 越界应返回越界提示，得到：\n%s", block)
	}
	if strings.Contains(block, "第1行-规范内容") {
		t.Errorf("start 越界不应注入任何行，得到：\n%s", block)
	}
}

func TestMaterializeFile_LinesInvalidFallsBackToWholeFile(t *testing.T) {
	root, _ := makeProject(t)
	rel := tenLineSpecFile(t, root)
	limits := limitsFromConfig(LoadConfig(root))
	b := &budget{maxTotal: 0}
	block := materializeFile(root, rel, "后端规范：接口定义", "abc", limits, b)
	if !strings.Contains(block, "第1行-规范内容") || !strings.Contains(block, "第10行-规范内容") {
		t.Errorf("lines 格式非法应降级为整文件注入，得到：\n%s", block)
	}
	if strings.Contains(block, "(lines ") {
		t.Errorf("降级注入不应带行范围块头，得到：\n%s", block)
	}
}

func TestMaterializeFile_NoLinesInjectsWholeFile(t *testing.T) {
	root, _ := makeProject(t)
	rel := tenLineSpecFile(t, root)
	limits := limitsFromConfig(LoadConfig(root))
	b := &budget{maxTotal: 0}
	block := materializeFile(root, rel, "后端规范：接口定义", "", limits, b)
	if !strings.Contains(block, "第1行-规范内容") || !strings.Contains(block, "第10行-规范内容") {
		t.Errorf("无 lines 应注入整文件，得到：\n%s", block)
	}
	if strings.Contains(block, "(lines ") {
		t.Errorf("无 lines 不应带行范围块头，得到：\n%s", block)
	}
}

// TestReadJSONLEntries_LinesInvalidWarns：注入解析时 lines 格式非法，
// 记 stderr 警告并降级为空（hook 绝不阻断会话）。
func TestReadJSONLEntries_LinesInvalidWarns(t *testing.T) {
	root, _ := makeProject(t)
	tenLineSpecFile(t, root)
	taskDir := ".sunshine/tasks/11-09-demo"
	mustWrite(t, filepath.Join(root, taskDir, "implement.jsonl"),
		"{\"file\": \".sunshine/spec/backend.md\", \"reason\": \"后端规范：接口定义\", \"lines\": \"abc\"}\n")
	var errBuf bytes.Buffer
	entries := readJSONLEntries(root, taskDir+"/implement.jsonl", &errBuf)
	if len(entries) != 1 || entries[0].Lines != "" {
		t.Errorf("非法 lines 应被清空，得到：%v", entries)
	}
	if !strings.Contains(errBuf.String(), "格式非法") {
		t.Errorf("非法 lines 应写 stderr 警告，得到：%q", errBuf.String())
	}
}

// TestSubagentContext_ImplementLines：端到端——implement 子代理的注入
// 提示词里只含原型切片行，不含范围外的行。
func TestSubagentContext_ImplementLines(t *testing.T) {
	root, _ := makeProject(t)
	tenLineSpecFile(t, root)
	taskDir := ".sunshine/tasks/11-09-demo"
	mustWrite(t, filepath.Join(root, taskDir, "implement.jsonl"),
		"{\"file\": \".sunshine/spec/backend.md\", \"reason\": \"后端规范：接口定义\", \"lines\": \"3-5\"}\n")
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunSubagentContext(in, o) },
		`{"session_id":"sess-1","cwd":`+strconv_quot(root)+`,"tool_name":"Task","tool_input":{"subagent_type":"sunshine-implement","prompt":"实现演示功能"}}`)
	data := mustJSON(t, out)
	hs := data["hookSpecificOutput"].(map[string]any)
	updated := hs["updatedInput"].(map[string]any)
	prompt, _ := updated["prompt"].(string)
	for _, want := range []string{"第3行-规范内容", "第5行-规范内容", "(lines 3-5 of 10)"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("implement 提示词缺少 %q\n全文：\n%s", want, prompt)
		}
	}
	for _, notWant := range []string{"第1行-规范内容", "第2行-规范内容", "第6行-规范内容"} {
		if strings.Contains(prompt, notWant) {
			t.Errorf("implement 提示词不应含 %q\n全文：\n%s", notWant, prompt)
		}
	}
}

// TestExtractSectionsByTitle：只保留标题含关键词的 ## / ### 节。
func TestExtractSectionsByTitle(t *testing.T) {
	content := "# 演示任务\n\n## 目标\n目标行\n\n## 验收标准\n标准一\n\n### 子项\n子项行\n\n## 参考\n参考行\n\n## 追加需求\n追加行\n\n### 客户列表的验收标准（R-006）\nR006标准行\n\n## 开放问题\n问题行\n"
	got := extractSectionsByTitle(content, "验收标准")
	for _, want := range []string{"## 验收标准", "标准一", "### 子项", "子项行", "### 客户列表的验收标准（R-006）", "R006标准行"} {
		if !strings.Contains(got, want) {
			t.Errorf("验收标准节应含 %q，得到：\n%s", want, got)
		}
	}
	for _, notWant := range []string{"目标行", "参考行", "追加行", "问题行", "## 目标", "## 参考", "## 追加需求", "## 开放问题"} {
		if strings.Contains(got, notWant) {
			t.Errorf("验收标准节不应含 %q，得到：\n%s", notWant, got)
		}
	}
	if got2 := extractSectionsByTitle(content, "不存在的关键词"); got2 != "" {
		t.Errorf("无命中应返回空串，得到：%q", got2)
	}
}

// checkDietProject 搭一个带 prd/design/implement.md 与 check.jsonl 的任务目录。
func checkDietProject(t *testing.T, prd string) (root, taskDir string) {
	t.Helper()
	root, _ = makeProject(t)
	tenLineSpecFile(t, root)
	taskDir = ".sunshine/tasks/11-09-demo"
	mustWrite(t, filepath.Join(root, taskDir, "check.jsonl"),
		"{\"file\": \".sunshine/spec/backend.md\", \"reason\": \"后端规范：接口定义\", \"lines\": \"3-5\"}\n")
	mustWrite(t, filepath.Join(root, taskDir, "prd.md"), prd)
	mustWrite(t, filepath.Join(root, taskDir, "design.md"), "# 技术设计\n\n设计正文-不应注入。\n")
	mustWrite(t, filepath.Join(root, taskDir, "implement.md"), "# 执行计划\n\n执行计划正文-不应注入。\n")
	return root, taskDir
}

func checkPrompt(t *testing.T, root string) string {
	t.Helper()
	out, _ := runWithInput(t, func(in *strings.Reader, o Options) error { return RunSubagentContext(in, o) },
		`{"session_id":"sess-1","cwd":`+strconv_quot(root)+`,"tool_name":"Task","tool_input":{"subagent_type":"sunshine-check","prompt":"复核演示改动"}}`)
	data := mustJSON(t, out)
	hs := data["hookSpecificOutput"].(map[string]any)
	updated := hs["updatedInput"].(map[string]any)
	prompt, _ := updated["prompt"].(string)
	return prompt
}

// TestSubagentContext_CheckDiet：端到端——check 注入只含验收标准节与
// check.jsonl 精选条目，不含 design.md / implement.md 全文与 prd 其余节。
func TestSubagentContext_CheckDiet(t *testing.T) {
	root, _ := checkDietProject(t, "# 演示任务\n\n## 目标\n目标正文-不应注入。\n\n## 需求\n需求正文-不应注入。\n\n## 验收标准\n- 验收标准一\n- 验收标准二\n\n## 参考文档\n参考正文-不应注入。\n")
	prompt := checkPrompt(t, root)
	for _, want := range []string{"验收标准一", "验收标准二", "第3行-规范内容", "check diet"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("check 提示词缺少 %q\n全文：\n%s", want, prompt)
		}
	}
	for _, notWant := range []string{"目标正文", "需求正文", "参考正文", "设计正文", "执行计划正文"} {
		if strings.Contains(prompt, notWant) {
			t.Errorf("check 提示词不应含 %q\n全文：\n%s", notWant, prompt)
		}
	}
}

// TestSubagentContext_CheckDietFallback：prd 无验收标准节时取需求节；
// 两者都没有时全文兜底。
func TestSubagentContext_CheckDietFallback(t *testing.T) {
	root, _ := checkDietProject(t, "# 演示任务\n\n## 目标\n目标正文-不应注入。\n\n## 需求\n需求正文-需求节。\n")
	prompt := checkPrompt(t, root)
	if !strings.Contains(prompt, "需求正文-需求节") {
		t.Errorf("无验收标准节时应取需求节，得到：\n%s", prompt)
	}
	if strings.Contains(prompt, "目标正文") {
		t.Errorf("取需求节时不应含目标节，得到：\n%s", prompt)
	}

	root2, _ := checkDietProject(t, "# 演示任务\n\n## 目标\n目标正文-全文兜底。\n")
	prompt2 := checkPrompt(t, root2)
	if !strings.Contains(prompt2, "目标正文-全文兜底") {
		t.Errorf("无验收标准与需求节时应全文兜底，得到：\n%s", prompt2)
	}
}

// TestSubagentContext_InjectionOrder：注入顺序——稳定在前、易变在后。
// implement / check / finish 提示词里，固定 Workflow / Constraints 应
// 在派发任务（Your Task）之前；research 的 Workflow / Report Format
// 也应在 Your Task 之前；Codex 的 Curated Context 应在 Active task 行之前。
func TestSubagentContext_InjectionOrder(t *testing.T) {
	before := func(t *testing.T, prompt, stable, volatile string) {
		t.Helper()
		i, j := strings.Index(prompt, stable), strings.Index(prompt, volatile)
		if i < 0 || j < 0 || i > j {
			t.Errorf("%q 应在 %q 之前\n全文：\n%s", stable, volatile, prompt)
		}
	}
	impl := buildImplementPrompt("实现演示功能", "规范上下文")
	before(t, impl, "## Your Context", "## Your Task")
	before(t, impl, "## Workflow", "## Your Task")
	before(t, impl, "## Important Constraints", "## Your Task")

	fin := buildFinishPrompt("收尾复核", "检查上下文")
	before(t, fin, "## Workflow", "## Your Task")
	before(t, fin, "## Important Constraints", "## Your Task")

	res := buildResearchPrompt("查一下接口", "规范目录树")
	before(t, res, "## Workflow", "## Your Task")
	before(t, res, "## Report Format", "## Your Task")

	// check：端到端，确认真实注入的提示词也是这个顺序。
	root, _ := makeProject(t)
	check := checkPrompt(t, root)
	before(t, check, "## Workflow", "## Your Task")
	before(t, check, "## Important Constraints", "## Your Task")

	// Codex：Curated Context 在前，Active task 行在后。
	codex := buildCodexSubagentContext("sunshine-check", ".sunshine/tasks/11-09-demo", "规范上下文")
	before(t, codex, "## Curated Context", "Active task:")
}
