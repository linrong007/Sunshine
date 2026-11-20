// Package codegraph 把 CodeGraph 代码地图浅集成进 sunshine init：
// 只针对存量项目，工具、项目图、平台接线三层都是缺什么补什么，
// 全程失败只警告、不阻断 init。CodeGraph 本体装到用户机器上，
// 不打进 Sunshine 发布包；图的同步与升级归 CodeGraph 自己。
package codegraph

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Runner 抽象外部命令执行，便于单测注入假实现。
type Runner interface {
	LookPath(name string) (string, bool)
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
}

type execRunner struct{}

// DefaultRunner 返回真实执行外部命令的 Runner。
func DefaultRunner() Runner { return execRunner{} }

func (execRunner) LookPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	return p, err == nil
}

func (execRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// manifestNames 是命中即判定存量项目的清单文件名。
var manifestNames = map[string]bool{
	"go.mod": true, "package.json": true, "Cargo.toml": true,
	"pom.xml": true, "build.gradle": true, "build.gradle.kts": true,
	"settings.gradle": true, "pyproject.toml": true, "requirements.txt": true,
	"setup.py": true, "Pipfile": true, "composer.json": true,
	"Gemfile": true, "mix.exs": true, "pubspec.yaml": true,
	"CMakeLists.txt": true,
}

// sourceExts 是命中即判定存量项目的源码扩展名。
var sourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".py": true, ".java": true, ".kt": true,
	".kts": true, ".rs": true, ".c": true, ".h": true, ".cpp": true,
	".hpp": true, ".cc": true, ".cs": true, ".php": true, ".rb": true,
	".swift": true, ".vue": true, ".svelte": true, ".scala": true,
	".lua": true, ".dart": true, ".ex": true, ".exs": true, ".erl": true,
	".zig": true, ".nim": true,
}

// skipDirs 是存量检测不进入的目录名（另有隐藏目录一律跳过，根目录除外）。
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".next": true, ".venv": true, "venv": true,
	"__pycache__": true,
}

// maxWalkEntries 是存量检测的遍历上限，防超大目录拖慢 init。
const maxWalkEntries = 20000

