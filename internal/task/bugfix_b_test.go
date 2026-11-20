package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// B1/B2 回归测试（bug 修复 B 队）。

// runWithTimeout 在超时内跑完 fn 并返回其退出码；超时判失败，
// 防止手改父子环时 list/context 无限递归把整个测试套件挂死。
func runWithTimeout(t *testing.T, fn func() int) int {
	t.Helper()
	done := make(chan int, 1)
	go func() { done <- fn() }()
	select {
	case code := <-done:
		return code
	case <-time.After(15 * time.Second):
		t.Fatal("操作超时（疑似父子环无限递归）")
		return -1
	}
}

func strPtrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func strVal(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// setTaskFields 直接改 task.json 字段，模拟用户手改数据造环。
func setTaskFields(t *testing.T, dir string, mutate func(obj *rawObject) error) {
	t.Helper()
	if err := updateTaskJSON(dir, mutate); err != nil {
		t.Fatalf("手改 task.json 失败：%v", err)
	}
}

func taskDirOf(root, name string) string {
	return filepath.Join(root, DirSunshine, DirTasks, name)
}

// TestLinkSubtask_SelfCycleRejected B1：add-subtask A A 自环必须被拒。
func TestLinkSubtask_SelfCycleRejected(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	a := createTask(t, r, "任务甲")
	errBuf.Reset()
	if code := r.AddSubtask(a, a); code == 0 {
		t.Fatalf("add-subtask 自环应被拒绝，实际成功；err=%s", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "环") {
		t.Errorf("自环报错应说明成环，实际：%s", errBuf.String())
	}
	rec, err := LoadRecord(taskDirOf(root, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Children) != 0 || rec.Parent != nil {
		t.Errorf("自环被拒后不应写入链接：children=%v parent=%v", rec.Children, rec.Parent)
	}
}

// TestLinkSubtask_CycleRejected B1：A->B 后再 B->A 成环必须被拒。
func TestLinkSubtask_CycleRejected(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	a := createTask(t, r, "任务甲")
	b := createTask(t, r, "任务乙")
	if code := r.AddSubtask(a, b); code != 0 {
		t.Fatalf("正常 add-subtask 应成功，code=%d err=%s", code, errBuf.String())
	}
	errBuf.Reset()
	if code := r.AddSubtask(b, a); code == 0 {
		t.Fatalf("add-subtask 成环（B->A）应被拒绝，实际成功；err=%s", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "环") {
		t.Errorf("成环报错应说明是成环，实际：%s", errBuf.String())
	}
	recA, err := LoadRecord(taskDirOf(root, a))
	if err != nil {
		t.Fatal(err)
	}
	if recA.Parent != nil {
		t.Errorf("成环被拒后 A 的 parent 应仍为空，实际 %v", *recA.Parent)
	}
	recB, err := LoadRecord(taskDirOf(root, b))
	if err != nil {
		t.Fatal(err)
	}
	if len(recB.Children) != 0 {
		t.Errorf("成环被拒后 B 的 children 应仍为空，实际 %v", recB.Children)
	}
}

// TestListContext_HandmadeCycle_B1 B1：手改 task.json 造出的环
// （子任务自指、父子互环无根）下，list/context 不能挂死、不能藏任务。
func TestListContext_HandmadeCycle_B1(t *testing.T) {
	// 场景一：根 P 的子任务 C 的 children 含 C 自身（自环）。
	root := newTestRoot(t)
	r, out, _ := newTestRunner(t, root)
	p := createTask(t, r, "根任务")
	c := createTask(t, r, "自环子任务")
	if code := r.AddSubtask(p, c); code != 0 {
		t.Fatalf("add-subtask failed: %d", code)
	}
	setTaskFields(t, taskDirOf(root, c), func(obj *rawObject) error {
		obj.setStringList("children", []string{c})
		return nil
	})
	out.Reset()
	if code := runWithTimeout(t, func() int { return r.List(false, "", false) }); code != 0 {
		t.Fatalf("list with self-cycle failed: %d", code)
	}
	if !strings.Contains(out.String(), "总计：2 个任务") {
		t.Errorf("自环时 list 应列出全部 2 个任务，实际：%s", out.String())
	}
	if !strings.Contains(out.String(), "已访问") {
		t.Errorf("自环时 list 应打印已访问警告行，实际：%s", out.String())
	}
	out.Reset()
	if code := runWithTimeout(t, func() int { return r.Context("default", "", "", false) }); code != 0 {
		t.Fatalf("context with self-cycle failed: %d", code)
	}
	if !strings.Contains(out.String(), "总计：2 个活动任务") || !strings.Contains(out.String(), "已访问") {
		t.Errorf("自环时 context 应列出全部任务并带警告，实际：%s", out.String())
	}

	// 场景二：A、B 互为父子（无根可达），list/context 不能把任务藏掉。
	root2 := newTestRoot(t)
	r2, out2, _ := newTestRunner(t, root2)
	a := createTask(t, r2, "互环甲")
	b := createTask(t, r2, "互环乙")
	if code := r2.AddSubtask(a, b); code != 0 {
		t.Fatalf("add-subtask failed: %d", code)
	}
	setTaskFields(t, taskDirOf(root2, a), func(obj *rawObject) error {
		obj.set("parent", b)
		return nil
	})
	setTaskFields(t, taskDirOf(root2, b), func(obj *rawObject) error {
		obj.setStringList("children", []string{a})
		return nil
	})
	out2.Reset()
	if code := runWithTimeout(t, func() int { return r2.List(false, "", false) }); code != 0 {
		t.Fatalf("list with mutual cycle failed: %d", code)
	}
	if !strings.Contains(out2.String(), "总计：2 个任务") {
		t.Errorf("互环时 list 不应藏任务（总计应为 2），实际：%s", out2.String())
	}
	out2.Reset()
	if code := runWithTimeout(t, func() int { return r2.Context("default", "", "", false) }); code != 0 {
		t.Fatalf("context with mutual cycle failed: %d", code)
	}
	if !strings.Contains(out2.String(), "总计：2 个活动任务") {
		t.Errorf("互环时 context 不应藏任务（总计应为 2），实际：%s", out2.String())
	}
}

// archiveOne 把任务归档并返回归档目录的相对路径（按归档路径传参用）。
func archiveOne(t *testing.T, r *Runner, root, name string) (archRel, archDir string) {
	t.Helper()
	if code := r.Archive(name, true, true); code != 0 {
		t.Fatalf("archive %s failed: %d", name, code)
	}
	matches, err := filepath.Glob(filepath.Join(root, DirSunshine, DirTasks, DirArchive, "*", name))
	if err != nil || len(matches) != 1 {
		t.Fatalf("找不到归档目录：matches=%v err=%v", matches, err)
	}
	archDir = matches[0]
	rel, err := filepath.Rel(root, archDir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel), archDir
}

// TestStart_ArchivedTaskRejected B2：按归档路径 start 必须被拒，
// 且不能把 .current-task 写成归档内路径。
func TestStart_ArchivedTaskRejected(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	name := createTask(t, r, "将归档任务")
	archRel, _ := archiveOne(t, r, root, name)
	errBuf.Reset()
	if code := r.Start(archRel, true); code == 0 {
		t.Fatalf("start 归档任务应被拒绝，实际成功；err=%s", errBuf.String())
	}
	if data, err := os.ReadFile(filepath.Join(root, DirSunshine, ".current-task")); err == nil {
		if strings.Contains(string(data), DirArchive) {
			t.Errorf(".current-task 不应指向归档任务，实际：%s", data)
		}
	}
}

// TestStart_CompletedStatusRejected B2：非 planning/in_progress 状态 start 直接拒绝。
func TestStart_CompletedStatusRejected(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	name := createTask(t, r, "已完成任务")
	setTaskFields(t, taskDirOf(root, name), func(obj *rawObject) error {
		obj.set("status", "completed")
		return nil
	})
	errBuf.Reset()
	if code := r.Start(name, true); code == 0 {
		t.Fatalf("start completed 任务应被拒绝，实际成功；err=%s", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "completed") {
		t.Errorf("拒绝报错应说明当前状态，实际：%s", errBuf.String())
	}
	// in_progress 任务重复 start 仍应成功（状态校验不误伤）。
	name2 := createTask(t, r, "进行中任务")
	if code := r.Start(name2, true); code != 0 {
		t.Fatalf("start planning 任务应成功，code=%d err=%s", code, errBuf.String())
	}
	if code := r.Start(name2, true); code != 0 {
		t.Errorf("start in_progress 任务应成功，code=%d err=%s", code, errBuf.String())
	}
}

// TestWriteOps_ArchivedTaskRejected B2：add-subtask / create --parent /
// set-* / set-meta / add-context 对归档路径均被拒，归档数据不被改写；
// 同批操作对活动任务不受影响。
func TestWriteOps_ArchivedTaskRejected(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	archName := createTask(t, r, "归档父任务")
	archRel, archDir := archiveOne(t, r, root, archName)
	// 写操作前先快照归档记录，事后逐项比对（create 本就会写 base_branch，
	// 不能简单断言为空）。
	beforeRec, err := LoadRecord(archDir)
	if err != nil {
		t.Fatal(err)
	}
	activeParent := createTask(t, r, "活动父任务")
	activeChild := createTask(t, r, "活动子任务")
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("# 笔记\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := map[string]func() int{
		"start":              func() int { return r.Start(archRel, true) },
		"add-subtask 父为归档":   func() int { return r.AddSubtask(archRel, activeChild) },
		"add-subtask 子为归档":   func() int { return r.AddSubtask(activeParent, archRel) },
		"remove-subtask 归档":  func() int { return r.RemoveSubtask(archRel, activeChild) },
		"create --parent 归档": func() int { return r.Create(CreateOptions{Title: "归档之子", Description: "d", Parent: archRel}) },
		"set-branch 归档":      func() int { return r.SetBranch(archRel, "feature/x") },
		"set-base-branch 归档": func() int { return r.SetBaseBranch(archRel, "main") },
		"set-scope 归档":       func() int { return r.SetScope(archRel, "scope") },
		"set-meta 归档":        func() int { return r.SetMeta(archRel, "k", "v") },
		"add-context 归档":     func() int { return r.AddContext(archRel, "implement", "note.md", "理由", "") },
	}
	for label, fn := range checks {
		errBuf.Reset()
		if code := fn(); code == 0 {
			t.Errorf("%s：对归档任务应被拒绝，实际成功；err=%s", label, errBuf.String())
		}
	}
	// 归档任务数据未被任何写操作改动。
	archRec, err := LoadRecord(archDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(archRec.Children) != len(beforeRec.Children) {
		t.Errorf("归档父任务 children 不应被改写：前 %v 后 %v", beforeRec.Children, archRec.Children)
	}
	if !strPtrEq(archRec.Branch, beforeRec.Branch) || !strPtrEq(archRec.BaseBranch, beforeRec.BaseBranch) ||
		!strPtrEq(archRec.Scope, beforeRec.Scope) || string(archRec.Meta) != string(beforeRec.Meta) {
		t.Errorf("归档任务字段不应被改写：branch %v→%v base %v→%v scope %v→%v meta %s→%s",
			strVal(beforeRec.Branch), strVal(archRec.Branch), strVal(beforeRec.BaseBranch), strVal(archRec.BaseBranch),
			strVal(beforeRec.Scope), strVal(archRec.Scope), beforeRec.Meta, archRec.Meta)
	}
	if _, err := os.Stat(filepath.Join(archDir, "implement.jsonl")); !os.IsNotExist(err) {
		t.Errorf("归档任务不应被 add-context 写入 implement.jsonl（err=%v）", err)
	}
	if _, err := os.Stat(taskDirOf(root, "归档之子")); err == nil {
		t.Error("create --parent 指向归档任务时不应创建出新任务")
	}
	entries, _ := filepath.Glob(filepath.Join(root, DirSunshine, DirTasks, "*-归档之子"))
	if len(entries) != 0 {
		t.Errorf("create --parent 指向归档任务时不应创建出新任务，实际 %v", entries)
	}

	// 活动任务不受影响：同批写操作全部应成功。
	if code := r.AddSubtask(activeParent, activeChild); code != 0 {
		t.Errorf("活动任务 add-subtask 应成功，code=%d err=%s", code, errBuf.String())
	}
	if code := r.Create(CreateOptions{Title: "活动之子", Description: "d", Parent: activeParent}); code != 0 {
		t.Errorf("活动任务 create --parent 应成功，code=%d err=%s", code, errBuf.String())
	}
	if code := r.SetBranch(activeChild, "feature/ok"); code != 0 {
		t.Errorf("活动任务 set-branch 应成功，code=%d", code)
	}
	if code := r.SetMeta(activeChild, "k", "v"); code != 0 {
		t.Errorf("活动任务 set-meta 应成功，code=%d", code)
	}
	if code := r.AddContext(activeChild, "implement", "note.md", "理由", ""); code != 0 {
		t.Errorf("活动任务 add-context 应成功，code=%d err=%s", code, errBuf.String())
	}
	if code := r.Start(activeChild, true); code != 0 {
		t.Errorf("活动任务 start 应成功，code=%d err=%s", code, errBuf.String())
	}
}
