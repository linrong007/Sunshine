package mirror

import (
	"strings"
	"testing"
)

func TestBasesOrder(t *testing.T) {
	t.Setenv(EnvVar, "")
	b := Bases()
	if len(b) != 3 || b[0] != "" || b[1] != "https://ghfast.top" || b[2] != "https://gh-proxy.com" {
		t.Fatalf("无环境变量时候选不对：%v", b)
	}
	t.Setenv(EnvVar, "https://mirror.example.com/")
	b = Bases()
	if len(b) != 4 || b[0] != "https://mirror.example.com" || b[1] != "" {
		t.Fatalf("环境变量应居首且去掉尾斜杠：%v", b)
	}
}

func TestRewrite(t *testing.T) {
	cases := []struct{ base, in, want string }{
		{"", "https://github.com/a/b", "https://github.com/a/b"},
		{"https://ghfast.top", "https://github.com/a/b/releases/download/v1/x.tar.gz", "https://ghfast.top/https://github.com/a/b/releases/download/v1/x.tar.gz"},
		{"https://ghfast.top", "https://api.github.com/repos/a/b/releases/latest", "https://ghfast.top/https://api.github.com/repos/a/b/releases/latest"},
		{"https://ghfast.top", "https://raw.githubusercontent.com/a/b/main/index.json", "https://ghfast.top/https://raw.githubusercontent.com/a/b/main/index.json"},
		// 非 GitHub 主机不改写：自建实例、本地测试服务器、SSH 地址。
		{"https://ghfast.top", "https://git.company.com/a/b.git", "https://git.company.com/a/b.git"},
		{"https://ghfast.top", "http://127.0.0.1:8080/x", "http://127.0.0.1:8080/x"},
		{"https://ghfast.top", "git@github.com:a/b.git", "git@github.com:a/b.git"},
	}
	for _, c := range cases {
		if got := Rewrite(c.base, c.in); got != c.want {
			t.Errorf("Rewrite(%q, %q) = %q，想 %q", c.base, c.in, got, c.want)
		}
	}
	if !strings.Contains(Label("https://ghfast.top"), "ghfast") || Label("") != "直连" {
		t.Errorf("Label 不对")
	}
}
