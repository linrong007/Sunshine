package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linrong007/Sunshine/internal/channel/supervisor"
)

func writeSpawnTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeSpawnCmd(t *testing.T, cwd, filePattern string) (SpawnWorkerOptions, error) {
	t.Helper()
	cmd := newChannelSpawnCmd()
	for k, v := range map[string]string{
		"provider": "claude",
		"as":       "worker1",
		"cwd":      cwd,
	} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("Set %s 失败：%v", k, err)
		}
	}
	if filePattern != "" {
		if err := cmd.Flags().Set("file", filePattern); err != nil {
			t.Fatalf("Set file 失败：%v", err)
		}
	}
	return parseSpawnFlags(cmd, "review")
}

// TestSpawnFileGlob_DeepPreserved 是 D2 回归测试：生产路径
// （channelWorkerRuntime == nil）下 CLI 不预展开 glob，原始
// "src/**/*.go" 原样到达 supervisor；supervisor 展开后深层文件
// 不被静默丢弃。另外 --file 命中目录时生产路径不因 CLI 预读失败。
func TestSpawnFileGlob_DeepPreserved(t *testing.T) {
	oldRuntime := channelWorkerRuntime
	channelWorkerRuntime = nil
	t.Cleanup(func() { channelWorkerRuntime = oldRuntime })

	dir := t.TempDir()
	writeSpawnTestFile(t, filepath.Join(dir, "src", "shallow.go"), "package src // shallow\n")
	writeSpawnTestFile(t, filepath.Join(dir, "src", "a", "mid.go"), "package a // mid\n")
	writeSpawnTestFile(t, filepath.Join(dir, "src", "a", "b", "deep.go"), "package b // deep\n")

	opts, err := makeSpawnCmd(t, dir, "src/**/*.go")
	if err != nil {
		t.Fatalf("parseSpawnFlags 不应失败：%v", err)
	}
	if len(opts.Files) != 1 || opts.Files[0] != "src/**/*.go" {
		t.Fatalf("生产路径应原样透传原始 pattern，实际 Files=%v", opts.Files)
	}
	if len(opts.JSONLManifests) != 0 {
		t.Fatalf("JSONLManifests 应为空，实际 %v", opts.JSONLManifests)
	}
	// CLI 不应预读内容：SystemPrompt 里不应含任何文件内容。
	if strings.Contains(opts.SystemPrompt, "deep") || strings.Contains(opts.SystemPrompt, "shallow") {
		t.Fatalf("生产路径 CLI 不应预读文件内容进 SystemPrompt：%q", opts.SystemPrompt)
	}
	// 原始 pattern 交给 supervisor 展开后应含深层文件。
	assembled := supervisor.AssembleContext(dir, opts.Files, opts.JSONLManifests, nil)
	if len(assembled.Paths) != 3 {
		t.Fatalf("supervisor 展开 src/**/*.go 应得到 3 个文件，实际 %v", assembled.Paths)
	}
	for _, want := range []string{"shallow", "mid", "deep"} {
		if !strings.Contains(assembled.Prompt, want) {
			t.Errorf("supervisor 组装结果应含 %q 内容，实际 Prompt：%q", want, assembled.Prompt)
		}
	}
	foundDeep := false
	for _, p := range assembled.Paths {
		if strings.HasSuffix(filepath.ToSlash(p), "src/a/b/deep.go") {
			foundDeep = true
		}
	}
	if !foundDeep {
		t.Errorf("深层文件 src/a/b/deep.go 应在 supervisor 展开结果里，实际 %v", assembled.Paths)
	}

	// --file 命中目录：CLI 预组装旧行为会报 is a directory 并杀掉 spawn；
	// 生产路径不预读，应成功，supervisor 侧 warn+跳过目录。
	if err := os.MkdirAll(filepath.Join(dir, "src2", "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpawnTestFile(t, filepath.Join(dir, "src2", "only.go"), "package src2 // only\n")
	opts2, err := makeSpawnCmd(t, dir, "src2/*")
	if err != nil {
		t.Fatalf("生产路径 --file 命中目录不应失败：%v", err)
	}
	assembled2 := supervisor.AssembleContext(dir, opts2.Files, opts2.JSONLManifests, nil)
	if len(assembled2.Paths) != 1 || !strings.Contains(assembled2.Prompt, "only") {
		t.Fatalf("supervisor 应跳过目录、只含 only.go，实际 Paths=%v Prompt=%q", assembled2.Paths, assembled2.Prompt)
	}
}
