package codegraph

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHasExistingCode(t *testing.T) {
	t.Run("空目录不是存量", func(t *testing.T) {
		if HasExistingCode(t.TempDir()) {
			t.Error("空目录不应判定为存量")
		}
	})
	t.Run("只有文档不是存量", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "README.md"))
		if HasExistingCode(dir) {
			t.Error("只有 README 不应判定为存量")
		}
	})
	t.Run("根目录清单文件算存量", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"))
		if !HasExistingCode(dir) {
			t.Error("go.mod 应判定为存量")
		}
	})
	t.Run("嵌套源码算存量", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "src", "app", "main.ts"))
		if !HasExistingCode(dir) {
			t.Error("嵌套 .ts 源码应判定为存量")
		}
	})
	t.Run("node_modules 里的不算", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"))
		if HasExistingCode(dir) {
			t.Error("node_modules 内的源码不应判定为存量")
		}
	})
	t.Run("隐藏目录里的不算", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".git", "hooks", "x.go"))
		writeFile(t, filepath.Join(dir, ".claude", "x.go"))
		if HasExistingCode(dir) {
			t.Error("隐藏目录内的源码不应判定为存量")
		}
	})
}

func TestTargetsFor(t *testing.T) {
	got := TargetsFor([]string{"claude-code", "dsh", "reasonix", "codex", "claude-code", "copilot"})
	want := []string{"claude", "dsh", "codex", "copilot-cli"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("TargetsFor = %v, want %v", got, want)
	}
	if got := TargetsFor([]string{"zcode", "trae"}); len(got) != 0 {
		t.Errorf("无映射平台应返回空, got %v", got)
	}
}

// fakeRunner 记录调用并按脚本应答。
type fakeRunner struct {
	paths   map[string]string // LookPath 结果
	graphAt string            // Provision 后应存在的图目录（由 init 动作创建）
	calls   []string
	failOn  string // 名字含该子串的 Run 返回错误
	events  *[]string
	onRun   func(call string)
}

func (f *fakeRunner) LookPath(name string) (string, bool) {
	p, ok := f.paths[name]
	return p, ok
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.events != nil {
		*f.events = append(*f.events, "R:"+call)
	}
	if f.onRun != nil {
		f.onRun(call)
	}
	if f.failOn != "" && strings.Contains(name+" "+strings.Join(args, " "), f.failOn) {
		return "", errors.New("boom")
	}
	// 模拟 codegraph init 建图、安装脚本装好工具。
	if len(args) > 0 && args[0] == "init" && f.graphAt != "" {
		_ = os.MkdirAll(f.graphAt, 0o755)
	}
	if name == "sh" || name == "npm" || name == "powershell" {
		if f.paths == nil {
			f.paths = map[string]string{}
		}
		f.paths["codegraph"] = "/fake/bin/codegraph"
	}
	return "", nil
}

func legacyOpts(dir string) Options {
	return Options{Dir: dir, Legacy: true, AllowInstall: true, Platforms: []string{"claude-code", "codex"}}
}

func TestProvisionNotApplicable(t *testing.T) {
	r := &fakeRunner{}
	o := legacyOpts(t.TempDir())
	o.Disabled = true
	if res := Provision(context.Background(), o, r); res.Applicable || len(r.calls) != 0 {
		t.Errorf("禁用时不应供给: %+v calls=%v", res, r.calls)
	}
	// 工具已在 + 新项目：无事可做（图等有代码后再说）。
	r2 := &fakeRunner{paths: map[string]string{"codegraph": "/bin/codegraph"}}
	if res := Provision(context.Background(), Options{Dir: t.TempDir()}, r2); res.Applicable || len(r2.calls) != 0 || len(res.Lines) != 0 {
		t.Errorf("工具已在的新项目应零动作: %+v calls=%v", res, r2.calls)
	}
}

