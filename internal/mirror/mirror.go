// Package mirror 提供 GitHub 下载的镜像回退通道。
//
// 背景：部分网络环境下 github.com 的发布下载（走独立 CDN）连不上
// 或超时，但网页与 API 尚通。策略是直连优先、失败后逐个试镜像
// 前缀（<镜像>/https://github.com/...）；镜像只作搬运，完整性仍由
// 调用方的校验（upgrade 的 checksums、git 的对象校验）保证。
//
// 候选顺序：环境变量 SUNSHINE_DOWNLOAD_MIRROR 指定的镜像 →
// 直连 → 内置镜像。镜像站是第三方代理、状态多变，内置名单只作
// 回退，绝不反客为主。
package mirror

import (
	"net/url"
	"os"
	"strings"
)

// EnvVar 是指定自定义镜像前缀的环境变量名。
const EnvVar = "SUNSHINE_DOWNLOAD_MIRROR"

// builtin 是内置候选镜像（2026-11-20 实测可用：12 个候选里唯二
// 能返回官方 checksums.txt 的；状态多变失效也无妨，逐个试）。
var builtin = []string{"https://ghfast.top", "https://gh-proxy.com"}

// githubHosts 是可被镜像前缀改写的 GitHub 系主机。
var githubHosts = map[string]bool{
	"github.com":                true,
	"api.github.com":            true,
	"raw.githubusercontent.com": true,
}

// Bases 返回按优先级排序的通道前缀列表：环境变量指定的镜像
// （若有）→ 空串（直连）→ 内置镜像。空串代表直连。
func Bases() []string {
	var out []string
	if v := strings.TrimSuffix(strings.TrimSpace(os.Getenv(EnvVar)), "/"); v != "" {
		out = append(out, v)
	}
	out = append(out, "")
	out = append(out, builtin...)
	return out
}

// Rewrite 把原始 URL 改写为经指定前缀通道的 URL。前缀为空、
// 或 URL 不是 GitHub 系主机（如本地测试服务器、自建实例）时
// 原样返回——镜像只碰 GitHub 的地址。
func Rewrite(base, rawURL string) string {
	if base == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || !githubHosts[u.Host] {
		return rawURL
	}
	return base + "/" + rawURL
}

// Label 返回通道的展示名：空前缀是「直连」，否则是前缀本身。
func Label(base string) string {
	if base == "" {
		return "直连"
	}
	return base
}
