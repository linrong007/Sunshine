package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startTask 建任务并 start 到 in_progress（verify 立单的前置状态）。
func startTask(t *testing.T, r *Runner, title string) string {
	t.Helper()
	name := createTask(t, r, title)
	if code := r.Start(name, true); code != 0 {
		t.Fatalf("start %s failed: %d", name, code)
	}
	return name
}

func TestVerifyOpenAndSlip(t *testing.T) {
	root := newTestRoot(t)
	r, out, _ := newTestRunner(t, root)
	verifier := startTask(t, r, "登录页")
	name := startTask(t, r, "地基")

	// planning 状态不能立单（还没做完、没 check）。
	plan := createTask(t, r, "还在规划")
	if code := r.VerifyOpen(plan, []string{"条目"}, []string{"标准"}, ""); code == 0 {
		t.Errorf("verify-open on planning task should fail")
	}
	// item 与 standard 数量不等拒绝。
	if code := r.VerifyOpen(name, []string{"a", "b"}, []string{"只有一条标准"}, ""); code == 0 {
		t.Errorf("verify-open with mismatched item/standard counts should fail")
	}
	// 验证任务不能是自己。
	if code := r.VerifyOpen(name, []string{"a"}, []string{"s"}, name); code == 0 {
		t.Errorf("verify-open with self as verifier should fail")
	}

	out.Reset()
	if code := r.VerifyOpen(name, []string{"登录后会话保持"}, []string{"真实登录后刷新仍保持登录"}, verifier); code != 0 {
		t.Fatalf("verify-open failed: %d", code)
	}
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	rec, err := LoadRecord(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "verifying" {
		t.Errorf("status = %q, want verifying", rec.Status)
	}
	items := rec.VerifyItems()
	if len(items) != 1 || items[0].ID != "V-1" || items[0].State != VerifyPending || items[0].Verifier != verifier {
		t.Errorf("items = %+v", items)
	}
	slip, err := os.ReadFile(filepath.Join(taskDir, FileVerifySlip))
	if err != nil {
		t.Fatalf("verify.md missing: %v", err)
	}
	if !strings.Contains(string(slip), "登录后会话保持") || !strings.Contains(string(slip), "待验单") {
		t.Errorf("slip content wrong:\n%s", slip)
	}
	// list 显形未销条数。
	out.Reset()
	if code := r.List(false, "", false); code != 0 {
		t.Fatalf("list failed: %d", code)
	}
	if !strings.Contains(out.String(), "[待验：未销 1 条]") {
		t.Errorf("list should show pending verify tag, got:\n%s", out.String())
	}
	// JSON 带 verify_pending。
	out.Reset()
	if code := r.List(false, "", true); code != 0 {
		t.Fatalf("list --json failed: %d", code)
	}
	if !strings.Contains(out.String(), "\"verify_pending\": 1") {
		t.Errorf("list json should carry verify_pending=1, got:\n%s", out.String())
	}
}

func TestVerifyArchiveGatesAndSignFlow(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	verifier := startTask(t, r, "登录页")
	name := startTask(t, r, "地基")
	if code := r.VerifyOpen(name, []string{"会话保持"}, []string{"登录后刷新仍登录"}, verifier); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)

	// 自己的单未销：归档拒绝且目录不动。
	errBuf.Reset()
	if code := r.Archive(name, true, true); code == 0 {
		t.Fatalf("archive with unresolved slip should fail")
	}
	if !strings.Contains(errBuf.String(), "待验单未清零") {
		t.Errorf("archive refusal should mention slip, got: %s", errBuf.String())
	}
	if _, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir should stay active after refused archive: %v", err)
	}
	// 验证者先归档：被指名未签，拒绝。
	if code := r.Archive(verifier, true, true); code == 0 {
		t.Errorf("archive of designated verifier should fail while items unsigned")
	}
	// 签字校验：坏结论、无证据都拒绝。
	if code := r.VerifySign(name, "V-1", "差不多", verifier, "证据"); code == 0 {
		t.Errorf("sign with bad result should fail")
	}
	if code := r.VerifySign(name, "V-1", "验通", verifier, ""); code == 0 {
		t.Errorf("sign without evidence should fail")
	}
	// 仍无法验：签了也不销，归档仍拒绝。
	if code := r.VerifySign(name, "V-1", "仍无法验", verifier, "环境缺短信网关，登录链路跑不通"); code != 0 {
		t.Fatalf("sign unverifiable: %d", code)
	}
	if code := r.Archive(name, true, true); code == 0 {
		t.Errorf("archive should still fail after unverifiable sign")
	}
	// 验通：先归档待验任务，再归档验证者，都放行。
	if code := r.VerifySign(name, "V-1", "验通", verifier, "真机登录后刷新仍保持登录，集成测试通过"); code != 0 {
		t.Fatalf("sign passed: %d", code)
	}
	if code := r.Archive(name, true, true); code != 0 {
		t.Errorf("archive after full sign-off should pass")
	}
	if code := r.Archive(verifier, true, true); code != 0 {
		t.Errorf("verifier archive after sign-off should pass")
	}
}

