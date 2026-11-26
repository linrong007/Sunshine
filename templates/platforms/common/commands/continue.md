# 继续当前任务

继续当前任务——在 `.sunshine/workflow.md` 的正确阶段/步骤处续接。

---

## 步骤 1：加载当前上下文

```bash
sunshine task context
```

确认：当前任务、Git 状态、最近提交。

同时看输出里的「未完成清单」一节（有子任务没做完的父任务才会出现）：清单走到哪、下一个是谁，以这一节为准，不靠记忆。当前任务不在任何清单里、或没有当前任务时，这一节就是续接的主要依据。

## 步骤 2：加载阶段索引

```bash
sunshine task context --mode phase
```

展示阶段索引（规划 / 执行 / 收尾）及路由与 skill 映射。

## 步骤 3：判断当前位置

`sunshine task context` 会显示当前任务的 `status` 字段。按 `status` + 产物情况分流。本命令代替用户记忆 Sunshine 流程；它本身不批准进入实现。

分流前先报清单：存在「未完成清单」时，先用一句话向用户报进度（哪个父任务、共几个子任务、已完成几个、正在做谁、下一个是谁），问用户是续当前任务还是转去做下一个；用户确认后再按下面的分流走。不要默默跳过清单里的下一个，也不要未经确认就启动它。

- `status=planning`，且没有 `prd.md` → **1.1**（加载 `sunshine-brainstorm`）
- `status=planning`，且只有 `prd.md` → 判断任务是轻量还是复杂。轻量可进入 **1.5** 评审；复杂回到 **1.1** 补充 `design.md` + `implement.md`。
- `status=planning`，规划产物齐全，但任务包含多个可独立验证的交付物且尚未拆解（children 为空）→ **1.2**（任务拆解；拆完后不要继续在父任务上走 1.4/1.5，改去子任务上走 1.1→1.5）
- `status=planning`，复杂产物齐全，但子代理 jsonl 未整理（为空，或只有旧的 `_example` 占位行）→ **1.4**
- `status=planning`，所需产物齐全，且所需 jsonl 已整理或为内联模式 → **1.5**（请用户做开始评审；只有用户确认后才运行 `sunshine task start`）
- `status=in_progress`，实现尚未开始 → **2.1**
- `status=in_progress`，实现已完成、尚未检查 → **2.2**
- `status=in_progress`，检查已通过 → **3.3**（更新规范）→ **3.4**（提交）
- `status=verifying`（待验）→ 先看待验单（`sunshine task verify <任务>`）：条目在等哪个任务验，就去推进那个任务；验证已做但没签字的，先补签字（`verify-sign`）；验不过要返工的，`sunshine task start` 把本任务回 in_progress 返工；单子签满则走归档流程
- `status=completed`（少见；通常会立即归档）→ 走归档流程

阶段规则（完整细节见 `.sunshine/workflow.md`）：

1. 同一阶段内按顺序执行步骤——`[required]` 步骤不得跳过
2. `[once]` 步骤在所需产出已存在时视为已完成。只有 `prd.md` 时，仅轻量任务足够；复杂任务还需要 `design.md` 和 `implement.md`。
3. 发现问题需要时，可以回到前面的阶段

## 步骤 4：加载具体步骤

确定续接的步骤后：

```bash
sunshine task context --mode phase --step <X.X> --platform {{CLI_FLAG}}
```

按加载的说明执行。每完成一个 `[required]` 步骤，进入下一个。

---

## 参考

完整工作流与各阶段的详细步骤见 `.sunshine/workflow.md`。本命令只是入口，权威指南在那里。
