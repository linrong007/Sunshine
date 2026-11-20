package workflow

import "testing"

// TestParseGitLabSubgroup 是 C2 回归（与 internal/registry 同规则）：
// GitLab 子群组仓库（多段路径）不能再被两段式切错。
func TestParseGitLabSubgroup(t *testing.T) {
	cases := []struct {
		in      string
		repo    string
		subdir  string
		ref     string
		gitURL  string
		rawBase string
	}{
		{"https://git.corp.com/group/sub/repo", "group/sub/repo", "", "main",
			"https://git.corp.com/group/sub/repo.git",
			"https://git.corp.com/group/sub/repo/-/raw/main"},
		{"https://gitlab.com/group/sub/repo", "group/sub/repo", "", "main",
			"https://gitlab.com/group/sub/repo.git",
			"https://gitlab.com/group/sub/repo/-/raw/main"},
		{"https://gitlab.com/group/sub/deep/repo", "group/sub/deep/repo", "", "main",
			"https://gitlab.com/group/sub/deep/repo.git",
			"https://gitlab.com/group/sub/deep/repo/-/raw/main"},
		{"https://git.corp.com/group/sub/repo/-/tree/main/marketplace", "group/sub/repo", "marketplace", "main",
			"https://git.corp.com/group/sub/repo.git",
			"https://git.corp.com/group/sub/repo/-/raw/main/marketplace"},
		{"https://gitlab.com/group/sub/repo/-/tree/dev/marketplace", "group/sub/repo", "marketplace", "dev",
			"https://gitlab.com/group/sub/repo.git",
			"https://gitlab.com/group/sub/repo/-/raw/dev/marketplace"},
		{"https://git.corp.com/org/repo", "org/repo", "", "main",
			"https://git.corp.com/org/repo.git",
			"https://git.corp.com/org/repo/-/raw/main"},
		{"https://github.com/org/repo/tree/main/marketplace", "org/repo", "marketplace", "main",
			"https://github.com/org/repo.git",
			"https://raw.githubusercontent.com/org/repo/main/marketplace"},
	}
	for _, c := range cases {
		got, err := ParseRegistrySource(c.in)
		if err != nil {
			t.Errorf("ParseRegistrySource(%q) 失败：%v", c.in, err)
			continue
		}
		if got.Repo != c.repo || got.Subdir != c.subdir || got.Ref != c.ref ||
			got.GitURL != c.gitURL || got.RawBaseURL != c.rawBase {
			t.Errorf("ParseRegistrySource(%q) =\nRepo=%q Subdir=%q Ref=%q GitURL=%q Raw=%q\n想得到 Repo=%q Subdir=%q Ref=%q GitURL=%q Raw=%q",
				c.in, got.Repo, got.Subdir, got.Ref, got.GitURL, got.RawBaseURL,
				c.repo, c.subdir, c.ref, c.gitURL, c.rawBase)
		}
	}
}

// TestNormalizeGitLabDashTree 是 C3 回归：公开 GitLab 的 /-/tree/
// 浏览链接要与自建 GitLab 一致地解析。
func TestNormalizeGitLabDashTree(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://gitlab.com/org/repo/-/tree/main/marketplace", "gitlab:org/repo/marketplace#main"},
		{"https://gitlab.com/org/repo/-/tree/main", "gitlab:org/repo#main"},
		{"https://gitlab.com/group/sub/repo/-/tree/dev/marketplace", "gitlab:group/sub/repo/marketplace#dev"},
	}
	for _, c := range cases {
		if got := normalizeRegistrySource(c.in); got != c.want {
			t.Errorf("normalizeRegistrySource(%q) = %q，想得到 %q", c.in, got, c.want)
		}
	}
	got, err := ParseRegistrySource("https://gitlab.com/org/repo/-/tree/main/marketplace")
	if err != nil {
		t.Fatalf("ParseRegistrySource 失败：%v", err)
	}
	if got.Repo != "org/repo" || got.Subdir != "marketplace" || got.Ref != "main" {
		t.Errorf("解析不符：%+v", got)
	}
	if got.RawBaseURL != "https://gitlab.com/org/repo/-/raw/main/marketplace" {
		t.Errorf("RawBaseURL = %q", got.RawBaseURL)
	}
}
