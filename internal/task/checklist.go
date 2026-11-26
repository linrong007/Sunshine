package task

import (
	"fmt"
	"sort"
	"strings"
)

// Checklist 是一个父任务的清单进度：子任务走到哪了、下一个是谁。
// 口径与 ChildrenProgress 一致：子任务不在活动集里（已归档）计完成。
type Checklist struct {
	Parent    string   // 父任务目录名
	Total     int      // 子任务总数
	Done      int      // 已完成（已归档或状态 completed）
	Doing     []string // 正在做（in_progress）的子任务，按 children 顺序
	Queued    []string // 排队等做（planning）的子任务，按 children 顺序
	Verifying int      // 待验（verifying）的子任务数
}

// Unfinished 返回还没完成的子任务数。
func (c Checklist) Unfinished() int { return c.Total - c.Done }

// Next 返回排队中的第一个子任务（children 顺序即拆解时的执行顺序）；
// 没有排队中的子任务时返回空串。
func (c Checklist) Next() string {
	if len(c.Queued) == 0 {
		return ""
	}
	return c.Queued[0]
}

// ChecklistOf 按 children 顺序计算一个父任务的清单进度。
// allStatuses 是活动任务的 {目录名: 状态}（AllStatuses 的结果）。
func ChecklistOf(parent string, children []string, allStatuses map[string]string) Checklist {
	c := Checklist{Parent: parent, Total: len(children)}
	for _, child := range children {
		st, ok := allStatuses[child]
		switch {
		case !ok || st == "completed" || st == "done":
			c.Done++
		case st == "in_progress":
			c.Doing = append(c.Doing, child)
		case st == "planning":
			c.Queued = append(c.Queued, child)
		case st == "verifying":
			c.Verifying++
		}
	}
	return c
}

// Checklists 扫全部活动任务，返回还有未完成子任务的父任务清单，
// 按父任务目录名排序。全做完的父任务不出现（清单清零就不显形）。
func Checklists(tasks []*TaskInfo) []Checklist {
	allStatuses := AllStatuses(tasks)
	var out []Checklist
	for _, t := range tasks {
		if len(t.Children) == 0 {
			continue
		}
		if c := ChecklistOf(t.DirName, t.Children, allStatuses); c.Unfinished() > 0 {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Parent < out[j].Parent })
	return out
}

// Line 渲染一行人读小结，context 的「未完成清单」一节用。
func (c Checklist) Line() string {
	parts := []string{fmt.Sprintf("%s：子任务共 %d 个，已完成 %d 个", c.Parent, c.Total, c.Done)}
	if len(c.Doing) > 0 {
		parts = append(parts, "正在做："+strings.Join(c.Doing, "、"))
	}
	if c.Verifying > 0 {
		parts = append(parts, fmt.Sprintf("待验：%d 个", c.Verifying))
	}
	if next := c.Next(); next != "" {
		parts = append(parts, fmt.Sprintf("下一个：%s（排队共 %d 个）", next, len(c.Queued)))
	}
	return strings.Join(parts, "；")
}
