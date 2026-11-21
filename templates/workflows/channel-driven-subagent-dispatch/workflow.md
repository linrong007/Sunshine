# Channel 驱动的子代理派发工作流

---

## 核心原则

1. **先规划再写代码**——实现之前先定好任务、规划产物与验收标准。
2. **主会话负责协调**——主会话澄清需求、规划任务、派发工作线程（worker）、更新规范、提交并收尾。
3. **实现与检查跑在 channel 工作线程里**——默认用 `sunshine channel spawn` 派发 implement/check 工作线程，而不是宿主原生的子代理。
4. **上下文显式传递**——工作线程上下文顺序是 `jsonl 条目 -> prd.md -> design.md -> implement.md`。
5. **结果可审计**——用 `sunshine channel messages --raw` 看工作线程事件；美化输出只是操作员看板，可能会截断进度。
6. **决策落盘**——需求、研究、计划与复核结论都属于任务文件。

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
第 2 阶段：执行   -> 通过 sunshine channel 工作线程实现/检查
第 3 阶段：收尾   -> 核查、更新规范、提交、收尾
```

### 需求分流

- 简单对话或小任务：只问这一轮是否要创建一个 Sunshine 任务。用户说不，就跳过本会话的 Sunshine。
- 复杂任务：问是否可以创建一个 Sunshine 任务并进入规划。用户说不，就不要大范围直接实现。
- 用户同意创建任务，不等于同意开始实现。实现要等产物复核通过并运行 `task start` 之后。

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
[/workflow-state:no_task]

### 第 1 阶段：规划

- 1.0 创建任务 `[required · once]`
- 1.1 需求探索 `[required · repeatable]`（先定设计先行还是直接开发）
- 1.2 任务拆解 `[optional · once]`（多个可独立验证的交付物时，拆成父任务加子任务）
- 1.3 研究 `[optional · repeatable]`
- 1.4 配置上下文 `[required · once]`（仅支持子代理派发的平台）
- 1.5 激活任务 `[required · once]`（需求评审，然后 `task start`）
- 1.6 完成标准

[workflow-state:planning]
加载 `sunshine-brainstorm`；留在规划阶段。
轻量任务：`prd.md` 可能就够了。复杂任务：写完 `prd.md`、`design.md` 和 `implement.md`；`task start` 之前先请用户复核。
多交付物范围：考虑父任务加多个可独立验证的子任务；依赖必须写进子任务产物，不能只靠树的位置暗示。
Channel 工作线程模式：开始前把 `implement.jsonl` 和 `check.jsonl` 整理成规范/研究清单。
[/workflow-state:planning]

[workflow-state:planning-inline]
加载 `sunshine-brainstorm`；留在规划阶段。
轻量任务：`prd.md` 可能就够了。复杂任务：写完 `prd.md`、`design.md` 和 `implement.md`；`task start` 之前先请用户复核。
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

**子任务顺序固定**：一个子任务的生命周期顺序是固定的，不随开场提示词的措辞而变：实现 → 检查 → 回写 spec → 提交 → 归档。检查验实现、回写 spec 在提交之前、归档在最后一步；提示词说“实现完直接归档/提交”也不跳过检查与回写。
主会话默认：用 `sunshine channel spawn` 配合 `.sunshine/agents/implement.md` 与 `.sunshine/agents/check.md`；除非明确要求或宿主专属工具必需，否则不要用原生的 Claude Task / Codex sub_agent。派发指实际执行 spawn，只在叙述里走过流程不算；发现没派发就动了手，立即停手补派。
工作线程上下文顺序：jsonl 条目 -> `prd.md` -> `design.md（如有）` -> `implement.md（如有）`。jsonl 条目只读 reason 指明的部分；带 `lines` 的条目注入时只切该行范围（1-based 闭区间），无 `lines` 的大文件先 grep 定位。用 `implement`、`check`、`check-cx`、`check-cc` 这类稳定的工作线程代号；需要精确结果时用 `sunshine channel messages --raw` 读结果。
[/workflow-state:in_progress]

[workflow-state:in_progress-inline]
流程：`sunshine-before-dev` -> 改代码 -> channel 驱动的 `check` 工作线程 -> 验证 -> `sunshine-update-spec` -> 提交（第 3.4 步）-> `/sunshine:finish-work`。
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

#### 1.1 需求探索 `[required · repeatable]`

加载 `sunshine-brainstorm`，把用户需求写进 `prd.md`。复杂任务还要 `design.md` 与 `implement.md`。

**先定模式**：这次走设计先行还是直接开发——开场提示词已说清的不重复问；没说清时，必须先问用户一句再往下走，不许默认（尤其不许默认直接开发）。可以检查 `doc/` 是否已有设计文档辅助判断，但不能拿它代替问。
- **设计先行**：按需求循环（需求列表 → 原型）→ 1.2 任务拆解 → 子任务各自走 1.1→1.5 → 开发 → 父任务收尾时交付文档经确认生成。设计先行只建一个父任务，循环在它里面跑。
- **直接开发**：没有设计环节，需求澄清 → 写 `prd.md`（轻量）或 `prd.md` + `design.md` + `implement.md`（复杂）→ 1.4 配上下文 → 1.5 激活 → 开发。

要求：

- 一次只问一个问题。
- 能调研的优先调研，而不是问用户。
- 需求变化时立即更新任务产物。
- 范围太大时走 1.2 任务拆解。
- `prd.md` 只聚焦需求与验收标准，不写实现检查清单。

**需求列表是主线与契约**：需求列表（`doc/design/需求列表.md`）贯穿全流程——设计按它产出原型，拆解按它定子任务（做什么、原型文件是哪个），开发按它实现与验收；它是产品与开发人员的契约。设计阶段产出的文档落盘在项目 `doc/`，不在 `.sunshine`；首次落盘时创建 `doc/` 目录。产品设计文档与任务目录里的技术设计 `design.md` 不是一回事，别混淆。

**不先定范围**：设计人员无法事先确定做哪几条需求——边做需求边做原型，后续还可能继续收集新需求进来。循环从开场提示词里已有的需求起步（起始批次，不是定死的范围）；共用约束（价格分存、接口格式、端口）与共用数据在首次需要时定下来；范围外边做边定边界。循环到设计人员与用户都认为当前批次够了、可以评审时停下来，进 1.5 评审与 1.2 拆解；评审后来的新需求走需求变更/新增流程。

**按需求循环**：每条需求一轮。设计人员先产出需求列表：问答问清该条细节（一次一个问题，每次给 2–4 个选项，其中一个标明是推荐项并写清取舍，外加一个“自己填写”的自由输入项（平台提供结构化提问工具时必须用工具提问，见 brainstorm 提问规则）；推荐项只是建议，不是默认选项，不替用户选；对照需求维度清单一轮一轮问到范围、边界、约束、验收标准全部对齐再往下走），答案立即记进 `prd.md`，该条写进 `doc/design/需求列表.md`（R-xxx 行：描述、验收标准、对应原型页；其中判定、状态流转、异常处理的验收标准按句式写：当……时 / 如果……，系统应……，一条一个动作，并逐条编号（AC-1、AC-2……））。再根据需求列表做原型（见下）。下一条，直到当前批次够了、双方同意评审。本轮只问清该条的细节：需求列表先列全条目（标题与一句话范围即可），其它条目的未定项标 `[待定]` 挂在那条名下，不在本轮展开问，轮到那条再问；共用项（视觉密度、品牌色、共用数据口径）在首次需要的那一轮定。不要为每条需求新建任务；新开会话回来继续做下一条需求时，先跑 `/sunshine:continue` 续接当前父任务，不要 `task create`。

**流程固定**：设计阶段的流程是固定的，不随开场提示词的措辞而变：按需求循环（每条需求问清 → 记 prd.md → 写进需求列表 → 派 prototype 子代理做/改对应页），循环结束后给最终规划总结请确认（这时需求列表与原型已在 doc/ 里；交付文档只是 init 时的占位，不在这一步生成），评审通过后才拆解。开场提示词只决定做什么（项目内容、范围边界、要点名的交付物），不决定流程顺序；提示词描述的顺序与本流程不一致时，按本流程走，并在最终规划总结里向用户说明。

**问答即记**：每答完一个问题立即记进 `prd.md`，不随开场提示词的措辞而变；提示词说“问完再一起记”也不推迟。一次只问一个问题，答案立即记进 `prd.md`，再问下一个。

**交付文档（只占位，项目收尾时经确认生成）**：设计阶段只产出需求列表与原型。七份交付文档在 init 选产品设计时已生成占位（`doc/delivery/` 下的概要设计、详细设计、数据库设计、接口文档、测试报告、用户手册、部署说明），占位只有标题与章节骨架，设计与开发阶段不读不填、不当设计依据。全部子任务归档、父任务最终集成复核通过后，主会话把占位清单列给用户确认：点头哪份就生成哪份，依据 `prd.md`、需求列表、spec、各子任务的 check 对账与代码事实来写；没点名的保持占位，或按用户意思删除。用户在开场或设计阶段点名要某份文档的，该份按点名提前生成，不受本节限制。什么设计文档都没有就是纯代码任务，走直接开发。

**高保真原型派发给子代理**：原型是代码（HTML/CSS/JS + mock 数据），不要由主会话顺手写。每轮按需求派发：用 `sunshine channel spawn --agent prototype` 派发（加载 `.sunshine/agents/prototype.md`），派发消息里给出需求列表 `doc/design/需求列表.md` 的路径、spec 里前端组的设计约束（如 `.sunshine/spec/frontend.md` 里的设计约束）、以及本轮只读的原型文件（`doc/prototype/index.html` 导航入口 + `doc/prototype/assets/tokens.css` + `doc/prototype/assets/components-index.md` + `doc/prototype/assets/mock-common.js` + 本轮要做/改的那一页 + 本轮的 `doc/prototype/assets/mock-Rxxx.js`，如已有；只点名这几个文件，不要给整个 `doc/prototype/` 目录，不要读其它已有页，不要读 `components.css` 全文和其它页的 mock 文件）；子代理按需求列表当前状态产出或更新该条需求对应的页（每页一个文件 + `index.html` 导航入口），共用页在已有页上加、不重建，完成后只汇报页数、mock 记录数与文件路径，汇报同时落盘到当前父任务目录的 `prototype-<需求编号>.md`，主会话不直接写原型，只收汇报并核对落盘文件。派发协议见 `sunshine-prototype` skill，动手前先加载。初版生成与结构性大改走子代理。主会话直接改原型文件只有一个例外：单处、单文件的文字、颜色、间距级小修，且用户明确要求主会话直接改时才可用（用 grep 定向改），模型不得自批例外；动手写原型文件前先自检：不在这个例外里的，先派发再动手，拿不准算大改、派发。

#### 1.2 任务拆解 `[optional · once]`

当一个任务包含多个可独立验证的交付物，或工作量明显超出单次实现时，拆成父任务加子任务。轻量任务、单交付物任务跳过本步。

**输入**：`prd.md` + `doc/` 里的设计文档（有什么读什么）。

**做法**：
- 读需求列表（主线），定子任务清单：每个子任务对应一条需求，独立可验证、有明确验收标准，写清做什么、原型文件是哪个。
- 为每个子任务建任务目录：`sunshine task create "<子任务标题>" --description "<一句话摘要>" --parent <父任务>`；已有任务挂成子任务用 `sunshine task add-subtask <父任务> <子任务>`。
- 父任务记好子任务映射、跨子任务验收标准与最终集成复核。
- 子任务随后各自走 1.1（自己的 `prd.md` / `design.md` / `implement.md`）→ 1.4（自己的 jsonl）→ 1.5（自己的 `task start`）→ 第 2 阶段。

**前后端基础**：拆解时除按原型页拆需求子任务外，还要判断前后端基础工作量（选型、脚手架、数据库建模、缓存）。默认先建一个“技术选型与脚手架”子任务，排在第一个需求子任务前面；前后端选型项（前端：框架、构建工具；后端：语言、框架、数据库、缓存，按需再加消息队列、对象存储等）以 `[待定]` 标在它的 `prd.md` 里。它的 1.1 读到 `[待定]` 先问答（一次只问一个问题，每次给 2–4 个选项，其中一个标明是推荐项并写清取舍，外加一个“自己填写”的自由输入项；推荐项只是建议，不替用户选；问到选型、约束、验收全部对齐再动手），开发人员确定后记进 `prd.md` / `design.md`，再做脚手架、建库、接缓存；它归档后需求子任务再启动（依赖写进需求子任务的 `prd.md`）。前后端量小、单独一个任务是 overhead 时，待定项写进第一个需求子任务的 `prd.md`，它的 1.1 先问答定选型再动手。依据按“有什么读什么”：概要设计已有实质内容（不是占位）时参考它的模块划分、数据、技术选型三节；只是占位或没有时，按需求列表 + 父任务 `prd.md` 建。选型定了以后不再改，后续子任务沿用；要改走需求变更流程。

**拆解只做一次**：评审通过后，读需求列表（主线）一次性定好子任务清单——每个子任务对应一条需求，写清做什么、原型文件是哪个；技术选型与脚手架子任务的选型看概要设计的技术选型节（概要设计已有实质内容时）。不要做好一个页面拆一次、边设计边拆——开发进场时设计包必须是完整的。（这句说的是拆解只做一次，不是设计；设计是按需求循环的。）

**技术待定分流**：拆解时把设计阶段记在父任务 `prd.md` 与需求列表里的 `[待定]` 技术项收齐分流——全局性的（存储方式、配置方式这类跨需求共用的）归“技术选型与脚手架”子任务，跟选型一起在它的 1.1 定；只关乎单条需求的写进对应子任务的 `prd.md`，那个子任务开工时先问答或先写 `design.md` 定下来再动手。开工问答只处理本子任务的 `[待定]`，不顺带澄清其它子任务的设计；过程中新发现的范围外问题记 `[待定]` 挂到对应子任务名下，不在本任务里展开。`[待定]` 不许原样带进实现阶段靠写代码时猜。

**风险标档**：拆解时逐个子任务定风险档，两档。轻量档：纯展示、文案、样式、配置默认值这类不改数据、不改判定逻辑、没有外部副作用的改动，建任务时带 `--meta risk=light`（`sunshine task create ... --meta risk=light`）。标准档（缺省，不用标）：其余一切，尤其金额与判定、登录与权限、删除或覆盖数据、数据迁移、对外接口契约变更——沾一条就不许标轻量。标档结果随子任务清单在呈报确认时一并给用户看，用户可改档；改档用 `sunshine task set-meta <任务> risk light|standard`。

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

整理工作线程上下文清单：

- `implement.jsonl`——实现工作线程需要的规范与研究。
- `check.jsonl`——检查工作线程需要的质量规范、测试规范与研究。
- 设计文档——`doc/` 下与本任务相关的设计产物（需求列表、概要设计、高保真原型等），只放本任务用得上的那几份，不用把整个 `doc/` 列进来。
- **挑规范的办法**——先看规范地图（`sunshine task context --mode packages`），按适用分组挑：前端任务挑前端组 + 通用组，后端任务挑后端组 + 通用组；只放本任务用得上的那几份。

**规范索引**：spec 文件头上有 `## 索引` 一节时，先读索引定位到相关章节，再按需读正文；索引是导航不是替代，拿不准就读正文。

