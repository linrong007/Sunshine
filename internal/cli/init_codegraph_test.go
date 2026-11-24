package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// copyTestBinary 把测试桩二进制复制到不含 sunshine-test-bin 标记的路径，
// 使代码地图供给在子进程里不被测试保护跳过。
func copyTestBinary(t *testing.T) string {
	t.Helper()
	src := buildTestBinary(t)
	dst := filepath.Join(t.TempDir(), "sunshine")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

// stubCodegraph 在 PATH 前段放一个假 codegraph：记录每次调用，
// init 子命令时在当前目录建 .codegraph/。返回调用日志路径。
func stubCodegraph(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"$@\" >> \"" + log + "\"\nif [ \"$1\" = \"init\" ]; then mkdir -p .codegraph; fi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "codegraph"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func runInitCopy(t *testing.T, bin, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"init"}, args...)...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("运行 init 失败：%v", err)
		}
	}
	return out.String(), code
}

func readStubLog(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func legacyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/demo\n\ngo 1.27\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	return dir
}

func TestInitCodegraphProvisioning(t *testing.T) {
	bin := copyTestBinary(t)

	t.Run("存量项目接线并建图", func(t *testing.T) {
		log := stubCodegraph(t)
		dir := legacyFixture(t)
		out, code := runInitCopy(t, bin, dir, "-y", "-u", "测试", "--claude")
		if code != 0 {
			t.Fatalf("init 退出码 %d，输出：%s", code, out)
		}
		calls := readStubLog(t, log)
		if !strings.Contains(calls, "install --target=claude --yes") {
			t.Errorf("应接线 claude，调用记录：%q", calls)
		}
		if !strings.Contains(calls, "init --yes") {
			t.Errorf("应建图，调用记录：%q", calls)
		}
		if _, err := os.Stat(filepath.Join(dir, ".codegraph")); err != nil {
			t.Errorf("图目录应已建：%v", err)
		}
		if !strings.Contains(out, "代码地图") {
			t.Errorf("输出应含代码地图行：\n%s", out)
		}
	})

	t.Run("空项目不触发", func(t *testing.T) {
		log := stubCodegraph(t)
		dir := t.TempDir()
		_, code := runInitCopy(t, bin, dir, "-y", "-u", "测试", "--claude")
		if code != 0 {
			t.Fatalf("init 退出码 %d", code)
		}
		if calls := readStubLog(t, log); calls != "" {
			t.Errorf("空项目不应调用 codegraph，记录：%q", calls)
		}
	})

	t.Run("--no-codegraph 跳过", func(t *testing.T) {
		log := stubCodegraph(t)
		dir := legacyFixture(t)
		_, code := runInitCopy(t, bin, dir, "-y", "-u", "测试", "--claude", "--no-codegraph")
		if code != 0 {
			t.Fatalf("init 退出码 %d", code)
		}
		if calls := readStubLog(t, log); calls != "" {
			t.Errorf("--no-codegraph 不应调用 codegraph，记录：%q", calls)
		}
	})

	t.Run("二次 init 全就绪零调用", func(t *testing.T) {
		log := stubCodegraph(t)
		dir := legacyFixture(t)
		if _, code := runInitCopy(t, bin, dir, "-y", "-u", "测试", "--claude"); code != 0 {
			t.Fatalf("首次 init 退出码 %d", code)
		}
		if err := os.WriteFile(log, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		out, code := runInitCopy(t, bin, dir, "-y", "-u", "测试", "--claude")
		if code != 0 {
			t.Fatalf("二次 init 退出码 %d，输出：%s", code, out)
		}
		if calls := readStubLog(t, log); calls != "" {
			t.Errorf("全就绪时不应再调用 codegraph，记录：%q", calls)
		}
		if !strings.Contains(out, "已就绪") {
			t.Errorf("二次 init 应提示代码地图已就绪：\n%s", out)
		}
	})
}

// TestCodegraphInstallConsent 装工具同意判定：只有「需要装 + 交互 +
// 用户点头」才同意；-y、非交互、用不上装、提问出错都不装。
func TestCodegraphInstallConsent(t *testing.T) {
	yes := func() (bool, error) { return true, nil }
	no := func() (bool, error) { return false, nil }
	boom := func() (bool, error) { return false, errors.New("interrupt") }
	never := func() (bool, error) { t.Error("不该提问"); return false, nil }
	cases := []struct {
		name         string
		needsInstall bool
		yesFlag      bool
		isTTY        bool
		ask          func() (bool, error)
		want         bool
	}{
		{"交互点头", true, false, true, yes, true},
		{"交互摇头", true, false, true, no, false},
		{"提问出错按不同意", true, false, true, boom, false},
		{"-y 不问不装", true, true, true, never, false},
		{"非交互不问不装", true, false, false, never, false},
		{"用不上装不问", false, false, true, never, false},
	}
	for _, c := range cases {
		if got := codegraphInstallConsent(c.needsInstall, c.yesFlag, c.isTTY, c.ask); got != c.want {
			t.Errorf("%s：got %v, want %v", c.name, got, c.want)
		}
	}
}
