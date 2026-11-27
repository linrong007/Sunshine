package workflow

import (
	"strings"
	"testing"
)

func TestListBundled(t *testing.T) {
	list := ListBundled()
	want := []string{"native", "channel-driven-subagent-dispatch"}
	if len(list) != len(want) {
		t.Fatalf("内置模板应为 %d 个，实际 %d 个", len(want), len(list))
	}
	for i, id := range want {
		if list[i].ID != id {
			t.Errorf("第 %d 个模板应为 %q，实际 %q", i, id, list[i].ID)
		}
		if list[i].Name == "" || list[i].Description == "" {
			t.Errorf("模板 %q 的名称/描述不应为空", id)
		}
		if list[i].Source != SourceBundled {
			t.Errorf("模板 %q 应为内置来源", id)
		}
	}
}

func TestBundledContent(t *testing.T) {
	for _, id := range []string{"native", "channel-driven-subagent-dispatch"} {
		content, err := BundledContent(id)
		if err != nil {
			t.Fatalf("读模板 %q 失败：%v", id, err)
		}
		if content == "" {
			t.Fatalf("模板 %q 内容为空", id)
		}
		// 中文翻译版：不得残留 Trellis 原文标识与 Python 脚本调用。
		for _, bad := range []string{"trellis", "Trellis", "python3", "task.py", "get_context.py", ".trellis/"} {
			if strings.Contains(content, bad) {
				t.Errorf("模板 %q 不应残留 %q", id, bad)
			}
		}
		// 关键结构必须保留。
		for _, keep := range []string{"[workflow-state:no_task]", "[workflow-state:planning]", "[workflow-state:in_progress]", "3.1", "sunshine task"} {
			if !strings.Contains(content, keep) {
				t.Errorf("模板 %q 应包含 %q", id, keep)
			}
		}
	}
	if _, err := BundledContent("no-such-template"); err == nil {
		t.Error("未知模板应报错")
	}
}

func TestBundledContentNativeHasTaskModes(t *testing.T) {
	native, _ := BundledContent("native")
	channel, _ := BundledContent("channel-driven-subagent-dispatch")
	if native == channel {
		t.Error("channel 模板应与 native 不同")
	}
	// 任务级三模式并入 native / channel：模式选择、双环分支、判卷、
	// 小修通道都必须在两份模板里。
	for _, keep := range []string{"meta.mode", "双环分支", "外环判卷", "小修通道", "和默认模式一样，只是不逼问"} {
		if !strings.Contains(native, keep) {
			t.Errorf("native 模板应包含 %q", keep)
		}
		if !strings.Contains(channel, keep) {
			t.Errorf("channel 模板应包含 %q", keep)
		}
	}
	if !strings.Contains(native, "task_error") {
		t.Error("native 模板应包含 task_error 面包屑块")
	}
}

func TestResolveRetiredTDD(t *testing.T) {
	if _, err := BundledContent("tdd"); err == nil {
		t.Error("tdd 已退役，BundledContent 应报错")
	}
	_, err := Resolve("tdd", "")
	if err == nil {
		t.Fatal("Resolve(tdd) 应报错指路")
	}
	if !strings.Contains(err.Error(), "任务级模式") {
		t.Errorf("报错应指路任务级模式，实际：%v", err)
	}
}

func TestBundledContentChannel(t *testing.T) {
	content, _ := BundledContent("channel-driven-subagent-dispatch")
	for _, keep := range []string{"sunshine channel spawn", "sunshine channel create", "check-cc", "implement.jsonl"} {
		if !strings.Contains(content, keep) {
			t.Errorf("channel 模板应包含 %q", keep)
		}
	}
}

func TestResolveBundledOffline(t *testing.T) {
	for _, id := range []string{"native", "channel-driven-subagent-dispatch"} {
		r, err := Resolve(id, "")
		if err != nil {
			t.Fatalf("解析 %q 失败：%v", id, err)
		}
		if r.ID != id || r.Content == "" {
			t.Errorf("解析 %q 结果不完整", id)
		}
		if r.Source != SourceBundled {
			t.Errorf("解析 %q 应为内置来源", id)
		}
	}
}

func TestResolveUnknownWithoutMarketplace(t *testing.T) {
	if _, err := Resolve("no-such-template", ""); err == nil {
		t.Error("未知模板且无市场来源时应报错")
	}
}

func TestResolveNativeIgnoresMarketplace(t *testing.T) {
	// native 是虚拟内置项（与上游一致）：即使传了市场来源也不联网。
	r, err := Resolve("native", "gh:someone/somewhere")
	if err != nil {
		t.Fatalf("解析 native 失败：%v", err)
	}
	if r.Source != SourceBundled || r.Content == "" {
		t.Error("native 应解析为内置模板")
	}
}

func TestListTemplatesWithoutMarketplace(t *testing.T) {
	list, errMsg := ListTemplates("")
	if errMsg != "" {
		t.Errorf("无市场来源时不应有错误信息：%s", errMsg)
	}
	if len(list) != 2 {
		t.Errorf("应列出 2 个内置模板，实际 %d 个", len(list))
	}
}

func TestListTemplatesBadMarketplaceFallsBack(t *testing.T) {
	list, errMsg := ListTemplates("not-a-valid-source")
	if len(list) != 2 {
		t.Errorf("市场拉取失败时应回落到 2 个内置模板，实际 %d 个", len(list))
	}
	if errMsg == "" {
		t.Error("市场拉取失败时应给出错误说明")
	}
}
