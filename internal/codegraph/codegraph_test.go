package codegraph

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
}

func (f *fakeRunner) LookPath(name string) (string, bool) {
	p, ok := f.paths[name]
	return p, ok
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
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
	return Options{Dir: dir, Legacy: true, Platforms: []string{"claude-code", "codex"}}
}

func TestProvisionNotApplicable(t *testing.T) {
	r := &fakeRunner{}
	if res := Provision(context.Background(), Options{Dir: t.TempDir()}, r); res.Applicable || len(r.calls) != 0 {
		t.Errorf("非存量不应供给: %+v calls=%v", res, r.calls)
	}
	o := legacyOpts(t.TempDir())
	o.Disabled = true
	if res := Provision(context.Background(), o, r); res.Applicable || len(r.calls) != 0 {
		t.Errorf("禁用时不应供给: %+v calls=%v", res, r.calls)
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
