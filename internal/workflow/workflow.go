// Package workflow 实现 `sunshine workflow` 的工作流模板注册与解析。
//
// 两个内置模板（native、channel-driven-subagent-dispatch）是 Trellis
// marketplace/workflows/ 的中文翻译版，随二进制以 go:embed 发布。
// tdd 模板已退役：TDD 改为任务级模式（meta.mode=tdd，在 native /
// channel 模板内以双环分支实现），Resolve 对 tdd 给出指路报错。
// 其中 channel-driven-subagent-dispatch 只安装模板文件，派发行为依赖
// Channel（阶段 6），本阶段不实现派发能力。
//
// 远程模板（-m/--marketplace）按 Trellis workflow-resolver.ts 的行为解析：
// 只支持 giget 风格来源（gh:/github:/gitlab:/bitbucket: 前缀）、公开或
// 自托管的 HTTPS/SSH Git 地址；不支持本地路径（上游同样不支持）。
package workflow

import (
	"fmt"

	"github.com/linrong007/Sunshine/templates"
)

// NativeID 是内置原生工作流的 id。
const NativeID = "native"

// RetiredTDDID 是已退役的 tdd 模板 id（TDD 改为任务级模式）。
const RetiredTDDID = "tdd"

// Source 是模板的来源：bundled（随二进制内置）或 marketplace（远程拉取）。
type Source string

const (
	// SourceBundled 随二进制内置，无需联网。
	SourceBundled Source = "bundled"
	// SourceMarketplace 来自 -m/--marketplace 指定的模板市场。
	SourceMarketplace Source = "marketplace"
)

// Template 是工作流模板的元数据（不含正文）。
type Template struct {
	ID          string
	Name        string
	Description string
	Path        string
	Source      Source
}

// Resolved 是解析出的工作流模板（含正文）。
type Resolved struct {
	Template
	Content string
}

// bundledTemplates 是两个内置模板的注册表（顺序即 --list 的展示顺序）。
// 名称与描述译自 marketplace/index.json。
var bundledTemplates = []Template{
	{
		ID:          NativeID,
		Name:        "原生 Sunshine 工作流",
		Description: "默认的 Sunshine 规划 / 执行 / 收尾工作流，含原生子代理与 inline 平台分支；任务模式（快速 / 默认 / TDD 双环）按任务在建任务时选",
		Path:        "bundled:sunshine/workflow.md",
		Source:      SourceBundled,
	},
	{
		ID:          "channel-driven-subagent-dispatch",
		Name:        "Channel 驱动的子代理派发",
		Description: "主会话通过 sunshine channel 协调 implement/check 工作线程的工作流变体",
		Path:        "bundled:workflows/channel-driven-subagent-dispatch/workflow.md",
		Source:      SourceBundled,
	},
}

// ListBundled 返回两个内置模板（按注册顺序）。
func ListBundled() []Template {
	out := make([]Template, len(bundledTemplates))
	copy(out, bundledTemplates)
	return out
}

// IsBundled 判断 id 是否为内置模板。
func IsBundled(id string) bool {
	for _, t := range bundledTemplates {
		if t.ID == id {
			return true
		}
	}
	return false
}

// BundledContent 读内置模板的中文正文。
func BundledContent(id string) (string, error) {
	for _, t := range bundledTemplates {
		if t.ID != id {
			continue
		}
		if id == NativeID {
			data, err := templates.Workflows.ReadFile("workflows/native/workflow.md")
			if err != nil {
				return "", fmt.Errorf("读取内置工作流模板 %s 失败：%w", id, err)
			}
			return string(data), nil
		}
		data, err := templates.Workflows.ReadFile("workflows/" + id + "/workflow.md")
		if err != nil {
			return "", fmt.Errorf("读取内置工作流模板 %s 失败：%w", id, err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("未知内置工作流模板 %q", id)
}

// bundledTemplate 返回内置模板的元数据。
func bundledTemplate(id string) (Template, bool) {
	for _, t := range bundledTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Resolve 把模板 id 解析为正文。
//
//   - 内置 id（native、channel-driven-subagent-dispatch）且未传
//     marketplace 来源时，直接读内置中文模板（离线可用）。
//   - 传了 marketplace 来源时，非 native 的内置 id 也按上游行为从该市场
//     拉取（与 Trellis resolveWorkflowTemplate 一致：只有 native 是虚拟
//     内置项，其余 id 一律走市场）；native 仍用内置模板。
//   - 未知 id 且未传来源时报错（不回落到 Trellis 的默认市场——
//     那会拉回英文 Trellis 模板；Sunshine 尚无自己的默认市场）。
//   - 未知 id 且传了来源时，从该市场拉取。
func Resolve(id, marketplaceSource string) (Resolved, error) {
	if id == NativeID {
		content, err := BundledContent(id)
		if err != nil {
			return Resolved{}, err
		}
		t, _ := bundledTemplate(id)
		return Resolved{Template: t, Content: content}, nil
	}
	if marketplaceSource == "" {
		if id == RetiredTDDID {
			return Resolved{}, fmt.Errorf("TDD 已改为任务级模式，不再是工作流模板：建任务时给任务选 TDD 模式即可（sunshine task create ... --meta mode=tdd）；已在用 tdd 模板的项目不受影响，项目里的 workflow.md 继续有效，想换回合并版用 sunshine workflow --template native --force")
		}
		if t, ok := bundledTemplate(id); ok {
			content, err := BundledContent(id)
			if err != nil {
				return Resolved{}, err
			}
			return Resolved{Template: t, Content: content}, nil
		}
		return Resolved{}, fmt.Errorf("未知工作流模板 %q，可用模板：native、channel-driven-subagent-dispatch", id)
	}
	return ResolveFromMarketplace(id, marketplaceSource)
}

// ListTemplates 列出可用模板：内置两项在前，marketplace 来源给定时
// 追加其 index.json 里的 workflow 条目。远程拉取失败时只返回内置项
// 并附 errorMessage（与上游 listWorkflowTemplates 一致：离线仍可
// 回落到内置 native 项）。
func ListTemplates(marketplaceSource string) ([]Template, string) {
	list := ListBundled()
	if marketplaceSource == "" {
		return list, ""
	}
	entries, errMsg := fetchMarketplaceIndex(marketplaceSource)
	if errMsg != "" {
		return list, errMsg
	}
	for _, e := range entries {
		if e.Type != "workflow" || e.ID == NativeID {
			continue
		}
		list = append(list, Template{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Path:        e.Path,
			Source:      SourceMarketplace,
		})
	}
	return list, ""
}
