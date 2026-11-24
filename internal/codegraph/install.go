package codegraph

// 自带安装 CodeGraph 工具：照官方 install.sh 的布局自己下载安装，
// 下载过程带真实进度（百分比与 MB 数），不再让用户对着官方脚本
// curl -s 的静默下载干等（2026-11-24 用户实测：脚本卡在
// 「Installing CodeGraph v1.6.4...」无任何输出，最后还失败了，
// 失败原因也看不到）。任何一步走不通，调用方回退官方脚本。

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// releaseBase 是 CodeGraph 的 GitHub 仓库根地址（测试可替换）。
var releaseBase = "https://github.com/colbymchenry/codegraph"

// selfInstallImpl 是自带安装的实现入口（测试可替换为桩，强制走回退）。
var selfInstallImpl = selfInstall

// targetTriple 把当前平台映射为发布包名里的 <os>-<arch>（与官方脚本同口径）。
func targetTriple() (string, bool) {
	var osName, arch string
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	default:
		return "", false
	}
	switch runtime.GOARCH {
	case "arm64":
		arch = "arm64"
	case "amd64":
		arch = "x64"
	default:
		return "", false
	}
	return osName + "-" + arch, true
}

// installDirs 返回安装目录与命令软链目录，与官方脚本同口径、
// 同样认 CODEGRAPH_INSTALL_DIR / CODEGRAPH_BIN_DIR 覆盖。
func installDirs() (installDir, binDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("取不到用户主目录：%w", err)
	}
	installDir = os.Getenv("CODEGRAPH_INSTALL_DIR")
	if installDir == "" {
		installDir = filepath.Join(home, ".codegraph")
	}
	binDir = os.Getenv("CODEGRAPH_BIN_DIR")
	if binDir == "" {
		binDir = filepath.Join(home, ".local", "bin")
	}
	return installDir, binDir, nil
}

// resolveLatestVersion 解析最新版本号：与官方脚本同法，读
// releases/latest 的重定向目标，不走 API（不吃未认证限流）。
// CODEGRAPH_VERSION 已设置时直接用它。
func resolveLatestVersion(ctx context.Context, client *http.Client) (string, error) {
	if v := os.Getenv("CODEGRAPH_VERSION"); v != "" {
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}
		return v, nil
	}
	noRedirect := *client
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodHead, releaseBase+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", fmt.Errorf("解析最新版本失败：%w", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	idx := strings.LastIndex(loc, "/releases/tag/")
	if idx < 0 || idx+len("/releases/tag/") >= len(loc) {
		return "", fmt.Errorf("解析最新版本失败：重定向地址异常（%s）", loc)
	}
	return loc[idx+len("/releases/tag/"):], nil
}

// downloadWithProgress 把 url 下载到 destFile，每约 200 毫秒向 out
// 刷新一行进度（回车覆盖同一行）：已下载 / 总大小 / 百分比；
// 服务端不给总大小时只报已下载量。out 为 nil 时静默下载。
func downloadWithProgress(ctx context.Context, client *http.Client, url, destFile string, out io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败：服务器返回 %s", resp.Status)
	}
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	last := time.Now()
	report := func(final bool) {
		if out == nil {
			return
		}
		if total > 0 {
			fmt.Fprintf(out, "\r  ... 正在下载 CodeGraph 安装包：%.1f / %.1f MB（%d%%）",
				float64(written)/1048576, float64(total)/1048576, written*100/total)
		} else {
			fmt.Fprintf(out, "\r  ... 正在下载 CodeGraph 安装包：已下载 %.1f MB", float64(written)/1048576)
		}
		if final {
			fmt.Fprint(out, "\n")
		}
	}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if time.Since(last) > 200*time.Millisecond {
				report(false)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if out != nil {
				fmt.Fprint(out, "\n")
			}
			return fmt.Errorf("下载中断（已下载 %.1f MB）：%w", float64(written)/1048576, rerr)
		}
	}
	report(true)
	return nil
}