不要把代码文件放进 jsonl。工作线程在执行期间自己读代码。

#### 1.5 激活任务 `[required · once]`

**需求评审**：激活前把整包规划产物列出来请用户确认——`prd.md`、设计文档清单（`doc/` 里与本任务相关的那几份）、拆解出的子任务（走了 1.2 时）。确认后才往下走。呈报前先逐条核对：父任务 `prd.md` 与 `doc/design/需求列表.md` 一致（每条需求的描述、验收标准、对应原型页；需求列表的变更记录与 `prd.md` 里的 `[变更]` 对得上）。不一致先补齐，不带矛盾进评审。另核对原型产出留痕：需求列表里每条需求对应的原型页，父任务目录里都要有子代理落盘的 `prototype-<需求编号>.md` 汇报文件（页数、mock 记录数、路径）；缺文件的页视为该条需求未完成，主会话自写的页（超出小修范围的）同样算未完成，补派 prototype 按现状复核或重做，有了汇报才算过评审。

**缺失项扫描（评审前必做）**：派 `sunshine-audit` 子代理专扫——它只读 `prd.md` 与需求列表、只出清单不改文档、清单同时落盘到当前父任务目录的 `audit.md`（无落盘视为扫描未完成），把两份文档当成待挑错的稿子逐条过一遍，专找四类缺失（平台不支持子代理派发或派发失败时，主会话按同一口径自己扫）：判定规则或术语没定义的；写了“可配置 / 灵活 / 适当”等词却没定口径的；文档内部自相矛盾的；引用了不存在的文档或数据的。另轻核技术型必问项：涉及却没定的技术口径只标未定与影响，按 audit 的分级口径归类。每条缺失项写清：背景（原文哪里含糊）、要确认的问题、建议选项、不确认会影响哪条需求（R-xxx）或哪个模块，汇成待确认清单。影响核心功能验收的，当场逐条问完（向用户提问按 brainstorm 提问规则：一次只问一个问题、给 2–4 个选项加“自己填写”；平台提供结构化提问工具时必须用工具提问，不用纯文本列选项）、记回 `prd.md`，才算评审材料齐；其余的转成 `[待定]`，拆解时按 1.2 的技术待定分流处理。被引用的基准文档缺失时，相关数值标“不冻结”，并写明暂按什么口径执行。

