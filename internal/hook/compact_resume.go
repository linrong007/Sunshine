// compact 后状态重注（squeez 增量机制，2026-11-19）：只在
// SessionStart 输入 source == "compact" 时追加一个
// `<compact-resume>` 指针块（≤4000 字符）；其余 source
// （startup/resume/clear/fork）与取不到 source 时一律不注。
// clear 不另加指针块：现有注入已含当前任务，再贴是重复。
//
// 指针不落盘：重注时直接从磁盘读现有状态拼（会话指针、
// task.json、activity.jsonl），复用 session_start.go 已有的
// 任务解析 helper。source=compact 时删本会话的
// dedup-<session>.jsonl（compact 前调用的输出模型已看不见，
// 旧引用会指到不存在的内容；删失败静默）。
package hook

import (
	"os"
	"path/filepath"
	"strings"
)

// compactResumeMaxChars 是指针块的最大字符数。
const compactResumeMaxChars = 4000

// compactResumeMaxCriteria 是关键约束最多取的验收标准条数。
const compactResumeMaxCriteria = 3

// compactResumeCriterionMaxChars 是每条验收标准的最大字符数。
const compactResumeCriterionMaxChars = 80

// compactDeleteDedup 在 source=compact 时删本会话的去重表；
// 删失败静默（下次写盘会重建）。
func compactDeleteDedup(root string, input HookInput) {
	if root == "" {
		return
	}
	sessionID := GetString(input, "session_id", "sessionId", "sessionID")
	if sessionID == "" {
		return
	}
	_ = os.Remove(dedupFilePath(root, sessionID))
}

// buildCompactResumeBlock 拼 `<compact-resume>` 指针块：当前任务
// （会话指针里的 current_task + 任务标题/状态）+ 下一步（任务
// 状态与最近 activity.jsonl 事件推导）+ 关键约束（任务 prd.md
// 的验收标准节，最多 3 条，每条 ≤80 字符；取不到就空着）。
// 全块 ≤4000 字符；不在指针里贴全文、不贴 jsonl 原文。
func buildCompactResumeBlock(root string, active ActiveTask) string {
	var lines []string
	title, status := "", "unknown"
	var taskDir string
	if active.TaskPath != "" {
		taskDir = ResolveTaskRef(active.TaskPath, root)
		title = active.TaskPath
		if taskDir != "" {
			if data := ReadJSONFile(filepath.Join(taskDir, "task.json")); data != nil {
				if t, ok := data["title"].(string); ok && t != "" {
					title = t
				}
				if s, ok := data["status"].(string); ok && s != "" {
					status = s
				}
			}
		}
		if active.Stale {
			status = "stale"
		}
		lines = append(lines, "当前任务："+title+"（"+active.TaskPath+"，状态 "+status+"）")
	} else {
		lines = append(lines, "当前任务：无")
	}
	lines = append(lines, "下一步："+compactNextStep(root, active, status))
	if taskDir != "" {
		if criteria := compactAcceptanceCriteria(filepath.Join(taskDir, "prd.md")); len(criteria) > 0 {
			lines = append(lines, "关键约束：")
			for _, c := range criteria {
				lines = append(lines, "- "+c)
			}
		}
	}
	block := "<compact-resume>\n" + strings.Join(lines, "\n") + "\n</compact-resume>"
	if r := []rune(block); len(r) > compactResumeMaxChars {
		// 先截内容再包标签：总长仍 ≤4000 字符，且闭合标签永在。
		body := strings.Join(lines, "\n")
		keep := compactResumeMaxChars - len([]rune("<compact-resume>\n\n</compact-resume>"))
		if keep < 0 {
			keep = 0
		}
		br := []rune(body)
		if len(br) > keep {
			br = br[:keep]
		}
		block = "<compact-resume>\n" + string(br) + "\n</compact-resume>"
	}
	return block
}

// compactNextStep 按任务状态与最近 activity 事件推导下一步。
func compactNextStep(root string, active ActiveTask, status string) string {
	if active.TaskPath == "" {
		return "确认本轮是否值得建任务。"
	}
	switch status {
	case "planning":
		return "完成规划后启动实现（先读 prd.md；设计先行父任务先过 1.5 评审，开发先行与子任务配完上下文直接 start）。"
	case "completed":
		return "运行收尾流程并归档。"
	case "stale":
		return "会话指针指向的任务目录已不存在，先清理指针再确认下一步。"
	case "in_progress":
		events, err := ReadActivityEvents(root, 1)
		if err == nil && len(events) > 0 {
			last := events[len(events)-1]
			switch last.Kind {
			case ActivityKindStopped:
				if last.Agent == "check" {
					return "复核检查结果，完成后归档并启动下一子任务。"
				}
				return "复核 " + last.Agent + " 子代理的完成结果，跑检查。"
			case ActivityKindStarted:
				return "等待 " + last.Agent + " 子代理完成。"
			}
		}
		return "继续执行 implement.md 中的当前步骤。"
	default:
		return "按任务状态继续推进。"
	}
}

// compactAcceptanceCriteria 从 prd.md 的验收标准节取最多 3 条
// 验收标准，每条 ≤80 字符；找不到该节返回空。
func compactAcceptanceCriteria(prdPath string) []string {
	content := ReadFile(prdPath, "")
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	inSection := false
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if inSection {
				break
			}
			if strings.Contains(trimmed, "验收") {
				inSection = true
			}
			continue
		}
		if !inSection {
			continue
		}
		item := compactCriterionLine(trimmed)
		if item == "" {
			continue
		}
		out = append(out, truncateRunes(item, compactResumeCriterionMaxChars))
		if len(out) >= compactResumeMaxCriteria {
			break
		}
	}
	return out
}

// compactCriterionLine 从一验收行取条目文本：去 "- "、"* "、
// "1. " 这类列表标记；非列表行返回 ""。
func compactCriterionLine(trimmed string) string {
	for _, prefix := range []string{"- ", "* "} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	// 数字序号 "1. xxx" / "1、xxx"。小数点后跟数字的不算序号
	//（"1.5 倍性能提升" 不是序号 "1." + "5 倍性能提升"）。
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i > 0 && i < len(trimmed) {
		rest := trimmed[i:]
		for _, sep := range []string{".", "、"} {
			if strings.HasPrefix(rest, sep) {
				after := strings.TrimPrefix(rest, sep)
				if sep == "." && after != "" && after[0] >= '0' && after[0] <= '9' {
					continue
				}
				if item := strings.TrimSpace(after); item != "" {
					return item
				}
			}
		}
	}
	return ""
}

// truncateRunes 按字符数截断（不切断多字节字符）。
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}
