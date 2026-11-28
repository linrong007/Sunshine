# Channel 驱动的子代理派发工作流

---

## 核心原则

1. **先规划再写代码**——实现之前先定好任务、规划产物与验收标准。
2. **主会话负责协调**——主会话澄清需求、规划任务、派发工作线程（worker）、更新规范、提交并收尾。
3. **实现与检查跑在 channel 工作线程里**——默认用 `sunshine channel spawn` 派发 implement/check 工作线程，而不是宿主原生的子代理。
4. **上下文显式传递**——工作线程上下文顺序是 `jsonl 条目 -> prd.md -> design.md -> implement.md`。
5. **结果可审计**——用 `sunshine channel messages --raw` 看工作线程事件；美化输出只是操作员看板，可能会截断进度。
6. **决策落盘**——需求、研究、计划与复核结论都属于任务文件。
7. **引用不猜**——用户用口头名指任务、需求或原型页时，先对任务清单与需求列表核对；对上多个候选就不许猜，列出候选（带中文标题 / 需求名）请用户点名。

---

## Sunshine 系统

### 开发者身份

首次使用时初始化你的身份：

```bash
sunshine init -u <你的名字>
```

### 规范系统

`.sunshine/spec/` 下所有 `.md` 文件都是规范：默认三份（`system.md`、`frontend.md`、`backend.md`），也可以是模板市场拉来的 `api.md`、`ui.md` 等，全部认。

每个规范文件头上有一行 `适用：前端 / 后端 / 通用`，写明这份规范给谁读：前端任务读前端组 + 通用组，后端任务读后端组 + 通用组。`guides/` 下是跨前后端的共享指南。写代码之前，先看规范地图，加载与本任务相关的那几份：

```bash
sunshine task context --mode packages   # 列出规范地图：按适用分组，一行一个
```

### 代码地图（CodeGraph，仅存量项目）

存量项目（已有代码）在 `sunshine init` 时会供给 CodeGraph 代码地图：机器没装就装、项目没图就建（`.codegraph/`），并把查询接进平台。用法只有三处：

- **整理规范与研究**：给存量项目落 spec（spec-bootstrap）或做 1.3 研究时，research 先查图摸清符号、调用关系与结构，再读文件落字；spec 与代码对不上时也先查图核对。
- **实现**：implement 动手前先查任务区域，改已有函数/方法前先查调用方确认波及面。
- **检查**：check 用图查受影响的测试与改动波及面，作为对账中超范围判定与回归范围的证据。

查询两路：平台支持 MCP 时用 `codegraph_explore` / `codegraph_context` / `codegraph_callers` / `codegraph_impact` 等工具；不支持时用等价的命令行 `codegraph explore <问题>`、`codegraph context <任务>`、`codegraph callers <符号>`、`codegraph impact <符号>`、`codegraph affected`。项目没有 `.codegraph/`（新建项目，或 init 时用 --no-codegraph 关掉）就跳过本节，按原有方式检索与读文件。图的同步与 CodeGraph 自身的升级不归 Sunshine 管。

### 任务系统

每个任务在 `.sunshine/tasks/{MM-DD-名称}/` 下有自己的目录，内含 `task.json`、`prd.md`、可选的 `design.md`、可选的 `implement.md`、可选的 `research/`，以及 `implement.jsonl` / `check.jsonl`。

常用命令：

```bash
sunshine task create "<标题>" [--slug <名称>] [--parent <父目录>]
sunshine task start <名称>
sunshine task current --source
sunshine task finish
sunshine task archive <名称>
sunshine task validate <名称>

# 待验单（check 落下本任务内验不了的集成级条目时立单；签满才许归档）
sunshine task verify <名称>                        # 看待验单
sunshine task verify-open <名称> --item <条目> --standard <验过标准> [--verifier <验证任务>]
sunshine task verify-assign <名称> <编号> --to <验证任务>     # 指名/改派验证任务
sunshine task verify-sign <名称> <编号> --result <验通|验不过|仍无法验> --by <验证任务> --evidence <证据>
sunshine task verify-waive <名称> <编号> --reason <作废原因>   # 用户批准后作废
sunshine task verify-reopen <名称> <编号> [--reason <原因>]    # 需求变更把前提改掉时重开
```

### Channel 系统

Channel 是工作线程协作与事件审计层。临时实现/检查 channel 用 `--ephemeral`。持久讨论板用 `--type forum`；`thread` 是 forum 里的一项。

稳定的工作线程代号：

- `implement`——实现工作线程
- `check`——默认检查工作线程
- `check-cc`——Claude 检查工作线程
- `check-cx`——Codex 检查工作线程

---

<!--
  WORKFLOW-STATE 面包屑契约

  [workflow-state:STATUS] 块是每轮提示词注入的唯一来源。
  不要删除标签或改动标签语法。正文可以改；解析器不应该改。
-->

## 阶段索引

```
第 1 阶段：规划   -> 分类、征得创建任务的同意，然后写规划产物
第 2 阶段：执行   -> 通过 sunshine channel 工作线程实现/检查；mode=tdd 的任务先出外环验收题再逐片实现（见 2.1 双环分支）
第 3 阶段：收尾   -> 核查、更新规范、提交、收尾
```

### 需求分流

- 简单对话或小任务：只问这一轮是否要创建一个 Sunshine 任务。用户说不，就跳过本会话的 Sunshine。
- 复杂任务：问是否可以创建一个 Sunshine 任务并进入规划。用户说不，就不要大范围直接实现。
- 用户同意创建任务，不等于同意开始实现。实现要等产物复核通过并运行 `task start` 之后。
- **点名即同意**：用户已在开场点名 Sunshine 工作流的，视为已同意创建任务，不再问，直接建任务进规划（选模式等后续确认照常）。没点名才问，且只问「是否创建任务并进入规划」这一个是非题——不许把「不建任务、直接写代码」做成选项。复杂任务被拒绝时只有解释、澄清范围、建议拆小三条路，不许直接动手写代码。

### 规划产物

- `prd.md`——需求、约束与验收标准。
- `design.md`——复杂任务的技术设计。
- `implement.md`——复杂任务的执行计划、验证命令、复核关卡与回滚点。
- `implement.jsonl` / `check.jsonl`——工作线程上下文清单。把规范与研究文件放这里，不放代码文件。

轻量任务可以只有 PRD。复杂任务在 `task start` 之前必须有 `prd.md`、`design.md` 与 `implement.md`。

### 父 / 子任务树

一个请求包含多个可独立验证的交付物时，用父任务。子任务拥有可独立规划、实现、检查、归档的交付物。父/子结构不是依赖系统；依赖必须写进子任务的 `prd.md` / `implement.md`。

[workflow-state:no_task]
无当前任务。先对本轮请求分类，征得创建任务的同意，再创建任何 Sunshine 任务。
简单对话 / 小任务：只问这一轮是否要创建一个 Sunshine 任务。用户说不，就跳过本会话的 Sunshine。
复杂任务：问用户是否可以创建一个 Sunshine 任务并进入规划阶段。用户说不，就先解释、澄清范围，或建议拆成更小的任务。
用户已在开场点名工作流的，视为已同意，不再问；没点名才问，且只问是否建任务，不许给「跳过流程直接写代码」的选项。
[/workflow-state:no_task]

