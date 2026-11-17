package adapters

import "github.com/linrong007/Sunshine/internal/mem"

// All 按固定顺序返回 8 个会话适配器：
// claude、codex、devin、grok、opencode、pi、zcode、dsh。
func All() []mem.Adapter {
	return []mem.Adapter{
		ClaudeAdapter{},
		CodexAdapter{},
		DevinAdapter{},
		GrokAdapter{},
		OpencodeAdapter{},
		PiAdapter{},
		ZcodeAdapter{},
		DshAdapter{},
	}
}
