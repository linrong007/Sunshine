package registry

import "testing"

func TestMirrorRetryable(t *testing.T) {
	netErr := NewError(ErrKindNetwork, "无法连接")
	authErr := NewError(ErrKindAuth, "需要认证")
	gh := &Source{Kind: SourceKindHTTPS, GitURL: "https://github.com/linrong007/Sunshine.git"}
	if !mirrorRetryable(gh, netErr) {
		t.Error("github.com HTTPS + 网络错误应可镜像重试")
	}
	if mirrorRetryable(gh, authErr) {
		t.Error("认证错误不应重试")
	}
	ssh := &Source{Kind: SourceKindSSH, GitURL: "git@github.com:linrong007/Sunshine.git"}
	if mirrorRetryable(ssh, netErr) {
		t.Error("SSH 来源不应重试")
	}
	self := &Source{Kind: SourceKindHTTPS, GitURL: "https://git.company.com/a/b.git"}
	if mirrorRetryable(self, netErr) {
		t.Error("自建实例不应套 GitHub 镜像")
	}
	if mirrorRetryable(nil, netErr) {
		t.Error("nil 来源不应重试")
	}
}