### 第 1 阶段：规划

- 1.0 创建任务 `[required · once]`
- 1.1 需求探索 `[required · repeatable]`（先定设计先行还是直接开发）
- 1.2 任务拆解 `[optional · once]`（多个可独立验证的交付物时，拆成父任务加子任务）
- 1.3 研究 `[optional · repeatable]`
- 1.4 配置上下文 `[required · once]`（仅支持子代理派发的平台）
- 规划关卡（仅 mode=tdd 的任务）：在 `task start` 之前记录可观测的行为切片、待测公共接口与 mock 边界。这批行为切片同时是第 2 阶段外环出题的输入。
- 1.5 需求评审与激活任务 `[required · once]`（评审仅设计先行父任务：拆解前审设计包、通过才许拆解；开发先行与子任务不评审，配完上下文直接 `task start`；status → in_progress）
- 1.6 完成标准

[workflow-state:planning]
加载 `sunshine-brainstorm`；留在规划阶段。
任务模式先看 task.json 的 `meta.mode`（fast / standard / tdd；没标按 standard，老任务标了 `risk=light` 的按 fast 读）：fast 只免 1.1 多轮澄清，其余照常；tdd 的任务在 `task start` 之前记好行为切片（见 1.1），第 2 阶段先出外环验收题再实现。
轻量任务：`prd.md` 可能就够了。复杂任务：写完 `prd.md`、`design.md` 和 `implement.md`。评审只有设计先行父任务有：拆解前先过 1.5 评审（逐条核对 + 缺失项扫描 + 必须确认清零 + 用户确认），通过后才许拆解；需求循环收尾只问是否进入评审，最终规划总结不算评审，评审未过前不得问用户是否拆解或启动，不得把拆解与实现捆成一个批准问题。开发先行的任务与设计先行的子任务不走评审，1.4 配完上下文直接 `task start`。
多交付物范围：考虑父任务加多个可独立验证的子任务；依赖必须写进子任务产物，不能只靠树的位置暗示。
Channel 工作线程模式：开始前把 `implement.jsonl` 和 `check.jsonl` 整理成规范/研究清单。
[/workflow-state:planning]

[workflow-state:planning-inline]
加载 `sunshine-brainstorm`；留在规划阶段。
任务模式先看 task.json 的 `meta.mode`（fast / standard / tdd；没标按 standard，老任务标了 `risk=light` 的按 fast 读）：fast 只免 1.1 多轮澄清，其余照常；tdd 的任务在 `task start` 之前记好行为切片（见 1.1），第 2 阶段先出外环验收题再实现。
轻量任务：`prd.md` 可能就够了。复杂任务：写完 `prd.md`、`design.md` 和 `implement.md`。评审只有设计先行父任务有：拆解前先过 1.5 评审（逐条核对 + 缺失项扫描 + 必须确认清零 + 用户确认），通过后才许拆解；需求循环收尾只问是否进入评审，最终规划总结不算评审，评审未过前不得问用户是否拆解或启动，不得把拆解与实现捆成一个批准问题。开发先行的任务与设计先行的子任务不走评审，1.4 配完上下文直接 `task start`。
多交付物范围：考虑父任务加多个可独立验证的子任务；依赖必须写进子任务产物，不能只靠树的位置暗示。
Inline 模式：跳过 jsonl 整理；第 2 阶段通过 `sunshine-before-dev` 读产物/规范。
[/workflow-state:planning-inline]

### 第 2 阶段：执行

- 2.1 实现 `[required · repeatable]`
- 2.2 质量检查 `[required · repeatable]`
- 2.3 回滚 `[on demand]`

Channel 驱动的子代理派发是本工作流的默认执行模型。主会话用 `sunshine channel create`、`sunshine channel spawn`、`sunshine channel send` 与 `sunshine channel wait` 协调工作线程。只有用户明确要求原生派发，或必须用宿主专属能力时，才回退到宿主原生子代理。

**交付验收封顶**：工作线程（或回退派发的子代理）交付后，主会话的验收只做三件事——读它的落盘汇报、按汇报核对产物清单齐不齐、抽查其中一个产物确认无误，到此为止。不逐项复核交付内部的接线与细节（链接、锚点、键名、数据对应关系等），那是 check 工作线程对账的活；不做渲染或截图验证，DeepSeek Harness 上模型读不了图片、截图一律跳过。验收中发现具体问题，记下来退回对应工作线程修，不自己下场逐项验。没有落盘汇报的交付视为未完成，先补汇报再验收。

[workflow-state:in_progress]
流程：channel 驱动的 `implement` 工作线程 -> channel 驱动的 `check` 工作线程 -> `sunshine-update-spec` -> 提交（第 3.4 步）-> `/sunshine:finish-work`。
**模式路由**：先看当前任务的 `meta.mode`。mode=tdd 的任务走双环：先 spawn 出题工作线程写外环验收题（只看任务产物、不看实现代码，先红验证后锁定）-> implement 工作线程逐个行为红灯 -> 绿灯 -> 重构 -> `check` 工作线程（含外环判卷）；其余模式走上面的流程。mode=fast 与 standard 在第 2 阶段走法相同（差别只在第 1 阶段免逼问）。

**子任务顺序固定**：一个子任务的生命周期顺序是固定的，不随开场提示词的措辞而变：实现 → 检查 → 回写 spec → 提交 → 归档。检查验实现、回写 spec 在提交之前、归档在最后一步；提示词说“实现完直接归档/提交”也不跳过检查与回写。
主会话默认：用 `sunshine channel spawn` 配合 `.sunshine/agents/implement.md` 与 `.sunshine/agents/check.md`；除非明确要求或宿主专属工具必需，否则不要用原生的 Claude Task / Codex sub_agent。派发指实际执行 spawn，只在叙述里走过流程不算；发现没派发就动了手，立即停手补派。
工作线程上下文顺序：jsonl 条目 -> `prd.md` -> `design.md（如有）` -> `implement.md（如有）`。jsonl 条目只读 reason 指明的部分；带 `lines` 的条目注入时只切该行范围（1-based 闭区间），无 `lines` 的大文件先 grep 定位。用 `implement`、`check`、`check-cx`、`check-cc` 这类稳定的工作线程代号；需要精确结果时用 `sunshine channel messages --raw` 读结果。
[/workflow-state:in_progress]

[workflow-state:in_progress-inline]
流程：`sunshine-before-dev` -> 改代码 -> channel 驱动的 `check` 工作线程 -> 验证 -> `sunshine-update-spec` -> 提交（第 3.4 步）-> `/sunshine:finish-work`。
mode=tdd 的任务在 inline 下同样先出题：先 spawn 出题工作线程写外环验收题并锁定后，主会话才开始自己实现（见 2.1 双环分支）。
只有用户要求 inline，或改动太小不值得派工作线程时，才允许 inline 实现。改完代码后，优先用 `sunshine channel spawn --agent check` 做独立复核。
改代码前读上下文：`prd.md` -> `design.md（如有）` -> `implement.md（如有）`，再加上 skill 加载的相关规范/研究。
[/workflow-state:in_progress-inline]

### 第 3 阶段：收尾

