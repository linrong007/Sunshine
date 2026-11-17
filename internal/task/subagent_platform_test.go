package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasSubagentPlatformDsh(t *testing.T) {
	root := newTestRoot(t)
	if HasSubagentPlatform(root) {
		t.Errorf("没有任何平台配置目录时不应算子代理平台")
	}
	if err := os.Mkdir(filepath.Join(root, ".dsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !HasSubagentPlatform(root) {
		t.Errorf("装了 DSH 的项目应算子代理平台：DSH 经 subagent 工具派发 implement/check")
	}
}

func TestCreateTaskCreatesJsonlForDsh(t *testing.T) {
	// DSH 有子代理（用户 2026-11-17 在 demo13 实测：主会话创建子智能体做原型），
	// 但 task create 之前不建 implement/check.jsonl——subagentConfigDirs
	// 里一直漏了 .dsh，派发 implement/check 子代理时读不到已整理的上下文。
	root := newTestRoot(t)
	if err := os.Mkdir(filepath.Join(root, ".dsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, _, _ := newTestRunner(t, root)
	name := createTask(t, r, "DSH 任务")
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	for _, f := range []string{"implement.jsonl", "check.jsonl"} {
		if _, err := os.Stat(filepath.Join(taskDir, f)); err != nil {
			t.Errorf("DSH 项目 task create 应建 %s：%v", f, err)
		}
	}
}

func TestCreateTaskNoJsonlWithoutSubagentPlatform(t *testing.T) {
	root := newTestRoot(t)
	r, _, _ := newTestRunner(t, root)
	name := createTask(t, r, "无子代理平台任务")
	taskDir := filepath.Join(root, DirSunshine, DirTasks, name)
	for _, f := range []string{"implement.jsonl", "check.jsonl"} {
		if _, err := os.Stat(filepath.Join(taskDir, f)); err == nil {
			t.Errorf("无子代理平台的项目不应建 %s", f)
		}
	}
}