// TestProvisionFindsToolOutsidePATH 工具装在 ~/.local/bin 但不在 PATH 里时
// 必须认出已装：只看 PATH 会误判没装、每次 init 重问重下（真实报障）。
func TestProvisionFindsToolOutsidePATH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "codegraph"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 存量项目、图已在、接线按 PlatformsInstalled=0/Force=false 不触发：
	// 认出已装就应是“已就绪跳过”，而不是走安装问询路径留提示行。
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{paths: map[string]string{}} // PATH 里没有 codegraph
	res := Provision(context.Background(), legacyOpts(dir), r)
	joined := strings.Join(res.Lines, "")
	if strings.Contains(joined, "未装") || strings.Contains(joined, "安装失败") {
		t.Errorf("PATH 外的已装工具被误判：%v", res.Lines)
	}
	if len(r.calls) != 0 {
		t.Errorf("已装已建图不应有外部调用：%v", r.calls)
	}
}

// TestProvisionNewProjectInstallsToolOnly 新项目同意装工具时只装工具：
// 不接线、不建图（没代码可建），留一行说明图等有代码后自动补。
func TestProvisionNewProjectInstallsToolOnly(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	old := selfInstallImpl
	defer func() { selfInstallImpl = old }()
	selfInstallImpl = func(context.Context, io.Writer) error {
		binDir := filepath.Join(home, ".local", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(binDir, "codegraph"), []byte("#!/bin/sh\n"), 0o755)
	}
	r := &fakeRunner{paths: map[string]string{"sh": "/bin/sh"}}
	o := Options{Dir: dir, Legacy: false, AllowInstall: true, Platforms: []string{"claude-code"}}
	res := Provision(context.Background(), o, r)
	if len(r.calls) != 0 {
		t.Errorf("新项目装完工具不应接线/建图：%v", r.calls)
	}
	if !res.Acted {
		t.Error("装工具应算动作")
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "有代码后") {
		t.Errorf("应说明图等有代码后自动补：%v", res.Lines)
	}
}