func TestVerifyFailedReworkAndSelfVerify(t *testing.T) {
	root := newTestRoot(t)
	r, _, _ := newTestRunner(t, root)
	verifier := startTask(t, r, "登录页")
	name := startTask(t, r, "地基")
	if code := r.VerifyOpen(name, []string{"拦截未登录"}, []string{"未登录访问受保护页被重定向"}, verifier); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	if code := r.VerifySign(name, "V-1", "验不过", verifier, "未登录直接打开了受保护页"); code != 0 {
		t.Fatalf("sign failed: %d", code)
	}
	if code := r.Archive(name, true, true); code == 0 {
		t.Errorf("archive should fail after failed sign")
	}
	// 返工：verifying 能 start 回 in_progress。
	if code := r.Start(name, true); code != 0 {
		t.Fatalf("start (rework) from verifying failed: %d", code)
	}
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	rec, err := LoadRecord(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "in_progress" {
		t.Errorf("after rework start status = %q, want in_progress", rec.Status)
	}
	// 返工后自验签字（下游已存在），归档放行。
	if code := r.VerifySign(name, "V-1", "验通", name, "返工后自测：未登录访问被重定向到登录页"); code != 0 {
		t.Fatalf("self sign after rework: %d", code)
	}
	if code := r.Archive(name, true, true); code != 0 {
		t.Errorf("archive after rework sign-off should pass")
	}
}

func TestVerifyWaiveAndReopen(t *testing.T) {
	root := newTestRoot(t)
	r, _, _ := newTestRunner(t, root)
	name := startTask(t, r, "地基")
	if code := r.VerifyOpen(name,
		[]string{"条目一", "条目二"}, []string{"标准一", "标准二"}, ""); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	// 作废必须记原因。
	if code := r.VerifyWaive(name, "V-2", ""); code == 0 {
		t.Errorf("waive without reason should fail")
	}
	if code := r.VerifyWaive(name, "V-2", "需求变更：该功能已砍"); code != 0 {
		t.Fatalf("waive: %d", code)
	}
	verifier := startTask(t, r, "登录页")
	if code := r.VerifySign(name, "V-1", "验通", verifier, "集成测试通过"); code != 0 {
		t.Fatalf("sign: %d", code)
	}
	// 全销后重开一条：状态回 verifying，归档重新被拦。
	if code := r.VerifyReopen(name, "V-1", "需求变更：标准改了"); code != 0 {
		t.Fatalf("reopen: %d", code)
	}
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	rec, err := LoadRecord(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "verifying" {
		t.Errorf("status after reopen = %q, want verifying", rec.Status)
	}
	if code := r.Archive(name, true, true); code == 0 {
		t.Errorf("archive should fail after reopen")
	}
	// 已作废条目不能签字。
	if code := r.VerifySign(name, "V-2", "验通", verifier, "证据"); code == 0 {
		t.Errorf("sign on waived item should fail")
	}
}

func TestVerifyParentSweep(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	parent := createTask(t, r, "父任务")
	child := startTask(t, r, "地基子任务")
	if code := r.AddSubtask(parent, child); code != 0 {
		t.Fatalf("add-subtask: %d", code)
	}
	if code := r.VerifyOpen(child, []string{"条目"}, []string{"标准"}, ""); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	errBuf.Reset()
	if code := r.Archive(parent, true, true); code == 0 {
		t.Fatalf("parent archive should fail while child slip unresolved")
	}
	if !strings.Contains(errBuf.String(), "子任务") || !strings.Contains(errBuf.String(), "待验单未销") {
		t.Errorf("parent refusal should list child slip, got: %s", errBuf.String())
	}
	verifier := startTask(t, r, "功能子任务")
	if code := r.VerifySign(child, "V-1", "验通", verifier, "集成验证通过"); code != 0 {
		t.Fatalf("sign: %d", code)
	}
	if code := r.Archive(parent, true, true); code != 0 {
		t.Errorf("parent archive after child sign-off should pass")
	}
}

func TestVerifyRemoveSubtaskGate(t *testing.T) {
	root := newTestRoot(t)
	r, _, _ := newTestRunner(t, root)
	parent := createTask(t, r, "父任务")
	childA := startTask(t, r, "地基子任务")
	childB := startTask(t, r, "登录子任务")
	if code := r.AddSubtask(parent, childA); code != 0 {
		t.Fatalf("add-subtask A: %d", code)
	}
	if code := r.AddSubtask(parent, childB); code != 0 {
		t.Fatalf("add-subtask B: %d", code)
	}
	if code := r.VerifyOpen(childA, []string{"条目"}, []string{"标准"}, childB); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	// B 被指名为验证者：摘除拒绝。
	if code := r.RemoveSubtask(parent, childB); code == 0 {
		t.Fatalf("remove-subtask of designated verifier should fail")
	}
	// 改派给父任务后，摘除放行。
	if code := r.VerifyAssign(childA, "V-1", parent); code != 0 {
		t.Fatalf("verify-assign: %d", code)
	}
	if code := r.RemoveSubtask(parent, childB); code != 0 {
		t.Errorf("remove-subtask after reassign should pass")
	}
}

func TestVerifyRenamePropagation(t *testing.T) {
	root := newTestRoot(t)
	r, _, _ := newTestRunner(t, root)
	verifier := startTask(t, r, "登录页")
	name := startTask(t, r, "地基")
	if code := r.VerifyOpen(name, []string{"条目"}, []string{"标准"}, verifier); code != 0 {
		t.Fatalf("verify-open: %d", code)
	}
	newName := "11-09-登录页改版"
	if code := r.Rename(verifier, "登录页改版", false); code != 0 {
		t.Fatalf("rename: %d", code)
	}
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	rec, err := LoadRecord(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	items := rec.VerifyItems()
	if len(items) != 1 || items[0].Verifier != newName {
		t.Errorf("verifier ref after rename = %+v, want %s", items, newName)
	}
	// 改名后的验证者归档仍被指名拦住（引用跟随生效）。
	if code := r.Archive(newName, true, true); code == 0 {
		t.Errorf("archive of renamed verifier should still be blocked")
	}
}