**深度逼问（条件触发，替代当轮扫描）**：命中下列任一场合，本轮不派普通扫描，改派 `sunshine-audit` 并注明「深度逼问」（逐条把正文规则与验收标准套进句式逼问未定点，并逐条核技术型必问项；逐轮深查直到一整轮无新增、上限 4 轮；覆盖普通扫描，不再叠加派一次；派不了子代理时主会话按同一口径自己做）：①设计先行父任务评审，且需求含规则型内容（金额与判定、登录与权限、状态流转、删除或覆盖数据、对外接口契约——沾一条即算）；②直接开发的任务或子任务在 1.5 激活前属标准档（未标 `risk=light`，判据见 1.2 风险标档）。轻量档不触发。用户可点名对任何任务补跑，也可豁免本轮（豁免记一笔进 `prd.md`）。任务级深挖另读相关 spec：已沉淀的全局规则视为已定、不重问；深挖新定下的全局规则当场回写 spec，不等 3.3。深挖清单的处置同扫描：必须确认当场问完记回（提问口径同扫描：按 brainstorm 提问规则，有结构化提问工具时用工具提问） `prd.md`，可延后转 `[待定]`。设计先行场合下，技术类未定点一律转 `[待定]` 挂技术选型子任务，不进必须确认。


产物复核通过后，启动任务：

