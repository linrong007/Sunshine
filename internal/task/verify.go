package task

// 待验单（verify）：任务做完时，有验收条目在本任务内验不了（要等后续
// 功能接上才能验的集成级条目），不许直接归档——开一张待验单，任务状态
// 转 verifying（待验），条目逐条记在 task.json 的 meta.verify_items 里，
// 并在任务目录生成人读的 verify.md 单子（由命令维护，不要手改）。
//
// 条目的终点只有三种：验通签字（passed）、用户签字作废（waived）、或
// 一直未销（pending / failed / unverifiable）。archive 命令核单：自己
// 有未销条目、名下子任务有未销条目、或有条目指名等本任务验而未签，
// 一律拒绝归档。签字必须带证据与签字任务，不许凭感觉签。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileVerifySlip 是任务目录里待验单的人读文件名（命令生成维护）。
const FileVerifySlip = "verify.md"

// 待验条目的状态值。
const (
	VerifyPending      = "pending"      // 待验
	VerifyPassed       = "passed"       // 验通
	VerifyFailed       = "failed"       // 验不过
	VerifyUnverifiable = "unverifiable" // 仍无法验
	VerifyWaived       = "waived"       // 作废（用户签字）
)

// VerifyItem 是待验单的一条欠账。
type VerifyItem struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Standard string `json:"standard"`
	State    string `json:"state"`
	Verifier string `json:"verifier,omitempty"` // 指名的验证任务（目录名）
	SignedBy string `json:"signed_by,omitempty"`
	Evidence string `json:"evidence,omitempty"`
	Note     string `json:"note,omitempty"` // 作废原因等
	Date     string `json:"date,omitempty"`
}

// Resolved 表示条目已终结（验通或作废），不再挡归档。
func (it VerifyItem) Resolved() bool {
	return it.State == VerifyPassed || it.State == VerifyWaived
}

// VerifyStateLabel 是条目状态的中文显示。
func VerifyStateLabel(state string) string {
	switch state {
	case VerifyPending:
		return "待验"
	case VerifyPassed:
		return "验通"
	case VerifyFailed:
		return "验不过"
	case VerifyUnverifiable:
		return "仍无法验"
	case VerifyWaived:
		return "作废"
	}
	return state
}

// ParseVerifyResult 把签字结论（中文或英文）解析为状态值。
func ParseVerifyResult(s string) (string, bool) {
	switch strings.TrimSpace(s) {
	case "验通", "passed", "pass":
		return VerifyPassed, true
	case "验不过", "failed", "fail":
		return VerifyFailed, true
	case "仍无法验", "unverifiable":
		return VerifyUnverifiable, true
	}
	return "", false
}

// verifyItemsFromMeta 从 meta map 里解出待验条目（形状不对时返回 nil）。
func verifyItemsFromMeta(m map[string]any) []VerifyItem {
	raw, ok := m["verify_items"]
	if !ok || raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var items []VerifyItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

// VerifyItems 取记录里的待验条目。
func (r *TaskRecord) VerifyItems() []VerifyItem {
	return verifyItemsFromMeta(r.MetaMap())
}

// verifyItemsFromRaw 从原始 task.json 对象里解出待验条目（归档等命令
// 走 rawObject 路径时用；meta 缺失/形状不对返回 nil）。
func verifyItemsFromRaw(obj *rawObject) []VerifyItem {
	rawMeta, ok := obj.vals["meta"]
	if !ok {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(rawMeta, &m); err != nil || m == nil {
		return nil
	}
	return verifyItemsFromMeta(m)
}

// setMetaRaw 把 meta 里指定键覆盖为任意 JSON 值，其余键保留，
// 键按字母序稳定写回（与 SetMetaValue 同一编码口径）。
func (r *TaskRecord) setMetaRaw(key string, value any) {
	m := r.MetaMap()
	m[key] = value
	r.setMetaRawMap(m)
}

// SetVerifyItems 把待验条目写回记录的 meta（空时删键，保持 task.json 干净）。
func (r *TaskRecord) SetVerifyItems(items []VerifyItem) {
	if len(items) == 0 {
		m := r.MetaMap()
		if _, ok := m["verify_items"]; !ok {
			return
		}
		delete(m, "verify_items")
		r.setMetaRawMap(m)
		return
	}
	r.setMetaRaw("verify_items", items)
}

// setMetaRawMap 用整个 map 覆盖 Meta（键按字母序）。
func (r *TaskRecord) setMetaRawMap(m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	buf.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			buf.WriteString(", ")
		}
		kb, _ := encodeJSONValue(k)
		vb, _ := encodeJSONValue(m[k])
		buf.Write(kb)
		buf.WriteString(": ")
		buf.Write(vb)
	}
	buf.WriteString("}")
	r.Meta = json.RawMessage(buf.String())
}

