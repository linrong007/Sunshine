package registry

import "testing"

// TestParseGitLabSubgroup 是 C2 回归：GitLab 子群组仓库（多段路径）
// 不能再被两段式切成 repo=group/sub、subdir=repo。
func TestParseGitLabSubgroup(t *testing.T) {
	cases := []struct {
		in      string
		repo    string
		subdir  string
		ref     string
		gitURL  string
		rawBase string
	}{
		// 自建 GitLab：三段裸路径整体是仓库。
		{"https://git.corp.com/group/sub/repo", "group/sub/repo", "", "main",
			"https://git.corp.com/group/sub/repo.git",
			"https://git.corp.com/group/sub/repo/-/raw/main"},
		// 公开 gitlab.com：三段裸路径整体是仓库。
		{"https://gitlab.com/group/sub/repo", "group/sub/repo", "", "main",
			"https://gitlab.com/group/sub/repo.git",
			"https://gitlab.com/group/sub/repo/-/raw/main"},
		// 四段裸路径（两层子群组）。
		{"https://gitlab.com/group/sub/deep/repo", "group/sub/deep/repo", "", "main",
			"https://gitlab.com/group/sub/deep/repo.git",
			"https://gitlab.com/group/sub/deep/repo/-/raw/main"},
		// 自建子群组 + /-/tree/：tree 之前是仓库、之后是子目录。
		{"https://git.corp.com/group/sub/repo/-/tree/main/marketplace", "group/sub/repo", "marketplace", "main",
			"https://git.corp.com/group/sub/repo.git",
			"https://git.corp.com/group/sub/repo/-/raw/main/marketplace"},
		// 公开子群组 + /-/tree/。
		{"https://gitlab.com/group/sub/repo/-/tree/dev/marketplace", "group/sub/repo", "marketplace", "dev",
			"https://gitlab.com/group/sub/repo.git",
			"https://gitlab.com/group/sub/repo/-/raw/dev/marketplace"},
		// 公开子群组 + /tree/（无 -）。
		{"https://gitlab.com/group/sub/repo/tree/main/marketplace", "group/sub/repo", "marketplace", "main",
			"https://gitlab.com/group/sub/repo.git",
			"https://gitlab.com/group/sub/repo/-/raw/main/marketplace"},
		// 既有形态不回退：两段仓库、自建两段、GitHub tree。
		{"https://git.corp.com/org/repo", "org/repo", "", "main",
			"https://git.corp.com/org/repo.git",
			"https://git.corp.com/org/repo/-/raw/main"},
		{"https://gitlab.com/org/repo", "org/repo", "", "main",
			"https://gitlab.com/org/repo.git",
			"https://gitlab.com/org/repo/-/raw/main"},
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
// 浏览链接要与自建 GitLab 一致地解析，不能把 -/tree/... 留在 subdir。
func TestNormalizeGitLabDashTree(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://gitlab.com/org/repo/-/tree/main/marketplace", "gitlab:org/repo/marketplace#main"},
		{"https://gitlab.com/org/repo/-/tree/main", "gitlab:org/repo#main"},
		{"https://gitlab.com/group/sub/repo/-/tree/dev/marketplace", "gitlab:group/sub/repo/marketplace#dev"},
		{"https://github.com/org/repo/-/tree/main/marketplace", "gh:org/repo/marketplace#main"},
	}
	for _, c := range cases {
		if got := NormalizeRegistrySource(c.in); got != c.want {
			t.Errorf("NormalizeRegistrySource(%q) = %q，想得到 %q", c.in, got, c.want)
		}
	}
	// 解析结果层面再钉一遍：subdir 与 RawBaseURL 不含 -/tree 残段。
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
	if got.GitURL != "https://gitlab.com/org/repo.git" {
		t.Errorf("GitURL = %q", got.GitURL)
	}
}
