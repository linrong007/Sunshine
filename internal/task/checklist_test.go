package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChecklistOfMixed(t *testing.T) {
	// children 顺序即执行顺序：已归档（不在活动集）计完成。
	statuses := map[string]string{
		"doing":  "in_progress",
		"wait1":  "planning",
		"verify": "verifying",
		"wait2":  "planning",
		"done2":  "completed",
	}
	c := ChecklistOf("parent", []string{"gone", "doing", "wait1", "verify", "wait2", "done2"}, statuses)
	if c.Total != 6 || c.Done != 2 || c.Unfinished() != 4 {
		t.Errorf("counts = %+v, want total 6 done 2 unfinished 4", c)
	}
	if len(c.Doing) != 1 || c.Doing[0] != "doing" {
		t.Errorf("doing = %v", c.Doing)
	}
	// 下一个按 children 顺序取，不按名字排序。
	if c.Next() != "wait1" || len(c.Queued) != 2 {
		t.Errorf("next = %q queued = %v, want wait1 + 2 queued", c.Next(), c.Queued)
	}
	if c.Verifying != 1 {
		t.Errorf("verifying = %d, want 1", c.Verifying)
	}
	line := c.Line()
	for _, want := range []string{"parent：子任务共 6 个，已完成 2 个", "正在做：doing", "待验：1 个", "下一个：wait1（排队共 2 个）"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q missing %q", line, want)
		}
	}
}

func TestChecklistsSkipsFinishedParents(t *testing.T) {
	tasks := []*TaskInfo{
		{DirName: "p-done", Children: []string{"a", "b"}},
		{DirName: "p-open", Children: []string{"c"}},
		{DirName: "a", Status: "completed"},
		{DirName: "b", Status: "completed"},
		{DirName: "c", Status: "planning"},
		{DirName: "flat", Status: "in_progress"},
	}
	cls := Checklists(tasks)
	if len(cls) != 1 || cls[0].Parent != "p-open" {
		t.Fatalf("checklists = %+v, want only p-open", cls)
	}
	if cls[0].Next() != "c" {
		t.Errorf("next = %q, want c", cls[0].Next())
	}
}

// context 的「未完成清单」一节：四种子任务状态混合时逐项对得上；
// 父任务子任务全归档后整节消失。
func TestContextShowsChecklistSection(t *testing.T) {
	root := newTestRoot(t)
	r, out, _ := newTestRunner(t, root)
	parent := createTask(t, r, "父任务")
	doneChild := createTask(t, r, "已完成子任务")
	doingChild := createTask(t, r, "在做子任务")
	verifyChild := createTask(t, r, "待验子任务")
	queuedChild := createTask(t, r, "排队子任务")
	for _, ch := range []string{doneChild, doingChild, verifyChild, queuedChild} {
		if code := r.AddSubtask(parent, ch); code != 0 {
			t.Fatalf("add-subtask %s failed: %d", ch, code)
		}
	}
	if code := r.Start(doingChild, true); code != 0 {
		t.Fatalf("start doing child: %d", code)
	}
	// 待验子任务：做到 in_progress 后立单转 verifying。
	if code := r.Start(verifyChild, true); code != 0 {
		t.Fatalf("start verify child: %d", code)
	}
	if code := r.VerifyOpen(verifyChild, []string{"等下游验"}, []string{"下游接上后真跑通"}, doingChild); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	// 已完成子任务：把目录挪进 archive/ 模拟归档（活动集里消失即计完成）。
	archDir := filepath.Join(root, DirSunshine, DirTasks, "archive", "2026-11")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, DirSunshine, DirTasks, doneChild), filepath.Join(archDir, doneChild)); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if code := r.Context("", "", "", false); code != 0 {
		t.Fatalf("context: %d", code)
	}
	text := out.String()
	idx := strings.Index(text, "## 未完成清单")
	if idx < 0 {
		t.Fatalf("context missing 未完成清单 section:\n%s", text)
	}
	section := text[idx:]
	for _, want := range []string{"子任务共 4 个，已完成 1 个", "正在做：" + doingChild, "待验：1 个", "下一个：" + queuedChild} {
		if !strings.Contains(section, want) {
			t.Errorf("checklist section missing %q:\n%s", want, section)
		}
	}

	// 其余子任务也都挪进 archive 后，整节消失。
	for _, ch := range []string{doingChild, verifyChild, queuedChild} {
		if err := os.Rename(filepath.Join(root, DirSunshine, DirTasks, ch), filepath.Join(archDir, ch)); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	if code := r.Context("", "", "", false); code != 0 {
		t.Fatalf("context after all archived: %d", code)
	}
	if strings.Contains(out.String(), "## 未完成清单") {
		t.Errorf("checklist section should disappear when all children archived:\n%s", out.String())
	}
}