// UnresolvedVerifyItems 过滤出未销条目（待验 / 验不过 / 仍无法验）。
func UnresolvedVerifyItems(items []VerifyItem) []VerifyItem {
	var out []VerifyItem
	for _, it := range items {
		if !it.Resolved() {
			out = append(out, it)
		}
	}
	return out
}

// RenderVerifySlip 渲染任务目录里的人读单子 verify.md。
func RenderVerifySlip(rec *TaskRecord) string {
	items := rec.VerifyItems()
	var b strings.Builder
	b.WriteString("# 待验单\n\n")
	b.WriteString("本文件由 sunshine 的 verify 命令生成维护，记录本任务欠下的、要在后续任务里验通的集成级验收条目。不要手改：立单用 verify-open、签字用 verify-sign、作废用 verify-waive、改派用 verify-assign。\n\n")
	fmt.Fprintf(&b, "任务：%s（%s）\n", rec.Title, rec.Name)
	fmt.Fprintf(&b, "状态：%s\n\n", rec.Status)
	if len(items) == 0 {
		b.WriteString("（暂无条目）\n")
		return b.String()
	}
	for _, it := range items {
		fmt.Fprintf(&b, "## %s %s\n\n", it.ID, it.Item)
		fmt.Fprintf(&b, "- 验过标准：%s\n", it.Standard)
		fmt.Fprintf(&b, "- 状态：%s\n", VerifyStateLabel(it.State))
		if it.Verifier != "" {
			fmt.Fprintf(&b, "- 指名验证任务：%s\n", it.Verifier)
		}
		if it.SignedBy != "" {
			fmt.Fprintf(&b, "- 签字任务：%s\n", it.SignedBy)
		}
		if it.Evidence != "" {
			fmt.Fprintf(&b, "- 证据：%s\n", it.Evidence)
		}
		if it.Note != "" {
			fmt.Fprintf(&b, "- 备注：%s\n", it.Note)
		}
		if it.Date != "" {
			fmt.Fprintf(&b, "- 日期：%s\n", it.Date)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// writeVerifySlip 把单子文件同步到任务目录（无条目时删掉旧单子）。
func writeVerifySlip(taskDir string, rec *TaskRecord) error {
	path := filepath.Join(taskDir, FileVerifySlip)
	if len(rec.VerifyItems()) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeFileAtomic(path, []byte(RenderVerifySlip(rec)), 0o644)
}

// loadVerifyTask 解析任务并读记录（verify 命令共用前置）。
func (r *Runner) loadVerifyTask(dir string) (string, *TaskRecord, int) {
	if !r.requireRoot() {
		return "", nil, 1
	}
	taskDir := r.resolveOrError(dir)
	if taskDir == "" {
		return "", nil, 1
	}
	if !r.requireActiveTaskDir(taskDir) {
		return "", nil, 1
	}
	rec, err := LoadRecord(taskDir)
	if err != nil {
		r.errf("读取任务 %s 的 task.json 失败：%v", dir, err)
		return "", nil, 1
	}
	return taskDir, rec, 0
}

// saveVerifyTask 写回记录并同步单子文件。
func (r *Runner) saveVerifyTask(taskDir string, rec *TaskRecord) int {
	if err := WriteRecord(taskDir, rec); err != nil {
		r.errf("写入 task.json 失败：%v", err)
		return 1
	}
	if err := writeVerifySlip(taskDir, rec); err != nil {
		r.errf("写入 %s 失败：%v", FileVerifySlip, err)
		return 1
	}
	return 0
}

// findVerifyItem 按编号找条目下标（找不到返回 -1）。
func findVerifyItem(items []VerifyItem, id string) int {
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

// VerifyShow 显示一个任务的待验单。
func (r *Runner) VerifyShow(dir string, asJSON bool) int {
	_, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	items := rec.VerifyItems()
	if asJSON {
		if items == nil {
			items = []VerifyItem{}
		}
		printJSON(r.out(), map[string]any{
			"task": rec.Name, "title": rec.Title, "status": rec.Status,
			"items": items, "unresolved": len(UnresolvedVerifyItems(items)),
		})
		return 0
	}
	if len(items) == 0 {
		fmt.Fprintf(r.out(), "%s 没有待验单。\n", rec.Name)
		return 0
	}
	fmt.Fprintf(r.out(), "待验单：%s（%s），状态 %s，未销 %d / %d 条\n", rec.Title, rec.Name, rec.Status, len(UnresolvedVerifyItems(items)), len(items))
	for _, it := range items {
		fmt.Fprintf(r.out(), "  - %s [%s] %s\n", it.ID, VerifyStateLabel(it.State), it.Item)
		fmt.Fprintf(r.out(), "      标准：%s\n", it.Standard)
		if it.Verifier != "" {
			fmt.Fprintf(r.out(), "      指名验证：%s\n", it.Verifier)
		}
		if it.SignedBy != "" {
			fmt.Fprintf(r.out(), "      签字：%s（%s）证据：%s\n", it.SignedBy, it.Date, it.Evidence)
		}
		if it.Note != "" {
			fmt.Fprintf(r.out(), "      备注：%s\n", it.Note)
		}
	}
	return 0
}

// VerifyOpen 立单：把 check 对账落下的「待集成验证」条目记成待验单，
// 任务状态转 verifying。items 与 standards 按下标配对，数量必须相等。
// verifier 非空时给全部新条目指名验证任务（须是另一个活动任务）。
func (r *Runner) VerifyOpen(dir string, items, standards []string, verifier string) int {
	taskDir, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	if len(items) == 0 {
		r.errf("至少要有一条待验条目（--item）")
		return 1
	}
	if len(items) != len(standards) {
		r.errf("--item 与 --standard 数量必须相等（各 %d / %d）", len(items), len(standards))
		return 1
	}
	if rec.Status != "in_progress" && rec.Status != "verifying" {
		r.errf("拒绝立单：任务 %s 当前状态为 %q，只有 in_progress（已做完、check 之后）或 verifying（追加条目）的任务能立单", rec.Name, rec.Status)
		return 1
	}
	verifierName := ""
	if strings.TrimSpace(verifier) != "" {
		vDir, err := FindTaskByName(verifier, TasksDir(r.Root))
		if err != nil || vDir == "" {
			r.errf("指名验证任务 %q 不是活动任务", verifier)
			return 1
		}
		verifierName = filepath.Base(vDir)
		if verifierName == filepath.Base(taskDir) {
			r.errf("指名验证任务不能是任务自己（本任务内验不了才立单）")
			return 1
		}
	}
	existing := rec.VerifyItems()
	maxID := 0
	for _, it := range existing {
		var n int
		if _, err := fmt.Sscanf(it.ID, "V-%d", &n); err == nil && n > maxID {
			maxID = n
		}
	}
	for i, text := range items {
		maxID++
		existing = append(existing, VerifyItem{
			ID:       fmt.Sprintf("V-%d", maxID),
			Item:     text,
			Standard: standards[i],
			State:    VerifyPending,
			Verifier: verifierName,
		})
	}
	rec.SetVerifyItems(existing)
	rec.Status = "verifying"
	if code := r.saveVerifyTask(taskDir, rec); code != 0 {
		return code
	}
	fmt.Fprintf(r.out(), "已立单：%s 转待验（verifying），新增 %d 条（共 %d 条未销）。单子见 %s。\n", rec.Name, len(items), len(UnresolvedVerifyItems(existing)), FileVerifySlip)
	fmt.Fprintln(r.out(), "这些条目验通签字（verify-sign）或用户作废（verify-waive）之前，本任务不能归档。")
	return 0
}

// VerifyAssign 给一条未销条目指名/改派验证任务。
func (r *Runner) VerifyAssign(dir, itemID, toTask string) int {
	taskDir, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	items := rec.VerifyItems()
	idx := findVerifyItem(items, itemID)
	if idx < 0 {
		r.errf("任务 %s 的待验单里没有条目 %s", rec.Name, itemID)
		return 1
	}
	if items[idx].Resolved() {
		r.errf("条目 %s 已%s，不用改派", itemID, VerifyStateLabel(items[idx].State))
		return 1
	}
	vDir, err := FindTaskByName(toTask, TasksDir(r.Root))
	if err != nil || vDir == "" {
		r.errf("验证任务 %q 不是活动任务", toTask)
		return 1
	}
	vName := filepath.Base(vDir)
	if vName == filepath.Base(taskDir) {
		r.errf("验证任务不能是任务自己")
		return 1
	}
	items[idx].Verifier = vName
	rec.SetVerifyItems(items)
	if code := r.saveVerifyTask(taskDir, rec); code != 0 {
		return code
	}
	fmt.Fprintf(r.out(), "已改派：%s 的 %s 指名由 %s 验证。\n", rec.Name, itemID, vName)
	return 0
}

// VerifySign 签字：验证任务的 check 对一条代验条目给出结论。
// result 为 验通 / 验不过 / 仍无法验；证据必填，不许凭感觉签。
func (r *Runner) VerifySign(dir, itemID, result, byTask, evidence string) int {
	taskDir, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	state, ok := ParseVerifyResult(result)
	if !ok {
		r.errf("签字结论只能是 验通 / 验不过 / 仍无法验，收到 %q", result)
		return 1
	}
	if strings.TrimSpace(evidence) == "" {
		r.errf("签字必须带证据（--evidence：跑通的链路 / 集成测试 / 真实输出摘要）")
		return 1
	}
	items := rec.VerifyItems()
	idx := findVerifyItem(items, itemID)
	if idx < 0 {
		r.errf("任务 %s 的待验单里没有条目 %s", rec.Name, itemID)
		return 1
	}
	if items[idx].State == VerifyWaived {
		r.errf("条目 %s 已作废，不能签字；要恢复用 verify-reopen 重开", itemID)
		return 1
	}
	bDir, err := FindTaskByName(byTask, TasksDir(r.Root))
	if err != nil || bDir == "" {
		r.errf("签字任务 %q 不是活动任务", byTask)
		return 1
	}
	items[idx].State = state
	items[idx].SignedBy = filepath.Base(bDir)
	items[idx].Evidence = evidence
	items[idx].Date = r.today()
	rec.SetVerifyItems(items)
	if code := r.saveVerifyTask(taskDir, rec); code != 0 {
		return code
	}
	fmt.Fprintf(r.out(), "已签字：%s 的 %s —— %s（签字任务 %s）。\n", rec.Name, itemID, VerifyStateLabel(state), items[idx].SignedBy)
	if rest := len(UnresolvedVerifyItems(items)); rest == 0 {
		fmt.Fprintf(r.out(), "%s 的待验单已签满，可以归档了（先归档本任务，再归档验证任务）。\n", rec.Name)
	} else {
		fmt.Fprintf(r.out(), "%s 的待验单还剩 %d 条未销。\n", rec.Name, rest)
	}
	return 0
}

// VerifyWaive 作废一条条目：需求砍掉等情形，用户批准后由主会话执行，
// 必须记原因。作废是终点之一，条目不再挡归档。
func (r *Runner) VerifyWaive(dir, itemID, reason string) int {
	taskDir, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	if strings.TrimSpace(reason) == "" {
		r.errf("作废必须记原因（--reason，如「需求变更 2026-11-26：该功能已砍」）")
		return 1
	}
	items := rec.VerifyItems()
	idx := findVerifyItem(items, itemID)
	if idx < 0 {
		r.errf("任务 %s 的待验单里没有条目 %s", rec.Name, itemID)
		return 1
	}
	if items[idx].Resolved() {
		r.errf("条目 %s 已%s，不能重复作废", itemID, VerifyStateLabel(items[idx].State))
		return 1
	}
	items[idx].State = VerifyWaived
	items[idx].Note = reason
	items[idx].Date = r.today()
	rec.SetVerifyItems(items)
	if code := r.saveVerifyTask(taskDir, rec); code != 0 {
		return code
	}
	fmt.Fprintf(r.out(), "已作废：%s 的 %s（%s）。\n", rec.Name, itemID, reason)
	if rest := len(UnresolvedVerifyItems(items)); rest == 0 {
		fmt.Fprintf(r.out(), "%s 的待验单已无未销条目，可以归档了。\n", rec.Name)
	}
	return 0
}

// VerifyReopen 重开一条已终结的条目（需求变更把已签/已作废条目的
// 前提改掉时用）：状态回待验，清掉签字信息，任务状态回 verifying。
func (r *Runner) VerifyReopen(dir, itemID, reason string) int {
	taskDir, rec, code := r.loadVerifyTask(dir)
	if code != 0 {
		return code
	}
	items := rec.VerifyItems()
	idx := findVerifyItem(items, itemID)
	if idx < 0 {
		r.errf("任务 %s 的待验单里没有条目 %s", rec.Name, itemID)
		return 1
	}
	if !items[idx].Resolved() {
		r.errf("条目 %s 当前是%s，本来就未销，不用重开", itemID, VerifyStateLabel(items[idx].State))
		return 1
	}
	items[idx].State = VerifyPending
	items[idx].SignedBy = ""
	items[idx].Evidence = ""
	items[idx].Date = ""
	if strings.TrimSpace(reason) != "" {
		items[idx].Note = reason
	}
	rec.SetVerifyItems(items)
	if rec.Status != "verifying" {
		rec.Status = "verifying"
	}
	if code := r.saveVerifyTask(taskDir, rec); code != 0 {
		return code
	}
	fmt.Fprintf(r.out(), "已重开：%s 的 %s 回到待验，任务状态 verifying。\n", rec.Name, itemID)
	return 0
}

// verifyArchiveGates 是 archive 的待验单硬拦（在归档动手改任何东西
// 之前调用）：① 本任务有未销条目；② 名下仍活动的子任务有未销条目；
// ③ 有条目指名等本任务验证而未销。任一命中，列全后拒绝（返回 1）。
func (r *Runner) verifyArchiveGates(taskDir, dirName string) int {
	tasksDir := TasksDir(r.Root)
	var blockers []string
	if obj, _, err := readRawObject(filepath.Join(taskDir, FileTaskJSON)); err == nil {
		for _, it := range UnresolvedVerifyItems(verifyItemsFromRaw(obj)) {
			blockers = append(blockers, fmt.Sprintf("本任务待验单未销：%s %s（%s）", it.ID, it.Item, VerifyStateLabel(it.State)))
		}
		for _, childName := range obj.getStringList("children") {
			childDir, err := FindTaskByName(childName, tasksDir)
			if err != nil || childDir == "" {
				continue
			}
			cObj, _, err := readRawObject(filepath.Join(childDir, FileTaskJSON))
			if err != nil {
				continue
			}
			for _, it := range UnresolvedVerifyItems(verifyItemsFromRaw(cObj)) {
				blockers = append(blockers, fmt.Sprintf("子任务 %s 待验单未销：%s %s", childName, it.ID, it.Item))
			}
		}
	}
	for _, t := range IterActiveTasks(tasksDir, fileWriter(r.errOut())) {
		if t.DirName == dirName {
			continue
		}
		tObj, _, err := readRawObject(filepath.Join(t.Dir, FileTaskJSON))
		if err != nil {
			continue
		}
		for _, it := range verifyItemsFromRaw(tObj) {
			if it.Verifier == dirName && !it.Resolved() {
				blockers = append(blockers, fmt.Sprintf("任务 %s 的 %s 指名等本任务验证：%s", t.DirName, it.ID, it.Item))
			}
		}
	}
	if len(blockers) == 0 {
		return 0
	}
	r.errf("拒绝归档 %s：待验单未清零", dirName)
	for _, b := range blockers {
		fmt.Fprintf(r.errOut(), "  - %s\n", b)
	}
	fmt.Fprintln(r.errOut(), "先把这些条目验通签字（sunshine task verify-sign）或经用户批准作废（sunshine task verify-waive），再归档。")
	return 1
}

// rewriteVerifyRefsAfterRename 在任务改名后，把其他活动任务待验单里
// 指向旧名的 verifier / signed_by 改成新名，并重渲染受影响的单子。
func (r *Runner) rewriteVerifyRefsAfterRename(oldName, newName string) {
	tasksDir := TasksDir(r.Root)
	for _, t := range IterActiveTasks(tasksDir, fileWriter(r.errOut())) {
		jsonPath := filepath.Join(t.Dir, FileTaskJSON)
		obj, _, err := readRawObject(jsonPath)
		if err != nil {
			continue
		}
		items := verifyItemsFromRaw(obj)
		changed := false
		for i := range items {
			if items[i].Verifier == oldName {
				items[i].Verifier = newName
				changed = true
			}
			if items[i].SignedBy == oldName {
				items[i].SignedBy = newName
				changed = true
			}
		}
		if !changed {
			continue
		}
		rawMeta, ok := obj.vals["meta"]
		if !ok {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(rawMeta, &m); err != nil || m == nil {
			continue
		}
		m["verify_items"] = items
		obj.set("meta", m)
		if err := writeRawObject(jsonPath, obj); err != nil {
			r.warnf("改名后更新 %s 的待验单引用失败：%v", t.DirName, err)
			continue
		}
		if rec, err := LoadRecord(t.Dir); err == nil {
			_ = writeVerifySlip(t.Dir, rec)
		}
	}
}

// VerifySummaryAll 扫全部活动任务，返回有未销条目的任务及其未销条目
// （会话注入与清点共用）。读不出的任务跳过（其他命令会各自报错）。
func (r *Runner) VerifySummaryAll() []VerifyTaskSummary {
	tasksDir := TasksDir(r.Root)
	var out []VerifyTaskSummary
	for _, t := range IterActiveTasks(tasksDir, fileWriter(r.errOut())) {
		rec, err := LoadRecord(t.Dir)
		if err != nil {
			continue
		}
		un := UnresolvedVerifyItems(rec.VerifyItems())
		if len(un) > 0 {
			out = append(out, VerifyTaskSummary{Name: t.DirName, Title: rec.Title, Status: rec.Status, Unresolved: un})
		}
	}
	return out
}

// VerifyTaskSummary 是一个任务的未销待验概览。
type VerifyTaskSummary struct {
	Name       string
	Title      string
	Status     string
	Unresolved []VerifyItem
}
