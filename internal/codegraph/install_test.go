package codegraph

// 自带安装的端到端单测：本地 HTTP 服务器冒充 GitHub 发布页，
// HOME 指到临时目录，验证版本解析、带进度下载、解压布局与软链。

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// buildFakeBundle 造一个与真实发布包同构的 tar.gz：
// 顶层目录 codegraph-<target>/，内含 bin/codegraph、lib/node 与一个包内软链。
func buildFakeBundle(t *testing.T, target string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	top := "codegraph-" + target
	files := []struct {
		name string
		body string
		mode int64
	}{
		{top + "/bin/codegraph", "#!/bin/sh\necho fake codegraph\n", 0o755},
		{top + "/lib/node", "fake-node-binary", 0o755},
	}
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.WriteHeader(&tar.Header{Name: top + "/bin/node", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "../lib/node"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestSelfInstallMirrorFailover 通道逐个试：版本解析第一把失败、
// 下载第一把 500，都应换下一个通道重试并最终装成，且有换通道提示行。
// 本地服务器不是 GitHub 主机，Rewrite 对所有通道都原样返回，
// 因此重试会再次打到同一服务器——正好验证重试回路本身。
func TestSelfInstallMirrorFailover(t *testing.T) {
	target, ok := targetTriple()
	if !ok {
		t.Skip("当前平台无预编译包口径")
	}
	bundle := buildFakeBundle(t, target)
	var latestHits, downloadHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			latestHits++
			if latestHits == 1 {
				w.WriteHeader(http.StatusOK) // 无 Location，第一把解析失败
				return
			}
			w.Header().Set("Location", "/releases/tag/v9.9.9")
			w.WriteHeader(http.StatusFound)
		case r.URL.Path == "/releases/download/v9.9.9/codegraph-"+target+".tar.gz":
			downloadHits++
			if downloadHits == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(bundle)))
			_, _ = w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	oldBase := releaseBase
	defer func() { releaseBase = oldBase }()
	releaseBase = srv.URL

	var out bytes.Buffer
	if err := selfInstall(context.Background(), &out); err != nil {
		t.Fatalf("换通道后应装成：%v", err)
	}
	if latestHits < 2 {
		t.Errorf("版本解析应重试，实际请求 %d 次", latestHits)
	}
	if downloadHits < 2 {
		t.Errorf("下载应换通道重试，实际请求 %d 次", downloadHits)
	}
	if !strings.Contains(out.String(), "改走") {
		t.Errorf("应有换通道提示行，实际输出：%q", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "codegraph")); err != nil {
		t.Errorf("装完应有 codegraph 软链：%v", err)
	}
}

func TestSelfInstallAgainstLocalServer(t *testing.T) {
	target, ok := targetTriple()
	if !ok {
		t.Skip("当前平台无预编译包口径")
	}
	bundle := buildFakeBundle(t, target)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			w.Header().Set("Location", "/releases/tag/v9.9.9")
			w.WriteHeader(http.StatusFound)
		case r.URL.Path == "/releases/download/v9.9.9/codegraph-"+target+".tar.gz":
			w.Header().Set("Content-Length", strconv.Itoa(len(bundle)))
			_, _ = w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	oldBase, oldImpl := releaseBase, selfInstallImpl
	defer func() { releaseBase, selfInstallImpl = oldBase, oldImpl }()
	releaseBase = srv.URL
	selfInstallImpl = selfInstall

	var progress bytes.Buffer
	if err := selfInstall(context.Background(), &progress); err != nil {
		t.Fatalf("自带安装失败：%v", err)
	}
	launcher := filepath.Join(home, ".codegraph", "versions", "v9.9.9", "bin", "codegraph")
	if _, err := os.Stat(launcher); err != nil {
		t.Fatalf("启动器应落在 versions/v9.9.9/bin/codegraph：%v", err)
	}
	link, err := os.Readlink(filepath.Join(home, ".local", "bin", "codegraph"))
	if err != nil || link != launcher {
		t.Errorf("bin 软链应指向启动器：link=%q err=%v", link, err)
	}
	cur, err := os.Readlink(filepath.Join(home, ".codegraph", "current"))
	if err != nil || cur != filepath.Join(home, ".codegraph", "versions", "v9.9.9") {
		t.Errorf("current 软链应指向版本目录：cur=%q err=%v", cur, err)
	}
	nodeLink, err := os.Readlink(filepath.Join(home, ".codegraph", "versions", "v9.9.9", "bin", "node"))
	if err != nil || nodeLink != "../lib/node" {
		t.Errorf("包内软链应被保留：nodeLink=%q err=%v", nodeLink, err)
	}
	if !strings.Contains(progress.String(), "正在下载 CodeGraph 安装包") || !strings.Contains(progress.String(), "%") {
		t.Errorf("下载应有带百分比的进度输出，实际：%q", progress.String())
	}
}

// TestSelfInstallBadBundleLayout 包里缺 bin/codegraph 时应报错，
// 让上层回退官方脚本，而不是装出一个坏工具。
func TestSelfInstallBadBundleLayout(t *testing.T) {
	target, ok := targetTriple()
	if !ok {
		t.Skip("当前平台无预编译包口径")
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := "nothing here"
	_ = tw.WriteHeader(&tar.Header{Name: "codegraph-" + target + "/README", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()
	_ = gz.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			w.Header().Set("Location", "/releases/tag/v9.9.9")
			w.WriteHeader(http.StatusFound)
			return
		}
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()
	t.Setenv("HOME", t.TempDir())
	oldBase := releaseBase
	defer func() { releaseBase = oldBase }()
	releaseBase = srv.URL
	err := selfInstall(context.Background(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "布局与预期不符") {
		t.Errorf("缺启动器的包应报错回退，实际 err=%v", err)
	}
}
