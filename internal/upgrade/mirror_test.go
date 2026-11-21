package upgrade

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mirrorTransport 模拟「直连不通、镜像可通」的网络：直连
// api.github.com / github.com 一律连接失败；经
// https://ghfast.top/ 前缀的请求按路径应答。
type mirrorTransport struct {
	sawMirror bool
	sawDirect bool
}

func (t *mirrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	if strings.HasPrefix(u, "https://ghfast.top/") {
		t.sawMirror = true
		body := ""
		switch {
		case strings.Contains(u, "/releases/latest"):
			body = `{"tag_name":"v9.9.9","assets":[{"name":"checksums.txt","browser_download_url":"https://github.com/linrong007/Sunshine/releases/download/v9.9.9/checksums.txt"}]}`
		case strings.Contains(u, "checksums.txt"):
			body = "payload"
		default:
			body = "{}"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
	}
	t.sawDirect = true
	return nil, errors.New("connection refused")
}

func TestMirrorFallback_ResolveAndDownload(t *testing.T) {
	tr := &mirrorTransport{}
	c := &Client{
		Repo:       "linrong007/Sunshine",
		HTTPClient: &http.Client{Transport: tr},
		Bases:      []string{"", "https://ghfast.top"},
	}
	rel, err := c.ResolveRelease(context.Background(), "latest")
	if err != nil {
		t.Fatalf("直连失败时应经镜像拿到发布：%v", err)
	}
	if rel.TagName != "v9.9.9" || c.UsedBase != "https://ghfast.top" || !tr.sawMirror || !tr.sawDirect {
		t.Fatalf("发布/通道记录不对：%+v UsedBase=%q mirror=%v direct=%v", rel, c.UsedBase, tr.sawMirror, tr.sawDirect)
	}
	data, err := c.download(context.Background(), "https://github.com/linrong007/Sunshine/releases/download/v9.9.9/checksums.txt")
	if err != nil || string(data) != "payload" {
		t.Fatalf("镜像下载不对：%q %v", data, err)
	}
}

// notFoundTransport 直连返回 404：确定性结论，不许再试镜像。
type notFoundTransport struct{ mirrorTried bool }

func (t *notFoundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasPrefix(req.URL.String(), "https://ghfast.top/") {
		t.mirrorTried = true
	}
	return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header), Request: req}, nil
}

func TestMirrorFallback_NotFoundShortCircuit(t *testing.T) {
	tr := &notFoundTransport{}
	c := &Client{
		Repo:       "linrong007/Sunshine",
		HTTPClient: &http.Client{Transport: tr},
		Bases:      []string{"", "https://ghfast.top"},
	}
	_, err := c.ResolveRelease(context.Background(), "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "没有找到版本") {
		t.Fatalf("404 应报发布不存在：%v", err)
	}
	if tr.mirrorTried {
		t.Fatalf("404 后不应再试镜像")
	}
}
