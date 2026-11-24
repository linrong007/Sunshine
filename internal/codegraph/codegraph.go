// Package codegraph 把 CodeGraph 代码地图浅集成进 sunshine init：
// 只针对存量项目，工具、项目图、平台接线三层都是缺什么补什么，
// 全程失败只警告、不阻断 init。CodeGraph 本体装到用户机器上，
// 不打进 Sunshine 发布包；图的同步与升级归 CodeGraph 自己。
package codegraph

import (
	"context"
	"io"
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

type execRunner struct {
	// live 非 nil 时子进程输出直通该写入端（实时可见），
	// 否则捕获后返回（静默，调用方通常直接丢弃）。
	live io.Writer
}

// DefaultRunner 返回真实执行外部命令的 Runner（输出捕获、不直通）。
func DefaultRunner() Runner { return execRunner{} }

// StreamingRunner 返回输出直通 w 的 Runner：装工具、建图这类
// 可能跑几分钟的外部命令，把子进程自己的进度（下载、索引文件数）
// 实时打到终端上，用户能看见它在动，而不是只等一句开始提示
// （2026-11-24 用户在 Mac 实测 v0.5.51 后追问「能不能看安装的进度」）。
func StreamingRunner(w io.Writer) Runner { return execRunner{live: w} }

func (execRunner) LookPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	return p, err == nil
}

func (r execRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if r.live != nil {
		cmd.Stdout = r.live
		cmd.Stderr = r.live
		err := cmd.Run()
		return "", err
	}
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
	Force              bool     // 手动触发（sunshine codegraph install）：无条件接线
	// AllowInstall 是否允许在本机安装 CodeGraph 工具本体。装工具是
	// 走网络、改用户机器的全局动作，必须有明确同意才做：手动命令
	// 本身即同意；init 由调用方在交互模式问过用户后传入，-y 与
	// 非交互一律 false（只提示不装）。建图与接线只写项目内与平台
	// 配置，不在此限、照旧自动（2026-11-24 用户点选 A 方案）。
	AllowInstall bool
	// Progress 在每个耗时步骤开始前收到一行进度提示（可为 nil）。
	// 供给全程要跑外部命令（装工具最长 5 分钟、建图最长 10 分钟），
	// 结果行只在全部结束后才汇总返回；没有这条预告，用户只会看到
	// 终端长时间无输出、以为卡死（2026-11-24 demo21 实测报障）。
	Progress func(string)
}

// Result 是 Provision 的结果：Lines 给用户看，Acted 表示是否动过外部状态。
type Result struct {
	Applicable bool
	Acted      bool
	Lines      []string
}

// cancelledLine 是用户取消（Ctrl+C）后的统一收尾行。
const cancelledLine = "  ○ 代码地图：已取消，跳过（之后可运行 sunshine codegraph install 补上）"

// Provision 按“缺什么补什么”供给 CodeGraph：
// 工具缺了装、项目图缺了建、接线只在刚装工具 / 刚建图 / 本次新装平台时补。
// 任何一步失败都只记警告行并体面收尾，绝不返回错误。
// ctx 被取消（如调用方接了 Ctrl+C 信号）时，正在跑的外部命令会被
// CommandContext 杀掉，本函数在下一步开始前收尾返回取消行，
// 不让用户只能强杀终端、还看不到卡在哪一步。
func Provision(ctx context.Context, opts Options, r Runner) Result {
	var res Result
	if !opts.Legacy || opts.Disabled {
		return res
	}
	res.Applicable = true
	progress := func(line string) {
		if opts.Progress != nil {
			opts.Progress(line)
		}
	}
	cancelled := func() bool {
		if ctx.Err() != nil {
			res.Lines = append(res.Lines, cancelledLine)
			return true
		}
		return false
	}

	bin, cliOK := r.LookPath("codegraph")
	graphOK := dirExists(filepath.Join(opts.Dir, ".codegraph"))

	installedCLI := false
	if !cliOK && !opts.AllowInstall {
		res.Lines = append(res.Lines, "  ○ 代码地图：本机未装 CodeGraph，已跳过（要装请运行 sunshine codegraph install）")
		return res
	}
	if !cliOK {
		progress("  … 代码地图：本机还没有 CodeGraph，正在用官方脚本安装（可能要几分钟；按 Ctrl+C 可取消，取消不影响 init 其余结果）……")
		if p, ok := installCLI(ctx, opts.Dir, r); ok {
			bin, cliOK, installedCLI = p, true, true
			res.Acted = true
			res.Lines = append(res.Lines, "  ✓ 代码地图：已安装 CodeGraph（官方安装脚本）")
		} else if cancelled() {
			return res
		} else {
			res.Lines = append(res.Lines, "  ○ 代码地图：CodeGraph 安装失败，已跳过（可手动安装：curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh）")
			return res
		}
	}
	if cancelled() {
		return res
	}

	needWire := opts.Force || installedCLI || !graphOK || opts.PlatformsInstalled > 0
	if targets := TargetsFor(opts.Platforms); len(targets) > 0 && needWire {
		progress("  … 代码地图：正在把 CodeGraph 查询接进所选平台（按 Ctrl+C 可取消）……")
		wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_, err := r.Run(wctx, opts.Dir, bin, "install", "--target="+strings.Join(targets, ","), "--yes")
		cancel()
		res.Acted = true
		if cancelled() {
			return res
		}
		if err != nil {
			res.Lines = append(res.Lines, "  ○ 代码地图：平台接线失败，已跳过（可手动执行 codegraph install）")
		} else {
			res.Lines = append(res.Lines, "  ✓ 代码地图：已把 CodeGraph 查询接进所选平台")
		}
	}

	if !graphOK {
		if cancelled() {
			return res
		}
		progress("  … 代码地图：正在为本项目建代码图谱（项目越大越久；按 Ctrl+C 可取消）……")
		gctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_, err := r.Run(gctx, opts.Dir, bin, "init", "--yes")
		cancel()
		res.Acted = true
		if cancelled() {
			return res
		}
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
