// RunCompress 实现 sunshine hook compress：运行原命令，压缩
// stdout/stderr，退出码原样返回。
//
// 用法：sunshine hook compress [--timeout <毫秒>] -- <命令>
// 只由 pre-tool-use 改写后的 Bash 命令调用，不直接给 agent 用。
// 退出码是底线：原命令失败几分就返回几分，绝不能像 `| head`
// 那样把失败掩盖成成功。
//
// 与其它 hook 事件不同，本函数返回 int（退出码），调用方
// （internal/cli/hook.go）用 os.Exit 转交——hook 的“永远退出 0”
// 铁律只管 hook 回调本身，不管被包住的命令。
package hook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// 压缩阈值与保留量。
const (
	// passThroughMaxLines / passThroughMaxBytes：不超过这两项时原样通过。
	passThroughMaxLines = 200
	passThroughMaxBytes = 12000
	// headKeepLines / tailKeepLines：压缩时保留的首尾行数。
	headKeepLines = 10
	tailKeepLines = 80
	// maxKeptLines：压缩后总保留行数上限。
	maxKeptLines = 200
)

// failureSubstrings 是失败特征行（小写子串匹配）。
var failureSubstrings = []string{
	"fail", "error", "panic", "exception", "✗", "✘", "not ok",
}

// RunCompress 运行命令，返回应退出的退出码。
// args 是 "compress" 之后的位置参数（cobra 已剥掉 `--`，
// 命令作为一个整体传进来）；timeoutMs 是 --timeout flag 的值，
// 0 表示不设内部超时（由 Bash 工具自己的 timeout 兜底）。
// 无命令时返回 0（静默，无输出）。
func RunCompress(args []string, timeoutMs int, opts Options) int {
	if len(args) == 0 {
		return 0
	}
	cmdStr := strings.Join(args, " ")
	if strings.TrimSpace(cmdStr) == "" {
		return 0
	}
	return runAndCompress(cmdStr, timeoutMs, opts)
}

// runAndCompress 用 bash -c 跑命令，stdout/stderr 分开抓、分别压缩、
// 分别写回，返回原命令的退出码。超时返回 124（timeout 命令惯例），
// 并在 stderr 补一句超时说明。
func runAndCompress(cmdStr string, timeoutMs int, opts Options) int {
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	cmd.Dir = ""
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	_, _ = io.WriteString(opts.out(), compressStream(stdoutBuf.String()))
	_, _ = io.WriteString(opts.errOut(), compressStream(stderrBuf.String()))
	if ctx.Err() == context.DeadlineExceeded {
		_, _ = fmt.Fprintf(opts.errOut(), "【Sunshine：命令超时（%d 毫秒），已终止】\n", timeoutMs)
		return 124
	}
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

// compressStream 压缩单条输出流：不超过阈值时原样返回；
// 否则保留首 headKeepLines 行 + 失败特征行 + 末 tailKeepLines 行
// （总计不超过 maxKeptLines 行），连续重复行折叠，并在行首加
// 截断说明。返回压缩后的文本。
func compressStream(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) <= passThroughMaxLines && len(s) <= passThroughMaxBytes {
		return s
	}
	folded := foldDuplicates(lines)
	kept := selectKeptLines(folded)
	var b strings.Builder
	fmt.Fprintf(&b, "【Sunshine：输出过长（原 %d 行），已压缩为 %d 行：首 %d 行 + 失败行 + 末 %d 行；需要完整输出时在命令后加 | cat】\n",
		len(lines), len(kept), headKeepLines, tailKeepLines)
	prev := -1
	for _, idx := range kept {
		if prev >= 0 && idx > prev+1 {
			fmt.Fprintf(&b, "…（省略 %d 行）…\n", idx-prev-1)
		}
		b.WriteString(folded[idx])
		b.WriteByte('\n')
		prev = idx
	}
	return b.String()
}

// foldDuplicates 折叠连续重复行：同一行连续出现 N 次（N>1）时
// 只留一行并在行尾记 `（×N）`。
func foldDuplicates(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	run := lines[0]
	count := 1
	flush := func() {
		if count > 1 {
			out = append(out, fmt.Sprintf("%s （×%d）", run, count))
		} else {
			out = append(out, run)
		}
	}
	for _, l := range lines[1:] {
		if l == run {
			count++
			continue
		}
		flush()
		run, count = l, 1
	}
	flush()
	return out
}

// selectKeptLines 选出要保留的行下标（已排序、去重、
// 不超过 maxKeptLines 行）：首 headKeepLines 行、全部失败特征行、
// 末 tailKeepLines 行；超限时失败行只取最靠前的部分。
func selectKeptLines(lines []string) []int {
	n := len(lines)
	keep := map[int]bool{}
	head := headKeepLines
	if head > n {
		head = n
	}
	for i := 0; i < head; i++ {
		keep[i] = true
	}
	tail := tailKeepLines
	if tail > n {
		tail = n
	}
	for i := n - tail; i < n; i++ {
		keep[i] = true
	}
	var failures []int
	for i, l := range lines {
		if keep[i] {
			continue
		}
		if isFailureLine(l) {
			failures = append(failures, i)
		}
	}
	room := maxKeptLines - len(keep)
	if room < 0 {
		room = 0
	}
	for _, i := range failures {
		if room <= 0 {
			break
		}
		keep[i] = true
		room--
	}
	idxs := make([]int, 0, len(keep))
	for i := range keep {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	return idxs
}

// isFailureLine 判断一行是否含失败特征（小写子串匹配）。
func isFailureLine(line string) bool {
	lower := strings.ToLower(line)
	for _, p := range failureSubstrings {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