func TestProvisionAllReady(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{paths: map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"}}
	res := Provision(context.Background(), legacyOpts(dir), r)
	if !res.Applicable || res.Acted || len(r.calls) != 0 {
		t.Errorf("全就绪应零动作: %+v calls=%v", res, r.calls)
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "已就绪") {
		t.Errorf("应提示已就绪: %v", res.Lines)
	}
}

func TestProvisionInstallsAndBuilds(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{
		paths:   map[string]string{"sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
	}
	res := Provision(context.Background(), legacyOpts(dir), r)
	if !res.Acted {
		t.Fatalf("应有动作: %+v", res)
	}
	joined := strings.Join(r.calls, "\n")
	if !strings.Contains(joined, "install.sh") {
		t.Errorf("应跑官方安装脚本: %v", r.calls)
	}
	if !strings.Contains(joined, "install --target=claude,codex --yes") {
		t.Errorf("应接线 claude,codex: %v", r.calls)
	}
	if !strings.Contains(joined, "init --yes") {
		t.Errorf("应建图: %v", r.calls)
	}
}

func TestProvisionBuildOnly(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{
		paths:   map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
	}
	res := Provision(context.Background(), legacyOpts(dir), r)
	joined := strings.Join(r.calls, "\n")
	if strings.Contains(joined, "install.sh") {
		t.Errorf("工具已装不应重装: %v", r.calls)
	}
	if !strings.Contains(joined, "install --target=claude,codex --yes") || !strings.Contains(joined, "init --yes") {
		t.Errorf("缺图时应接线并建图: %v", r.calls)
	}
	if !res.Acted {
		t.Error("应有动作")
	}
}

func TestProvisionWireOnNewPlatform(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{paths: map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"}}
	o := legacyOpts(dir)
	o.PlatformsInstalled = 1
	res := Provision(context.Background(), o, r)
	if len(r.calls) != 1 || !strings.Contains(r.calls[0], "install --target=") {
		t.Errorf("新装平台时应只补接线: %+v calls=%v", res, r.calls)
	}
}

func TestProvisionInstallFailureSoft(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{paths: map[string]string{"sh": "/bin/sh"}, failOn: "install.sh"}
	res := Provision(context.Background(), legacyOpts(dir), r)
	if !res.Applicable {
		t.Error("应进入供给流程")
	}
	joined := strings.Join(r.calls, "\n")
	if strings.Contains(joined, "init --yes") {
		t.Errorf("安装失败后不应继续建图: %v", r.calls)
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "安装失败") {
		t.Errorf("应给出安装失败提示: %v", res.Lines)
	}
}

// Force（手动触发）时工具与图都就绪也要重新接线一次。
func TestProvisionForceWires(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{paths: map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"}}
	o := legacyOpts(dir)
	o.Force = true
	res := Provision(context.Background(), o, r)
	if !res.Acted || len(r.calls) != 1 || !strings.Contains(r.calls[0], "install --target=claude,codex") {
		t.Errorf("Force 应只接线一次: %+v calls=%v", res, r.calls)
	}
}

// TestProvisionProgressBeforeSteps 每步开始前必须先有进度提示：
// 提示与外部命令调用交错记录，断言每类命令之前都出现过对应提示。
func TestProvisionProgressBeforeSteps(t *testing.T) {
	dir := t.TempDir()
	var events []string
	r := &fakeRunner{
		paths:   map[string]string{"sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
		events:  &events,
	}
	o := legacyOpts(dir)
	o.Progress = func(line string) { events = append(events, "P:"+line) }
	res := Provision(context.Background(), o, r)
	if !res.Acted {
		t.Fatalf("应有动作: %+v", res)
	}
	sawInstallHint, sawWireHint, sawBuildHint := false, false, false
	for _, ev := range events {
		switch {
		case strings.HasPrefix(ev, "P:") && strings.Contains(ev, "安装"):
			sawInstallHint = true
		case strings.HasPrefix(ev, "P:") && strings.Contains(ev, "接进所选平台"):
			sawWireHint = true
		case strings.HasPrefix(ev, "P:") && strings.Contains(ev, "建代码图谱"):
			sawBuildHint = true
		case strings.HasPrefix(ev, "R:") && strings.Contains(ev, "install.sh"):
			if !sawInstallHint {
				t.Errorf("跑安装脚本前应先出安装提示，事件序：%v", events)
			}
		case strings.HasPrefix(ev, "R:") && strings.Contains(ev, "install --target="):
			if !sawWireHint {
				t.Errorf("接线前应先出接线提示，事件序：%v", events)
			}
		case strings.HasPrefix(ev, "R:") && strings.Contains(ev, "init --yes"):
			if !sawBuildHint {
				t.Errorf("建图前应先出建图提示，事件序：%v", events)
			}
		}
	}
	if !sawInstallHint || !sawWireHint || !sawBuildHint {
		t.Errorf("三步提示应齐全：install=%v wire=%v build=%v，事件：%v", sawInstallHint, sawWireHint, sawBuildHint, events)
	}
}

// TestProvisionCancelAfterInstall 装工具途中被取消：不再接线、不建图，
// 结果行里有取消说明。
func TestProvisionCancelAfterInstall(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	r := &fakeRunner{
		paths:   map[string]string{"sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
		onRun: func(call string) {
			if strings.Contains(call, "install.sh") {
				cancel()
			}
		},
	}
	res := Provision(ctx, legacyOpts(dir), r)
	if len(r.calls) != 1 {
		t.Errorf("取消后应只跑过安装脚本，calls=%v", r.calls)
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "已取消") {
		t.Errorf("应给出取消行：%v", res.Lines)
	}
}

// TestProvisionCancelDuringWire 接线途中被取消：不再建图。
func TestProvisionCancelDuringWire(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	r := &fakeRunner{
		paths:   map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
		onRun: func(call string) {
			if strings.Contains(call, "install --target=") {
				cancel()
			}
		},
	}
	res := Provision(ctx, legacyOpts(dir), r)
	joined := strings.Join(r.calls, "\n")
	if strings.Contains(joined, "init --yes") {
		t.Errorf("取消后不应再建图：%v", r.calls)
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "已取消") {
		t.Errorf("应给出取消行：%v", res.Lines)
	}
}

// TestStreamingRunnerPassesOutputThrough 流式 Runner 把子进程的
// stdout 与 stderr 都实时直通给定的写入端，返回值不再带输出。
func TestStreamingRunnerPassesOutputThrough(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("用 sh 模拟子进程，Windows 跳过")
	}
	var buf bytes.Buffer
	out, err := StreamingRunner(&buf).Run(context.Background(), t.TempDir(), "sh", "-c", "echo out-line; echo err-line 1>&2")
	if err != nil {
		t.Fatalf("Run 出错：%v", err)
	}
	if out != "" {
		t.Errorf("流式模式返回值应为空，实际 %q", out)
	}
	if !strings.Contains(buf.String(), "out-line") || !strings.Contains(buf.String(), "err-line") {
		t.Errorf("stdout/stderr 都应直通，实际 %q", buf.String())
	}
}

// TestDefaultRunnerCaptures 默认 Runner 仍捕获输出、不直通。
func TestDefaultRunnerCaptures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("用 sh 模拟子进程，Windows 跳过")
	}
	out, err := DefaultRunner().Run(context.Background(), t.TempDir(), "sh", "-c", "echo captured-line")
	if err != nil {
		t.Fatalf("Run 出错：%v", err)
	}
	if !strings.Contains(out, "captured-line") {
		t.Errorf("默认模式应返回捕获的输出，实际 %q", out)
	}
}

// TestProvisionNoConsentNoInstall 未同意装工具时：零外部调用、
// 不动手，只留一行指向手动命令的提示（A 方案）。
func TestProvisionNoConsentNoInstall(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{paths: map[string]string{"sh": "/bin/sh"}}
	o := legacyOpts(dir)
	o.AllowInstall = false
	res := Provision(context.Background(), o, r)
	if len(r.calls) != 0 {
		t.Errorf("未同意不应跑任何外部命令，calls=%v", r.calls)
	}
	if res.Acted {
		t.Error("未同意不应有动作")
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "sunshine codegraph install") {
		t.Errorf("应给出手动安装提示行：%v", res.Lines)
	}
}

// TestProvisionBuildWithoutInstallConsent 工具已装时建图与接线
// 不受装工具同意限制，照旧自动（只写项目内与平台配置）。
func TestProvisionBuildWithoutInstallConsent(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{
		paths:   map[string]string{"codegraph": "/bin/codegraph", "sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
	}
	o := legacyOpts(dir)
	o.AllowInstall = false
	res := Provision(context.Background(), o, r)
	joined := strings.Join(r.calls, "\n")
	if !strings.Contains(joined, "install --target=") || !strings.Contains(joined, "init --yes") {
		t.Errorf("工具已装时应自动接线并建图：%v", r.calls)
	}
	if !res.Acted {
		t.Error("应有动作")
	}
}

// 单测默认禁用自带安装（它会真走网络与真实主目录），
// 需要测自带安装本身的用例在 install_test.go 里单独恢复真实实现。
func init() {
	selfInstallImpl = func(context.Context, io.Writer) error {
		return errors.New("单测桩：自带安装不可用")
	}
}

// TestProvisionSelfInstallSuccess 自带安装走通时不再跑官方脚本，
// 直接用装好的二进制继续接线、建图。
func TestProvisionSelfInstallSuccess(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	old := selfInstallImpl
	defer func() { selfInstallImpl = old }()
	selfInstallImpl = func(context.Context, io.Writer) error {
		binDir := filepath.Join(home, ".local", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(binDir, "codegraph"), []byte("#!/bin/sh\n"), 0o755)
	}
	r := &fakeRunner{
		paths:   map[string]string{"sh": "/bin/sh"},
		graphAt: filepath.Join(dir, ".codegraph"),
	}
	res := Provision(context.Background(), legacyOpts(dir), r)
	joined := strings.Join(r.calls, "\n")
	if strings.Contains(joined, "install.sh") {
		t.Errorf("自带安装成功后不应再跑官方脚本：%v", r.calls)
	}
	if !strings.Contains(strings.Join(res.Lines, ""), "已安装 CodeGraph") {
		t.Errorf("应报告已安装：%v", res.Lines)
	}
	if !strings.Contains(joined, "init --yes") {
		t.Errorf("装完应继续建图：%v", r.calls)
	}
}

// TestProvisionDeclinedSilent 已拒绝过的项目：静默跳过，
// 不出提示行、不跑任何命令（问一次口径）。
func TestProvisionDeclinedSilent(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{paths: map[string]string{"sh": "/bin/sh"}}
	o := legacyOpts(dir)
	o.AllowInstall = false
	o.InstallDeclined = true
	res := Provision(context.Background(), o, r)
	if len(res.Lines) != 0 || res.Acted || len(r.calls) != 0 {
		t.Errorf("已拒绝应完全静默：lines=%v acted=%v calls=%v", res.Lines, res.Acted, r.calls)
	}
}
