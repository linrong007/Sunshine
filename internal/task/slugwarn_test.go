package task

import (
	"strings"
	"testing"
)

func TestSimilarSlug(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"leave-management", "leave-request-mgmt", true}, // 只差通用词+一词，问题清单第 6 条实例
		{"额度管理", "假期额度管理", true},                         // 子串包含
		{"user-list", "user-detail", true},               // 只差通用词
		{"login", "login-page", true},                    // 子串包含
		{"refund-reason", "refund-list", true},           // 剥后 [refund] 包含于 [refund reason]
		{"payment-refund", "user-profile", false},
		{"auth-token", "token-refresh", false}, // 词序不同、无包含
		{"leave-management", "leave-management", false},
		{"", "anything", false},
		{"page-list", "view-module", false}, // 全是通用词，不判近似
	}
	for _, c := range cases {
		if got := SimilarSlug(c.a, c.b); got != c.want {
			t.Errorf("SimilarSlug(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := SimilarSlug(c.b, c.a); got != c.want {
			t.Errorf("SimilarSlug(%q, %q) = %v, want %v (reversed)", c.b, c.a, got, c.want)
		}
	}
}

func TestDirSlug(t *testing.T) {
	if got := DirSlug("11-24-leave-management"); got != "leave-management" {
		t.Errorf("DirSlug = %q", got)
	}
	if got := DirSlug("00-bootstrap-guidelines"); got != "bootstrap-guidelines" {
		t.Errorf("DirSlug system = %q", got)
	}
	if got := DirSlug("archive"); got != "" {
		t.Errorf("DirSlug non-task = %q", got)
	}
}

func TestCreateWarnsOnSimilarSlug(t *testing.T) {
	root := newTestRoot(t)
	r, _, errBuf := newTestRunner(t, root)
	if code := r.Create(CreateOptions{Title: "调休管理", Description: "d", Slug: "leave-management"}); code != 0 {
		t.Fatalf("first create failed: %d", code)
	}
	errBuf.Reset()
	if code := r.Create(CreateOptions{Title: "请假管理", Description: "d", Slug: "leave-request-mgmt"}); code != 0 {
		t.Fatalf("similar create should still succeed, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "近似") || !strings.Contains(errBuf.String(), "leave-management") {
		t.Errorf("expected similar-name warning mentioning the existing task, got: %s", errBuf.String())
	}
	errBuf.Reset()
	if code := r.Create(CreateOptions{Title: "报销审批", Description: "d", Slug: "expense-approval"}); code != 0 {
		t.Fatalf("unrelated create failed: %d", code)
	}
	if strings.Contains(errBuf.String(), "近似") {
		t.Errorf("unrelated name should not warn, got: %s", errBuf.String())
	}
}