```bash
sunshine task start <任务目录>
```

父任务不激活（除非自身有直接实现工作）：逐个激活子任务；全部子任务完成后，父任务做最终集成复核。复核读验收标准 + 各子任务的检查结果 + 接口约定，不重读整个原型。复核通过后、父任务归档前，按 1.1 的交付文档节处理占位文档：把清单列给用户确认，点头的才生成。

**需求变更（开发中途）**：需求在开发中途变了（新增或变更），不要原地改代码，也不要重开整个任务——只针对变化的部分回炉：回到 1.1 问清变化点（只问 delta，不重问已定的）→ 按顺序更新事实来源：`prd.md` 里记一笔 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）→ `doc/` 设计文档（概要设计、原型）同步改 → 受影响的子任务改 `prd.md` / `design.md` / `implement.md`，范围用 `sunshine task set-scope` 改、新文件用 `sunshine task add-context` 加进清单 → 回到 1.5 只评审变更的部分，评审通过再动手。只审变更时逐项核对：①父任务 `prd.md` 已记 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）；②受影响的子任务 `prd.md` / `design.md` / `implement.md` 已同步改，范围与清单已更新（`set-scope` / `add-context`）；③新文件已进 jsonl；④未受影响的子任务未动。设计人员反场先改 `doc/` 也可以，但退场前任务侧必须补齐——缺一项就不通过，开发不许动手。定波及面时对照父任务的子任务映射：没被波及的子任务不停；还在 planning 的直接改；已经 in_progress 的先停下手头的活，回炉完再继续（状态不用改）；已经归档的不 reopen（归档的是历史），新建子任务挂在父任务下走完整流程，描述里写清原任务与变更原因。新增需求同理：迷你一轮 1.1 → 1.2 → 1.4 → 1.5，新子任务挂父任务下。上下文清单只能加不能删（没有 remove-context），失效的条目要么手动改 jsonl，要么在 `prd.md` 里注明不用读哪份。

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