// HasExistingCode 判断目录里是否已有代码（清单文件或源码文件任一命中）。
// 必须在 init 写任何文件之前调用，避免把 Sunshine 自己生成的文件算作存量。
func HasExistingCode(dir string) bool {
	visited := 0
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if found || err != nil {
			if err != nil && d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		visited++
		if visited > maxWalkEntries {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if path != dir {
				name := d.Name()
				if skipDirs[name] || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := d.Name()
		if manifestNames[name] || sourceExts[strings.ToLower(filepath.Ext(name))] ||
			strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".csproj") ||
			strings.HasSuffix(name, ".fsproj") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// targetByPlatform 是 Sunshine 平台 ID 到 CodeGraph installer target 名的映射，
// 已按 CodeGraph 仓库 src/installer/targets 的文件名核对（2026-11-20）。
// 没有对应 target 的平台（zcode、trae、omp、snow、reasonix、kilo、devin、
// codebuddy）不在表里，靠 CLI 孪生命令与模板规矩覆盖。
var targetByPlatform = map[string]string{
	"claude-code": "claude",
	"cursor":      "cursor",
	"codex":       "codex",
	"kiro":        "kiro",
	"gemini":      "gemini",
	"qoder":       "qoder",
	"pi":          "pi",
	"opencode":    "opencode",
	"dsh":         "dsh",
	"grok":        "grok",
	"kimi":        "kimi",
	"antigravity": "antigravity",
	"copilot":     "copilot-cli",
	"droid":       "droid",
}

// TargetsFor 把 Sunshine 平台 ID 列表映射为 CodeGraph target 列表，
// 保持输入顺序并去重，未映射的平台直接丢弃。
func TargetsFor(platformIDs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range platformIDs {
		t, ok := targetByPlatform[id]
		if !ok || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// Options 是 Provision 的输入。
type Options struct {
	Dir                string   // 项目目录
	Legacy             bool     // 存量项目（init 写文件之前判定）
	Disabled           bool     // --no-codegraph 或 SUNSHINE_NO_CODEGRAPH
	Platforms          []string // 本次 init 选中的 Sunshine 平台 ID
	PlatformsInstalled int      // 本次 init 实际新装的平台数
}

// Result 是 Provision 的结果：Lines 给用户看，Acted 表示是否动过外部状态。
type Result struct {
	Applicable bool
	Acted      bool
	Lines      []string
}

// Provision 按“缺什么补什么”供给 CodeGraph：
// 工具缺了装、项目图缺了建、接线只在刚装工具 / 刚建图 / 本次新装平台时补。
// 任何一步失败都只记警告行并体面收尾，绝不返回错误。
func Provision(ctx context.Context, opts Options, r Runner) Result {
	var res Result
	if !opts.Legacy || opts.Disabled {
		return res
	}
	res.Applicable = true

	bin, cliOK := r.LookPath("codegraph")
	graphOK := dirExists(filepath.Join(opts.Dir, ".codegraph"))

	installedCLI := false
	if !cliOK {
		if p, ok := installCLI(ctx, opts.Dir, r); ok {
			bin, cliOK, installedCLI = p, true, true
			res.Acted = true
			res.Lines = append(res.Lines, "  ✓ 代码地图：已安装 CodeGraph（官方安装脚本）")
		} else {
			res.Lines = append(res.Lines, "  ○ 代码地图：CodeGraph 安装失败，已跳过（可手动安装：curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh）")
			return res
		}
	}

	needWire := installedCLI || !graphOK || opts.PlatformsInstalled > 0
	if targets := TargetsFor(opts.Platforms); len(targets) > 0 && needWire {
		wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_, err := r.Run(wctx, opts.Dir, bin, "install", "--target="+strings.Join(targets, ","), "--yes")
		cancel()
		res.Acted = true
		if err != nil {
			res.Lines = append(res.Lines, "  ○ 代码地图：平台接线失败，已跳过（可手动执行 codegraph install）")
		} else {
			res.Lines = append(res.Lines, "  ✓ 代码地图：已把 CodeGraph 查询接进所选平台")
		}
	}

	if !graphOK {
		gctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_, err := r.Run(gctx, opts.Dir, bin, "init", "--yes")
		cancel()
		res.Acted = true
		if err != nil {
			res.Lines = append(res.Lines, "  ○ 代码地图：建图失败，已跳过（可手动执行 codegraph init）")
		} else {
			res.Lines = append(res.Lines, "  ✓ 代码地图：已为本项目建好代码图谱（.codegraph/）")
		}
	}

	if !res.Acted {
		res.Lines = append(res.Lines, "  ○ 代码地图（CodeGraph）已就绪，跳过")
	}
	return res
}

// installCLI 用官方安装脚本安装 CodeGraph，失败再退 npm 全局安装；
// 装完在 PATH 与常见安装位置里找二进制，返回可用路径。
func installCLI(ctx context.Context, dir string, r Runner) (string, bool) {
	ictx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	installed := false
	if runtime.GOOS == "windows" {
		_, err := r.Run(ictx, dir, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
			"-Command", "irm https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.ps1 | iex")
		installed = err == nil
	} else if _, ok := r.LookPath("sh"); ok {
		_, err := r.Run(ictx, dir, "sh", "-c",
			"curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh")
		installed = err == nil
	}
	if !installed {
		if _, ok := r.LookPath("npm"); ok {
			_, err := r.Run(ictx, dir, "npm", "install", "-g", "@colbymchenry/codegraph")
			installed = err == nil
		}
	}
	if !installed {
		return "", false
	}
	if p, ok := r.LookPath("codegraph"); ok {
		return p, true
	}
	// 安装脚本可能装到当前进程 PATH 之外的位置（如 ~/.local/bin），
	// 本次供给先用绝对路径把后续步骤走完。
	home, err := os.UserHomeDir()
	if err == nil {
		for _, cand := range []string{
			filepath.Join(home, ".local", "bin", "codegraph"),
			filepath.Join(home, "bin", "codegraph"),
			filepath.Join(home, ".codegraph", "bin", "codegraph"),
		} {
			if fileExists(cand) {
				return cand, true
			}
		}
	}
	return "", false
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