// extractBundle 解压 tar.gz 到 dest，剥掉包内顶层目录（与官方脚本
// tar --strip-components=1 同口径）；拒绝逃逸出 dest 的路径与链接。
func extractBundle(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("安装包不是有效的 gzip：%w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("读取安装包失败：%w", err)
		}
		rel := stripTopComponent(hdr.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, rel)
		if target != dest && !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("安装包内路径异常：%s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			wf, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode).Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(wf, tr); err != nil {
				wf.Close()
				return err
			}
			if err := wf.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := safeSymlink(dest, target, hdr.Linkname); err != nil {
				return err
			}
		case tar.TypeLink:
			linkRel := stripTopComponent(hdr.Linkname)
			if linkRel == "" {
				continue
			}
			src := filepath.Join(dest, linkRel)
			_ = os.Remove(target)
			if err := os.Link(src, target); err != nil {
				return err
			}
		}
	}
}

// stripTopComponent 去掉包内路径的第一层目录；顶层本身返回空串。
func stripTopComponent(name string) string {
	name = strings.TrimPrefix(name, "./")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// safeSymlink 创建符号链接，链接目标解析后必须仍在 dest 内
// （发布包内的链接都是包内相对链接，如 bin 指向 ../lib 的情形）。
func safeSymlink(dest, target, linkname string) error {
	if filepath.IsAbs(linkname) {
		return fmt.Errorf("安装包内有绝对路径链接：%s", linkname)
	}
	resolved := filepath.Join(filepath.Dir(target), linkname)
	if resolved != dest && !strings.HasPrefix(resolved, dest+string(os.PathSeparator)) {
		return fmt.Errorf("安装包内链接逃逸出安装目录：%s", linkname)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	_ = os.Remove(target)
	return os.Symlink(linkname, target)
}

// replaceSymlink 把 link 指向 target（先删后建，幂等）。
func replaceSymlink(link, target string) error {
	_ = os.Remove(link)
	return os.Symlink(target, link)
}

// selfInstall 自带安装 CodeGraph：解析版本 → 带进度下载 → 解压到
// ~/.codegraph/versions/<版本> → 软链 current 与 ~/.local/bin/codegraph
// → 清理旧版本目录。布局与官方脚本完全一致，两种装法装出的东西无区别。
func selfInstall(ctx context.Context, out io.Writer) error {
	target, ok := targetTriple()
	if !ok {
		return fmt.Errorf("当前平台 %s/%s 没有预编译安装包", runtime.GOOS, runtime.GOARCH)
	}
	installDir, binDir, err := installDirs()
	if err != nil {
		return err
	}
	client := &http.Client{}
	version, err := resolveLatestVersion(ctx, client)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/releases/download/%s/codegraph-%s.tar.gz", releaseBase, version, target)
	tmp, err := os.MkdirTemp("", "sunshine-codegraph-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, "cg.tar.gz")
	if err := downloadWithProgress(ctx, client, url, archive, out); err != nil {
		return err
	}
	dest := filepath.Join(installDir, "versions", version)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := extractBundle(archive, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	launcher := filepath.Join(dest, "bin", "codegraph")
	if _, err := os.Stat(launcher); err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("安装包布局与预期不符（缺 bin/codegraph）")
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if err := replaceSymlink(filepath.Join(installDir, "current"), dest); err != nil {
		return err
	}
	if err := replaceSymlink(filepath.Join(binDir, "codegraph"), launcher); err != nil {
		return err
	}
	// 清理旧版本目录，与官方脚本同口径（每个版本约 50MB，不清会堆积）。
	entries, _ := os.ReadDir(filepath.Join(installDir, "versions"))
	for _, e := range entries {
		if e.IsDir() && filepath.Join(installDir, "versions", e.Name()) != dest {
			_ = os.RemoveAll(filepath.Join(installDir, "versions", e.Name()))
		}
	}
	return nil
}
