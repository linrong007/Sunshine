package platforms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnableCodexHooks(t *testing.T) {
	tests := []struct {
		name    string
		initial string // 初始文件内容
		exists  bool   // false 表示文件不存在
		want    string // 期望的最终文件内容
	}{
		{
			name:   "文件不存在",
			exists: false,
			want:   "[features]\nhooks = true\n",
		},
		{
			name:    "无 [features] 节",
			exists:  true,
			initial: "# 我的配置\n[projects.\"/x\"]\ntrust_level = \"trusted\"\n",
			want:    "# 我的配置\n[projects.\"/x\"]\ntrust_level = \"trusted\"\n\n[features]\nhooks = true\n",
		},
		{
			name:    "已有 hooks=false",
			exists:  true,
			initial: "[features]\nhooks = false\n",
			want:    "[features]\nhooks = true\n",
		},
		{
			name:    "已有 codex_hooks=false",
			exists:  true,
			initial: "[features]\ncodex_hooks = false\n",
			want:    "[features]\nhooks = true\ncodex_hooks = true\n",
		},
		{
			name:    "已有 hooks=true 不动",
			exists:  true,
			initial: "[features]\nhooks = true\n",
			want:    "[features]\nhooks = true\n",
		},
		{
			name:    "带注释与其它配置原样保留",
			exists:  true,
			initial: "# 注释\n[features] # 节注释\nhooks = false # 行尾注释\n[other]\nfoo = 1\n",
			want:    "# 注释\n[features] # 节注释\nhooks = true # 行尾注释\n[other]\nfoo = 1\n",
		},
		{
			name:    "hooks 在别的节里不动",
			exists:  true,
			initial: "[other]\nhooks = false\n",
			want:    "[other]\nhooks = false\n\n[features]\nhooks = true\n",
		},
		{
			name:    "空文件",
			exists:  true,
			initial: "",
			want:    "[features]\nhooks = true\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".codex", "config.toml")
			if tc.exists {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := EnableCodexHooks(); err != nil {
				t.Fatalf("EnableCodexHooks 失败：%v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读回 config.toml 失败：%v", err)
			}
			if string(got) != tc.want {
				t.Errorf("文件内容不对：\n实际：%q\n期望：%q", string(got), tc.want)
			}
			if !CodexHooksEnabled() {
				t.Error("写完后 CodexHooksEnabled 应为 true")
			}
			// 幂等：再跑一次内容不变。
			if err := EnableCodexHooks(); err != nil {
				t.Fatalf("第二次 EnableCodexHooks 失败：%v", err)
			}
			got2, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got2) != tc.want {
				t.Errorf("第二次运行后内容变了：%q，期望 %q", string(got2), tc.want)
			}
		})
	}
}

func TestEnableCodexHooks_DirBlocked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// ~/.codex 是个文件，MkdirAll 失败，应返回 error。
	if err := os.WriteFile(filepath.Join(home, ".codex"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnableCodexHooks(); err == nil {
		t.Error("~/.codex 被文件占住时应返回 error")
	}
}