- 3.2 调试复盘 `[on demand]`
- 3.3 规范更新 `[required · once]`
- 3.4 提交改动 `[required · once]`
- 3.5 收尾提醒

> 注意：步骤 3.1 已并入 2.2（最后一轮的全范围检查）与 3.4（提交前导说明）。编号保持不变，以免破坏外部引用。

[workflow-state:completed]
代码已提交。运行 `/sunshine:finish-work`；如果工作区还脏，先回第 3.4 步。
[/workflow-state:completed]

---

## 规则

1. 先确认当前在哪个阶段，再从该阶段的下一步继续。
2. 每个阶段内按顺序执行步骤；`[required]` 步骤不能跳过。
3. 第 2 阶段默认用 channel 工作线程。除非用户要求 inline，或任务小到够用，否则不要在主会话里直接做大的改动。
4. 工作线程简报必须写明当前任务、目标、可编辑范围、验证命令与禁止事项。
5. `sunshine channel messages --raw` 是精确的审计路径；美化输出只用于快速看状态。
6. 工作线程完成后，主会话整合结果，并在需要时跑检查工作线程。最终判断权在主会话。

### 当前任务路由

[Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

- 规划或需求不清 -> `sunshine-brainstorm`。
- `in_progress` 的实现 -> `sunshine channel spawn --agent implement`。
- `in_progress` 的质量检查 -> `sunshine channel spawn --agent check`。
- 反复调试 -> `sunshine-break-loop`；规范更新 -> `sunshine-update-spec`。

[/Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

[codex-inline, Kilo, Antigravity, Devin]

- 规划或需求不清 -> `sunshine-brainstorm`。
- 改代码前 -> `sunshine-before-dev`；改完后 -> 优先用 channel 驱动的 `check` 工作线程。
- 反复调试 -> `sunshine-break-loop`；规范更新 -> `sunshine-update-spec`。

[/codex-inline, Kilo, Antigravity, Devin]

---

## 第 1 阶段：规划

目标：澄清需求、征得创建任务的同意，并产出实现前必须复核的规划产物。

#### 开始纪律（第 1 阶段全程适用）

- **只读本项目**：规划阶段的上下文只来自当前项目——任务文件、`.sunshine/spec/`、项目 `doc/` 与用户本轮点名的文件。不读其它项目、历史 demo 的设计产物（需求列表、prd、原型、概要设计）作为本项目的设计依据，除非用户点名要参考；别的项目的答案不是本项目的澄清。
- **开局不全盘扫描**：开始阶段按 start 入口的清单加载（任务状态、工作流概览、spec 地图），工作流按当前阶段读对应节，不全文通读、不漫游目录找参考；问出第一个问题之前，不做项目外检索。

#### 1.0 创建任务 `[required · once]`

征得创建任务的同意后才能创建任务目录：

```bash
sunshine task create "<任务标题>" --slug <名称>
```

这里只运行 `create`，不要顺手运行 `start`。`start` 会把 status 切到 `in_progress`，面包屑随即进入执行阶段。

**选模式（建任务时定，三种模式选一种）**：开发先行在 `create` 之前弹结构化提问让用户选（用户开场已点名模式的就不弹，直接按点名的记）：标题「这个任务用哪种方式做？」，三个选项各带一行说明——快速模式「和默认模式一样，只是不逼问。适合想清楚了的小改动。」/ 默认模式（推荐，预选）「先问清、评审，再做、再逐条检查。适合大多数任务。」/ TDD 模式「先出验收题锁住，再写代码过题。适合复杂逻辑，最稳、最慢。」无结构化提问工具的平台退化为文字选项、文案照抄。选完 `create` 时带 `--meta mode=fast|standard|tdd` 落盘。设计先行不在此弹窗：模式随子任务清单在 1.2 拆解呈报时逐行定（呈报时附上面三行说明一次，每行写建议模式 + 一句短理由，用户一次确认）；父任务不设模式。快速模式的适用判据见 1.2（纯展示、文案、样式、配置默认值这类不改数据、不改判定逻辑、无副作用的改动），拿不准就选默认模式。

slug 要带上标题里能区分本任务的词（中文 slug 可以，同批任务不许只差一个词根，如「调休」与「请假」要分别进 slug）；`task create` 发现新名与已有任务近似时会警告，收到警告就换个更有区分度的名字。

#### 1.1 需求探索 `[required · repeatable]`

加载 `sunshine-brainstorm`，把用户需求写进 `prd.md`。复杂任务还要 `design.md` 与 `implement.md`。

**先定模式**：这次走设计先行还是直接开发——开场提示词已说清的不重复问；没说清时，必须先问用户一句再往下走，不许默认（尤其不许默认直接开发）。可以检查 `doc/` 是否已有设计文档辅助判断，但不能拿它代替问。
- **设计先行**：按需求循环（需求列表 → 原型）→ 1.5 评审（审设计包：逐条核对 + 缺失项扫描 + 必须确认清零）→ 评审通过后 1.2 任务拆解 → 子任务各自走 1.1 → 1.4 → `task start` → 开发（不走评审） → 父任务收尾时交付文档经确认生成。设计先行只建一个父任务，循环在它里面跑。
- **直接开发**：没有设计环节，也没有评审，需求澄清 → 写 `prd.md`（轻量）或 `prd.md` + `design.md` + `implement.md`（复杂）→ 1.4 配上下文 → `task start` → 开发。

**行为切片（仅 mode=tdd 的任务）**：需求探索在实现开始前必须产出可观测的行为：捕获行为切片——公共接口、输入/动作、预期结果，以及要 mock 或不 mock 的边界，记进 `implement.md`；至少第一个行为切片具体到能写出失败测试之前，不要开始实现。这批行为切片是第 2 阶段外环出题的输入。

要求：

- 一次只问一个问题。
- 能调研的优先调研，而不是问用户。
- 需求变化时立即更新任务产物。
- 范围太大时走 1.2 任务拆解。
- `prd.md` 只聚焦需求与验收标准，不写实现检查清单。

**需求列表是主线与契约**：需求列表（`doc/design/需求列表.md`）贯穿全流程——设计按它产出原型，拆解按它定子任务（做什么、原型文件是哪个），开发按它实现与验收；它是产品与开发人员的契约。设计阶段产出的文档落盘在项目 `doc/`，不在 `.sunshine`；首次落盘时创建 `doc/` 目录。产品设计文档与任务目录里的技术设计 `design.md` 不是一回事，别混淆。

**不先定范围**：设计人员无法事先确定做哪几条需求——边做需求边做原型，后续还可能继续收集新需求进来。循环从开场提示词里已有的需求起步（起始批次，不是定死的范围）；共用约束（价格分存、接口格式、端口）与共用数据在首次需要时定下来；范围外边做边定边界。循环到设计人员与用户都认为当前批次够了、可以评审时停下来，进 1.5 评审；评审通过后才进 1.2 拆解。评审后来的新需求走需求变更/新增流程。

**按需求循环**：每条需求一轮。设计人员先产出需求列表：问答问清该条细节（一次一个问题，每次给 2–4 个选项，其中一个标明是推荐项并写清取舍，外加一个“自己填写”的自由输入项（平台提供结构化提问工具时必须用工具提问，见 brainstorm 提问规则）；推荐项只是建议，不是默认选项，不替用户选；对照需求维度清单一轮一轮问到范围、边界、约束、验收标准全部对齐再往下走），答案立即记进 `prd.md`，该条写进 `doc/design/需求列表.md`（R-xxx 行：描述、验收标准、对应原型页；其中判定、状态流转、异常处理的验收标准按句式写：当……时 / 如果……，系统应……，一条一个动作，并逐条编号（AC-1、AC-2……））。再根据需求列表做原型（见下）。下一条，直到当前批次够了、双方同意评审。本轮只问清该条的细节：需求列表先列全条目（标题与一句话范围即可），其它条目的未定项标 `[待定]` 挂在那条名下，不在本轮展开问，轮到那条再问；共用项（视觉密度、品牌色、共用数据口径）在首次需要的那一轮定。不要为每条需求新建任务；新开会话回来继续做下一条需求时，先跑 `/sunshine:continue` 续接当前父任务，不要 `task create`。

**流程固定**：设计阶段的流程是固定的，不随开场提示词的措辞而变：按需求循环（每条需求问清 → 记 prd.md → 写进需求列表 → 派 prototype 子代理做/改对应页），循环结束后给最终规划总结请确认（这时需求列表与原型已在 doc/ 里；交付文档只是 init 时的占位，不在这一步生成）——总结只是呈报材料、不是评审：总结里不得把「是否拆解 / 是否开始实现」列作下一步选项向用户要批准——收尾要问的下一步只有一件事：当前批次是否够了、是否进入评审；总结确认后进 1.5 评审（逐条核对 + 缺失项扫描 + 必须确认清零），评审通过后才拆解，拆解完才逐个子任务 start。开场提示词只决定做什么（项目内容、范围边界、要点名的交付物），不决定流程顺序；提示词描述的顺序与本流程不一致时，按本流程走，并在最终规划总结里向用户说明。

**问答即记**：每答完一个问题立即记进 `prd.md`，不随开场提示词的措辞而变；提示词说“问完再一起记”也不推迟。一次只问一个问题，答案立即记进 `prd.md`，再问下一个。

**交付文档（只占位，项目收尾时经确认生成）**：设计阶段只产出需求列表与原型。七份交付文档在 init 选产品设计时已生成占位（`doc/delivery/` 下的概要设计、详细设计、数据库设计、接口文档、测试报告、用户手册、部署说明），占位只有标题与章节骨架，设计与开发阶段不读不填、不当设计依据。全部子任务归档、父任务最终集成复核通过后，主会话把占位清单列给用户确认：点头哪份就生成哪份，依据 `prd.md`、需求列表、spec、各子任务的 check 对账与代码事实来写；没点名的保持占位，或按用户意思删除。用户在开场或设计阶段点名要某份文档的，该份按点名提前生成，不受本节限制。什么设计文档都没有就是纯代码任务，走直接开发。

**高保真原型派发给子代理**：原型是代码（HTML/CSS/JS + mock 数据），不要由主会话顺手写。每轮按需求派发：用 `sunshine channel spawn --agent prototype` 派发（加载 `.sunshine/agents/prototype.md`），派发消息里给出需求列表 `doc/design/需求列表.md` 的路径、spec 里前端组的设计约束（如 `.sunshine/spec/frontend.md` 里的设计约束）、以及本轮只读的原型文件（`doc/prototype/index.html` 导航入口 + `doc/prototype/assets/tokens.css` + `doc/prototype/assets/components-index.md` + `doc/prototype/assets/mock-common.js` + 本轮要做/改的那一页 + 本轮的 `doc/prototype/assets/mock-Rxxx.js`，如已有；只点名这几个文件，不要给整个 `doc/prototype/` 目录，不要读其它已有页，不要读 `components.css` 全文和其它页的 mock 文件）；子代理按需求列表当前状态产出或更新该条需求对应的页（每页一个文件 + `index.html` 导航入口），共用页在已有页上加、不重建，完成后只汇报页数、mock 记录数与文件路径，汇报同时落盘到当前父任务目录的 `prototype-<需求编号>.md`，主会话不直接写原型，只收汇报并核对落盘文件。派发协议见 `sunshine-prototype-dispatch` skill，动手前先加载。初版生成与结构性大改走子代理。主会话直接改原型文件只有一个例外：单处、单文件的文字、颜色、间距级小修，且用户明确要求主会话直接改时才可用（用 grep 定向改），模型不得自批例外；动手写原型文件前先自检：不在这个例外里的，先派发再动手，拿不准算大改、派发。

#### 1.2 任务拆解 `[optional · once]`

当一个任务包含多个可独立验证的交付物，或工作量明显超出单次实现时，拆成父任务加子任务。轻量任务、单交付物任务跳过本步。

**输入**：`prd.md` + `doc/` 里的设计文档（有什么读什么）。

**做法**：
- 读需求列表（主线），定子任务清单：每个子任务对应一条需求，独立可验证、有明确验收标准，写清做什么、原型文件是哪个。
- 为每个子任务建任务目录：`sunshine task create "<子任务标题>" --description "<一句话摘要>" --parent <父任务>`；已有任务挂成子任务用 `sunshine task add-subtask <父任务> <子任务>`。子任务 slug 带各自需求的区分词，兄弟之间不许近似（口径同 1.0 的 slug 规则）。
- 父任务记好子任务映射、跨子任务验收标准与最终集成复核。
- 子任务随后各自走 1.1（自己的 `prd.md` / `design.md` / `implement.md`）→ 1.4（自己的 jsonl）→ `task start`（不走评审）→ 第 2 阶段。

**前后端基础**：拆解时除按原型页拆需求子任务外，还要判断前后端基础工作量（选型、脚手架、数据库建模、缓存）。默认先建一个“技术选型与脚手架”子任务，排在第一个需求子任务前面；前后端选型项（前端：框架、构建工具；后端：语言、框架、数据库、缓存，按需再加消息队列、对象存储等）以 `[待定]` 标在它的 `prd.md` 里。它的 1.1 读到 `[待定]` 先问答（一次只问一个问题，每次给 2–4 个选项，其中一个标明是推荐项并写清取舍，外加一个“自己填写”的自由输入项；推荐项只是建议，不替用户选；问到选型、约束、验收全部对齐再动手），开发人员确定后记进 `prd.md` / `design.md`，再做脚手架、建库、接缓存；它归档后需求子任务再启动（依赖写进需求子任务的 `prd.md`）。前后端量小、单独一个任务是 overhead 时，待定项写进第一个需求子任务的 `prd.md`，它的 1.1 先问答定选型再动手。依据按“有什么读什么”：概要设计已有实质内容（不是占位）时参考它的模块划分、数据、技术选型三节；只是占位或没有时，按需求列表 + 父任务 `prd.md` 建。选型定了以后不再改，后续子任务沿用；要改走需求变更流程。

**拆解只做一次**：评审通过后，读需求列表（主线）一次性定好子任务清单——每个子任务对应一条需求，写清做什么、原型文件是哪个；技术选型与脚手架子任务的选型看概要设计的技术选型节（概要设计已有实质内容时）。不要做好一个页面拆一次、边设计边拆——开发进场时设计包必须是完整的。（这句说的是拆解只做一次，不是设计；设计是按需求循环的。）

**技术待定分流**：拆解时把设计阶段记在父任务 `prd.md` 与需求列表里的 `[待定]` 技术项收齐分流——全局性的（存储方式、配置方式这类跨需求共用的）归“技术选型与脚手架”子任务，跟选型一起在它的 1.1 定；只关乎单条需求的写进对应子任务的 `prd.md`，那个子任务开工时先问答或先写 `design.md` 定下来再动手。开工问答只处理本子任务的 `[待定]`，不顺带澄清其它子任务的设计；过程中新发现的范围外问题记 `[待定]` 挂到对应子任务名下，不在本任务里展开。`[待定]` 不许原样带进实现阶段靠写代码时猜。

**任务模式（建任务时定，记 `meta.mode`）**：每个任务一种模式：fast（快速）/ standard（默认）/ tdd（双环）。开发先行在 1.0 建任务前弹窗选（选项文案见 1.0）；设计先行随子任务清单在本步逐行定——主会话按判据给每个子任务提议模式、随清单呈报用户一次确认（呈报附三行说明，见 1.0）。快速模式的判据：纯展示、文案、样式、配置默认值这类不改数据、不改判定逻辑、没有外部副作用的改动；TDD 模式的判据：逻辑复杂、边界多、改错代价高（金额与判定、状态流转、数据处理）；其余选默认模式，尤其金额与判定、登录与权限、删除或覆盖数据、数据迁移、对外接口契约变更——沾一条就不许选快速。改模式用 `sunshine task set-meta <任务> mode fast|standard|tdd`：planning 阶段随便改；开工（`task start`）后不许降档（tdd → standard、standard → fast、tdd → fast 都不许），fast 升 standard 允许（补走 1.1 多轮澄清），不许升 tdd（题必须赶在代码之前，开工后再选是假双环，要走只能废弃重建）。兼容读法：老任务没有 mode、只有 `risk=light` 的，按 fast 读；`risk` 字段从此不再新标。

**快速模式流程（mode=fast 全程按此走）**：① 1.1 只把需求一句话记进 `prd.md`（改什么、验收怎么看），不展开多轮澄清；真有歧义才问，只问关键的一两个，问完即记。② 评审本就与快速模式无关：开发先行与子任务不走评审（评审只有设计先行父任务在拆解前有），在 `prd.md` 记一笔「快速模式」即可；1.4 配上下文照常，配完照常 `task start`。③ 第 2 阶段照常 spawn implement 实现、照常 spawn check，对账口径不变，check 落下的待验单照常立单、代验、签字。④ 收尾照常：有值得沉淀的规则才回写 spec，提交、归档不省——快速模式只免逼问（1.1 的多轮澄清），其余步骤与默认模式完全一样。**安全阀**：实现中发现实际沾了快速判据外任何一条，立即停手：`sunshine task set-meta <任务> mode standard` 升回默认模式，补走 1.1 多轮澄清后再继续；不许揣着快速模式把默认模式的活干完。

**验证关系预定**：拆解时顺带预判——哪些子任务（尤其技术选型与脚手架这类地基任务）会有在本任务内验不了的验收条目（要等某个功能接上才能验的集成级条目）、分别等哪个子任务验，把这层关系写进父任务 `prd.md` 的子任务映射与对应子任务的 `prd.md`。预判只是计划：立不立单以每个子任务 check 的实际对账为准（见 2.2 对账把关），没落下这类条目就不立单；地基子任务能自带一条最细端到端链路当场验掉的，就不要欠账。

**父/子规则**：
- 一个请求包含多个可独立验证的交付物时，用父任务。
- 父任务拥有原始需求、子任务映射、跨子任务验收标准与最终集成复核。
- 子任务拥有可独立规划、实现、检查、归档的实际交付物。
- 父/子结构不是依赖系统。子任务 B 依赖子任务 A 时，把顺序写进子任务 B 的 `prd.md` / `implement.md`。
- 启动拥有下一个交付物的子任务。除非父任务自身有直接实现工作，否则不要启动父任务。

**完成标准**：每个子任务独立可验证；父任务有子任务映射与跨子任务验收标准；子任务都已建好（task.json 的 parent/children 双向一致）。

#### 1.3 研究 `[optional · repeatable]`

需要研究时，把结果写到 `{TASK_DIR}/research/`。研究文件必须能被后续工作线程使用。

#### 1.4 配置上下文 `[required · once]`

快速模式（mode=fast）任务本步照常：快速模式只免 1.1 的多轮澄清，上下文照常配，见 1.2「快速模式流程」。

整理工作线程上下文清单：

- `implement.jsonl`——实现工作线程需要的规范与研究。
- `check.jsonl`——检查工作线程需要的质量规范、测试规范与研究。
- 设计文档——`doc/` 下与本任务相关的设计产物（需求列表、概要设计、高保真原型等），只放本任务用得上的那几份，不用把整个 `doc/` 列进来。
- **挑规范的办法**——先看规范地图（`sunshine task context --mode packages`），按适用分组挑：前端任务挑前端组 + 通用组，后端任务挑后端组 + 通用组；只放本任务用得上的那几份。

**规范索引**：spec 文件头上有 `## 索引` 一节时，先读索引定位到相关章节，再按需读正文；索引是导航不是替代，拿不准就读正文。

不要把代码文件放进 jsonl。工作线程在执行期间自己读代码。

**待验单派单**：配上下文时先扫一遍未销待验单（`sunshine task list` 里带 `[待验：未销 n 条]` 的任务，或逐个 `sunshine task verify <任务>` 看单）。本任务能验的条目，把对方任务的 `verify.md` 用 `add-context` 加进本任务的 `check.jsonl`（原因写「代验：<对方任务> <编号>」），派 check 时在提示里点名代验条目；条目指名本任务为验证者的必须接，不许漏。没有未销单子时本步零负担，跳过即可。

#### 1.5 需求评审与激活任务 `[required · once]`

**评审只有设计先行有**：本节的评审（逐条核对、缺失项扫描、处置回写、深度逼问）只对设计先行父任务做，在拆解前。开发先行的任务与设计先行的子任务不走评审——1.1 问清、规划产物写完、1.4 配好上下文，直接到本节末尾 `task start` 激活；实现中碰到没定的口径，停下来问清、记回 `prd.md` 再动手。

**需求评审**：先审后动。设计先行父任务在拆解前先过本节——评审对象是设计包：`prd.md`、需求列表、设计文档清单（`doc/` 里与本任务相关的那几份）与原型；下面两段（逐条核对、缺失项扫描）做完、必须确认清零、用户确认通过后，才许进 1.2 拆解。拆解完成后，子任务清单与各子任务的模式另行呈报用户确认（只是呈报确认，不是评审、不再派 audit），然后逐个子任务走自己的 1.4 与本节激活。开发先行的任务与子任务不审规划产物，写完、配完直接激活（见本节开头）。设计先行时，呈报前先逐条核对：父任务 `prd.md` 与 `doc/design/需求列表.md` 一致（每条需求的描述、验收标准、对应原型页；需求列表的变更记录与 `prd.md` 里的 `[变更]` 对得上）。不一致先补齐，不带矛盾进评审。另核对原型产出留痕：需求列表里每条需求对应的原型页，父任务目录里都要有子代理落盘的 `prototype-<需求编号>.md` 汇报文件（页数、mock 记录数、路径）；缺文件的页视为该条需求未完成，主会话自写的页（超出小修范围的）同样算未完成，补派 prototype 按现状复核或重做，有了汇报才算过评审。

**缺失项扫描（评审前必做）**：派 `sunshine-audit` 子代理专扫——它只读父任务 `prd.md` 与需求列表、只出清单不改文档、清单同时落盘到当前任务目录的 `audit.md`（设计先行时为父任务目录；无落盘视为扫描未完成），把两份文档当成待挑错的稿子逐条过一遍，专找四类缺失（平台不支持子代理派发或派发失败时，主会话按同一口径自己扫）：判定规则或术语没定义的；写了“可配置 / 灵活 / 适当”等词却没定口径的；文档内部自相矛盾的；引用了不存在的文档或数据的。另轻核技术型必问项：涉及却没定的技术口径只标未定与影响，按 audit 的分级口径归类。每条缺失项写清：背景（原文哪里含糊）、要确认的问题、建议选项、不确认会影响哪条需求（R-xxx）或哪个模块，汇成待确认清单。影响核心功能验收的，当场逐条问完（向用户提问按 brainstorm 提问规则：一次只问一个问题、给 2–4 个选项加“自己填写”；平台提供结构化提问工具时必须用工具提问，不用纯文本列选项）、记回 `prd.md`，才算评审材料齐；其余的转成 `[待定]`，在拆解时按 1.2 的技术待定分流处理。被引用的基准文档缺失时，相关数值标“不冻结”，并写明暂按什么口径执行。

**处置回写（评审收尾必做）**：清单每条都要有结局，且当场落盘——主会话在每条条目下回填「处置」行：定了的写结论 + 来源，来源只有两种才算定：`用户敲定`（用户自己选的或填的）、`用户采纳建议`（提问后用户选了建议选项）；没定的写 `[待定]` 并注明挂在哪条需求 / 哪个子任务名下。没向用户提问过的条目不许写结论，更不许把建议选项默认为结论。记回 `prd.md` 的结论后面带同一个来源标记（如「……（用户敲定）」），与 `audit.md` 两边对账。**业务决策类硬闸**：性质为业务决策的条目（金额与判定口径、业务规则、权限与范围、对外契约与承诺、数据删除与留存），处置来源不是上面两种之一的，一律视为未定；实现阶段碰到未定的业务决策，必须停下来向用户问清、按答复回填处置后才许动手，不许按建议选项默认执行。技术口径类可 `[待定]` 延后，实现碰到时先问清再动手，处置随定随回填。评审收尾自查：`audit.md` 每条都有处置行、`prd.md` 结论都带来源标记，才算评审完成。

**深度逼问（条件触发，替代当轮扫描）**：命中下列任一场合，本轮不派普通扫描，改派 `sunshine-audit` 并注明「深度逼问」（逐条把正文规则与验收标准套进句式逼问未定点，并逐条核技术型必问项；逐轮深查直到一整轮无新增、上限 4 轮；覆盖普通扫描，不再叠加派一次；派不了子代理时主会话按同一口径自己做）：设计先行父任务评审，且需求含规则型内容（金额与判定、登录与权限、状态流转、删除或覆盖数据、对外接口契约——沾一条即算）。用户可点名对任何任务补跑，也可豁免本轮（豁免记一笔进 `prd.md`）。任务级深挖另读相关 spec：已沉淀的全局规则视为已定、不重问；深挖新定下的全局规则当场回写 spec，不等 3.3。深挖清单的处置同扫描：必须确认当场问完记回（提问口径同扫描：按 brainstorm 提问规则，有结构化提问工具时用工具提问） `prd.md`，可延后转 `[待定]`。设计先行场合下，技术类未定点一律转 `[待定]` 挂技术选型子任务，不进必须确认。


产物复核通过后，启动任务：

```bash
sunshine task start <任务目录>
```

父任务不激活（除非自身有直接实现工作）：逐个激活子任务；全部子任务完成后，父任务做最终集成复核。复核读验收标准 + 各子任务的检查结果 + 接口约定，不重读整个原型。复核先扫未销待验单（`sunshine task list` 看 `[待验]` 标记）：没人认领的条目由集成复核逐条验掉并签字（`sunshine task verify-sign <任务> <编号> --by <父任务>`），或经用户批准作废；单子没清零，父任务归档会被命令拒绝。复核通过后、父任务归档前，按 1.1 的交付文档节处理占位文档：把清单列给用户确认，点头的才生成。

**子任务清单进度**：逐个激活子任务时，每归档一个子任务、或新会话续接时，先报一次子任务清单进度（`sunshine task context` 的「未完成清单」一节现成：还剩几个、正在做谁、下一个是谁），经用户确认再 start 下一个；不许默默连做到底，也不许岔开回来后漏报清单里还没做的子任务。

**需求变更（开发中途）**：需求在开发中途变了（新增或变更），不要原地改代码，也不要重开整个任务——只针对变化的部分回炉：回到 1.1 问清变化点（只问 delta，不重问已定的）→ 按顺序更新事实来源：`prd.md` 里记一笔 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）→ `doc/` 设计文档（概要设计、原型）同步改 → 受影响的子任务改 `prd.md` / `design.md` / `implement.md`，范围用 `sunshine task set-scope` 改、新文件用 `sunshine task add-context` 加进清单 → 设计先行回到 1.5 只评审变更的部分，评审通过再动手（开发先行无评审：问清变更点、记完 `[变更]`、更新产物与清单后直接继续）。只审变更时逐项核对：①父任务 `prd.md` 已记 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）；②受影响的子任务 `prd.md` / `design.md` / `implement.md` 已同步改，范围与清单已更新（`set-scope` / `add-context`）；③新文件已进 jsonl；④未受影响的子任务未动；⑤未销待验单的受影响情况：波及区域里有未销单子的，逐条定去向——继续挂、`verify-assign` 改派、经用户批准 `verify-waive` 作废、已签条目的前提被改掉的 `verify-reopen` 重开；待验（verifying）任务本身被波及的，原地回炉（`task start` 回到 in_progress），条目随复查逐条处置。设计人员反场先改 `doc/` 也可以，但退场前任务侧必须补齐——缺一项就不通过，开发不许动手。定波及面时对照父任务的子任务映射：没被波及的子任务不停；还在 planning 的直接改；已经 in_progress 的先停下手头的活，回炉完再继续（状态不用改）；已经归档的不 reopen（归档的是历史），新建子任务挂在父任务下走完整流程，描述里写清原任务与变更原因。新增需求同理：迷你一轮 1.1 → 1.2 → 1.4 → `task start`，新子任务挂父任务下。上下文清单只能加不能删（没有 remove-context），失效的条目要么手动改 jsonl，要么在 `prd.md` 里注明不用读哪份。

#### 1.6 完成标准

| 条件 | 是否必需 |
| --- | :---: |
| `prd.md` 存在 | 是 |
| 用户确认任务可以进入实现 | 是 |
| 已运行 `task start` | 是 |
| 复杂任务有 `design.md` | 是 |
| 复杂任务有 `implement.md` | 是 |
| `implement.jsonl` 与 `check.jsonl` 各至少含一条真实整理条目（预置行不算） | 是 |

运行时消费者出于兼容会容忍缺失或只有预置行的清单，但那不是规划就绪状态。

---

## 第 2 阶段：执行

目标：主会话通过 channel 工作线程，把复核通过的规划产物变成通过检查的代码。

#### 2.1 实现 `[required · repeatable]`

**双环分支（仅 mode=tdd 的任务；其余模式跳过本段，按下面各平台块派发）**：本阶段有两层测试，粒度不同、分工不同——**外环（验收题）**：动手实现之前，由出题工作线程只按任务产物把整套验收题一次写完，管「这个功能整体做对没有」；**内环（切片测试）**：实现过程中逐个行为写的红灯测试，管「这一片代码写对没有」。

外环出题（第一个行为切片之前做，只做一次）：

1. 主会话先 spawn 一个出题工作线程（简报自包含）：只给它 `prd.md`、`design.md`、`implement.md` 与规划关卡记录的行为切片，**不给实现代码、不给 implement 的上下文**。简报写明：按验收标准逐条写可执行测试，专挑边界（空、零、极值、刚好卡边界、重复提交、异常中断）；每条题注明对应的验收标准与期望值的算法来源；算不出期望值的条目不许瞎猜，列成「口径不明」清单返回。
2. 出题产物集中放在一处、与内环测试分开（按项目测试约定定目录或文件名前缀），出题汇报写明位置，主会话把位置记进 `implement.md`。**外环题从此锁定**：implement 工作线程与主会话都不许改；发现题错了只能上报——回 1.1 修订 `prd.md` 口径，再重新出题。
3. **先红验证**：锁定前把整套外环题在未实现的代码上跑一遍，结果记进出题汇报。对未实现行为的题必须以正确的理由失败；跑出来就全绿的题视为无效，退回重出。给已实现行为补的题（返工轮、中途补题）是回归守卫，不要求先红，汇报里注明即可。
4. 出题汇报带「口径不明」清单的，主会话先回 1.1 问清口径、回填 `prd.md`，再重新出题；不许揣着不明口径开始实现。

内环切片循环（只约束内环；外环题已按上面一次写完，不受「不要先把全部测试写完」之限）：一次只做一个行为切片，不要在看到失败测试之前实现多个行为。每个行为：从 `prd.md` 或 `implement.md` 挑下一个行为 → 确定要测试的公共接口（优先能表达该行为的最小用户侧或模块侧边界）→ 写一个失败测试，描述预期行为（实现开始前必须以正确的理由失败）→ 实现能让该测试通过的最小代码路径 → 运行聚焦测试，本行为对应的外环题同轮顺手跑一遍（红了先修实现，不许动外环题）→ 变绿后只有代码确实需要时才重构，每次重构后重跑聚焦测试 → 在 `implement.md` 或任务笔记里把该行为标为完成，再进入下一个行为。测试规则：测公共行为，不测私有方法或内部调用顺序；只 mock 系统边界（网络、时间、随机数、文件系统、子进程或外部服务）；边界协作者优先用依赖注入。

spawn implement 时在简报里补一句：按行为切片实现（红灯测试 → 绿灯实现 → 只在绿灯下重构）；外环验收题已锁定，只许运行、不许修改，跑不绿只许修实现。inline 分支同样先 spawn 出题工作线程写外环验收题、先红验证并锁定后，主会话才开始自己实现。

[Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

用 channel 驱动的 implement 派发：

```bash
TASK=.sunshine/tasks/<当前任务>
sunshine channel create impl-<主题> --task "$TASK" --by main --ephemeral
sunshine channel spawn impl-<主题> \
  --agent implement \
  --as implement \
  --jsonl "$TASK/implement.jsonl" \
  --file "$TASK/prd.md" \
  --file "$TASK/design.md" \
  --file "$TASK/implement.md" \
  --cwd "$PWD" \
  --timeout 60m
sunshine channel send impl-<主题> --as main --to implement --text-file /tmp/implement-brief.md
sunshine channel wait impl-<主题> --as main --kind done --from implement --timeout 60m
sunshine channel messages impl-<主题> --raw --from implement --last 20
```

文件不存在时，省略对应的 `design.md` 或 `implement.md` 的 `--file`。简报必须写明工作线程目标、禁止事项、验证命令与预期的完成摘要。

DeepSeek Harness 上每次 spawn 都加 `--provider dsh`（agent 文件默认 provider 是 claude；DSH worker 是 headless 一次一任务，`--session-id` 复用会话模拟常驻，模型用 profile 默认值）。

只有用户明确要求，或必须用宿主专属能力时，才允许回退到原生子代理。

**模式路由**：激活子任务前先看它的 task.json `meta.mode`（fast / standard / tdd；没标按 standard；老任务只有 `risk=light`、没有 mode 的按 fast 读）。三种模式的派发流程相同：implement、check 照常 spawn，回写 spec → 提交 → 归档的顺序不变。fast 与 standard 的差别只在第 1 阶段：fast 免 1.1 多轮澄清（见 1.2「快速模式流程」），派发一环不省。tdd 在第 2 阶段多走双环（见 2.1 双环分支与 2.2 外环判卷）。模式在拆解时（设计先行）或建任务时（开发先行）定（见 1.0 选模式）：开工后不许降档（fast 升 standard 须补走 1.1 多轮澄清），不许升 tdd。fast 任务实现中发现实际动了数据或判定逻辑，立即停手：改模式为 standard（`task set-meta`），补走 1.1 多轮澄清后再继续。

[/Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

[codex-inline, Kilo, Antigravity, Devin]

1. 加载 `sunshine-before-dev`。
2. 读 `prd.md`，有则读 `design.md`，有则读 `implement.md`。
3. 读相关研究。
4. 小改动可以 inline 实现；大改动仍应用 channel 工作线程。
5. 实现后进入 channel 驱动的检查。

[/codex-inline, Kilo, Antigravity, Devin]

#### 2.2 质量检查 `[required · repeatable]`

[Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

用 channel 驱动的 check 派发：

```bash
TASK=.sunshine/tasks/<当前任务>
sunshine channel create cr-<主题> --task "$TASK" --by main --ephemeral
sunshine channel spawn cr-<主题> \
  --agent check \
  --as check \
  --jsonl "$TASK/check.jsonl" \
  --file "$TASK/prd.md" \
  --cwd "$PWD" \
  --timeout 30m
sunshine channel send cr-<主题> --as main --to check --text-file /tmp/check-brief.md
sunshine channel wait cr-<主题> --as main --kind done --from check --timeout 30m
sunshine channel messages cr-<主题> --raw --from check --last 40
```

检查节食：`design.md` / `implement.md` 不随 spawn 注入，需要判断实现方式是否符合设计时，检查工作线程再按需读。

要做跨供应商的独立复核时，在同一个 channel 里派生 `check-cc`、`check-cx` 与 `check-dsh`：

```bash
sunshine channel spawn cr-<主题> --agent check --provider claude --as check-cc --cwd "$PWD" --timeout 30m
sunshine channel spawn cr-<主题> --agent check --provider codex --as check-cx --cwd "$PWD" --timeout 30m
sunshine channel spawn cr-<主题> --agent check --provider dsh --as check-dsh --cwd "$PWD" --timeout 30m
sunshine channel send cr-<主题> --as main --to check-cc --text-file /tmp/check-brief.md
sunshine channel send cr-<主题> --as main --to check-cx --text-file /tmp/check-brief.md
sunshine channel send cr-<主题> --as main --to check-dsh --text-file /tmp/check-brief.md
sunshine channel wait cr-<主题> --as main --kind done --from check-cc,check-cx,check-dsh --all --timeout 30m
```

DeepSeek Harness 上每次 spawn 都加 `--provider dsh`（同 2.1 的说明）。

检查工作线程应该直接修复明确的问题。主会话读原始事件，做最终判断。

**对账把关**：收到 check 汇报先看「验收标准对账」段——有未实现或未验证条目就不算检查通过，退回 2.1 补实现或补证据后再查；汇报里缺对账段的，视为检查未完成。check 的对账结论还要落盘：由 check 子代理写入子任务目录的 `check.md`；汇报有对账段、盘上没有 `check.md` 的，同样视为检查未完成。归档前先验 `check.md` 存在，缺了补派 check 并落盘后再归档，不许跳过。 check 汇报落下「待集成验证」条目时：主会话在收尾按条目立单——`sunshine task verify-open <本任务> --item <条目> --standard <验过标准> --verifier <验证任务>`（验证者按 1.2 的预定关系，没有预定的现场定），任务转待验；本任务照常回写 spec、提交，但不许归档，等签字。check 汇报带「代验条目」段时：主会话逐条签字——`sunshine task verify-sign <对方任务> <编号> --result <结论> --by <本任务> --evidence <证据>`；验不过的，把失败现象转告对方任务，对方 `task start` 返工（状态回 in_progress）后复验，或返工后对方自验签字。代验条目没签完，本任务自己也不许归档（命令会拦）。

**外环判卷（仅 mode=tdd 的任务）**：check 工作线程须运行锁定的外环验收题，在汇报对账段单列「外环判卷」一节，逐条列通过 / 失败；失败条目须结合代码复核再归入对账结论——实现没做到的归未实现，题错的注明题错并上报主会话（不许自行改题）。外环判卷现为观察口径：只报，不单独拦归档，不设「外环全绿才算检查通过」的硬门；但题红暴露的未实现条目仍按对账口径处理，不许放过。另核查每个已完成的行为都有测试（去掉实现会失败、经公共接口能通过）、mock 只限系统边界，不是内部实现细节。

[/Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

[codex-inline, Kilo, Antigravity, Devin]

加载 `sunshine-quality-check` 或用 channel 驱动的检查工作线程。发现问题就修复并再检查，直到全绿。mode=tdd 的任务另按外环判卷口径运行锁定的外环验收题并单列报告（只报不单独拦归档；题错只上报）。

[/codex-inline, Kilo, Antigravity, Devin]

#### 小修通道（2.2 之后，返工阶段，三种模式通用）

任务已实现、已 check、未归档时发现小处没改对，走本段，不必把 implement 与 check 整个重派。

- **固定检查点（正门）**：2.2 check 全部通过、主会话呈报 check 结论时，在问提交 / 归档之前紧接着弹结构化提问「检查已通过，有没有要小修的地方？」，选项：「没有，继续收尾（默认）/ 有小修 / 有问题要返工」。选「有小修」后用户说明改哪里，主会话再按下面的判据定性。弹窗即授权（与原型小修例外「须用户明确要求」同源）：主会话不许自行决定走小修。无结构化提问工具的平台退化为文字选项。check 通过且立了待验单的，检查点照常触发，小修与待验互不影响（任务之后转 verifying 也照常处理小修）。
- **定性分岔**：改动单处、不动数据与判定逻辑、无副作用（判据同快速模式），且是实现偏了已定口径——走小修；沾逻辑 / 数据的走标准返工（重新 spawn implement + 全量 check）；口径本身变了的先记 `[变更]` 进 `prd.md`，再按影响面返工。不够格的当场向用户说明原因、转标准返工。
- **走法**：主会话直接改，不 spawn implement；改完 spawn check 工作线程做**增量复核**（只核改动点 + 受影响的验收条目，不重跑全量对账）；问题、改动、复核结论留痕进 check 汇报；回写、提交、归档照常（提交后发现的多一笔修正提交）。任务模式不变、不重新选模式。mode=tdd 的任务追加一步：受影响的外环题重跑判卷。
- **防滥用**：小修次数以 check 汇报留痕计数；同一任务第三次小修起，检查点不再提供小修选项，转标准返工，主会话须挑明（反复改不对说明不是「小」的问题）。增量复核通过后回到收尾呈报，用户还能继续提（计次照旧），没提就确认归档。用户在检查点之外主动指出问题的，就地按同一套定性 + 选项处理，不必等到检查点。
- 已归档才发现的不在此列：不 reopen，另起新任务（小改可选快速模式）。

#### 2.3 回滚 `[on demand]`

- 检查发现 PRD 缺陷 -> 回第 1 阶段修产物，再重新执行。
- 实现工作线程跑偏了 -> 收窄简报、重新派发，或回退那部分工作。
- 需要更多研究 -> 写进 `{TASK_DIR}/research/`，再重新派发。

---

## 第 3 阶段：收尾

目标：核查质量、沉淀教训、提交工作。

#### 3.2 调试复盘 `[on demand]`

同一类问题反复出现时，加载 `sunshine-break-loop`，记录根因与预防办法。

#### 3.3 规范更新 `[required · once]`

加载 `sunshine-update-spec`，判断新模式、踩过的坑或技术决策是否应该写回 `.sunshine/spec/`。规则进 spec；规则背后的故事（为什么这样定、当时踩了什么坑）可写进 `.sunshine/memory/`（可选，挑着记；拿不准就只写 spec）。

#### 3.4 提交改动 `[required · once]`

**规范同步前导**：起草提交之前先问：本任务修 Bug 或发现的不显而易见的知识，是否应该进 `.sunshine/spec/`，让未来的你（或未来的 AI）不再重蹈覆辙？如果应该，先回第 3.3 步——规范写入属于本次任务的同一批提交，不能事后忘掉。

主会话提交工作改动。提交前，把 AI 改过的文件与未识别的脏文件分开。

```bash
git status --porcelain
git log --oneline -5
```

不要 amend。不要 push。

#### 3.5 收尾提醒

提交后，提醒用户运行 `/sunshine:finish-work`，归档任务并记录会话。 归档顺序：待验任务签满后先归档待验任务、再归档它的验证任务；`task archive` 被待验单拦下时，照命令输出把条目处理完再归档，不许绕过。

---

## 自定义 Sunshine

本工作流通过 `.sunshine/workflow.md` 自定义。脚本解析标签与标题；它们不保存兜底文案。

### 改动某个步骤

编辑对应第 1 / 2 / 3 阶段步骤的正文。

### 改动每轮提示词文本

编辑对应 `[workflow-state:STATUS]` 块的正文。不要改动标签名或语法。

### 添加自定义状态

新增：

```text
[workflow-state:my-status]
...
[/workflow-state:my-status]
```

生命周期 hook 或脚本必须把 `task.json.status` 写成该值，否则该块永远不会被读取。