**风险档路由**：激活子任务前先看它的 task.json `meta.risk`。标了 `risk=light`（轻量档，拆解时定，见 1.2「风险标档」）的，主会话直接实现，不 spawn implement 工作线程；check 照常 spawn，回写 spec → 提交 → 归档的顺序不变。没标的（标准档）走上面的 channel 派发。档位只在拆解时定，主会话开发时不得临场改档；轻量档实现中发现实际动了数据或判定逻辑，立即停手，改档为标准并按标准档流程补走。

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

**对账把关**：收到 check 汇报先看「验收标准对账」段——有未实现或未验证条目就不算检查通过，退回 2.1 补实现或补证据后再查；汇报里缺对账段的，视为检查未完成。check 的对账结论还要落盘：由 check 子代理写入子任务目录的 `check.md`；汇报有对账段、盘上没有 `check.md` 的，同样视为检查未完成。归档前先验 `check.md` 存在，缺了补派 check 并落盘后再归档，不许跳过。

[/Claude Code, Cursor, OpenCode, codex-sub-agent, Kiro, Gemini, Qoder, CodeBuddy, Copilot, Droid, Pi, DeepSeek Harness]

[codex-inline, Kilo, Antigravity, Devin]

加载 `sunshine-check` 或用 channel 驱动的检查工作线程。发现问题就修复并再检查，直到全绿。

[/codex-inline, Kilo, Antigravity, Devin]

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

提交后，提醒用户运行 `/sunshine:finish-work`，归档任务并记录会话。

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
