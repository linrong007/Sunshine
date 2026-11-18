package cli

import (
	"testing"

	"github.com/linrong007/Sunshine/internal/registry"
)

func TestResolveRegistryStrategy(t *testing.T) {
	set := func(overwrite, append_ bool, template string) {
		initOverwrite, initAppend, initTemplate = overwrite, append_, template
	}
	defer set(false, false, "")

	set(false, false, "")
	if got, err := resolveRegistryStrategy(); err != nil || got != registry.StrategyAppend {
		t.Errorf("默认应为 append（只补缺失），得到 %q, %v", got, err)
	}
	set(true, false, "demo-spec")
	if got, err := resolveRegistryStrategy(); err != nil || got != registry.StrategyOverwrite {
		t.Errorf("应为 overwrite，得到 %q, %v", got, err)
	}
	set(false, true, "demo-spec")
	if got, err := resolveRegistryStrategy(); err != nil || got != registry.StrategyAppend {
		t.Errorf("应为 append，得到 %q, %v", got, err)
	}
	// --overwrite/--append 不再要求 --template（自动选能补上时放行，
	// 校验移到目标解析后，见 init.go）。
	set(true, false, "")
	if got, err := resolveRegistryStrategy(); err != nil || got != registry.StrategyOverwrite {
		t.Errorf("--overwrite 不配 --template 也应返回 overwrite，得到 %q, %v", got, err)
	}
	set(false, true, "")
	if got, err := resolveRegistryStrategy(); err != nil || got != registry.StrategyAppend {
		t.Errorf("--append 不配 --template 也应返回 append，得到 %q, %v", got, err)
	}
	// 两者互斥。
	set(true, true, "demo-spec")
	if _, err := resolveRegistryStrategy(); err == nil {
		t.Error("--overwrite 与 --append 同时使用应报错")
	}
}
