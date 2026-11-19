package hook

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCompressStreamPassthrough(t *testing.T) {
	cases := []string{
		"",
		"a\nb\nc\n",
		strings.Repeat("x\n", 200),
	}
	for i, in := range cases {
		if got := compressStream(in); got != in {
			t.Fatalf("case %d: 短输出应原样通过，实际 %q", i, got)
		}
	}
}

func TestCompressStreamTruncate(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	out := compressStream(b.String())
	if !strings.Contains(out, "输出过长") {
		t.Fatal("长输出应带截断说明")
	}
	if !strings.Contains(out, "原 500 行") {
		t.Fatalf("截断说明应写原行数，实际首行: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if !strings.Contains(out, "line 0\n") {
		t.Fatal("应保留首行")
	}
	if !strings.Contains(out, "line 499\n") {
		t.Fatal("应保留末行")
	}
	if !strings.Contains(out, "省略") {
		t.Fatal("应有省略标记")
	}
	lines := strings.Count(out, "\n")
	if lines > 201 { // 标记行 + 最多 200 行
		t.Fatalf("压缩后 %d 行，超过上限", lines)
	}
}

func TestCompressStreamKeepsFailures(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 500; i++ {
		if i == 250 {
			b.WriteString("--- FAIL: TestFoo (0.01s)\n")
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	out := compressStream(b.String())
	if !strings.Contains(out, "--- FAIL: TestFoo") {
		t.Fatal("中间的失败行应被保留")
	}
}

func TestCompressStreamDuplicates(t *testing.T) {
	in := strings.Repeat("same warning\n", 300)
	out := compressStream(in)
	if !strings.Contains(out, "same warning （×300）") {
		t.Fatalf("连续重复行应折叠，实际 %q", out)
	}
}

func TestCompressStreamBytesThreshold(t *testing.T) {
	// 行数不多但单行很长：字节超阈值也应压缩。
	in := strings.Repeat("x", 13000) + "\n"
	out := compressStream(in)
	if !strings.Contains(out, "输出过长") {
		t.Fatal("字节超阈值应压缩")
	}
}

func TestRunCompress(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := Options{Stdout: &stdout, Stderr: &stderr}
	// 注意：真实调用时 cobra 已把 `--` 剥掉，args 里只有命令本身，
	// timeout 由调用方经 --timeout flag 另外传——这里按新签名直调。
	if code := RunCompress([]string{"echo hi"}, 0, opts); code != 0 {
		t.Fatalf("echo hi 退出码 %d", code)
	}
	if stdout.String() != "hi\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunCompress([]string{"exit 3"}, 0, opts); code != 3 {
		t.Fatalf("exit 3 退出码 %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunCompress([]string{"echo out; echo err >&2"}, 0, opts); code != 0 {
		t.Fatalf("退出码 %d", code)
	}
	if stdout.String() != "out\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "err\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}

	// 长输出压缩，退出码不变。
	stdout.Reset()
	stderr.Reset()
	if code := RunCompress([]string{"seq 1 500"}, 0, opts); code != 0 {
		t.Fatalf("seq 退出码 %d", code)
	}
	if !strings.Contains(stdout.String(), "输出过长") {
		t.Fatal("500 行输出应被压缩")
	}
	if !strings.Contains(stdout.String(), "500\n") {
		t.Fatal("应保留末行")
	}

	// 失败行保留。
	stdout.Reset()
	stderr.Reset()
	script := `for i in $(seq 1 300); do if [ $i = 150 ]; then echo "FAIL: test 150"; else echo "line $i"; fi; done`
	if code := RunCompress([]string{script}, 0, opts); code != 0 {
		t.Fatalf("退出码 %d", code)
	}
	if !strings.Contains(stdout.String(), "FAIL: test 150") {
		t.Fatal("失败行应被保留")
	}

	// 超时返回 124。
	stdout.Reset()
	stderr.Reset()
	if code := RunCompress([]string{"sleep 5"}, 100, opts); code != 124 {
		t.Fatalf("超时退出码 %d, want 124", code)
	}
	if !strings.Contains(stderr.String(), "超时") {
		t.Fatalf("超时应在 stderr 说明，实际 %q", stderr.String())
	}

	// 无命令返回 0。
	if code := RunCompress(nil, 0, opts); code != 0 {
		t.Fatalf("无命令退出码 %d", code)
	}
	if code := RunCompress([]string{""}, 0, opts); code != 0 {
		t.Fatalf("空命令退出码 %d", code)
	}
}
