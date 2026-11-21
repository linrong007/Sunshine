# Sunshine 重构规划（Go）

> 产品：用 Go 重新实现 Trellis 同功能的产品，内部名 Sunshine。
> 需求来源：`~/workspace/your_files/trellis-feature-requirements.md`（13 个模块、142 条功能，均附源码出处；另有 12 条未确认点）。
> 源码对照：`~/workspace/research/trellis-source`（commit `f089cb3`，`@mindfoldhq/trellis` 0.6.17）。

## 唯一硬性要求

- **CLI 界面全部中文**：帮助、提示、报错、交互提问。
- **生成的 spec 文件内容全部中文**：`.sunshine/spec/` 下的规范模板、工作流、配置注释。

## 技术选型

| 事项 | 选择 | 说明 |
| --- | --- | --- |
| 语言 | Go 1.27 | 单二进制分发、启动快、常驻 supervisor 进程管理稳健 |
| CLI 框架 | cobra | 与 Trellis 的 commander 对应，生态最成熟 |
| 配置解析 | gopkg.in/yaml.v3 | 完整 YAML 解析（Trellis 用行级解析，Go 版不沿用该取巧做法） |
| SQLite（mem 用） | modernc.org/sqlite | 纯 Go，免 cgo，跨平台编译无负担 |
| 事件存储 | 标准库（JSONL + 文件锁） | Channel 的 JSONL 事件日志与 `.seq` sidecar 直接用标准库实现 |
| 模板 | go:embed | 技能、规范、工作流模板以内嵌文件随二进制发布 |
| 交互提问 | survey/v2 | 对应 Trellis 的 inquirer |

## 目录结构

```
sunshine/
  cmd/sunshine/main.go        # 入口
  internal/cli/               # cobra 命令：root/init/version/update/…
  internal/scaffold/          # init：生成 .sunshine/ 骨架、模板哈希
  internal/config/            # config.yaml 读写
  internal/template/          # 内嵌模板（go:embed）与渲染
  internal/platforms/         # 平台适配（22 个平台，见阶段 2）
  internal/task/              # 任务系统（task.json，见阶段 3）
  internal/workflow/          # 工作流模板切换（见阶段 3）
  internal/registry/          # 模板市场与 --registry（见阶段 4）
  internal/update/            # update、迁移、模板哈希（见阶段 4）
  internal/mem/               # 跨平台会话检索（见阶段 5）
  internal/channel/           # 多 Agent 协同：事件存储、supervisor、适配器（见阶段 6）
  templates/templates.go      # 内嵌模板入口（go:embed）
  templates/sunshine/         # .sunshine/ 骨架中文模板
  templates/common/           # 通用 skill / 命令模板（中文，阶段 2）
  PLAN.md                     # 本文件
```

## 分阶段路线图

### 阶段 0：工程底座（已启动）

- Go module、目录骨架、cobra 接入、中文输出规范、version 命令。
- 验收：`go build` 通过；`sunshine --help`、`sunshine version` 全中文。

### 阶段 1：init 最小闭环（本轮正在做）

- 对应需求清单模块 3、12：`sunshine init` 生成 `.sunshine/` 骨架——`config.yaml`、`workflow.md`、`spec/` 中文规范模板、`AGENTS.md`、`.gitignore`、`.version`、`tasks/`、`workspace/`、`.template-hashes.json`（SHA-256 追踪）。
- flags：`-u/--user`、`-y/--yes`、`-f/--force`；`$HOME` 守卫（`SUNSHINE_ALLOW_HOMEDIR=1` 可绕过）。
- 验收：在空目录 `sunshine init -u <名> -y` 成功，生成文件齐全且为中文；重复 init 无 `--force` 时明确拒绝；`--force` 可覆盖。

### 阶段 2：平台适配最小集（已完成，2026-11-09）

- 对应模块 5：Claude Code、Cursor、Codex 三个平台（用户 2026-11-09 确认增加 Codex，后续再加其他平台）。
- 内容：skills / slash 命令生成、平台配置目录写入、hooks 接入。
- 验收：`init` 后 `.claude/` 与 `.cursor/` 下技能与命令就位；Agent 能按技能工作。
- 实施记录与差异见文末“阶段 2 实施记录”。

### 阶段 3：任务系统 + 工作流切换

- 对应模块 9、2：`task.json`（24 字段）读写、任务目录 `MM-DD-slug`、归档、`workflow --template`（native / tdd）。
- 完整 `config.yaml` 字段（`internal/config/`，对应模块 12）：`session_auto_commit`、hooks（`after_create` / `start` / `finish` / `archive`）、`codex.dispatch_mode`、`context_injection` 上限、`prompt_injection.skip_keyword` 等，阶段 1 的最小版在此补全。
- `workflow --create-new`（写 `.sunshine/workflow.md.new`）与 `--list`、`--force`（对应模块 2 的 workflow 命令）。
- sunshine-meta 文档适配：`sunshine-meta` 的 customize-local 与 local-architecture 参考文档改写为 Go 版现状（去掉 npm 安装、task.py、Python hooks 的说法，改成 Go 二进制与 `sunshine task` 子命令；与 task 子命令定名同步，references 里约 30 处 `task.py` 调用按映射表改写）。
- 验收：任务创建、查看、归档闭环；工作流模板可切换；`sunshine task` 子命令定名落地。

### 阶段 4：update / registry / 模板市场 / 项目生命周期

- 对应模块 4、8：`update`（模板哈希三态判断、交互确认、备份）、`--registry`（Marketplace 与直接下载两种模式）、`registry.spec` 持续刷新、迁移 manifests。
- 对应模块 2 剩余命令：`uninstall`（要求 manifest 齐全、只删托管内容、未提交用户数据警告、非交互 shell 下 `--yes` 且有未提交数据时拒绝）、`ablate`（可逆：项目外版本化恢复事务 `state.json` + `backup/`，分 stage→apply→verify→applied，失败回滚）、`restore`（精确恢复、校验事务版本与创建时间、恢复后删除事务目录）、`platforms`（按哈希清单判断已配置平台，`--json`）。
- 用户定制保证（对应截图里的定制线）：`update` 只动哈希清单里的托管文件，用户自加的命令/skill/agent/hook 不在清单里、不纳入管理、不被覆盖；`uninstall`/`ablate` 同样只动托管内容。
- 验收：模板改动能被检测并交互确认；`--registry` 能装入模板仓库；生命周期命令实测可逆且不丢用户数据。

### 阶段 5：mem 跨平台会话检索

- 对应模块 10：只读解析各平台会话库（Claude / Codex 的 SQLite 等），`mem list/search/context/extract/projects`。
- 验收：能列出并检索本机真实会话。

### 阶段 6：Channel 多 Agent 协同（最重，最后做）

- 对应模块 6、7：JSONL 事件溯源存储、21 种事件、文件锁、Inbox 策略、投递模式、supervisor、claude/codex 适配器、`channel create/send/wait/spawn/run` 等命令。
- 验收：双 worker 能通过 channel 收发消息、supervisor 能驱动任务完成。

### 阶段 7：发布

- 对应模块 1：goreleaser 打包、GitHub Releases、`upgrade` 自更新、启动时版本检查。
- CI 与 Git hooks（对应模块 1）：GitHub Actions（build / vet / gofmt / test）；pre-commit（gofmt + go vet）。

### 滚动轨道：剩余 19 个平台（阶段 2 之后分批）

- 待扩展：opencode、kilo、kiro、gemini、antigravity、devin、qoder、codebuddy、copilot、droid、dsh、pi、reasonix、zcode、trae、omp、grok、kimi、snow（共 19 个）。
- 每个平台沿用阶段 2 的适配器接口与模板管线（commands/skills/agents/hooks + SHARED_HOOKS_BY_PLATFORM 分配），逐个按需求清单模块 5 核对后接入；扩展顺序由用户定。
- 注意存疑点 5：Trellis 源码 `templates/` 下无 kilo、antigravity、devin 的独立模板目录，接入这三家时先核对是复用其他模板还是仅有 configurator。

## 确认不做 / 待确认清单

- 模块 13：无自带 MCP server——确认不做（只消费第三方 MCP）；无 TUI——确认不做；docs-site 子模块——确认不做（文档站不在重构范围）；telemetry——待确认（Trellis 自身有无数据上报未核实，见存疑点 9）。
- 存疑点逐条落点：① docs-site——确认不做；② `.version` 与包版本不一致——Go 版 `.version` 与 CLI 版本同源，阶段 7 发布流程确认；③ marketplace README 与 index.json 不符——阶段 4 核对；④ `bootstarp` 拼写——Sunshine 用正确拼写 `sunshine-spec-bootstrap`，不沿用错拼；⑤ 平台覆盖——滚动轨道核对；⑥ workspace 的 `tasks/` 描述——阶段 3 核对；⑦ workflow Phase 3 编号从 3.2 开始——阶段 3 核对原文；⑧ skill 跨平台差异——阶段 2 已按平台解析处理，见实施记录差异 7；⑨ telemetry——待确认；⑩ marketplace 子模块初始化——阶段 4 处理；⑪ WorkerRuntime 缺 `start`——已解决（关键设计决策 3）；⑫ channel store 平行副本——已解决（关键设计决策 2）。
- `ts-sdk-author` skill：Trellis 自身 TS SDK 开发用，Sunshine 版是否改为 Go SDK 编写 skill 待定（阶段 2 收尾待定项）。

## 关键设计决策

1. **Hooks 不用 Python**：Trellis 的 hooks 是 Python 脚本，要求用户机器装 Python ≥ 3.9。Go 版把 hook 逻辑做成 `sunshine hook <事件>` 子命令，各平台 hook 配置里调它，不再依赖 Python 运行时。
2. **Channel store 只留一份**：Trellis 的 CLI 里有一份 channel store 平行副本，与 core 实现有差异（需求清单存疑点 12）。Go 版从一开始就只实现一份存储。
3. **WorkerRuntime 接口先定好**：Trellis 的 core 接口缺 `start` 方法但 `spawnWorker` 调了它（存疑点 11）。Go 版先定义完整接口再写实现。
4. **不复制源码**：按需求清单重新实现行为。Trellis 是 AGPL-3.0-only；行为重实现，不直接复制代码。
5. **从哪里开始**：阶段 0 + 阶段 1（init）。所有后续功能（平台、任务、Channel）都要写进 init 创建的 `.sunshine/` 结构里，init 是用户最先触达的入口，先把它跑通，后面的阶段才有落点。

## 阶段 2 实施说明（平台适配：Claude Code、Cursor、Codex）

依据：需求清单第 5 模块；源码对照 `packages/cli/src/configurators/{claude,cursor,codex,shared}.ts`、`packages/cli/src/types/ai-tools.ts`、`packages/cli/src/templates/{common,shared-hooks,claude,cursor,codex}/`。

### 范围

- 只做 claude-code、cursor、codex 三个平台；注册表先只含这三家，后续平台按同一接口扩展。
- 不做 AI CLI 安装检测：Trellis init 也不检测 claude/codex/cursor 是否已装（只探 Python 版本与 git 用户名）；Go 版无 Python 探测。

### 命名与改名规则

- `trellis` → `sunshine`、`Trellis` → `Sunshine`、`.trellis/` → `.sunshine/`；skill 与命令前缀 `trellis-` → `sunshine-`。
- 命令引用前缀：Claude `/sunshine:`、Cursor `/sunshine-`、Codex `$`（与 ai-tools.ts 的 templateContext 一致）。
- 技术术语保留英文原词（SessionStart、PreToolUse、JSONL、SKILL.md 等）。

### 每平台文件清单

**Claude Code**（`--claude`，默认勾选；`templateContext.cmdRefPrefix=/sunshine:`、`agentCapable=true`、`hasHooks=true`）：
- `.claude/settings.json`（来自 templates/claude/settings.json；已存在时保留用户已有顶层键，只补写 Sunshine 的 hooks/env 段——合并规则以 Trellis update.ts 的 settings 合并逻辑为准，核对源码后确定）。
- `.claude/agents/sunshine-{implement,check,research}.md`（来自 templates/claude/agents/）。
- `.claude/commands/sunshine/{continue,finish-work}.md`（start 不生成：agentCapable 且 hasHooks 的平台由 SessionStart hook 自动注入工作流概览，start 命令多余——见 shared.ts filterCommands）。
- `.claude/skills/sunshine-{before-dev,brainstorm,break-loop,check,update-spec}/SKILL.md`（来自 templates/common/skills/，带 YAML frontmatter 的 name + description）。
- 4 个内置多文件 skill：`.claude/skills/sunshine-{channel,meta,spec-bootstrap,session-insight}/`（含 references，来自 templates/common/bundled-skills/）。
- 共享 hooks（按 SHARED_HOOKS_BY_PLATFORM，claude 分到 3 个）：`.claude/hooks/{session-start.py→改为 Go hook 调用，见下}`——注意：Go 版不写 `.py` 文件，hooks 由 `sunshine hook` 子命令实现，settings.json 里的命令写 `sunshine hook <事件>`。
- `--with-statusline` 时才装：settings.json 注入 statusLine 段 + `.claude/hooks/statusline.py` 对应的 Go 实现（`sunshine hook statusline`，行为按 templates/claude/hooks/statusline.py）。

**Cursor**（`--cursor`，默认勾选；`cmdRefPrefix=/sunshine-`、`agentCapable=true`、`hasHooks=true`）：
- `.cursor/commands/sunshine-{continue,finish-work}.md`（扁平命名，start 不生成，理由同上）。
- `.cursor/skills/sunshine-*/SKILL.md`（5 个自动触发 skill + 4 个内置多文件 skill，与 Claude 同源）。
- `.cursor/agents/sunshine-{implement,check,research}.md`（来自 templates/cursor/agents/）。
- `.cursor/hooks.json`（hook 配置：preToolUse Task|Subagent、sessionStart、beforeShellExecution，命令写 `sunshine hook <事件>`）。
- 共享 hooks（cursor 分到 3 个）：session-start、inject-shell-session-context、inject-subagent-context（Go 实现，不写 `.py`）。

**Codex**（`--codex`，默认不勾选；`cmdRefPrefix=$`、`agentCapable=true`、`hasHooks=false`、`supportsAgentSkills=true`）：
- `.agents/skills/sunshine-*/SKILL.md`（5 个自动触发 skill + 4 个内置多文件 skill；占位符用中性解析，命令引用写 `` `name` (Sunshine command) `` 形式，保证多平台写同一目录时字节一致——见 shared.ts resolvePlaceholdersNeutral）。
- 命令转 skill（hasHooks=false，start 保留）：`.agents/skills/sunshine-{start,continue,finish-work}/SKILL.md`。
- `.codex/agents/sunshine-{implement,check,research}.toml`（来自 templates/codex/agents/；带 marker-gated pull fallback，即子代理自行加载任务上下文的兜底说明，翻译时保留）。
- `.codex/hooks.json`（UserPromptSubmit → workflow-state；SubagentStart 匹配 `sunshine-implement|sunshine-check|sunshine-research` → subagent-context；命令写 `sunshine hook <事件>`，不要 `-X utf8`，Go 天生 UTF-8）。
- `.codex/hooks/session-start.py` 对应的 Go 实现文件不写 `.py`，但 Codex 自有的 session-start 逻辑（templates/codex/hooks/session-start.py，552 行）要读一遍：hooks.json 未注册它，init 仍按 Trellis 原样处理（不擅自“修复”接线，只记录发现）。
- `.codex/config.toml`（来自 templates/codex/config.toml；注释里的 Trellis 字样改 Sunshine，`.trellis` 改 `.sunshine`；保留 `[agents] max_depth = 1` 与 multi_agent_v2 不写块的说明）。
- 确保 `.codex/skills/` 空目录存在；重装时保留用户在 `sunshine-*.toml` 里自设的 `model` / `model_reasoning_effort`（按 codex.ts extract/apply 逻辑，逐行匹配未注释的 `key = "value"`，多行字符串内不提取）。
- init 完成后打印 Codex hooks 启用提示：用户须在 `~/.codex/config.toml` 设 `features.hooks = true`（Codex 0.129+；旧版 `codex_hooks = true`），并在 Codex 里运行一次 `/hooks` 审批，否则 hooks.json 不生效。

### `sunshine hook` 子命令（替代全部 Python hooks）

行为逐条对齐 `packages/cli/src/templates/shared-hooks/` 的 4 个脚本（session-start.py 949 行、inject-workflow-state.py 488 行、inject-subagent-context.py 1216 行、inject-shell-session-context.py 292 行）与 Codex 自有 session-start.py、Claude statusline.py：

- `sunshine hook session-start`：读 stdin 的 hook JSON，定位仓库根目录，注入结构化上下文（workflow 概览、开发者身份、git 状态、当前任务、项目规范），输出 additionalContext JSON；无 `.sunshine/` 时静默退出 0。
- `sunshine hook workflow-state`（UserPromptSubmit，每轮提示词面包屑）：内容只从 workflow.md 的 `[workflow-state:STATUS]` 标签块取，workflow.md 缺失或标签缺失时退化为通用提示行；无任务时 Codex 侧输出 bootstrap 提示；无 `.sunshine/` 静默退出 0；任务目录的 task.json 缺失/损坏/无可用状态时输出 task_error 面包屑。
- `sunshine hook subagent-context`（PreToolUse / SubagentStart）：给 implement/check/research 子代理注入任务上下文（implement.jsonl / check.jsonl + prd.md / design.md / implement.md）；research 不依赖当前任务。
- `sunshine hook shell-session`（beforeShellExecution）：待执行命令调用任务命令（Go 版：`sunshine task start/current/finish`，阶段 3 实现）时，在 `.sunshine/.runtime/shell-tickets/` 写一张 TTL 30 秒的票据，把会话身份桥接给子进程；shell 执行类事件返回 `{"permission":"allow"}`。
- `sunshine hook statusline`（仅 `--with-statusline`）：行为按 templates/claude/hooks/statusline.py。
- 通用铁律：stdin 读 JSON，容忍各平台字段大小写差异（session_id/sessionId、tool_input/toolInput 等）；stdout 只输出规定格式；任何失败都静默退出 0，绝不能阻断 agent 会话；中文任务名、中文路径必须正确处理（Go 天生 UTF-8）。

### init 接入

- flags：`--claude`、`--cursor`、`--codex`、`--with-statusline`（仅 Claude 入选时有效）。
- 交互多选（survey）：默认勾选 Claude Code 与 Cursor（与 ai-tools.ts 的 defaultChecked 一致，Codex 默认不勾选）；Claude 入选时追问是否装 statusline；`-y` 跳过全部提问用默认。
- 重复 init（已初始化项目）：进入增量模式——只追加新选平台的文件，不动已有平台文件与 `.sunshine/` 骨架。
- 生成的平台文件全部进入 `.template-hashes.json`（供阶段 4 的 update 做漂移检测）。

### 模板来源与翻译要求

- 全部模板内容来自 Trellis 源码对应文件，翻译成中文并改名，不得直接发布英文原文；技术术语保留英文原词。
- 4 个内置多文件 skill 的 references 也要翻译（sunshine-meta 体量最大，约 150KB），不得跳过、不得只翻 SKILL.md 主文件。
- 校验：生成文件里不得残留 trellis/Trellis 字样；中文标点；命令前缀与上文一致；占位符全部解析，不得残留 `{{...}}`。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- 临时目录实测：`init -y` 生成 claude+cursor 两套；`init --codex` 生成 codex 一套；`--with-statusline` 装 statusline；重复 init 追加平台不丢已有文件。
- 每个 hook 子命令用模拟的 stdin JSON 实测：有 `.sunshine/` 时输出上下文，无 `.sunshine/` 时静默退出 0。
- 生成文件抽查：中文、无 trellis 残留、路径与清单一致。

## 阶段 2 实施记录（2026-11-09，已完成）

分工：A 组平台注册表与适配器 + 小模板翻译；B 组 4 个内置多文件 skill 全量翻译（38 个文件）；C 组 `sunshine hook` 5 个子命令；D 组 init 接入与测试；协调人合并验证。

- 代码：`internal/platforms/`（注册表、占位符解析、settings 合并、Codex model 保留）、`internal/hook/`（5 个子命令）、`internal/cli/`（init 四 flag + survey 多选 + 增量模式、`hook` 命令）、`internal/scaffold/`（新增 `RecordHashes`）、`templates/platforms/`（全部中文模板）。
- 验证：`go build`、`go vet`、`gofmt`、`go test ./...`（61 个测试）全过；临时目录实测 `init -y`（110 个文件，claude+cursor）、`init --codex`（仅 codex 一套）、`--with-statusline`、`init --codex` 增量追加（已有文件 sha256 校验完好）、settings 合并保留用户键、Codex model 保留、5 个 hook 子命令模拟 stdin JSON 实测（有/无 `.sunshine/`、有/无任务、坏 JSON、未知事件均退出 0）；生成文件 `grep -ri trellis` 零命中，残留占位符仅 `{{platform_name}}` / `{{python_cmd}}`（见差异 3）。

### 与需求清单 / 源码的差异（以源码为准）

1. **settings.json 合并**：`update.ts` 里没有通用合并逻辑，唯一特殊处理是 `preserveExistingClaudeStatusLine`（仅当磁盘已有 `statusLine`、新模板没有时才带过去），其余顶层键走哈希比对/冲突流程。Go 版 `Install` 仍按任务与 PLAN.md 指定的合并实现（已有顶层键全保留、顺序不动；缺失键按模板顺序追加；`env` 按变量名补写；`hooks` 按事件补写、条目按内容判重；已有 `statusLine` 一律保留）。**阶段 4 的 update 需另行决定沿用此合并还是走 Trellis 冲突流程，两者行为不同。**
2. **哈希清单键格式**：Trellis 的 `.template-hashes.json` 键为相对项目根的 POSIX 路径（`file-writer.ts` 的 recording 注释）。阶段 1 的 `Init` 曾把骨架文件写成相对 `.sunshine/` 的键（如 `config.yaml`），与 D 组平台文件键（项目根相对，如 `.claude/settings.json`）混用；阶段 2 已统一为项目根相对路径（`.sunshine/config.yaml`）。另 Go 版清单用 `files` 键名，Trellis v2 用 `hashes`，属有意偏差，阶段 4 沿用 Go 版格式。
3. **残留占位符**：`{{platform_name}}` / `{{python_cmd}}` 在 Trellis 解析器里也不是可解析 token（`shared.ts` 只处理 `{{CMD_REF}}`、`{{EXECUTOR_AI}}`、`{{USER_ACTION_LABEL}}`、`{{CLI_FLAG}}`、`{{PYTHON_CMD}}` 与条件块），它们只出现在 sunshine-meta 的文档措辞里（说明 resolvePlaceholders 支持哪些 token）。生成文件原样保留，与上游 Trellis 输出一致；PLAN.md“不得残留 `{{...}}`”的校验按“解析器支持的占位符”理解。
4. **Codex 旧版开关**：PLAN 称旧版 `codex_hooks = true`；源码更精确：0.129+ 上旧写法是 `features.codex_hooks = true`（可用但报弃用警告），0.129 之前只接受 `codex_hooks`。`CodexHooksHint()` 按 PLAN 措辞输出。
5. **Codex agent 的 pull fallback 不是注入的**：`codex.ts` 未调用 `applyPullBasedPreludeToml`，fallback 是三个 `.toml` 正文自带的 marker 判断 + 兜底步骤，翻译时原样保留并改名标记。
6. **Codex 自有的 session-start.py（552 行）已接线（2026-11-09 用户确认）**：Trellis 的 `.codex/hooks.json` 未注册 SessionStart 事件，属上游遗漏；Codex 官方支持 SessionStart 事件（startup/resume/clear/compact，matcher 按 `source` 过滤，输出 `additionalContext`）。Sunshine 在 `.codex/hooks.json` 增加 SessionStart 条目 → `SUNSHINE_HOOK_PLATFORM=codex sunshine hook session-start`（timeout 30，不设 matcher，全来源触发）；Go 的 `RunSessionStart` 本就输出 `hookSpecificOutput.hookEventName = "SessionStart"`，与 Codex 协议一致，无需改 hook 实现。
7. **bundled-skills 用平台解析（非中性）**写到共享 `.agents/skills/`，即使多平台共享同一目录也是如此（`resolveBundledSkills`）；只有 5 个自动触发 skill 在 Codex 侧用中性解析。
8. **重复 init**：Trellis 无 flag + `-y` 时什么都不做直接返回，交互时是三选一菜单（加平台 / 设开发者身份 / 完整重初始化），加平台时只列未配置平台；任务规格简化为增量模式（多选仍列全部三家，默认勾选 claude+cursor，已安装的跳过）；“已安装”判定 Trellis 按哈希清单里有无该平台被追踪模板，任务规格按配置目录（`.claude` / `.cursor` / `.codex`）存在判定。
9. **Hook 平台检测**：Python 靠 `sys.argv[0]` 所在目录识别平台；Go 二进制无此信号，改用 `SUNSHINE_HOOK_PLATFORM` 环境变量，hook 命令写成 `SUNSHINE_HOOK_PLATFORM=<平台> sunshine hook <事件>`（settings.json、两份 hooks.json、statusLine 命令均已加前缀）。
10. **session-start / statusline 无 `.sunshine/` 时**：Go 版统一静默退出 0（Python 原版不检查 `.trellis/` 存在，statusline 仍输出信息行）；按任务要求与验收口径执行。
11. **YAML 解析**：Go 版 hook 用零依赖迷你解析器（Trellis `trellis_config.py` 同款子集），未引入 `gopkg.in/yaml.v3`。
12. **`sunshine task` 暂定映射**：模板里 `task.py` → `sunshine task`、`get_context.py` → `sunshine task context`（暂定）、`add_session.py` → `sunshine task add-session`（暂定），待阶段 3 任务系统定稿时确认；B 组 references 里约 30 处 `python3 ./.sunshine/scripts/task.py …` 调用原文待阶段 3 按映射表改写（其中 `add-context` / `validate` / `list-context` / `add-subtask` / `remove-subtask` 等子命令在映射表之外，需一并定名）。
13. **sunshine-meta 文档未适配 Go 版现状**：仍描述上游 21 个平台、Python hooks、`.trellis/scripts/task.py`，忠实翻译未改写；如需 meta 文档反映 Go 版实际形态（3 平台、`sunshine hook`、`sunshine task`），需另立修订任务。
14. **开发者 worktree 继承未实现**：Python `get_developer` 还会读 git 主工作区的 `.developer`；Go 版只读环境变量与 `.sunshine/.developer`。
15. **阶段 3 待确认**：`sunshine task list --mine` 是否存在（session-start 的 compact state 行引用了它）；task 命令消费 shell 票据时实现 `internal/hook/NOTES.md` 里的四条接受条件；`sunshine task start` 须写 `.sunshine/.runtime/sessions/<context_key>.json`（`{"current_task": "<引用>"}`）。

### 阶段 2 协调人修复（合并验证时发现）

- 清单键混用（差异 2）：`internal/scaffold/scaffold.go` 的 `Init` 改为写项目根相对键；`go test ./...` 仍全过，临时目录重测清单 107 条全为项目根相对路径。
- hook 运行时输出残留 `<trellis-workflow>` 标签（Python 原版 `session-start.py:887` 如此，Go 版按改名规则改为 `<sunshine-workflow>`）；另 `sunshine hook --help` 的 Long 文案去掉 Trellis 字样。`TRELLIS_*` 环境变量与 `trellis-` 子代理名只作为兼容回退保留（读旧值、接受旧名），不产生新输出。

## 阶段 3 实施说明（任务系统 + 工作流切换）

依据：需求清单第 9 模块（任务系统）、第 2 模块（`workflow` 命令）、第 12 模块（config 与 workflow 定义）；源码对照 `packages/core/src/task/`、`packages/cli/src/commands/workflow.ts`、`packages/cli/src/utils/workflow-resolver.ts`、`.trellis/scripts/task.py`（801 行）、`.trellis/scripts/add_session.py`（1545 行）、`.trellis/scripts/get_context.py`、`marketplace/workflows/{native,tdd,channel-driven-subagent-dispatch}`、`.trellis/agents/`。

### 范围

- `sunshine task`：任务生命周期命令（Go 实现，替代 73 个 Python 脚本中任务相关的部分）。
- `sunshine workflow`：工作流模板查看与切换。
- `internal/config/`：完整 `config.yaml` 字段读写。
- sunshine-meta 文档适配（customize-local 与 local-architecture 改写为 Go 版现状）。

### task.json（24 字段，顺序与校验与源码一致）

字段顺序（`packages/core/src/task/schema.ts` 的 `TASK_RECORD_FIELD_ORDER`）：id、name、title、description、status、dev_type、scope、package、priority、creator、assignee、createdAt、completedAt、branch、base_branch、worktree_path、commit、pr_url、subtasks、children、parent、relatedFiles、notes、meta。

- 默认值：status `"planning"`、priority `"P2"`、createdAt 当天日期、subtasks/children/relatedFiles 为 `[]`、meta 为 `{}`，其余可空字段为 `null`。
- 校验：24 个字段全部必填（缺失即报错，不补默认值）；字符串/可空字符串/字符串数组/meta 对象四类按 schema.ts 的集合校验；未知字段不进入结构化输出，但 `writeTaskRecord` 写盘时要与磁盘已有 JSON 合并保留（不得丢用户在 task.json 里手加的字段）。
- 任务目录：`.sunshine/tasks/<MM-DD-slug>/`；归档到 `tasks/archive/YYYY-MM/`；当前任务记录在 `.sunshine/.current-task`（文件内容为任务路径）。

### `sunshine task` 子命令（17 个，对应 task.py 的 argparse 子命令）

create、add-context、validate、list-context、start、current、finish、set-branch、set-base-branch、set-scope、set-meta、rename、archive、list、add-subtask、remove-subtask、list-archive。全部中文帮助；行为逐条对齐 task.py 对应 `cmd_*`（含：`create` 写 task.json + 任务目录骨架；`start` 写 `.current-task`；`finish` 清空当前任务；`archive` 移入 `tasks/archive/YYYY-MM/`；`rename` 改目录名并更新引用；`set-meta` 覆盖指定元数据键；`add-subtask` / `remove-subtask` 维护父子链接）。

- `sunshine task list --mine`：实现（session-start 的 compact state 行引用了它；按当前开发者过滤）。
- shell 票据消费：`sunshine task start/current/finish` 须实现 `internal/hook/NOTES.md` 的四条接受条件；`sunshine task start` 写 `.sunshine/.runtime/sessions/<context_key>.json`（`{"current_task": "<引用>"}`）。

### 其余 task.py 映射（阶段 2 遗留，阶段 3 定稿）

- `get_context.py`（16 行薄壳，调 `common.git_context.main`）→ `sunshine task context`（文本/JSON 两种输出）。
- `add_session.py` → `sunshine task add-session`：往开发者 journal 追加会话记录并更新 index.md；flags：`--title/--commit/--summary/--branch/--stdin/--change/--test/--next-step/--commit-subject/--idempotency-key`；commit OID 先在本地 git 对象库解析、解析失败则不写任何文件；每条记录带指纹标记、可重放修复；分支解析顺序 `--branch` > task.json 的 branch > `git branch --show-current`。
- B 组 references 里约 30 处 `python3 ./.sunshine/scripts/task.py …` 调用按本节映射表改写；`init_developer.py` / `get_developer.py` 的行为并入 `sunshine init` 与开发者身份解析（阶段 1 已实现读 git user.name 与 `.developer` 文件）。

### 工作区与 journal

- workspace 按开发者分目录；journal 按 `max_journal_lines`（默认 2000 行）轮换；项目根 `.gitattributes` 追加 `journal-*.md merge=union`（已存在时不重复追加）。
- 存疑点 6 核对：`.sunshine/workspace/index.md` 称各开发者目录下有 `tasks/*.json` 与 `archive/`，实查 Trellis 仓库 7 个开发者目录均无；核对后如属过时描述，修正阶段 1 生成的 `workspace/index.md`。

### `sunshine workflow` 命令（对应 workflow.ts + workflow-resolver.ts）

- flags：`-t/--template <id>`（native、tdd、channel-driven-subagent-dispatch）、`-m/--marketplace <source>`、`--list`、`-f/--force`、`-n/--create-new`（写 `.sunshine/workflow.md.new`）；无 `--template` 时交互选择。
- 覆盖决策按现有 workflow.md 的哈希状态（与上游 `classifyExistingWorkflow` 四态一致）：未改动→直接覆盖；已改动或清单无记录→交互确认（覆盖 / 写 `.new` / 跳过）；模板引用的 agent 缺失时警告。
- 三个工作流模板来自 `marketplace/workflows/`，翻译成中文并改名（trellis→sunshine）；其中 channel-driven-subagent-dispatch 的派发行为依赖 Channel（阶段 6），阶段 3 只安装模板文件，派发能力待阶段 6。
- 存疑点 7 核对：Trellis workflow.md 的 Phase 3 编号从 3.2 开始（无 3.1）；核对模板原文后如属有意编号则原样保留。

### 完整 config.yaml 字段（`internal/config/`）

已启用字段：`session_commit_message`、`max_journal_lines`（2000）、`packages`、`default_package`、`channel.worker_guard`（`idle_timeout` 5m、`max_live_workers` 6）。注释字段（同样要支持读写）：`session_auto_commit`、hooks（`after_create` / `start` / `finish` / `archive`）、`codex.dispatch_mode`、`context_injection` 上限（32768/65536/131072）、`prompt_injection.skip_keyword`（`"no-trellis"` → Go 版为 `"no-sunshine"`）。阶段 1 生成的最小版 config.yaml 在此补全为完整字段（带中文注释）。

### `.sunshine/agents/`（5 个 agent 定义）

`.trellis/agents/` 的 architect、check、implement、plan、research 五个定义翻译成中文并改名，写入 `.sunshine/agents/`（init 时生成；已初始化项目由 update 补装，见阶段 4）。

### sunshine-meta 文档适配

`sunshine-meta` 的 customize-local 与 local-architecture 参考文档改写为 Go 版现状：去掉 npm 安装、task.py、Python hooks 的说法，改成 Go 二进制、`sunshine hook`、`sunshine task` 子命令；平台数量按实际写（3 个已接 + 滚动轨道 19 个待扩展，不写 21 个）。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- 临时目录实测：`sunshine task create/list/start/current/finish/archive` 全流程闭环；task.json 24 字段顺序与默认值与 schema.ts 一致；归档路径正确；`.current-task` 正确写入/清空；`sunshine workflow --list` 列出三个模板，`--template tdd` 切换成功，已改动 workflow.md 时按规则确认或写 `.new`。
- `sunshine task context` 与 `add-session` 实测：journal 追加、index.md 更新、commit OID 解析失败不写文件。
- 生成文件抽查：中文、无 trellis 残留；references 里的 task.py 调用已按映射表改写。

## 阶段 3 实施记录（2026-11-09，已完成）

分工：A 组 task.json 与 `sunshine task` 命令；B 组 workflow 命令与模板；C 组 config 与 agents 与 meta 文档适配；协调人合并验证。`go build`、`go vet`、`gofmt`、`go test ./...`（148 个测试，含子测试）全过；临时目录实测任务全流程（create/add-context/validate/start/current/add-session/finish/archive/list-archive）与 workflow 切换（--list/--template tdd/已改动确认/写 `.new`）闭环；task.json 24 字段顺序与 schema.ts 一致；生成文件 `grep -ri trellis` 零命中。

### 协调人合并修复（2 项）

- A 组 `LoadConfigExtras` 临时行级扫描改为由 C 组 `internal/config`（yaml.v3）提供，删除重复解析器；`go test ./...` 仍全过。
- sunshine-meta 文档 channel/mem 加“规划中”标注（SKILL.md 正文与 local-architecture/multi-agent-channel.md）：`sunshine channel`（阶段 6）与 `sunshine mem`（阶段 5）当前版本尚未提供，定制时不要调用。

### 用户确认后的修正（2026-11-09）

- **init 改用完整版 workflow.md**（用户确认）：`templates/sunshine/workflow.md`（28 行简版）删除，`scaffold.Init` 直接写 `templates/workflows/native/workflow.md`（714 行完整版，与 `workflow --template native` 同源）；新增测试锁定两者逐字节一致；`workflow --template native` 在 init 后为无操作（只刷新哈希条目）。
- **Classify 五态 → 四态（与上游对齐）**：核对 `workflow.ts` 的 `classifyExistingWorkflow`，上游只有 missing/identical/pristine/modified 四态——清单无记录时按 modified 走交互确认（覆盖 / 写 `.new` / 跳过），不自动写 `.new`。Go 版阶段 3 的 `ClassUnknown` 自动写 `.new` 属偏离（导致 native→tdd→native 往返时静默写 `.new` 而不切换）；2026-11-09 删除 `ClassUnknown`，`Classify` 清单无记录时返回 `ClassModified`，`Apply` 与 `workflow --help` 文案同步修正，`state_test.go` / `apply_test.go` 相应改写；阶段 3 实施说明里的“未知来源→写 `.new`”一并更正。

### 与需求清单 / 源码的差异（以源码为准）

1. **hooks 事件名，PLAN.md 有误**：阶段 3 实施说明写成 `after_create/start/finish/archive`；config.yaml 原文注释是 `after_create/after_start/after_finish/after_archive`。Go 版按源码用后者，`internal/config` 测试已锁定。
2. **存疑点 6 已核实**：Trellis 仓库 7 个开发者目录均无 `tasks/` 子目录，`.trellis/workspace/index.md` 的说法属过时描述；Sunshine 短版模板本就无此说法，A 组补一行写清实际结构（开发者目录只有 `index.md` + `journal-N.md`；任务在 `.sunshine/tasks/`）。
3. **存疑点 7 已核实**：workflow.md 的 Phase 3 从 3.2 开始属有意编号（原文注释：3.1 已并入 2.2 与 3.4，编号保持不变以免破坏外部引用）；翻译版原样保留。
4. **task.py 实际 17 个子命令与 PLAN.md 无出入，无 `show` 子命令**（`show_usage` 只是帮助函数名）；`init-context` 是已移除命令的守卫，不是子命令。
5. **`.current-task` 双写**：Trellis 源码里 `.current-task` 常量存在但运行时从未读写，当前任务只存在 `.runtime/sessions/<context_key>.json`。Go 版按 PLAN 要求两者都写（`start` 写 session 文件 + `.current-task`；`finish` 两边都清）；`.current-task` 是 Go 版为纯 CLI 可见性额外写的指针，hook 不读它。
6. **`context_key` 来源链**：① `SUNSHINE_CONTEXT_ID` / `TRELLIS_CONTEXT_ID` 环境变量；② 平台会话环境变量（经 `hook.ResolveContextKey`）；③ shell 票据。
7. **`DefaultBranchLocal` 不做联网探测**：Trellis `resolve_default_branch` 会 `git remote show origin` 联网；Go 版只做本地解析（`origin/HEAD` 符号引用 → main/master），解析不到用当前检出分支并告警。
8. **`Slugify` / `ValidateTaskDirName` 放宽到中文**：Trellis `_slugify` 只认 ASCII，中文标题会产出空 slug；Sunshine 任务名常用中文，故保留中文等 Unicode 字母/数字。
9. **add-session 另实现 `--content-file` / `--package` / `--no-commit`**（上游 flags 全集；`--content-file` 不存在时静默忽略，与 Python 一致）。
10. **finish 不改任务状态**：只清当前任务指针，与 Trellis `cmd_finish` 一致（合并验证时核对源码）。
11. **archive 的分支校验**：branch 与 base_branch 相同时拒绝归档（与上游一致），可用 `--skip-branch-validation` 跳过（仅当任务从未走 PR）。
12. **workflow 已改动 + 显式 `--template` 时也弹确认**：上游显式传 `--template` 时直接报错（确认只在交互选模板时出现）；按任务书与 PLAN.md“已改动→交互确认”实现（TTY 弹确认，非 TTY 报错）。
13. **无 `-m` 时未知 id 不回落 Trellis 默认市场**：上游默认拉 `raw.githubusercontent.com/mindfold-ai/marketplace/main/index.json`，会拉回英文 Trellis 模板；Go 版直接报错（只认内置三项）。有 `-m` 时未知 id 仍按上游从该市场拉取。
14. **`-m` 不支持本地路径**：核实上游 `parseRegistrySource` 同样不支持本地路径；传本地路径时报中文错并说明改用 `gh:user/repo` 式地址。
15. **native 模板与 init 的 workflow.md 内容不同**：init 生成 28 行简版（阶段 1 产物，未动）；`workflow --template native` 写 marketplace native 的 721 行完整翻译版。切回 native ≠ 恢复 init 原文件。**待定：是否让 init 改用完整版。**
16. **packages / default_package**：Trellis 原文启用值（cli/core/docs-site、default cli）是其仓库自身 monorepo 配置，不可移植；Go 模板改为注释示例，`internal/config` 默认空 packages/空 default_package，读写完整支持。
17. **`auto_trust_trellis_symlinks` → Go 版改名 `auto_trust_sunshine_symlinks`**（其余字段名与原文一致）。
18. **阶段 1 的 `version` / `tasks_dir` / `spec_dir` 保留**为 Sunshine 扩展字段（`internal/config` 与模板一致），未删除；其中 `version` 与 `.sunshine/.version` 文件含义重复，留待定夺。
19. **yaml.v3 Marshal 丢注释**：`Marshal` / `Write` 只用于生成新文件，绝不用于原地改写用户文件。
20. **`registry.spec` / `update.skip` 不在 Trellis config.yaml 原文里**（是 update/registry 流程后续写入的），阶段 3 未建模；阶段 4 接入时补。
21. **`.sunshine/hooks/` 约定位置**：C 组在 change-task-lifecycle.md 里把它写为用户自放 hook 脚本的约定位置示例（不在模板哈希管理内）；属新增文档约定，非上游行为。
22. **模板内平台列表保留 Trellis 原文全部 21 个平台名**（忠实翻译，未按 Sunshine 已接 3 平台裁剪）；如需裁剪属另立修订任务。
23. **`sunshine task context --mode phase --step` 已实现**（A 组四模式 default/record/packages/phase，`--step` 取 workflow.md 某一步）；B 组模板映射确认。
24. **`-m` 真实远程拉取未做联网实测**（无可用的 Sunshine 市场；Trellis 默认市场未用作验证，以免拉回英文模板）。HTTP/git 两条传输路径均有 httptest + 本地 git 仓库单元测试覆盖。

## 阶段 4 实施说明（update / registry / 模板市场 / 项目生命周期）

依据：需求清单第 4 模块（update / registry 同步 / 模板哈希 / 迁移）、第 8 模块（模板市场与 `--registry`）、第 2 模块（uninstall / ablate / restore / platforms）；源码对照 `packages/cli/src/commands/update.ts`（2924 行）、`uninstall.ts`（300 行）、`ablate.ts`（500 行）、`packages/cli/src/utils/template-fetcher.ts`（1396 行）、`template-hash.ts`、`registry-config.ts`、`uninstall-scrubbers.ts`（476 行）、`ablation-store.ts`、`packages/cli/src/types/migration.ts`、`packages/cli/src/migrations/manifests/`（149 份）、`packages/cli/src/cli/index.ts`（platforms 命令）、`packages/cli/src/configurators/index.ts`（`getConfiguredPlatforms`）。

### 范围

- `sunshine update`：模板同步更新（哈希三态 + 交互确认 + 备份 + 迁移 + registry 持续刷新）。
- `sunshine init --registry <url>`：从模板市场或直接下载装模板；`registry.spec` 持久化进 config.yaml。
- 迁移系统：引擎 + 清单格式 + Sunshine 自己的清单（从空开始，见下）。
- 生命周期命令：`sunshine uninstall`、`sunshine ablate`、`sunshine restore`、`sunshine platforms`。
- `internal/config` 补建模：`registry.spec`、`update.skip`（阶段 3 未建模，阶段 4 补）。

### `sunshine update` 命令

flags：`--dry-run`、`-f/--force`、`-s/--skip-all`、`-n/--create-new`（新版写 `.new` 副本）、`--allow-downgrade`、`--migrate`。

- **前置**：要求已存在 `.sunshine/`；读项目版（`.sunshine/.version`）与 CLI 版（内嵌版本）；CLI 比项目旧时拒绝升级（除非 `--allow-downgrade`）。“最新版”检查：Go 无 npm，对应实现为 GitHub Releases 检查（仓库地址待阶段 7 定模块路径后填入）；未配置或离线时只警告、不中断，项目版↔CLI 版比对是硬门。
- **流程顺序**（按 `update()` 主流程）：版本比对 → Codex 升级修复（`needsCodexUpgrade`，含 SessionStart 接线补全）→ 完整备份（先备份再写盘）→ 清单孤儿键清理（`pruneOrphanManifestKeys`，注意须在 Codex 修复之后，否则会误删尚未补装的 Codex 条目）→ registry.spec 持续刷新 → 变更分析 → 交互确认 → 写盘 → 迁移执行 → 更新 `.version`。
- **哈希三态 + 交互确认**：按 `.template-hashes.json` 区分未改动（自动更新）、用户已改动（逐个交互确认 Overwrite / Skip / Create copy，及 overwrite-all / skip-all / create-new-all 批量项，默认 Skip）、新增、已删除（`userDeletedFiles`：清单有记录但文件已删）。`--dry-run` 只打印计划不写盘；非交互（无 TTY）且有冲突时默认 Skip（安全），`--force` 可覆盖；`--skip-all` 全部跳过；`--create-new` 新版一律写 `.new` 副本。
- **备份**：更新前完整备份（`BACKUP_DIRS = ALL_MANAGED_DIRS`，`BACKUP_FILES = [AGENTS.md]`，`shouldExcludeFromBackup` 排除规则；备份目录命名与位置按 `createBackupDirPath` / `createFullBackup` 实现，置于项目内 `.sunshine/.backups/<时间戳>/`，备份本身不进哈希清单）。`PROTECTED_PATHS`（用户数据路径，含 spec/tasks/workspace/journal）永不迁移、不删除。
- **`update.skip`**：`config.yaml` 的 `update.skip` 跳过列表；破坏性版本 + `--migrate` 时可绕过（按 `loadUpdateSkipPaths` 实现）。
- **registry.spec 持续刷新**：`config.yaml` 的 `registry: spec: {source, template}`；update 时 `collectRegistrySpecTemplates` 重新拉取模板到临时目录作为本次 update 的 spec 模板集合；marketplace 型 registry 缺 template 则跳过并警告；直连型走 `fetchRegistrySpecTemplates`；失败只警告不中断；registry spec 同样受 hash 追踪，本地编辑走正常冲突流程。
- **`.version`**：更新成功后写 CLI 版进 `.sunshine/.version`。
- **statusline 保留（定稿，阶段 2 遗留）**：按 Trellis 行为——`.claude/settings.json` 走正常哈希冲突流程，写盘前仅 `statusline` 字段按 `preserveExistingClaudeStatusLine` 保留用户现有值；Go 版阶段 2 的“保留全部顶层键”改为此行为，记录在实施记录里。
- **registry 配置保留**：`preserveExistingRegistryConfig`——update 不得丢用户 config.yaml 里的 `registry.spec` 与其他自加字段与注释；实现时用行级编辑或谨慎合并，并用测试锁定（用户自加命令/skill/agent/hook 不被覆盖，见 PLAN 补入的用户定制保证）。
- **孤儿迁移检测**：源存在且目标不存在的迁移项视为孤儿，加入待迁移清单（按 `update.ts:2271-2297` 的判定）。
- **managed block 合并**：AGENTS.md 的托管块按 `getManagedBlock` / `replaceManagedBlockContent` / `mergeManagedBlockContent` 合并，不直接覆盖用户在块外的编辑；Copilot instructions 文件存在时同样处理（`isSafeUntrackedCopilotInstructionsMerge`）。

### 迁移系统

- **引擎**：`classifyMigrations`（按项目版→CLI 版区间筛选适用清单）、`sortMigrationsForExecution`（执行顺序）、`promptMigrationAction`（破坏性项交互确认）、`collectSafeFileDeletes` / `executeSafeFileDeletes`（safe-file-delete 自动执行并汇总）。
- **清单格式**：`MigrationItem`（type: rename | rename-dir | delete | safe-file-delete，from/to、description、reason、safe-file-delete 的 `allowed_hashes`）与 `ConfigSectionAdded`（file/sentinel/sectionHeading：新版 config 段落按 sentinel 判定后追加，不覆盖用户其他定制）与 `MigrationManifest`（version/description/breaking/recommendMigrate/changelog/migrations/notes），Go 版用 JSON（与上游清单格式一致）。
- **Sunshine 自己的清单（重要）**：Trellis 的 149 份清单（0.1.9 → 0.7.0-beta.3）描述的是 Trellis 自身模板文件的演变，路径与 Sunshine 结构不同，**不得照搬**。Go 版实现引擎 + 格式，清单从 Sunshine 自己的版本线起步（当前版本尚无历史迁移，清单集初始为空）；后续每次模板结构变更由该版本补一份清单。`renameTracesToJournal` 是 Trellis 专项（traces 改名 journal），Sunshine 无此历史，**不移植**。
- **执行规则**：`--migrate` 才执行 rename/delete 类迁移（未改动模板文件可自动迁移，已改动文件逐个交互确认）；safe-file-delete 满足条件（内容哈希在 `allowed_hashes` 内）时无需 `--migrate` 也自动执行；无 `--migrate` 且有破坏性待迁移项时 update 硬停并提示 `sunshine update --migrate`。
- **实施记录（阶段 4 · 迁移系统，2026-11-09）**：`internal/migrate/` 已实现引擎 + 清单格式 + 清单集骨架，17 个测试全过；清单集初始为空（`NewRegistry()`），后续模板结构变更由该版本补 `manifests/<版本>.json`。与上游源码对照的差异（以源码为准）：① 哈希算法——上游 `computeHash` 先把 CRLF 统一成 LF 再算 SHA-256，Sunshine 的 `.template-hashes.json` 由 scaffold 以原始字节算哈希，迁移系统沿用原始字节算法以与已存哈希一致；② 预发布版本比较——上游 `compare-versions` 未细查，Go 版按 semver 规则逐段比（`beta.3 < beta.10`，数字段按数值比）；③ 上游的 `renameTracesToJournal` 是其 traces 改名 journal 的专项，Sunshine 无此历史，不移植；④ `MigrationAction` 的 `view-diff` 选项上游类型里有但交互里未提供，Go 版同样保留类型、暂不提供该选项。

### registry 与模板市场

- **下载实现**：Trellis 用 Node 的 giget（`downloadTemplate`）；Go 版用 go-git（纯 Go，单二进制无外部依赖）做 git  clone / 稀疏检出，index.json 探测与原始文件拉取用 `net/http`。如 go-git 不可用，退回 `git` 命令行并报错说明（实现时二选一并记录）。
- **`--registry` 三种来源**（`normalizeRegistrySource` / `parseRegistrySource`）：`gh:owner/repo`（可带分支/子目录）、完整 git URL、HTTP(S) URL；另支持 SSH 形式 `git@`（需求清单未核实项，实现后实测验证并记录结果）。
- **两种模式**：有 index.json → Marketplace 模式（按 id 查表，`INSTALL_PATHS` Go 版映射：spec → `.sunshine/spec`、skill → `.agents/skills`、command → `.claude/commands`、full → 项目根）；无 index.json → 直接下载模式（整个 `marketplace/specs/` 当一个模板）。官方模板仓库结构是 `marketplace/index.json` + `marketplace/specs/`（已核实）；支持任意私有 Git 仓库。
- **冲突策略**：skip / overwrite / append（`TemplateStrategy`）；`--template <id>` 指定 marketplace 条目；`--overwrite` / `--append` 仅配合 template 生效。
- **init 接入**：`sunshine init --registry <url>`（`--template`、`--overwrite`/`--append` 同上）；装完后把 `registry.spec.source/template` 持久化进 config.yaml（行级 YAML 写，`registry-config.ts` 的 `loadSpecRegistryConfig` / `writeSpecRegistryConfig` 对应实现，`stripYamlScalar` 与 `specRegistryLines` 一并移植）。
- **核对点**：需求清单出处称 `packages/cli/src/utils/registry.ts`，实查该文件不存在（逻辑在 template-fetcher.ts + registry-config.ts + init.ts）；实现时以实查为准并在实施记录里说明。`marketplace/README.md` 称含 `commands/` 与 `agents/` 目录、实查无此二目录——Go 版 marketplace 说明文档按实际结构写，不照抄 README。`mem-recall` skill 目录存在但未列入 index.json——Go 版 marketplace 索引只收已确认的 9 个条目（3 spec + 3 skill + 3 workflow，`bootstarp` 错拼不沿用，Sunshine 用 `sunshine-spec-bootstrap`），mem-recall 待阶段 5 定。
- **`sunshine workflow --template <id>` 是工作流模板，别与 Spec 模板混淆**（阶段 3 已实现，阶段 4 不动）。

### 生命周期命令

- **`sunshine uninstall`**（对应 `uninstall.ts` + `uninstall-scrubbers.ts`）：要求 manifest（`.template-hashes.json`）齐全——无 manifest 时拒绝（无法确定删哪些平台文件）；只删 manifest 列出的托管内容，manifest 外的文件永不删；先 `pruneOrphanManifestKeys` 自愈被污染的清单（删用户自加文件的孤儿键）；未提交数据警告（`collectUncommittedTrellisData`：spec/tasks/workspace 的未提交内容）；非交互 `--yes` 且有未提交数据时拒绝（`dirtyUninstallBypassEnabled` 仅作显式绕过）；`$HOME` 守卫（与 init 一致）；`--dry-run` 预览；删后整个 `.sunshine/` 目录一并移除。
- **`sunshine ablate`**（对应 `ablate.ts` + `ablation-store.ts`）：项目外版本化恢复事务——事务目录在项目外（`SUNSHINE_ABLATION_STATE_ROOT` 环境变量，Trellis 对应 `TRELLIS_ABLATION_STATE_ROOT`；未设时用默认位置，必须在项目外，否则拒绝）；事务含 `state.json` + `backup/`；流程 stage → apply → verify → applied，失败回滚（`rollbackFailedAblation`）；已 ablate 或中断的项目先提示 `sunshine restore`；非交互时拒绝交互确认（`refuseNonInteractivePrompt`）；只动托管领地（`isManagedDirectoryTerritory` / `collectPrunableDirectories` / `buildAblationEntries`）。
- **`sunshine restore`**（对应 `ablate.ts` 的 `restore()`）：精确恢复——校验事务版本与创建时间；按 `state.json` 恢复全部条目；恢复后删除事务目录；同样要求项目外事务与非交互拒绝。
- **`sunshine platforms`**（对应 `cli/index.ts:357` 的内联命令 + `getConfiguredPlatforms`）：按哈希清单判断已配置平台（清单里有该平台配置目录下条目的即为已配置）；输出 id / displayName / configDir；`--json` 输出 `{"platforms": [...]}`；未配置任何平台时打印提示。平台注册表复用 `internal/platforms`（3 个已接 + 滚动轨道 19 个待扩展）。

### 不属于阶段 4

- `sunshine upgrade`（CLI 自更新，`upgrade.ts`）归阶段 7。
- 滚动轨道 19 个平台不在本阶段（沿用阶段 2 适配器接口，分批扩展）。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- 临时目录实测：`init` → 改一个模板文件 → `update --dry-run` 打印计划且不写盘 → `update` 交互确认（覆盖/跳过/写 `.new` 各走一遍）→ 备份目录生成且含被改文件旧内容 → `.version` 更新；`update --migrate` 在有破坏性迁移项时才执行 rename/delete；safe-file-delete 无 `--migrate` 也自动执行。
- `init --registry <本地 git 仓库>`（marketplace 模式与直接下载模式各一）装模板成功，`registry.spec` 写入 config.yaml；改一次 spec 模板后 `update` 能重新拉取刷新。
- `uninstall`（有未提交数据时 `--yes` 拒绝、无数据时删净托管内容）、`ablate` → `restore` 往返（恢复后逐字节一致、事务目录删除）、`platforms`（`--json` 与人类可读输出一致）实测通过。
- 生成文件抽查：中文、无 trellis 残留；update 不丢用户自加的 config 字段、注释、命令、skill、agent、hook（用测试锁定）。

## 阶段 4 实施记录（D 组：生命周期命令，2026-11-09）

范围：`sunshine uninstall` / `ablate` / `restore` / `platforms`。
新增 `internal/lifecycle/`（manifest、主目录守卫、清单剪枝、移除计划、
清理器、切除事务存储、三个命令流程、平台查询）与 `internal/cli/` 下
的 `uninstall.go`、`ablate.go`、`restore.go`、`platforms.go`；
命令文件只提供 `RegisterUninstallCommand` / `RegisterAblateCommand` /
`RegisterRestoreCommand` / `RegisterPlatformsCommand`，由总协调人统一接线，
未改 `internal/cli/root.go`。未动 `internal/update/`、`internal/registry/`、
`internal/migrate/`、`internal/scaffold/`、`internal/config/`。

### 与需求清单 / 源码的差异（以源码为准）

1. **清单键名**：Go 版清单是 `{"__version": 2, "files": {...}}`（scaffold /
   workflow 已定），上游是 `"hashes"` 键；lifecycle 按 Go 版 `"files"` 读写。
   上游旧格式清单读到即判版本不受支持，uninstall / ablate 拒绝执行。
2. **平台只有三家**：结构化清理器只覆盖三平台实际写出的文件
   （`.claude/settings.json`、`.codex/hooks.json` 嵌套形、`.cursor/hooks.json`
   扁平形、`.codex/config.toml`）。上游的 opencode package.json、pi
   settings.json、copilot instructions 托管块、根 AGENTS.md 托管块在 Go 版
   没有对应文件，清单里出现即按孤儿键剪掉；滚动轨道接新平台时补清理器。
3. **Hook 命令形态变了**：上游清理器按“命令末 token 是被删脚本路径”判定
   托管条目；Go 版 hook 命令是 `SUNSHINE_HOOK_PLATFORM=<平台> sunshine hook <事件>`，
   清理器按“命令含 `sunshine hook`”判定，statusLine 按含
   `sunshine hook statusline` 判定。规则等价（只认自家托管命令）。
4. **根 AGENTS.md 不托管**：Go 版 init 只写 `.sunshine/AGENTS.md`，不写根
   AGENTS.md；清单里的根 AGENTS.md 一律视为孤儿键剪掉（上游是“有托管块
   标记才保留”，Go 版无根托管块概念，故直接剪）。
5. **`.cursor/hooks.json` 卸载后留壳**：该文件除 hooks 外还有 `version` 键，
   清理后剩 `{"version": 1}`，按上游 fullyEmpty 规则（只看顶层键是否全空）
   是修改不是删除；`.codex/hooks.json` 只有 hooks 键，清理后删除。
   `.claude/settings.json` 同理留 env 等键的壳。切除后亦然。
6. **`.codex/skills/` 空目录残留**：Install 建的空目录无清单条目，
   uninstall / ablate 后可能剩一个空 `.codex/skills/`（及 `.codex/` 壳）；
   上游同样行为（该目录不在其托管删除路径上），不是遗漏。
7. **切除事务标识**：能力标识为 `sunshine.full`（上游同名改写），
   默认事务根 `~/.sunshine/ablations/v1`，环境变量
   `SUNSHINE_ABLATION_STATE_ROOT`；主目录绕过 `SUNSHINE_ALLOW_HOMEDIR`，
   脏卸载绕过 `SUNSHINE_ALLOW_DIRTY_UNINSTALL`。
8. **平台 displayName**：取 Go 注册表的 Name；上游 Codex 的 displayName
   带一段 `.agents/skills/` 说明性后缀，Go 版注册表已定为短名 `Codex`，
   此处沿用注册表。
9. **恢复文件写盘**：上游恢复文件分支在期望切除后状态为 absent 时用硬链接、
   否则改名覆盖；Go 版统一用同目录临时文件 + 改名（原子替换），
   内容与权限等价。
10. **剪枝的“已知键”来源**：`pruneOrphanManifestKeys` 的已知集合取自
    `platforms.Install` 本身（在临时目录跑一次已配置平台，收集写出表），
    与 init 共用同一份“写了什么”，不会各说各话。A 组的 update 也需要
    同一份逻辑——本包自备实现，未动 `internal/update/`；
    总协调人合并时再决定是否抽公共函数。
11. **指纹权限位**：只记录 0o777 权限位（`FileMode.Perm()`）；上游记
    0o7777（含 setuid/setgid/sticky）。托管文件不含此类位，行为等价。
12. **platformConfigDirs 重复**：`internal/lifecycle` 自备一份平台 ID →
    配置目录映射，与 `internal/cli/init.go` 的同名映射重复；待注册表补上
    ConfigDir 字段后由总协调人抽成一处。新平台接入时两处都要改。
13. **混合文件恢复的冲突语义**：切除期间被改动的混合文件（如 settings.json），
    恢复前须先回到“切除后的清理形态”，只删文件不够——期望的切除后状态
    是清理后的文件而非不存在（与上游 `collectRestoreConflicts` 一致，
    已用测试锁定）。

### 验证（2026-11-09）

- `gofmt`、`go build ./...`、`go vet ./internal/lifecycle/ ./internal/cli/`、
  `go test ./...` 全过（期间 B 组 registry 曾短暂编译失败，
  当时 lifecycle 测试与 scaffold 解耦后仍全绿；现全树已绿）。
- lifecycle 22 个测试 + cli 5 个测试全过：剪枝（孤儿键保护用户文件）、
  清理器（hooks/settings/config.toml）、uninstall（dry-run 不动盘、
  无清单拒绝、脏数据 `--yes` 拒绝与绕过、交互确认）、ablate → restore
  往返、恢复冲突拒绝、事务根目录在项目内拒绝、platforms。
- 临时目录实测（`cmd/harness-tmp` 临时验收入口，已删除）：
  init 三平台 → `platforms` / `platforms --json` 一致 →
  `uninstall --dry-run` / `ablate --dry-run` 不动盘 →
  `ablate -y`（只剩 2 个清理后的壳文件，事务在项目外，含 state.json + backup/，
  211 条目）→ 二次 ablate 提示先 restore（exit 1）→
  `restore --dry-run` 无冲突不动盘 → `restore -y` 后逐字节与权限
  与切除前完全一致（sha256 与 mode 双 diff 为空）、事务目录删除 →
  `uninstall -y`（删除 161 文件、修改 2 文件、移除 3 目录，
  用户自有 skill 与 settings.json 的 env 保留）→ 脏数据 `--yes` 拒绝、
  绕过后成功 → 污染清单（`.codex/sessions/`、根 AGENTS.md）下用户文件
  均存活 → 无清单拒绝（exit 1）、无事务 restore 提示（exit 0）、
  无平台时 platforms 输出 `{"platforms": []}`。
- 生成文件与 CLI 输出抽查：中文、无 trellis 残留
  （`grep -ri trellis` 在新文件与验收输出里均为 0 命中）。

### 遗留（交总协调人）

- 四个命令尚未接入根命令：请在接线处调用四个 Register 函数
  （不要用 init() 自注册，以免与现有命令的注册方式冲突由你定）。
- `pruneOrphanManifestKeys` 与 A 组 update 的剪枝逻辑重复，合并时决定
  是否抽公共函数（见差异 10）。
- `platformConfigDirs` 与 init.go 的映射重复（见差异 12）。
- `sunshine update` 尚未接线时，ablate 的“清单缺失请先运行 sunshine update”
  报错文案指向一个暂未就绪的命令，待 A 组命令接线后自然成立。

## 阶段 4 实施记录（B 组：registry 与模板市场，2026-11-09）

范围：`internal/registry/`（新建）、`internal/config` 的 registry.spec 建模、
`internal/scaffold` 的 init --registry 接入、`internal/cli` 的 flags 与接线。
未动 `internal/update/`、`internal/migrate/`、`internal/cli/update.go`、
`internal/cli/root.go` 与 `~/workspace/research/trellis-source/`。

### 交付

- `internal/registry/`：`source.go`（Source/Error/Backend/SourceKind）、
  `parse.go`（NormalizeRegistrySource/ParseRegistrySource，含本地路径补充来源）、
  `index.go`（SpecTemplate/TemplateIndex/ParseTemplateIndex/InstallPaths/
  ProbeRegistryIndex）、`git.go`（go-git 克隆、稀疏检出、SSH 认证与
  accept-new 主机密钥回调、错误分类）、`download.go`（TemplateStrategy、
  DownloadTemplateByID/DownloadRegistryDirect/FetchRegistrySpecTemplates、
  CollectDirectoryFiles）、`install.go`（ResolveTarget/Install）、
  `refresh.go`（RefreshSpecTemplates/Refresher/CollectRegistrySpecTemplates）、
  `marketplace.md`（按实查结构写的市场格式说明）。
- `internal/config/registry.go`：SpecRegistryConfig/RegistryConfig、
  LoadSpecRegistryConfig/WriteSpecRegistryConfig（行级 YAML，
  stripYamlScalar 与 specRegistryLines 一并移植）；Config 补
  `Registry *RegistryConfig`（`yaml:"registry,omitempty"`）。
- `internal/scaffold/registry.go`：InstallRegistry、WriteDefaultSpec
  （registry 安装失败时回退内置空白 spec）；`scaffold.go` 的 Init
  拆出 InitWithOptions（Force/SkipSpec，SkipSpec 对应上游 skipSpecTemplates）。
- `internal/cli/registry.go`：registerRegistryFlags（--registry/--template/
  --overwrite/--append）、resolveRegistryStrategy、installRegistryTemplates；
  `internal/cli/init.go` 的 RunE 接入（解析目标在 scaffold.Init 之前，
  以便按 WritesSpec 决定 SkipSpec；安装失败回退空白 spec 不中断 init；
  增量模式支持只装 registry 模板）。接线在 init.go 的 init() 里调
  registerRegistryFlags(initCmd)，root.go 未动。

### 关键决策与核对

- **下载实现**：go-git v5.19.3 可用（`go get` 拉取成功，go.mod/go.sum 已更新），
  采用 go-git，不退回 git 命令行。浅克隆（depth 1、单分支）+ 稀疏检出
  （`NoCheckout` + `CheckoutOptions.SparseCheckoutDirectories`，按 HEAD 哈希检出；
  go-git 的 Checkout 会把空 Branch 默认成 master，直接用分支名会在 main
  默认分支的仓库上报错，实测发现后改用哈希）。
- **三种来源 + SSH**：gh:/完整 git URL/HTTP(S) URL 按上游解析；SSH（`git@`、
  `ssh://`）解析已用单元测试覆盖；SSH 克隆实测：沙箱出口代理重置 22 端口
  （`ssh -T git@github.com` 报 `Connection reset by 198.19.0.1 port 3128`），
  无法对 github.com 完成端到端 SSH 克隆——解析与凭据查找、主机密钥回调已验证，
  真实 SSH 克隆待有凭据与 22 端口出口的环境复验。
- **SSH 主机密钥**：go-git 默认回调在无 known_hosts 文件时直接报错
  （`cannot create known hosts callback`，用最小复现程序确认）；Go 版改用
  accept-new 回调：已知主机严格校验（密钥变更拒绝），未知主机接受并追加记入
  `~/.ssh/known_hosts`（`$SSH_KNOWN_HOSTS` 优先），与 git 的
  `StrictHostKeyChecking=accept-new` 一致；回调逻辑有单元测试
  （接受并记录 / 同密钥通过 / 换密钥拒绝 / 环境变量路径）。
- **错误分类修正**：`handshake failed` 常由连接重置引起，之前误归认证；
  现按“连接重置/拒绝 → network”优先，`authentication failed` 等明确文案才归 auth。
- **核对点（清单与源码不一致，以源码为准）**：
  - 需求清单称 `packages/cli/src/utils/registry.ts`，实查不存在，逻辑在
    template-fetcher.ts + registry-config.ts + init.ts（update.ts 的
    collectRegistrySpecTemplates 亦然）。
  - 模板 path 按 registry 根（index.json 所在目录）解析；上游 git 路径按
    仓库根解析，对带 subdir 的 registry 会取错路径（如
    `gh:mindfold-ai/Trellis/marketplace` 的 `specs/` 在仓库根不存在，
    实查 trellis-source 仓库根下无 `specs/` 目录），Go 版以 registry 根为准。
  - 官方市场仓库实查（2026-11-09 GitHub API）：根含 `index.json`、`specs/`、
    `skills/`、`workflows/`、README.md，无 `commands/`、`agents/` 目录；
    其 README 称含此二目录的说法不实，Go 版 marketplace.md 按实查写。
  - 直接下载模式：registry 根下有 `marketplace/specs/` 时以它为模板，
    否则以 registry 根本身为模板（私有 spec 仓库）。
  - 上游 downloadTemplateById 只支持 spec（MVP）；Go 版按 INSTALL_PATHS
    同时支持 spec/skill/command/full，workflow 仍拒绝并提示用
    `sunshine workflow --template`。skill 装进 `.agents/skills/<id>/`
    （每个 skill 自成目录，避免 SKILL.md 互撞）；full 装进项目根时永不删除
    根目录（skip 按 append 处理，overwrite 只覆盖同名文件——上游未做此保护）。
  - 克隆的 `.git` 目录绝不跟着模板装进项目（初版实测曾带入，已修并加测试锁定）。
- **marketplace 索引 9 条目**：只收已确认的 9 条（3 spec + 3 skill + 3 workflow），
  `bootstarp` 错拼不沿用（Sunshine 用 `sunshine-spec-bootstrap`），
  mem-recall 未列入 index.json，不收。测试夹具索引即按此 9 条写。
- **本地路径是 Go 版补充来源**（上游不支持）：已存在目录或 `file://` URL，
  向上找 enclosing 的 `.git` 定仓库根、剩余部分为 subdir；不在任何仓库里
  的路径在克隆阶段报“不是有效的 git 仓库”。
- **重复实现提示（请总协调人定）**：阶段 3 的 `internal/workflow/registry.go`
  已有一份 ParseRegistrySource（不含本地路径）；A 组的
  `internal/update/configyaml.go` 另有一份 registry.spec 行级读写。
  本组按分工在 `internal/registry` 与 `internal/config` 各实现了一份，
  未动既有文件；后续可由总协调人统一归并。

### 给 A 组 update 的刷新接口（总协调人接线）

```go
// internal/registry/refresh.go
type Refresher struct{}
func (Refresher) RefreshSpecTemplates(source, template string) (map[string]string, error)
func RefreshSpecTemplates(source, template string) (map[string]string, error)
func FetchRegistrySpecTemplates(ctx context.Context, src *Source) (map[string]string, error)
func CollectRegistrySpecTemplates(cwd string) map[string]string
```

接线（一行）：`update.RegistryRefresh = registry.Refresher{}`。
语义与 A 组 `internal/update/registry.go` 的 RegistryRefresher 契约一致：
marketplace 型缺 template、模板 id 不在 index 里、拉取失败时返回 error
（update 只警告、不中断）；成功返回 `.sunshine/spec/**` 文件集合
（键为相对项目根的 POSIX 路径）。注意 A 组 registry.go 注释称实现在
`internal/workflow/marketplace.go`，实际落在 `internal/registry`，以此处为准。
CollectRegistrySpecTemplates 是上游原样移植（内部打印警告、只返回 map），
供不需要 error 值的调用方使用。

### 验证（2026-11-09）

- `go build` / `go vet` / `gofmt` / `go test`：本组范围
  （internal/registry、config、scaffold、cli、hook、platforms、task、
  workflow、templates）全过；`go test ./...` 全量暂不可过——A 组进行中的
  `internal/update` 编译失败（`ConflictAction`、`MigrationItem`、
  `DefaultMigrator`、`SafeFileDelete` 未定义），与本组无关，待 A 组补齐后复验。
- 单元测试：registry（含解析、index、本地 git 夹具端到端、策略、刷新、
  SSH 回调、错误分类）、config（registry.spec 行级读写往返）、
  scaffold（SkipSpec/WriteDefaultSpec）、cli（策略校验）。
- 二进制实测（旧二进制，A 组代码导致重建失败前构建的版本；分类修复后的
  新逻辑以单元测试为准）：临时目录 `init -y --registry <本地市场仓库>
  --template demo-spec`（marketplace 模式）装入 2 文件，registry.spec
  写入 config.yaml 且进哈希清单；`init -y --registry <本地直连仓库>`
  （直接下载模式）装入 2 文件；改模板后 `--force --overwrite` 重拉取到新内容；
  skill 模板装进 `.agents/skills/demo-skill/`；flag 校验三条报错均符合预期；
  生成文件无 trellis 残留。
- 远程实测（一次性测试，跑完即删，未留联网测试）：`gh:mindfold-ai/marketplace`
  HTTP 探测到 9 个模板，`electron-fullstack` 经 go-git HTTPS 克隆装入 50 个文件，
  用时 2.19s。
- SSH：解析与回调已验证；端到端克隆因沙箱 22 端口被出口代理重置而无法完成，
  待有凭据与 22 端口出口的环境复验；分类修复后该类失败应报网络错误而非认证错误
  （单元测试已锁定，旧二进制复验待整包可构建后进行）。

### 遗留问题

- `sunshine update` 的 registry.spec 持续刷新端到端复验，待 A 组 update
  命令可构建后由总协调人复验（本组接口已就绪，见上）。
- SSH 端到端克隆待真实凭据 + 22 端口出口环境复验。
- 阶段 4 全量 `go build ./...` 待 A 组补齐 internal/update 后复验。
- skill 模板安装同样会持久化 registry.spec（含 skill 的 template id）；
  update 刷新时非 spec 模板会警告跳过，与上游行为一致。

## 阶段 4 实施记录（A 组：`sunshine update`，2026-11-09）

范围：新建 `internal/update/`（哈希三态、备份、交互确认、managed block 合并、
statusline 保留、registry 持续刷新接线、迁移执行接线）、新建
`internal/cli/update.go`（`RegisterUpdateCommand`，由总协调人接线，未改
`internal/cli/root.go`）、`internal/config` 补 `update.skip` 建模、阶段 2
`internal/platforms` 的 settings 合并行为按定稿改掉。未动
`~/workspace/research/trellis-source/`、`go.mod`/`go.sum`、`internal/scaffold/`。

### 交付

- `internal/update/`：`hash.go`（ComputeHash/LoadHashes/SaveHashes/
  UpdateHashes/RemoveHash/RenameHash/ShouldExcludeFromHash）、`version.go`
  （CompareVersions/GetInstalledVersion/WriteVersionFile）、`paths.go`
  （托管块标记、ProtectedPaths、BackupDirs/BackupFiles/排除规则）、
  `managed.go`（托管块 get/replace/merge、Copilot 合并判定）、`backup.go`
  （CreateFullBackup，`.sunshine/.backups/<UTC 时间戳>/`）、`analyze.go`
  （AnalyzeChanges 三态判定、PrintChangeSummary）、`statusline.go`
  （PreserveExistingClaudeStatusLine）、`codex.go`（NeedsCodexUpgrade）、
  `prune.go`（ConfiguredPlatforms/PruneOrphanManifestKeys）、`configyaml.go`
  （PreserveExistingRegistryConfig、ExtractConfigSections/
  AppendMissingConfigSections）、`registry.go`（RegistryRefresher 接口、
  RegistryRefresh 变量、CollectRegistrySpecTemplates）、`migrate.go`
  （MigrationRegistry / MigrationPrompter 变量，薄胶水层）、`templates.go`
  （CollectTemplateFiles）、`prompt.go`（ConflictAction 六值、Prompter 接口、
  resolveConflict）、`update.go`（Run 主流程，Options/Summary）。
- `internal/cli/update.go`：flags `--dry-run`、`-f/--force`、`-s/--skip-all`、
  `-n/--create-new`、`--allow-downgrade`、`--migrate`；surveyPrompter
  （Confirm 默认继续；Select 六项中文，默认“跳过”；非交互默认 Skip）。
- `internal/config`：`Config.Update`（`Update{Skip []string}`）、
  `SkipPaths()`、`LoadUpdateSkipPaths`（行级解析，不走 YAML）。
- `internal/platforms`：`Platform` 补 `ConfigDir` 字段（claude-code→
  `.claude`、cursor→`.cursor`、codex→`.codex`；D 组记录的
  platformConfigDirs 重复问题后续可改用本字段）；新增导出
  `RenderTemplates`（供 update 收集期望模板集合）；`installSettings` 改为
  普通模板文件语义（不存在写、相同跳过、不同时 Force 为假跳过、为真覆盖），
  删除 `mergeSettingsJSON` / `mergeEnvObject` / `mergeHooksObject` /
  `canonicalJSON`；`internal/platforms/NOTES.md` 第 3 节记了本次行为变更。

### statusline 保留定稿（阶段 2 行为变更）

`.claude/settings.json` 走正常哈希冲突流程，写盘前仅 `statusLine` 按
`preserveExistingClaudeStatusLine` 保留用户现有值（仅当新模板无
statusLine、磁盘有且 JSON 合法时，把磁盘 statusLine 追加到新模板对象末尾，
2 空格缩进 + 末尾换行，行级拼接保序）。阶段 2 的“已有顶层键全保留、
env/hooks 补写”合并行为已删除，4 个旧 settings 测试已按新行为重写，
`TestSettingsMergeHooksDedup` 已删除（合并行为不存在了）。

### 与上游源码的差异（以源码为准）

1. **清单键名**：Sunshine 用 `{"__version": 2, "files": {...}}`（阶段 1
   scaffold 已定），上游 v2 用 `"hashes"` 键；读取兼容两者，写入用
   `"files"`。迁移系统（C 组）与 lifecycle（D 组）同此约定。
2. **哈希算法**：上游 `computeHash` 先把 CRLF 归一化为 LF 再算 SHA-256；
   Sunshine 用原始字节 SHA-256（与 scaffold 一致），否则每个文件都会被
   误判为已改动。C 组的迁移系统同此算法。
3. **dry-run 不写盘**：上游即使 dry-run 也会持久化清单孤儿键清理；
   Sunshine 的 dry-run 完全不写盘（`persist=false`）。
4. **流程顺序**：以源码 `update()` 的实际顺序为准，PLAN.md 实施说明的
   简写列表只是逻辑分组（例如上游是先清理孤儿键、收集模板、分析、再备份
   写盘；备份在统一确认之后、任何写盘之前）。
5. **config.yaml 写盘是合并写**：`AppendMissingConfigSections`——覆盖绝不丢
   用户已有的行（字段、注释、自定义段落），只追加模板缺失的段落；跳过的
   文件写盘后仍补齐缺失段落（上游行为）；段落追加不更新哈希（下次 update
   在 unchanged 桶里自动修复清单，避免静默收编用户的跳过）。
6. **`PreserveExistingRegistryConfig` 与 B 组 `WriteSpecRegistryConfig` 的
   字节一致性要求**：两者追加的 registry 段须字节一致（否则模板+registry
   与磁盘内容对不上，会误报冲突）；A 组 preserve 用 Sunshine 措辞
   （`.sunshine/spec`、`sunshine update`）。A 组原先在 configyaml.go 里
   自写的 `LoadSpecRegistryConfig` 副本已删除，改用 B 组
   `config.LoadSpecRegistryConfig`（B 组记录的“重复实现”已归并一处）。
7. **Codex 升级修复 = 把 codex 纳入本次模板收集**，`.codex/` 文件经正常
   新文件流程写出；`.codex/hooks.json` 模板本身已含 SessionStart 接线。
8. **`renameTracesToJournal` 不移植**（Trellis 专项，Sunshine 无 traces 历史）。
9. **“最新版”检查**：`LatestVersionFunc` 变量，nil 表示未配置；GitHub
   Releases 检查的仓库地址待阶段 7 定模块路径后填入；未配置或离线只警告
   不中断。
10. **根级 AGENTS.md**：Sunshine init 不生成它，update 从不新建；仅磁盘已
    存在且带 Sunshine 托管块时才纳入管理。
11. **Copilot instructions 合并函数已实现并测试，但现阶段是休眠路径**
    （Sunshine 暂无 Copilot 平台，不会进入模板集合）。
12. **剪枝逻辑重复**：`internal/update/prune.go` 的
    `PruneOrphanManifestKeys` 与 D 组 `internal/lifecycle` 的剪枝实现重复
    （D 组记录差异 10 已说明）；总协调人合并时再决定是否抽公共函数。
    另注意上游剪枝在 Codex 修复之后执行，A 组 Run 保持此顺序。

### B/C 组接线（已落地，非占位）

- **B 组 registry 刷新**：`update.RegistryRefresh` 默认就是
  `registry.Refresher{}`（`internal/registry/refresh.go` 的实现与 A 组
  `RegistryRefresher` 接口契约一致）；无 registry.spec 返回空，刷新失败
  只警告不中断。B 组记录的接线行已实际执行。
- **C 组迁移引擎**：`Run` 直接调 `internal/migrate` 的真实 API——
  `reg.Metadata` / `MigrationsForVersion` / `FindOrphanMigrations` /
  `ClassifyMigrations` / `ShouldHardStop` / `HardStopMessage` /
  `MigrateHint` / `PrintMigrationSummary` / `ExecuteMigrations` /
  `CollectSafeFileDeletes` / `ExecuteSafeFileDeletes` /
  `PrintSafeFileDeleteSummary` / `ConfigSectionsAddedBetween` /
  `ApplyConfigSectionsAdded`。`MigrationRegistry` 变量默认 nil（空清单集），
  **清单集如何随二进制发布、由谁加载进 `MigrationRegistry`，待总协调人
  定**（`internal/migrate/manifests/` 目前只有 README.md）。
- **`version` 字段与 `.sunshine/.version` 文件含义重复**的问题（阶段 3
  遗留）未在本次处理，仍待定。

### 验证（2026-11-09）

- `gofmt`（无输出）、`go build ./...`、`go vet ./...`、`go test ./...`
  （11 个包，`-count=1` 无缓存）全过；update 包 30+ 测试全过
  （三态判定、备份排除与旧内容、冲突三动作与批量 skip-all、非交互默认
  Skip、config.yaml 定制保留、statusLine 保留、update.skip、用户删除尊重、
  降级拒绝与 `--allow-downgrade`、Codex 升级、迁移硬停与 safe-file-delete、
  无 trellis 残留）。
- 临时目录真实二进制端到端（仓库副本 + 临时 init() 接线 + CLI 版临时改为
  0.2.0，副本与二进制已删，工作区未动）：`init`（项目版 0.1.0）→ 改一个
  模板文件 → `update --dry-run` 只打印计划不写盘（文件未动、无
  `.backups/`、`.version` 仍 0.1.0）→ `update --force` 覆盖恢复、备份
  `.sunshine/.backups/2026-11-09T08-09-09/` 含被改文件旧内容、`.version`
  → 0.2.0 → 再改三个文件后 pty 交互：check.md 上箭头覆盖（改动消失）、
  implement.md 回车跳过（改动保留）、research.md 下箭头创建副本
  （原文保留改动，`.new` 存新版模板），汇总“已覆盖 1 / 已跳过 1 /
  已创建 .new 副本 1”，备份 `2026-11-09T08-09-40/` 含 check.md 旧内容。
  （注：第一次 pty 驱动因未应答 survey 的光标位置查询 `\x1b[6n` 而卡住；
  应答 `\x1b[1;1R` 后正常。）
- 生成文件与 CLI 输出抽查：中文、无 trellis 残留。

### 遗留（交总协调人）

- `RegisterUpdateCommand` 尚未接入根命令（`internal/cli/root.go` 未动，
  请总协调人统一接线）。
- `MigrationRegistry` 的清单加载与随二进制发布方式待定（见上）。
- `LatestVersionFunc` 的 GitHub Releases 检查仓库地址待阶段 7 定。
- `version` 字段与 `.sunshine/.version` 含义重复是否去掉其一（阶段 3 遗留）。
- `ts-sdk-author` 是否改为 Go SDK 编写 skill（阶段 3 遗留）。
- `pruneOrphanManifestKeys` 与 lifecycle 的剪枝重复、以及
  `platformConfigDirs`（init.go）与 `Platform.ConfigDir` 的重复，
  合并时决定是否归并。

## 阶段 4 实施记录（总协调人合并，2026-11-09，已完成）

四组全部完成，总协调人做合并接线与最终全量验证：`gofmt`、`go vet`、`go build`、`go test ./...` 全过（11 个包、248 个测试，含子测试）。

### 合并接线（总协调人所做）

1. **命令接线**：`internal/cli/root.go` 新增 `init()`，调用五个 `RegisterXxxCommand`（update、uninstall、ablate、restore、platforms）；B 组的 init `--registry` flags 已在 `init.go` 的 `init()` 里接线，`root.go` 本体未动其他逻辑。
2. **platformConfigDirs 归并（已做）**：三处重复映射统一到 `Platform.ConfigDir` 字段——删除 `internal/cli/init.go` 与 `internal/lifecycle/lifecycle.go` 的 `platformConfigDirs` 映射，`platformInstalled` / `ConfiguredPlatformIDs` / `ConfiguredPlatforms` 改查注册表。`platformDisplayNames`（init.go 的 survey 展示名）未动，与 `Platform.Name` 取值相同但用途不同，留待后续。
3. **迁移清单随二进制发布（已定）**：`internal/migrate` 新增 `embedded.go`（`//go:embed manifests`）+ `LoadEmbeddedRegistry()`；`LoadManifestDir` 改为委托共用的 `LoadManifestFS(fs.FS, dir)`；`internal/update` 的 `migrationRegistry()` 在 `MigrationRegistry` 为 nil 时回落到内嵌清单集（目前为空）。新增 `TestLoadEmbeddedRegistryEmpty` 锁定。后续每次模板结构变更只需补 `manifests/<版本>.json`，无需改代码。
4. **statusline 保留定稿（已改）**：A 组执行——`internal/platforms` 删除“保留全部顶层键、env/hooks 补写”的合并行为，`settings.json` 走正常哈希冲突流程，写盘前仅 `statusLine` 保留用户值；`Platform` 补 `ConfigDir` 字段。实测验证：用户自加顶层键被覆盖丢弃，`statusLine` 保留。

### 有意保留的重复（暂不归并，原因如下）

- **`PruneOrphanManifestKeys`（update vs lifecycle）**：语义不同——update 版保留迁移清单引用的路径、根 AGENTS.md 按“文件缺失或带托管块标记”保留；lifecycle 版用实际 Install 输出建已知集、根 AGENTS.md 一律剪（Go 版 init 不写根 AGENTS.md）。各自有测试锁定，强行统一会改行为，留待后续。
- **`ParseRegistrySource`（internal/workflow vs internal/registry）**：workflow 版是阶段 3 已测试的 `workflow -m` 路径（返回 `RegistrySource`，不支持本地路径）；registry 版是阶段 4 新实现（返回 `*Source`，支持本地路径与 SSH）。新代码一律用 registry 版；workflow 的 `-m` 路径后续再切。

### 最终端到端验证（总协调人，合并后二进制实测）

- **update**：init → 改 `.sunshine/workflow.md` 与 `AGENTS.md` → `--dry-run` 只列计划不写盘、无备份目录 → `--force` 覆盖恢复、备份含两文件旧内容、`.version` 0.0.1→0.1.0；`--create-new` 原文件保留改动、`.new` 存新版模板。交互 survey 三动作（覆盖/跳过/写 `.new`）由 A 组 pty 实测。
- **定制保留**：自加命令/skill/agent/hook 文件、config.yaml 自加字段与注释，`update --force` 后全部保留（测试 + 实测双锁定）。
- **registry**：本地 git 仓库 marketplace 模式（`--registry <仓库>/marketplace --template demo-spec`，registry 根按 subdir 定）与直接下载模式各装成功，`registry.spec` 写入 config.yaml；改模板提交后 `update --force` 刷新到 v2（哈希键 `.sunshine/spec/guide.md`，与上游 INSTALL_PATHS 一致：spec 模板文件直接装进 `.sunshine/spec/`，不按 id 分子目录）。远程 `gh:mindfold-ai/marketplace` 由 B 组实测（9 个模板，electron-fullstack 50 文件）。SSH 端到端克隆因沙箱 22 端口被出口代理重置未完成，待有凭据环境复验。
- **生命周期**：`platforms` 与 `--json` 输出一致（未装 Codex 时不列出）；`uninstall --dry-run` 只警告不写盘；脏数据 + `--yes` 拒绝（exit 1，`.sunshine/` 完好）；提交后 `uninstall --yes` 删 110 文件、改 2 文件、移除 `.sunshine/`，用户自加文件存活，`settings.json` 剥离 Sunshine hooks 后保留 env 与 statusLine；`ablate -y` → 二次 ablate 提示先 restore → `restore -y` 后 sha256 全量 diff 为空、事务子目录删除。
- **无 trellis 残留**：三个实测项目 grep 零命中；生成文件与 CLI 输出全部中文。

### 遗留问题

- `update --migrate` 的 CLI 级端到端未做：内嵌清单集目前为空，CLI 无法注入测试清单；A 组已用真实 `migrate.Registry` + 两份清单在单元测试里验证（无 `--migrate` 硬停、`--migrate` 后改名与删文件实际发生）。第一份真实清单落地时再走 CLI 级全流程。
- `LatestVersionFunc` 的 GitHub Releases 检查仓库地址待阶段 7 定模块路径后填入。
- `version` 字段与 `.sunshine/.version` 含义重复是否去掉其一（阶段 3 遗留）。
- `ts-sdk-author` 是否改为 Go SDK 编写 skill（阶段 3 遗留）。
- SSH 端到端克隆待真实凭据 + 22 端口出口环境复验。

## 阶段 5 实施说明（mem 跨平台会话检索）

依据：需求清单第 10 模块（记忆与回想）、第 2 模块（`trellis mem` 子命令与 flags）；源码对照 `packages/cli/src/commands/mem.ts`（514 行）、`packages/core/src/mem/`（types.ts、sessions.ts、search.ts、context.ts、dialogue.ts、filter.ts、phase.ts、platforms.ts、projects.ts、adapters/{claude,codex,devin,grok,opencode,pi,zcode}.ts、internal/{jsonl.ts,paths.ts,sqlite-adapter.ts,sqlite-readonly.ts}）、`marketplace/skills/mem-recall/SKILL.md`。

### 范围

- `sunshine mem`：跨平台只读会话检索（关键词匹配，无向量检索，无 L0/L1/L2 分级）；v1 不读 channel 事件（与上游 `types.ts` 的约定一致，channel 有自己的事件日志）。
- 7 个会话适配器（claude / codex / devin / grok / opencode / pi / zcode），统一为 session / turn / event 模型。
- `mem-recall` skill 中文翻译并收入 Sunshine 模板市场索引（阶段 4 遗留定稿：收录为第 10 个条目；Trellis 未收录的原因未确认，但对 Sunshine 而言它是 `sunshine mem` 的用户入口，应当收录）。

### `sunshine mem` 子命令与 flags

子命令：list（默认）、search `<keyword>`、context `<session-id>`、extract `<session-id>`、projects、help。参数透传后由 Go 端自行解析（对应 `runMem` / `parseArgv`）。

flags：`--platform <claude|codex|devin|grok|opencode|pi|zcode|all>`（默认 all）、`--since/--until YYYY-MM-DD`（until 取当天 23:59:59.999Z）、`--global`、`--cwd`、`--limit N`（默认 50）、`--grep KW`（多 token 为 AND）、`--phase brainstorm|implement|all`、`--turns N`（默认 3）、`--around N`（默认 1）、`--max-chars N`（默认 6000）、`--include-children`、`--json`。

- 默认作用域：未传 `--global` 时限定当前项目（`--cwd` 指定目录或当前工作目录，含子目录）；`--global` 搜全部项目。
- 非法 `--platform`、非法日期直接报错退出（core 不见原始 flags，对应 `buildFilter` 的校验）。

### 统一模型（对应 `types.ts`）

- `MemSourceKind`：7 个平台；`MemFilter`：platform / since / until / cwd / limit；`MemSessionInfo`：platform / id / title / cwd / created / updated / filePath / parent_id（仅 OpenCode，子代理链）。
- `DialogueTurn`：role（user / assistant）/ text / kind（`"marker"` 标注压缩边界——留在池子里供 extract/context 展示模型上下文被截断的位置，search 计分时跳过，避免与平台摘要重复计分）。
- `SearchHit`：count（全部命中 token 数）/ userCount / asstCount / totalTurns（已清理对话规模，相关度分母）/ excerpts（段落对齐的摘录）。

### 7 个适配器（会话存储路径与格式，按 `internal/paths.ts` 与各 adapter）

| 平台 | 存储位置 | 格式 |
|---|---|---|
| claude | `~/.claude/projects` | JSONL |
| codex | `~/.codex/sessions` | JSONL |
| devin | `$XDG_DATA_HOME/devin/cli`（否则 `~/.local/share/devin/cli`；Windows 为 `%APPDATA%\devin\cli`）下的 `sessions.db`；`DEVIN_DB_PATH` 可覆盖（绝对路径直接用、`~/` 展开、相对名拼到数据目录、`:memory:` 表示无文件） | SQLite |
| grok | `~/.grok/sessions/<cwd url-encoded>/<session-id>/chat_history.jsonl`；同目录的 `session_search.sqlite` 只是搜索索引，**不读** | JSONL |
| opencode | `$XDG_DATA_HOME/opencode`（否则 `~/.local/share/opencode`）下的 `opencode.db`（XDG 规则全平台一致，无 Windows/macOS 分支） | SQLite |
| pi | `~/.pi/agent/sessions/--<encoded-cwd>--/<timestamp>_<id>.jsonl` | JSONL |
| zcode | `~/.zcode/cli/db/db.sqlite`（伴生 `-wal` 文件须一并识别） | SQLite |

- 存储缺失是正常空结果，不是错误：该平台跳过并给出警告（对应 `printWarnings`），不中断其他平台。
- **只读**：绝不写会话存储；SQLite 以只读方式打开（不建 `-wal` / `-shm` 副作用，不锁用户正在用的会话库；读锁冲突时短暂重试，仍失败则警告并跳过该存储）。
- **SQLite 实现**：Trellis 用零依赖纯 TS 只读解析器（`internal/sqlite-readonly.ts`）；Go 版用 pure Go 的 `modernc.org/sqlite`（无 cgo，单二进制无外部依赖）。实现时核实模块可用性。
- **JSONL 解析**：逐行解析，坏行容忍（跳过并警告，不中断整个文件）；会话事件形态多样（tool_result、errors、telemetry 等），读取时一律容忍未知事件类型（对应 `.trellis/spec/cli/backend/commands-mem.md` 的要求）。
- **对话清理**（对应 `dialogue.ts`）：去系统提示词、工具噪音、hook 注入；压缩摘要按 marker 处理（见统一模型）。

### 检索与上下文

- **search**（对应 `search.ts`）：关键词匹配；`--grep` 多 token 为 AND；命中按相关度密度（count / totalTurns）排序；摘录段落对齐；marker 轮次不计分。
- **context**（对应 `context.ts`）：取会话上下文；`--turns`（默认 3）取目标轮次前后；`--around`（默认 1）取邻近轮次；`--max-chars`（默认 6000）截断；`--include-children` 沿 OpenCode 的 parent_id 链纳入子会话。
- **extract**：抽取会话完整内容（含 marker 边界展示）。
- **projects**（对应 `projects.ts`）：列出有会话的项目。
- **phase**（对应 `phase.ts`）：按工作阶段过滤。判定方法：从会话的 shell 调用字符串里找任务创建/启动命令——Trellis 版找 `task.py create|start`；Go 版找 `sunshine task create|start`，保留同样的防误判规则（须出现在命令开头、空白后或路径分隔符后，`--slug=...` 这类 flag 值里的不算；一句里多个调用按源码顺序返回；纯英文散文句式排除）。
- **输出**：人类可读列表（会话行含平台/id/标题/时间/路径，`shortDate` / `shortPath` 格式化）与 `--json` 机器输出；字段名与上游保持一致（platform、parent_id、filePath）。

### mem-recall skill

`marketplace/skills/mem-recall/SKILL.md` 翻译成中文并改名（trellis→sunshine、`trellis mem`→`sunshine mem`、`npm install -g @mindfoldhq/trellis` 改为 Go 二进制安装说明）；收入 Sunshine 模板市场索引（第 10 个条目，type: skill）；触发语里的中文例句保留。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- 临时 HOME 夹具实测（各平台会话存储各造一份，含坏行、未知事件、压缩 marker、-wal 伴生）：`mem list`（默认 cwd 作用域与 `--global`）、`mem search`（单 token 与多 token AND）、`mem context`（turns/around/max-chars/include-children）、`mem extract`、`mem projects`、`--json` 输出字段与上游一致；缺失存储时警告并跳过，不报错中断。
- `mem --platform <单平台>` 与 `--platform all` 均实测；`--phase brainstorm|implement` 按 `sunshine task create|start` 调用正确分段。
- 只读验证：实测前后会话存储文件哈希不变（含 SQLite -wal）。
- 生成文件抽查：中文、无 trellis 残留。

## 阶段 5 基线修复（2026-11-09，接手时已存在，与阶段 5 无关）

- 现象：`go test ./...` 在 `internal/hook` 有 2 个失败（`TestSessionStart_NoSunshine`、`TestShellSession_NoSunshine`），其余包全过。
- 原因：`/home/hatch/.sunshine/` 存在（阶段 4 验证时建的默认 ablate 事务根 `~/.sunshine/ablations/v1` + 一个过期 shell 票据，非真实项目）；`ResolveProjectRoot` 回退到进程 cwd 后向上查找，找到 `$HOME/.sunshine`，误把 $HOME 当项目根。
- 修复（仅测试，不动生产代码）：两处 `NoSunshine` 测试加 `t.Chdir(empty)`，使项目根查找与祖先目录的残留 `.sunshine/` 无关；修复后 `go test ./...` 全绿。`ablations/v1` 目前为空，无未恢复事务；`$HOME/.sunshine` 目录本身是阶段 4 设计的默认事务根位置，保留不动。

## 阶段 5 实施记录（2026-11-09，已完成）

分工：A 组统一模型与 CLI；B 组 7 个适配器；C 组检索/上下文/phase/对话清理；D 组 mem-recall skill 翻译与市场索引收录；协调人合并验证 + phase 事件通道跟进。`go build`、`go vet`、`gofmt`、`go test ./...`（13 个包，398 个测试运行含子用例）全过；独立端到端实测（见下）通过。

### 交付物

- `internal/mem/`（新建）：types.go（统一模型 + Adapter 接口）、filter.go、sessions.go（依赖注入：CLI 调 `adapters.All()` 后传入，避开 Go 循环引用）、projects.go、dialogue.go、search.go、context.go、phase.go。
- `internal/mem/adapters/`（新建）：7 个适配器（claude/codex/devin/grok/opencode/pi/zcode）+ jsonl.go + sqlite.go（modernc.org/sqlite v1.62.0，DSN `mode=ro` + `query_only(1)` + `busy_timeout(3000)`，锁冲突重试 5 次）+ paths.go + registry.go（`All()` 固定顺序）。
- `internal/cli/mem.go`（新建，815 行）：`sunshine mem` 六个子命令 list（默认）/search/context/extract/projects/help，全部 flags；`root.go` 的 init() 接线。
- `templates/marketplace/skills/mem-recall/SKILL.md`（新建，295 行）：全文中文翻译；`internal/registry/marketplace.md` 改为 10 个条目（3 spec + 4 skill + 3 workflow）；`internal/registry/source_test.go` 夹具同步为 10 条并改断言；sunshine-meta SKILL.md 的 mem 标注改为已实现（channel 仍为规划中）。
- phase 事件通道（跟进补完）：Adapter 接口加 `Collect(ctx, s) ([]DialogueTurn, []MemEvent, []string, error)`，7 个适配器实现（事件从各平台工具调用抽取，turnIndex 为调用时已产出轮次数；ZCode 例外：先入轮次再记事件，按上游约定）；`SliceMemPhase` 按上游语义（brainstorm→窗口内、无窗口时全文+警告；implement→窗口外、无窗口时空+警告；仅 opencode 不支持 phase，降级全文+警告）；CLI extract --phase 改走 Collect + SliceMemPhase；C 组的单值 `DetectPhase` 已删除（上游没有会话级单值阶段标签）。

### 与需求清单 / 源码的差异（以源码为准）

1. **存储缺失给警告**：上游 TS 静默空结果；按任务书改为统一返回一条中文警告后跳过，不中断。
2. **坏行警告汇总**：上游静默跳过；Go 版每文件汇总一条警告（如“跳过 2 行无法解析的内容”）。
3. **无 search-scoped 预加载缓存**：上游有 `prepare*SessionStore`（一次读全库供 search 复用）；Adapter 接口无此钩子，每次 Dialogue 都重新只读打开库。v1 可接受，search 性能成问题再补。
4. **Pi `idFromFile` 按首个下划线切分**：与上游一致；时间戳含下划线时取到 `120000_pi-1`，测试已按上游行为标注。
5. **排序键用普通密度 `count/totalTurns`**（任务书口径），不用上游加权分 `(3u+a)/totalTurns`；加权分仍以 `RelevanceScore` 导出备用。
6. **`ExtractDialogue` 的 grep 用多 token AND**（与 CLI 帮助、context 语义一致）；上游 extract 是整串子串匹配，此处有意偏离。
7. **长度按字符（rune）算**，不是字节；截断标记写成中文 `…[另有 N 字符]`。
8. **`MemFilter` 是 plain int**：Turns/Around/MaxChars 为 0 时取默认 3/1/6000，显式 `--around 0` 无法表达；负数按 0 计。
9. **`$(sunshine task create …)` 不命中**：与上游正则语义一致（`(` 不是分隔符），测试已锁定。
10. **`ExtractDialogue` 的 Phase 切片未统一**：它仍扫轮次文本，现有测试钉住了该行为；CLI extract 已改走事件通道（`extractGroups` 只传 PhaseAll 给它，切片走 `SliceMemPhase`）。后续统一时先改那几个测试的预期。
11. **`PhaseSupported` 未放进 Adapter 接口**：包级函数 `mem.PhaseSupported(kind)`（仅 opencode 为 false）。
12. **`--phase` 只对 extract 生效**（上游行为；list/search 忽略 --phase，CLI 已钉测试）；`--limit 0` 视为默认 50；非法 flags 退出码为 1（上游为 2）；projects 的 `--limit` 未显式传时默认 30（用 `Flags().Changed` 判定，与上游一致）。
13. **Codex 会话无 cwd**：rollout 文件不带工作目录，cwd 为空的会话不进 cwd 作用域的 list/projects，只进 `--global`（上游同款结构所致）。
14. **mem-recall 译文**：上游“Trellis 0.6.17 新增 Devin、0.6.14 增加 Grok、0.6.16 恢复 OpenCode”是 Trellis 发版史，Sunshine 版本线 0.1.0 无法沿用，改为现状表述；安装说明改为 Go 单二进制口径，未写下载 URL 与版本号（GitHub Releases 地址待阶段 7 定）。
15. **`dialogue.go` 与 `adapters/dialogue.go` 约 60 行重复**（标签剥离等）：`mem` 包不能反向引用 `adapters`（循环引用），后续可让适配器改调 `mem` 的导出函数去重。
16. **`marketplace.md` 仍有 3 处 Trellis 字样**：均为上游引用说明（`gh:mindfold-ai/Trellis/marketplace` 来源写法示例、`trellis-spec-bootstarp` 错拼注释），属文档引用而非生成物残留，保留。

### 协调人独立验证（临时 HOME 夹具，/tmp/memhome + /tmp/memproj）

- 7 平台会话各一份（含坏行、未知 telemetry 事件、压缩 marker）：`mem list` cwd 作用域 7 个、`--global` 9 个、`--platform claude` 1 个；`--json` 字段与上游一致（platform/parent_id/filePath，oc-child 带 parent_id）。
- `search 记忆` 命中 2 个（codex 密度 0.500 排前、claude 0.167）；引号多 token `"记忆 模块"` AND 命中 1 个（codex，count=2）；`--grep` 多 token AND 正常；`search` 多位置参数报 cobra 错退出 1（合规）。
- `context cld1 --turns 2 --around 1 --max-chars 100` 正常截断；`--include-children` 合并 oc-child（3 轮）；`projects` 2 个项目；`--since/--until` 区间重叠正常；非法 `--platform` / `--phase` 报错退出 1。
- `extract cld1 --phase brainstorm` 2/7 轮（仅窗口 [1,3)，分组“任务：demo”）；`--phase implement` 5/7 轮（窗口外 + marker）；`--json` 含 windows `[{label demo [1,3)}]`、total_turns 7；无边界会话 brainstorm 返回全文+警告、implement 返回空+警告。
- 只读验证：全矩阵实测前后 11 个夹具文件哈希不变；ZCode -wal 专项：写连接持打开、-wal 有未检查点数据时，`context`/`extract` 能读到 -wal 里的追加行，且库文件与 -wal 文件哈希均不变（-shm 是读标记文件，排除在比对之外）。
- `mem help` 中文帮助完整；`grep -ri trellis` 在 internal/mem、internal/cli/mem.go、mem-recall 译文里零命中（唯一命中是 mem_test.go 里断言无残留的守卫本身）。

## 阶段 6 实施说明（Channel 多 Agent 协同）

依据：需求清单第 6 模块（核心事件溯源存储）、第 7 模块（CLI 命令、supervisor 与适配器）；源码对照 `packages/core/src/channel/`（api/ 16 文件、internal/store/ 11 文件）与 `packages/cli/src/commands/channel/`（约 40 文件，含 adapters/、supervisor/、store/ 平行副本），合计约 12927 行。本阶段是 7 个阶段中最重的一块。

### 范围

- `internal/channel/store`：事件溯源存储（唯一一份，见下）。
- `internal/channel`（core API 层）：create / send / read / watch / workers / spawn / interrupt / context / post-thread / title / resolve / inbox / runtime。
- `sunshine channel` CLI 命令全套（见命令表）。
- supervisor（`__supervisor` 子进程）与 claude/codex worker 适配器、guard、上下文注入与信任、agent 加载。
- 与阶段 3 已安装的 channel-driven-subagent-dispatch 工作流模板联动（本阶段让它真正能跑）。

### 核心决策

- **单一 store**：Go 只实现一份 `internal/channel/store`。Trellis 的 CLI 有一份平行副本（`packages/cli/src/commands/channel/store/`）且与 core 实现有差异（漂移源）；Go 版不复制该副本，CLI 与 API 共用同一份 store（阶段 2 已定）。
- **WorkerRuntime 接口定完整**：Trellis 存疑点 11——core 的 WorkerRuntime 接口没有 start 方法（只有 interrupt/stop），但 spawnWorker 调用的 runtime.start 在类型层面未声明。Go 版接口明确包含 Start / Interrupt / Stop 三个方法，解决该不一致。
- **无 `_legacy` 迁移**：Trellis 的 `_legacy/` 是扁平旧布局的迁移逻辑；Sunshine 是全新产品，无旧布局，不实现。

### 事件溯源存储（`internal/channel/store`）

- **存储根**：`~/.sunshine/channels/`，环境变量 `SUNSHINE_CHANNEL_ROOT` 可覆盖；按项目分桶 `~/.sunshine/channels/<projectKey>/<channel>/events.jsonl`；projectKey 由 cwd 按 Claude Code 约定生成（`/`、`\`、`_` → `-`，其余非法字符 → `-`）；global 桶固定 `_global`。
- **模型**：ChannelRef {name, scope: project|global, project, dir}；ChannelMetadata {type: chat|forum, title?, description?, context?, labels?}；Channel 类型仅 chat（时间线式）和 forum（主题区）；旧日志里的 "thread"/"threads" 不归一化为 forum。
- **事件日志**：每个 channel 一个 `events.jsonl`，每行一个 JSON 事件；另有 `.seq` sidecar 存最后 seq；BaseChannelEvent {seq, ts, kind, by, idempotencyKey?, to?, origin?: cli|api|worker, meta?}。
- **21 种事件 kind**：create, join, leave, message, thread, context, channel, spawned, killed, respawned, progress, done, error, waiting, awake, undeliverable, interrupt_requested, turn_started, turn_finished, interrupted, supervisor_warning。
- **appendEvent 写路径**：在 channel 锁内先截断撕裂的 JSONL 尾部，再按 idempotencyKey 去重（同 key 不同 kind 报错；同 kind 返回已有事件），seq = lastSeq + 1，追加一行 JSON，更新 `.seq` sidecar。
- **读与分页**：readChannelEvents（分页 {afterSeq?, beforeSeq?, limit?}，afterSeq 与 beforeSeq 不可同传；默认页大小 200；损坏行解析失败时跳过）；seq 对账 reconcileSeq 以 JSONL 尾部扫描为准修复 sidecar。
- **并发锁**：acquireLock 用 O_EXCL 原子建锁文件（Go：`os.OpenFile(O_CREATE|O_EXCL|O_WRONLY)`），文件内写持有者 pid；默认重试间隔 25ms、最长等待 5000ms；锁文件存在但 pid 已死则窃取；releaseLock 仅当文件内容等于自身 pid 时才删除；锁文件 `<channelDir>/<name>.lock`；另有每 worker 的 `<worker>.spawnlock`；所有 appendEvent 写操作都在锁内执行。
- **Worker 注册表投影**：reduceWorkerRegistry 是纯函数，只从事件日志折叠出 WorkerRegistry，不读 pid 文件、不读本地游标；WorkerState 含 workerId/channel/agent?/provider?/lifecycle/terminal/activity/activeTurnId?/activeTurnStartedAt?/pendingMessageCount/inboxPolicy/spawnedAt?/updatedAt/startedBy?/exitCode?/signal?/reason?/error?/idleSince?/lastSeq；生命周期 starting|running|done|error|killed|crashed（终态 done/error/killed/crashed）；活动 idle|mid-turn。
- **Inbox 策略**：explicitOnly（默认，仅消费 to 指向自己的消息）与 broadcastAndExplicit（另消费无 to 的广播）；matchesInboxPolicy 只对 kind: message 生效；worker 永不消费自己发的消息。
- **投递模式 DeliveryMode**：appendOnly（默认，只追加，永不报错，保留 pre-spawn 积压投递）、requireKnownWorker（目标从未出现 → 追加 undeliverable{reason: worker-unknown}）、requireRunningWorker（未知或终态 → undeliverable{reason: worker-unknown|worker-terminal}）；严格模式下消息本身先落盘再为每个失败目标追加 undeliverable 事件；广播消息永不产生 undeliverable。
- **Forum（主题区）**：postThread（opened/comment/status/labels/assignees/summary/processed）、renameThread、listForumThreads、showThread、collectThreadTimeline、rename 别名解析 ThreadAliasResolver；thread 状态/标签/负责人/摘要均以事件投影。
- **监听**：watchChannelEvents（Go 用 goroutine + channel 实现流式推送）、watchChannels 跨 channel 监听 + ChannelCursor（每 channel 一个 seq 游标）；watchWorkers / listWorkers / probeWorkerRuntime / reconcileWorkerLiveness。
- **Channel 上下文**：channel/thread 两级 context（file/raw 条目），API addChannelContext / deleteChannelContext / listChannelContext / addThreadContext / deleteThreadContext / listThreadContext；ContextEntry {type: file, path（须绝对路径）} 或 {type: raw, text}。
- **中断**：requestInterrupt 只追加 interrupt_requested 事件（持久化意图，不碰 runtime）；interruptWorker 在有 runtime 时尝试中断，返回 {event, interrupted, delivery}。

### CLI 命令表（`sunshine channel`，按模块 7）

- `create <name>`：`--scope project|global`、`--type chat|forum`（默认 chat）、`--task`、`--project`、`--labels`、`--description`、`--cwd`、`--by`（默认 main）、`--force`；`--context-file/--linked-context-file/--context-raw/--linked-context-raw`；`--ephemeral`（临时 channel，默认不列入 `channel list`，可由 `channel prune --ephemeral` 清理）。
- `send <name>`：`--to <worker>` 定向、`--stdin/--text-file` 正文来源、`--scope`、`--by`；正文为位置参数。
- `wait <name>`：阻塞等待匹配事件或超时；`--timeout <30s|2m|1h>`（Go 的 time.ParseDuration 直接支持）、`--from`、`--thread`、`--action`、`--include-progress`。
- `interrupt <name>`：打断 worker 当前回合并发送替换指令；`--stdin/--text-file`。
- `spawn <name>`：注册 worker（claude/codex），空闲等待第一条 send --to 才工作；`--agent`（加载 `.sunshine/agents/<name>.md`，设定默认 provider/model/system prompt）、`--provider claude|codex`、`--as`、`--cwd`、`--model`、`--resume`、`--sandbox`（仅 codex，默认 workspace-write）、`--timeout`、`--warn-before`（默认 5m，0 禁用）、`--file`（可 glob、可重复）、`--jsonl`（Trellis jsonl 清单 {file, reason}）、`--by`、`--inbox-policy explicitOnly|broadcastAndExplicit`（默认 explicitOnly）、`--idle-timeout`（默认 5m）、`--max-live-workers`（默认 6）。
- `run [name]`：一次性：建临时 channel → spawn worker → 发 prompt → 等完成 → 打印最终答案 → 清理；flags 与 spawn 大致相同，另有 `--message/--message-file/--stdin`。
- `rm <name>`：杀掉全部 worker 并删除整个 channel 目录；`--scope`。
- `prune`：`--scope`、`--all`、`--empty`、`--keep`，默认 dry-run，`--yes` 才真删。
- `list`：`--scope`、`--json` 等；`messages <name>`：`--thread`、`--action` 过滤；`kill <name>`：杀掉 worker。
- `post <name> <action>`：向 forum channel 追加结构化 thread 事件；`--as`（必填）、`--thread`（opened 外必填）、`--title`、`--description`、`--status`、`--labels`、`--assignees`、`--summary`、上下文 flags。
- `forum <name>`：列出 forum 的 thread（`--status` 过滤、`--raw`）；`thread`：查看/变更单个 thread 时间线；`rename <name> <oldThread> <newThread>`：重命名 thread。
- `context`：管理 channel/thread 级上下文条目；子命令 add/delete/list；`title`：设置/清除 channel 显示标题（set/clear 子命令）。
- `__supervisor`：内部 supervisor 命令；`__parse-trace`：内部开发用解析命令。

### Supervisor 与 worker 适配器

- **Supervisor 协同**：spawn 时派生 supervisor 子进程（`sunshine channel __supervisor`），监听事件并驱动 worker；仅 claude/codex 适配器；inbox 策略与投递模式见存储一节。
- **Worker 适配器**：解析 claude/codex 的 JSONL 输出流，提取 tool_use/tool_result、文本、token 用量、错误；支持 resume（`--resume <session-id>`）、sandbox（codex）、model 选择；适配器差异：claude 与 codex 的 JSONL 格式、工具名、错误形态均不同（按 `adapters/` 逐项实现）。
- **Worker 守卫（guard）**：idle 超时（默认 5m）、最大存活 worker 数（默认 6）、OOM guard、warn-before 预警；session 身份桥接（worker 的 session id 与 channel 关联）；配置来自 config.yaml 的 `channel.worker_guard`（阶段 3 已建模：idle_timeout 5m、max_live_workers 6）。
- **上下文注入与信任**：`--file` / `--jsonl` 上下文加载进 worker system prompt；context-trust 模块处理上下文来源可信度（按 `context-loader.ts` / `context-trust.ts` 实现）。
- **Agent 定义加载**：`--agent` 加载 `.sunshine/agents/<name>.md`（architect/check/implement/plan/research，阶段 3 已生成）。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- 存储层实测：并发 append（多进程同时写同一 channel，seq 连续无丢失、无撕裂行）、幂等去重（同 key 同 kind 返回已有事件、同 key 不同 kind 报错）、锁窃取（杀死持锁进程后可恢复）、reconcileSeq（删 sidecar 后按 JSONL 尾部修复）、分页（afterSeq/beforeSeq 互斥、默认 200、坏行跳过）。
- 投递与 inbox 实测：appendOnly / requireKnownWorker / requireRunningWorker 三种模式、undeliverable 原因、广播不产生 undeliverable、explicitOnly 与 broadcastAndExplicit、worker 不消费自己发的消息。
- CLI 实测：create/send/list/messages/wait（超时与匹配）、forum 全套（post/forum/thread/rename）、context/title、prune dry-run 与 --yes、rm；`channel run` 一次性流程（用 stub runtime）。
- Supervisor 与 worker 实测：用注入的 stub WorkerRuntime（实现完整 Start/Interrupt/Stop 接口，模拟 claude/codex JSONL 输出流）验证 spawn → send --to → worker 工作 → done 的完整闭环、idle 超时回收、warn-before、interrupt；沙箱如有 claude/codex 二进制再做真实拉起，否则记为待真实环境复验。
- 生成文件抽查：中文、无 trellis 残留；channel 存储根只在 `~/.sunshine/channels/`（或 SUNSHINE_CHANNEL_ROOT），不写项目目录。

## 阶段 6 实施记录

### A 组：事件溯源存储（`internal/channel/store`，2026-11-09，已完成）

- 代码：`internal/channel/store/`（doc.go、paths.go、schema.go、events.go、seq.go、lock.go、worker.go、inbox.go、delivery.go、filter.go、metadata.go、thread.go、watch.go），只做存储层，未碰 API、CLI、supervisor、forum。
- 导出 API：ResolveChannel / ResolveExistingChannelRef / AppendEvent / ReadChannelEvents / ReconcileSeq / ReconcileSeqFile / AcquireLock / ReleaseLock / WithLock / ReduceWorkerRegistry / ReduceChannelMetadata / ReduceThreads / BuildThreadAliasResolver / CollectThreadTimeline / MatchesInboxPolicy / MatchesEventFilter / ClassifyDelivery / AppendMessage / WatchChannelEvents / WatchChannels / ChannelCursor / ChannelCursorKey，及 ChannelRef / ChannelMetadata / ChannelEvent / AppendableEvent / WorkerState / WorkerRegistry / DeliveryMode / InboxPolicy 等类型。
- 验证：`go build`、`go vet`、`gofmt`、`go test ./internal/channel/store/ -count=1`（23 个测试）与 `-race` 全过；多 goroutine（20×25）与多进程（4 进程×25，测试二进制重跑自身）并发追加 seq 连续无丢失、无撕裂行；幂等去重、锁窃取、reconcileSeq、分页（默认 200、坏行跳过）、三种投递模式与 inbox 策略均有测试覆盖；生成文件与代码注释全中文，`grep -ri trellis internal/channel/store/` 零命中。
- 其余包未破坏：除 `internal/channel` 与 `internal/cli` 外全部包 `go test` 全过；`cmd/sunshine` 与 `internal/cli` 当时因并行组正在写的 `internal/channel` 包（forum.go 等引用未定义的 `Store`、同名符号重复声明）无法编译，与本组无关，待协调人合并时处理。

#### 与需求清单 / 源码的差异（以源码为准）

1. **锁窃取加 1 秒保护期（并发正确性必需）**：上游 `checkAndStealStale` 把 pid 无法解析的锁文件直接视为可窃取；Go 版 `AcquireLock` 在创建锁文件后写 pid 之前存在一个窗口，并发创建时其他等待者会读到空文件、误判为死锁而窃取，实测导致两个持有者同时写、seq 重复。Go 版改为：pid 无法解析时只在锁文件已存在超过 1 秒才窃取；能解析出已死 pid 时立即窃取（行为与上游一致）。
2. **WatchChannels 初始扫描读积压**：与上游 `watch-channels.ts` 一致——初始扫描到的 channel 读积压（fromStart），之后才发现的 channel 按 `fromStartNewChannels`（默认只推新事件）；有游标的 channel 一律从头读起、按 seq 过滤。
3. **AppendMessage 幂等重放不重复产生 undeliverable**：上游 `sendMessage` 不返回 undeliverable 列表，无此问题；Go 版 `AppendMessage` 返回本次新追加的 undeliverable 事件，消息重放时派生键已存在则跳过、返回空列表。
4. **EnsureChannelDir 一并写 `.bucket` 标记**：上游由 create 路径另行调 `ensureBucketMarker`；Go 版在 `EnsureChannelDir` 里一并保证，否则 `ListProjects` 找不到经 `AppendEvent` 创建的桶，`ResolveExistingChannelRef` 的多桶歧义判定失效。
5. **To 字段归一化**：读入时字符串或字符串数组统一为 `[]string`；写出时单个目标写成字符串、多个写成数组，与上游 JSON 形态一致；各 kind 专有字段与未知字段放在 `Extra` 里原样保留。
6. **WorkerState.ExitCode 为 `*int`**：区分“未设置”与“退出码 0”。
7. **无 `_legacy` 迁移**：按本阶段核心决策，Sunshine 是全新产品，不实现旧扁平布局迁移。
8. **锁窃取日志为中文**（写 stderr），符合 CLI/生成文件全中文的要求。
9. **ReconcileSeq 公开版不加锁**：仅用于读侧对账；写路径（AppendEvent）已在锁内调 `ReconcileSeqFile`。

### E 组：Forum 主题区（`internal/channel` forum.go / threadstate.go / forum_store.go、`internal/cli/channel_forum.go`，2026-11-09，已完成）

- 代码：`internal/channel/threadstate.go`（ThreadState、ThreadAliasResolver、ReduceThreads、CollectThreadTimeline，纯函数，只从事件日志折叠）、`internal/channel/forum.go`（PostThread、RenameThread、ListForumThreads、ShowThread、ReadForumChannelEvents、FormatThreadBoard、ParseThreadAction/ParseCSV/ParseScope/NormalizeThreadKey）、`internal/channel/forum_store.go`（ForumStore 最小接口 + fileStoreAdapter：经 JSON 往返把 API 层 ChannelEvent 接到 A 组 store 的 AppendEvent/ReadChannelEvents 上；频道解析复用 B 组 api.go 的 ResolveChannel）、`internal/cli/channel_forum.go`（`channel post/forum/thread/thread rename`，挂到 C 组 channel.go 的包级 channelCmd 上，root.go 的 init 在 RegisterChannelCommand 之后调 RegisterChannelForumCommand，幂等守卫防重复挂接）。
- 测试：`internal/channel`（forum_test.go、threadstate_test.go，12 个测试）与 `internal/cli`（channel_forum_test.go，4 个测试）全过；用临时 SUNSHINE_CHANNEL_ROOT 验证 post 六种动作全套、forum 列表与 --status 过滤、thread 时间线、rename（含别名解析：改名后旧名仍能解析到新 thread、二次改名走别名链）、chat 频道上 post/forum/thread/rename 均报 forum 类型错误；另用编译出的二进制实测（种子 create 事件建 forum 频道）：post opened/comment/processed、forum 列表、--status 过滤、rename、旧名查时间线、--raw JSON 均正常，chat 频道报错退出码 1。
- 验证：`go build ./...`、`go vet ./...` 全过；E 组文件 `gofmt` 干净、`grep -ri trellis` 零命中；生成文件与输出全部中文。

#### 与需求清单 / 源码的差异（以源码为准）

1. **ThreadState 增加 Processed/CreatedAt/UpdatedAt 等任务规格字段**：上游 ThreadState 字段为 thread/title/status/labels/assignees/description/context/summary/openedAt/updatedAt/lastSeq/comments/aliases；Go 版按任务书用 ID（json 键仍为 "thread"，与上游 --raw 输出一致）、Processed（状态为 processed 时为真）、CreatedAt（= openedAt）、UpdatedAt，另保留 Description/Context/Comments/Aliases/LastSeq。
2. **ListForumThreads 的 --status 过滤在 CLI 层做**：上游 core 的 listForumThreads 不带 status 参数，由 CLI 过滤；Go 版 API 同样不过滤，`channel forum --status` 在命令层过滤。
3. **forum 看板表头用中文**：上游 formatThreadBoard 表头为英文 `THREAD  STATUS  TITLE`、空表为 `(no threads)`；Go 版按全中文要求用 `主题  状态  标题`、空表为 `（暂无主题）`；行格式（`名 [状态] 标题 labels=… assignees=…`）与上游一致。
4. **thread 时间线与错误文案为中文**：上游 `thread show` 的 description/labels/assignees/summary 行与 `  <ts> <action> by=<by>` 时间线行为英文；Go 版用 `描述/标签/负责人/摘要` 与 `发帖人=`，其余格式（时间截到秒、context 事件标 `context-<动作>`）与上游一致。
5. **API 层与 store 层各有一份 thread 投影**：A 组 store/thread.go 已有 ReduceThreads/BuildThreadAliasResolver/CollectThreadTimeline（操作 store.ChannelEvent）；E 组 threadstate.go 是同语义的 API 层版本（操作 channel.ChannelEvent，B 组 types.go 的富事件结构）。两份投影行为一致（别名链展平、rename 只走别名解析、时间线含 thread 级 context 事件），合并时由总协调人决定是否合一。
6. **B 组 WatchChannels 的 CrossChannelEvent 别名问题（E 组最小修复）**：B 组 watch.go 的 WatchChannels 曾把转换后的 API 层事件赋给 store.CrossChannelEvent 的 Event 字段（类型不匹配，无法编译）；因 channel.CrossChannelEvent 是 store 同名类型的别名、其 Event 字段只能放 store.ChannelEvent，E 组改为原样转发 store 事件并加注释说明，待总协调人决定是否把 channel.CrossChannelEvent 改为独立结构后再转换。WatchEvents（单 channel）不受影响，仍转换后推送。
7. **并行落地期间的临时文件已清理**：E 组开工时 A/B/C 组尚未落地，E 组曾自带最小 event.go/store.go 先行实现；A 组 store 与 B 组 types.go/api.go 落地后，E 组已删掉这两份临时实现，改复用 A 组 store 与 B 组类型/ResolveChannel；B 组 types.go 一度短暂消失又重新落地，期间 E 组恢复的 event.go 也已随 B 组落地而删除，不留重复定义。

#### 顺手修的别组编译错误（均一两行、零行为变化，已在上文各节注明备查）

1. `internal/cli/channel.go`：interrupt 命令里 `scope, err := parseChannelScopeFlag(scope)` 左侧无新变量（scope 与 err 均已声明），改为 `=`；另有一处同形语句经核对是正确的首次声明，未动。
2. `internal/cli/channel_spawn.go`：缺 `"io"` 导入（`io.ReadAll` 未定义），补上。
3. `internal/channel/supervisor/run.go`：声明后未使用的 `logFile` 变量删除（`logWriter` 与 `defer f.Close()` 保留）。
4. `internal/channel/supervisor/guard.go`：引用不存在变量的死代码 `_ = ref` 删除。
5. `internal/channel/spawn_interrupt_test.go:359`：`inbox1, err = …` 中 err 未定义（前面的 err 只在 if 作用域内），改为 `:=`。

#### 遗留问题

1. **C 组 `channel create` 仍是未接线占位**：二进制实测时 `channel create board --type forum` 报“channel 功能尚未接入：内部 channel API（B 组）未就绪”，forum 的二进制端到端只能用种子 create 事件建频道；待 C 组把后端切到 B 组 API 后复验 `create → post → forum` 全流程。
2. **全量测试有两处既有抖动/失败与 E 组无关**：`TestWatchEventsLiveAndFromStart`（监听超时，store watch 的字节偏移竞态：监听 goroutine 首次 os.Stat 若晚于 SendMessage 落盘会漏掉新事件；单跑 3 次全过，全量跑时失败）与 C 组 6 个测试（wait/prune/context/spawn/run/messages，第一次全量跑失败、第二次全量跑全过，均为时序敏感）；`TestWatchWorkerInbox` 同样抖动。建议总协调人复核 watch 的初始偏移逻辑（应以 seq 游标而非文件字节偏移起步）与 C 组测试的超时余量。

### D 组：supervisor + claude/codex 适配器 + guard + 上下文注入与信任 + agent 加载（`internal/channel/adapters/`、`internal/channel/supervisor/`，2026-11-09，已完成）

- 代码：`internal/channel/adapters/`（types.go、claude.go、codex.go、runtime.go、stub.go）；`internal/channel/supervisor/`（supervisor.go、turns.go、shutdown.go、idle.go、warning.go、run.go、inbox.go、session.go、guard.go、context.go、trust.go、agent.go、trace.go、spawn.go）。只做 D 组范围，未碰存储（A 组）、API（B 组）、CLI 命令（C 组）、forum（E 组）。
- 适配器：claude 用 `--input-format stream-json --output-format stream-json --permission-mode bypassPermissions --dangerously-skip-permissions --verbose [--resume] [--model] [--append-system-prompt(-file)]`，system init 持久化 session_id，assistant 文本块 → message、tool_use → progress（含 MCP `mcp__server__tool` 拆分）、thinking 跳过、result → done（不复制 result 文本，带 duration_ms/total_cost_usd/num_turns/usage）或 error；codex 用 `app-server` JSON-RPC 2.0（initialize → thread/start → turn/start），thread/start 的 `result.thread.id` → persistThreadId + persistSessionId（同一值），commentary → progress（截断 4000 字）、final_answer/无 phase → message、turn/completed 失败去重、`mcpServer/elicitation/request` 自动 accept、collabAgentToolCall → error；`INLINE_SYSTEM_PROMPT_MAX_CHARS = 8000`，超长提示词走 `<worker>.system-prompt.md` 文件。
- supervisor：`RunSupervisor` / `RunSupervisorFromConfigFile`（`sunshine channel __supervisor` 入口，C 组已接线）/ `WriteSupervisorConfig`；写 `<worker>.pid`、超大提示词写 `<worker>.system-prompt.md`、环境变量 `SUNSHINE_HOOKS=0`、`SUNSHINE_CHANNEL`、`SUNSHINE_CHANNEL_AS`；spawned 事件须等子进程 spawn/error 先定（失败只写 supervisor 的 error 事件、不写 spawned）；关停宽限 `SHUTDOWN_GRACE_MS = 3000`（关 stdin → 3 秒 → SIGTERM → 3 秒 → SIGKILL）；退出时排干 stdout（3 秒上限）再合成兜底终态、清理临时文件（保留 log、session-id/thread-id、inbox-cursor）；idle 定时器在 spawned 后启动，turn_started 暂停、turn_finished/interrupted 重置，到时按 idle-timeout 关停；warn-before 默认 5 分钟、在 `timeoutMs - warnBeforeMs` 发一次 `supervisor_warning{reason: approaching_timeout}`；收件箱 100ms 轮询持久游标 `<worker>.inbox-cursor`，message 按 inbox 策略过滤、interrupt_requested 按 worker 字段过滤，非中断等当前回合结束再投递，中断先 abort 回合再发 `interrupted{reason: user, method: stdin, outcome: interrupted|no-active-turn}`。
- guard：`ResolveGuardConfig`（优先级 flag > 环境变量 `SUNSHINE_CHANNEL_WORKER_IDLE_TIMEOUT` / `SUNSHINE_CHANNEL_MAX_LIVE_WORKERS` > `.sunshine/config.yaml` 的 `channel.worker_guard` > 内置默认 idle 5m / max 6；裸整数按毫秒计）；`ScanLiveWorkers` 只认非终态 + supervisor pid 存活的 worker（含 `.reservation` 预留位），`isSupervisorProcess` 用 `ps -p <pid> -o args=` 匹配 `channel __supervisor <channel> <worker>`；清理先写 `shutdown-reason` 文件（`idle-timeout`）再 SIGTERM；`EnforceSpawnBudget` 先清理、重扫（排除刚杀的）、再判预算。
- 上下文与信任：`AssembleContext`（--file 支持 glob `*? **`、可重复；--jsonl 为 `{file, reason}` 清单，跳过 `_example` 行；路径相对 cwd，缺失只警告；`jailedRealpath` 校验真实路径在 cwd 或信任根内；单文件硬上限 1MB、200KB 警告、总超 500KB 警告；块头 `# Context: <path>`，jsonl 带 `# Reason:`）；`ResolveTrustedRoots`（`channel.worker_guard.trusted_context_dirs` + `.sunshine/tasks` / `.sunshine/workspace` 顶层符号链接窄幅自动信任，`auto_trust_sunshine_symlinks` 默认开）；`BuildSystemPrompt`（协议前缀 + `# AGENT ROLE` + `# CONTEXT FILES`）；`LoadAgent`（`.sunshine/agents/<name>.md` 或 `<name>/AGENT.md`，frontmatter + 正文，名字须匹配 `^[A-Za-z0-9._-]+$`，极小扁平 YAML 解析器，拒绝 `__proto__/prototype/constructor`）。
- spawn：`ResolveSpawn`（agent → provider/model/as 回落，组装上下文，组合系统提示词）+ `SpawnWorker`（项目桶级 `.worker-guard.lock` 内做 `EnforceSpawnBudget`，再 worker 级 `.spawnlock` 内写 supervisor 配置与 `.reservation`，detached 派生 `sunshine channel __supervisor <channel> <worker> <config>`，环境变量 `SUNSHINE_CHANNEL_PROJECT`）；`RunParseTrace`（`sunshine channel __parse-trace`，codex 预置 pending id 1=initialize、2=thread/start、3=turn/start）。
- 验证：`go vet ./internal/channel/adapters/ ./internal/channel/supervisor/`、`go test ./internal/channel/... -count=1`（channel、adapters、store、supervisor 4 包）与 `go build ./...` 全过。supervisor 包 33 个测试、adapters 包 36 个测试：StubWorkerRuntime 闭环（spawn → send --to → worker 工作 → done，含 turn_started/turn_finished、session-id 桥接、广播不投递 explicitOnly、取消后 killed explicit-kill、临时文件清理）、broadcastAndExplicit、idle 超时回收（TTL 300ms → killed idle-timeout）、warn-before（timeout 1500ms、warnBefore 800ms → supervisor_warning 后 killed timeout；warnBefore 0 禁用）、空闲中断（outcome: no-active-turn）、回合中中断（turn_finished{aborted} + interrupted{interrupted}）、冷退出兜底合成 done/error、Start 失败只写 supervisor error 不写 spawned、guard 解析优先级/预算/清理发 SIGTERM、上下文 glob/jsonl/jail/1MB 上限、agent 加载/路径穿越/块标量、trace 解析、SpawnWorker 写盘 + 重复 spawn 报错、SIGTERM 优雅关停。
- 真实拉起（沙箱，2026-11-09）：`which claude` 无结果，claude 真实拉起待真实环境复验；`codex` 为 `/opt/hatch-image/bin/codex`（codex-cli 0.164.0-alpha.9）。用真实 codex 二进制走 `processRuntime` 探针：Start + 握手（initialize → thread/start）成功，Send 成功；回合随后失败——codex 向 `wss://api.openai.com/v1/responses` 与 `https://api.openai.com/v1/responses` 请求时报 401 Unauthorized（Missing bearer 认证，沙箱无 codex 登录信息与 API Key），经 5 次重连后回落 HTTPS 再 401，最终以 error 事件上报；适配器对 warning/error 通知的解析与上报均正常。另有一条环境警告：code-mode host 二进制缺失（`/opt/hatch-image/bin/codex-code-mode-host` 不存在），与本组代码无关。
- 真实二进制端到端（假 claude 脚本，`sunshine channel create → spawn --agent implement → send --to implement → wait --kind done`）：两回合均完成（turn_started → message → done → turn_finished），`wait`（先启动、后发送）即时返回 done 事件，session-id 文件正确桥接（`fake-sess-1`）。注意：`wait` 只看调用之后新到的事件，事后启动的 wait 等不到已落盘的 done，这是设计行为、不是缺陷。
- 工作流模板联动：模板引用的 `create`（--task/--by/--ephemeral）、`spawn`（--agent/--provider/--as/--cwd/--timeout）、`send`（--as/--to/--text-file）、`wait`（--as/--kind/--from/--all/--timeout）、`messages`（--raw/--from/--last）命令与 flags 均已注册；`__supervisor` 已接 `supervisor.RunSupervisorFromConfigFile`，`__parse-trace` 已接 `supervisor.RunParseTrace`。无缺口。

#### 与需求清单 / 源码的差异（以源码为准）

1. **WorkerRuntime 接口扩展**：PLAN 定的接口只有 Start/Interrupt/Stop 三方法（对应上游存疑点 11 的修复）。Go 版补充 Send/Wait/Ready/ExitState：上游 supervisor 直接写 `child.stdin`，Go 版把子进程封装在 runtime 内，supervisor 必须经 runtime 投递消息、等退出、查就绪与退出状态。
2. **Claude tool_result 提取**：上游 `user` 行直接跳过（嫌噪音大）；本包按任务要求提取为 progress 摘要事件（`detail{tool_result: true, tool_use_id, output_summary}`，内容截断 200 字）。
3. **Claude token 用量提取**：上游不提取；本包按任务要求提取 `assistant.message.usage`（顶层 usage 兜底）进 message 事件，result 行的 usage 进 done 事件。
4. **Codex token 用量提取**：上游跳过 `thread/tokenUsage/updated`；本包按任务要求提取为 progress 事件（`detail{kind: token_usage, usage: {total, last}}`）。
5. **WarnBeforeMs 用指针**：上游 `warnBeforeMs` 未设置时默认 5 分钟，`<=0` 禁用，无法区分“未设置”与“显式 0”。Go 版 `SupervisorConfig.WarnBeforeMs *int64`：nil = 默认 5 分钟，0 = 禁用，>0 = 显式提前量。
6. **信任配置位置**：上游信任键在 `channel:` 直属下（`channel.trusted_context_dirs` / `channel.auto_trust_trellis_symlinks`）；Go 版 config（阶段 3 已建模）把它们放在 `channel.worker_guard` 下（`trusted_context_dirs` / `auto_trust_sunshine_symlinks`），`ResolveTrustedRoots` 按 Go 版位置读取。
7. **收件箱监听实现**：上游用 `watchEvents` AsyncGenerator 流式推送；Go 版用 100ms 轮询 `ReadChannelEvents{AfterSeq: cursor, Limit: 200}`（store 暂无 watch API 导出，D 组不另起存储）。语义一致：游标持久化、策略过滤、中断处理、投递失败推进游标。
8. **Timeout 定时器取消**：上游 `setTimeout(...).unref()` 不取消；Go 版在 `RunSupervisor` 返回时 `defer timeoutTimer.Stop()`，避免 worker 正常退出后定时器仍触发 `Request(timeout)` 追加 killed 事件。
9. **Spawn scope 默认**：上游 `parseChannelScope` 未指定时由 `resolveExistingChannelRef` 自动解析；Go 版 `SpawnWorker` 在 opts.Scope 为空时传空 Scope（不预设 project），由 `store.ResolveExistingChannelRef` 做同一自动解析。
10. **信号接管放在进程入口**：上游 supervisor 进程自己处理 SIGTERM/SIGINT/SIGHUP。Go 版 `RunSupervisorFromConfigFile` 内用 `signal.NotifyContext` 接管三信号、转成 ctx 取消；CLI `__supervisor` 入口传的是 `context.Background()`（C 组预留入口未接信号），不在进程里接信号的话 SIGTERM 会直接杀掉进程、来不及写 killed 事件与清理临时文件——实测已复现并修复，单元测试 `TestSupervisorSigtermGracefulShutdown` 与真实二进制 SIGTERM 复验均通过。
11. **上下文绝对路径修补**：`expandGlob` / `--jsonl` 原实现用 `filepath.Join(cwd, spec)`，Go 的 Join 不丢弃前段，绝对路径会被拼成 `cwd + 绝对路径`（如 `/tmp/x/secret.md` 变成 `/tmp/proj/tmp/x/secret.md`，文件不存在）。测试 `TestAssembleContextJail` 发现后修补：新增 `joinCwd`（绝对路径原样保留），绝对路径 glob 从根目录起步。上游按 cwd 解析相对路径，绝对路径行为未明确，此处按“绝对路径应可用”处理。
12. **warn-before 与 timeout 相等时预警立即触发**：`--timeout 5m` + 默认 warn-before 5m 时，预警在 `timeoutMs - warnBeforeMs = 0` 处、即 spawn 后立即发出（上游 `setTimeout` 负延迟同款语义）。端到端实测已观测到该行为；如需“预警不早于 spawn 后 N 秒”，属产品策略，待总协调人定。
13. **StubWorkerRuntime 的 Interrupt 播放下一回合**：真实 worker 收到中断帧后会按替换指令继续工作直至回合结束；stub 的 Interrupt 在取消当前播放后同步播放下一回合脚本（模拟该行为），使中断闭环测试能等到 done；Send 仍为逐回合播放，回合耗尽后重复最后一回合。

#### 遗留问题

1. **claude 真实拉起未做**：沙箱无 claude 二进制，待真实环境复验（`processRuntime` + 真实 claude 的 Start/握手/回合）。
2. **codex 真实回合未完成**：握手与 Send 已通，但沙箱无 codex 登录信息与 API Key，回合在 401 失败；待有认证的真实环境复验完整回合（message → done）。
3. **D 组新文件 `grep -ri trellis` 剩余 5 行、3 类**：兼容回退读旧值（`guard.go` 的旧环境变量 `TRELLIS_CHANNEL_WORKER_IDLE_TIMEOUT` / `TRELLIS_CHANNEL_MAX_LIVE_WORKERS`、`spawn.go` 的 `TRELLIS_CHANNEL_AS` 回落）与测试中断言无残留的守卫本身（2 行）；生成文件与 CLI 输出无 trellis 字样。
4. **CLI `__supervisor` 入口的 TODO 注释未清**：`internal/cli/channel_manage.go` 的 `newChannelSupervisorCmd` / `newChannelParseTraceCmd` 里仍留着“D 组提交后改指”的 TODO，实际接线已在 `internal/cli/channel_backend.go`（`runSupervisorFunc` / `runParseTraceFunc`）完成；属 C 组文件，D 组未动，待 C 组清理。

## 阶段 6 实施记录

### 阶段 6 实施记录（B 组：core API 层，2026-11-09）

范围：新建 `internal/channel/` 的 API 层（create / send / read / watch /
workers / spawn / interrupt / context / title / runtime），类型收敛与
测试替身。未动 `internal/channel/store/`（A 组）、`forum.go` /
`threadstate.go` / `forum_store.go`（E 组）、`adapters/` /
`supervisor/`（D 组）、`~/workspace/research/trellis-source/`。

#### 交付

- `internal/channel/types.go`：共享类型。forum 与 core API 共用；除
  ChannelEvent 外全部是 store 子包同名类型的别名（ChannelRef、
  ChannelMetadata、ContextEntry、WorkerState、WorkerRegistry、
  DeliveryMode、InboxPolicy、InterruptReason/Method/Outcome、
  ThreadAction、ContextTarget、Pagination 等，零转换成本）；
  ChannelEvent 是 API 层的富事件结构（基础字段 + 各 kind 具名字段，
  未知字段进 Extra；Extra 同时镜像全部已知字段，供 threadstate 的
  contextTarget 读 `target` 用；`to` 接受字符串或数组，
  `title` 区分 null 与缺失），与 store.ChannelEvent 之间用 JSON
  往返转换（复用 `forum_store.go` 的 fromStoreEvent）。
- `internal/channel/doc.go`：包文档（API 层 + forum，持久化走 store 子包）。
- `internal/channel/api.go`：寻址（ChannelAddress）与解析
  （ResolveChannel / ResolveChannelForCreate）、选项类型、辅助函数。
- `internal/channel/runtime.go`：WorkerRuntime / WorkerSpec / WorkerStart。
- `internal/channel/create.go`：CreateChannel（含 --force 清理）。
- `internal/channel/send.go`：SendMessage（三种投递模式）。
- `internal/channel/read.go`：ReadEvents（分页透传）、
  ReadChannelMetadata、ReadWorkerInbox。
- `internal/channel/watch.go`：WatchEvents、WatchChannels、
  WatchWorkers、WatchWorkers 的快照、WatchWorkerInbox。
- `internal/channel/workers.go`：ListWorkers、ProbeWorkerRuntime、
  ReconcileWorkerLiveness。
- `internal/channel/spawn.go`：SpawnWorker。
- `internal/channel/interrupt.go`：RequestInterrupt、InterruptWorker。
- `internal/channel/context.go`：channel/thread 两级 context 增删查。
- `internal/channel/title.go`：SetChannelTitle、ClearChannelTitle。
- `internal/channel/teststub.go`：StubRuntime（记录调用、可注入失败，
  供 C/D/E 组联调用；不要在生产路径使用）。
- 测试：`api_test.go`、`create_send_test.go`、
  `spawn_interrupt_test.go`、`context_title_test.go`、
  `watch_workers_test.go`（临时目录存储根 + StubRuntime，
  与 store 包自身的测试做法一致）。

#### 与需求清单 / 源码的差异（以源码为准）

1. **WorkerRuntime 接口完整版（存疑点 11 已解决）**：Go 版接口明确
   包含 Start / Interrupt / Stop 三个方法；上游 core 的接口缺 start
   方法但 spawnWorker 调了它。另按任务书简化：Interrupt / Stop
   签名为 `(ctx, workerID string) error`（上游 interrupt 带
   turnId/reason/message、stop 带 reason/signal/force 结构）；需要
   这些细节时由 D 组适配器层补，不进 core 接口。
2. **类型收敛**：channel 包内此前 event.go / store.go 的最小模型
   （ChannelEvent、ChannelRef、ChannelMetadata、FileStore）已被 E 组
   在重构中删除，改由本组 types.go 提供；forum.go 注释所说的
   “类型复用 types.go”现已成立。ChannelEvent 与 store.ChannelEvent
   两套事件模型并存是刻意的分层（API 层富结构 vs store 的
   Extra 结构），转换只走 JSON 往返，不手写字段映射。
3. **无独立 Store 接口**：A 组的 store 落地为包级函数（非接口），
   API 层直接调 `store.AppendEvent` / `store.ReadChannelEvents` /
   `store.WatchChannelEvents` / `store.WatchChannels` /
   `store.ReduceWorkerRegistry` / `store.ClassifyDelivery` /
   `store.AppendMessage` / `store.ReduceChannelMetadata` /
   `store.ReduceThreads` / `store.ResolveChannel` /
   `store.ResolveExistingChannelRef` 等，不再另立接口；
   ForumStore 仍是 E 组 forum 专用的最小接口（见 forum_store.go）。
4. **SendMessage 直接用 `store.AppendMessage`**：严格模式的
   undeliverable 追加（含 `<key>:undeliverable:<target>` 的幂等键
   派生）由 store 实现，API 层只做参数校验与事件转换。
5. **By 为空默认 main**（与 CLI 的 --by 默认值一致；上游 core 无默认值）。
6. **CreateChannel --force**：先结束 channel 目录里 .pid 文件指向的
   活进程（SIGTERM，等 1.5 秒仍活着则 SIGKILL），再删整个目录，
   与上游 forceCleanChannel 一致。
7. **Watch 是 Go channel 形式**（`(<-chan, <-chan error)`），
   对应上游的 AsyncGenerator。WatchEvents / WatchWorkers /
   WatchWorkerInbox 在调用返回前同步快照最后 seq（无 FromStart /
   SinceSeq 时以它为续跑点走 HasSinceSeq），避免 store watch 的
   tail 偏移在 goroutine 内取值导致的竞态丢事件（监听启动后才
   落盘的事件也可能因偏移已越过而丢失）；WatchWorkerInbox 的
   worker 校验（不存在/终态）同样同步返回错误，不走 errCh。
   WatchWorkers / WatchWorkerInbox 的过滤与上游一致：
   includeNonMeaningful=true、includeProgress=false。
8. **InterruptWorker 无 runtime 时**仍追加 interrupted 事件
   （method none / outcome unsupported），与上游无 interrupt
   实现时的行为一致；有 runtime 且失败时 outcome 为 failed，
   message 写 runtime 的错误。
9. **spawn 空闲语义**：spawn 只调 runtime.Start 并追加 spawned
   事件，不发消息、不派活；新 worker 的 pendingMessageCount 为 0、
   activity 为 idle，等待第一条 send --to。
10. **thread 级 context 的已知判定**用 `store.ReduceThreads`
    （thread 事件先建状态，context 事件不建状态），与上游
    assertKnownThread 一致；追加时写传入的规范化键（不先解析别名），
    读时由别名解析器展平，与上游一致。
11. **Resolve 顺序**与上游 resolve.ts 一致：显式 scope 只查该桶；
    未指定时 global 与项目桶同名即报错，global 优先于当前项目桶，
    再按其他项目桶搜索；ProjectKey 非空时直接定桶。
12. **未做**：forum 的 PostThread / RenameThread /
    ListForumThreads / ShowThread（E 组，已有）；supervisor /
    适配器（D 组）；CLI 命令（C 组）；KillWorker（rm/kill 命令层）。

#### 验证（2026-11-09）

- `gofmt`、`go build ./...`、`go vet ./internal/channel/ ./internal/channel/store/`、
  `go test ./internal/channel/ -count=1` 全过（33 个测试：本组新增 21 个
  + E 组 forum/threadstate 12 个）；`go test ./internal/channel/store/` 全过；
  `go test ./...` 全仓 17 个包全过。
- 本组测试覆盖：spawn→send→read→interrupt→done 闭环、spawn 空闲语义
  （spawn 后只有 create + spawned 两条事件）、三种投递模式与
  undeliverable 原因（worker-unknown / worker-terminal、广播不产生
  undeliverable）、explicitOnly 与 broadcastAndExplicit、worker 不消费
  自己发的消息、context 两级增删查与校验（相对路径 file、空 raw、
  空列表均报错）、title 设置/清除（含 `title: null` 的 JSON 往返）、
  WatchEvents（实时 + FromStart 回放）、WatchChannels（动态发现新
  channel，游标快照）、WatchWorkers（快照 + 终态后空快照）、
  WatchWorkerInbox（定向消息到达、终态后流关闭、未知 worker 同步报错）、
  ProbeWorkerRuntime（活/死 pid）、ReconcileWorkerLiveness（只提议
  不写盘、追加落盘后 worker 变 error 终态、再次对账不再提议）、
  幂等去重（同 key 同 kind 返回已有事件、同 key 不同 kind 报错）、
  分页（afterSeq/beforeSeq 互斥）。
- 生成文件与测试输出抽查：中文、无 trellis 残留（本组新文件
  `grep -ri trellis` 零命中；唯一命中是 E 组 forum.go 首行注释里
  的源码对照说明，非生成物）。

#### 遗留（交总协调人）

- channel 包内两份投影并存，合并时统一：ThreadState /
  ReduceThreads（channel/threadstate.go 的 API 层版 vs
  store/thread.go 的 store 版，语义一致）；ReduceChannelMetadata
  （channel/forum.go 的简化版只读 create 事件的 type/description/
  labels/title，store/metadata.go 的完整版另含 context add/delete
  与 title set/clear 投影——ListChannelContext 用的是完整版，
  forum 的 ReadForumChannelEvents 用的是简化版，两者只在 type 判定
  上一致，合并时以完整版为准）。
- ChannelEvent 与 store.ChannelEvent 两套模型并存是刻意分层，
  但后续如需在热路径上避免 JSON 往返，可评估把 API 层事件
  改为 store 事件的别名（需 forum.go / threadstate.go 改用
  StrField / Extra 访问，改动面较大，另立任务）。
- `sunshine channel` CLI 命令尚未接线（C 组）：接线时直接调本包
  函数，存储根由 SUNSHINE_CHANNEL_ROOT 环境变量决定；
  spawn 的 SystemPrompt 由调用方（agent 加载）提供，本包只校验非空。
- 阶段 3/4 遗留与本组无关：`version` 字段与 `.sunshine/.version`
  含义重复、ts-sdk-author 是否改为 Go SDK 编写 skill。

### 阶段 6 实施记录（C 组：CLI 命令全套，2026-11-09）

范围：`sunshine channel` 根命令与除 forum 外的全部子命令
（create / send / wait / interrupt / spawn / run / rm / prune /
list / messages / kill / context / title / `__supervisor` /
`__parse-trace`），全部中文帮助与输出；forum 的 post/forum/thread/
rename 由 E 组负责。未动 `internal/channel/`（B 组）、
`internal/channel/store/`（A 组）、`internal/channel/supervisor/` 与
`internal/channel/adapters/`（D 组）、`~/workspace/research/trellis-source/`。

#### 交付

- `internal/cli/channel.go`：`channelCmd` 包级变量（供 E 组
  `channel_forum.go` 挂接）、`RegisterChannelCommand` /
  `buildChannelCmd`（每次新建命令树，flags 不跨调用残留）、
  create / send / wait / interrupt；共享 helpers：`parseCSV`、
  `parseChannelScopeFlag`、`parseChannelTypeFlag`、
  `validateChannelName`、`resolveSender`（`--by` 优先、`--as` 为隐藏
  兼容写法）、`parseDurationFlag`（`time.ParseDuration`，另兼容纯
  数字视为秒）、`resolveChannelTextBody`（位置参数 >
  `--text-file` > `--stdin`；必填时缺正文/空白正文报错，与上游
  text-body.ts 一致）、`buildContextEntries`（file 须绝对路径）、
  `validateKindList`、`validateThreadAction`、`printChannelEventJSON`、
  `channelEventMatchesWait`（与上游 `matchesEventFilter` 同语义：
  排除自己发的事件；无显式 kind 时只认 meaningful kind；progress
  须 `--include-progress`；`--thread/--action` 只对 kind=thread
  生效；`--to` 无 to 的广播算匹配，有 to 的须包含目标）。
- `internal/cli/channel_backend.go`：CLI 与 B 组 API 之间的接线面。
  `channelBackend` 接口（方法名与 PLAN.md 阶段 6 规格一致）；
  `WorkerRuntime` 接口（Start/Interrupt/Stop，与 B 组一致）；
  `channelWorkerRuntime` 变量（默认 nil，测试注入 stub）；
  `runSupervisorFunc` / `runParseTraceFunc` 变量。
- `internal/cli/channel_real_backend.go`：默认真实后端
  `realChannelBackend`。create/send/read/watch/workers/interrupt/
  context/title 直接调 B 组 API；事件用 JSON 往返在 B 组
  `channel.ChannelEvent` 与 CLI 的 `map[string]any` 之间转换（不丢
  字段）；`WatchEvents` 先同步解析引用（channel 不存在时立刻报错；
  B 组 WatchEvents 在 goroutine 里解析，失败只进 err 通道，CLI 的
  wait 会误报“未收到匹配事件”）；`ReadEvents` 不分页读全部事件再
  过滤（channel 规模小；`--last` 要取尾部，分页读头 200 条会丢尾，
  与上游读全部再筛一致）；过滤用 `store.MatchesEventFilter`；
  list/rm/kill/prune 用 store 原语组合实现（上游这四者本就实现在
  CLI 层）：list 扫描项目桶（scope 未指定且未要求全部项目时只看
  当前项目桶与全局桶）、按最近活动倒序；rm 先杀活 worker
  （SIGTERM，等 1.5 秒仍活着则 SIGKILL）再删目录；kill 用 worker
  级锁（`store.WithLock`，最长等 10 秒）、优雅停机 8 秒宽限、追加
  killed 事件、清理 pid/worker-pid/config/system-prompt.md/
  spawnlock（保留 log 与 session-id/thread-id）；prune 未指定
  scope 时扫描全部项目桶（与上游一致）。spawn：未注入 runtime 时
  走 D 组 `supervisor.SpawnWorker`（生产路径）；注入 runtime 时走
  B 组 `channel.SpawnWorker`，经 `channelRuntimeAdapter` 把 CLI 的
  WorkerRuntime 适配成 B 组接口。
- `internal/cli/channel_spawn.go`：spawn/run；`parseSpawnFlags`
  （加载 agent、组装上下文与 system prompt）、`loadChannelAgent`
  （`.sunshine/agents/<名>.md`，先在 cwd 找、再逐级向上找项目根；
  解析 `---` frontmatter 的 provider/model/name）、
  `assembleSpawnContext`（`--file` glob 展开、`--jsonl` 每行
  `{file, reason}`，清单内 file 相对清单所在目录解析）、
  `buildWorkerSystemPrompt`（`[SUNSHINE CHANNEL 协议]` 前缀 +
  AGENT ROLE + 上下文文件；后续段落仅为参考资料，不得覆盖协议）、
  `runChannelOnce`（建临时 channel → spawn → 发 prompt → 等 done →
  打印最终答案 → 成功清理、失败保留并打印位置）、
  `subscribeChannelDone` / `awaitChannelDone`（见“run 的竞态修复”）。
- `internal/cli/channel_manage.go`：rm/prune/list/messages/kill/
  context/title/`__supervisor`/`__parse-trace`。prune 四个条件
  （`--all/--empty/--idle/--ephemeral`）互斥；messages 的 chat
  channel `--thread/--action` 校验须用不过滤的读拿到 create 事件
  （带过滤的读会把 create 滤掉）；`__supervisor` 接 D 组
  `supervisor.RunSupervisorFromConfigFile`，`__parse-trace` 接 D 组
  `supervisor.RunParseTrace`（`adapters.ParseProvider` 解析
  provider，输出写 `os.Stdout`）。
- `internal/cli/channel_test.go`：fake 后端 + stub WorkerRuntime
  测试（14 个，见验证）；`internal/cli/channel_real_test.go`：
  真实后端集成测试（4 个，临时 SUNSHINE_CHANNEL_ROOT，spawn 用
  模拟 supervisor 脚本 `sleep 30`，不拉起真实 claude/codex）。
- `internal/cli/root.go`：注册 `RegisterChannelCommand(rootCmd)` 与
  `RegisterChannelForumCommand()`（后者由 E 组文件提供，无参版）。

#### 与需求清单 / 源码的差异（以源码为准）

1. **发送者 flag**：PLAN.md 命令表写 `--by`，上游 send/wait/
   interrupt/context/title 用 `--as`（send/wait/interrupt/kill
   还是必填）。Go 版两者都接受，`--by` 优先，`--as` 为隐藏兼容
   写法；默认 main。kill 的 `--as` 仍为 worker 名（必填），与上游
   一致。另保留 `TRELLIS_CHANNEL_AS` 环境变量只读回退（兼容旧
   值，不产生新 trellis 输出）。
2. **wait 超时退出码**：上游超时退出码 124；Go 版 cobra `RunE`
   返回错误时进程退出码为 1。`waitTimeoutError` 的报错文案里注明
   “上游退出码 124”，但实际退出码仍为 1。
3. **`--type thread/threads`**：上游 `ParseChannelType` 对旧值报错
   “请用 `--type forum`”；Go 版 `parseChannelTypeFlag` 对非
   chat/forum 一律报“类型须为 chat 或 forum”。
4. **`create` 的 `meta` 键**：上游为 `meta.trellis.createMode`；
   Go 版改为 `meta.sunshine.createMode`（不产生新 trellis 输出）。
5. **`spawn --sandbox` 校验时机**：上游 `parseCodexSandboxMode`
   只在传了值时解析；Go 版未传时默认 `workspace-write`，且
   claude 配 `--sandbox` 时报错（与“仅 codex”一致）。
6. **`run` 的 `--timeout` 默认值**：上游 run 的 timeout 默认 5m；
   Go 版 `parseDurationFlagDefault(timeoutStr, 5*time.Minute)`，
   与上游一致；但 spawn 的 `--timeout` 未传时为 0（不设），run 会
   把 spawn 的 Timeout 覆盖为 run 的 timeout。
7. **B 组 API 名字与 PLAN.md 规格的出入**：B 组 `CreateChannel`
   只返回事件不返回 ref（ref 由 CLI 经
   `ResolveChannelForCreate` 另取）；`SpawnWorker` 返回
   `WorkerState` 而非 `{pid, log, worker}`（pid/log 须由 CLI 从
   `WorkerStart` 与 `store.WorkerFile` 补）；`InterruptWorker`
   的 Delivery 为枚举（`interrupted-current-turn` 等），CLI 转
   字符串输出；`WatchEvents` 返回双通道（事件 + 错误），CLI 须先
   同步解析引用。合并时以 B 组为准，CLI 的 `channelBackend` 接口
   是适配层，不直接暴露 B 组签名。
8. **D 组 `adapters.WorkerRuntime` 与 B 组 `channel.WorkerRuntime`
   接口不同**（D 组多 Send/Wait/Ready/ExitState）；CLI 的
   WorkerRuntime 接口（Start/Interrupt/Stop）与 B 组一致，D 组
   真实 runtime 须经 supervisor 层接入，CLI 不直接持有 adapters
   runtime。
9. **channel 名只允许字母、数字、`.`、`_`、`-`**：上游
   `assertSafeName` 与 B 组 store 的 `IsSafeName` 均如此（名字会
   成为文件系统路径段；中文名、空格等一律拒绝）。CLI 的
   `validateChannelName` 已按同一规则收紧，提前报错；帮助示例与
   全部测试均已改用合法 ASCII 名（如 review / worker1）。
10. **run 的竞态修复**：`runChannelOnce` 原先先发 prompt 再订阅
    done 事件；stub worker 回得快时 done 可能在订阅前落下，
    from-now 订阅会永远等不到（全套测试里偶发 30s 超时，单测必过）。
    已改为先订阅（`subscribeChannelDone`）再发 prompt，再等
    （`awaitChannelDone`）。上游 run.ts 同样先发后等，但真实
    supervisor 启动有延迟，生产上不易触发；先订阅 strictly 更
    正确。

#### 验证（2026-11-09）

- `gofmt`、`go build ./...`、`go vet ./internal/cli/`、
  `go test ./internal/cli/ -count=1` 全过（连续 3 次）；
  `go test ./...` 全仓全过。
- C 组测试 18 个：fake 后端 14 个（create/send/list/messages、
  create 重复与 `--force`、send 校验、wait 匹配与超时、wait
  `--all`、prune dry-run 与 `--yes`、rm、context/title、
  spawn 与 kill、spawn 带 agent 与文件、run 一次性流程、
  messages 过滤、名称校验、中文帮助无 trellis 残留）+ 真实后端
  4 个（端到端 create/send/list/messages/wait 超时与匹配/prune/
  rm、context/title、spawn 与 kill（模拟 supervisor 脚本）、
  `__supervisor`/`__parse-trace` 报错路径）。
- 真实后端实测：临时 `SUNSHINE_CHANNEL_ROOT` 下 create/send/
  list/messages/wait（超时与匹配）、prune dry-run 与 `--yes`、
  rm、context/title、`__supervisor`/`__parse-trace` 均通过；
  spawn 经 D 组 `supervisor.SpawnWorker` + 模拟 supervisor 脚本
  （sleep 30）验证派生、pid 文件落盘、kill（SIGTERM 8 秒宽限后
  SIGKILL）与 killed 事件落盘均通过。
- 生成文件与输出抽查：中文、无 trellis 残留（本组新文件
  `grep -ri trellis` 唯一命中是 `TRELLIS_CHANNEL_AS` 环境变量
  只读回退与测试里的断言本身，不产生新 trellis 输出）。

#### 遗留（交总协调人）

- `channel run` 的 `--scope`/`--by` 在 run 命令上未单独设 flag
  （由 spawn 解析的环境变量规则处理）。
- `list` 的 `--project` 按 task 子串过滤（与上游一致）。
- `prune --idle` 的时长解析用 `time.ParseDuration`（上游支持
  `7d` 写法，Go 的 `ParseDuration` 不支持 `d`，须确认是否补
  `d` 解析）。
- spawn 真实拉起 claude/codex 尚未验证（本机无这两个二进制，
  模拟 supervisor 脚本只验了派生与 kill 闭环），待真实环境复验。
- 阶段 3/4 遗留与本组无关：`version` 字段与 `.sunshine/.version`
  含义重复、ts-sdk-author 是否改为 Go SDK 编写 skill。

### 协调人合并记录（2026-11-09，已完成）

五组全部完成并通过最终全量验证：`go build`、`go vet`、`gofmt`、`go test ./...`（17 个包、525 个测试，含子测试）全过；channel 相关 5 个包（channel、adapters、store、supervisor、cli）连续 3 次全过，无抖动。

#### 合并时做的修正（4 项）

- **TODO 清理**：`internal/cli/channel_manage.go` 里 `__supervisor` / `__parse-trace` 入口的两处 TODO 注释（“D 组提交后改指”）已过时——接线实际已在 `internal/cli/channel_backend.go`（`runSupervisorFunc` / `runParseTraceFunc`）完成；注释改为准确说明，代码未动。
- **`prune --idle 7d` 补天单位**：`parseDurationFlag` 只认 Go 的 `time.ParseDuration`（无 `d` 单位）与纯数字（秒），但 prune 帮助与示例写的是 `7d`，实测会报错。已补 `Nd` 解析（`7d` = 168h），并加 `TestParseDurationFlagDays` 锁定（7d/1d/30s/2m/纯数字/空串/非法）。
- **sunshine-meta 文档去“规划中”标注**：`sunshine-meta/SKILL.md` 与 `references/local-architecture/multi-agent-channel.md` 仍写 channel 为规划中功能；现已实现，改为“已实现，可直接使用”。（mem 的标注阶段 5 已改过；channel 是最后一处。）
- **A 组报告的并行期构建失败已收敛**：E 组 forum.go 曾引用未定义的 `Store`、同名符号重复声明——E 组落地 `ForumStore` 最小接口 + `forum_store.go` 适配器后解决；B 组 `types.go` 中途消失又重建一事已随各组落地收敛，最终全树可编译。

#### 合并决策（有意保留，不再改）

- **两份 thread 投影并存**：`store/thread.go` 的 `ReduceThreads`（store.ThreadState，字段 Thread/OpenedAt）与 `internal/channel/threadstate.go` 的 `ReduceThreads`（channel.ThreadState，字段 ID/CreatedAt/Processed）语义一致、测试各自锁定；channel 版多 `Processed` 字段与 ID 命名，合一须对齐 ThreadState 形态并改 forum/CLI 调用点，回归风险大于收益，暂保留，另立任务再合一。同理 `ReduceChannelMetadata` 也有两份：store 完整版（create + context 增删 + title 设置/清除 + linkedContext）是唯一事实来源；`internal/channel/forum.go` 简化版（只读 create 事件）仅供 `ReadForumChannelEvents` 判类型用——类型只由 create 事件决定，两版判定一致。
- **`ChannelEvent` 两套模型并存是刻意分层**（B 组）：store 版是落盘结构，API 层是富结构，之间 JSON 往返转换；去掉往返须 forum/threadstate 改用 StrField/Extra 访问，另立任务。
- **store 层 Watch 仍用字节偏移起步**（无 SinceSeq 时以文件大小为偏移）：E 组建议改为 seq 游标起步；CLI 路径已有 B 组“返回前同步快照 lastSeq”的修复，实测 wait 匹配正常（见下），store 层偏移逻辑另立任务再评估。
- **`wait` 超时退出码**：上游 124，Go 版 cobra RunE 报错时实际为 1，文案注明上游退出码（C 组偏差 2，保留）。
- **channel 名只允许 `[A-Za-z0-9._-]+`**（中文名不可用）：与上游 `assertSafeName` 一致，CLI 已提前报错（C 组偏差 9，保留）。

#### 协调人独立实测（临时 SUNSHINE_CHANNEL_ROOT，/tmp/ch6root）

- create/send/list（含 --json）/messages 正常；事件落盘 `~/.sunshine/channels/<projectKey>/<channel>/events.jsonl`（本例 `-tmp-ch6proj`），项目目录内无任何写入。
- wait：`--kind done` 2s 超时正确报错；`--kind message` 在 worker1 发消息后约 1s 匹配到并打印事件 JSON 退出。注意：wait 默认排除自己发的事件（Self 过滤），须以另一方身份发消息才能唤醒——首次实测以 main 身份自发自等未匹配，属设计行为，非 bug。
- forum 全套：post opened/comment/status、forum 列表与 --status 过滤、thread 时间线、thread rename（须 --as；改名后旧名经别名解析仍可查，--raw 含 aliases:["t1"]）、chat 频道上 forum 操作报中文类型错误。
- context add/list、title set/clear 正常；prune 默认 dry-run、--yes 真删；rm 删除 channel。
- `sunshine channel --help` 列出 16 个子命令，无 trellis 字样；channel 代码内 trellis 残留仅 3 处兼容回退读旧值（TRELLIS_CHANNEL_WORKER_IDLE_TIMEOUT / TRELLIS_CHANNEL_MAX_LIVE_WORKERS / TRELLIS_CHANNEL_AS）与测试守卫本身。
- supervisor 闭环：D 组 StubWorkerRuntime + 假 claude 脚本端到端（create → spawn → send --to → wait done 两回合）已验证；codex 真实拉起：Start + 握手 + Send 成功，回合因 401 Unauthorized 失败（沙箱无登录信息与 API Key），待有认证的真实环境复验；claude 沙箱无二进制，待真实环境复验。

#### 遗留问题

- 真实拉起：claude 无二进制、codex 回合 401，均待有认证的真实环境复验（同 D 组汇报）。
- `version` 字段与 `.sunshine/.version` 含义重复、ts-sdk-author 是否改为 Go SDK 编写版（阶段 3/4 遗留，与本阶段无关）。
- 上述“合并决策”里三处另立任务：两份 thread 投影合一、ChannelEvent 去 JSON 往返、store 层 Watch 改 seq 游标起步。

## 阶段 7 实施说明（发布：goreleaser、upgrade 自更新、CI/Git hooks）

依据：需求清单第 1 模块（分发与工程底座）、第 13 模块（其他子系统：upgrade 相关）；源码对照 `packages/cli/src/commands/upgrade.ts`（148 行）、`packages/cli/src/commands/update.ts` 的 `getLatestNpmVersion`（最新版检查）。

### 范围

- 版本注入（ldflags）与 `sunshine --version`。
- goreleaser 发布配置（多平台产物 + checksums + changelog + GitHub Release）。
- `sunshine upgrade` 自更新命令（查最新版 → 下载 → 校验 → 替换 → 校验新版 → 失败回滚）。
- `LatestVersionFunc` 接线（阶段 4 遗留：update 的最新版检查与 upgrade 共用同一函数与同一仓库变量）。
- CI（GitHub Actions）与 pre-commit（gofmt + go vet）。

### 版本注入

- `internal/cli/root.go` 的 `const version = "0.1.0"` 改为 var（ldflags 可注入：`-X github.com/BondLam/Sunshine/internal/cli.version=<tag>`），默认值保持 `"0.1.0"`（开发构建行为不变）；goreleaser 发布时从 git tag 注入。`sunshine --version` 打印该值；`init` 写入 `.sunshine/.version` 的仍是该值。
- 仓库地址同样用 ldflags 变量注入（`github.com/BondLam/Sunshine/internal/cli.repo`）：升级与 update 的最新版检查共用它。仓库已定为 `github.com/BondLam/Sunshine`（2026-11-09），`go.mod` 的 module 名已同步改为 `github.com/BondLam/Sunshine`，全部 import 路径与 ldflags 包路径已随之改掉。

### goreleaser 配置（`.goreleaser.yaml`）

- builds：CGO_ENABLED=0（纯 Go，modernc.org/sqlite 与 go-git 均无 cgo 要求）；goos: linux / darwin / windows；goarch: amd64 / arm64（windows 只要 amd64）；ldflags 注入 version 与 repo；binary 名 `sunshine`。
- archives：tar.gz（windows 用 zip）；checksums（sha256，checksums.txt）；changelog（按 git log 自动生成，分组常规提交）。
- release：GitHub（draft: false，prerelease 按 tag 含 -beta/-rc 自动判定）。
- 本地验证：`goreleaser build --snapshot --clean`（需安装：`go install github.com/goreleaser/goreleaser/v2@latest`；沙箱如装不上，记为待验证并说明原因）。

### `sunshine upgrade` 命令

对照 `upgrade.ts` 的行为，Go 版把 npm 换成 GitHub Releases 二进制自更新：

- flags：`--tag`（目标版本）、`--dry-run`（只打印计划，不写盘）。
- tag 解析（对应 `resolveUpgradeTag`）：显式 `--tag` 须过校验（同 NPM_TAG_RE 规则：`^[A-Za-z0-9][A-Za-z0-9._-]*$`，即简单 dist-tag 或版本号，如 latest/beta/rc/0.6.0-beta.8）；不传时按当前版本推断：含 `-beta` → beta，含 `-rc` → rc，否则 latest。
- 流程：查最新版（GitHub Releases API，仓库变量；未配置或离线时报错说明，不静默跳过）→ 已是最新则提示退出 → 按当前 GOOS/GOARCH 选资产 → 下载 → 按 checksums.txt 校验 sha256 → 备份当前二进制（同目录 `.bak`，权限保留）→ 替换 → 运行新版 `--version` 校验 → 失败回滚备份并报错。
- 报错附 troubleshooting 文案（对应 `troubleshooting()`）：手动下载地址、检查 `which sunshine`、权限说明（不 sudo）、锁文件/占用处理（不 --force）。
- Windows：运行中二进制不能直接覆盖，v1 先实现 Unix 路径（darwin/linux），Windows 替换走 `.new` + 交接脚本并标记待验证。
- `upgrade` 与 `update` 的区别写进帮助：`upgrade` 升级 CLI 自身，`update` 更新项目内模板。

### CI（`.github/workflows/ci.yml`）

- 触发：push 与 pull_request。
- steps：checkout、setup-go（1.27）、go mod 缓存、gofmt 检查（`gofmt -l .` 有输出即失败）、go vet、`go test ./...`、`go build ./...`；另跑一次 `goreleaser build --snapshot --clean`（或 `goreleaser check`）验证发布配置有效。
- 工作流文件须 YAML 解析通过（有 actionlint/yamllint 时加跑一次，没有则以解析通过为准并记录）。

### pre-commit（`.pre-commit-config.yaml`）

- hooks：gofmt（`gofmt -l -w` 或 gofmt 检查）与 go vet；用 pre-commit 框架的本地钩子或 golang 官方钩子，二选一并记录；README/PLAN 里写明 `pre-commit install` 启用方式（用户机器无 pre-commit 时不强制）。

### 发布流程文档

- 在 PLAN.md（或 README）写明：打 tag（如 `v0.2.0`，beta/rc 按 `-beta`/`-rc` 后缀）→ 推送触发 CI → goreleaser 发布 GitHub Release → 用户 `sunshine upgrade` 自更新；仓库建立后把占位 repo 换成真实地址（改 ldflags 默认值与 goreleaser 配置里的仓库引用）。

### 不属于阶段 7

- `version` 字段与 `.sunshine/.version` 重复、ts-sdk-author、update --migrate 端到端、SSH 克隆复验——均在“待一并处理”清单里，另行集中处理，不在本阶段改。
- 滚动轨道 19 个平台不在本阶段。

### 验收标准

- `go build`、`go vet`、`gofmt`、`go test ./...` 全过。
- `goreleaser build --snapshot --clean` 成功，多平台产物齐全（darwin/amd64 必须有——用户 Intel Mac；darwin/arm64、linux/amd64、linux/arm64、windows/amd64）；`sunshine --version` 在快照产物上打印注入的版本。
- `upgrade --dry-run` 打印计划不写盘；upgrade 端到端用本地测试服务（httptest 模拟 GitHub Releases API + 资产下载）验证下载/校验/替换/回滚全流程；真实 GitHub 发布待仓库建立后。
- CI 与 pre-commit 配置文件 YAML 解析通过；发布流程文档写明仓库占位替换点。
- 生成文件抽查：中文、无 trellis 残留。

## 发布流程

> 本节是阶段 7 的发布流程文档。README 已建（2026-11-09，随 v0.2.0 一起，只有安装与快速上手），详细发布流程仍写在这里。

### 发版流程（四步）

1. **打 tag 并推送**：在主分支上打版本 tag，例如 `v0.2.0`；预发布用 `-beta` / `-rc` 后缀，例如 `v0.2.0-beta.1`、`v0.2.0-rc.1`。推送 tag 到 GitHub。
2. **CI 自动检查**：推送触发 `.github/workflows/ci.yml`——gofmt 格式、go vet、单元测试、构建全过后，再跑 `goreleaser build --snapshot --clean` 验证发布配置有效。CI 不通过不发版。
3. **goreleaser 发布 GitHub Release**：CI 通过后由 goreleaser 打包多平台二进制（darwin/amd64、darwin/arm64、linux/amd64、linux/arm64、windows/amd64），生成 sha256 校验文件（checksums.txt）与更新日志，发布到 GitHub Release。tag 含 `-beta` / `-rc` 后缀时自动标记为预发布（prerelease），不作为最新正式版。
4. **用户自更新**：用户在终端运行 `sunshine upgrade`，按当前系统与芯片架构从 GitHub Release 下载对应资产，校验 sha256 后替换当前二进制并校验新版，失败时回滚。`sunshine upgrade` 升级的是 CLI 自身；`sunshine update` 更新的是项目内的模板，两者不要混淆。

### 版本号规则

- 正式版：`v<主>.<次>.<修订>`，例如 `v0.2.0`。`sunshine --version` 与 `init` 写入 `.sunshine/.version` 的都是该值（去掉 `v` 前缀的纯版本号）。
- 预发布：正式版后加 `-beta.<n>` 或 `-rc.<n>`，例如 `v0.2.0-beta.1`。当前版本含 `-beta` 时 `sunshine upgrade` 默认跟 beta 通道，含 `-rc` 时跟 rc 通道，否则跟 latest 通道；也可用 `sunshine upgrade --tag <版本或通道>` 显式指定。
- 版本注入：发布时 goreleaser 用 ldflags 从 git tag 注入版本（`-X github.com/BondLam/Sunshine/internal/cli.version=<tag>`）；本地开发构建不注入时默认 `0.1.0`。

### 仓库地址（已定，2026-11-09）

仓库已定为 `github.com/BondLam/Sunshine`，以下三处已同步换成真实地址（首次推送前完成）：

| 位置 | 换后值 |
| --- | --- |
| `internal/cli/root.go` 的 `repo` 变量（ldflags 可注入，默认值） | `github.com/BondLam/Sunshine` |
| `.goreleaser.yaml` 里的仓库引用（`builds` 的 ldflags 与 `release.github` 的 owner/name） | `github.com/BondLam/Sunshine` |
| `go.mod` 的 module 名 | `github.com/BondLam/Sunshine`（全部 import 路径与 ldflags 包路径已随之改掉） |

`sunshine upgrade` 与 `update` 的最新版检查共用同一个仓库变量（`internal/update/update.go` 的 `LatestVersionFunc`），换一处即两处生效。旧占位值 `github.com/sunshine/sunshine` 仅保留在 `internal/upgrade/upgrade.go` 的 `PlaceholderRepo` 哨兵常量里（显式传入该值仍视为未配置）。

### 首次推送记录（2026-11-09）

- 339 个文件已推到 `https://github.com/BondLam/Sunshine.git` 的 main 分支，远端提交 `2c3d007`，本地已 `git fetch` + `git reset --hard origin/main` 对齐，工作树干净。
- 沙箱出站代理挡住了 git 协议与 SSH（22 与 443 端口均被重置），`git push` 无法用；改走 GitHub Git Data API（连接器文档见 `~/workspace/custom-connectors/custom.github/CONNECTOR.md`，脚本在该目录的 `bin/push-initial.py`）。
- 空仓库上 Git Data API 拒绝建 blob/tree（409 "Git Repository is empty"），先用 Contents API 推 `.gitignore` 建种子提交 `211458f`，再以它为父提交推完整提交（tree 与本地提交一致，均为 `4a039f79`）。
- 移动 main 引用时报 403：提交含 `.github/workflows/ci.yml`，token 除 Contents 读写外还需 Workflows 读写（GitHub 返回头 `x-accepted-github-permissions: contents=write; contents=write,workflows=write`）。用户在 token 上补了 Workflows 读写权限后重试成功；token 本身不用重建。

### v0.1.0 发布记录（2026-11-09）

- 新增 `.github/workflows/release.yml`（tag `v*` 触发，goreleaser 发布 GitHub Release，`permissions: contents: write`）；同时修正 `.goreleaser.yaml` 里 `release.github` 的 owner/name（之前仍是占位 `sunshine/sunshine`，已改为 `BondLam/Sunshine`）。
- 第一轮 tag v0.1.0 触发的 CI 与发布工作流全部失败、0 个 job：job ID 用了中文（`检查`、`发布配置检查`、`发布`），GitHub job ID 只接受 ASCII 字母/数字/横线/下划线且以字母或下划线开头，中文只能放在 display name（`name` 字段）里。已修正 job ID（`check`、`goreleaser-check`、`release`），删掉旧 tag 后在新提交上重打 v0.1.0。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.1.0，6 个资产（checksums.txt + darwin/linux amd64/arm64、windows amd64 共 5 个包），CI 与发布工作流均 success。实测验证：从 Release 下载的 darwin/amd64 包 checksum 通过，解出 Mach-O x86_64 二进制；本地构建的 sunshine（0.1.0）跑 `upgrade` 与 `upgrade --dry-run` 均对真实 Release 查到最新 0.1.0 并报“已是最新”，自更新检查链路已通。

### v0.2.0 发布记录（2026-11-09）

- 内容：“待一并处理”7 项批次（迁移清单 0.2.0、SSH 克隆验证、config version 字段去重、go-sdk-author、ExtractDialogue 切片统一、search 预加载缓存）+ 新增 `install.sh` 一键安装脚本与 `README.md`。
- install.sh（仓库根目录）：macOS / Linux 一行安装，`curl -fsSL https://raw.githubusercontent.com/BondLam/Sunshine/main/install.sh | sh`，自动识别 OS/架构、默认装最新版（GitHub API 查 latest）、sha256 校验、落盘 `/usr/local/bin`（不可写时退到 `~/.local/bin`）；支持 `sh -s -- v0.2.0` / `SUNSHINE_VERSION` 指定版本、`SUNSHINE_INSTALL_DIR` 指定目录。写完后用真实 v0.1.0 Release 验了三路（指定版本、latest 解析、管道形式）全过；发布后又用真实一键命令对 v0.2.0 复验通过。写脚本时踩到一个坑：`SUNSHINE_INSTALL_DIR` 指定的目录不存在时 `[ ! -w "$DIR" ]` 为真，会误退回 `~/.local/bin`——已改成“显式指定目录时建不出/不可写直接报错”，并用 `install -m 755` 代替 `mv`（沙箱里 mv 跨文件系统报无法保留属主的警告）。
- 用户问“发版的 releases 都是打包文件，利于后续安装吗”——答：包本身是 Go CLI 标准做法，也是 `sunshine upgrade` 用的格式，但首次安装要手动下载解压放 PATH；这次先加 install.sh，Homebrew 以后再说（要多建 homebrew-tap 仓库 + 存一个能写它的密钥，setup 更重）。
- 推送：批次提交 `c431dde`（28 文件）与 install.sh 提交 `06be4d5` 均走 GitHub API（`push-initial.py`），本地已对齐，工作树干净；tag v0.2.0 为注释 tag（对象 `a6e618b`），同样经 Git Tags API 创建。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.2.0，6 个资产（checksums.txt + 5 个平台包）。CI（2 次）与发布工作流均 success。实测验证：用真实一键命令装上 v0.2.0，`--version` 打印“Sunshine 版本 0.2.0”；0.1.0 二进制跑 `upgrade --dry-run` 正确查到 0.2.0 并给出升级计划；linux 与 darwin 包 checksum 均 OK，darwin/amd64 解出 Mach-O x86_64 二进制。

### 本地验证命令（发版前）

```sh
export PATH=$HOME/go/bin:$PATH GOPATH=$HOME/.gopath
gofmt -l .                                  # 应无输出
go vet ./...
go test ./...
go build ./...
goreleaser build --snapshot --clean         # 需先安装 goreleaser；验证多平台产物
./dist/sunshine-darwin-amd64/sunshine --version   # 快照产物应打印注入的版本
```

### CI 与 pre-commit

- CI：`.github/workflows/ci.yml`，push 与 pull_request 均触发。两个任务：`检查`（gofmt、go vet、`go test ./...`、`go build ./...`）与 `发布配置检查`（`goreleaser build --snapshot --clean`，依赖 `检查` 通过）。
- pre-commit：`.pre-commit-config.yaml`，钩子为 gofmt（自动格式化）与 go vet。启用方式（可选，用户机器无 pre-commit 时不强制）：

```sh
pip install pre-commit   # 或 brew install pre-commit
pre-commit install       # 在仓库根目录执行一次
```

本仓库目前不是 git 仓库时，`pre-commit install` 无法执行；建仓并首次提交前补跑一次即可。

## 阶段 7 实施记录（C 组：CI 与 pre-commit 配置及发布流程文档，2026-11-09）

- 新增 `.github/workflows/ci.yml`：push 与 pull_request 触发；`检查`任务依次跑 gofmt 检查、go vet、`go test ./...`、`go build ./...`（Go 1.27，setup-go 内置 go mod 缓存）；`发布配置检查`任务跑 `goreleaser build --snapshot --clean` 验证发布配置有效。
- 新增 `.pre-commit-config.yaml`：选型为 **pre-commit 框架的本地钩子**（repo: local），未用 golang 官方钩子仓库——本地钩子零外部依赖、离线可用，且检查内容与 CI 完全一致（gofmt 自动格式化 + go vet 全模块检查）。
- 发布流程文档：写在本文件末尾“发布流程”一节（仓库无 README，按任务要求直接写进 PLAN.md）。
- 与 A/B 组实际落地的对照（2026-11-09 核查，A/B 组在本组工作期间并行落地，已逐项核对一致）：`.goreleaser.yaml` 已落地（v2，CGO_ENABLED=0，linux/darwin/windows × amd64/arm64、windows 仅 amd64，ldflags 注入 version 与 repo，产物名模板 `sunshine_<版本>_<系统>_<架构>`，sha256 校验，prerelease 按 tag 自动判定）；`internal/cli/root.go` 的 `version` 已改为 var（默认值 `0.1.0`），`repo` 变量已建立（默认值 `github.com/sunshine/sunshine`，另有 `Version()` / `Repo()` 导出函数供共用）；`sunshine upgrade` 命令已落地（`internal/cli/upgrade.go`，--tag/--dry-run，`update.LatestVersionFunc` 已接线）。本文档引用的文件名、变量名与占位地址与实际落地一致。
- B 组并行落地遗留（已修复，验证期间 B 组自行处理完毕）：核查之初 `internal/upgrade/upgrade.go` 未按 gofmt 格式化，且 `go test ./...` 在 `internal/upgrade` 有 3 个失败——TestAssetNameAndSelect（测试里模拟的 Release 资产名 `sunshine-0.2.0-linux-amd64.tar.gz` 与 A 组 goreleaser 产物名模板 `sunshine_0.2.0_linux_amd64.tar.gz` 不一致）、TestRunSuccess（备份权限期望 0755，实际 0750）、TestRunWindowsHandover（交接输出缺“待验证”标注）；终验时 gofmt 已无输出，`internal/upgrade` 测试已全过。C 组未动 B 组文件。
- goreleaser 本地验证（2026-11-09）：goreleaser v2.20.0（`go install github.com/goreleaser/goreleaser/v2@latest`；本机 /tmp 仅 512M tmpfs，首次安装因磁盘满失败，改 `GOTMPDIR=$HOME/.gopath/gotmp` 后成功；CI 在 GitHub 运行不受此影响）；`goreleaser check` 通过（验证中途 A 组把第 44 行 `checksums:` 改为 v2 的 `checksum:`，此前 check 报错 `field checksums not found in type config.Project`）；`goreleaser build --snapshot --clean` 耗时 11 分 7 秒，4 个 Unix 目标成功（darwin/amd64 为 Mach-O x86_64、darwin/arm64、linux/amd64、linux/arm64），linux/amd64 快照产物打印 `Sunshine 版本 0.0.0-SNAPSHOT-none`（ldflags 版本注入生效）；**windows/amd64 失败**——三处 Unix 专属调用无 Windows 分支：`internal/channel/store/lock.go:128` 的 `syscall.Kill`、`internal/lifecycle/ablation.go:628` 的 `syscall.Kill`、`internal/task/hooks.go:93` 的 `SysProcAttr.Setsid`；因此未产出归档包与 checksums.txt。CI 的“发布配置检查”任务在修复前会失败，需相关代码补 Windows 分支（或临时调整发布配置），不在 C 组范围。验证产物 dist/（80M）已清理；仓库根目录目前无 .gitignore，建仓时建议把 dist/ 加进去。
- 遗留：本机工作目录目前不是 git 仓库，`pre-commit install` 的真实启用未实测（建仓后补跑）。

## 阶段 7 实施记录（总协调合并验证，2026-11-09）

- 分工：A 组版本注入与 goreleaser 配置；B 组 sunshine upgrade 命令与 LatestVersionFunc 接线；C 组 CI 与 pre-commit 配置及发布流程文档。三组并行落地后总协调做合并验证。
- **Windows 交叉编译修复（总协调，C 组发现的阻塞项）**：`GOOS=windows go build ./...` 失败不止 C 组报的三处，完整清单共 10 处（首次构建的 `head -20` 截断导致漏报）：`syscall.Kill` 出现在 `internal/channel/store/lock.go`、`internal/lifecycle/ablation.go`、`internal/channel/create.go`、`internal/channel/workers.go`、`internal/channel/supervisor/guard.go`、`internal/cli/channel_real_backend.go`（多处，含 SIGTERM→1.5s/8s→SIGKILL 的优雅停机逻辑）；`SysProcAttr.Setsid` 出现在 `internal/task/hooks.go`、`internal/channel/supervisor/spawn.go`；另 `internal/channel/adapters/runtime.go` 的 `Process.Signal(SIGTERM/SIGKILL)` 在 Windows 上编译通过但运行时不生效。修复方式：每包一对构建标签文件（`proc_unix.go` / `proc_windows.go`、`hookcmd_unix.go` / `hookcmd_windows.go`）——pid 存活判断（Unix：`kill(pid, 0)`；Windows：`OpenProcess` 试开句柄）、发信号（Unix：`syscall.Kill`；Windows：一律 TerminateProcess，并注明无 SIGTERM/SIGKILL 语义区分）、hook 命令（Unix：`sh -c` + Setsid；Windows：`cmd.exe /c`）、supervisor 脱离（Unix：Setsid；Windows：`CREATE_NEW_PROCESS_GROUP`）、adapter 停机（Unix：SIGTERM/SIGKILL；Windows：os.Interrupt 近似优雅 + Kill 强制）。另 `internal/channel/supervisor/supervisor_test.go` 的自杀式 SIGTERM 测试改用 `os.Interrupt`（与 SIGTERM 走同一条 NotifyContext 路径，且 Windows 可编译）。各包原有语义保持不变（如 store/lifecycle 的 pidAlive 把 EPERM 算存活，channel/cli 的算已死）。修复后 `GOOS=windows go build ./...`、`GOOS=windows go vet ./...`、`GOOS=windows go test -c` 全过；Unix 全量测试仍全过。**Windows 运行时行为未在真实 Windows 环境验证**（沙箱无 Windows）：信号语义退化（优雅停机变强制结束）与 cmd.exe 跑 hooks 已在代码注释里写明，建仓后需在真实 Windows 上复验。
- **goreleaser 全量验证（总协调）**：`goreleaser check` 通过；`goreleaser build --snapshot --clean` 5 个目标全部成功（darwin/amd64、darwin/arm64、linux/amd64、linux/arm64、windows/amd64），35 秒；`goreleaser release --snapshot --clean --skip=publish` 产出 5 个归档包（windows 为 zip）与 checksums.txt，`sha256sum -c` 全过；darwin/amd64 包内二进制为 Mach-O 64-bit x86_64（用户 Intel Mac），快照产物打印 `Sunshine 版本 0.0.0-SNAPSHOT-none`（ldflags 注入生效）。验证在 `$HOME/tmp/sunshine-goreleaser-test`（源码副本 + git init）里做，仓库本身仍不是 git 仓库。
- **版本注入核对**：`internal/cli/root.go` 的 `version` 已为 var（默认 `0.1.0`）、`repo` 默认 `github.com/sunshine/sunshine`，导出 `Version()` / `Repo()` 供 upgrade 与 update 共用；`scaffold.InitOptions` 新增 `Version` 字段，init 的 `.sunshine/.version` 与 `--version` 同源（ldflags 注入验证通过：注入 `0.9.9-test` 后 init 生成的 `.version` 同为该值）；`templates/sunshine/.version` 仍含字面 `0.1.0`，仅作 scaffold 不传 Version 时的 fallback。
- **upgrade 核对**：`sunshine upgrade`（--tag/--dry-run）已接入根命令；`RegisterUpgradeCommand` 里把 GitHub Releases 查询赋给 `update.LatestVersionFunc`；占位仓库下 `upgrade --dry-run` 报错说明（不静默跳过）并附中文 troubleshooting；`--tag "bad tag!"` 校验拒绝；httptest 端到端覆盖下载/校验/替换/回滚；`update --dry-run` 在未配置仓库时只警告不中断（实测 exit 0）。
- **遗留**：真实 GitHub 发布待仓库建立后验证（真实 Release、真实资产名；upgrade 资产名按 goreleaser 默认模板 `sunshine_<版本>_<系统>_<架构>.tar.gz`，另有后缀退化匹配兜底）；Windows 运行时行为待真实 Windows 环境验证；`pre-commit install` 未实测（工作目录目前不是 git 仓库，建仓后补跑）；仓库根目录无 `.gitignore`，建仓时建议把 `dist/` 加进去；两个历史注释仍有 trellis 字样（`internal/cli/init.go:266`、`internal/scaffold/scaffold.go:129`，均为上游语义对照注释，未动）。

## 待一并处理批次实施说明（2026-11-09 用户确认集中处理）

依据：GOAL.md“待一并处理”一节 7 项；其中第 2 项已完成（GitHub Releases 检查地址已接线并对真实 v0.1.0 Release 验证通过），本批次只做其余 6 项。全部中文；不复制 Trellis 源码，按需求清单重实现行为；每项做完须 `go build`、`go vet`、`gofmt`、`go test ./...` 全过，生成文件抽查中文与无 trellis 残留。

### 第 1 项：update --migrate 的 CLI 级端到端

- 现状：迁移引擎（`internal/migrate/`）与 `update --migrate` 开关（`internal/cli/update.go`）已就位，引擎已用真实清单做过单元测试；但内嵌清单集（`internal/migrate/manifests/`）为空，CLI 级全流程从未跑过。
- 做法：先落地第一份真实迁移清单——取材须有据：以 Trellis 模板历史上真实发生的一次改名/删除类变更为蓝本（到 `~/workspace/research/trellis-source` 里找一例，只取“改了什么”的事实，不抄代码；`renameTracesToJournal` 已定不移植，另选一例），写成 JSON 清单进内嵌清单集；再搭一个夹具项目（旧版模板文件 + 待迁移内容），跑 `update --migrate` 全流程（交互与非交互两路），验证改名/删除/迁移门禁/备份/回滚与 `.version` 更新。
- 验收：端到端跑通；清单内容有上游出处；新清单的加入方式写进 PLAN.md（之后每加一份清单照此办理）。

#### 第 1 项结论（2026-11-09，已完成）

- **第一份真实清单**：`internal/migrate/manifests/0.2.0.json`（6 条改名 + 4 条 safe-file-delete）。蓝本是 Trellis `packages/cli/src/migrations/manifests/0.5.0-beta.0.json` 的 Skill-first 架构迁移（该版含 68 条改名与 138 条 safe-file-delete，breaking 与 recommendMigrate 均为真；另核对过 0.1.9、0.2.0、0.2.14、0.3.4、0.4.0-beta.1、0.5.0-beta.5 六份清单才定下这一份）；只取“改了什么”的事实，未复制上游代码与原文；路径按 Sunshine 三平台（Claude Code、Cursor、Codex）与 `sunshine-` 前缀改写；出处写进清单的 `notes`。`renameTracesToJournal` 未用（已定不移植）。
- **回归测试**：`internal/migrate/migrate_test.go` 的 `TestLoadEmbeddedRegistryEmpty` 改为 `TestLoadEmbeddedRegistry020`（版本、条数、breaking/recommendMigrate、无 trellis 残留）；新增 `internal/update/migrate020_test.go`（门禁 / 试运行 / `--migrate --force` / 非交互 / 备份后执行五路，全过）。
- **二进制端到端**（0.1.0 二进制真实 `init` 建夹具：植入旧版文件并记哈希、两处本地改动、删掉改名目标的文件与哈希条目；0.2.0 二进制经 ldflags 跑 `update`，四路全过，断言脚本 39 项全绿）：门禁（无 `--migrate`）exit 1、硬停提示、零写盘；试运行 exit 0、只列计划、零写盘；非交互 `--migrate` exit 0（5 改名 + 1 跳过、清理 3 个退役文件、`.version` 0.1.0→0.2.0、备份含更新前内容、按备份恢复后与更新前夹具逐字节一致）；交互 PTY `--migrate` exit 0（6 改名，改动项默认“备份后执行”写 `.backup`、清理 3 个、`.version`→0.2.0）。
- **端到端发现并修复的缺陷**：非交互 `update --migrate` 遇已改动迁移项时，survey 读 stdin 得 EOF，`ExecuteMigrations` 把报错当跳过又记为错误，导致整个 update exit 1、后续步骤全没跑（`.version` 不更新）。修复在 `internal/update/migrate.go` 的 `migrationPrompterFor`：stdin 非终端时根本不调 survey，直接返回跳过（与 `surveyPrompter` 的非交互门禁一致；引擎层“报错即记错”的已钉住行为未动）。回归测试 `TestRunMigration020MigrateNonInteractive` 锁定。另记：PTY 驱动 survey 时，两个 `\x1b[6n` 查询的应答须逐个间隔发送（survey 的 `Location()` 每次新建 bufio.Reader，同时在队的第二个应答会被丢弃致死锁）。
- **适用边界（如实记录）**：plain `delete` 类型不在本清单里（上游仅 0.3.4 用过一次，删改名后留下的空目录；本清单的“删除”由 safe-file-delete 覆盖；`delete` 类型仍由阶段 4 单元测试覆盖）。safe-file-delete“无 `--migrate` 也执行”的语义在本夹具上被破坏性门禁挡住（无 `--migrate` 时直接硬停），CLI 级只由 `--dry-run` 的清理计划 + 阶段 4 单元测试覆盖，未单独跑一条无 `--migrate` 的删除实测。

#### 新增迁移清单的加入方式（之后每加一份照此办理）

完整六步见 `internal/migrate/manifests/README.md`“新增清单”一节（取材 / 文件命名 / 内容字段 / 禁区 / 验证 / 发布）。要点：只写 Sunshine 自己版本线上的真实结构变更，文件为 `manifests/<目标版本号>.json`，破坏性变更置 `breaking` 与 `recommendMigrate` 为 true，`safe-file-delete` 的 `allowed_hashes` 用旧版模板文件内容的原始字节 SHA-256，受保护路径不得进清单，改名源不得是当前模板集合里仍有的路径；验证分两层——`internal/update/` 的 CLI 级回归（门禁 / 试运行 / `--migrate` 执行 / `.version` 更新 / 备份含更新前内容）+ 真实二进制夹具跑交互与非交互两路（夹具做法：真实 `init` 建旧版项目，植入旧版文件并记哈希，删掉改名目标的文件与哈希条目，再以新版二进制跑 `update`）；只加文件、不改代码，清单随二进制内嵌发布。

### 第 3 项：SSH 形式 --registry 端到端克隆

- 现状：`init --registry` 支持 SSH 形式（go-git），但从未端到端验证；沙箱 22 端口被出口代理重置，无法直连外部 SSH。
- 做法：在沙箱内起本地 sshd（有 openssh-server 则直接用，没有则装；装不上就如实记录），建一个本地 git 仓库作 registry 源（marketplace 结构），配好 key 认证，跑 `sunshine init --registry ssh://git@localhost:<端口>/<路径>` 全流程，验证克隆、marketplace 解析与模板安装。
- 若沙箱确实起不了 sshd：只做到 go-git SSH 代码路径的单测覆盖，并如实记录阻塞点，该项记为“待真实环境复验”，不谎报通过。

#### 第 3 项结论（2026-11-09，已完成；sshd 真实跑通，不用记“待复验”）

- **环境**：沙箱起初无 openssh-server（`apt list` 无候选），`apt-get update` 跑完（约 12 分钟，代理很慢）后装上 `openssh-server 1:9.6p1-3ubuntu13.20`。本地 sshd 跑在 `127.0.0.1:2222`（配置 `/tmp/ssh-e2e/sshd_config`，pid 文件 `/tmp/ssh-e2e/sshd.pid`，仅监听回环；密钥认证、无口令；验证时仍在运行）。`git` 用户已建（`/home/git`，`usermod -p '*'` 解锁——新建账号默认锁定态，sshd 会报 `User git not allowed because account is locked`）；客户端钥匙 `/tmp/ssh-e2e/home/.ssh/id_ed25519`（无口令），公钥已入 `/home/git/.ssh/authorized_keys`。
- **Registry 源**：裸仓 `/git/registry.git`（git 用户所有），marketplace 结构（`marketplace/index.json` + `specs/demo-spec` + `skills/demo-skill` + `workflows/native`），默认分支 `main`。
- **验证**：`sunshine init -u 测试 -y --registry ssh://git@localhost:2222/git/registry/marketplace --template demo-spec` exit 0——go-git SSH 克隆成功、accept-new 回调把 `[localhost]:2222` 主机密钥记入 `known_hosts`、marketplace 解析出 3 条模板、demo-spec 2 文件装进 `.sunshine/spec`、registry.spec 写入 config.yaml、哈希清单就位、无 trellis 残留；改模板推 v2 后 `sunshine update --force` 把 `backend.md` 刷到 v2（registry 持续刷新走 SSH 也通）。git CLI `ls-remote` 同 URL 也通（交叉验证服务端）。
- **发现的解析局限（未修，报总协调人定）**：`ParseRegistrySource` 把 ssh:// 路径按 `user/repo[/subdir]` 切分，三段式绝对路径 `ssh://git@localhost:2222/home/git/registry.git` 被误解析为 Repo=`home/git`、Subdir=`registry`、GitURL=`ssh://git@localhost:2222/home/git.git`（临时程序实测输出）；两段式 `ssh://git@localhost:2222/git/registry[.git]` 与带 subdir 的 `.../git/registry/marketplace` 解析正确。结论：SSH registry 目前只支持两段式路径；要支持任意绝对路径须改解析器（另立任务，不在本项修）。
- **环境坑（夹具侧，已解决，非产品缺陷）**：`/git` 目录须对 git 用户可遍历（初建时 root 770，git 用户进不去）；root 推上去的引用文件会变成 root 所有，致使 git 用户读不到、upload-pack 通告零 OID、go-git 报 `malformed zero-id ref`——推完须 `chown -R git:git /git/registry.git`（或改以 git 用户推送）。
- **复验 / 停止**：重跑 `HOME=/tmp/ssh-e2e/home /tmp/sunshine-e2e/sunshine-0.1.0 init -u 测试 -y --registry ssh://git@localhost:2222/git/registry/marketplace --template demo-spec`（须先 `rm -rf` 目标项目目录；sshd 在跑的前提下）。停止：`kill $(cat /tmp/ssh-e2e/sshd.pid)`；重起：`/usr/sbin/sshd -f /tmp/ssh-e2e/sshd_config -E /tmp/ssh-e2e/sshd.log`（`/run/sshd` 须为 root 755）。

### 第 4 项：version 字段与 .sunshine/.version 重复（已查清，按此办）

- 查清的结论：`config.yaml` 的 `version` 字段（`internal/config/config.go` 的 `Version`、`templates/sunshine/config.yaml:153` 的 `version: 0.1.0`）没有任何代码读或维护它；版本比较与更新全走 `.sunshine/.version` 文件（`internal/update/version.go` 的 `GetInstalledVersion` / `WriteVersionFile`，init 与 update 共同维护）。两者不是“各有分工”，是字段冗余。
- 做法：去掉 `config.yaml` 的 `version` 字段（struct 字段 + 模板行 + 相关文档与注释），`.sunshine/.version` 保留为唯一版本标记。
- 动手前先全仓 grep 一遍确认确实无读者（若发现读者，停下来改方案：保留字段并在 PLAN.md 写明分工，不硬删）。
- 验收：删后全量测试过；旧项目 config.yaml 里残留的 `version` 行不报错（未知字段应被忽略或给提示，不得崩）。

#### 第 4 项结论（2026-11-09，已完成）

- **动手前 grep**：`internal/config/config.go` 的 `Version` 字段没有任何生产代码读者；命中的只有测试夹具里的 `version:` 行（`config_test.go`、`registry_test.go`、`hook_test.go`、`configyaml_test.go`，测的是行级保留/容忍逻辑）与 `.version` 文件本身的版本逻辑（`internal/update/version.go` 的 `GetInstalledVersion` / `WriteVersionFile`、CLI 的 `version` 变量）。`LoadBytes` 用非严格 `yaml.Unmarshal`，未知字段默认忽略——残留行不报错有实现依据，符合规格，无需改方案。
- **改动**：删 `internal/config/config.go` 的 `Version string \`yaml:"version"\`` 字段及注释；删 `templates/sunshine/config.yaml` 的 `version: 0.1.0` 行及上方注释行。`.sunshine/.version` 保留为唯一版本标记。
- **测试**：`internal/config/config_test.go` 的 `TestLoadFullFields` 夹具 `version: 0.2.0` 行保留（改为残留行容忍的证明），断言只验 `tasks_dir` / `spec_dir`；新增 `TestLoadIgnoresResidualVersionLine`（只含 `version: 9.9.9` 残留行的旧 config 应解析成功）。其余三处测试夹具的 `version:` 行有意保留（即残留行不报错的证据）。
- **验证**：`sunshine init` 后 `.sunshine/config.yaml` 无 version 行，`.sunshine/.version` 内容为 `0.1.0`；追加 `version: 0.1.0` 残留行后 `sunshine task list` 与 `sunshine update --dry-run` 均正常退出。`go build`、`go vet`、`gofmt`、`go test ./...` 全过（560 个测试，0 失败）。

### 第 5 项：ts-sdk-author 改为 Go SDK 编写版

- 现状：Trellis 的 `ts-sdk-author` skill（`~/workspace/research/trellis-source/.agents/skills/ts-sdk-author/`，教写生产级 TypeScript SDK：npm/pnpm/tsdown/双 ESM+CJS/changesets）在 Sunshine 里不存在；Sunshine 是 Go 项目，没有 TS SDK。
- 做法：新写 `go-sdk-author` skill，落到 `templates/marketplace/skills/go-sdk-author/SKILL.md`，并收入模板市场索引（mem-recall 的索引位置见 `internal/registry/marketplace.md` 与 `templates/marketplace/skills/`，照同格式加第 11 条）。
- 内容要求：参照 ts-sdk-author 的章节结构（工程布局、公共 API 与模块边界、插件扩展点、版本与发布生命周期）重写 Go 版；TS 专属内容（tsdown、pnpm、changesets、双 ESM+CJS）不逐字翻译，换成 Go 对应物（Go module 布局、公开 API 设计、semver tag、goreleaser 发布、GoReleaser + GitHub Release 流程）；不复制 Trellis 技能原文。
- 验收：skill 可读、Go 内容准确、索引条目就位；`sunshine init --registry` 能装上它（走一次本地 registry 安装验证）。

#### 第 5 项结论（2026-11-09，已完成）

- **新增**：`templates/marketplace/skills/go-sdk-author/SKILL.md`（522 行，全中文，frontmatter 的 `name` / `description` 格式与 `mem-recall` 一致）。参照 `ts-sdk-author` 的章节结构重写，未复制原文：何时使用 → 七阶段工作流（1. Go module 与工程布局；2. 公开 API 与模块边界——函数式选项、不透明 ID、`%w` 错误链；3. 构建配置——`go build/vet/test -race`、CGO_ENABLED=0 静态交叉编译、ldflags 注入版本；4. 分发形态——module 路径、v2 的 `/v2` 后缀、`go install`、伪版本；5. 多包流水线——GitHub Actions 矩阵、`go.work`；6. 验证——冒烟安装；7. 发布生命周期——semver tag、GoReleaser + GitHub Release、预发布隔离、`retract` 撤回）→ 速查表 → 发布前检查清单 → 常见错误表；末尾附 Sunshine 自身对照。
- **TS 专属内容已全部替换**：tsdown、pnpm、changesets、双 ESM+CJS 在新 skill 里零出现（grep 验证），对应物为 Go module 布局、公开 API 设计、semver tag、GoReleaser 发布、GoReleaser + GitHub Release 流程。
- **索引（第 11 条）**：`internal/registry/marketplace.md`（“已确认的 10 个条目”→“11 个条目”，skill 由 4 个→5 个）与 `internal/registry/source_test.go`（`TestParseTemplateIndex` 夹具 10→11 条，新增 `{"id": "go-sdk-author", "type": "skill", "name": "Go SDK Author", "path": "skills/go-sdk-author"}`，计数断言改为 11，`FindTemplate` 检查加入 `go-sdk-author`）两处同步。
- **验证**：本地 registry 安装走通（`/tmp/b5market`：11 条 `index.json` + `skills/go-sdk-author/SKILL.md`），`sunshine init --registry /tmp/b5market --template go-sdk-author` 退出 0，`.agents/skills/go-sdk-author/SKILL.md` 已装上，`diff` 与模板字节一致。`go build`、`go vet`、`gofmt`、`go test ./...` 全过（560 个测试，0 失败）。

### 第 6 项：ExtractDialogue 内部 phase 切片统一

- 现状：`internal/mem/context.go` 的 `ExtractDialogue` 内部自己按 `filter.Phase` 切片；生产路径（CLI 的 extract）走的是适配器 `Collect` 的事件 + `SliceMemPhase`（`internal/mem/phase.go`），两路切片语义未统一。
- 做法：把 `ExtractDialogue` 改成走事件通道（与 CLI 路径同一套 `SliceMemPhase` 语义），删掉内部自有的切片分支；按原备注“统一时先改相关测试的预期”，先改测试预期再改实现。
- 验收：`extract --phase` 两种入口（直接调与 CLI）结果一致；全量测试过。

#### 第 6 项结论（2026-11-09，已完成）

- **改法**：`MemFilter` 新增 `Events []MemEvent`（事件通道：适配器 `Collect` 从工具调用里抽出的事件）与 `PlatformKind MemSourceKind`（只有 OpenCode 不支持切片，降级返回全文）；`ExtractDialogue` 删掉内部自有的 `sliceByPhase`（全文删除），改为 `SliceMemPhase(cleaned, filter.Events, phase, filter.PlatformKind)` 再按 grep 过滤；非法 phase 按 all 处理（与统一前一致）。`FindTaskCommands` 本身保留（`phase_test.go` 仍直接测它），只是不再是切片的事件来源。
- **测试先行**：先改 `internal/mem/context_test.go` 的预期再改实现——有事件时 brainstorm 取窗口内 2 轮、implement 取窗口外 4 轮；无事件时文本里的任务命令不再算边界（`TestExtractDialogue_TextCommandsIgnored`：brainstorm 返回全文、implement 为空，旧实现下此测试会失败）；新增 `TestExtractDialogue_MatchesSliceMemPhase`（直接调与 `SliceMemPhase` 三阶段逐轮一致）、`TestExtractDialogue_OpencodeDowngrade`、`TestExtractDialogue_InvalidPhase`。
- **CLI 接线**：`internal/cli/mem.go` 的 `extractGroups` 切片改走 `ExtractDialogue` 的事件通道（`sliceFilter.Events / PlatformKind`），窗口与警告仍由同一批输入的 `SliceMemPhase` 给出，供分组与展示；CLI 的 `TestMemExtractPhase`（工具调用藏命令的 Claude 夹具）仍全过，两种入口一致。
- **验证**：`go build`、`go vet`、`gofmt`、`go test ./...` 全过（560 个测试，0 失败）。

### 第 7 项：search 的 SQLite 预加载缓存（先测再定）

- 现状：search 每次打开 SQLite 再查，未做预加载缓存；原备注“v1 可接受，search 性能成问题再补”。
- 做法：先测：造一份有规模的会话库（量级与真实用户使用相当，至少数千条会话/数万条消息），测 `sunshine mem search` 的耗时（冷查与热查）；达标线：单次 scoped search p95 < 500ms。
- 达标则该项记为“已实测，不需要补”，把测试数据与结论写进 PLAN.md；不达标再实现预加载缓存（缓存须有失效策略，不得读脏数据），实现后复测达标。
- 验收：有实测数字与结论；若实现缓存，全量测试过且只读哈希不变（mem 的 SQLite 只读约定不得破）。

#### 第 7 项结论（2026-11-09，已完成——实测不达标，已实现预加载缓存并复测达标）

- **实测不达标（改造前）**：性能库为 OpenCode 单库 3000 会话 / 30000 消息 / 30000 part（9.4MB，`/tmp/sunshine-perf-home`，由 `SUNSHINE_PERF_GEN=1 go test ./internal/mem/adapters/ -run TestSearchPerfGenerate` 生成，复现方法见 `internal/mem/adapters/search_perf_test.go` 文件头），scope 为项目目录（默认 cwd scope）。进程内 `ListSessions + SearchMatches`：命中关键词“缓存检索”（30 个命中会话）三轮 31.86s / 31.69s / 31.82s（p95 31.86s），无命中关键词 32.65s；达标线 500ms，超标约 64 倍，必须补缓存。
- **根因（分段诊断，见已删的临时诊断测试，数字留此）**：开库仅 117µs、`checkMsgPartSchema` 仅 50µs、列会话仅 15ms；瓶颈是单会话 `loadMsgPartStore` 均值 9.3ms——按会话过滤的查询无索引，每会话都要全表扫描，×3000 ≈ 28s；全库 bulk load 一次仅约 275ms；纯内存匹配约 437ms（其中 `CleanTurns` 的重复正则清洗占约 60% CPU）。
- **实现——预加载缓存**（`internal/mem/adapters/preload.go`，search 入口显式调用，`internal/cli/mem.go` 的 `runMemSearch` 在列出会话后、检索前调 `adapters.PreloadSearchStores`）：
  - 覆盖三家 SQLite 共享库：OpenCode / ZCode 的 message / part（共用 `msgPartStore`）、Devin 的 message_nodes + 各会话 main_chain_id；JSONL 平台（Claude / Codex / Grok / Pi）单会话单文件，无共享库可预加载，不走本缓存。
  - 只预加载平台过滤范围内的库（`--platform` 只指定某一家时其余家不读）。
  - 失效策略：缓存只活在单个 CLI 进程内，不落盘，进程结束即失效；每次取用前按（库文件 + `-wal` 文件）的 mtime 与 size 指纹校验，不一致就丢弃、当次回退逐会话读；预加载时加载前后各取一次指纹，前后不一致（加载中有人写库）就不入缓存；预加载失败只记警告，search 回退逐会话读，不中断。
  - 只读约定：预加载只读打开（`mode=ro`），不写会话库；实测前后库文件 SHA-256 不变（`abe4feeff9d428c8bbc951f6ee4ea24f967fada954487024b83dd40f5982a95d`）。
  - bulk load 用 lean 扫描（`loadMsgPartStoreBulk`：直接按列扫描、message/part 两表并行、part data 强类型 `partData` 解析，struct 解析失败时回退 map 解析再转换；与通用全库路径逐行等价，`TestLoadMsgPartStoreBulk_EqualsGeneric` 与畸形数据等价测试 `TestLoadMsgPartStoreBulk_MalformedEqualsGeneric` 锁定）：bulk load 约 275ms → 约 168ms。
- **实现——匹配快路径（达标必需的配套改动，语义不变）**：`StripInjectionTags`（`internal/mem/dialogue.go` 与 `internal/mem/adapters/dialogue.go` 两份实现）加触发子串快路径（无 `<`、无 `# AGENTS.md instructions for`、无 `\n\n\n` 时只裁剪首尾空白，慢速路径每步都是恒等变换，结果完全一致），并把每次现编译的 `\n{3,}` 正则提到包级只编译一次；`CleanTurns` 的 `len([]rune(...))` 改为 `utf8.RuneCountInString`。纯内存匹配 352ms → 76ms（benchmark `BenchmarkSearchMatchesMemory`）；预加载命中后的整条 search 路径 453ms → 约 121ms（benchmark `BenchmarkSearchWithPreload`）。
- **复测达标（改造后，CLI 二进制全新进程、`--json`、各 31 次，页缓存热状态）**：
  - 命中关键词“缓存检索”：min 284ms、p50 327ms、**p95 375ms**、max 392ms、均值 330ms；
  - 无命中关键词：min 285ms、p50 334ms、**p95 380ms**、max 442ms、均值 340ms；
  - 进程内：预加载 181ms，缓存命中后单轮检索 p50 117ms / p95 122ms。
  - 均低于 500ms 达标线。冷查未能在沙箱隔离：`/proc` 只读、清不了页缓存，上述 62 次均为页缓存热状态；冷查只多一次 9.4MB 磁盘读，量级上不影响结论（未给冷查数字，不谎报）。
- **测试**：新增 `internal/mem/adapters/preload_test.go`（预加载命中与逐会话读结果一致、改写库后指纹失效、ZCode 事件一致、Devin 一致、空 HOME 无警告、平台过滤）与 `internal/mem/adapters/search_perf_test.go`（造库 / 计时 / 两个 benchmark，默认跳过，`SUNSHINE_PERF_GEN=1`、`SUNSHINE_PERF_SEARCH=1` 才跑，不拖慢全量测试）；`go build`、`go vet`、`gofmt`、`go test ./...` 全过（560 个测试，0 失败），adapters 包 `-race` 通过。
- **未做**：未做跨进程持久化缓存（进程内预加载已达标，持久化缓存要另做落盘失效与并发控制，收益已不大）；Devin 预加载未单独做大规模实测（与 OpenCode/ZCode 同一模式，夹具级测试覆盖）。

### 批次验收（总）

- 6 项逐项有结论（完成 / 已实测不需要 / 待真实环境复验并写明阻塞点），不把“未做”写成“已做”。
- `go build`、`go vet`、`gofmt`、`go test ./...` 全过；生成文件抽查中文与无 trellis 残留。
- PLAN.md 与 GOAL.md 的“待一并处理”一节同步更新：逐项写清结论与落点。

#### 总协调终验（2026-11-09）

- `gofmt -l .` 无输出；`go vet ./...` 通过；`go build ./...` 通过；`go test ./... -count=1` 18 个包全过，560 个测试，0 失败。
- 独立复验：SSH 端到端重跑一次（`init --registry ssh://git@localhost:2222/git/registry/marketplace --template demo-spec`，demo-spec 2 文件装进 `.sunshine/spec`）；迁移五路测试全过；ExtractDialogue 十路测试全过；`mem search` 在 3000 会话 / 30000 消息性能库上三轮 325/291/329ms（30 个命中），库文件 SHA-256 与实测前一致（`abe4feef…2a95d`）。
- 新增 trellis 字样核查：改动代码与新增文件里无新输出的 trellis 字样；仅有的 1 处是 `0.2.0.json` 的 `notes` 字段（上游出处说明，CLI 不打印，`From`/`To` 路径有测试 guard 锁死不得残留）与 PLAN.md 文档里的出处引用。
- **后续任务（另立，不在本批次）**：SSH registry 的 `ParseRegistrySource` 只支持两段式路径，三段式绝对路径（如 `ssh://git@localhost:2222/home/git/registry.git`）会被误解析（见第 3 项结论）；要支持任意绝对路径须改解析器。本批次已随 v0.2.0 提交推送并发布（见“v0.2.0 发布记录”）。

## SSH 三段式路径解析 + README 补齐与中英文教程（2026-11-09，用户确认做）

### 实施说明

#### A. SSH 解析：支持任意段数的绝对路径

- **根因**（`internal/registry/parse.go` 的 `ParseRegistrySource`）：解析完把路径按 `/` 切段后写死 `repo = segments[0] + "/" + segments[1]`、其余全当 subdir。这是 GitHub 式 `owner/repo` 的假设；自建服务器上仓库放在绝对路径（如 `/home/git/registry.git`）时就错：`ssh://git@localhost:2222/home/git/registry.git` 被误解析为 Repo=`home/git`、Subdir=`registry`、GitURL=`ssh://git@localhost:2222/home/git.git`（批次 7 第 3 项结论里临时程序实测输出）。两段式与带 subdir 的两段式（`.../git/registry/marketplace`）目前解析正确，修的时候不能改坏这两路。
- **定下的修法（显式 `.git` 规则）**：路径以 `.git` 结尾（`#ref` 在 `#` 之前判断）时，整个路径就是仓库路径，subdir 为空；不带 `.git` 时保持现有两段式规则（前两段是 repo、其余是 subdir）。理由：`.git` 后缀是用户明确说“这就是仓库”的信号，无歧义；不带后缀的 `.../org/repo/marketplace` 无法区分“repo 是 org/repo、subdir 是 marketplace”还是“repo 就是 org/repo/marketplace”，只能沿用现有约定。
- **适用范围**：三路输入都要覆盖——`ssh://git@host[:port]/路径`、`git@host:路径`（scp 式）、自建 HTTPS `https://host[:port]/路径`（`httpsUnknownRe` 分支）。giget 前缀（`gh:` 等）与公开域名（github/gitlab/bitbucket）不受影响，仍按两段式走。
- **注意第二处同款实现**：`internal/workflow/registry.go:126` 的 `ParseRegistrySource`（`workflow -m/--marketplace` 用）第 218 行也是同样的 `segments[0] + "/" + segments[1]`。两处行为要一致：优先让 workflow 版复用 registry 版（若结构允许），否则同规则各修一处；workflow 现有测试不能坏。
- **测试**：`source_test.go`（与 workflow 侧对应测试）加表驱动用例——两段式现状不变（`.../git/registry`、带 subdir 的 `.../git/registry/marketplace`）；三段式带 `.git`（ssh-url / scp / https 三路，含带端口）：Repo 为完整三段路径、Subdir 为空、GitURL 拼回原路径；四段式带 `.git`；三段式不带 `.git` 保持现状（Repo 前两段、余下为 subdir）；带 `#ref` 时 `.git#ref` 也要判对。
- **端到端**：复用批次 7 的本地 sshd（配置 `/tmp/ssh-e2e/sshd_config`，重起：`/usr/sbin/sshd -f /tmp/ssh-e2e/sshd_config -E /tmp/ssh-e2e/sshd.log`，`/run/sshd` 须为 root 755；停：`kill $(cat /tmp/ssh-e2e/sshd.pid)`；客户端钥匙 `/tmp/ssh-e2e/home/.ssh/id_ed25519`）。新建一个三段式绝对路径的裸仓（如 `/home/git/registry3.git`，marketplace 结构），跑 `sunshine init --registry ssh://git@localhost:2222/home/git/registry3.git --template demo-spec` 全流程（克隆、marketplace 解析、模板安装、registry.spec、哈希清单）。同时回归两段式老路（`.../git/registry/marketplace`，批次 7 已验过的命令重跑一次）。sshd 验后停掉，配置与密钥保留。

#### B. README 补齐与中英文教程：布局已定

- **README.md（中文，主）**：保持“门口”定位，补齐但不写成手册——安装（已有）、快速上手、核心命令一览（init / task / workflow / update / registry / mem / channel / upgrade，各一行说明 + 一条最常用示例）、配置要点（config.yaml 关键字段）、模板市场（`--registry` 一句话 + 链接）、链接（教程、PLAN.md、Releases）。不塞长篇教程。
- **README.en.md（英文）**：README.md 的完整英文版，同结构；CLI 本身是中文，英文文档里的命令与输出示例保持中文原文，加一句说明（CLI 输出为中文）。
- **教程单独成文件，不进 README**：`docs/tutorial.md`（中文）与 `docs/tutorial.en.md`（英文）。内容一条真实主线走到底：init 建项目 → 写 spec（三份规范）→ task 拆任务 → 执行与 workflow 切换 → update 与 registry（模板市场）→ mem 回顾会话 → channel 多 Agent 协同 → upgrade 升级。每一步的命令都要实跑验证（`sunshine --help` 各子命令、临时目录 init 全流程），不编造 flag 与输出；中文教程里的输出示例用真实中文输出，英文教程同命令、输出保留中文原文并加英文解释。
- **互链**：两份 README 都链向两份教程（中文 README 主链中文教程、附英文版链接；英文 README 反之）。docs 目录下只放这两份教程，不另起别的文档。
- **不推送**：本批次改动先不提交推送，等用户定夺（是否随下个版本发）。

### 实施记录（2026-11-09）

- **A 路（SSH 解析）**：`internal/registry/parse.go` 与 `internal/workflow/registry.go` 各加 `hasExplicitGitSuffix()`（先切 `#ref`、再去末尾 `/`、判 `.git` 后缀），`dotGit && host != ""` 时整个路径当仓库、subdir 为空；giget 前缀与公开域名仍两段式。workflow 未复用 registry 版：返回结构体（`*Source` vs 值类型 `RegistrySource`）、错误类型与文案不同，复用要加映射层有改坏已断言行为的风险；规则约 10 行，两处用同一用例表保证一致。测试各加 `TestParseRegistrySourceDotGitPaths`（14 用例）：两段式现状不变、三段式带 `.git`（ssh-url/scp/https 三路，含端口）、四段式带 `.git`、三段式不带 `.git` 保持现状、`.git#ref` 三路判对、末尾 `/`、公开域名仍两段式。A 路环境说明：ssh-url 来源的 RawBaseURL 不带端口是既有行为（`host` 只存主机名，端口另存 `sshPort`），改的是测试期望不是代码；git CLI 交叉验证时主机密钥校验失败过一次（服务端优先 ed25519、旧 known_hosts 只有 rsa 记录），在 `/tmp/ssh-e2e/home/.ssh/known_hosts` 追加了 ed25519 记录（现 3 行），go-git 走 accept-new 回调不受影响。
- **B 路（文档）**：README.md 补齐到 84 行（快速上手、核心命令一览 8 命令、配置要点、模板市场、链接）；README.en.md 新建 86 行；docs/tutorial.md 740 行、docs/tutorial.en.md 781 行，主线 init→spec→task→workflow→update/registry→mem→channel→upgrade；两份 README 都互链两份教程，锚点与第 5 步实际标题对得上。教程命令逐条实跑、输出为真实摘录；未能实跑的部分如实写清未编造：`channel spawn` 真实拉起——claude 二进制不在 `$PATH`（记 `error` 事件）、codex 未登录报 401；mem 无真实会话，先给真实用法、再用手工夹具演示并注明；`update`/`upgrade` 只用 `--dry-run`。
- **独立复验（主代理）**：`gofmt -l .` 无输出、`go vet` 通过、`go test ./... -count=1` 562 个 PASS、0 FAIL（修前 560）。复验时遇到 `/tmp`（512M tmpfs）满了导致 4 个包链接失败（`no space left on device`），清掉旧验证残留目录（约 400M，多为 v0.1.0/v0.2.0 时期的 /tmp 临时目录）后全过——与代码无关。SSH 端到端用当前树新构建的二进制重跑：三段式 `ssh://git@localhost:2222/home/git/registry3.git --template demo-spec` exit 0（2 文件装好，registry.spec.source 为完整三段式 URL）；两段式回归 `.../git/registry/marketplace` 同样通过；生成文件无 trellis 残留；sshd 验后已停。文档抽查：干净目录 init 正好 115 个文件（与教程一致）、`task create` 不带 `--description` 报“错误：--description 必填”、带上后成功且 task.json 24 字段顺序与清单一致、`workflow --list` 与 `channel create --type chat` 用法与 README 一致、config.yaml 里 README 列的字段与默认值逐项对得上。复验发现一个小问题已直接修掉：README 核心命令表里 `registry` 一行写在“命令”表里，但没有 `sunshine registry` 这个子命令——中英文两份都已改成 `` `init --registry` ``。
- **本批次已随 v0.2.1 提交推送并发布**（见下“v0.2.1 发布记录”）。

- 内容：SSH 三段式路径解析修复 + README 补齐与中英文教程（上一节的批次）。提交 `e70d56b`（351 个文件），tag v0.2.1 为注释 tag（对象 `59d13c8`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.2.1，6 个资产。实测验证：真实一键命令装上 v0.2.1（`--version` 打印 0.2.1）；0.2.0 二进制 `upgrade --dry-run` 正确查到 0.2.1；darwin 包 checksum OK，解出 Mach-O x86_64 二进制。
- **CI 抖动（本次发现，未随发版修）**：打 tag 触发的 CI 与发布均 success；但推 main 提交触发的那次 CI 里 `TestSupervisorClosedLoop` 失败了一次（同一次提交、同一套代码，另一路 CI 全过）。日志：`supervisor_test.go:149: 应有 turn_started 与 turn_finished(done)：[create spawned message turn_started message done]`——`turn_finished` 在读事件时还没落盘。根因：`supervisorSink.Emit`（`internal/channel/supervisor/run.go`）先落 `done`、再 `tracker.Finish()` 后落 `turn_finished`，两者之间有窗口；测试只等 `done` 就读事件，赶上调度间隙就会误判。这是测试的时序假设问题，不是生产行为问题（事件最终都会落盘；该测试本地 20 次全过，同一提交另一路 CI 也全过）。修复方向（测试侧）：等 `done` 后再等 `turn_finished` 落盘再断言；不要改生产侧的落盘顺序（`done` 先落是 `MarkTerminalEmitted` 兜底逻辑有意为之）。修复已提交推送（不发版），见下。
- **修复（已提交推送，不发版）**：`internal/channel/supervisor/supervisor_test.go` 的 `TestSupervisorClosedLoop` 里，等 `done` 之后再 `waitForEvent(KindTurnFinished, 5s)` 再读事件。本地验证：`TestSupervisorClosedLoop -count=30` 全过，`go test ./...` 18 个包全过，gofmt 干净。

## 教程与 README 按 agent 界面用法重写（2026-11-10，用户确认）

### 背景与定性

- 用户反馈：v0.2.1 版中文教程“跟 trellis 的使用很大区别”——Trellis 是 init 完后全程在 agent 界面里工作（Claude 就在 Claude 界面，Codex 同理）。2026-11-10 用当前树新 init 核实：init 生成 `.sunshine/`（workflow.md、spec、AGENTS.md）+ `.claude/skills|commands|agents`（Codex/Cursor 同理各平台配置），AGENTS.md 开头即“本文件面向在本仓库工作的 AI 编程助手”，workflow.md 里写的是 agent 的操作步骤（派发 implement/check 子代理、`/sunshine:finish-work`）。结论：v1 教程写偏了——它把 `sunshine task create/start/...` 写成人要在终端一步步敲的，实际上这些命令是 agent 在底下调的，是机制不是人的日常界面。主次颠倒。
- 范围（用户 2026-11-10 明确）：中英文教程与 README 都要重写，不只是教程。

### 实施说明

#### 主线（中英文教程、README 均按此口径）

1. 终端只跑一次：`sunshine init`（装好 + 初始化项目），顺带说明装了哪些平台配置、生成了哪些文件（给 agent 读的）。
2. 打开项目进 agent 界面（Claude Code 为主，Codex/Cursor 同理一句话带过）：人在界面里用自然语言提需求（沿用退款原因必填校验的演示项目，保持连续性）。
3. agent 按 workflow.md 干活：建任务目录、写 prd/design、派 implement/check 子代理、核查、把学到的写回 spec、`/sunshine:finish-work` 收尾；人检查 diff、验收、提交。
4. 终端命令只出现在两处：**“底下发生了什么”**（agent 实际调的命令 + 真实输出，须实跑验证）与**“手动干预时”**（`task list/current` 查状态、`update --dry-run`、`upgrade`、`mem search` 回顾等）。
5. update / registry / mem / channel / upgrade 各归位到这条主线里讲（谁在 agent 界面里触发、谁在终端手动跑），不单列 CLI 手册式章节。

#### 诚实规则

- 与 agent 的对话示例是“示例提示词”，须标明是示例，不伪装成真实会话记录；展示的文件内容与命令输出必须是真实跑出来的（跑等价命令取输出）；沿用 v1 已有的如实说明：`channel spawn` 真实拉起受限（claude 二进制不在 `$PATH`、codex 未登录 401）、mem 无真实会话时先给用法再用手工夹具演示并注明、`update`/`upgrade` 只用 `--dry-run`。
- 不编造 flag、命令与输出；不写没有验证过的断言。

#### 各文件要求

- `docs/tutorial.md`（中文）与 `docs/tutorial.en.md`（英文）：全文重写，同结构；演示项目不变（退款原因必填校验）；英文版命令与输出保持中文原文并加英文解释（CLI 输出为中文）。
- `README.md`（中文）与 `README.en.md`（英文）：全文重写，同结构。README 仍是“门口”：安装、快速上手（init + 进 agent 界面）、主线一句话、命令表改为“agent 在底下调的命令 + 人手动干预时用的命令”双口径（表还是命令表，但不再写成人每天要敲的流程）、配置要点、模板市场、链接（教程、PLAN.md、Releases）。不塞长篇教程。
- 两份 README 与两份教程互链，锚点须与重写后的实际标题对得上。
- 不提交推送，改动留工作树等主代理验收。

### 实施记录（2026-11-10）

- **分工**：中英文两路并行——中文组重写 `docs/tutorial.md` + `README.md`，英文组重写 `docs/tutorial.en.md` + `README.en.md`；共用同一套 12 节结构与同一演示项目（退款原因必填校验），各自在独立临时目录实跑命令取输出。改动未提交推送，等主代理验收。
- **改了 4 个文件**：`docs/tutorial.md` 740 行 → 837 行；`docs/tutorial.en.md` 781 行 → 940 行；`README.md` 84 行 → 89 行；`README.en.md` 86 行 → 90 行。主线均已改为 agent 界面优先：开场一句话即“终端只跑一次 init，进 agent 界面提需求，agent 按 workflow.md 干活，人检查验收”；终端命令只在第 4 节“底下发生了什么”与第 12 节“手动干预速查”出现；README 命令表均改为双口径（“谁用”/“Called by”列：agent 在底下调 / 人手动干预时用）。
- **诚实规则执行情况**：两版对话示例均已标明是示例提示词、不是真实会话记录；mem 均用手工夹具并注明；channel spawn 拉起受限均按实测写（claude 二进制缺失记 error 事件、codex 未登录 401）；update/upgrade 只用 `--dry-run`（英文版另加一次真实 update 以验证备份行为，备份路径与时间戳为真实值）。§4 的 task.json 两版都用归档后版本并注明状态为 completed。
- **总协调独立复验**：`gofmt -l .` 无输出、`go vet ./...` 通过、`go test ./... -count=1` 562 个 PASS、0 FAIL（18 个包）。用当前树新构建的二进制重跑：init 输出与教程引用一字不差、115 个文件（不含 `.git`）、workflow.md 714 行、`--help` 里“先用 sunshine init 初始化当前项目，再让 AI 助手按 .sunshine/workflow.md 推进任务”为真实文案；`task create` 输出与教程引用一致（日期 11-10 为实跑当天）；workflow.md 里 1.1/1.3/1.4/3.3/3.4 与 `/sunshine:finish-work` 均真实存在，与教程第 3 节的步骤号对得上。两份 README 的锚点逐个核对过，与重写后实际标题一致；全文无残留旧路径（`/tmp/sunshine-docs`）、旧日期（11-09）与旧版本（0.2.0 作版本号处），仅一处 `upgrade --tag 0.2.0` 是指定版本的示例写法，合理。
- **与 PLAN.md 的偏离**：结构与范围无偏离。各路自行判断两处已在汇报里说明并采纳：英文版 §4 用归档后 task.json 并注明、§8 补一次真实 update 引用以验证备份说法；中文版用 0.2.1 二进制重建 demo 以得到干净的 update 三态输出、mem 夹具按适配器要求修过一次（助手 content 改块数组并补 `cwd`）、与 v1 有出入的输出均按本次实测写（spawn 多行 pretty JSON、kill 对拉起失败 worker 的报错、chat 上 post 的报错文案等）。

- **主代理独立复验（2026-11-10）**：`sunshine version` 存在（教程引用有效）；workflow.md 714 行、1.1/1.2/1.3/1.4/2.1/2.2/3.3/3.4 与 `/sunshine:finish-work` 均真实存在；init 输出与教程引用一字不差、干净目录 115 个文件；`--help` 文案真实；`task create` 的 prd.md 骨架与教程引用一字不差；两份 README 的教程锚点（§4/§8/§12）与重写后实际标题逐个对得上。复验发现一个小缺口已直接补上：中文教程第 3 节列了 1.1/1.3/1.4，漏了可选的 **1.2 研究**且未说明——已补一行（英文版第 3 节是散文结构、未枚举步骤号，无此问题）。另：复验第一轮 `go test ./...` 末尾出过一次 FAIL，但输出只留了尾部 3 行、没抓到是哪个测试失败；随后连续 5 轮全量（562 个 PASS）与 supervisor/store 各 15 次、cli 3 次定向压测全部通过，未能复现。原因未定位，不谎报已修；若 CI 再抖，先抓完整日志再定。
- **本批次已随 v0.2.2 提交推送并发布**（见下“v0.2.2 发布记录”）。

- 内容：教程与 README 按 agent 界面用法重写（上一节的批次）。提交 `618d4d0`（351 个文件），tag v0.2.2 为注释 tag（对象 `8ad8f01`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.2.2，6 个资产；发布与两路 CI 均 success——这次没有抖动。实测验证：真实一键命令装上 v0.2.2（`--version` 打印 0.2.2）；0.2.1 二进制 `upgrade --dry-run` 正确查到 0.2.2；darwin 包 checksum OK，解出 Mach-O x86_64 二进制。
- 上一轮复验那次未定位的 FAIL 仍未复现（本轮发布三路 CI 全过，本地又跑过数轮全量全过）；若 CI 再抖，先抓完整日志再定。

## init 项目类型检测 + 手动选择（2026-11-10，用户定）

### 背景与范围

- 起因：用户反馈 init 完成后的“下一步”误导——spec 与 workflow 都是 init 时自动落好的，不用人工改。上一轮已把“下一步”改成 agent 界面优先的两行（工作树未提交，见 `git status`：`internal/cli/init.go`、`docs/tutorial.md`、`docs/tutorial.en.md`），本批在其上继续做。
- 用户定：**检测加手动选**——检测不到时 init 问一句，或加 `--type` 参数。
- 本批范围（单仓）：init 时检测项目类型（前端/后端/全栈），只生成对应的 spec 模板；检测不到（unknown）时交互问一句；`--type` 可显式指定。**不在本批**：monorepo 检测（需求清单第 46 条后半部分）、bootstrap 任务（清单第 50 条，Trellis init 生成的 00-bootstrap-guidelines 任务，Sunshine 阶段 1 没做）、把类型持久化进 config.yaml、类型影响 update/registry 的行为。

### 上游行为（以源码为准，出处）

- 检测：`packages/cli/src/utils/project-detector.ts` 的 `detectProjectType(cwd)`。前端信号：vite/next/nuxt/webpack/rollup/svelte/astro/angular/vue 配置文件、`src/App.tsx|jsx|vue`、`src/app/page.tsx`、`app/page.tsx`、`pages/index.tsx|jsx`、package.json 的前端依赖（react、vue、svelte、angular、@angular/core、next、nuxt、astro、solid-js、preact、lit、@remix-run/react）；后端信号：go.mod/go.sum、Cargo.toml/lock、requirements.txt、pyproject.toml、setup.py、Pipfile、poetry.lock、pom.xml、build.gradle(.kts)、Gemfile、composer.json、*.csproj/*.fsproj、mix.exs、server.ts/js、src/server.ts、src/index.ts、package.json 的后端依赖（express、fastify、hono、koa、hapi、nest、@nestjs/core、fastapi、flask、django）。前后都有 → fullstack；都没有 → unknown；init 里 unknown 按 fullstack 处理。
- **上游 quirks，不照搬**：`FRONTEND_INDICATORS` 里含 `"package.json"`，而 `detectProjectType` 是 `FRONTEND_INDICATORS.some(fileExists)`，所以只要有 package.json 就算前端信号——纯 Express 后端项目会被判成 fullstack。Sunshine 的检测只认依赖本身：package.json 存在本身不算前端信号，只有命中前端依赖才算。这是与上游有意的偏离，理由如上，测试要钉住（express-only 的 package.json → backend，不是 fullstack）。
- spec 生成：`packages/cli/src/configurators/workflow.ts` 的 `writeSpecForType`——frontend 只写前端文档、backend 只写后端文档、fullstack 两边都写；另有“Guides - always created regardless of mode”（与类型无关的通用指南永远写）。对应到 Sunshine 的 3 份扁平模板：`system.md`（总原则，与类型无关，永远写）、`frontend.md`（只前端/全栈写）、`backend.md`（只后端/全栈写）。
- 上游 init **没有**项目类型交互提问、也没有 `--type` 参数，只有检测；手动选是用户要求的新增能力。类型也不写进 config.yaml，只影响本次 init 生成的 spec 文件与 bootstrap 任务内容。

### 设计决策（已定）

1. 新包 `internal/projecttype/`：`Detect(cwd) ProjectType`，值 `frontend | backend | fullstack | unknown`；指示器与依赖表按上游原样移植（除 package.json quirks 外）；另给中文展示名（前端/后端/前后端全栈）与 `Parse`（供 `--type` 校验）。
2. 优先级（高→低）：`--type` 显式值 > 检测出的 frontend/backend/fullstack > 检测 unknown 时交互提问（仅 TTY 且非 `-y`）> 默认 fullstack（`-y`、非 TTY、提问失败/EOF 时）。`--type` 非法值直接报错并列出合法值。
3. 提问只在需要生成默认 spec 时发生：`--registry` 写 spec（SkipSpec）时不问——类型用不上；增量模式不动 spec，也不问。`--force` 重建走与全新 init 同样的流程（检测/提问/按类型写 spec）。
4. 提问形式沿用 init 现有模式：survey Select（与 `promptPlatforms` / `promptStatusline` 同款，`var promptX = func...` 便于测试替换），问题“检测不到项目类型，请选择：”，选项 前端项目/后端项目/前后端全栈，默认 全栈。
5. scaffold 接线：`InitOptions` 加类型字段（如 `SpecType`），walk 时按类型跳过不匹配的 spec 文件（与现有 `SkipSpec` 跳 `spec/` 前缀同款写法）；哈希清单只记录实际写出的文件（现成逻辑：只对写出的文件算哈希，跳过即不入清单）。
6. init 输出要让用户看见类型与来源：一行 `项目类型：前端（自动检测）` / `后端（--type 指定）` / `全栈（手动选择）` / `全栈（默认）` 之类；“下一步”第 2 行按实际类型措辞（与生成的文件集一致），registry 时保持上一轮的“从模板市场装进”写法。
7. 文档：init `--help` 的 Long 加 `--type` 与检测说明；中英文教程 §1 的 init 输出引用按实跑结果更新，并在 §1 附近补一小段说明类型检测与手动选择（中文版与英文版都要，英文版输出引用仍为中文原文+英文翻译）；README 核心命令表如列 init 参数则同步（没有则不动）。
8. 测试：`Detect` 单元测试（前端/后端/全栈/unknown 各至少一个夹具；express-only package.json → backend 钉住 quirks 偏离；package.json 损坏 → unknown；无 package.json 的纯 go.mod → backend）；解析优先级测试（flag > 检测 > 提问 > 默认；`-y` + unknown → fullstack；非 TTY + unknown → fullstack；registry 时不提问）；init 端到端（临时目录实跑：前端项目只剩 system.md + frontend.md；后端只剩 system.md + backend.md；全栈三份都在；`--type` 覆盖检测结果；提问路径用 stub 的 prompt 函数）。
9. 验收（主代理独立复验）：`gofmt`、`go vet`、全量 `go test`；临时目录实跑四条路径（前端检测、后端检测、unknown 提问、`--type` 覆盖）+ registry 回归；教程引用与实跑输出逐字一致；全仓无 trellis 残留（新代码原创实现，不复制上游源码）。

### 实施记录（2026-11-10）

- **分工**：检测包与 init 接线两路并行（契约先定死：`projecttype.Type` 常量顺序 Unknown/Frontend/Backend/Fullstack、`String()`、`DisplayName()` 前端/后端/全栈/未知、`Parse`、`Detect`），文档由总协调合流后自己收尾。
- **检测包（`internal/projecttype/`，新建）**：`projecttype.go` + `projecttype_test.go`（3 个测试函数、42 个子测试）。指示器与依赖表按上游原样移植（仅读 dependencies + devDependencies 键名；`*.csproj`/`*.fsproj` 按根目录后缀匹配；文件不存在/读失败/package.json 解析失败一律当无信号，不报错）。与上游的有意偏离已实现并钉住：前端文件信号里不含 `package.json`，只有命中前端依赖才算前端信号（express-only 的 package.json → Backend，不是 Fullstack）。`Parse` 精确匹配（无大小写折叠、无去空格），非法值错误文案为 `无效的项目类型：%s（合法值：前端、后端、全栈）`。
- **init 接线**：`internal/scaffold/scaffold.go`（`InitOptions` 加 `SpecType`，walk 按类型跳过不匹配的 spec 文件，跳过的不进哈希清单）与 `internal/cli/init.go`（`--type` flag、Long 补检测说明、`resolveProjectType` 纯函数 + `promptProjectType` var func 注入、全新 init 与 `--force` 两个分支解析并传 `SpecType`、打印 `项目类型：<展示名>（<来源>）`、`specNextStepLine()` 按类型措辞“下一步”第 2 行）。新增测试：`internal/cli/init_projecttype_test.go`（解析优先级 11 组表驱动、行文案、5 组端到端实跑二进制）与 `internal/scaffold/scaffold_test.go` 的 `TestInitSpecTypeSkipsMismatchedSpec`（四档文件集 + 哈希清单断言）。
- **文档（总协调）**：中英文教程 §1 的 init 输出引用已按实跑逐字更新（机械 diff 校验通过，含 `项目类型：全栈（默认）` 与第 2 行文件清单），§1 附近各补一段类型检测与手动选择说明（英文版输出仍为中文原文 + 英文翻译）；§8 registry 引用未变（实跑确认无项目类型行、模板市场写法不变）。README 核心命令表未列 init 参数，按 PLAN 第 7 条不动。上一轮“下一步”文案改动仍在工作树内，未回退。
- **总协调独立复验**：`gofmt -l .` 无输出、`go vet ./...` 通过；全量 `go test ./... -count=1` 19 个包全过（本批合流后连续 3 轮全过，另基线 1 轮）。临时目录实跑：空目录 `-y` → `项目类型：全栈（默认）`、三份 spec、115 个文件；vite 项目 → `前端（自动检测）`、只剩 system.md + frontend.md；go.mod 项目 → `后端（自动检测）`、只剩 system.md + backend.md；go.mod 项目 `--type 前端` → `前端（--type 指定）` 覆盖检测；`--type web` → 报错、exit 1、不生成 `.sunshine/`；本地路径 registry 回归 → 无项目类型行、“从模板市场装进”写法不变、demo.md 装好。
- **与 PLAN.md 的偏离**：无实质偏离。两处微小实现选择（接线一路汇报）：① `resolveProjectType` 对 flag 值先 TrimSpace 再 Parse（Parse 本身不接受带空格值，仅作容错）；② e2e 测试里构建目标用 `./cmd/sunshine`（仓库根目录无 main 包）。另：`resolveInitProjectType` 仅当无 `--type` 且非 SkipSpec 时才调 `Detect`（SkipSpec 时连检测都不做，与“类型用不上”一致）。
- **未复现的抖动（两路各遇一次，实话记录）**：接线一路全量第一轮 `internal/cli` 曾 FAIL（尾部是 update 干跑输出，未抓到具体用例），随后复跑全过；总协调合流后 3 轮全量全过，未能复现。原因未定位，不谎报已修；若 CI 再抖，先抓完整日志再定。
- **验证局限**：unknown 提问的真实 TTY 路径未实跑成功——`script` 分配 pty 后 survey Select 挂起（管道送的换行没被接受），已 kill；提问路径由 stub prompt 的单元测试覆盖（PLAN 第 8 条要求的覆盖方式），与已在用的 promptPlatforms/promptStatusline 同款 survey 模式。registry 回归用的是本地路径 registry（上一轮 SSH 端到端的那套本地 sshd 环境已不在——虚拟机换过，/git 与 /tmp/ssh-e2e 均已消失；SSH 路径本身未动，不影响本批结论）。
- **本批已随 v0.2.3 提交推送并发布**（见下“v0.2.3 发布记录”）。

- **主代理独立复验（2026-11-10）**：`gofmt -l .` 无输出、`go vet ./...` 通过、全量 `go test ./... -count=1` 19 个包全过（本批新增 `internal/projecttype`）；改动一处后 `internal/cli` 复跑通过。临时目录实跑与总协调一致：空目录 `-y` → 全栈（默认）、115 个文件；vite → 前端（自动检测）；go.mod → 后端（自动检测）；`--type 前端` 覆盖 go.mod 检测；`--type web` 报错 exit 1、不生成 `.sunshine/`。中英文教程 §1 引用与实跑输出做了机械逐字 diff，一致；registry §8 实跑确认无项目类型行。
- **主代理复验时修掉一个真 bug**：上一轮“下一步”第 2 行的 registry 条件写的是 `target != nil`，但非 spec 模板（如 skill）经 `--registry` 安装时 `WritesSpec()` 为 false、默认 spec 实际照常生成，旧条件会错印“规范已从模板市场装进 .sunshine/spec/”。已改成 `if skipSpec`，并用 skill 模板本地 registry 实跑验证：正确打印 `项目类型：全栈（默认）` 与默认模板行，skill 正常装进 `.agents/skills/`。工作流模板经 `--registry` 本就被拒（“请用 sunshine workflow --template 安装”），不受影响。
- **真实 TTY 提问路径已跑通（补上总协调的验证局限）**：`script` 挂起的根因是 survey 在 Confirm/Select 前会发 `\x1b[6n` 问终端光标位置，等不到终端回 `\x1b[行;列R` 就一直等——真实终端会自动回，属验证手段问题，不是产品 bug。用 Python pty 并回 DSR 后，全程交互跑通：平台多选 → 状态栏确认 → “检测不到项目类型，请选择：”回车取默认 → `项目类型：全栈（手动选择）`，三份 spec 落盘。
- **未复现的抖动**：本轮我的全量 1 轮 + cli 复跑均全过，仍未复现；两路各遇一次、均无完整日志，原因未定位，不谎报已修；若 CI 再抖，先抓完整日志再定。
- **本批已随 v0.2.3 提交推送并发布**（见下“v0.2.3 发布记录”）。

### v0.2.3 发布记录（2026-11-10）

- 内容：init 项目类型检测 + 手动选择（上一节的批次，含“下一步”文案重写）。提交 `6a6243a`（354 个文件），tag v0.2.3 为注释 tag（对象 `2aedc44`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.2.3，6 个资产；发布与两路 CI 均 success——这次也没有抖动。实测验证：真实一键命令装上 v0.2.3（`--version` 打印 0.2.3）；0.2.2 二进制 `upgrade --dry-run` 正确查到 0.2.3；darwin 包 checksum OK，解出 Mach-O x86_64 二进制；用发布的 0.2.3 二进制实跑：vite 项目 → `项目类型：前端（自动检测）`，只生成 system.md + frontend.md。
- 未复现的 FAIL 仍未复现（本轮发布三路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

## 滚动轨道：剩余 19 个平台（2026-11-10，用户定做）

### 背景与范围

- 用户定：把滚动轨道 19 个平台全部接上，做完后**所有功能跑一遍真实场景测试**。
- 19 个：opencode、kilo、kiro、gemini、antigravity、devin、qoder、codebuddy、copilot、droid、dsh、pi、reasonix、zcode、trae、omp、grok、kimi、snow。加已有的 claude-code、cursor、codex，共 22 个平台。
- 依据：需求清单第 5 模块（第 47、48、67、90、91、93、94 条）；源码对照 `packages/cli/src/types/ai-tools.ts`（AI_TOOLS 注册表，唯一事实源）、`packages/cli/src/configurators/{<平台>,shared,index}.ts`、`packages/cli/src/templates/{common,<平台>}/`。

### 上游注册表（ai-tools.ts，已核对，19 家全为 defaultChecked=false）

| 平台 | configDir | cmdRefPrefix | agentCapable | hasHooks | supportsAgentSkills | 备注 |
|---|---|---|---|---|---|---|
| opencode | .opencode | /sunshine: | true | false | — | 无 session-start hook；agentCapable && !hasHooks 时有 fallback 命令 |
| kilo | .kilocode | /sunshine: | false | false | — | 无独立模板目录（见下） |
| kiro | .kiro/skills | $ | true | true | — | agents 为 .json |
| gemini | .gemini | /sunshine: | true | true | true | |
| antigravity | .agent/workflows | / | false | false | — | 无独立模板目录（见下） |
| devin | .devin/workflows | /sunshine- | false | false | — | 无独立模板目录（见下） |
| qoder | .qoder | $ | true | true | — | |
| codebuddy | .codebuddy | /sunshine: | true | true | — | |
| copilot | .github/copilot | / | true | true | — | 另有 .github/hooks 等管理路径 |
| droid | .factory | /sunshine- | true | true | — | |
| dsh | .dsh | trellis-（裸名） | true | false | true | |
| pi | .pi | /sunshine- | true | true | true | |
| reasonix | .reasonix | /skill trellis- | true | false | — | |
| zcode | .zcode | /sunshine: | true | true | — | 另有 globalHookPlugin trellis-bridge（外部仓库），项目 hook 经 .zcode/config.json 注册 |
| trae | .trae | /sunshine- | true | true | — | |
| omp | .omp | /sunshine: | true | true | — | |
| grok | .grok | /sunshine- | true | false | — | extraManagedPaths：.grok/skills、.grok/commands、.grok/agents；hook stdout 不注入模型，hasHooks 保持 false |
| kimi | .kimi-code | /skill:trellis- | true | false | true | 技能双份：共享 .agents/skills（中性解析）+ .kimi-code/skills 入口；无项目级 hooks 文件 |
| snow | .snow/skills | /sunshine- | true | true | — | extraManagedPaths：.snow/commands、.snow/agents、.snow/hooks、.snow/SNOW.md；commands 为 .json（type: prompt） |

- 各平台模板目录文件数（`templates/<平台>/`）：opencode 10、kiro 6、gemini 5、qoder 5、codebuddy 5、copilot 17、droid 5、dsh 2、pi 6、reasonix 3、zcode 5、trae 5、omp 5、grok 4、kimi 4、snow 9；kilo、antigravity、devin 为 0（无独立模板目录）。
- executorAI / userActionLabel 按 ai-tools.ts 原样登记（kilo/antigravity/devin 为 "Bash scripts or file reads" / "Workflows"；其余见源码）。

### 设计决策（已定）

1. 沿用阶段 2 的适配器接口与模板管线：注册表 `internal/platforms` 是唯一事实源；skills/commands/agents/hooks + SHARED_HOOKS_BY_PLATFORM 分配；占位符解析（`{{CLI_FLAG}}`、`{{PYTHON_CMD}}` → sunshine、`{{CMD_REF:name}}` 按 cmdRefPrefix、条件块 `{{#HAS_HOOKS}}`/`{{#AGENT_CAPABLE}}`）；无 hooks 平台注入 pull-based prelude，有 hooks 平台过滤 `/start` 入口命令（shared.ts filterCommands）；hooks 一律调 `sunshine hook <事件>`，命令前加 `SUNSHINE_HOOK_PLATFORM=<平台>`（阶段 2 关键决策 1，不写 Python）。
2. 每平台一个 init flag（需求清单第 47 条）：`--opencode --kilo --kiro --gemini --antigravity --devin --qoder --codebuddy --copilot --droid --dsh --pi --reasonix --zcode --omp --trae --grok --kimi --snow`；`--windsurf` 为已废弃别名，自动转 `--devin` 并打印警告。交互多选列出全部 22 个平台，默认勾选仍只 claude + cursor。`--with-statusline` 仍仅 Claude Code。用户 2026-11-10 确认：保持 Trellis 做法（核对源码：没带 flag + `-y` 只装默认勾选的 claude + cursor，交互是多选框，无“没带 flag 就全装”行为，也无 `--all` 参数）。
3. kilo、antigravity、devin（存疑点 5）先读各自 configurator（`configurators/{kilo,antigravity,devin}.ts`）再定：核对是复用其他平台模板、还是只写 workflows 目录（`.kilocode` / `.agent/workflows` / `.devin/workflows`）、有无 skills/commands/agents；三家 agentCapable 均为 false、hasHooks 均为 false，按上游形态写 workflow 文件，不硬凑 skills/commands。
4. 平台专属文件格式按各 configurator 走：Kiro agents 为 `.json`、Codex 为 `.toml`、Snow commands 为 `.json`（type: prompt）、Copilot 另有 `.github/hooks`、ZCode hook 经 `.zcode/config.json` 注册（globalHookPlugin 是外部仓库，只作提示、不自动装）、Grok/Kimi 的 hasHooks 保持 false（上游有明确说明，不擅自改 true）。
5. 模板翻译：各平台目录文件译成中文进 `templates/platforms/<平台>/`（命名与改名规则同阶段 2：trellis→sunshine、`.trellis/`→`.sunshine/`、skill/命令前缀 `trellis-`→`sunshine-`、技术术语留英文）；commands/skills/agents 的形态转换（slash 命令 vs skill）按 shared.ts 与各 configurator；翻译后模板不得残留未解析占位符（buildFiles 已有校验）。
6. 注册表驱动的下游自动跟上，逐项核对：`sunshine platforms`（按哈希清单判断已配置）、`sunshine update` 的 RenderTemplates（每平台期望模板集合）、uninstall/ablate/restore（阶段 4 的孤儿键清理器：接新平台时补对应清理器）、`sunshine-meta` skill 里平台数量的说法（现写“3 个已接 + 滚动轨道 19 个待扩展”，改成 22 个）。
7. 文档：init `--help` 补 19 个 flag 与平台列表；中英文教程：§1 的 init 输出引用不变（默认仍只装 claude + cursor），但 §1 附近补一句 22 个平台与 `--help` 可查；README 如列平台则同步为 22 个。`sunshine-meta` 的平台数量同步改。
8. 测试（每平台）：临时目录实跑 `init --<flag> -y`（或交互多选 stub），断言生成文件集与上游 configurator 一致、内容中文、无 trellis 残留、无未解析占位符、哈希清单包含写出文件；`sunshine platforms` 能认出已装平台；`update --dry-run` 在新平台项目上无异常；`--windsurf` 转 `--devin` 并警告。全量 `go test` 通过。

### 真实场景全功能测试（用户明确要求，本批收尾做）

- 平台批次完成后，**所有功能跑一遍真实场景测试**，不是只测新平台：init（默认/全平台/各类型项目/registry）、task 全生命周期（建任务、add-context、start、进度、完成、归档）、workflow 三模板切换、update（三态/备份/registry 刷新/migrate）、mem（真实会话检索）、channel（create/send/wait/forum/spawn）、upgrade（dry-run 与真实升级路径）、uninstall/ablate/restore/platforms、项目类型检测（含真实 TTY 提问）。
- 每条场景用真实二进制在临时目录实跑，输出与文件落盘逐项核对；受环境所限跑不了的（如 channel 真实拉起缺 claude 二进制、mem 缺真实会话、SSH 缺凭据）如实写清、标未覆盖，不编造通过。
- 验收（主代理独立复验）：同上逐项复跑 + 全量 `go test` + `gofmt`/`go vet` + 教程引用与实跑输出一致 + 全仓无 trellis 残留。

### 滚动轨道实施记录（2026-11-10，D 组接线：16 个平台）

分工：D 组接线（注册表、buildFiles 数据驱动化、init 接入、下游联动、文档、测试）。模板翻译与四份集成规格由 A1/B/A3/C 路先行交付（`/tmp/plat-specs/`），本批只做 Go 接线，不改模板内容（除下述 bundled-skills.md 第 90 行一处共享文件修复）。

- 代码：`internal/platforms/`（注册表扩至 19 条、`buildFiles` 改为注册表驱动通用引擎、grok/kimi 的 pull prelude 注入、COMMAND_DESCRIPTIONS 统一、ZcodeSetupHint）、`internal/cli/init.go`（16 个 flag + `--windsurf` 废弃别名、交互多选列出 19 个平台）、`internal/lifecycle/`（AllManagedDirs 扩至 19 家托管目录、新清理器、devin 遗留检测）、`internal/update/prune.go`（devin 遗留检测与孤儿键保留）。
- 验证：`go build`、`go vet`、`gofmt`、全量 `go test ./...`（581 个测试，19 个包）全过；真实二进制在临时目录逐平台实跑 `init --<flag> -y`，文件数与各规格 §3 一致（kiro 50、gemini 49、qoder 49、pi 51、zcode 49、trae 49、omp 49、snow 54、opencode 56、dsh 47、reasonix 47、grok 49、kimi 52、kilo/antigravity/devin 各 46，另加 17 个 `.sunshine/` 骨架文件）；`sunshine platforms` 能认出已装平台（含 antigravity 按 `.agent/workflows`、snow 按 `.snow/skills`、devin 的 `.windsurf/workflows/sunshine-*` 遗留兼容）；`update --dry-run` 在新平台项目上无异常（kimi、grok 实测“已是最新”）；`--windsurf` 转 `--devin` 并打印中文警告；原有三平台既有测试全过（行为字节一致）。
- 顺带修复：`templates/platforms/common/bundled-skills/sunshine-meta/references/local-architecture/bundled-skills.md` 第 90 行渲染时自我损毁——改写为不含字面可解析 token 的表述，claude（两项 true）与 antigravity（两项 false）各渲染一次验证句子完整，回归测试已钉住。

### 滚动轨道实施记录（2026-11-10，G 组接线：codebuddy、copilot、droid 三家）

分工：G 组接线（注册表、init 接入、下游联动、文档、测试）。模板翻译与 E 路集成规格已先行交付（`templates/platforms/{codebuddy,copilot,droid}/` 共 26 个文件），本批只做 Go 接线，不改模板内容；三家至此全部接完，平台总数 22 个。

- 代码：`internal/platforms/`（注册表扩至 22 条；命令形态新增 `prompt`，只 copilot 用——`.github/prompts/<名>.prompt.md` 裸名、无 frontmatter；其余两家走既有 `subdir` 形态；`TestRegistry` 改为 22 条、三家核心字段投影、`TestInstallUnknownPlatform` 的未知平台改用 `not-a-platform`）、`internal/cli/init.go`（`--codebuddy --copilot --droid` 三个 flag、交互多选列出 22 个平台、默认勾选仍只 claude + cursor、`--with-statusline` 仍仅 Claude）、`internal/lifecycle/`（`AllManagedDirs` 补 `.codebuddy`、`.factory` 与 copilot 的 5 条子路径 + `.github/copilot-instructions.md` 本人；清理器：codebuddy/droid settings.json 嵌套 hooks 形、copilot 两份 hooks.json 扁平形、copilot-instructions.md 托管块清理器）、`internal/update/`（`CollectTemplateFiles` 对 copilot-instructions.md 走托管块合并，`AnalyzeChanges` 的 `copilotTemplate` 改传合并后内容）。
- 文档：sunshine-meta skill 内 13 个文件、platform-map.md（三家状态改为“已接”，hook 列按实际文件修正：codebuddy/droid 只有 settings.json，copilot 为两份 hooks.json）、中英文教程 §1、中英文 README 全部由“19 个平台（三家待后续处理）”改为 22 个；copilot-instructions.md 的平台目录列表（上游 14 项）维持原样，未扩到 22 个（E 规格遗留问题 7，另立任务再定）。
- 验证：`gofmt`、`go vet`、`go build`、全量 `go test ./...` 全过；真实二进制在临时目录逐平台实跑 `init --<flag> -y`，文件数与 E 规格 §3 一致（codebuddy 49、droid 49、copilot 51，另加 17 个 `.sunshine/` 骨架文件）；`sunshine platforms` 能认出三家；`update --dry-run` 在三家项目上“已是最新”无异常；原有 19 平台既有测试全过（行为字节一致）。
- 与 E 规格 §9/D 路管线的偏离：无功能偏离。两处说明：① `buildFiles` 从不按目录遍历写出，`templates/platforms/copilot/prompts/` 的 13 个携带件天然不在写出集合里，无需显式排除（NOTES.md 第 8 节已记）；② copilot-instructions.md 的 uninstall 清理器是新增的托管块剥离（只删 Sunshine 块、保留块外用户内容），E 规格 §9.7 只点了“孤儿键清理器覆盖托管路径”，清理器形态按上游 scrubber 补齐；③ `AllManagedDirs` 把 `.github/copilot-instructions.md` 文件本人也列入，只为让覆盖测试认领它，`.github` 本身不在此列。

### 与规格 / PLAN 的偏离（以规格正文 + 上游源码为准）

1. **平台总数按实写 19 个，不是本节原写的 22 个**：codebuddy、copilot、droid 三家的模板翻译与规格被安全策略拒绝、无交付物，本批未为它们写模板、注册表或 flag；注册表、sunshine-meta、教程、README 凡写平台数量处一律写 19 个，并注明三家待后续处理。本节“背景与范围”“设计决策 2/6/7”里的 22 个、19 个 flag 等说法以本条为准，三家接入时另立任务。**（2026-11-10 G 组已接完三家：本条失效，平台总数现为 22 个，见上节 E 路实施记录。）**
2. **设计决策 3 的措辞已由 C 规格订正**：kilo/antigravity/devin 除 workflow 文件外还写 5 个自动 skill + 4 个内置多文件 skill（`collectBothTemplates` 全量调用），不是“只写 workflows 目录”；三家 configurator 文件头“start + finish-work”注释滞后，实为 3 个命令全保留。本批按 C 规格 §3 实现（每平台 46 个文件）。
3. **COMMAND_DESCRIPTIONS 统一为 A3 §6.2 的值**：qoder/trae/omp/snow 四家共用；A1 的 qoder 译文与 A3 §6.1 的 snow 译文与之不同时以 A3 §6.2 为准（接线任务书明确要求）。
4. **kiro 的 ConfigDir 登记为 `.kiro/skills`**（A1 规格原值）：`sunshine platforms` 的显示与判定按该值；托管根 `.kiro` 另由 AllManagedDirs 覆盖。
5. **清理器补齐超出阶段 4 的原登记**：除 gemini/qoder/trae（嵌套 hooks 形）与 zcode/snow/opencode/pi 外，另给 `.pi/settings.json` 加了清理器（删 Sunshine 的三个注册键、保留用户键）；kiro 的 agent JSON 含 hooks 但属 Sunshine 自有文件，uninstall 按清单整文件删，不另设清理器。
6. **platform-map.md 另修两处与模板不符的 hook 列**：Gemini 无 `.gemini/hooks/`（只有 settings.json）、Trae 无 `.trae/hooks/`（只有 hooks.json）、Qoder 无 `.qoder/hooks/`（只有 settings.json），已按实际文件修正。

### 遗留问题

1. **codebuddy、copilot、droid 三家已接**（2026-11-10，G 组，见上节 E 路实施记录）：22 个平台全部接完；本条原“待后续处理”已失效。
2. **snow 的面包屑与深度裁剪缺口仍在**（A3 §7 第 2 条）：`.snow/log/sunshine-context.txt` 的写盘与 session/user/subagent 三档深度裁剪目前无 Go 实现；模板侧已按“如果存在才读”写成条件式，不阻塞使用。
3. **pi/omp 扩展依赖 `sunshine` 在 PATH 中**（A3 §7 第 3 条）：扩展在 `sunshine` 不可用时静默降级为空上下文，无任何提示；是否加一次性提示由产品侧定。
4. **opencode 的 `trellis` 字样只剩 15 行有意保留的旧版只读兼容读点**（B 规格 §2.3 第 5 条：`TRELLIS_HOOKS` / `TRELLIS_DISABLE_HOOKS` / `TRELLIS_CONTEXT_ID` / `metadata.trellis` 的读取 + 说明注释）；其余 21 个平台的生成文件 `grep -ri trellis` 零命中。
5. **本批未提交、未推送、未打 tag**（按任务要求）；工作树另有父代理的 PLAN.md 修改与 16 个平台的模板目录（未跟踪），随本批改动一并待协调人处理。

### 滚动轨道实施记录（2026-11-10，接任协调人收尾：审计 + E/F/G/H）

- **接手审计**：上一任协调人未完成就结束（无最终汇报；A2 路 codebuddy/copilot/droid 被安全策略 2 秒拒掉、D 路接线进行中）。接任后先审计：13 个模板目录实为齐全（每家“少 1 个”都是上游 index.ts 代码文件，不应复制；reasonix 本就只有 2 个 agent；snow 的 .py 不译）；opencode 的 trellis 字样全是只读兼容（已逐行核对，15 行：session-utils 2、sunshine-context 2、inject-subagent-context 7、inject-workflow-state 2、session-start 2）；kilo/antigravity/devin 需 0 个模板文件（C 规格：46 个文件全部由 common/ 渲染）；A1/A3/B/C 四份规格已从会话记录恢复到 `workspace/goals/go-sunshine-trellis/hidden_files/plat-specs/`（/tmp 下的原文件已被清空）。
- **E 路（codebuddy/copilot/droid 模板）**：拒绝对话原文写明“仅限那一次、用户在之后轮次再问可试一次”——父代理在新任务里明确再要这三家，按原文这是一次允许的重试，只试一次；结果一次成功，未再被拒。交付 26 个模板文件 + 集成规格（e-codebuddy-copilot-droid.md）；关键发现：copilot 的 13 个 prompts/*.prompt.md 是死模板（上游 collect 从不写出），只翻译携带、不安装。
- **F 路（13 目录内容审计）**：逐平台通过；修了 5 个文件（pi 扩展 3 处英文残留、trae/pi 4 份前置段措辞统一为 A1 版）；JSON 全过、opencode 6 个 JS `node --check` 全过、TS 扩展 esbuild 解析通过。
- **D 路（16 家接线）**：接任时仍在跑，实际在接任前已完成，报告从会话记录恢复（d-wiring-report.md）：注册表 19 条、通用引擎重构、16 个 flag + --windsurf 别名、下游清理器、文档同步；581 个测试。
- **G 路（三家接线）**：注册表 19→22 条，新增 prompt 命令形态（仅 copilot），copilot hooks 双路径写出，copilot-instructions 托管块合并经核对为阶段 4 已有逻辑（启用非新实现），文档 19→22；589 个测试。
- **H 路（真实场景全功能测试）**：9 大项全部通过（init 22 平台逐个实跑文件数一致、一次装 22 家共 932 文件、项目类型检测含真实 TTY 提问、task 全生命周期、workflow 三模板往返 sha256 一致、update 三态/备份/migrate/registry 刷新、mem 真实 Codex 会话 2 条、channel 全套、upgrade 真实升级、uninstall/ablate/restore、5 个 hook 子命令）。未覆盖：upgrade 失败回滚（httptest 已覆盖）、channel 真实拉起成功（无 claude 二进制、codex 未登录 401）、mem 其余 6 个适配器（无真实会话）、mem search 性能、SSH 克隆（此前已验证）。
- **接任协调人独立复验与修掉的两个小问题**：全量 `go test` 19 个包全过（588 个 `--- PASS` 行；G 路的 589 为另一种计数口径）、`gofmt`/`go vet` 通过；init 抽查（gemini 66、copilot 68、kilo 63、--windsurf 别名、22 家一次装 932 文件、platforms 全认出、update --dry-run 已是最新）与 D/G/H 一致。① H 路发现的迁移汇总文案 bug 已修：`PrintMigrationSummary` 无冲突时也印“请手工合并”，改为仅 `len(Conflict) > 0` 时打印（internal/migrate/execute.go，临时测试验证后已删）。② `task current` 无任务时默认形态静默失败（rc=1、零输出）已修：现打印“当前任务：(无)”再返回 1（internal/task/lifecycle.go，与 --source 形态措辞一致；JSON 形态不变）。
- **与 PLAN.md 的偏离**：无实质偏离。两处措辞订正：① C 规格订正决策 3（三家除 workflow 外还写 skills，不是“只写 workflows 目录”）；② 前置段译文以 A1 版为准统一（gemini/qoder/trae/pi/copilot；grok/kimi 的由 Go 侧注入，措辞仍有细微差异，见遗留）。
- **遗留问题**：① `sunshine platforms` 里 dsh 显示为 `DeepSeek Harness (dsh) (dsh)`（上游登记名自带 (dsh) 后缀 + 输出格式又加 (id)；修要动注册表、init 显示名、测试、sunshine-meta 十几处，本批未动，等定夺）。② Snow 面包屑与三档裁剪缺口、pi/omp 扩展依赖 sunshine 在 PATH、opencode 插件运行时依赖、Gemini BeforeAgent 语义、copilot-instructions 平台目录列表未扩 22 个、copilot agents 未在真实宿主加载验证、三平台 hook 真实触发未测（同 D/E 各路遗留）。③ /tmp 是 512M tmpfs，`go test` 的 sunshine-test-bin-* 残留（每份 31M）会写满导致链接失败假失败——本次复验中途即因此 FAIL，已清理并改用 GOTMPDIR=$HOME/.gopath/tmp；并行批次跑测试前先看 `df /tmp`。④ update --migrate 在开发版 0.1.0 二进制上是空操作（0.2.0 清单不在适用区间），须用注入版本号的二进制才跑通真实迁移（此前已记录，H 路再次确认）。⑤ 未复现的 FAIL 仍未复现（本批全量多轮全过）；若 CI 再抖，先抓完整日志再定。
- **本批已随 v0.3.0 提交推送并发布**（见下“v0.3.0 发布记录”）。

### v0.3.0 发布记录（2026-11-10）

- 内容：滚动轨道剩余 19 个平台（上一节的批次），共支持 22 个 AI 编程平台。提交 `b2922ae`（441 个文件），tag v0.3.0 为注释 tag（对象 `16dceb2`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.0，6 个资产；发布与两路 CI 均 success——这次也没有抖动。实测验证：真实一键命令装上 v0.3.0（`--version` 打印 0.3.0）；0.2.3 二进制 `upgrade --dry-run` 正确查到 0.3.0；darwin 包 checksum OK，解出 Mach-O x86_64 二进制；用发布的 0.3.0 二进制实跑：`init --gemini --kilo --copilot` 三家均装好，`platforms` 认出 3 家。
- 未复现的 FAIL 仍未复现（本轮发布三路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

### 教程第 13 节：五个场景（2026-11-10，用户定做）

- 内容：按用户给的五场景表（从零新项目 / 接入存量项目 / 交付产品功能 / 重构老模块 / 修复反复 bug）给中英文教程各加第 13 节，全部命令用 v0.3.0 真实二进制实跑验证，演示目录 `/tmp/scenes/s1-new`、`s2-old`、`s3-cross`、`s4-refactor`、`s5-bug`。
- 五个场景实跑要点：① 空目录 init 默认全栈，第一任务值班表小工具（Go 读 CSV，实跑输出三行值班表），任务归档进 archive/2026-11/。② 存量 Express 项目 init 判后端（只认依赖，express-only 不误判全栈），只生成 system+backend，现有文件不动；第一任务把 README 三条约定落进 spec，grep 验证。③ 全链路任务：implement 上下文三层（Refund.vue/server.js/refunds.json）+ check 一份规范；切 channel-driven 工作流，channel 绑任务，send 两条协调消息；wait 无 worker 超时（上游退出码 124）；收尾切回 native 要 --force。④ TDD 重构：先写特征测试钉住行为（go test 全绿 0.003s），再把折扣规则抽成常量与小函数（仍全绿 0.002s），新约定写进 spec。⑤ 反复 bug：真实 channel spawn 拉起 codex worker，因未登录报 401（worker.log 原文），会话落盘；mem list/search/context 实测可检索；实测边界：mem 只搜对话轮次文字，不搜错误事件（11-10 凌晨的 01a1c376 会话搜 401 无结果，因 401 只在错误事件里）；修掉一个核查错误：11-09 两个探针会话只有一个以 401 结束（另一个 turn_aborted），文档按“11 月 9 日一次、11 月 10 日凌晨一次、今天一次”写。⑤ 的 spawn 另记一坑：`--agent` 必须填 `.sunshine/agents/` 里已有的定义，填不存在的直接报错。
- 本批已随 v0.3.1 提交推送并发布（见下“v0.3.1 发布记录”）。

### v0.3.1 发布记录（2026-11-10）

- 内容：教程第 13 节五个场景（中英文，均已实跑验证；上一节的批次），文档-only。提交 `6ba9d0e`（441 个文件），tag v0.3.1 为注释 tag（对象 `887ae1d`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.1，6 个资产；发布与两路 CI 均 success——这次也没有抖动。实测验证：真实一键命令装上 v0.3.1（`--version` 打印 0.3.1）；0.3.0 二进制 `upgrade --dry-run` 正确查到 0.3.1；darwin 包 checksum OK，解出 Mach-O x86_64 二进制；主分支中英文教程均含第 13 节。
- 未复现的 FAIL 仍未复现（本轮发布三路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

### install.sh 无参安装报 TAG: unbound variable（2026-11-10，用户截图报障）

- 现象：用户在 Mac 上跑 `curl -fsSL .../install.sh | sh`（不带版本），打印“正在查询最新版本……”后报 `sh: line 68: TAG: unbound variable`，两次一样。带版本参数的装法（`sh -s -- v0.3.1`）不受影响——它跳过查询步骤，之前各版本验证用的都是带版本参数的装法，所以一直没踩到。
- 复现与核查：沙箱里复现不了——不带参数在 dash、bash、bash --posix（即 macOS 的 /bin/sh）下都能装上 v0.3.1；又用桩 API 模拟了 403 与空 JSON 两种查询失败，旧脚本的 `[ -n "$VERSION" ] || die` 兜底都正常触发，报的是中文“查不到最新版本”，不是 TAG 错。main 上的 install.sh 与工作区一致，且该文件历史上只有一个版本，不存在“用户拿到旧版”的可能。结论：沙箱里无法复现用户 Mac 上的确切触发机制（用户的 Mac 不可达），如实记下，不谎报已定位。
- 修复（install.sh）：查询最新版不再只走 API——先走 `releases/latest` 的 302 跳转取 Location 头里的 tag（不走 API：不怕限流、不用解析 JSON），拿不到再试 API；两路都拿不到时 die 报中文错；TAG 赋值后加 `[ -n "${TAG:-}" ] || die` 守卫，以后就算版本为空也报中文错，不再出现 `TAG: unbound variable` 这种 shell 报错。另把脚本里的示例版本号 v0.2.0 统一改成 v0.3.1。
- 验证（新脚本，7 项全过）：① 不带参数真实网络 bash --posix 装上 v0.3.1；② 不带参数 dash 装上 v0.3.1；③ 显式版本 v0.3.0；④ SUNSHINE_VERSION=v0.3.0；⑤ -h；⑥ 跳转 404 + API 正常 → 走 API 兜底装上 v0.3.1；⑦ 两路都 404 → 报“查不到最新版本，请检查网络，或用 SUNSHINE_VERSION=v0.3.1 指定版本”，exit 1，不再是 TAG 错。
- 本批已随 v0.3.2 提交推送并发布（见下“v0.3.2 发布记录”）。

### v0.3.2 发布记录（2026-11-10）

- 内容：修 install.sh 无参安装（上一节的批次）。提交 `2b47b03`（441 个文件），tag v0.3.2 为注释 tag（对象 `4322d04`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.2，6 个资产；发布与两路 CI 均 success——这次也没有抖动。实测验证：用户原先那条不带参数的一键命令（从 main 拉新脚本）装上 v0.3.2；0.3.1 二进制 `upgrade --dry-run` 正确查到 0.3.2；darwin 包 checksum OK，解出 Mach-O x86_64 二进制；main 上的 install.sh 含跳转查询与 TAG 守卫。
- 未复现的 FAIL 仍未复现（本轮发布三路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

### install.sh 在用户 Mac 上仍报 TAG: unbound variable（2026-11-10，v0.3.2 发布后用户再报）

- 现象：v0.3.2 的修复推上去后，用户在 Mac 上再跑不带参数的一键命令，仍报 `sh: line 75: TAG: unbound variable`，且这次用户指出 TAG 后面有个乱码字符（�）。行号从 68 移到 75，与 v0.3.2 改动加的 7 行对得上，说明用户跑的确实是新脚本。注意这是第一次安装（本机没有旧 sunshine）。
- 根因定位（从用户机器上的证据推断）：报错行是 `echo "安装 $PROG $TAG（$OS/$ARCH）……"`，而 TAG 在此之前已被守卫（`[ -n "${TAG:-}" ]`，第 56 行）与 URL 行（第 74 行，`$TAG/`）验证过是已设置的——74 行的 `$TAG` 后跟 `/` 能过，75 行的 `$TAG` 后跟全角括号 `（` 就报未设置，唯一差别就是 `$TAG` 后面紧跟的字符。结论：macOS 自带的 bash 3.2.57 在解析 `$TAG（` 时把全角括号吞进了变量名，把一个不存在的变量当成未设置，报错信息里的变量名带了被截断的多字节字符，终端显示为 �。v0.3.2 的跳转查询修的是另一段，与这次无关。
- 复现尝试（均未复现，如实记下）：从 GNU 源码编译了真正的 bash 3.2.0，在 C、C.UTF-8、POSIX、zh_CN.UTF-8 四种 locale 下跑旧脚本都不报错，`$TAG（` 解析也正常，报错信息是干净的 `TAG: unbound variable`。所以触发条件是 macOS 自身的 bash 3.2.57 构建（Apple 的补丁 / macOS libc 的 locale 处理），Linux 上的 bash 3.2.0 复现不了。用户的 Mac 不可达，无法直接验证，只能以用户机器上的证据为准。
- 修复（install.sh）：把脚本里所有“`$变量` 后紧跟全角字符”的位置改成 `${变量}` 写法，共 5 处：`need()` 里的 `$1，`、`$OS（`、`$ARCH（`、第 75 行的 `$TAG（` 与 `$ARCH）`、`$URL（`。全文件扫描确认已无 `$变量` 后紧跟非 ASCII 字符的位置。花括号写法在所有 shell 下都正确，是纯粹的加固。
- 验证（新脚本）：bash 3.2.0 不带参数装上 v0.3.2；dash 与 bash --posix 不带参数装上 v0.3.2；显式版本 v0.3.1；`-h` 正常。最终确认要靠用户在 Mac 上再跑一次一键命令——若仍失败，下一步是 `sh -x` 跟踪。
- 本批已随 v0.3.3 提交推送并发布（见下“v0.3.3 发布记录”）。

### v0.3.3 发布记录（2026-11-10）

- 内容：修 install.sh 里 `$变量` 后紧跟全角字符被 macOS bash 3.2 吞进变量名的问题（上一节的批次）。提交 `ac2d1e5`（441 个文件），tag v0.3.3 为注释 tag（对象 `dd03611`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.3，6 个资产；发布与两路 CI 均 success——这次也没有抖动。实测验证：用户原先那条不带参数的一键命令（从 main 拉新脚本）装上 v0.3.3；0.3.2 二进制 `upgrade --dry-run` 正确查到 0.3.3；darwin 包 checksum OK，解出 Mach-O x86_64 二进制；main 上的 install.sh 含 `${TAG}（${OS}` 花括号写法。
- 未复现的 FAIL 仍未复现（本轮发布三路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

## init 交互选模板（对齐 Trellis，2026-11-10，用户定做）

### 背景

- 用户问 Trellis init 时的弹出选项（以为是选项目性质），核实后确认：上游 init 没有选项目类型的提问（类型是静默自动检测），弹出的选项是 **"Select a spec template:"**（init.ts:1602、1649）：交互模式下载模板索引后列出 from scratch（默认）/ 远程 spec 模板 / custom（输入 registry 来源）；选远程模板则下载进 `.trellis/spec/`，空白模板一套不写；下载失败回退空白模板；`-y` + 市场型 registry 不带 `--template` 直接报错退出。
- Sunshine init 没有这一步：选模板只能靠 `--registry` + `--template` 两个 flag；市场型 registry 不带 `--template` 时 `ResolveTarget` 直接报错列出可选 id。用户确认要补一个交互选单（“让不带 --template 时也能像 Trellis 那样列出来选”）。

### 上游行为（init.ts:1483–1830，对照点）

- 触发：单仓模式、交互模式（非 `-y`）、未传 `--template`。先按 registry（有则自定义，无则官方 `TEMPLATE_INDEX_URL`）拉索引，拉不到时官方源报“用空白模板”，自定义源无 index.json 走直接下载、瞬时失败直接 abort。
- 选单内容：有 registry 时只列 spec 型模板（无 blank、无 custom、无默认）；无 registry（官方）时是 from scratch（默认）+ spec 模板 + custom。选 custom → 输来源（空回选单）→ 探测：市场则再出该来源的模板选单，直接下载则直接装，瞬时失败回选单。
- 选远程模板后：`.trellis/spec` 已存在且无 `--overwrite/--append` 时追问 Skip/Overwrite/Append；下载成功则 `skipSpecTemplates: true`，失败回退空白模板。
- `-y` + 市场型 registry 不带 `--template`：报错退出（须指定 `--template`）。

### Sunshine 方案

- 触发条件与上游一致：交互模式（`!initYes && stdinIsTTY()`）且未传 `--template`；`-y`、非交互终端、已传 `--template` 时一律不走选单（现有行为不变）。
- 选单内容（Sunshine 没有官方模板市场，所以中间一档为空）：
  - 未传 `--registry`：选单为“从零开始（默认）”+“自定义（输入一个模板市场来源）”；选自定义后输来源（空回选单）→ 探测：市场则列出该来源的 spec 型模板再选，无 index.json 则直接下载，探测瞬时失败打印原因后回选单。
  - 已传 `--registry`：探测到市场就直接列出其中的 spec 型模板（无 blank、无 custom，与上游 registry 情形一致）；无 index.json 则直接下载，不出选单。
  - 只列 `type == "spec"` 的条目（与上游的 filter 一致；skill/command/workflow 型不在 init 选单里）。市场里没有 spec 型模板时打印提示、改用空白模板。
- 探测复用：选单探测与安装共用一次 probe——新增 `registry.ResolveTargetWithProbe(src, probe, templateID)`，`ResolveTarget` 改为解析 + 探测后委托给它；出错时 WithProbe 负责关闭 probe，成功时 Target 接管（与现有 ownership 一致，`ProbeResult.Close` 已是幂等的）。
- 接线位置：`RunE` 里平台选择之后、原有 `--registry/--template` 校验之前。选单结果写回 `initRegistry`/`initTemplate`（`installRegistryTemplates` 与重试提示读的是这两个全局量），已拿到 Target 时不再二次 `ResolveTarget`；`--overwrite/--append` 的校验仍在选单之后跑，选中的模板可与它们搭配（与上游“选完再定策略”一致）。
- 提问函数：包级可替换的 `promptSpecTemplate` / `promptCustomRegistrySource`（与 `promptPlatforms`/`promptStatusline`/`promptProjectType` 同一模式，测试替换它们）；中断/EOF 时返回错误，init 直接失败（与平台选择失败的处理一致）。
- 下载失败回退：沿用现有 `installRegistryTemplates`（安装失败时警告 + 写回空白 spec，不中断 init），与上游“Falling back to blank templates”一致。

### 有意不做（与上游的差别，记下来）

- 不做“`.sunshine/spec` 已存在时追问 Skip/Overwrite/Append”：全新 init 时 spec 目录不存在或为空，文件级 skip 策略不会误跳过；增量模式下已有内容时沿用现有的 flag 策略（默认 skip，打印“模板已存在，已跳过安装”），与现有 flag 驱动行为一致。
- 不做官方模板市场：Sunshine 没有托管的官方 registry，选单中间一档为空；想从市场装模板仍需 `--registry` 或选单里的自定义来源。
- 增量模式也走选单（与上游交互 init 一律出选单一致）；选空白在增量模式下是空操作（scaffold 本来就跳过），选模板则按现有增量路径安装。

### 实施检查（2026-11-10，已完成并实测验证）

- [x] `internal/registry/install.go`：新增 `ResolveTargetWithProbe`，`ResolveTarget` 委托给它（行为不变，现有测试覆盖）。
- [x] `internal/cli/init_template.go`（新文件）：包级可替换的提问函数 + `specTemplateChoices`（过滤 + 标签 `id（name）`，无 name 时只写 id）+ `resolveInteractiveTemplate`（循环选单/自定义/探测）。
- [x] `internal/cli/init.go`：`RunE` 接线（平台选择后、registry 校验前），Long 帮助补一句交互选模板说明。
- [x] 测试：选单构造（过滤、标签）；交互流程（本地 git 市场夹具，复用 `download_test.go` 的 `marketplaceIndex`/`marketplaceFiles` 模式，在 cli 测试里自建 git 夹具）：registry 市场选模板、直接下载、无 registry 选空白、自定义来源、空输入回选单、无效来源回选单、市场无 spec 模板改空白、提问出错向上传。
- [x] 全量 `gofmt` / `go vet` / `go test ./...`；真实二进制实跑：pty 交互三路（无 registry 选空白、`--registry` 市场选模板、自定义来源再选模板）+ 非交互回归（`-y` 空白、`-y` 市场无 `--template` 报错、`-y --template` 正常装）。

### 实施记录（2026-11-10）

- 改动（6 个文件，未提交）：`internal/registry/install.go`（`ResolveTargetWithProbe`，`ResolveTarget` 委托）；`internal/cli/init_template.go`（新，选单 + `resolveInteractiveTemplate`）；`internal/cli/init.go`（接线 + Long 说明）；`internal/cli/init_template_test.go`（新，10 个测试）；`internal/registry/download_test.go`（`TestResolveTargetWithProbe`）；PLAN.md（本节）。
- 接线细节：无平台选择时先取消（与上游“先验工具再选模板”的顺序一致，原来在 `case !initialized:` 里的检查上提到平台选择之后）；选单结果写回 `initRegistry`/`initTemplate`，已拿到 Target 时不再二次 `ResolveTarget`（探测只跑一次）；`--registry` 显式来源解析/探测失败直接报错退出，自定义输入的才回选单（与上游一致）；选单中断/EOF 返回错误，init 直接失败。
- 验证：全量 601 个测试全过（19 个包），`gofmt`/`go vet` 干净。真实二进制 pty 三路全过（Python pty 应答 survey 的 DSR 查询）：① 无 registry 选空白 → 四问（平台/状态栏/选模板/项目类型）后空白 spec 三件套；② `--registry` 市场 → 选单只列 `demo-spec（演示规范）`，装进 spec 的是模板的 backend/frontend（无默认 system.md），config.yaml 记 `registry.spec`（模板 demo-spec）；③ 自定义来源 → 下移到自定义、输入本地市场路径、再出模板选单选 demo-spec，同样装好。非交互回归四项全过：`-y` 空白、`-y` 市场无 `--template` 报错 exit 1（列出 demo-spec）、`-y --template` 正常装、管道 stdin（非 TTY）不带 `-y` 不出选单走默认。
- 验证中的插曲：pty 驱动第一版按累计输出匹配标记，重复出现的“请选择规范模板”在第二轮选单渲染前就误触发，导致自定义那路卡住超时；修成只匹配上一步之后的新输出后通过——是驱动 bug，不是产品 bug（单元测试里自定义那路本来就是通的）。
- 待用户定夺提交/发版——已随 v0.3.4 提交推送并发布（见下“v0.3.4 发布记录”）。

### v0.3.4 发布记录（2026-11-10）

- 内容：init 交互选模板（对齐 Trellis 的 "Select a spec template"，上一节的批次）+ 中英文教程与 README 同步维护。提交 `34aa005`（10 个文件），tag v0.3.4 为注释 tag（对象 `f493bd4`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.4，6 个资产；发布与 CI 均 success，无抖动（发布 run 42780012997、CI run 42780012688，约 2.5 分钟跑完）。
- 实测验证：用户原先那条不带参数的一键命令装上 v0.3.4（`Sunshine 版本 0.3.4`）；0.3.3 二进制 `upgrade --dry-run` 正确查到 0.3.4；darwin_amd64 包 checksum OK，解出 Mach-O x86_64 二进制。
- 插曲：本机 git 提交时报 Author identity unknown（沙箱环境无 user.name/user.email 配置），用 `-c user.name=BondLam -c user.email=BondLam@users.noreply.github.com` 显式指定提交，与之前各版作者一致；以后提交都这么带。
- 未复现的 FAIL 仍未复现（本轮两路 CI 全过，本地全量也全过）；若 CI 再抖，先抓完整日志再定。

## workflow inline 分支加平台守卫（2026-11-10，用户定做）

### 背景
用户在 Mac 上用 Claude Code（Opus 5）跑任务：任务已建、分支已切，到第 2 阶段实现时主会话没有派发 `sunshine-implement` 子代理，而是调了 `sunshine-before-dev` skill、主会话直接写代码。用户指出：开场提示词不该点名派发——派发是 workflow 自身的默认做法；问走 inline 是否合理。

### 核实结论
- inline 分支是 Codex 专用的：workflow.md 里 `[workflow-state:in_progress-inline]` 是“第 2/3 阶段的 Codex inline 变体”，只在 `codex.dispatch_mode=inline` 时出现；hook（`internal/hook/workflow_state.go`）只在 Codex 上才发 `-inline` 标签，非 Codex 平台原样返回派发版。Claude Code 会话收到的本来就是派发版，模型是自己拐进了 inline。
- 根因在 workflow.md 文本本身：inline 块内部没有任何平台限定词，“Codex 专用”只写在块外的 HTML 注释与状态表里，模型通读全文时容易把 inline 当作通用可选分支。
- inline 本身不是坏流程（before-dev → 改代码 → check → update-spec → 提交 → finish-work 链路完整），但在 Claude Code 上不是规定路线；代价是主会话上下文被实现细节占满、失去子代理隔离。

### 方案
只改 workflow 模板文字，不改 hook（hook 的路由已正确）：
- native 与 tdd 模板的 `[workflow-state:planning-inline]`、`[workflow-state:in_progress-inline]` 块内首行加守卫：仅 Codex 且 dispatch_mode=inline 时可用；Claude Code、Cursor 等支持子代理的平台一律走派发版，不得用本分支；in_progress-inline 守卫另加“不得以任务小、改动少为由改走 inline”。
- native 与 tdd 模板的 `[workflow-state:in_progress]` 派发句后加 reinforcement：不得以任务小、改动少为由改走 inline；inline 分支只属于 Codex。
- 教程 §4 的 2.1 摘要行同步加“仅 Codex”限定（中英文）。
- 有意不动 channel-driven 模板：它的 in_progress-inline 是设计好的有条件许可（“只有用户要求 inline，或改动太小不值得派工作线程时，才允许 inline 实现”，且复核仍走 channel 派 check 工作线程），不是 Codex 专用分支，改它等于改行为。
- 有意不动 `[codex-inline, Kilo, Antigravity, Devin, DeepSeek Harness]` 平台块：那些已按平台名正确限定，是不支持派发的平台的 inline 落点。
- 既有项目通过 `sunshine update` 刷新 workflow.md 获得新写法（哈希变化走正常三态流程）。

### v0.3.5 发布记录（2026-11-10）
- 内容：workflow inline 分支加平台守卫（native 与 tdd 模板 inline 块首行守卫 + 派发句 reinforcement；教程 §4 的 2.1 摘要行同步加“仅 Codex”限定）。提交 `b7896ac`（4 个文件），tag v0.3.5 为注释 tag（对象 `a18dbfb`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.5，6 个资产；发布 run 42789481102、两路 CI（tag 42789166884、push 42789481568）均 success 无抖动。
- 实测验证：不带参数的一键命令装上 v0.3.5（`Sunshine 版本 0.3.5`）；0.3.4 二进制 `upgrade --dry-run` 正确查到 0.3.5；darwin_amd64 包 checksum OK（Mach-O x86_64）。
- 注意：沙箱里之前装的 v0.3.4 二进制（/usr/local/bin/sunshine）在虚拟机更换后已不在，一键安装重新装上 v0.3.5。

## task start 成功后输出 in_progress 面包屑（2026-11-10，用户定做）

### 背景与核实结论
- 用户在 Mac 上实测：已升 v0.3.5、项目 workflow.md 已更新（与 v0.3.5 native 模板逐字节一致，守卫行 212/235/244 都在），但批准 → task start → 实现这一轮里主会话还是直接写代码，没派发 sunshine-implement；同一轮稍后的 check 阶段却正确派发了 sunshine-check 真子代理（Background agent 卡片可展开，底部 agent 列表可见）。
- 根因（源码级核实）：派发指令与守卫都在 `[workflow-state:in_progress]` 块里，该块只通过两条路进模型上下文——① agent 自己读 workflow.md，② UserPromptSubmit hook（`sunshine hook workflow-state`）在每条用户消息前注入当前状态的块。`task start` 成功后只打印“当前任务已设为”与“hook 现在会注入 jsonl 上下文”，不输出 in_progress 块；session-start hook 也只给指针（“Follow the matching per-turn workflow-state”），不给块正文。批准那条消息到达时状态还是 planning，hook 注入的是 planning 块；task start 之后没有新的用户消息，hook 不再触发，in_progress 块在那一轮里从未被递到模型面前。workflow.md 452 行“命令成功后，面包屑自动切换到 `[workflow-state:in_progress]`”的承诺在轮内并不成立。
- check 能派发的原因：check 派发指令在 workflow.md 里有多处（路由节“`in_progress` 的实现/检查 -> 派发 `sunshine-implement` / `sunshine-check`”、2.2 节“子代理类型：`sunshine-check`”），模型转到核查阶段时读到了；implement 的派发指令只在 in_progress 块与 2.1 节派发守则里，2.1 那一刻模型没重读、hook 又没注入，所以漏了。派发机制本身是通的，缺的是 2.1 那一刻的指令递送。
- 用户要求：task start 之后，后续的编码和 check 之类都要开启子代理处理（不追究机制细节，以结果为准）。

### 方案
只改 `sunshine task start` 的成功输出，不改 hook、不改 workflow 模板、不改派发路由：
- `internal/task/lifecycle.go` 的 `Start()` 在两个成功出口（正常模式与无会话身份的降级模式）都输出 in_progress 面包屑，复用 hook 现成的取块逻辑：`hook.LoadConfig`（读 `codex.dispatch_mode`）→ `hook.DetectPlatform(hook.HookInput{})`（`SUNSHINE_HOOK_PLATFORM` 与平台环境变量表；Codex 另认 `CODEX_THREAD_ID`，该变量在 hook 的 `envSessionKeys` 里已核实为 Codex 会话变量，`DetectPlatform` 的表里没有，start 内补一行判断）→ `hook.LoadBreadcrumbs`（读项目 `.sunshine/workflow.md`）→ `hook.ResolveBreadcrumbKey("in_progress", platform, cfg)`（仅 Codex + dispatch_mode=inline 时取 `-inline` 块，其余一律派发版）→ `hook.BuildBreadcrumb(任务目录名, "in_progress", templates, key)`，与 hook 注入的 `<workflow-state>` 块格式完全一致（任务 ID 用目录名，与 `GetActiveTaskForWorkflow` 一致）。
- 输出位置：正常模式放在“hook 现在会从该任务的 jsonl 文件注入上下文。”之后、`after_start` 任务钩子之前；降级模式放在写 `.current-task` 之后、返回之前。输出走 `r.errOut()`，与 Start 其余输出同一流。
- workflow.md 缺失或无 in_progress 块时退化为 `BuildBreadcrumb` 的通用行（“Refer to workflow.md for current step.”），与 hook 行为一致，不静默跳过。
- workflow.md 452 行文字不动：修完后“面包屑自动切换”成为事实，文字描述与行为一致。
- 有意不做：不在 `task start` 里硬拦 Write/Edit/Bash——主会话在 in_progress 阶段仍有合法写操作（任务日志、3.3 规范更新、3.4 提交），仅凭状态分不清 2.1 与 3.3，硬拦会误伤；本次修的是指令递送，派发仍由模型按 workflow 执行。

### 验证
- 新增测试（`internal/task/lifecycle_test.go`）：① workflow.md 含 in_progress 块时 start 输出含派发句；② workflow.md 缺失时输出通用行；③ Codex + `CODEX_THREAD_ID` + `codex.dispatch_mode=inline` 时输出 inline 块、不含派发句。
- 全量 `go test ./...`、gofmt/vet；真实二进制在临时项目实跑 `task start`，核对输出里有 `<workflow-state>` 与派发句。

### 实测发现的更深根因：面包屑扫描器 bug（修 task start 输出时发现）
- 现象：真实二进制实跑 `task start`，输出的面包屑是通用行（“Refer to workflow.md for current step.”），不是真实的 in_progress 块；随手一测，`sunshine hook workflow-state` 在真实项目里也只输出通用行。
- 根因：`internal/hook/workflow_state.go` 的 `scanWorkflowStateBlocks` 手工扫描器有两个与 Python 原版正则 `\[workflow-state:(STATUS)\]\n(.*?)\n\s*\[/workflow-state:\1\]` 不等价的地方：① 遇到没有闭标签的开标签就 `break` 整个循环（应跳过继续找）；② 不要求开标签独占一行，表格、注释、正文里的提及也被当成块的开始（Python 要求开标签后紧跟 `\n`）。真实模板（native/tdd/channel-driven 三份都有）的契约注释第 104 行就是无闭标签的 `[workflow-state:STATUS]` 占位写法，扫描到它就整个返回空表——**hook 从一开始就没能从真实模板里注入出任何真实块**，每轮注入的都是通用行。既有 hook 测试没发现，是因为夹具 workflow.md 是干净的最小 synthetic 文件，没有占位写法和表格提及。
- 修复（与 start 输出同批）：`scanWorkflowStateBlocks` 改为只有独占一行的开标签才算块的开始（行尾可带空格/制表符）、无闭标签的提及跳过不中断、闭标签须位于行首（仅允许前导空白），三点都与 Python 正则等价；同文件的 `stripBreadcrumbTagBlocks`（session-start 剥块用）是同一族 bug，一并改成同样的跳过逻辑（旧逻辑遇到 STATUS 占位就把剩余全文原样写出、再也不剥任何块，session-start 概览里会原样带上全部块正文）。
- 新增测试（`internal/hook/hook_test.go`）：① synthetic 内容（占位写法+表格提及+正文提及+两个真实块）只解析出两块；② 直接读真实 native 模板断言 7 个块都在、in_progress 块含派发句、no_task 块是真实短文案；③ 真实模板剥块后无 in_progress 正文、保留阶段正文。
- 标题对齐：start 输出的面包屑标题任务名与 hook 一致，优先用 task.json 的 `id` 字段、缺失时用目录名（`startTaskDisplayName`）；此前 start 用目录名、hook 用 id，两边标题不一致。
- 验证：全量测试全过（0 失败，新增 6 个：task 3 个 + hook 3 个）；真实二进制两路实跑——正常模式与降级模式的 `task start` 都输出真实 in_progress 块（含“主会话默认：派发 implement/check 子代理”与守卫句），标题与 hook 一致；`hook workflow-state` 同一会话也注入真实块。

### v0.3.6 发布记录（2026-11-11）
- 内容：task start 成功后输出 in_progress 面包屑（正常与降级两路，复用 hook 取块逻辑；仅 Codex + dispatch_mode=inline 时取 inline 块；标题任务名与 hook 一致）；修面包屑扫描器 bug（`scanWorkflowStateBlocks`：独占一行的开标签才算块、无闭标签的提及跳过不中断、闭标签须行首；同族的 session-start 剥块函数一并修）。提交 `74ea104`（6 个文件），tag v0.3.6 为注释 tag（对象 `2567fdc`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.6，6 个资产；发布 run 42955259493、两路 CI（tag 42955259500、push 42955247189）均 success 无抖动。
- 实测验证：不带参数的一键命令装上 v0.3.6（`Sunshine 版本 0.3.6`）；0.3.5 二进制 `upgrade --dry-run` 正确查到 0.3.6；darwin_amd64 包 checksum OK（Mach-O x86_64）。

## 设计先行模式：双模式分流 + 显性任务拆解（2026-11-11，用户定做）

### 背景与用户定夺
- 用户要把“产品设计至代码开发”打通，场景是：需求 → 设计文档 → 任务拆解 → 代码 → 核查；同时兼容没有设计环节的场景（代码 → 核查），模式不定。
- 2026-11-11 连续定夺（均以用户原话为准）：
  1. 交付物清单**不固定**（每个项目/设计者要的文档不一样，原型都可能不要）；设计阶段产出的文档放项目里单独的文档夹 `doc/`，**不在 `.sunshine`**；`.sunshine` 只存索引和指向（任务记录、prd.md、jsonl 清单记着文档在哪），开发进场时按索引去 `doc/` 里取，开发人员/agent 自主按项目情况决定怎么接手。
  2. 任务拆解**不单独做命令**：拆解分两部分——“怎么拆”是判断（agent 读设计文档决定，命令帮不上），“建子任务”是机械活（`task create --parent` 与 `task add-subtask` 已有）。工作流里把拆解立成显性步骤 + skill 规则就够了。
  3. 教程要把这个场景写进去（新增 §12.6 walkthrough），让用户看到整条链是怎么串起来的。

### 设计要点
- **双模式**：设计先行（需求 → 设计文档 `doc/` → 任务拆解 → 开发）与直接开发（需求 → prd → task start → 开发）并行。模式在 1.0/1.1 开头定：agent 问一句，或看 `doc/` 里是否已有设计文档；开场提示词里已说清的不重复问。
- **原型不强制**：`doc/` 里有什么就用什么——有原型按原型拆，有需求列表按需求列表拆，只有 PRD 就按 PRD 拆；什么设计文档都没有就是纯代码任务，走直接开发模式。
- **开发进场不重复读全量**：原型只在规划时读一次（拆任务、编每个子任务的包时）；每个子任务只读自己的包（自己的 implement.jsonl 指针 + prd.md + design.md + implement.md）。1.3（新编号 1.4）配置上下文是编包的步骤。
- **产品设计文档 vs 技术设计文档**：产品设计文档（需求列表、概要设计、高保真原型等）放项目 `doc/`，设计人员产；任务目录里的 `design.md` 是复杂任务的技术设计，开发视角。两者别混淆。
- **父/子任务**：父任务拥有原始需求、子任务映射、跨子任务验收标准与最终集成复核；子任务拥有可独立规划、实现、检查、归档的实际交付物；父/子不是依赖系统（顺序写进子任务 prd/implement）；除非父任务自身有直接实现工作，否则不要启动父任务；逐个启动拥有下一个交付物的子任务。

### 编号方案：插入新 1.2，全员后移
- 新顺序：第 1 阶段 1.0 创建任务 → 1.1 需求探索 → **1.2 任务拆解 `[optional · once]`** → 1.3 研究（原 1.2）→ 1.4 配置上下文（原 1.3）→ 1.5 激活任务（原 1.4）→ 1.6 完成标准（原 1.5）。
- 1.2 放 1.1 之后、1.3 研究之前：拆解的输入是 1.1 产出的 prd.md + `doc/` 设计文档；拆解后，研究按子任务做更聚焦，拆解前需要的调研在 1.1 的证据检查里做。
- 1.2 只在需要拆时跑：轻量任务、单交付物任务跳过。触发条件：一个任务包含多个可独立验证的交付物，或工作量明显超出单次实现。
- 1.2 的完成标准：每个子任务独立可验证、有明确验收标准；父任务记好子任务映射与跨子任务验收标准；子任务都已建好（task.json 的 parent/children 双向一致）；子任务随后各自走 1.1（自己的 prd/design/implement）→ 1.4（自己的 jsonl）→ 1.5（自己的 task start）→ 第 2 阶段。

### 改动清单（CLI 不动）
1. **三份 workflow 模板**（native、tdd、channel-driven-subagent-dispatch，全文中文）：
   - 第 1 阶段索引：插入 1.2 任务拆解，后面各项重编号（1.2→1.3、1.3→1.4、1.4→1.5、1.5→1.6）。
   - 各节标题重编号；正文交叉引用同步改：native/tdd 的“`start` 留到步骤 1.4”→1.5、“做研究（同第 1.2 步）”→1.3。
   - 1.1 需求探索：加双模式分流文字（设计先行 vs 直接开发、怎么定）；加设计文档落盘 `doc/` 的约定；原“范围太大时拆成父任务加子任务”改为指向 1.2（native/tdd 的“考虑父/子拆分时”规则块移到 1.2；channel-driven 的拆分 bullet 改为指向 1.2）。
   - 新增 1.2 任务拆解节：触发条件、输入、做法（`task create --parent` / `task add-subtask`）、父/子规则、完成标准。
   - 1.4 配置上下文（原 1.3）“放什么”加一类：**设计文档**——`doc/` 下与本任务相关的设计产物（需求列表、概要设计、高保真原型等），只放本任务用得上的那几份，不用把整个 `doc/` 列进来。
   - 1.5 激活任务（原 1.4）：加需求评审措辞（激活前把整包规划产物与拆解出的子任务列出来请用户确认，确认后才 `task start`）；加父任务不激活规则（除非自身有直接实现工作），逐个激活子任务，全部完成后父任务做最终集成复核。
   - 1.6 完成标准（原 1.5）：只重编号，表内容不动。
   - **红线**：不动 `[workflow-state:...]` 面包屑块结构；新增文字里不出现独占一行的 `[workflow-state:...]` 提及（扫描器只认独占一行的开标签，见 v0.3.6 修复）。
2. **`templates/platforms/common/commands/continue.md`**：路由文字里的步骤号同步改（1.4→1.5、1.3→1.4）。
3. **`templates/platforms/common/bundled-skills/sunshine-meta/references/local-architecture/workflow.md`**：例子里的 `1.3 Configure context`→`1.4 Configure context`。
4. **`templates/platforms/common/skills/brainstorm.md`**：
   - 规划流程开头加定模式一步：问用户这次是否设计先行，或检查 `doc/` 是否已有设计文档；开场已说清的不重复问。
   - 需求收敛后加拆解触发：需要拆解时走 1.2 任务拆解（读 `doc/` 设计文档 → 定子任务 → `create --parent` / `add-subtask` → 父任务记映射与跨子任务验收标准）。
   - 产物规则加产品设计文档 vs 技术设计文档的区分：产品设计文档落盘项目 `doc/`（首次落盘时创建目录），不在 `.sunshine`；路径登记进 prd.md 参考文档，并在 1.4 配置上下文时编进 implement.jsonl/check.jsonl。
5. **中英文教程**（`docs/tutorial.md`、`docs/tutorial.en.md`）：
   - §3 步骤列表：重编号 + 加 1.2 任务拆解行 + 1.1 行提双模式（英文 §3 无步骤号，镜像改法即可）。
   - §4：如有步骤号引用一并改（中文 §4 实测输出里的文字不动）。
   - §12 新增 **12.6 设计先行：需求 → 设计文档 → 拆解 → 开发** walkthrough：以退款演示项目为底，开场提示词（设计先行四要素）→ brainstorm → prd.md → 设计文档落盘 `doc/`（概要设计 + 高保真原型 refund.html）→ 需求评审 → 拆解成父任务 + 2 个子任务（`create --parent` 实跑）→ 子任务配上下文 → `task start` → 说明后续与 §2/§3 相同、最后父任务集成复核。命令与输出用新二进制在 `/tmp/scenes/s6-design` 实跑取真实值；§12 标题与引言“五个场景”→“六个场景”，表格加第 6 行。§12 引言现有的“本节用当前 Release v0.3.0 验证”不动（已知滞后，见 MEMORY；§12.6 的输出来自本次新编二进制，尚未发版）。
   - 六处开场提示词一个字不动（§2 + 五个场景）；新增 §12.6 的开场提示词按四要素写。
6. **README**（`README.md`、`README.en.md`）：“五个场景”→“六个场景”，场景表加第 6 行（锚点指向教程 §12.6）；日常四步段落加一句设计先行指引。
7. **不在本次范围**：设计文档模板（需求列表/概要设计/原型各一份模板）——用户定：不做，需要的话让设计人员去模板市场下载；`doc/` 目录不在 init 时创建，首次落盘设计文档时由 agent 创建。

### 验证
- `go build ./...`、`gofmt -l`、`go vet ./...`、全量 `go test ./...` 全过。
- 全仓 grep 无残留旧步骤号（“步骤 1.4”、“同第 1.2 步”、“1.3 配置上下文”、“1.4 激活任务”等）。
- 真实二进制：`init` + `workflow --template <三模板>` 落盘检查（新 1.2 节在、编号连续、模式分流与设计文档类在）；`hook workflow-state` 仍注入真实面包屑块（7 个块都在）；§12.6 全部命令在 `/tmp/scenes/s6-design` 用新编二进制实跑，输出进教程。
- 锚点脚本核过（README 新锚点 + 教程 §12.6 锚点）。

### 实施记录（2026-11-11，已完成，未提交）
- 改动 11 个文件（+524/-60）：三份 workflow 模板、continue.md、sunshine-meta 架构文档、brainstorm skill、中英文教程、中英文 README、PLAN.md。CLI 未动（用户定夺：不用新命令）。
- 编号：新 1.2 任务拆解插入，全员后移（研究→1.3、配置上下文→1.4、激活→1.5、完成标准→1.6）；交叉引用全改（“留到步骤 1.4”→1.5、“同第 1.2 步”→1.3、continue.md 路由、sunshine-meta 例子）；新增 continue.md 路由一行：规划产物齐全但多交付物未拆解 → 1.2，拆完后不在父任务上走 1.4/1.5。
- 内容：1.1 双模式分流（设计先行 vs 直接开发）+ 设计文档落盘 `doc/` 约定；1.2 任务拆解（触发条件、输入、做法 `create --parent` / `add-subtask`、父/子规则、完成标准）；1.4 配置上下文“放什么”加设计文档类；1.5 激活任务加需求评审措辞 + 父任务不激活规则；brainstorm skill 加定模式、拆解触发、产品设计文档 vs 技术设计文档区分。
- 教程：§3 步骤列表重编号 + 1.2 行 + 双模式；§12 新增 **12.6 产品设计先行** walkthrough（退款演示项目：开场提示词 → prd → `doc/design` + `doc/prototype` → 需求评审 → 父任务 + 2 子任务 → 子任务配上下文 → `task start` → `task list`）；§12 标题与引言“五个场景”→“六个场景”；六处旧开场提示词一个字没动（git diff 只多出 §12.6 的 1 行提示词）。README 中英文“五个场景”→“六个场景” + 场景表第 6 行。
- 验证：`go build`、`gofmt`、`go vet` 干净，全量测试全过 0 失败（19 个包）；全仓 grep 无残留旧步骤号；锚点脚本核过 BAD: 0（含同文件锚点，补修了英文教程第 5 行的 `#12-five-scenarios` 断链）；真实二进制三模板落盘检查（init + `workflow --template tdd/channel-driven` 均含新 1.2 节与重编号）；§12.6 全部命令在 `/tmp/scenes/s6-design` 用新编二进制实跑（init、父任务创建、两个 `--parent` 子任务、add-context、list-context、`task list` 父子嵌套显示、`task start` 降级与正常两路均输出真实 in_progress 面包屑、`hook workflow-state` 注入真实块）；`task start` 对无上下文的子任务正确拒绝（jsonl 门禁生效）。`--allow-empty-context` 经核实只在清单为空时生效，教程里 `task start` 不带该 flag，输出与实跑一致。
- 已随 v0.3.7 提交推送并发布（见下“v0.3.7 发布记录”）。设计文档模板不做：用户定需要的话让设计人员去模板市场下载。教程 §12 引言“本节用当前 Release v0.3.0 验证”仍滞后（已知；§12.6 输出来自 v0.3.7 代码，下次改文档时一并处理）。

### v0.3.7 发布记录（2026-11-11）
- 内容：设计先行模式——三份 workflow 模板第 1 阶段新增 1.2 任务拆解（optional），研究/配置上下文/激活/完成标准后移至 1.3-1.6；1.1 加双模式分流（设计先行 vs 直接开发）与设计文档落盘 `doc/` 约定；1.4 配置上下文加设计文档类；1.5 激活任务加需求评审措辞与父任务不激活规则；continue.md 路由与 sunshine-meta 例子同步改号；brainstorm skill 加定模式、拆解触发、产品设计文档 vs 技术设计文档区分；教程新增 §12.6 设计先行 walkthrough（退款演示项目，实跑），中英文教程与 README 由五个场景改为六个场景。CLI 未动（用户定夺：不用新命令）。提交 `ceaf552`（11 个文件），tag v0.3.7 为注释 tag（对象 `279c616`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.7，6 个资产；发布 run 42967234680、两路 CI（42967234661、42967209609）均 success 无抖动。
- 实测验证：不带参数的一键命令装上 v0.3.7（`Sunshine 版本 0.3.7`）；0.3.6 二进制 `upgrade --dry-run` 正确查到 0.3.7；darwin_amd64 包 checksum OK（Mach-O x86_64）。
- 用户定：设计文档模板不做，需要的话让设计人员去模板市场下载。

## init 产品设计类型：空白模板选产品设计时生成 doc/ 设计文档模板（2026-11-11，用户定做）

### 背景与用户定夺
- 2026-11-11 早些时候用户定“设计文档模板不做，需要的话让设计人员去模板市场下载”（已随 v0.3.7 发布记录）。同日用户改主意：设计文档模板（需求列表、概要设计、原型各一份）还是要有，放在 `sunshine init` 选空白模板时——项目类型提问（前端/后端/全栈）里加多一个“产品设计”选项。
- 选“产品设计”时生成什么，用户在 options widget 上点选：**只生成 doc/ 模板 + 通用规范**（不生成前后端规范；技术栈待定，以后定了再 `init -f` 补）。

### 设计要点
- **新类型 `projecttype.Design`**：不参与自动检测（没有任何代码信号能检测出“产品设计”），只能手动选——交互提问或 `--type 产品设计`（另接受英文 `design`）。`String()` 为 `design`，`DisplayName()` 为“产品设计”。
- **只在空白模板路径出现**：项目类型提问只在需要生成默认 spec 时跑（`--registry` 写 spec 时不检测、不提问，直接默认全栈），与用户说的“选择空白时”一致。`-y` / 非交互仍默认全栈，不带设计模板。
- **生成内容**：`doc/design/需求列表.md`、`doc/design/概要设计.md`、`doc/prototype/原型.html`（三份，中文，占位用“（请填写：……）”写法；落盘路径沿用教程 §12.6 的 `doc/design/` + `doc/prototype/` 约定）。spec 只写 `system.md`（guides 照常），不写 `frontend.md` / `backend.md`。
- **doc/ 是设计人员的交付物，不是模板管理文件**：三份模板不进 `.sunshine/.template-hashes.json`，`sunshine update` 不碰它们；写入时只写不存在的文件——已填过的内容绝不覆盖，`init -f` 重建也不覆盖，只补写缺失的。
- **模板内嵌**：`templates/design/` 独立 embed FS（`templates.Design`，与 `.sunshine/` 骨架的 `templates.Sunshine` 分开）；源文件名用英文（`requirements.md`、`outline.md`、`prototype.html`），落盘目标用中文名。

### 改动清单
1. **`internal/projecttype/projecttype.go`**：新增 `Design`；`String()` / `DisplayName()` / `Parse()` 跟进；`Parse` 非法值文案合法值加“产品设计”；包注释说明产品设计不参与检测。
2. **`templates/templates.go`**：新增 `Design` embed FS（`all:design`）。
3. **`templates/design/`（新建）**：`requirements.md`（产品信息、用户角色、功能需求表 R-001 起、非功能需求、待确认、变更记录）、`outline.md`（目标与范围、用户与场景、模块划分、关键流程、接口、数据、验收、风险）、`prototype.html`（可直接用浏览器打开的起手页：标题、导航、第一屏、示例表单、列表示例，内联样式只为看清结构）。
4. **`internal/scaffold/scaffold.go`**：`InitWithOptions` 里 Design 时前后端 spec 都跳过；新增 `designTemplates` 映射表与 `writeDesignTemplates`（只写不存在的文件），在 `InitWithOptions` 末尾调用。
5. **`internal/cli/init.go`**：`promptProjectType` 选项加“产品设计”（默认仍是“前后端全栈”）；`--type` flag 与 Long 说明合法值加产品设计；`specNextStepLine` 加 Design 分支（提 system.md 与 doc/ 三份模板）。
6. **中英文教程**（`docs/tutorial.md`、`docs/tutorial.en.md`）：附录 init 参考——第 4 问加产品设计选项与含义、检测段加“产品设计不参与检测”、`--type` 表格行加产品设计。

### 验证
- `go build ./...`、`gofmt -l` 无输出、`go vet ./...` OK、全量 `go test ./...` 全过 0 失败（19 个包）；新增测试：`projecttype` 的 Design 解析/展示名、`resolveProjectType` 的 `--type 产品设计` 与提问选产品设计两路、`specNextStepLine` 的 Design 分支、scaffold 的 Design 落盘 + 不进哈希清单 + 非 Design 不写 doc/ + `-f` 不覆盖已填内容只补缺失、CLI 端到端 `--type 产品设计`。
- 真实二进制三路实跑：① pty 交互四问全过，类型选单现 4 个选项，默认全栈下移一项选中产品设计，输出“项目类型：产品设计（手动选择）”；② `--type 产品设计`（-y）同样落盘；③ `-f` 重建：已填的 需求列表.md 内容保留、删掉的 原型.html 补写回来。两路的 spec/ 都只有 system.md，doc/ 三份都在，哈希清单无 doc/ 条目；原型 HTML 用解析器验过结构无错。
- 注意：第一版原型模板用的【产品名称】等全角括号占位写法在写盘时被整段去掉（落盘后只剩空标签），改用“（请填写：……）”写法后正常；以后写模板文件不要用【】占位。

### 实施记录（2026-11-11，已完成，未提交）
- 改动 9 个文件 + 3 个新模板文件：projecttype、scaffold、init、templates.go、两份项目类型测试、scaffold 测试、中英文教程附录。CLI 除 init 外未动；workflow 模板未动。
- 已随 v0.3.8 提交推送并发布（见下“v0.3.8 发布记录”）。

### v0.3.8 发布记录（2026-11-11）
- 内容：init 产品设计类型——项目类型提问加第四个选项“产品设计”（默认仍全栈）；选它只生成通用规范 system.md（guides 照常），不写前后端规范，另写 doc/ 设计文档模板：doc/design/需求列表.md、doc/design/概要设计.md、doc/prototype/原型.html。新增 projecttype.Design（不参与检测，只能手动选；--type 也支持产品设计/design）。doc/ 模板不进哈希清单，update 不碰；只写不存在的文件，已填内容绝不覆盖，init -f 重建也不覆盖，只补写缺失的。中英文教程附录 init 参考同步。提交 `5b8970b`（13 个文件），tag v0.3.8 为注释 tag（对象 `c4eafb7`）。
- Release 已发布：https://github.com/BondLam/Sunshine/releases/tag/v0.3.8，6 个资产；发布 run 42970039441、两路 CI（42970039335、42969768358）均 success 无抖动。
- 实测验证：不带参数的一键命令装上 v0.3.8（`Sunshine 版本 0.3.8`）；0.3.7 二进制 `upgrade --dry-run` 正确查到 0.3.8；darwin_amd64 包 checksum OK（Mach-O x86_64）。注：验证时沙箱 /usr/local/bin 里的 0.3.7 二进制已被 VM 更换清掉（/usr/local/bin 不跨 VM 持久），upgrade 检查改用从 v0.3.7 Release 现下载的原版二进制跑的，结果一样。
- 权限讨论（未定）：用户问“只让授权的人用 sunshine”能不能做。已答：能做，三条路（仓库改私有 / CLI 加授权校验 / 两者结合），并说明公开源码的前提下纯 CLI 校验防不住改源码重编；v0.3.7 及之前已公开发出去了，收不回来，权限只能管以后的版本。用户选继续发 v0.3.8；授权给哪些人（团队成员 / 指定 GitHub 账号 / 付费用户）还没定，等用户再说。

## License 验证：签名 license 文件 + 每日联网核查（2026-11-11，用户定方向，未实施）

### 背景与用户定夺
- 用户定 Sunshine 未来方向：仓库肯定要迁到私有、源码不公开；调用 sunshine 时要求验证 license。对内使用，担心离职后继续用。
- 机制讨论经过（2026-11-11）：先给的推荐是离线签名的 license 文件 + 本地验签（Ed25519），不用服务器；用户问“只让授权的人用”能不能做时答了三条路（仓库改私有 / CLI 加授权校验 / 两者结合），并说明公开源码的前提下纯 CLI 校验防不住改源码重编。用户又问签发机制怎么工作，解释后用户定：先把这套写进 PLAN.md。
- 关键结论：**光靠离线 license 解决不了离职问题**——发出去的 license 只能等过期，离职的人最长能用到有效期满。所以离线验签之外必须加一道每日联网核查（组织成员或 allowlist），离职 = 移出组织 / 从 allowlist 删掉，第二天就用不了；断网宽限 7 天兜底。

### 设计要点
- **License 文件**：JSON，字段 `id`（随机）、`name`、`email`、`issued_at`、`expires_at`、`signature`。签名是对除 `signature` 外全部字段的规范序列化做 Ed25519 签名（base64）。文件放 `~/.sunshine/license.json`（与 `~/.sunshine/channels/`、`~/.sunshine/ablations/` 同一约定）。
- **密钥对**：`sunshine license keygen` 只做一次，生成私钥（只放签发者机器上，最好再备份一份在密码管理器里，绝不进 git、绝不进仓库）与公钥（嵌进二进制，随发版走；内嵌位置待定，如 `internal/license/` 下的 Go 常量或 pem 文件）。私钥只能签名，公钥只能验签——公钥公开了也签不出有效的 license；签名能验过 ⟺ 持有私钥的人签发 ⟺ 只有签发者能发 license。`keygen` 做一次，`issue` 想发多少发多少（每份内容不同，签名也不同，都用同一个公钥验）。
- **签发**：`sunshine license issue --name <名字> --email <邮箱> --expiry <天数/日期>`，读私钥文件签名，输出 license 文件；签发者把文件发给同事，同事跑 `sunshine license activate --file license.json` 装进 `~/.sunshine/license.json`。`license status` 显示持有人与有效期（不拦），`license` 相关命令与 `version` / `--help` 不验 license（否则连看都看不了）。
- **验签（每次调用都验）**：CLI 统一入口拦一道（cobra 统一入口，所有子命令都过它，只放行 version/help/license），分两步——① 用内嵌公钥验签名，证明整份内容（包括有效期）没被改过，改一个字签名就失效；② 拿当前时间跟 `expires_at` 比，过了就拒绝并提示找签发者续签。Ed25519 验签一次几十微秒，无感。默认有效期 90 天，到期前 30 天起提醒。
- **每日联网核查（吊销）**：每天第一次调用时联网查一次“这个人还在不在”，结果缓存 24 小时。两个来源二选一（待定，取决于团队有没有 GitHub org；仓库目前在个人账号 BondLam 下，未见 org）：① GitHub 组织成员身份——离职就是移出组织，自动吊销，用 `gh` 已登录的 token 或一个只读 token；② 私有仓库里的 allowlist 文件——离职就是从文件里删掉那一行。断网时用缓存，最多宽限 7 天；超过 7 天没核查过就拒绝。时间来源也以核查那一步的服务器时间为准，本机时钟往回拨最多拖 7 天。
- **最坏情况**：离职的人天天联网，第二天就用不了；故意一直断网，7 天后停；有效期 90 天只是兜底。
- **密钥轮换**：私钥泄露/丢失 → 重新 keygen → 重新发版（新公钥）→ 给所有人重发 license；旧 license 随之作废（新公钥验不过旧签名）。
- **与迁私有的关系**：license 机制先做（公开仓库也能跑）；迁私有那天，一键安装与 `sunshine upgrade` 要改成带 token 拉取（私有仓库的 raw 与 release 都要认证），顺手一起改。迁私有后，已装到别人机器上的旧二进制仍不受 license 约束之外的管控——管住以后版本靠 license，旧版本收不回来（v0.3.8 及之前已公开发出去了）。

### 改动清单（待实施）
1. **`internal/license/`（新建）**：Ed25519 签名/验签、license 文件读写（`~/.sunshine/license.json`）、有效期检查、核查缓存（`~/.sunshine/license-check.json`，含核查时间与结果）。
2. **`sunshine license` 命令组（新建）**：`keygen`（生成密钥对，私钥写指定路径，默认 `~/.sunshine/license-admin.key`）、`issue`（读私钥签发）、`activate --file`（装 license）、`status`（持有人/有效期/上次核查时间）。
3. **CLI 入口验签**：统一入口拦一道，验签 + 有效期 + 每日联网核查（组织成员或 allowlist，二选一）；放行 `version` / `--help` / `license` 相关命令；拒绝时给明确提示（缺 license / 签名无效 / 已过期 / 不在授权名单 / 超过 7 天未核查）。
4. **公钥内嵌**：发版时把公钥编进二进制；`.gitignore` 加私钥文件名，私钥绝不进仓库。
5. **文档**：教程加一节 license（签发、激活、续签、离职吊销、常见报错）；README 提一句。

### 验证（待实施）
- 单元测试：签发→验签 round-trip；改一个字验签失败；过期拒绝；缺 license 拒绝；私钥丢了轮换后旧 license 失效。
- 端到端：真实二进制跑 `keygen` → `issue` → `activate` → 正常命令放行；缺/坏/过期 license 时各命令拒绝，`version` 与 `license status` 不拦；核查缓存 24 小时内不重复联网（断网也能跑），超过 7 天未核查拒绝。
- 数字（90 天有效期、30 天提醒、每天核查一次、7 天宽限）都是默认值，可调；实施前与用户确认。

### 实施记录
- 未开始：等用户定夺实施（先写 PLAN.md）。

## init 产品设计类型改成全栈规范（2026-11-11，用户改主意，未提交）

### 背景
- v0.3.8 做“init 产品设计类型”时用户点选“只生成 doc/ 模板 + 通用规范”：`doc/` 三份模板 + `system.md`，不生成 `frontend.md` / `backend.md`（理由是技术栈待定，以后定了再 `init -f` 补）。
- 2026-11-11 用户改主意（原话）：“选择产品设计，不仅仅 doc/ 模板 + 通用规范，还要加上全栈规范（system+前后端）这个才合理”。即：选产品设计时，规范按全栈生成（三份全有），外加 `doc/` 三份模板。
- 注意：这是同一天内的决策反转，不是 v0.3.8 的实现错了；v0.3.8 的记录不动，改动另记。

### 改动
1. `internal/scaffold/scaffold.go`：`InitOptions` 注释改成 Design 与 Fullstack 一样三份全写；walk 里 `frontend.md` / `backend.md` 的跳过条件去掉 `|| SpecType == Design`（Design 不再跳过前后端 spec）；`writeDesignTemplates` 不动（doc/ 三份照写、不进哈希清单、-f 不覆盖只补缺失）。
2. `internal/cli/init.go`：`specNextStepLine` 的 Design 分支改成“已生成 system.md、frontend.md、backend.md”+ doc/ 一句。
3. `internal/projecttype/projecttype.go`：Design 注释改成“生成全套规范（与全栈相同），另写 doc/ 设计文档模板”。
4. 测试：`TestInitDesignWritesDocTemplates`（spec 与哈希断言由 `(false, false)` 改 `(true, true)`）、`TestSpecNextStepLine` 的 Design 期望值、`--type 产品设计` 端到端（`assertInitSpecFiles` 由 `(false, false)` 改 `(true, true)`）。
5. 中英文教程附录 init 参考第 4 问同步：产品设计生成全套规范 + `doc/` 模板。

### 验证
- `go build` / `gofmt` / `go vet` 干净，全量 `go test ./...` 全过 0 失败（19 个包；gofmt 曾报 `scaffold_test.go` 一处注释空格，已修）。
- 真实二进制实跑 `--type 产品设计`：spec/ 三份全有，doc/ 三份全有，“下一步”第 2 行写三份规范 + doc/ 一句；哈希清单含三份 spec、不含 doc/。

### 实施记录
- 已随 v0.3.9 发布（提交 dc3863f，tag 对象 cb50671）。

## 需求变更处理（2026-11-11，用户问后定做，已随 v0.3.9 发布）

### 用户的问题
设计先行流程（需求 → 设计文档 → 拆解 → 开发）走到开发后，需求可能再次变更（新增或变更），怎么处理。

### 定下的处理（写进工作流模板 1.5 与 brainstorm skill）
- 不走新命令，走一次只针对变化部分（delta）的回炉：回 1.1 问清变化点（只问 delta，不重问已定的）→ 按顺序更新事实来源：`prd.md` 里记一笔 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）→ `doc/` 设计文档（概要设计、原型）同步改 → 受影响的子任务改 `prd.md` / `design.md` / `implement.md`，范围用 `sunshine task set-scope` 改、新文件用 `sunshine task add-context` 加进清单 → 回 1.5 只评审变更的部分，评审通过再动手。
- 定波及面时对照父任务的子任务映射：没被波及的子任务不停；还在 planning 的直接改；已经 in_progress 的先停下手头的活，回炉完再继续（状态不用改）；已经归档的不 reopen（归档的是历史），新建子任务挂在父任务下走完整流程，描述里写清原任务与变更原因。新增需求同理：迷你一轮 1.1 → 1.2 → 1.4 → 1.5，新子任务挂父任务下。
- 全做完已归档后才来的变更：就是一个新任务，描述里链回原任务。
- 改动位置：三份 workflow 模板 1.5 末尾（“父任务不激活……”行后）各加一段“需求变更（开发中途）”；brainstorm skill 规划流程第 10 步后加一段“需求变更”。不加步骤号（1.0–1.6 不变），新增文字里无 `[workflow-state:...]` 提及。

### 教程 §12.6 续一段变更示例（真实跑通）
- 例子：项目做完归档后，上限 5000 → 3000。演示目录 `/tmp/scenes/s6-design`（接上轮的最终状态，0 活动、3 归档）。
- 流程：新任务 `11-11-退款上限改成-3000-元`（create --description 链回原任务）→ prd.md 记 `[变更 2026-11-11]` → 概要设计接口行与原型 `max="3000"` 同步改 → add-context 三条（server.js、概要设计进 implement，backend.md 进 check）→ start → server.js 第 17 行 `> 5000` 改 `> 3000` → curl 验新边界（3001 → NEED_APPROVAL；3000 → 200，记录 id 3）→ backend.md 规范更新 → 提交 `8d09672` → finish → archive --skip-branch-validation → list-archive 4 个任务。
- 演示要点：旧的三个任务原样留在归档里（不 reopen），变更单独一个任务；doc/ 是活文档跟着改，已归档的任务记录不动。
- 注意：实跑时 `pkill` 式停服务会误杀自己的 shell（上轮教训）；这次用 `nohup node` 起服务，停服务时 `process.kill` 杀会话树，事后查 3000 端口无人监听才确认已停。

### 已知缺口（实话）
- 上下文清单只能加不能删（CLI 没有 remove-context）：变更后失效的条目只能手动改 jsonl，或在 `prd.md` 里注明不用读哪份。已写进工作流规则与教程，不藏着。

### 验证
- `go build` / `gofmt` / `go vet` 干净，全量 `go test ./...` 全过 0 失败（19 个包）。
- 真实二进制三模板落盘检查：native、tdd、channel-driven 的 workflow.md 里各有 1 处“需求变更（开发中途）”；brainstorm skill 落盘有 1 处。channel-driven 切换时因 workflow.md 已改被拒，属 `workflow --template` 的哈希守卫设计行为，加 `--force` 后换模板成功。
- 教程核对：文档级标题中英文各 36/34 行、改前改后一致；删除行只有四行（§12.6 两句旧总结是上轮已汇报的改写、附录两行产品设计改写），七处开场提示词一个字没动。

## v0.3.9 发布记录（2026-11-11）

- 内容：init 产品设计改成全栈规范（spec 按全栈生成 + doc/ 三份模板）+ 需求变更处理进工作流与教程 §12.6（delta 回炉规则进三份 workflow 模板 1.5 与 brainstorm skill；§12.6 续一段变更示例，上限 5000→3000 真实跑通，提交 8d09672）+ 上轮 §12.6 开发一段补齐（前端/后端子任务完整流转，提交 729fa17、aef98d6）。主提交远端 `dc3863f`（12 个文件），tag 对象 `cb50671`。
- Release：https://github.com/BondLam/Sunshine/releases/tag/v0.3.9，6 个资产；发布 run 42979134926、两路 CI（42979134941、42979097187）均 success 无抖动。
- 实测：不带参数一键装上 v0.3.9（`sunshine version` = 0.3.9）；v0.3.8 原版二进制 upgrade --dry-run 查到 v0.3.9；darwin arm64 包 checksum OK（327da167…）、Mach-O arm64。

## §12.6 例子纠正：设计包做完才拆解，一次拆完（2026-11-11，已随 v0.3.10 发布）

### 用户的纠正
- 原例子读起来像“做好一个页面就拆一次任务、边设计边开发”；用户定：正常产品设计人员是把所有产出物都产出了，开发才进厂——高保真原型做出来后，开发才开始做；拆解只做一次，拆的是整个范围。
- 注意：原例子本身的流程顺序（设计文档落盘 → 评审 → 拆解 → 开发）没错，错在没把“所有产出物先做完、一次拆完整个范围”写死，单页演示项目容易被读成按页拆分。

### 改动
1. 教程 §12.6（中英文）三处：引言（设计人员先把所有产出物都做完，评审通过后开发人员接手；拆解只做一次，拆整个范围）；设计包一段（演示项目只有一个页面所以设计包是一份概要设计 + 一个原型，真实项目里先把所有产出物做完、评审通过后才拆解、开发才进场）；拆解过渡句（一次性拆解，子任务清单一次定好）。
2. 三份 workflow 模板 1.2 各加一段“拆解只做一次”（评审通过后读齐 `doc/` 全部设计文档、一次性定好子任务清单；不要做好一个页面拆一次、边设计边拆——开发进场时设计包必须是完整的）。步骤号不动，新增文字里无 `[workflow-state:...]` 提及。
3. brainstorm skill 第 8 步加“一次拆完整个范围，不要做好一个页面拆一次”。
4. 另含上轮的 §12.6 顺序说明（拆解完不自动开发，评审通过、逐个激活才开始；前端归档后后端才启动），同批未提交。

### 验证
- `go build` / `gofmt` / `go vet` 干净，全量 `go test ./...` 全过 0 失败（19 个包）。
- 真实二进制三模板落盘检查：native、tdd、channel-driven 的 workflow.md 里各有 1 处“拆解只做一次”；brainstorm skill 落盘有 1 处“一次拆完整个范围”。
- 教程核对：删除行只有被改写的旧措辞（本轮 6 行 + 上轮 4 行），七处开场提示词一个字没动，标题未变。

## v0.3.10 发布记录（2026-11-11）

- 内容：§12.6 例子纠正——设计包做完才拆解，一次拆完（教程 §12.6 引言/设计包/拆解过渡三处同步；三份 workflow 模板 1.2 加“拆解只做一次”，brainstorm skill 第 8 步同步）+ 上轮 §12.6 顺序说明（拆解完不自动开发，评审通过、逐个激活才开始；前端归档后后端才启动）。主提交远端 `c772558`（7 个文件），tag 对象 `b4b90c5`。
- Release：https://github.com/BondLam/Sunshine/releases/tag/v0.3.10，6 个资产；发布 run 42982006525、两路 CI（42982006622、42981987667）均 success 无抖动。
- 实测：不带参数一键装上 v0.3.10（`sunshine version` = 0.3.10）；v0.3.9 原版二进制 upgrade --dry-run 查到 v0.3.10；darwin arm64 包 checksum OK（9207a76a…）、Mach-O arm64。

## §12.6 重写：全新项目请假申请小工具，对话驱动 + 进退场标记（2026-11-11，已只提交推送、不发版，远端提交 09503a8）

- 用户纠正：§12.6 的例子不对——“改版退款流程”是旧项目改版，要用全新项目演示需求 → 设计文档 → 拆解 → 开发；篇幅不要花在讲 sunshine 子命令上，只关心全流程人机对话；要标清楚设计人员何时进厂退厂、开发人员何时进厂。
- 新例子：团队请假申请小工具（申请页 + 审批列表页），从零开始。演示目录 `/tmp/scenes/s6-leave` 真实跑通全流程：设计包（`doc/design/需求列表.md`、`doc/design/概要设计.md`、`doc/prototype/leave-new.html`、`doc/prototype/leave-list.html`）→ 评审通过 → 一次性拆解（父任务 + 前端两页 + 后端接口）→ 前端（grep 验收 3/2/1/1，提交 `7d54a51`）→ 后端（curl 三验：缺 reason 400、11 天 NEED_APPROVAL、2 天 200 id 2，提交 `1ce3c59`）→ 父任务集成复核 → 归档（0 活动、3 归档）→ 变更示例（年假上限 10→5 天，新任务，curl 验 6 天 NEED_APPROVAL、5 天 200 id 3，提交 `d020234`，归档后 4 个任务）。
- 写法：中英文 §12.6 整节重写（中文 490→135 行、英文 496→137 行），对话驱动——对话带流程，命令只贴关键输出（task start、curl 三验、最终 list/list-archive）；三处进退场标记：**【设计人员进场】** / **【设计人员退场，开发人员进场】** / **【开发人员退场】**（英文同款 `[Designer enters]` / `[Designer exits, developers enter]` / `[Developers exit]`）。
- 红线说明：§12.6 的开场提示词是新的（例子换了，全新项目四要素），其余六处开场提示词（§2 + §12.1–12.5）一个字没动；标题、锚点未变，§12.6 内已无退款字样。
- 两个真实教训：① `task archive` 自动提交要 git 身份，演示目录须先 `git config user.name/email`，否则归档已移动、提交失败（前端归档那次就是这么补的）；② `pkill -f "node src/server.js"` 会把自己的 shell 一起杀掉（命令行含该字符串），停服务时不要用这招，改查端口确认。

## §12.6 再重写：一份系统原型对多个需求 + 设计人员返场（2026-11-11，已只提交推送、不发版，远端提交 b3906e9）

- 用户定：设计阶段产出**一份最终的系统原型**（一个文件，多个页面 + 页面间导航），不是按需求拆多个原型；**一个原型对应多个需求**（需求列表按需求分条，每条对应系统原型里一个页面）；**拆解跟着需求走**，子任务按需求拆；原型**带 mock 假数据**，能点着给用户演示。
- 演示目录 `/tmp/scenes/s6-leave` 重建后真实跑通：设计包是 `doc/design/需求列表.md`（需求一提交、需求二审批，各指系统原型里一页）、`doc/design/概要设计.md`、`doc/prototype/原型.html`（一份系统原型，申请页 + 审批列表页 + 导航，列表里三条 mock 假记录：张三待审批、李四已通过、王五待审批）→ 评审 → 一次性按需求拆（父任务 + 需求一 + 需求二）→ 需求一（申请页 + POST，grep 3/2/1，curl 三验：缺 reason 400、11 天 NEED_APPROVAL、2 天 200 id 2，提交 `31e6c36`）→ 需求二（审批列表页 + GET，表头六列、通过按钮在，GET 返回 2 条记录，提交 `b3893e2`）→ 父任务集成复核 → 归档（0 活动、3 归档）→ 变更示例（上限 10→5 天：**设计人员先返场**改需求列表、概要设计、系统原型并评审通过，再建新任务走开发，curl 验 6 天 NEED_APPROVAL、5 天 200 id 3，提交 `add1c30`，归档后 4 个任务）。
- 写法：中英文 §12.6 整节重写，对话驱动，命令只贴关键输出；五处进退场标记：**【设计人员进场】** / **【设计人员退场，开发人员进场】** / **【开发人员退场】** / **【设计人员返场】**（英文同款 `[Designer enters]` / `[Designer exits, developers enter]` / `[Developers exit]` / `[Designer returns]`，退场进场标记在变更段再出现一次）。
- 教训（第二次）：`muse.write` 写中文教程时会把 `**【……】**` 里的【】内容整段去掉，只剩 `****`——本轮五处标记全中招，英文 `[]` 不受影响。以后写完中文标记行必须 grep 复核，有 `****` 残留就用 `muse.edit` 逐行补。
- 附带：沙箱 `/usr/local/bin/sunshine` 被 VM 更换清掉过一次（不带参数一键装回 v0.3.10）；原型 mock 假数据是事后补的两行（提交 `ee83237`，在变更归档之后），教程里关于原型内容的说法以最终文件为准，任务/curl/提交等实跑输出不受影响。

## §12.6 变更段展开：设计人员返场有实活，交接有凭据（2026-11-11，已只提交推送、不发版，远端提交 679057d）

- 用户纠正：变更段太省略了——设计人员返场要重新设计原型、调整相应文档，评审后开发才进场，中间的交接不能让 Claude 一笔带过。
- 交接机制（已写进教程）：① 改好的设计包（`doc/` 里的文件就是合同，开发照着改）；② 评审确认（只审变更部分，确认完才往下走）；③ 新任务的上下文清单（`implement.jsonl`/`check.jsonl`——开发人员进场只读这两份清单里的文件，不用重读整个设计包）；④ PRD 里的 `[变更 YYYY-MM-DD]` 记录（审计线索）。
- 变更段重写（中英文，中文 18→55 行、英文 18→57 行）：设计人员返场改三处（真实行号+内容：需求列表.md:9、概要设计.md:16、原型.html:12，改后都是 5 天）→ 只审变更部分的评审对话 → “交接：开发人员拿到什么”（三样东西 + 变更任务真实的 implement.jsonl/check.jsonl 内容 + PRD 变更记录原文）→ 开发人员进场（curl 两档、提交 `add1c30`、归档后 4 个任务）。演示目录不用重跑，交接物都是 `/tmp/scenes/s6-leave` 里已有的真实文件。
- 教训（第三次）：`muse.write` 写中文教程时 `**【……】**` 里的【】内容又被整段去掉、只剩 `****`——本轮变更段两处新标记全中招，已用 `muse.edit` 逐行补回并复核（中英文各 5 处标记、无残留）。

## §12.6 中段重写：拆解产出物 + 一轮一轮的开发循环 + 关闭重开后接上（2026-11-11，已只提交推送、不发版，远端提交 679057d）

- 用户要求：需求 → 设计文档 → 拆解 → 开发，要写清拆解的产出物是什么、开发进场后怎么按交接物一轮一轮编码；Claude 关闭重开后开发人员也要知道怎么接上，不能让 Claude 一句话就带去工作。
- 拆解产出物（真实目录树 + task.json 摘录）：每个任务目录四份文件（task.json、prd.md、implement.jsonl、check.jsonl）；父任务 task.json 的 children 记两个子任务、子任务的 parent 指回父任务、status 是 planning；父任务的两个 jsonl 是空文件（0 字节）；需求一 implement.jsonl 指原型申请页 + 概要设计，check.jsonl 指前端规范 + 后端接口契约；prd.md 是骨架（“待定”），agent 在子任务自己的 1.1 里填实，复杂任务 start 前补 design.md/implement.md（prd.md 模板原话）。
- 开发循环（五轮，每轮写清读什么、产出什么）：2.1 实现（读 implement.jsonl + prd.md，派 implement，产出改好的代码）→ 2.2 检查（读 check.jsonl，派 check，产出 grep/curl 验收）→ 3.3 回写规范（产出更新后的 spec）→ 3.4 提交（产出提交）→ 收尾（finish 清指针、archive 搬目录、父任务 1/2）。需求二走同样的五轮（对话略）。
- 关闭重开后接上（单独小演示 /tmp/scenes/s6-resume 真实跑通）：状态全在文件里（task.json 的 status、.current-task 指针、jsonl、prd.md、spec），新会话跑 /sunshine:continue → task context → 按 status 分流（continue.md 路由规则原文）；新 shell 里 task current 返回任务路径、task context 显示“来源：file:.sunshine/.current-task、状态：in_progress、[!] 该任务有 prd.md，详情请读它”；.current-task 里就一行任务路径。两个注意点（真实踩过）：task start 有守卫，check.jsonl 为空直接报“错误：check.jsonl 没有已整理条目”（第一次 start 失败就是被我的 grep 滤掉了报错）；归档完的任务不在活动列表、不会再被续接，变更走新任务。
- 教训（第四次）：muse.write 写中文教程时 **【……】** 又被整段去掉——本轮中段文件首行和第 105 行两处标记全中招，splice 的标题断言拦住了错误拼接（工作树没被破坏），用 sed 按行号补回后重拼成功。

## §12.6 补三处：设计阶段顺序 + 一需求多任务 + 全流程闭环一览（2026-11-11，已只提交推送、不发版，远端提交 ef574e9）

- 用户确认两点并要求写进教程、整应用场景梳理通透、完全闭环：① 一个需求对应一个或多个任务（有前后端时）；② 设计阶段建原型也是先需求后原型、一步步做出最终原型。
- 补的三处（中英文同步，只增不改，62 行新增）：① 设计阶段顺序——先需求列表、再概要设计、再按需求列表一页一页做出原型，原型是设计阶段的终点不是起点；做原型发现缺口则需求列表跟着改（走变更）。② 一需求多任务——默认一个需求一个子任务（包揽一页 + 接口）；前后端都大/不同人做/后端多页共用时再拆前端+后端两个子任务，交接物是概要设计里的接口约定，前端按约定 + 原型 mock 数据做、不用等后端；机制已实测（/tmp/scenes/s6-resume：`task create --parent`，父任务 children 记两个子任务，`task list` 嵌套显示 [0/2 已完成]，真实输出贴进教程并标注是单独小演示）。③ 文末“全流程闭环一览”——9 步闭环（需求提出 → 需求澄清 → 设计包 → 需求评审 → 拆解 → 开发五轮 → 集成复核与归档 → 需求变更 → 关闭重开后接上），每步写清谁在场、交接物是什么。
- 核对：标题一致、中英文各 5 处标记、无 `****` 残留、六处旧开场提示词未动、全量测试无 FAIL。

## §12.1–12.5 重写：对话驱动，子命令只留验证结果（2026-11-11，已只提交推送、不发版，远端提交 f1d7fe8）

- 用户指出：12.1–12.5 五个场景还是太多篇幅在介绍 sunshine 子命令，用户不关心，只关心在 Claude Code 这类 agent coding 工具里是怎么工作的、对话的实现流程。
- 改法（中英文同步，§12.6 未动）：整节替换，每场景只留三样东西——场景说明、开场提示词（一个字未动）、与 Claude Code 的对话（问 → 答 → 确认规划 → 汇报进展）；`sunshine ...` 命令行在 12.1–12.5 里清零（原来 30 行）；只保留验证结果（值班表输出、审计日志出现 1 次、测试全绿 `ok shop`、401 日志、spec 写进去的内容、代码本身），且都以“Claude Code 跑给你看 / 验给你看”的口吻出现，不贴产生它们的命令。12.3 的 channel 协调只留两条消息的文本（对话里说），12.5 的 mem 只留找到的会话编号与命中句（对话里说）。
- 规模：中文 450→242 行、英文 462→252 行，共删 418 行、增 48 行。
- 核对：标题一致；开场提示词逐块与 HEAD 比对一致（中文 9 处、英文 6 处）；§12.6 一字未动；中英文各 5 处进退场标记；无 `****` 残留；全量测试无 FAIL。

## v0.3.11：原型 agent + token 精简（2026-11-12，已随 v0.3.11 发布）

- 动因（用户指出）：生成高保真原型就是写代码（HTML/CSS/JS + mock 数据），不应由主会话在设计阶段顺手写——原型文本会住在主会话上下文里，之后每轮（评审、拆解、子任务规划、父任务复核）都要再付一次。省的不是写原型的成本（谁写都一样），而是重复上下文成本。另附一次 token 审计，见下。
- 定下的模型：**原型归设计管（需求、评审、验收在设计阶段），生产交给子代理**。新增 `sunshine-prototype` agent；初版生成与结构性大改走该 agent，小修（改个按钮颜色、一行文案）留在主会话，用 grep 只读相关那一页。
- 工作项（共 7 项）：
  1. **新 agent `sunshine-prototype`**：brief 只给两样——`doc/design/需求列表.md` + `doc/design/概要设计.md`（主会话在派发提示里给路径）；产出单文件系统原型 `doc/prototype/原型.html`（全部页面 + 导航 + mock 数据）；mock 数据只放演示够用的几条，只做需求列表里的页，不打磨样式细节；不读 `.sunshine/spec/` 与 workflow.md（原型是独立演示物，不是产品）；不 git commit；不派生其他子代理；最后汇报只写页数、mock 记录数、文件路径，**不准把原型全文贴回汇报里**。不复用 `sunshine-implement`（已核实：implement 定义强制动手前先读 workflow.md + `.sunshine/spec/`，设计阶段还没有上下文清单，复用会白读）。
  2. **三份 workflow 模板的 1.1 设计先行分支**（native 341 行、tdd 334 行、channel-driven 209 行附近）：原型生成改为派发 `sunshine-prototype`。native/tdd 用宿主原生子代理（Task 工具，`subagent_type` 为 `sunshine-prototype`）；channel-driven 用 `sunshine channel spawn --agent prototype`（加载 `.sunshine/agents/prototype.md`）。
  3. **派发协议加一句“只读 reason 指明的部分”**：native 237 行、tdd 230 行附近、channel-driven 137 行的“读上下文的顺序：jsonl 条目 → prd.md → design.md → implement.md”之后加——jsonl 条目只读 reason 指明的部分，大文件先 grep 定位。jsonl 的 reason 本来就写了是哪页（“系统原型：申请页”）。
  4. **设计先行模式下父任务 prd.md 改成索引**：brainstorm skill 产物规则 168 行段落后加——父任务 prd.md 只写目标、验收标准与指向（需求见 `doc/design/需求列表.md`，概要见 `doc/design/概要设计.md`），不把需求列表全文复述；需求的唯一事实来源是 `doc/` 里的设计文档。子任务自己的 prd.md 不动（那是子任务自己的合同）。与 skill 已有的收敛检查（“把重复的事实合并到一个权威章节”）一致。
  5. **父任务集成复核的读法**：native 478 行、channel-driven 270 行附近加——复核读验收标准 + 各子任务的检查结果 + 接口约定，不重读整个原型。
  6. **教程 §12.6（中英文）同步三处**：① 设计阶段做原型那段（中文 1078 行、闭环一览 1329 行）——改为派发 `sunshine-prototype` 做原型，主会话只收汇报；② 父任务集成复核（中文 1199–1201 行）——按第 5 项的读法改；③ 变更时改原型（中文 1275 行附近，上限提示 10→5 是一行小改）——注明小改在主会话里 grep 定向改，不重新派发。§12.6 开场提示词一个字不动（它不点名派发，派发是 workflow 自身的默认做法）；其余六处开场提示词一个字不动。
  7. **README 装指定版本示例改到 v0.3.11**：README.md:20、README.en.md:17/20 的 v0.3.8 改成 v0.3.11（随本版发版一起做；此前待定的“改成 v0.3.10”widget 已被本版 supersede，不再单独做）。
- 已核实的接线事实（实施时照此办，不要另发明）：
  - agent 安装：`internal/platforms/platforms.go:481` 的 `agentNames = ["implement", "check", "research"]` 是 md 平台的唯一事实源；`mdAgents`/`pullAgents`/`agentSkillCopies` 由它生成登记。加 `"prototype"` 即自动装到 12 个 md 平台 + grok/kimi；但每个平台的定义文件必须存在：`templates/platforms/<tmplDir>/agents/sunshine-prototype.md`（claude、cursor、gemini、qoder、pi、zcode、trae、omp、snow、opencode、grok、kimi；以各平台现有 implement.md 为蓝本改前置元数据，正文同一份原型 brief）。codex（.toml）、kiro（.json）、copilot（.agent.md）、droid（.factory/droids/）是显式登记，各补一份同名定义。codebuddy 的 AgentFiles 是 `mdAgents("codebuddy", ".codebuddy/agents")`（platforms.go:391），加 `"prototype"` 进 `agentNames` 即自动覆盖，已放定义文件并验证。dsh/kilo/antigravity/devin 没有 agents（kilo/antigravity/devin 的 AgentCapable=false；dsh 由 common 渲染）——prototype 的平台覆盖与 research 完全一致；reasonix 目前没有 research（只有 implement+check），不加 prototype，原因已核实（见下）。
  - `.sunshine/agents/` 通用定义：`templates/templates.go` 的 `Sunshine` FS（`//go:embed all:sunshine`）+ `internal/scaffold/scaffold.go:106` 的 WalkDir 全目录拷入，新增 `templates/sunshine/agents/prototype.md` 即自动随 init 落盘（以 `templates/sunshine/agents/implement.md` 为蓝本改写）。`internal/workflow/agents.go` 的 `CollectMissingAgents` 会校验 workflow 引用的 agent 文件在磁盘上存在——所以通用定义必须有，否则 `workflow` 命令会报缺 agent。
  - hook 不动：`internal/hook/subagent.go` 的 `agentsAll` 只有 implement/check/research；prototype 不在列时 `subagent-context` hook 对它直接返回 nil，派发提示原样通过——这正是要的行为（原型 agent 的上下文是派发提示里给的设计文档路径，不需要 hook 再注入任务上下文）。不要“顺手”把它加进 agentsAll。
  - `continue.md` 不用改：原型生成发生在 1.1（planning）内，continue 按 status 分流到 1.1 即可；原型做到一半关闭重开，主会话进 1.1 看到 `doc/prototype/原型.html` 缺失或不完整就重新派发，状态在文件里。
  - sunshine-meta skill 的 workflow.md 例子（`templates/platforms/common/bundled-skills/sunshine-meta/references/local-architecture/workflow.md`）如含 1.1 设计先行分支或派发协议文字，同步改（v0.3.7 时同步过编号）。
- 测试与核对（缺一不可）：
  - 文件数断言必须同步改：`internal/platforms/platforms_new_test.go`（16 平台）、`internal/platforms/platforms_e_test.go`（codebuddy/copilot/droid），以及所有其他断言安装文件数的测试（grep `wantCounts` 与安装文件数，全改）；`internal/scaffold/scaffold_test.go:194-198` 的 agents 清单 5→6。
  - 全量 `go build`/`gofmt`/`go vet`/`go test ./...` 0 失败；新 agent 定义逐平台抽查（frontmatter 格式与各平台现有 implement 一致）；真实二进制 `init` 后核对 `.claude/agents/sunshine-prototype.md` 与 `.sunshine/agents/prototype.md` 落盘；三份 workflow 模板的 1.1/派发协议/父任务复核文字逐处核对；六处开场提示词与 §12.6 开场提示词逐块比对一个字未动；全文无 trellis 残留、无 `{{}}` 占位；`sunshine workflow` 无参数查模板、task validate 不报缺 agent。
  - 模板是编译进二进制的（go:embed），改动只随发版生效；不要用“只推 main、用户手动拷”代替发版（v0.3.5 的守卫是特例，本批直接发版）。
- 发版：下一版 v0.3.11，走既定发版固定动作（本地 commit → 确认工作树干净 → push-initial.py → fetch + reset 对齐 → Git Tags API 两步打 tag → 轮询 CI/发布 run → 核 Release 6 个资产 → 真实一键装新版 + 旧版 upgrade --dry-run + darwin checksum/Mach-O → PLAN.md 追加发布记录并再推一笔 → 更新 GOAL.md/MEMORY.md/goal entry → 汇报）。
- 审计时查过、决定不动的（不要在本批里翻出来）：hook 每轮面包屑实测只有四五行；check 与 update-spec 分开读是刻意的职责分离且 spec 文件小；每子任务五轮是用户定的“都要走子代理”；1.2 拆解的整包通读省不掉。
- 独立验证（2026-11-12，我复核）：全量测试 19 包 0 失败（我亲自跑过，含两处小修复后重跑 platforms/scaffold/workflow/cli）；真实二进制 init 核对（`.claude/agents/sunshine-prototype.md`、`.sunshine/agents/prototype.md` 落盘，channel-driven 切换成功且 223 行有原型派发段，task validate 只报 2 个预期 jsonl 错误、无缺 agent 报错）；六处开场提示词与 §12.6 开场提示词逐块比对未动（教程 diff 只动 §12.6 正文与复核对话两句）；新增行与新文件无 trellis、无 `{{}}`、无 `****` 残留（两处 grep 命中是本节核对文字本身）；hook 与 continue.md 未动。
- 两处我亲自改的：汇报格式示例页名由“需求一的页面/需求二的页面”改成“申请页/审批列表页”（17 个文件，codex .toml 无此例）；本节实施说明里 codebuddy 一句有误已纠正（platforms.go:391 是 `mdAgents("codebuddy", ".codebuddy/agents")`）。
- 两处定夺：kiro prototype.json 的 agentSpawn hook 块保留（与 research.json 同形；hook 对 prototype 直接返回 nil，是一次无害空调用）；native/tdd 1.1 的“用 Task 工具派发”不展开 Kimi 等平台变体（派发协议已通用于所有平台与所有子代理，主平台是 Claude Code）。
- reasonix 缺 research 是故意的（上游 Trellis 自身的 reasonix 模板目录就只有 check 与 implement 两份；`platforms_new_test.go:301` 有测试钉死），不用另立任务。

## v0.3.11 发布记录（2026-11-12）
- 主提交远端 `92626af`（本地 `f4264db`，32 个文件：18 个新 agent 定义 + 14 个修改），tag 对象 `a2df33f`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.3.11，6 个资产；发布 run 43134240817、两路 CI（43134240787、43134211026）均 success 无抖动。
- 实测：不带参数一键装上 v0.3.11（`sunshine version` = 0.3.11）；v0.3.10 原版二进制 upgrade --dry-run 查到 v0.3.11；darwin arm64 包 checksum OK（`0ac0f5fa…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.4.0：只读观测后台 `sunshine serve` v1（实施说明，2026-11-12，用户已批“开始做 v1”）

- 设计方案：`~/workspace/goals/go-sunshine-trellis/files/只读观测后台设计方案.md`（已全部定完：v1 18 项、暂缓 8 项、7 个待定问题、前端 Alpine.js、视觉深色霓虹、虚拟智能体 Canvas 动画）。实施以该方案为准，不另发明；方案与本节不一致时以方案为准并记一笔。
- 目标：新增 `sunshine serve` 命令（Go + Gin，嵌进二进制），本机只读观测后台：顶栏指标行 + 实时时钟、任务总览/详情、Channel 列表/事件流/worker、Forum、会话/记忆、项目健康、设计文档、Journal，顶部 hero 区一块 Canvas 2D 虚拟智能体动画（核心脉冲 + worker 轨道节点 + 事件脉冲 + 五轮流程线 + HUD 数字，全部由真实事件/状态驱动）。
- 已定决策（实施时照此办）：
  - 默认只绑 `127.0.0.1`，端口 **17890**；flags：`--port`（被占报错、不自动跳）、`--interval`（轮询秒数，默认 3，范围 1–30）、`--addr`（显式开 0.0.0.0 时必须同时有鉴权——v1 无鉴权，`--addr` 非 127.0.0.1 时直接拒绝并提示等 license 落地）。
  - 轮询默认 **3 秒**；子任务进度口径 **`ChildrenProgress`**（页面注明“归档子女算完成”）；v1 只看当前项目（serve 在哪个项目目录跑就看哪个项目；channel 列表只显示当前项目 projectKey 下的 channel）。
  - workflow 模板名：项目 `.sunshine/workflow.md` 的 sha256 对三个内嵌模板（native/tdd/channel-driven）的哈希，命中显示名、未命中显示“已修改”（init 写的 workflow.md 与模板同源字节一致；实现时先核实内嵌模板哈希可取，若比对机制有问题则回退到不展示）。
  - 前端：**Alpine.js**（单个 JS 文件，vendored 进仓库并嵌进二进制，pin 版本并记在本节；不用 HTMX、不用 SPA、不用 CDN）；样式单个 CSS 文件嵌进二进制，不用 Tailwind CDN（离线可用）。
  - 视觉：**深色霓虹科幻**（整个页面深色底 + 霓虹辉光蓝/青为主，琥珀等候、红色错误；深色实体卡，仍不用玻璃卡片；HUD 等宽数字）。万表归一纸色体系的一次有意例外，仅限后台大屏；文档/教程保持纸色。
  - 虚拟智能体动画：Canvas 2D + requestAnimationFrame，单文件 JS 嵌进二进制；不用 three.js、不用 WebGL、不用 CDN。诚实规则：环境层（呼吸、慢轨道）只表示“系统活着在监听”，事件层（脉冲、闪烁、点亮）只由真实事件/状态变化触发；**绝不伪造事件、绝不让节点假装在跑**；页面不可见时暂停（visibilitychange），`prefers-reduced-motion` 时只显示静态帧，低端机自动降粒子数，Canvas 不可用时 hero 区退化为静态卡片。
  - token 专项：**暂缓**（2026-11-12 用户说 token 验证暂时不做）；v1 不含 token 面板、不画 token 消耗墙（没有数据的面板宁可空着/注明，不用假数据填充）；质检台 PASS/DROP 等 v2 埋点（check 完成事件）才有结构化结果，v1 不做。
  - Native 活动流：v1 用 **mtime 比对**（每轮轮询比对 task.json、prd.md、implement/check.jsonl、doc/ 的 mtime，变化记一条活动）；hook 埋点是 v2 的事，**本批不改 hook**。
- 代码落点（已核实 2026-11-12）：
  - 工作树干净，HEAD = 远端 main = `cd2d965`；Go 1.27，module `github.com/BondLam/Sunshine`。
  - **Gin 不在 go.mod 里**（已核实：go.mod 只有 survey/go-git/cobra/yaml/sqlite 等，无 gin）。实施须 `go get github.com/gin-gonic/gin`，go.mod/go.sum 的变化记在本节；版本 pin 当时最新的稳定版。
  - 新建 `internal/serve/`（Gin 路由 + handlers + 数据组装）与 `internal/cli/serve.go`（cobra 命令，`init()` 里 `root.AddCommand`，与 ablate.go:43、channel.go:32 同一模式）；前端静态文件（index.html、app.js、hero.js、style.css、alpine.min.js vendored）放 `internal/serve/static/`，用 `//go:embed` 嵌进二进制（与 templates/templates.go 同一模式）。
  - 复用内部包拿结构化数据，不重新解析文件、不解析 CLI 纯文本输出：`internal/task`（list/current/context）、`internal/channel/store`（ChannelSummary、events.jsonl、`ReduceWorkerRegistry`）、`internal/mem`（MemSessionInfo/MemProjectSummary）、`internal/update`（`AnalyzeChanges`）、`internal/upgrade`（`ResolveTag`/`LatestVersionFunc`）、`internal/config`、`internal/platforms`。**两处禁区**：`task list-archive` 本身无 `--json`，serve 复用内部目录扫描，不要调它解析文本；`update --dry-run` 返回值计数字段基本是 0，健康数据必须直接复用 `AnalyzeChanges`。
  - 只读：全部 GET，无任何 POST/PUT/DELETE；不写盘（除 Gin 自身日志外不落文件）；不改 task.json schema、不改 hook、不改事件 kind、不接 license（v1 无鉴权，只绑本机是前提）。
  - 非 Sunshine 项目目录下跑 `serve` 直接拒绝（找不到 `.sunshine/`，与其它命令一致）；`serve` 在项目子目录里跑也行（往上找 `.sunshine/`）。
- API 清单（与方案一一对应，全部 GET）：`/api/status`、`/api/tasks`、`/api/tasks/current`、`/api/tasks/:id`、`/api/tasks/archive?month=`、`/api/tasks/:id/commits`、`/api/channels`、`/api/channels/:name/events?since=&kind=&from=&last=`、`/api/channels/:name/workers`、`/api/channels/:name/workers/:worker/log?lines=`、`/api/channels/:name/threads`、`/api/channels/:name/threads/:thread`、`/api/mem/sessions?platform=`、`/api/mem/projects`、`/api/health`、`/api/version`、`/api/design`、`/api/journal`。另有 `/` 返回页面（index.html）。事件流用 `?since=<seq>` 增量拉取；不要用 channel `wait`（阻塞等待，不适合 HTTP 轮询）。
- 测试与核对（缺一不可）：
  - 全量 `go build`/`gofmt`/`go vet`/`go test ./...` 0 失败；新增 `internal/serve/*_test.go`（handlers 的 JSON 字段、端口/绑定守卫、非项目目录拒绝、模板名哈希比对命中/未命中、mtime 活动流）。
  - 真实二进制端到端（临时项目目录 + 构造的 task.json/channel events.jsonl/doc/）：`sunshine serve --port <端口>` 起服务，curl 逐个端点核对字段与设计方案一致；`/` 返回的 HTML 含 Alpine.js 与 hero canvas 挂载点、含深色样式；`--port` 被占报错；`--addr 0.0.0.0` 被拒；非项目目录被拒；`serve --help` 中文。
  - 沙箱无真实 channel 事件流、不能真看浏览器渲染效果——前端视觉（深色霓虹、动画）以代码审查 + 静态检查为准，真实渲染由用户在 Mac 上自验；不要谎称“已看过渲染效果”。
  - 全文无 trellis 残留、无 `{{}}` 占位、无模拟数据（grep 核对：任何写死的示例数字/事件都必须删掉，活动流与动画只认真实文件与真实事件）。
- 发版：**本批不发版**——只做实现 + 验证，不打 tag、不发 Release。`sunshine serve` 随下一版（v0.4.0）发版，等用户指令再走既定发版固定动作。
- 风险：Gin 是新依赖（go.mod/go.sum 变化 + 二进制体积变化）；Alpine.js 须 vendored（不要运行时从 CDN 拉）；沙箱验证只能到 curl/JSON 层，渲染效果待用户 Mac 实测。

## 实施结果（2026-11-12，只提交、不发版）

- **Gin 版本**：`github.com/gin-gonic/gin v1.12.0`（`go get` 当时最新稳定版）；go.mod +25 行（1 个直接依赖 + 间接依赖）、go.sum +64 行。
- **Alpine.js 版本**：**v3.17.5**（jsDelivr 查到的最新 3.x，pin 死；`internal/serve/static/alpine.min.js` 原样 vendored，未改源码）。
- **新增/修改文件**：
  - 新增 `internal/serve/serve.go`（Server、路由、Run、版本/workflow 模板名/模板缓存、display_status 规则抄录）、`static.go`（`//go:embed all:static`，`/` 与 `/static/*`）、`activity.go`（mtime 快照 + 环形缓冲 100 条）、`handlers_tasks.go`、`handlers_channels.go`、`handlers_misc.go`、`serve_test.go`（23 个测试）。
  - 新增 `internal/serve/static/`：`index.html`、`style.css`、`app.js`、`hero.js`、`alpine.min.js`。
  - 新增 `internal/cli/serve.go`（cobra 命令，全中文）与 `internal/cli/serve_test.go`（4 个测试）。
  - 修改 `internal/cli/root.go`（`init()` 加 `RegisterServeCommand` 一行）、`go.mod`、`go.sum`。
- **验证结果**：
  - `go build`/`gofmt`/`go vet` 全过；`go test ./...` **19 个包全过**（含 serve 23 个、cli serve 4 个测试）。
  - 真实二进制端到端（`/tmp/srv-e2e` 临时项目：init + 2 任务 + 子任务 + 归档 + doc/ + channel e2e-chan 手工事件 + forum-1 + mem 夹具）：18 个 GET 端点 + `/` + `/static` 逐个 curl 核对，字段与设计方案一致。`/api/status` 含 `poll_interval_sec=3`、`workflow_template` 命中 `"native"`、`latest_version` 实测解析到 `"0.3.11"`；事件增量 `?since=3` 正常、progress 事件不丢；worker impl-1 running/mid-turn、check-1 done/exitCode 0、workersAlive 1/total 2；归档提交定位到真实提交 `8068cb6`；mtime 活动流 touch 后出现 file_change 条目；mem 会话夹具被正确读出。守卫：端口被占中文报错 exit 1、`--addr 0.0.0.0` 被拒、非项目目录被拒、子目录启动 200、`--interval 0/31` 被拒、`serve --help` 全中文。
  - **实施中修掉的 1 个 bug**：`/api/tasks/:id` 与 `/api/tasks/:id/commits` 对已归档任务返回 404（`ResolveTaskDir` 只认活动任务），已加 `resolveTaskDirWithArchive` 回退（活动 → 归档 `FindArchivedTaskByDirName`），补测试 `TestArchivedTaskDetailAndCommits`。另核对中发现 workers 缺 impl-1 是我的手工事件夹具缺 `as` 字段（spawned 事件取 `as` 不取 `worker`），不是后端 bug，夹具修正后正常。
  - **契约外多给的字段**（只增不改）：mem 会话同时出小写与首字母大写两套键；`/api/health` 另给 drift 计数与顶层 added/updatable/modified/unchanged/deleted；`/api/design` 另给 docs 数组；事件另给 from/author/text/summary/target。新增 `/api/activity`（mtime 活动流与 channel 事件合并；设计方案 liveness 一节已定 mtime 流，前端需一个端点取它）。
- **未验证项**：浏览器真实渲染效果（深色霓虹、Canvas 动画）待用户 Mac 自验；真实 channel 事件流（live worker、supervisor pid 探活非空）只在夹具事件上验证，未在真实跑起来的 channel 上验证；`/api/tasks/:id/commits` 的 git 定位只在单提交夹具上验证。
- **发版**：已随 v0.4.0 发布（见下节发布记录）。

## v0.4.0 发布记录（2026-11-12）
- 主提交远端 `f13a297`（本地 `9adf802`，18 个文件：只读观测后台 `sunshine serve` v1——Gin v1.12.0 + vendored Alpine.js v3.17.5，深色霓虹；18 个 GET 端点 + `/api/activity`；默认 127.0.0.1:17890；hero 区 Canvas 2D 虚拟智能体动画），tag 对象 `d0aa1ce`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.4.0，6 个资产；发布 run 43142479526、两路 CI（43142479484、43142456389）均 success 无抖动。
- 实测：不带参数一键装上 v0.4.0（`sunshine version` = 0.4.0）；v0.3.11 原版二进制 upgrade --dry-run 查到 v0.4.0（下载地址指向 v0.4.0 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`f016f017…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.4.1 发布记录（2026-11-12）
- 主提交远端 `6f37de0`（本地 `fc7b7b5`，5 个文件）：修 serve 前端无数据——Alpine 在自身脚本执行完后经 queueMicrotask 立即启动并派发 alpine:init，而 app.js/hero.js 排在 alpine.min.js 之后，serveApp 注册不上，控制台报 serveApp is not defined、页面全是 —；改法是把 app.js/hero.js 移到 alpine.min.js 前面执行（defer 顺序保证）。另：errors.status 加可见横条（此前失败时无任何显示）；hero 待机加亮（轨道/星点/核心辉光 + 待命节点描边）；hero-caption 移到页脚；journal 工具栏 x-for 在 journal 为 null 时报 TypeError（x-show 不阻止求值）改成守卫表达式，另修 journal 文件按钮显示 [object Object]（files 是 {name, path} 对象，改取 f.name，loadJournal 存名字符串）。
- 验证：go build/vet/gofmt 全过；go test ./... 19 个包全过（新增 script 顺序回归断言 TestIndexAndStatic）；node --check 三个 JS 文件全过；**新增 jsdom 真实渲染验证**（`~/workspace/.jsdom-test/render-test.mjs`）：有数据项目与空项目两路，顶栏/时钟/心跳/任务全部填上，控制台零报错（jsdom 下 canvas getContext 不可用走 heroFallback 静态卡片，是设计的降级）；验证中发现 /tmp tmpfs 曾满（多个 49MB 的 go test 临时二进制 + 73MB 验证目录），已清理。教训：go test 临时二进制落 /tmp，大批量跑测试前后注意 /tmp 空间；`pkill -f` 的模式若出现在自己的命令行里会杀掉自己的 shell，停 serve 进程改按 PID kill。
- tag 对象 `9402cd8`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.4.1，6 个资产；发布 run 43146603139、两路 CI（43146603248、43146525334）均 success 无抖动。
- 实测：不带参数一键装上 v0.4.1（`sunshine version` = 0.4.1）；v0.4.0 原版二进制 upgrade --dry-run 查到 v0.4.1（下载地址指向 v0.4.1 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`0467c52f…`，Mach-O arm64；Linux 下执行 darwin 二进制报 Exec format error 属正常）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.4.2：agent-display 风格大屏（实施说明，2026-11-12，用户已批“中央轨道升级暂缓、先做其它显示”）
- 目标：按 agent-display THEME 03（NEURAL RING，源码在 `~/workspace/research/agent-display`，只借鉴思路、不复制代码）的布局与 HUD 语言重做 serve 大屏，数据全部真实，绝不填模拟数据。
- 布局（上 → 下）：顶栏（现有字段 + 新增 LAST SIGNAL：最近一次真实事件的文本与时间，取 /api/activity 首条）→ 三列：左智能体团列表 / 中现有 hero（本版不动）/ 右热力格 + 信号流 → 底部大指标行 → 下方保留现有全部详情面板（任务总览/详情、Channel 列表/事件流、mem、健康、设计、journal），不删功能、不重数据。
- 左智能体团：5 个角色（implement/check/research/prototype/主会话），每行状态灯（worker 真实状态）+ 当前任务行（/api/channels/:name/workers + /api/tasks/current；无 worker 时显示待命，不假装在跑）。
- 右热力格：Canvas 2D，行 = 5 个角色，列 = 最近 30 秒、每 2.5 秒一格（12 列），格值 = 该角色该时间段的 channel 事件数（/api/activity + channel events；事件到角色的映射复用 hero.js 的 roleOfWorkerName）；token 列不做（无数据）。
- 右信号流：即现有最近活动（/api/activity），移入右列，样式按 SIGNAL FEED（时间 + 角色 + 文本），不另建数据源。
- 底部大指标行：大号等宽数字 + 标签 + 信号速率火花线（Canvas 2D，取最近活动时间戳算每秒事件数，60 点）：活动任务数、运行中 worker、channel 事件总数、今日活动、归档任务数、健康漂移（added/updatable/modified）、版本三方（项目版/CLI 版/最新版）。token/成本两格不做（无数据）。原 stat-row 四卡片并入底部指标行，不重复展示。
- 中央暂缓（本版不做，记在这里）：轨道智能体系统升级——能量环（72 格）+ 文字带 + 更丰富的事件脉冲，复用 hero.js 结构，难度中等（约 1–2 天），等其它显示落地、用户看过效果后再定。hero.js 本版不动（v0.4.1 的待机加亮保留）。
- 不做：token 相关面板、CRAWLER 巡检蛛、任何模拟数据填充；three.js 不引入（Canvas 2D 已够，调研结论见 MEMORY.md）。
- 验证：go build/vet/gofmt + go test ./... 全过；jsdom 真实渲染验证（有数据/空项目两路，顶栏/左列/热力格/信号流/底部指标全部填上，控制台零报错；布局变了要同步改 render-test.mjs 的选择器）；用户 Mac 实测后发版（发版另等用户指令）。
- 顺序原则（用户 2026-11-12）：不照搬 agent-display，按由易到难做——先信号流/LAST SIGNAL/左列/底部指标/热力格这些容易的，布局不必死抠三列对齐，简单好用优先。

## v0.4.2（改向）：Tailwind 卡片运维仪表盘（2026-11-12，用户叫停 agent-display 方向后定）
- 背景：agent-display 风格大屏的协调人实现已完成、我独立验证通过（go 全量 19 包、node --check、jsdom 有数据/空项目两路控制台零报错），但用户说太久、暂时不做；且用户 Mac 实测 v0.4.1 截图显示当前版 hero 画布很丑（中央大青色光斑盖住一半画面、轨道节点标签糊成白块）。决定：精美版以后再做，本版改成普通 Tailwind + 卡片的运维仪表盘。
- 做法（在已验证的 v0.4.2 面板逻辑上改样式，不推倒重做）：① hero 画布整块去掉（hero.js 不再加载，serve_test.go 的 script 顺序断言同步改）；② 页面改 Tailwind Play CDN + 卡片布局：深色底、实体卡片、无辉光无渐变，顶部状态栏 + 指标卡片行 + 左智能体团/右信号流 + 热力格 + 下方原有详情面板（任务总览/详情、Channel、mem、健康、设计、journal 全部保留）；③ 保留已验证的面板数据逻辑（LAST SIGNAL、智能体团 5 行、热力格 5×12、信号流、底部 9 格指标、归档数与 API 对照）；④ Tailwind Play CDN 需联网（浏览器端加载），离线可用以后再处理。
- 不做：中央星环/轨道升级、token 面板、巡检蛛、任何模拟数据。
- 验证：go 全量 + node --check + jsdom 两路渲染（控制台零报错）；用户 Mac 实测后发版（发版另等用户指令）。

## v0.4.2 发布记录（2026-11-12）
- 主提交远端 `963d0c8`（本地 `a59a9f3`，5 个文件：serve 改 Tailwind 卡片运维仪表盘——hero 画布整块去掉、页面不再加载 hero.js（文件留着以后精美版再用），Tailwind Play CDN + 深色实体卡片、无辉光无渐变；保留已验证的面板数据逻辑 LAST SIGNAL/智能体团/热力格/信号流/9 格指标；hero-caption 文案改成中性真实说明），tag 对象 `a7af48a`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.4.2，6 个资产；发布 run 43150423965、两路 CI（43150424060、43150398406）均 success 无抖动。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过；node --check app.js 过；jsdom 真实渲染验证（有数据/空项目两路）控制台零报错（Tailwind CDN 在 jsdom 里正常加载到）；云端浏览器打不开沙箱 localhost 做真实截图（预期：serve 只绑 127.0.0.1、无隧道），最终观感待用户 Mac 实测。
- 实测：不带参数一键装上 v0.4.2（`sunshine version` = 0.4.2）；v0.4.1 原版二进制 upgrade --dry-run 查到 v0.4.2（下载地址指向 v0.4.2 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`77aa316f…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.4.3：serve 改万表归一纸色风格（2026-11-12，用户定）
- 背景：v0.4.2 刚发深色卡片版，用户看后定不要暗黑风格，改用原来 Tailwind 方案里的风格（即《万表归一原站风格版》：基准文件 `~/workspace/your_files/wanbiao-guiyi-original-style.html`，Tailwind Play + 原站内联设计系统 token，纸色 #f7f4ee、墨色、衬线标题），卡片也用这种风格。深色霓虹仅限后台大屏的例外就此取消。
- 做法（只换视觉，数据逻辑不动）：① 页面底色改纸色，卡片改白底/米白底 + 彩色顶边（品红/金色，照基准文件的卡片样式），标题改衬线体，正文墨色/灰色——具体 token 以基准文件为准，不要凭印象编色值；② 状态灯、徽标、热力格、火花线配色改成与纸色协调的版本（热力格/火花线原来是 slate/blue，要换成纸色底上看得清的配色）；③ Tailwind Play CDN 保留；④ 详情面板、错误横条、页脚文案、全部 Alpine 绑定与数据逻辑原样不动。
- 不做：中央星环/轨道升级、token 面板、巡检蛛、任何模拟数据；不改布局结构（只换配色与字体）。
- 验证：go 全量 + node --check + jsdom 两路渲染（控制台零报错）；用户 Mac 实测后发版（发版另等用户指令）。

## v0.4.3 发布记录（2026-11-12）
- 主提交远端 `d844c11`（本地 `ceb1d72`，5 个文件：serve 改万表归一纸色风格——token 全部取自基准文件 `~/workspace/your_files/wanbiao-guiyi-original-style.html` 的 :root 与 Tailwind config（纸色 #f7f4ee、墨色、衬线标题、品红 #be185d/金 #b8860b 交替彩色顶边、强调 #c2410c），深色 slate 类全局换成纸色 token（110 处），热力格/火花线改强调色；数据逻辑与布局结构不动），tag 对象 `8549923`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.4.3，6 个资产；发布 run 43153159435、两路 CI（43153159341、43153143425）均 success 无抖动。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过；node --check app.js 过；jsdom 真实渲染验证（有数据/空项目两路）控制台零报错；真实浏览器截图仍做不了（沙箱无 Chromium、云端浏览器够不到沙箱 localhost），纸色观感待用户 Mac 实测。
- 实测：不带参数一键装上 v0.4.3（`sunshine version` = 0.4.3）；v0.4.2 原版二进制 upgrade --dry-run 查到 v0.4.3（下载地址指向 v0.4.3 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`61989360…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.4.4：卡片顶边改六色循环（2026-11-12，用户问“卡片就只有两种颜色吗”）
- 背景：v0.4.3 卡片顶边只在品红/金两色间交替，用户问是不是只有两种颜色。基准文件 `~/workspace/your_files/wanbiao-guiyi-original-style.html` 本身用六色循环（`const palette = ['#7c3aed','#1d4ed8','#0f766e','#c2410c','#be185d','#b8860b']`，各 stage 按文档顺序轮换），serve 只取了其中两色。
- 做法：9 张指标卡 + 13 个 section 按文档顺序轮换六色（紫 #7c3aed、蓝 #1d4ed8、青 #0f766e、橙 #c2410c、品红 #be185d、金 #b8860b），新增 panel-purple/blue/teal/orange 与 metric-purple/blue/teal/orange 八个类（style.css，色值照基准 palette）；数据逻辑、布局、其它样式不动。
- 验证：go 全量 + node --check + jsdom 两路渲染（控制台零报错）；用户 Mac 实测后发版（发版另等用户指令）。

## v0.4.4 发布记录（2026-11-12）
- 主提交远端 `9ee9731`（本地 `260b9c5`，4 个文件：serve 卡片顶边改六色循环——9 张指标卡 + 13 个 section 按文档顺序轮换六色（紫/蓝/青/橙/品红/金，色值照基准 palette），style.css 新增 panel-purple/blue/teal/orange 与 metric-purple/blue/teal/orange 八个类，serve_test.go 断言补 4 个新 panel 类；数据逻辑与布局结构不动），tag 对象 `1874c81`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.4.4，6 个资产；发布 run 43155496547、两路 CI（43155496508、43155451283）均 success 无抖动。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过；node --check app.js 过；jsdom 真实渲染验证（有数据/空项目两路）控制台零报错，六色计数齐（指标卡紫2蓝2青2橙1品红1金1，section 紫2蓝2青2橙3品红2金2）；真实浏览器截图仍做不了（沙箱无 Chromium、云端浏览器够不到沙箱 localhost），纸色观感待用户 Mac 实测。
- 实测：不带参数一键装上 v0.4.4（`sunshine version` = 0.4.4）；v0.4.3 原版二进制 upgrade --dry-run 查到 v0.4.4（下载地址指向 v0.4.4 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`59c0d903…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.5.0（实施说明）：Native hook 埋点——智能体团与热力格亮起来（2026-11-12，用户定“直接做 hook 埋点”）
- 背景：用户在 Mac 上跑 v0.4.4 看 serve，后台显示当前任务 hello 接口与工程基座、信号流里 implement.jsonl/check.jsonl/task.json 都有更新，但智能体团 5 个角色全是“待命”、活动热力格全空。已核实是设计使然不是 bug：智能体团只认所选 channel 的 worker（`app.js clusterRoles`，无 worker 即灰灯待命），热力格只数 channel 事件（`heatEvents` 明确排除 mtime，caption 也写着“mtime 文件变化不计入”）；native 模板的项目没有 channel，implement/check 是主会话直接派的 Claude Code 后台子代理，不走 channel、不注册 worker，serve 只读文件连不上 agent，所以看不到。小修方案（热力格与智能体团改吃 mtime 文件变化）已被用户否掉，定直接做 hook 埋点。设计文档表二第 1/2 行与 §6 第 6 条早已把这项排进 v2（“v1 先用 mtime 活动流顶着”），现在做。
- 目标：native 模式下子代理一派发，智能体团对应角色亮“工作中”、热力格对应格子着色；子代理一结束，灯灭。数据必须是真的启停事件，不许用“文件刚改过”冒充正在跑。
- 事件落盘：新文件 `.sunshine/.runtime/activity.jsonl`（append-only JSONL，`.sunshine/.runtime/` 已在 .gitignore 忽略清单里）。每行字段：ts（RFC3339）、kind、agent（implement/check/research/prototype/main）、task（任务目录名）、session_id（有则写）、source="hook"。kind 本批只写两种：`subagent_started`、`subagent_stopped`；字段名与 kind 集合预留扩展（任务生命周期事件以后再加）。写盘只增不减会膨胀：上限由实现定一个数（如最近 1000 行或 7 天）并写进本节，写盘时顺手截断；serve 只读最近 N 条。解析失败的行跳过，不报错。
- Start 事件：在 `RunSubagentContext`（`internal/hook/subagent.go`）成功注入上下文后追加一次写盘（agent、任务目录名、时间）。只在真正注入时写（agent 已识别且 context 非空）；写盘失败静默丢弃。铁律不变：hook 任何情况下不得以非零码退出、绝不阻断 agent 会话；`HooksDisabled()` 时不写；无 `.sunshine/` 时不写。
- Stop 事件：新增 `sunshine hook subagent-stop` 子命令（`internal/cli/hook.go` 分发里加一项，Long 里的事件列表同步），Claude 模板 `templates/platforms/claude/settings.json` 加 `SubagentStop` matcher（command 为 `SUNSHINE_HOOK_PLATFORM=claude sunshine hook subagent-stop`，timeout 照现有 30 秒）。难点：SubagentStop 的 payload 不含 agent 类型，关联办法由实现定，优先方案是读 payload 的 transcript_path 前若干行，从注入提示词的角色头（`# Implement Agent Task` 等）与 `Active task:` 取 agent 与任务目录；退路是按当前任务 + 最近一个未结束的 start 事件匹配（顺序派发下成立），并在实现里写明该退路的成立条件与失效时行为（匹配不上只记 session_id，serve 忽略未匹配的 stop）。Codex 等平台：核查其 hook 系统有无 SubagentStop，有则按同法跟进，无则记为后续；本批至少覆盖 Claude（用户实测环境）。已接 `subagent-context` 的平台现为 claude/codebuddy/droid/codex/cursor（settings.json/hooks.json），逐个核查。
- Prototype：把 `sunshine-prototype` 加进 `internal/hook/subagent.go` 的 `agentsAll`（设计文档 v2 要求）。但上下文注入按 v0.3.11 的 token 规则：prototype 只认任务、不注入 implement/check 式的大上下文（brief 由主会话在提示词里给，hook 只记 start 事件；如需注入，只给最小 brief——需求列表 + 概要设计两条路径）。实现前重读本 PLAN.md v0.3.11 节与 `templates/platforms/claude/agents/sunshine-prototype.md`，不得破坏“父任务 prd.md 索引、子代理只读需求列表 + 概要设计”的既定规则。
- Serve：新增 native 活动读取（读 `.sunshine/.runtime/activity.jsonl`），`/api/activity` 合并第三路 source="native"（条目形状与 channel 条目一致：ts/text/source，text 写明角色与启停，如 `implement 已启动`）。运行中推导：有 start 且无对应 stop → 运行中，但必须有防 stale 规则（start 超过阈值仍无 stop → 视为未知，回落待命并在 caption 说明；阈值由实现按实测定，不得无上限显示运行中）。`mergeActivityEntries`（`internal/serve/activity.go`）扩展为三路合并，排序与 30 条上限不变。
- 前端（`internal/serve/static/app.js`）：`heatEvents()` 计入 native 事件（role 按 agent）；`clusterRoles()` 无 channel worker 时回落到 native 推导（运行中→绿灯/工作中，taskLine 复用现有逻辑）；“运行中 Worker”指标卡计入 native 运行中的子代理数（native 与 channel 不会同时有同一角色在跑，写明该假设）。caption 必须同步改：智能体团 caption 加 native hook 事件来源说明；热力格 caption 改成“channel 事件 + native hook 事件；mtime 文件变化仍不计入”。主会话行本批不动（仍显示当前任务行 + 待命灯，不许用提示词心跳冒充主会话在跑）。
- 安装与迁移：`sunshine init` 新项目自动带上 SubagentStop 接线；已有项目由实现核查 `sunshine update` 是否刷新 `.claude/settings.json`（哈希清单与平台文件更新路径），能安全合并则走 update（只补缺失的 SubagentStop 项，不动用户已有配置），不能则给出明确的手动步骤并写进本节与教程。用户 Mac 上的 demo 项目（已有项目）必须覆盖，否则用户实测看不到效果。
- 测试：hook 包单测（start/stop 写盘、prototype 识别且不注入大上下文、写盘失败静默、HooksDisabled 时不写、非 Sunshine 目录不写、subagent-stop 关联成功与匹配失败两路）；serve 单测（native 读取、合并排序、运行中推导 + stale 回落）；jsdom 两路渲染 + 新增 native 活动夹具（一个 implement 已启动未结束 → 智能体团 implement 显示工作中、热力格对应格子着色；空 activity → 仍显示待命）。验证仍按既定动作：go build/vet/gofmt 全过、go test ./... 全过、node --check、jsdom 真实渲染。
- 不做：任务生命周期事件（create/start/finish/archive）本批不写盘（留给实时日志面板）；token/耗时统计不动；不改 task.json schema；不改 channel 事件 kind；不碰 mtime 活动流逻辑；不改“运行中 Worker”以外的指标卡口径。
- 版本号：发版时定，建议 v0.5.0（新功能）。
- 【已实现，未提交/未推送/未发版，2026-11-12】实现完成，本地验证全过，等用户定夺提交与发版。改动文件 19 个：新建 internal/hook/activity.go、activity_test.go、subagent_stop.go、subagent_stop_test.go、internal/platforms/settings_merge.go、settings_merge_test.go；改动 internal/hook/subagent.go、internal/cli/hook.go、internal/serve/activity.go、handlers_misc.go、serve_test.go、static/app.js、static/index.html、internal/update/update.go、update_test.go、internal/platforms/platforms_test.go、templates/platforms/claude/settings.json、codex/hooks.json、cursor/hooks.json、templates/sunshine/.gitignore。
- 【关键修正，与本节初稿不同】SubagentStop payload 实际含 agent 类型：按 Claude Code 官方 hooks 文档（code.claude.com/docs/en/hooks，2026-11-12 抓取），SubagentStop 输入含 `agent_type`、`agent_id`、`agent_transcript_path`、`stop_hook_active`、`last_assistant_message`（`transcript_path` 是主会话的 transcript，`agent_transcript_path` 才是子代理的；matcher 可按 agent type 过滤）。本节初稿“payload 不含 agent 类型”的说法已过时。实现改为：首选 payload 的 agent_type；transcript 扫描（前 256KB，角色头 `# Implement Agent Task`/`# Check Agent Task`/`# Finish Agent Task`/`# Research Agent Task` 与 `.sunshine/tasks/<名>/` 或 `Active task:`）作旧版宿主退路；当前任务 + 最近未结束 start 作最终退路；agent 也取不到才不写盘。Codex 经官方文档核查有 SubagentStop（payload 同样带 agent_type，已接线）；Cursor 经核查有 subagentStop（camelCase，payload 带 subagent_type，已接线）；CodeBuddy/Droid 未能核实有无 SubagentStop，本批不接线，记为后续。
- 【数值与阈值】写盘上限 1000 行（写盘时顺手截断）；serve 只读最近 500 行；运行中 stale 阈值 2 小时（初值：demo 里 implement 一轮约 2 分钟，留足余量，待真实使用后按实测再调）；无 start 绝不显示运行中；stale 或 ts 非法回落待命。
- 【更正】本节初稿称“`.sunshine/.runtime/` 已在 .gitignore 忽略清单里”不准确：模板原先只有 `*.log`，本次在 `templates/sunshine/.gitignore` 补了 `.runtime/`。
- 【update 合并结论】`sunshine update` 能安全刷新 settings.json：新增 `internal/platforms/settings_merge.go`（保序 JSON 解析，按 command 去重，把模板里有、用户文件里缺失的 hook 组追加到对应事件末尾，用户已有配置一行不动，解析失败返回错误）。未改动的文件走自动更新（含 statusLine 保留）；改动过的文件冲突覆盖时走合并补缺失 hook 组；合并失败回落模板全文。手动步骤只需留给 update 覆盖失败或用户选跳过的情况。
- 【验证】go build/vet/gofmt 全过；go test ./... 20 个包全过（含新单测：hook 写盘/读盘/坏行跳过/截断 1000 行/UnendedStart 配对、start 事件、prototype 只记事件不注入、HooksDisabled/无 .sunshine/ 不写、写盘失败静默、Codex start、subagent-stop 三路关联、Cursor payload；serve native 读取/合并排序/运行中推导 + stale 回落；platforms claude/cursor/codex settings 断言 + settings_merge 补 SubagentStop/已有不重复/坏 JSON 报错；update TestRunMergesMissingHooksOnConflict）；node --check app.js 过；jsdom 真实渲染三路全过（有数据/空项目/native 活动夹具，控制台除已知 jsdom canvas getContext 未实现外零报错）：native 夹具（真实 hook 命令生成：implement 启动未结束、check 启动后结束）→ 智能体团 implement 显示工作中、LAST SIGNAL“implement 已启动”、信号流首条角色 implement、运行中 Worker=1、今日活动=3、`/api/activity` native_running=[implement]、heatEvents() implement 事件数 1（30 秒窗口内）。
- 【遗留/未实测】真实 Claude Code/Codex/Cursor 上的启停事件端到端（沙箱无这些宿主，只做了 hook 命令级实测与单测）；旧版 Claude 无 agent_type 时的 transcript 退路；并行派发同一角色时的错配边界（退路按最近未结束 start 配对，顺序派发下成立，并行同角色可能错配）；CodeBuddy/Droid 的 SubagentStop 待核实接线。

## v0.5.0 发布记录（2026-11-12）
- 主提交远端 `4864b10`（本地 `ec2c490`，21 个文件：Native hook 埋点——子代理启停事件写进 `.sunshine/.runtime/activity.jsonl`（上限 1000 行），新增 `sunshine hook subagent-stop` 与 `internal/hook/activity.go`、`subagent_stop.go`，`agentsAll` 加 prototype（只记事件不注入大上下文），Claude/Codex/Cursor 模板加 SubagentStop 接线，`internal/platforms/settings_merge.go`（settings.json 安全合并），serve `/api/activity` 合并 native 第三路 + 运行中推导（2 小时防 stale），`sunshine update` 冲突时合并补缺失 hook 组，`.gitignore` 补 `.runtime/`），tag 对象 `b5295af`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.5.0，6 个资产；发布 run 43167789539、两路 CI（43167789538、43167768068）均 success 无抖动。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过；node --check app.js 过；jsdom 真实渲染验证（有数据/空项目/native 活动三路）控制台零报错（除已知 jsdom canvas getContext 未实现），native 夹具 implement 启动未结束 → 智能体团显示工作中、运行中 Worker=1、native_running=[implement]；真实 hook 命令端到端：subagent-context 派发与 subagent-stop 结束均写盘、退出码 0。
- 实测：不带参数一键装上 v0.5.0（`sunshine version` = 0.5.0）；v0.4.4 原版二进制（从 Release 下载，沙箱 /usr/local/bin 在 VM 更换后已清空属预期）upgrade --dry-run 查到 v0.5.0（下载地址指向 v0.5.0 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`12cc934e…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.5.1 发布记录（2026-11-12）
- 主提交远端 `18f8c12`（本地 `1f02eaa`，3 个文件：init 启动打印 Sunshine 大字——FIGlet "Rebel" 字体预渲染的 "Sunshine"（8 行、最宽 79 列，node figlet@1.9.4，与 Trellis init.ts 的 figlet "Rebel" 做法一致），青色大字 + 灰色标语“开箱即用的 AI 编程工程框架”，`internal/cli/banner.go` 内嵌常量存放、`internal/cli/init.go` 的 RunE 开头调用；着色只在 stdout 是终端且未设 NO_COLOR、TERM 不是 dumb 时生效），tag 对象 `7dccd43`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.5.1，6 个资产；发布 run 43180143015、两路 CI（43180143121、43180003561）均 success 无抖动。本版同时含 README 按 Trellis 风格重写（远端 `08912ac`：中英同结构，加粗一句话 + 问题段落 + 徽章 + Why 表格 + Prerequisites + Quick Start + How to Use + How It Works + 六个场景 + 核心命令 + 配置要点 + 模板市场 + Resources + FAQ + 社区；旧 README 的 v0.3.11 顺手改成 v0.5.0）与 README 翻英文为主（远端 `ab3e441`：README.md 换成英文版，中文版改名 README.cn.md；英文教程只修两处漏翻，其余中文是 CLI 真实输出/引用文件/用户输入原文，按约定保留）。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过（含新单测 TestRenderInitBanner：着色/无色内容一致、行宽 ≤80 列、无行尾空格）；真实 init 跑出大字（管道无色）、伪终端出青色（`^[[36m`）、NO_COLOR=1 无转义码；两份 README 全部锚点核对通过、`**` 成对无吞字。
- 实测：不带参数一键装上 v0.5.1（`sunshine version` = 0.5.1）；v0.5.0 原版二进制（从 Release 下载，沙箱 /usr/local/bin 在 VM 更换后已清空属预期）upgrade --dry-run 查到 v0.5.1（下载地址指向 v0.5.1 linux amd64，试运行未做修改）；darwin arm64 包 checksum OK（`c9f72742…`，Mach-O arm64）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.5.2 发布记录（2026-11-12）
- 主提交远端 `871e99a`（本地 `158233e`，3 个文件：init 提问支持干净退出——Ctrl+C（survey terminal.InterruptErr）或选“退出”时打印“初始化已取消。”、退出码 0，不留文件；项目类型与模板选单加“退出（取消初始化）”选项，平台多选提示语加“按 Ctrl+C 取消”；`internal/cli/init.go`（isInitCancelled/handleInitErr/parseProjectTypeChoice，提问出错或 EOF 仍静默默认全栈）+ `init_template.go`（两处选单加退出）+ 新测试 `init_cancel_test.go`（7 个测试）），tag 对象 `65611487`，Release：https://github.com/BondLam/Sunshine/releases/tag/v0.5.2，6 个资产；发布 run 43197861154、两路 CI（43197861186、43197804522）均 success 无抖动。本版同时含用户名改名（远端 `d13c50a`：全仓 141 个文件 linrong1982 → BondLam，go.mod、全部 import、.goreleaser.yaml、install.sh、README、教程、PLAN.md；install.sh 查最新版改走跟随全部跳转取最终地址）。
- 验证：go build/vet/gofmt 全过；go test ./... 20 个包全过（含新单测：中断/退出判别、干净退出、选项映射、提问取消、模板选单退出）；伪终端真实验证 5 路全过（平台多选/模板选单/项目类型三处 Ctrl+C 均干净取消退出码 0、不留文件；项目类型选“退出”取消；正常选全栈完成 init）。
- 实测：不带参数一键装上 v0.5.2（`sunshine version` = 0.5.2）；v0.5.1 原版二进制 upgrade --dry-run 查到 v0.5.2（下载地址指向 v0.5.2 linux amd64，试运行未做修改；第一次撞上 GitHub API 匿名限流 403，10 分钟后重试通过；v0.5.1 旧二进制报错里的下载链接仍是旧用户名，是旧二进制内嵌的，新版已是 BondLam）；darwin arm64 包 checksum OK（`b77826f0…`，Mach-O arm64）；`go install github.com/BondLam/Sunshine/cmd/sunshine@v0.5.2` 与 `@latest` 均通（@latest 初次因本地模块缓存的版本列表滞后指到 v0.5.1，清缓存后通）。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## v0.5.3（实施说明）：implement.jsonl 条目加行范围字段，注入时只切那一段（2026-11-13，用户定）

**背景**：设计先行模式下每轮实现都要读原型文件。工作流的规矩是“jsonl 条目只读 reason 指明的部分，大文件先 grep 定位”（workflow.md:237），但执行靠 agent 自律——原型是一份文件，Read 工具一次就是整份读进来，原型一大每轮都读整份会撑上下文。用户定：给条目加行范围字段，机制上只读那一段。

**字段格式**：`"lines": "120-180"`，1-based 闭区间行号字符串，单行写 `"120-120"`。缺省（旧条目无此字段）= 不限制，整文件注入，旧条目零改动全兼容。目录型条目忽略 lines。格式：`^\d+-\d+$` 且 start>=1 且 start<=end。

**注入切片点（核心机制）**：`internal/hook/subagent.go` 是唯一注入实现方（`GetImplementContext`/`GetCheckContext` → `getTaskContext` → `getAgentContext` → `materializeJSONLEntries` → `materializeFile`，目前 `materializeFile` 读整文件、只按 `MaxFileBytes` 截断）：
- `jsonlEntry` 加 `Lines string`；`readJSONLEntries` 解析 `item["lines"]`（map 解析，无 json tag，与现有 File/Type/Reason 同款）。
- `materializeFile` 签名加 `lines` 参数：先按行切片（1-based 闭区间，`strings.Split` 按 `\n`），再走现有 `MaxFileBytes` 截断；切片后字节数计入总预算（已有 block 计费逻辑不动）。
- 越界：end 超总行数截到文件末尾（不报错，块头注明实际行号）；start 超总行数注入空块 + stderr 警告。格式非法：注入时降级为整文件 + stderr 警告（hook 铁律：绝不阻断 agent 会话），`validate` 侧报硬错误。
- 块头注明范围：`=== <路径> (lines 120-180 of 450) ===`；无 lines 时块头不变。`materializeDirectory` 调 `materializeFile` 时传空串。`materializeArtifact`（prd/design/implement.md）不动。
- 同一套解析与注入代码服务于 implement/check 两个文件：字段与机制对两个文件通用（check.jsonl 顺带获得同款能力，不另做区分）；用户定的是 implement.jsonl，workflow 文案里两个文件一起写。

**task 侧**：`internal/task/store.go` 的 `jsonlEntry` 加 `Lines string \`json:"lines,omitempty"\``；`internal/task/lifecycle.go`：
- `AddContext` 加 lines 参数；CLI `task add-context` 加 `--lines "120-180"` flag；目录型条目带 lines 时拒绝（明确报错）。
- `Validate` 的 `validateJSONL`：校验 lines 格式（正则 + start<=end，非法报硬错误）；文件存在时再校验 start<=总行数（start 越界报错误，end 越界报警告并提示注入时截到末尾）。
- `ListContext` 显示 lines（reason 行后附 `(lines 120-180)`）。
- `HasCuratedJSONLEntry` 不动（只判断有无真实条目）。

**serve 侧**：`internal/serve/handlers_tasks.go` 的 `jsonlEntry` + `readJSONLEntries` 加 Lines 字段透传显示即可（只读观测，显示条目原文，不切片）。

**workflow 模板**：`templates/workflows/native/workflow.md` 两处（先 grep 确认 tdd 与 channel-driven 模板是否复用同一段文字，有则同步改）：
- 1.4 格式行：`{"file": "<路径>", "reason": "<原因>", "lines": "<起始行>-<结束行>"}`，说明 lines 可选、1-based 闭区间、缺省整文件、只对文件型条目有效。
- line 237 规则句改为机制化表述：有 lines 的条目注入时只切那一段；无 lines 的大文件先 grep 定位。
- workflow.md 改了，`sunshine update` 的哈希清单会提示用户项目里的 workflow.md 有更新，属预期行为，走正常 update 流程。

**测试**：hook 侧新增单测（解析合法/非法/缺省、切片中间段/首行/末行、end 越界截断、start 越界空块、空 lines 整文件、注入块头含行号、目录条目不受影响、非法格式降级整文件）；task 侧（AddContext --lines 写入、Validate 格式错误/越界、ListContext 显示）。全量 go test 0 FAIL。真实注入实测：演示任务 implement.jsonl 写原型条目带 lines，跑注入命令生成子代理上下文，断言只含切片行（命令名以 `internal/hook/hook.go` 为准）。

**版本与交付**：功能改动，定 v0.5.3；已随 v0.5.6 发布（四批合发）。

## v0.5.4（实施说明）：原型按页拆文件——`doc/prototype/` 一页一个文件（2026-11-13，用户定）

**背景**：原型是最终的系统原型，可能很大。单文件的前提下，不管行号还是锚点，都是在给一个大文件内部寻址：行号会漂移，锚点要标记纪律加解析逻辑。用户问“拆解时是不是带上原型从哪里读、有没可能读错”后定：原型生成时就按页拆成多个文件，拆解/1.4 时子任务直接读对的文件。`lines` 字段不白费，仍留给其它长单文件（长规范、长设计文档）。

**新结构**：`doc/prototype/` 文件夹：
- `index.html`——导航入口（产品名 + 到各页的链接 + 一句话介绍），设计评审打开它点着看；
- 每页一个文件，文件名 = 页名（如 `申请页.html`、`审批列表页.html`）；
- 共享样式/mock 数据抽成共用文件（`assets/style.css`、`assets/mock.js`，各页引用；小原型每页自包含也可，由原型子代理定）。
- 旧的单文件 `doc/prototype/原型.html` 不再是约定；已有的旧原型继续可用（`lines` 仍支持），不强制迁移。

**改动点**：
1. 原型子代理定义：`templates/sunshine/agents/prototype.md`（channel 版）+ 17 个平台文件（claude、codebuddy、codex toml、copilot、cursor、droid、gemini、grok、kimi、kiro json、omp、opencode、pi、qoder、snow、trae、zcode）。平台文件是手工同步的（无生成器，与 canonical 文本并不完全相同，如 claude 版有原生子代理守卫句），每份都要改同一处意思：把“单文件系统原型 `doc/prototype/原型.html`”改为按页文件 + `index.html` 导航入口；汇报格式改成列出文件清单（每页一个文件）；“结构性大改时先读现有的原型文件再改”改为读 `index.html` + 相关页。
2. init 产品设计模板：`templates/design/prototype.html`（66 行单文件）拆成 `templates/design/prototype/index.html`（导航入口 + 介绍）+ `templates/design/prototype/示例页.html`（核心操作表单示例）；`internal/scaffold/scaffold.go` 的 designTemplates 映射改成四条（新增两条原型文件）；只写不存在的文件、不覆盖的规则不变。`templates.Design` 是 `all:design` embed，新文件自动进二进制。
3. workflow 模板三份：原型派发句（native:356、tdd:350、channel:223）“产出 `doc/prototype/原型.html`”改为“按页产出 `doc/prototype/`（每页一个文件 + `index.html` 导航入口）”；1.4 的 add-context 例子（native/tdd）原型条目改成页文件（去掉 `--lines`，`lines` 留给其它长单文件）；1.4 格式行保留 `lines` 说明。
4. CLI 例子：`internal/cli/task.go` 的 add-context `--lines` 例子改用长规范文件（不用原型文件举例，避免误导）。
5. serve：
   - `internal/serve/activity.go`：watchedDesignFiles 去掉单文件，`watchedFiles()` 里 ReadDir `doc/prototype/` 把各页 `.html` 加进盯梢（新增/删除页也会被活动流捕获；目录本身的 mtime 只反映增删，不反映页内编辑，故必须展开到文件）。
   - `internal/serve/handlers_misc.go` handleDesign：prototype 改成文件夹口径 `{exists: 目录存在，lines: 各页总行数，countText: "N 页"}`；docs 数组里 `"原型.html"` 改 `"原型/"`，带 countText。前端 designDocs 已支持 countText（有则优先显示），不用改前端；兜底分支核对一下也用 countText。
6. 测试：`internal/scaffold/scaffold_test.go` 与 `internal/cli/init_projecttype_test.go` 的单文件断言改成文件夹结构断言；v0.5.3 的 `lines` 测试不动（fixture 路径是任意长单文件，与原型结构无关）。
7. 教程 §12.6（中英文）：请假例子改按页结构——文件清单、需求一/二的 `implement.jsonl` 原型条目改成页文件、删掉 v0.5.3 刚加的那句 `lines` 说明（原型场景不再需要 `lines`；`lines` 的说明留在 1.4 与 add-context 帮助里）。“真实内容”处理：结构部分实跑更新（init 产品设计 + 建父子任务 + 写 jsonl，取真实输出）；演示核心（curl 校验等）不受原型布局影响，不动。

**版本**：v0.5.4；已随 v0.5.6 发布（四批合发）。

## v0.5.5（实施说明）：1.2 前置后端选型任务，选型以 `[待定]` 进任务、1.1 问答定（2026-11-13，用户定）

**背景**：拆解按原型页驱动，清单天然是前端形状；后端除接口外的工作（选型语言/框架/数据库/缓存、脚手架、建库、接缓存）没有任务、没有归属，implement agent 只能在自己那一页范围内现想。用户定：前置一个后端选型任务，开发进场读到待定就先问答、确定后再开始第一个任务。

**规则（1.2，三份模板同改）**：
- 拆解时除按原型页拆需求子任务外，还要判断后端基础工作量（选型、脚手架、数据库建模、缓存）。
- 默认：先建一个“后端基础：技术选型与脚手架”子任务，排在第一个需求子任务前面；后端选型项（语言、框架、数据库、缓存，按需再加消息队列、对象存储等）以 `[待定]` 标在它的 prd 里。它的 1.1 读到 `[待定]` 先问答（1.1 已有“一次只问一个问题、优先给选项”的问答机制，不用新写），开发人员确定后记进 prd/design，再做脚手架、建库、接缓存；归档后需求子任务再启动（依赖写进需求子任务的 prd.md，机制现成）。
- 小项目：后端量小、单独一个任务是 overhead，待定项写进第一个需求子任务的 prd，它的 1.1 先问答定选型再动手。
- 依据按“有什么读什么”：有概要设计时从模块划分、数据、技术选型三节拆；没有时按父任务 prd.md 建。概要设计仍不是必须的（1.1 已写明设计文档“按项目与设计者自定，有没有都行”）。
- 选型定了以后不再改：后续子任务沿用；要改走需求变更流程（回炉）。

**概要设计模板**（`templates/design/outline.md`）：在“数据”后、“验收标准”前加“技术选型”一节：只写约束（已有系统、必须用的库/数据库、性能要求）与待定项（`[待定]` + 待谁定），不强制设计人员拍板技术栈；空着也行。

**教程 §12.6（中英文）**：在拆解段落后补一小段后端规则（规范性文字，不动演示的真实内容；不谎称请假演示当时就这么做的）。

**版本**：v0.5.5；已随 v0.5.6 发布（四批合发）。

## v0.5.6（实施说明）：brainstorm 加需求维度清单与排序规则（2026-11-13，用户定）

**背景**：1.1 问询的质量靠 agent 自己判断：覆盖全靠它觉得哪个维度重要就问哪个，漏了也没人提醒；“最高价值”也没有明确的排序规则。用户定：加维度清单 + 排序规则。

**改动**（`templates/platforms/common/skills/brainstorm.md` 与 copilot 变体 `templates/platforms/copilot/prompts/brainstorm.prompt.md`，两份是平行全文、须同步改）：
- **需求维度清单**（新一节，放在“提问规则”后、“思考框架”前）：八个维度——用户与场景、功能边界、数据、性能、安全、兼容性、验收、范围外；每轮提问前对照清单，看哪个还没覆盖（已确认 / 已明确划在范围外 / 不适用，三者之一才算覆盖），没覆盖的进决策清单；不适用的维度直接标不适用，不硬问。清单只保证不漏，不保证每个问题都问得准；问得准还是靠判断，最终评审兜底。
- **排序规则**（新一节，与维度清单并列）：每轮只问一个，从决策清单里按顺序挑——1. 阻塞后续工作的先问（不答没法往下走的）；2. 不可逆的先问（选错了返工成本大的）；3. 成本高的先问（影响范围大的）；4. 枝节后问（细节、偏好、可以后补的）。
- **接线**（两处小改，让清单进循环）：规划流程步骤 4 “单个最高价值问题”后加“（按排序规则挑）”；步骤 5 “重新计算决策清单”改成“对照需求维度清单重新计算决策清单”。
- 不动：提问规则本身（证据先查、每问四要素、每轮只问一个）、第一性原理框架、收敛门禁、最终评审；教程不写 brainstorm 内部规则，不用改。

**版本**：v0.5.6；已随 v0.5.6 发布（四批合发）。

## v0.5.6（发布记录，2026-11-13）：四批合发

- **主提交**：远端 `38b68c8`（本地 `bb002d4`），tag 对象 `1bc0f18`，Release https://github.com/BondLam/Sunshine/releases/tag/v0.5.6
- **内容**：v0.5.3 implement.jsonl 行范围、v0.5.4 原型按页拆文件、v0.5.5 1.2 前置后端选型任务、v0.5.6 brainstorm 维度清单与排序规则（四批一个提交、一个 tag）。
- **CI**：发布 run 43348838970 success；持续集成两路，一路 43348788555 success，一路 43348839058 failure——`TestSupervisorInterruptIdle` 时序抖动（channel/supervisor 包，本批未动；本地重跑 5 次全过；属 v0.2.1 `TestSupervisorClosedLoop` 同类抖动）。API 重跑失败任务报 403（token 无重跑权限），未重跑；同提交的另一路 CI 已过，确认是抖动不是代码问题。**教训**：token 无 Actions 重跑权限，重跑失败任务需在 GitHub 网页上点重跑，或给 token 加 Actions 写权限。
- **验证**：一键装 v0.5.6（linux/amd64）报版本 0.5.6；旧版二进制 `upgrade --dry-run` 查到 0.5.6；darwin arm64 checksum 对（e3c802bb…）、Mach-O arm64 对；Release 6 资产齐。
- 其后 PLAN.md 发布记录补提交一笔（远端 SHA 见 GOAL.md）。

## 教程维护（2026-11-13）：§12.6 补四批改动 + 场景人性化（只改教程，不发版）

**背景**：v0.5.3–v0.5.6 四批已发，但教程 §12.6 只跟进了其中三批的皮毛：v0.5.4 漏了一句旧话（“系统原型是一份文件”与按页结构对不上）、v0.5.5 的后端规则是孤立的抽象段落、v0.5.6 的提问机制完全没进教程；§12.6 的对话还一次问四个问题，与“每轮只问一个”对不上。用户定：四批维护进教程，场景要更像真实工作、例子更现实，不必要的不加、要精练好上手。

**改法**（中英文同步，只动 §12.6，六处旧开场提示词与版本标注一个字不动）：
- **对话一次问一个（v0.5.6）**：§12.6 开场对话改成先问一个决定拆分的阻塞问题（提交和审批是两个需求还是一个），其余问题叙述带过——一次一个、每次给选项、答一个记一个，答案直接进需求列表和概要设计。原先两轮各四个问题的问答块删掉，改后对话比原来短，答案不丢（都在下面的设计包节选里）。
- **闭环一览第 2 步补维度清单（v0.5.6，人话版，不写 skill 内部规则）**：提问一次只问一个，先问堵住后面工作的，再问细节；问之前先对照八个方面看哪个还没问到——谁用、做什么、不做什么、数据、性能、安全、兼容、怎么验收。
- **修旧话（v0.5.4 漏的）**：“系统原型是一份文件，两个页面 + 页面间的导航”改成“`doc/prototype/` 里的一组页面 + `index.html` 导航入口”，一个原型多个需求、各占一页的说法保留。
- **后端规则贴回例子（v0.5.5）**：孤立的抽象段落改成贴着请假小工具讲——后端只有一个 `server.js`，量小，待定项直接写进需求一的 prd；量大（选语言、建表、接缓存）才前置“后端基础”子任务。
- **第 3 步补限定（2026-11-12 待定项，顺手落定）**：“交接物：`doc/` 里的三份设计文档。”后加一句——本例的三份，不是硬性清单；设计产出由设计者定，`doc/` 里有什么、评审就审什么、拆解就依据什么，没有设计文档就是纯代码任务。
- **精简两处**：拆解产出物一节删掉父/子任务 `task.json` 的两段 JSON 引用（目录树注释里已经写了，是纯重复）；“关闭重开”一节的 `task context` 输出用 `……` 省略中间行，只留路径与状态。
- **不动**：§12.1–12.5 的对话仍是“一次问几个问题”的简写示意，这次不统一改（全改会把教程拉长）；§4 的 implement.jsonl 示例与 §12.6 变更任务的 jsonl 里仍有源码文件（`src/Refund.vue`、`src/server.js`）——那是真实实跑输出，与 1.4“代码文件不进 jsonl”对不上，改等于改写实录，这次不动，记在这里等用户定夺。

## 教程场景重写（2026-11-13）：§12 开头说明段删除 + 12.1 改 web 商城（只改教程，不发版）

**背景**：用户逐个过六个场景，先改两处：① §12 开头的验证说明段（v0.3.0、/tmp/scenes/、任务目录日期、实录说明）多余、让用户困扰，删掉；② 12.1 不用“值班表小工具”举例，改 web 商城，只演示第一个需求接着第二个需求，之后不说“以此类推”。

**改法**（中英文同步，只动 §12 开头 + 12.1；12.2–12.5 的开场提示词一个字不动）：
- §12 开头：整段验证说明删掉（表格式的场景一览保留）。场景内的“示例提示词（示例，不是真实会话记录）”与“会话示意（对话是示例；命令与输出是真实实跑值）”标注还在，诚实标注不断。
- 12.1 改 web 商城（实跑验证，非编造）：演示目录 `/tmp/scenes/s1-webstore`，`sunshine init -y`（空目录默认全栈，三份规范），需求一“商品列表”（首页展示名称、价格、库存，三件商品先写死在代码里），需求二“加入购物车”（点按钮放进购物车，`GET /api/cart` 看件数和总价）。对话一次问一个（先问商品数据放哪、购物车放哪这类决定做法的问题）；输出全是真实跑出来的：`curl /api/products` 的三件商品 JSON、加购后 `GET /api/cart` 的两件商品与总价 388、归档后 `[2026-11] - 2 个任务`。结尾只说两个需求走完项目里有了什么，不说“以此类推”。
- 12.2 改存量项目（实跑验证）：单文件 `src/server.js` 改成更像跑了两年的样子——入口、退款接口、数据读写分三处（`src/server.js`、`src/routes/refunds.js`、`src/db.js`），依赖仍只有 `express`，init 仍只认后端、只生成 `system.md` 与 `backend.md`，git status 仍是三个新增目录；删掉两处“审计日志出现 1 次”这种数数式验收（规划里改成“三条约定都写进 spec 了”，验证改成直接贴写进去的两段 + 归档清单）。
- 12.2 再改：用户定六个场景都用 web 商城举例，退款接口版作废，改成跑了两年的 web 商城（`src/server.js` 入口、`src/routes/` 商品和订单接口、`src/db.js` 数据读写，价格按约定用“分”存）；三条约定改成接口格式、价格用“分”、端口 3000，实跑重验一遍（init、git status、写 spec、归档输出与退款版一致）。
- 12.3 改 web 商城（实跑验证）：任务改成“下单必填收货地址”，演示项目就是 12.2 那个跑了两年的商城（`/tmp/scenes/s3-cross`，另加 `public/checkout.html` 结算页）；对话一次问一个（先问缺地址返回 400 还是 200）；输出全真实：缺地址 `{"code":"ADDRESS_REQUIRED","data":null}` + HTTP 400、带地址下单成功订单里有 address + HTTP 200、结算页含 `name="address"`、归档后 `[2026-11] - 1 个任务`。channel 两条协调消息实发（`channel messages` 显示序号 2、3，创建事件里带任务路径），等 worker 回复 10 秒超时是真实复现的。另：本版按 workflow 1.4 写上下文清单——implement/check 两份 jsonl 都只放 `backend.md` 接口契约，代码文件（checkout.html、orders.js、db.js）由 agent 实现时自己读，旧退款版把三份代码文件列进 implement 的写法不再用。
- 12.4 改 web 商城（实跑验证）：本来就是商城价格函数（单价 100 分、满 10 件九折、VIP 九五折），改法是点名商城（购物车和订单总价都走它）+ 对话一次问一个（先问钉哪几档，再问重构边界）；两遍测试都实跑全绿（`ok shop 0.003s`，前后一致）；收尾补归档清单。另：`workflow --template native` 切回时报“已有本地修改”，需加 --force（切走再切回即被当作本地修改）；实跑小插曲：第一次 `go test` 忘加 Go 的 PATH 没跑成、重构用的 python 替换没命中（静默无操作），改成整文件重写 + grep 核对后才拿到两遍真实绿灯。
- 12.5 改 web 商城（实跑验证，按用户定的方案：401 那条留着，项目换成商城）：bug 是商城项目派 codex worker 报 401；对话一次问一个（先问是不是同一类问题，再问复现条件）；复现在沙箱重跑了一遍（`/tmp/s5-repro`：建 channel、发原话、拉起 worker，worker.log 里是 `failed to connect to websocket: HTTP error: 401 Unauthorized, url: wss://api.openai.com/v1/responses`，与旧文的 `unexpected status 401 ... /v1/responses` 是同因不同端点）；`mem search 401` 在沙箱重跑命中 01a1c3d8（1 次，原话）；任务描述里写清会话编号，教训写进 system.md，归档后 `[2026-11] - 1 个任务`。
- 12.6 改 web 商城（实跑验证，两轮演示）：请假小工具改成商城优惠券（需求一 下单用优惠券、需求二 优惠券列表）；设计包、原型 mock 三条券、两轮开发、满减门槛 100→200 变更全部重跑。第一轮验证了券设计（四档 curl 全对）但漏了两步（每子任务先提交代码、回写 spec），且 /tmp 被清、演示目录消失；第二轮从零重建商城（s2-old 已被清，按行为规格重写），补上两步：功能提交 7634b3b / 51920ca / 06a179c，spec 回写（backend 优惠券接口节、frontend 优惠券节），归档 b67ebfc / e15f11c / 69da5c3 / eb0b8c3。实值：券码 NOPE → 400 COUPON_INVALID、1 件机械键盘实付 28900、1 件无线鼠标 400 COUPON_THRESHOLD、2 件无线鼠标实付 16800；变更后 2 件鼠标 400、1 件键盘仍 200 实付 28900；GET /api/coupons 三条券；父任务 [0/2]→[1/2]→[2/2]，归档 3→4 个任务。六个场景至此全部改完。注意：变更任务的 implement.jsonl 这次是设计文档（需求列表 + 概要设计），不再是旧文的 src/server.js——12.6 变更任务的 jsonl 与 1.4 对不上问题已顺带修掉（§4 的仍在）；概要设计.md:36 结算页行仍写“满 100 元减 10 元”，变更三处没含它，属演示残留；init --help 文案过期（称产品设计不生成前后端规范，实际生成三份 spec）；首次 archive 未自动提交是因演示仓库缺 git identity（手动按同格式补提交 b67ebfc，后两次自动提交正常）；变更后订单 id 又是 1（服务重起内存订单清零），教程里只引实付金额不引订单号。

## GitHub 用户名改动（2026-11-13，已改代码，随 v0.5.7 发布）

用户把 GitHub 用户名由 BondLam 改为 **linrong007**（https://github.com/linrong007），仓库现为 https://github.com/linrong007/Sunshine。2026-11-13 用户点选全仓改名：代码与文档里的 BondLam 已全部改成 linrong007（Go 模块路径 `github.com/linrong007/Sunshine`、仓库地址、install.sh、`.goreleaser.yaml`、README 链接等，共 142 个文件；PLAN.md 里发版记录与 2026-11-09 定址等历史记录保留原样）。改名提交随 v0.5.7 一起发版（tag v0.5.7 移到改名提交后重新触发 Release 工作流）。新提交的 git 身份改用 linrong007（已记 AGENTS.md）。

## 教程流程统一（2026-11-13）：六个场景按真实流程重写（只改教程，不发版，已推送远端 c1ce0dc）

**背景**：用户贴了自己 Mac 上周三跑的 Gin 登录项目真实会话记录，指出教程的流程是错的/被压缩了。真实流程：问需求（一次一个问题）→ Claude 给结构化「最终规划总结」（目标/范围内/范围外/关键决策/风险·延期项/产物状态）请用户确认 → 批准后规划产出首个实现任务的 PRD + spec 三件套（零产品代码、不 task start）→ 用户再批准 → 启动任务、派 `sunshine-implement` 子代理实现 → 实现完成派 `sunshine-check` 子代理核查（发现问题直接修复）→ 3.3 回写 spec → 3.4 起草提交计划、展示一次、请一次确认再提交（不 amend、不 push）→ 3.5 提醒运行 `/finish-work` → 用户确认后 finish-work 归档（当前任务 + 用户确认的其他已完成任务）并记会话日志；最终 git log 顺序是工作提交 → 归档提交 → 日志提交。归档时当前分支就是 base_branch 会被拒，需先切功能分支 + `task set-branch`。用户在 options 里选了“都统一按这个步骤”（六个场景都统一），各场景按工作流各有差异、总体同一流程，由我自行判断，不再追问。

**依据**：工作流模板 `templates/workflows/native/workflow.md` 第 1/2/3 阶段 + `templates/platforms/common/commands/finish-work.md` 已核对：1.0 只 create 不 start（start 留到 1.5）；1.1 一次问一个、先定模式（设计先行/直接开发）；1.2 拆解只做一次、子任务各自走 1.1→1.5；1.4 jsonl 只放规范/研究/`doc/` 设计产物、不放代码文件；1.5 激活前做需求评审（整包规划产物列出请用户确认——这就是「最终规划总结」的对应物），确认后才 `task start`；2.1 派 implement、2.2 派 check（最后一轮须全范围）；3.3 回写 spec（即使“没有要更新的”也要走一遍判断）；3.4 提交计划只展示一次、请一次性确认；3.5 提醒 `/finish-work`；finish-work 步骤 1 看 `task context --mode record`（一并归档其他已完成任务）、步骤 2 给脏路径分类（当前任务的未提交改动直接退出、回 3.4）、步骤 3 归档、步骤 4 `add-session` 记日志（`--commit` 只用 3.4 的工作提交哈希，不含归档提交）。

**重跑**：六个场景全部按各自流程从零重跑（协调人子代理 + 六个场景子代理，`sunshine` 用源码构建的 v0.5.6，git 身份 linrong007），取真实输出；演示目录 `/tmp/scenes/s1-webstore`、`s2-old`、`s3-cross`、`s4-refactor`、`s5-repro`、`s6-design`。新提交 SHA：12.1 工作提交 98d1e5d（骨架）/acaa3e3（商品列表）/ba61066（加入购物车），归档 71f3de9/e961e60/01d6eba，日志 eb7adc7/2d87dc7，归档 3 个任务；12.2 工作提交 0476a72，归档 b8fcd1a，日志 f0c95ed，归档 1 个任务；12.3 工作提交 da27e80，归档 c956c40，日志 c229570，归档 1 个任务；12.4 工作提交 f73b1e3，归档 50c33dc，日志 2dc676c，归档 1 个任务；12.5 工作提交 9a4b2bd，归档 38775f6，日志 c29bd5e，归档 1 个任务；12.6 工作提交 057abbd/d44083b/759af17，归档 be797d4/b0044a9/444e009/3d18edf，日志 e4ac2b2/eb9b771/1618fe9/42afb6f，归档 4 个任务。旧 SHA（7634b3b/51920ca/06a179c/b67ebfc/e15f11c/69da5c3/eb0b8c3）已全部换掉。

**各场景流程映射（已写入教程）**：
- 12.1 新项目：规划任务 `11-13-webstore` 产 spec 三件套 + 首个实现任务 `11-13-product-list` 的 PRD（零代码、不启动）→ 批准 → start + implement + check → 3.3 无更新 → 3.4 两条提交 → finish-work 归档商品列表 + 规划任务 → 加入购物车同样流程（PRD → start → implement → check → 3.3 无更新 → 提交 → finish-work）。归档时 `main` 就是 base_branch 被拒，切 `feat/product-list` + `set-branch` 解决。
- 12.2 写规范任务：Q&A 五问 → 最终规划总结 → 批准 → PRD 在规划阶段写好，spec 三行在 `task start` 之后由 implement 写入（写 spec 就是这个任务的实现，1.5 要求实现等 start 之后；与“规划产出 PRD+spec（不 task start）”的口径张力按后者执行，教程里已明说）→ check 对照代码逐条验证 + 实跑验证（GET /products、POST /orders totalPrice 89700）→ 3.3 无新增 → 3.4 一条提交 → finish-work（归档先被拒，切 `docs/old-shop-conventions` + `set-branch` 后通过）。任务 `11-13-old-shop-conventions`。
- 12.3 跨层：Q&A 五问 → 最终规划总结 → 批准 → PRD + 上下文（各 1 条，只放 backend.md 契约）→ 批准 → start（切 `feature/address-required` + `set-branch`）→ channel 两条协调消息（seq 2、3）→ implement 四层（先 API 再 UI）→ channel wait 10 秒超时复现 → check 两档 curl（400/200，空串与空白地址同样 400，系实现按“必填”语义定的、问答未单独问，已写进 spec）→ 3.3 backend.md 新增一节 → 3.4 → finish-work。
- 12.4 重构（TDD）：Q&A 两问 → 最终规划总结 → 批准 → PRD + 上下文 → 批准 → 切 TDD 模板 → start（`refactor-price` + `set-branch`，start 前已设好）→ 特征测试先行（第一遍即绿灯 `ok shop 0.003s`——老代码本身就输出这三档值，特征测试只能钉住现有行为，不可能红；不要写跑红）→ 重构 → 第二遍 `ok shop 0.011s`（含编译），`-count=1` 重跑 `0.003s` → check（签名未变、调用方未变、vet/fmt 干净）→ 3.3 backend.md:17 新增“价格计算”→ 3.4 → finish-work → 切回 native 报错需 `--force`。
- 12.5 修 bug：mem 搜 401 命中 01a1c3d8（2026-11-10 05:05，相关度 1.000，命中 1 次，用户原话）→ Q&A 两问（本机、codex 未登录，`ls ~/.codex/auth.json` 不存在）→ 最终规划总结 → 批准 → PRD（写清会话编号 01a1c3d8，4 条验收）+ 上下文（各 2 条）→ 批准 → start（切 `fix/codex-worker-401` + `set-branch`）→ implement 复现（建 channel、发原话、拉起 codex worker；worker.log 401 行 `failed to connect to websocket: HTTP error: 401 Unauthorized, url: wss://api.openai.com/v1/responses` 共 7 次，根因行 `unexpected status 401 Unauthorized: Missing bearer or basic authentication in header`，worker 自行重试 `Reconnecting... 2/5` 至 `5/5`，channel wait 终态 error 事件 seq 16，随后 `channel kill --as worker`）→ check 全绿（mem 搜 401 复现后变 2 命中，新增本次复现的 01a1d311-99c；01a1c376 仍不命中，边界保持）→ 3.3 无其他新知识 → 3.4 一条 `docs:` 提交（只改 spec）→ finish-work。教训写进 system.md“多 Agent 派发”三条（登录前置检查、401 即未登录不要反复重派、复现记录）。
- 12.6 设计先行：1.1 问答 → 设计包（doc/，原型按页拆文件）→ 1.2 拆解（父 + 两子任务，`[0/2]`）→ 1.5 评审（即本节的“最终规划总结”）→ 批准 → 子任务各自走完整流程（批准 → implement → check → 3.3 → 3.4 提交计划 → 提交 → finish-work 归档该子任务）；需求一 057abbd + 归档 be797d4 + 日志 e4ac2b2，需求二 d44083b + 归档 b0044a9 + 日志 eb9b771（需求二 check 真发现一处问题：初版优惠券页是写死三条券的静态表格，与刚写的规范“数据调 GET /api/coupons，不写死 mock”不符，已改成 fetch 渲染并复验；3.3 判断无新增内容、未改 spec）；父任务集成复核（五档全绿）后 finish-work 归档（444e009 + 1618fe9）。变更（门槛 100→200）：1.1 只问 delta 一个问题 → 设计返场改三处（需求列表.md:23、概要设计.md:41、结算页.html:39，另同步了同一文件里的验收标准、待确认事项、变更记录与概要设计数据节）→ 评审只审三处 → 建任务时 `--parent` 已归档父任务报错“父任务无法解析”，改成独立任务 `11-13-满减门槛改成-200` → implement（db.js threshold 10000→20000、backend.md 补 200 元/20000 分一行——变更前 backend.md 从未写明具体门槛值、checkout.html:12 门槛提示同步改）→ check 两档（2 件鼠标 400、1 件键盘 200/28900，GET 再验 threshold 20000）→ 3.4 → 759af17 → finish-work（3d18edf + 42afb6f），归档共 4 个任务。券名与门槛名不副实（MAN100JIAN10 的 name 仍是“满100减10”，threshold 已是 20000；未要求改名，概要设计数据节注明，代码与 GET 输出未改）。

**重跑发现的跨场景共性问题（工具/文档层，记在这里，未改产品代码，等用户定夺是否修）**：
1. 归档自动提交在沙箱新仓库里普遍失败（12.1、12.2、12.4、12.5、12.6 均遇 `Author identity unknown`——sunshine 内部 `git commit` 不带 `-c`，沙箱无全局 git 身份）；均按工具内提示手动提交已暂存的归档内容（同款消息），随后设仓库本地身份，后续自动提交恢复。12.3 的归档自动提交一次成功。真实用户机器一般已有全局身份，不会遇到。
2. `add-session` 留下的 `.gitattributes`（journal `merge=union` 规则）普遍未进提交：12.1（后随购物车工作提交一并提交）、12.2、12.3、12.4、12.5、12.6 均有 `?? .gitattributes` 残留——hook 建文件、日志提交不收它。
3. `implement.jsonl` / `check.jsonl` 创建时为空：六个场景一致——v0.5.6 的 `task create` 未预置 workflow.md 1.4 所说的 `_example` 行；各场景用 `add-context` 补真实条目后 `validate` 通过。
4. `.sunshine/.developer` 未被 gitignore：12.1、12.2 均核实 `.sunshine/.gitignore` 只有 `*.log` 与 `.runtime/` 两行，与 workflow.md 的说法不符。
5. **已归档父任务下建不了子任务（工作流规则层的重要发现）**：12.1（加入购物车被迫建成独立任务）与 12.6（变更任务被迫建成独立任务）都撞上。含义：规划时应按 workflow 1.2 一次性建好全部子任务，或父任务等全部子任务归档后再归档；“已归档不 reopen、新建任务走完整流程”在工具层面只能建成独立任务、挂不上已归档的父任务。教程 12.1 与 12.6 已把这条写进去（12.1 末尾的坑 + 12.6 评审波及面一节的“已归档”条）。
6. 12.4 的“跑红”不存在：纯重构第一遍测试即绿灯，教程“绿灯→绿灯”与此一致，不要写跑红。
7. 12.5 显示层改写：工具输出把 `Missing bearer or basic authentication in header` 显示为 `Missing bearer <redacted> basic authentication in header`（`or`→`<redacted>`），`bearer token`→`bearer <redacted>`；磁盘原文已用 `od -c` 核对，教程引文一律按磁盘原文。
8. 12.2 的口径张力：任务文本一处写“批准后规划产出 PRD + spec（不 task start）”，另一处写“写 spec 就是实现”。执行按后者——PRD 在规划阶段写好，spec 三行在 `task start` 之后由 implement 写入（workflow 1.5：实现须等 start 之后；规划阶段写 spec 等于 start 前实现）。教程采用后者口径并已明说。
9. 12.3 的基线取舍：s6 的基线 orders.js 已含地址处理与 total，不能当 §12.3 前的基线用；本次用最简回显版（items 原样、无 total），否则下单成功输出会多出 total、与教程逐字输出对不上。
10. 12.6 变更任务 PRD 的 `[变更 2026-11-13]` 行与需求列表 R-001 改后行：协调人报告只给了改动描述，未逐字引用改后整行；教程里 R-001 改后行最初是按改动描述（满 200 元减 10 元、验收同步：2 件鼠标由实付 16800 改为 400 COUPON_THRESHOLD、保留 1 件鼠标 400 项）重建的。2026-11-13 已按实跑文件（`/tmp/scenes/s6-design/doc/design/需求列表.md:23`）逐字核对，补上缺的“满 200 减 10 的”，中英文教程现与实跑文件逐字一致。

**改动**：只改 `docs/tutorial.md` 与 `docs/tutorial.en.md` 的 12.1–12.6 六节（各 685 行变更，两文件共 1194 插入/176 删除），开场提示词一个字未动；六个场景的开场提示词与 §2–§4 主线（退款例子）未动。只改教程、不发版。

## 教程 12.6 推倒重做（2026-11-13）：全新商城从零做，设计先行（只改教程，不发版，已推送远端 993a57c）

**背景**：用户指出 12.6 应从新开始、从 0 到 1 演示一个全新项目怎么流转闭环——旧版 12.6 是从现成商城底稿（0fae77b，按 12.2 底稿重建）起步、往已有项目上加优惠券，不算从零开始。范围经 options 确认：整个商城从零做（带优惠券），不是只做优惠券；门槛 100→200 变更演示默认保留。旧版 12.6 整节替换（含此前未推送的 R-001 行补丁——旧节已不存在，补丁随之作废，无需单独推送）。

**重跑**：单场景子代理从零重跑，演示目录 `/tmp/scenes/s6-new`（旧演示 `/tmp/scenes/s6-design` 只读参考，未动）；`sunshine` 用源码构建的 v0.5.6，git 身份 linrong007，无 amend、无 push。开场提示词（逐字）：“我想从零做一个带优惠券的 web 商城，走设计先行：先把需求和设计定下来，评审通过后再动手写代码。商城要有商品列表、加入购物车、下单结算，下单时能用优惠券抵扣，还要有一个优惠券列表页能看到每张券的门槛、面额、状态和有效期。技术栈还没定，你先帮我把需求问清楚。”

**新 12.6 结构**：init（产品设计，基线 `3ba152f`）→ 1.1 问答八问（Q1 五条全做、Q2 只定约束选型留给后端基础、Q3 按给定数据、Q4 四档券校验、Q5 地址必填、Q6 券码随订单提交、Q7 券列表五字段、Q8 范围外六项；八维度全覆盖）→ 设计包落 `doc/`（需求列表 54 行：R-001 商品列表:23、R-002 加入购物车:24、R-003 下单结算:25、R-004 下单用优惠券:26、R-005 优惠券列表:27；概要设计 85 行：接口 GET /api/products:45、POST /api/orders:46、GET /api/coupons:47，技术选型:62–71 四项 `[待定]`；原型 4 页 + index + mock.js，R-003 与 R-004 共用结算页）→ 1.5 最终规划总结请确认 → 1.2 一次拆 6 个子任务（后端基础在前 + 五条需求各一）→ 逐个子任务完整走完（实现 → 检查 → 回写规范 → 提交 → 归档）→ 父任务集成复核后归档 → 变更演示（门槛 100→200）。

**各子任务实跑值**：后端基础（选型 Node.js/Express/内存/无缓存，1.1 四问；工作 `1e3fc51`，规划产物提交 `9ad44e3` 随其 3.4 第一笔提交，归档 `0866131`，日志 `a1b867a`）；商品列表（工作 `4134922`，归档 `7e4d47b`，日志 `df6838b`；GET /api/products 三件商品 JSON）；加入购物车（工作 `60f7b93`，归档 `b05f8df`，日志 `fdf8ca8`；页面 200/2595 字节，node 验算 1 键盘 + 2 鼠标合计 47700 分）；下单结算（工作 `a88a79e`，归档 `5d9ce45`，日志 `9f34611`；缺地址/空地址 → 400 ADDRESS_REQUIRED，带地址 1 键盘 → 200/total 29900）；下单用优惠券（工作 `346c7d2`，归档 `91106ee`，日志 `220790d`；六档 curl 全对：NOPE 400、已使用 400、已过期 400、1 键盘 200/28900、2 鼠标 200/16800、1 鼠标 400）；优惠券列表（工作 `08ff4d4`，归档 `e71f8c2`，日志 `99d36ab`；GET /api/coupons 三条券 JSON；末轮 2.2 全范围回归——单仓项目 `task context --mode packages` 无包可列，改为回归全部接口）；父任务集成复核（六档全绿）后归档 `257b690`、日志 `d120f25`（`--commit` 记规划产物 `9ad44e3`，父任务无独立工作提交）。最终 git log 25 条提交，归档 8 个任务（7 主循环 + 1 变更）。

**变更演示（门槛 100→200）**：`task create --parent 11-13-商城` 报错“父任务无法解析”（已归档父任务下建不了子任务，符合预期），改独立任务 `11-13-满减门槛改成-200`；设计返场 8 处（需求列表 R-004:26 与变更记录:54、概要设计:29/:50/:59/:77、结算页原型:36、mock.js:8）；implement 3 处（db.js:12 threshold 10000→20000、backend.md:21 新增行、checkout.html:35 门槛提示）；curl 验证（2 鼠标 → 400 COUPON_THRESHOLD，1 键盘 → 200/28900，GET /api/coupons threshold 已 20000）；工作 `eec9467` → 归档 `7648b07` → 日志 `ba86c65`。券名名不副实（MAN100JIAN10 的 name 仍“满100减10”，threshold 已 20000，概要设计数据节注明，未改名）。

**口径定夺（已按工作流规则定，写进教程，请用户确认）**：`[变更 2026-11-13]` 不记进已归档父任务的 prd.md——归档的是历史，不 reopen；旧演示已归档父任务的 prd 同样没有这一笔。`[变更]` 只记在变更任务自己的 prd.md 与设计文档的变更记录里。另：本轮下单用优惠券是 R-004（五条需求），不是旧演示的 R-001，教程引用行号一律用新编号。

**重跑发现（与上一轮对照）**：归档/add-session 自动提交本轮 8 次全部一次成功——演示仓库事先已设本地 git 身份，旧轮“无 git 身份失败、手动补提交”的情况本轮没复现；`.gitattributes` 仍残留（8 次 journal 提交都没带进去，最终 `?? .gitattributes`）；jsonl 创建时仍为空（7 个任务全部 `add-context` 补条目后 `validate` 通过）；`.sunshine/.developer` 仍未被 gitignore，已随基线提交 `3ba152f` 进库；init 仍未生成仓库根 `.gitignore`（`node_modules/` 是后端基础子任务新建的）；`task start` 在沙箱 7 次均为降级模式（会话身份不可用），状态照常翻转，未阻塞。

**改动**：只改 `docs/tutorial.md` 与 `docs/tutorial.en.md` 的 12.6 一节（各 378 行，两文件共 514 插入/584 删除），12.1–12.5 与六个开场提示词未动；§2–§4 主线（退款例子）未动。只改教程、不发版。其后（同日）用户指出新 12.6 开场提示词未点名 sunshine 工作流（只说“走设计先行”），经核查 12.1–12.5 中英文都有“请按 sunshine 工作流走”、仅 12.6 漏了；已只改 12.6 开场提示词中英文（补上工作流调用及设计先行交付物：PRD、doc/ 设计文档、评审后拆子任务），其余内容未动，未提交未推送。

## 设计先行改按需求循环（2026-11-13，已发布 v0.5.7，含全仓改名）

**背景**：用户指出 12.6 设计阶段被一笔带过——问答后直接“设计包落盘”，没展示设计过程；并定调设计过程应跟开发一样按需求循环、像搭积木（每次对话产出一轮需求、产出一轮原型），概要设计等整系统文档等全部需求和原型做完再产出。

**新模型（已实施）**：
1. 不先定范围（范围无法先定）：设计人员无法事先确定做哪几条需求——边做需求边做原型，后续还可能继续收集新需求进来。所以没有“一轮问答先定范围”这步。循环从开场提示词里已有的需求起步（起始批次，不是定死的范围）；每轮对话产出一条新需求（澄清、落盘、原型），新需求随时可以进来；共用约束（价格分存、接口格式、端口）与共用数据（商品、券）在首次需要时定下来（通常在前一两轮），不是先定范围；范围外也是边做边界定。循环到设计人员与用户都认为当前批次够了、可以评审时停下来（不是范围被定死了），进评审与拆解；评审后来的新需求走需求变更/新增流程（迷你一轮 1.1 → 1.2 → 1.4 → 1.5）。
2. 按需求循环（每条需求一轮）：设计人员先产出需求列表（问答问清该条细节，一次一个，优先给选项，答案立即记 prd.md；该条写进 `doc/design/需求列表.md`，R-xxx 行）→ 再根据需求列表做原型：派 `sunshine-prototype` 子代理，给需求列表（当前状态）+ spec 下的前端样式约束（如 `.sunshine/spec/frontend.md` 里的设计约束）+ 已有原型目录 `doc/prototype/`（保一致：共用 mock.js、index.html 导航、样式），产出或更新该条对应的页，汇报页数、mock 数、路径；共用页（如 R-004 用 R-003 的结算页）由子代理在已有页上加，不重建；下一条，直到设计人员与用户都认为当前批次够了、可以评审。
3. 交付客户的文档（循环结束后按需生成）：全部需求和原型做完后，按需生成交付客户的文档——不一定是设计概要，概要设计只是其中一种可能，由设计者按客户需要定；要出概要设计时再写 `doc/design/概要设计.md`（模块、接口、数据、技术选型 `[待定]`、验收、风险），依据完整的需求列表 + prd；技术选型 `[待定]` 仍留给后端基础子任务。
4. 评审（1.5，不变）：整包请确认。5. 拆解（1.2，不变）：一次拆完整个范围，但拆解依据是需求列表——每个子任务对应一条需求（做什么、原型文件是哪个）。6. 开发（不变）：子任务各自走全套，逐个激活。

**需求列表是主线与契约**：需求列表贯穿全流程（问答 → 落盘 → 原型 → 拆解 → 开发），是产品与开发人员的契约——设计按它产出原型，拆解按它定子任务（做什么、原型文件是什么），开发按它实现与验收。

**相对现行的变化**：范围不再先定（设计人员无法事先确定做哪几条需求，边做边收集，循环到当前批次够了再评审）；需求列表从“整包一次写”变“每轮先产出一条”，且定位为主线与契约（贯穿设计 → 原型 → 拆解 → 开发）；原型从“一次派发、读两份完整文档、产全部页”变“每轮派发、读需求列表当前状态 + spec 前端样式约束 + 已有原型目录、产/改一页”；概要设计从“和需求列表一起写、必出”变“循环结束后按需生成，不一定是设计概要”；拆解依据明确为需求列表（每条需求 → 一个子任务，写清做什么与原型文件）。评审、拆解一次、开发不变。workflow.md 里“不要做好一个页面拆一次”说的是拆解（1.2）不是设计，改时要把两句分开写清。

**实施（已完成，已随 v0.5.7 发布）**：workflow 模板三份（native、tdd、channel-driven）的 1.1 设计先行节；brainstorm skill（`templates/platforms/common/skills/brainstorm.md`，规划流程步骤 8 的“一次拆完整个范围，不要做好一个页面拆一次”是拆解规则、不用动，改的是设计先行模式描述）；prototype agent 定义 18 个文件（`templates/sunshine/agents/prototype.md` + 17 个平台文件，输入从“两份设计文档”改成“需求列表 + spec 前端样式约束 + 已有原型目录”，并说明“产/改一页、保一致”）。共 23 个文件、176 处替换（脚本 `/tmp/apply_loop_model.py` 两阶段校验后写盘，另 4 处 frontmatter 手工补改）；验证：旧模型文字 grep 全清零、新文字在位、kiro JSON 与 codex TOML 可解析、`go build/vet/gofmt/test ./...` 全过；二进制重建 `/tmp/sunshine-v056-loop`（版本串仍 v0.5.6），在 /tmp/loop-check 实测 `init -y` 产物含新工作流文字。copilot 的 brainstorm.prompt.md 经核查不含设计先行内容，未列入。

**重跑与教程重写（已完成，已随 v0.5.7 发布）**：子代理用新二进制从零重跑 12.6（演示目录 `/tmp/scenes/s6-loop`，报告 `/tmp/s6-loop-report.md` 474 行），主会话独立核对演示目录并逐条 curl 复验关键结果（变更后 2 鼠标 400 COUPON_THRESHOLD、1 键盘 200/28900、GET /api/coupons threshold 20000、缺地址 400 ADDRESS_REQUIRED 均实测通过；另在 7901f3e/8c03060 工作树复验下单与券列表的逐字 JSON）。重跑结果：设计循环 5 轮（需求列表 45→46→47→48→49 行，原型 4 页 + 导航入口，R-004 轮 0 页共用结算页；概要设计 78 行、技术选型四项 `[待定]`，开场提示词点名要才写）；1.5 评审通过、1.2 一次拆 6 子任务（规划产物提交 `b4bf106`）；6 子任务逐个走完（工作/归档/日志 SHA 见报告 §11.1；3.3 六次均为“无 spec 更新”，选型与接口记在任务 prd/design 与概要设计里）；父任务集成复核归档 `13c9f37`；门槛 100→200 变更演示（已归档父任务下建子任务复现报错，改独立任务；设计返场 6 处、实现 3 处，工作提交 `3f80cf9`）；最终 git log 25 条、8 任务全归档、工作区只剩未跟踪 `.gitattributes`。工具问题对照：归档/add-session 自动提交本轮 8 次全过（事先设了 git 身份，上轮无身份失败未复现）；`.gitattributes` 残留、jsonl 创建为空、`.developer` 未被 gitignore、`task start` 无会话记录（本轮 `.sunshine/.runtime/` 目录根本不存在）、init 不生成根 `.gitignore` 全部复现；本轮新发现两处子代理 exec 报 `subprocess.output_drain_timeout`（exitCode 0，汇报完整、结果已独立复核）。教程 12.6 中英文已按重跑结果整节重写（各 417 行，旧节 379 行；开场提示词一个字未动；§2 的设计/开发角色段同期补进），新旧 SHA 与行号均以重跑为准，旧版 12.6 的 SHA（如 `9ad44e3`、`257b690`）与旧行号已清零。另：同日用户认可“设计人员与开发人员是一套工作流的两个阶段，提示词不用点名角色、只点名工作流和模式，角色跟着阶段走，唯一要明确说的是范围边界”这一回复，已总结进教程 §2 中英文（紧接“提示词要写明按 sunshine 工作流走”的注意段之后）。

**发布 v0.5.7（2026-11-13，已发布，已验证）**：主提交两笔——设计先行改按需求循环（本地 `f73e658` → 远端 `db86aa5`，25 个文件）与全仓改名 BondLam → linrong007（本地 `b769481` → 远端 `ac7dcca`，142 个文件、239 处；PLAN.md 里 47 处历史记录保留原样）。tag v0.5.7 对象 `d4b1af5` → `ac7dcca`。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.7，6 个资产。发布 run `43383896867` success。验证：changelog 含改名与设计循环两笔主提交；darwin arm64 checksum 对上（`a6eefaef…`）；Mach-O 64-bit arm64 确认；一键装（新 linrong007 地址）装上 v0.5.7；v0.5.6 旧二进制 `upgrade --dry-run` 查到 0.5.7（下载地址已是 linrong007）。首轮发布曾失败：tag 指向 `db86aa5` 时 run `43381946424` 在 goreleaser 发布步失败（只建了空草稿，已删），CI 两路均 success；改名后（`.goreleaser.yaml` 的 owner 由 BondLam 改成 linrong007）重新触发即成功，改名很可能是失败根因（未读到工作流日志，未完全证实）。另：`go install goreleaser` 在沙箱失败（/tmp tmpfs 512MB 空间不足），已改下预编译 v2.12.1 备用（/tmp/goreleaser-bin/goreleaser），最终没用上。

## 设计先行两处提问问题（2026-11-13，用户全流程实跑发现，已发布 v0.5.8）

**背景**：用户在 Mac 上从零跑带优惠券的 web 商城（设计先行），开场提示词点名 sunshine 工作流 + 设计先行 + 需求问清 + PRD + doc/ 设计文档 + 评审后按需求拆子任务。1.1 提问阶段发现两个问题：

1. **问了“是真实商城还是演示项目”**。用户指出：开场提示词里已说“按 sunshine 工作流走”“做产品设计”，设计环节即已确认，不用再问。既然是产品设计（设计先行），原型肯定是多页面 HTML 带演示 mock 数据的版本，“真实还是演示”不是设计阶段要问的。
2. **问了“前端用什么技术栈”**。用户指出这是遗漏点：原型只会是简单 HTML，真正开发进场后才要确定前后端用什么技术栈。技术栈（前后端）不在设计阶段问，留到需求拆解成任务的时候——第一个任务就要分前后端的技术栈，现在只分后端（v0.5.5 的“后端基础：技术选型与脚手架”只覆盖后端选型，前端选型没有任务承接）。

**根因**：
- brainstorm skill（`templates/platforms/common/skills/brainstorm.md`）的“先定模式”一句只说设计先行时按需求循环产原型，没写明原型默认是演示 mock 版、不用问真实/演示；也没写明技术栈不在设计阶段问。需求维度清单里的“用户与场景（谁用、在什么场景下用）”会把 agent 引向“真实还是演示”这一问；用户开场提示词里“技术栈还没定，你先帮我把需求问清楚”又把 agent 引向问技术栈。
- workflow 模板三份（native、tdd、channel-driven）的 1.2“后端基础”一节只建后端选型任务；`templates/design/outline.md` 的技术选型节也只有后端四项（语言、框架、数据库、缓存），没有前端项。

**改法（已定）**：
- brainstorm skill 的“先定模式”一句补两条：设计先行时原型默认是带 mock 数据的演示版（多页面 HTML），不用问是真实项目还是演示——开场提示词已说“按 sunshine 工作流走”“做产品设计”，设计环节即已确认，原型形式不用再确认；前后端技术栈也不在设计阶段问，留到 1.2 拆解时由第一个任务定。
- 三份 workflow 模板的“后端基础”一节改成“前后端基础”：默认先建一个“技术选型与脚手架”子任务排在第一个需求子任务前面，前后端选型项（前端：框架、构建工具；后端：语言、框架、数据库、缓存，按需再加消息队列、对象存储等）以 `[待定]` 标在它的 `prd.md` 里；它的 1.1 读到 `[待定]` 先问答，确定后记进 `prd.md` / `design.md` 再做脚手架、建库、接缓存；归档后需求子任务再启动。“拆解只做一次”一句里的“后端基础子任务的选型”同步改成“技术选型与脚手架子任务的选型”。量小 overhead 的兜底（待定项写进第一个需求子任务）同步改成前后端。
- `templates/design/outline.md` 技术选型节：提示语改成“选型在 1.2 拆解出的技术选型与脚手架子任务里定（前后端一起定）”；选型项补前端两项（前端框架、构建工具），后端两项改名明确（后端语言、后端框架），数据库、缓存不变。
- 不动教程 §12.6：它是 s6-loop 真实跑的记录（当时第一个子任务就是后端基础），改等于改写实录；新工作流只影响以后跑的项目。

**当前这轮的绕法**：用户 Mac 上这轮跑的是 v0.5.7（旧工作流），两处问题在这一轮里还会问，回答默认值即可继续——第一问答“演示（mock）”，第二问答“原型用简单 HTML，技术栈等拆解时第一个任务再定”。改完发新版后升级才生效。

**追加（2026-11-13，用户点选“模式必须问”）**：用户又指出，提示词里没提设计时这版好像直接跳开发——模式没说清就默认，等于把设计阶段悄悄跳过了。已定：模式必须问。brainstorm skill 与三份 workflow 模板的“先定模式”一句同步改成硬性要求——开场提示词已说清的不重复问；没说清时，必须先问用户一句再往下走，不许默认（尤其不许默认直接开发）；可以检查 `doc/` 是否已有设计文档辅助判断，但不能拿它代替问。这次只改了这一句（brainstorm 1 处 + 三份 workflow 各 1 处），其余不动；验证：go build/vet/test 全过，init 实测新文字在位（workflow.md 与两份 skills）。

**发布 v0.5.8（2026-11-13，已发布，已验证）**：主提交两笔——设计先行两处提问问题修复（本地 `e65ece5` → 远端 `a5e440c`，6 个文件）与模式必须问（本地 `8362a99` → 远端 `331edff`，5 个文件）。tag v0.5.8 对象 `a43417e` → `331edff`。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.8，6 个资产。发布 run `43392424428` success。验证：changelog 含两笔主提交 + 两笔 docs 提交；darwin arm64 checksum 对上（`cb667201…`）；Mach-O 64-bit arm64 确认；一键装（linrong007 地址）装上 v0.5.8；v0.5.7 发布版二进制 `upgrade --dry-run` 查到 0.5.8（下载地址已是 linrong007）。

## 教程 §12.6 开场提示词修正（2026-11-13，只改教程、不发版）

**背景**：用户在 Mac 上用奶茶商城提示词跑设计先行时，agent 把需求列表、概要设计、原型页都推迟到评审后才做子任务（评审批的是计划、不是设计包），偏离按需求循环的正常逻辑。追查发现用户奶茶提示词照抄了教程 §12.6 的开场提示词——那两句关键话一字不差（“先向我提问，把不清楚的产品逻辑和技术决策问清楚；确认后，帮我创建任务、写好 PRD，再把需求列表、概要设计和系统原型落到 doc/ 里”）。§12.6 的实跑（s6-loop）本身是对的，是因为跑的那只子代理任务书里直接写了循环做法，没按开场提示词的字面意思走；但摆在最前面让用户照抄的那句示例提示词是错的，照抄就会走到偏路上。

**改法**：中英文 §12.6 开场提示词同步重写为正确结构——设计阶段按需求循环（每条需求先提问问清产品逻辑，一次一个、优先给选项，答案立即记 prd.md，该条写进 `doc/design/需求列表.md` 的 R-xxx 行，再按需求列表派 prototype 子代理做/改对应页；原型默认多页面 HTML 演示版带 mock 数据，不用问真实还是演示；设计阶段不问技术栈）；循环结束后按需生成交付客户的文档（概要设计是可能之一），然后给最终规划总结请确认（这时三件套已在 `doc/` 里）；评审通过后按需求拆成子任务再动手实现。删掉了“技术决策”与“确认后……再把……落到 doc/ 里”两处措辞。§12.1 的直接开发提示词不动（直接开发的任务 1.1 问技术决策是合理的，不是设计阶段）；§12.6 正文与实跑记录不动（只改开场提示词，不改写实录）。

**验证**：旧句“确认后，帮我创建任务、写好 PRD，再把需求列表”中英文 grep 清零；新提示词中英文均在位；§12.6 的“技术决策”/“technical decisions”清零（剩余在 §12.1 直接开发提示词与 §3.3 规范更新说明，均属合理）；两份教程各 +6 行（1 行变 7 行多段引用）。

## 固化三处流程顺序（2026-11-13，已发布 v0.5.9）

**背景**：用户在 Mac 上用奶茶商城提示词跑设计先行时，agent 把需求列表、概要设计、原型页推迟到评审后才做子任务，偏离按需求循环。追查确认是开场提示词的措辞把顺序带偏了（“确认后……再把……落到 doc/ 里”），而 §12.6 教程开场提示词有同款问题（已先行修正，只推教程不发版，远端 02c612e）。用户问“还有哪些需要在全流程里固化”，经盘点：已固化的有模式必须问、原型默认演示 mock、设计阶段不问技术栈、按需求循环、拆解只做一次、需求列表是契约、原型派子代理、技术选型与脚手架第一个、1.0 同意、1.1 一次一问、3.4 提交确认、平台派发规则等；值得补的有两处顺序类缺口（子任务生命周期顺序、问答即记），而 1.0 同意、1.5 评审、3.4 确认三处是“门”（用户可主动 waive），不固化。

**改法**：三处顺序全部定成“固定的，不随开场提示词的措辞而变”，提示词只决定做什么（项目内容、范围边界、要点名的交付物），不决定流程顺序；提示词描述的顺序与流程不一致时按流程走，并在最终规划总结里向用户说明。
1. **设计阶段流程固定**：按需求循环（每条需求问清 → 记 prd.md → 写进需求列表 → 派 prototype 子代理做/改对应页），循环结束后按需生成交付客户的文档，再给最终规划总结请确认（这时三件套已在 doc/ 里），评审通过后才拆解。brainstorm skill 先定模式段补一句；三份 workflow 模板“按需求循环”段后各加一段。
2. **子任务顺序固定**：一个子任务的生命周期顺序固定：实现 → 检查 → 回写 spec → 提交 → 归档；检查验实现、回写 spec 在提交之前、归档在最后一步；提示词说“实现完直接归档/提交”也不跳过检查与回写。三份 workflow 模板各在主流程行后加一段（native/tdd/channel-driven 流程行措辞各异，分别锚定）。
3. **问答即记**：每答完一个问题立即记进 prd.md；提示词说“问完再一起记”也不推迟。brainstorm skill 规划流程步骤 5 补一句；三份 workflow 模板“流程固定”段后各加一段。

**验证**：三处新规则 grep 在位（流程固定 4 处、子任务顺序固定 3 处、问答即记 4 处）；go build/vet/gofmt/test 全过（20 包）；init 实测新二进制，.sunshine/workflow.md 三处各 1、.claude/skills/sunshine-brainstorm/SKILL.md 两处各 1。

**发布 v0.5.9（2026-11-13，已发布，已验证）**：主提交一笔——固化三处流程顺序（本地 523cbc8 → 远端 5662531，4 个文件，20 插入/2 删除）。tag v0.5.9 对象 d5b857f → 5662531。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.9，6 个资产。发布 run 43403535594 success。验证：changelog 含主提交 5662531；darwin arm64 checksum 对上（b6ffa66c…）；Mach-O 64-bit arm64 确认；一键装（linrong007 地址）装上 v0.5.9；v0.5.8 发布版二进制 upgrade --dry-run 查到 0.5.9（下载地址已是 linrong007）。

## 提问措辞统一为 brainstorm skill（2026-11-14，已发布 v0.5.10）

**背景**：用户让重写产品设计阶段工作流程并逐条核对，核对时指出“一次只问一个问题，优先给选项”不是要的效果。翻查确认：workflow 模板写的是“优先给选项，而不是开放式提问”，但 brainstorm skill 写的是“提出单个最高价值问题，附上你的建议与取舍”，而且明确说“建议不是默认选项，不要因为用户要求实现，就替用户选了推荐的产品决策”。两处意思不一样：列选项是让用户挑一个；给建议讲取舍，是把分析、建议、代价讲清楚，让用户自己定。用户定按 brainstorm skill 统一，立即改并发版。

**改法**：三份 workflow 模板里的“优先给选项”全部改成“附上你的建议与取舍”，并补“建议不是默认选项，不替用户选”限定。共 8 处：每份模板的 1.1 提问列表项、按需求循环段、技术选型段各 1 处（channel-driven 无提问列表项，只有后两处）；其中提问列表项与按需求循环段各补“建议不是默认选项，不替用户选”一句，共 5 处。教程 §12.6 开场提示词中英文同步改（“优先给选项”→“附上你的建议与取舍”，英文“options first”→“with your suggestion and trade-offs”），因为那是用户照抄的范例，要与工作流一致。PLAN.md 历史记录里的 3 处“优先给选项”保留原样（是过去的记录，不改写历史）。brainstorm skill 本身不用动（它本来就是对的）。

**验证**：workflow + 教程里的“优先给选项”清零（PLAN 历史 3 处保留）；“附上你的建议与取舍” workflow 里 8 处在位，“建议不是默认选项” 5 处在位；英文教程“options first”清零、“with your suggestion and trade-offs” 1 处在位；go build/vet/gofmt/test 全过；init 实测新二进制，.sunshine/workflow.md 里“附上你的建议与取舍” 3 处、“建议不是默认选项” 2 处、“优先给选项” 0 处。

**发布 v0.5.10（2026-11-14，已发布，已验证）**：主提交一笔——提问措辞统一为 brainstorm skill（本地 68719cf → 远端 e40bd45，6 个文件，18 插入/10 删除）。tag v0.5.10 对象 1ec676a6 → e40bd457。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.10，6 个资产。发布 run 43548519899 success。验证：changelog 含主提交 e40bd45；darwin arm64 checksum 对上（28c883a3…）；Mach-O 64-bit arm64 确认；一键装（linrong007 地址）装上 v0.5.10；v0.5.9 发布版二进制 upgrade --dry-run 两次均报 GitHub API 403 rate limit（未鉴权共享 IP 限流，环境问题，与发版无关），该路径未验证，其余口径均已通过。

## spec 索引层（2026-11-14，实施中，v0.5.11 候选）

**背景**：用户问 OpenViking 能不能替 spec、mem 是干什么的、spec 和 mem 怎么区分，聊到 OpenViking 主要是长期记忆和跨平台记忆；用户说那作用不大，他真正要的是减少上下文 token 消耗、类似 CodeGraph / Graphify 把 spec 压缩一下的功能。已核实 CodeGraph / Graphify 的机制：CodeGraph 用 tree-sitter 把代码解析成 AST，抽函数、类、调用、导入、继承关系存本地 SQLite，agent 查图谱不读文件，`export --compact` 只吐骨架 JSON（4.5 万 token 的代码库压到 4 千，约省 90%）；Graphify 把代码、文档、PDF、图片建成知识图谱，问的时候只把相关子图塞给模型，混合语料号称省 71.5 倍。关键区别：代码有 AST 可确定性抽取，spec 是 Markdown 散文，没有 AST；要把 spec 变成图谱，要么用 LLM 抽（花 token、可能抽错），要么 spec 一开始就写成结构化形式。

**做法（分三步，先做第一步）**：第一步 spec 索引层——每个 spec 文件头上加一张致密索引表（接口、数据模型、依赖关系各一行），agent 先读索引（几百 token），需要才钻进正文读细节；图谱只当导航、不当契约，正文永远是权威（“索引是导航不是替代，拿不准就读正文”）。第二步结构化 spec（接口、模型、依赖写 YAML / 紧凑 DSL，正文只留需解释的部分）。第三步接图谱工具（Graphify 这类对 spec 建图、让 agent 去查）。

**影响评估（已核代码）**：第一步零 Go 代码改动，只改文档和模板：①三份 spec 空白模板（`templates/sunshine/spec/*.md`）头上加索引节；②三份 workflow 模板各加一两句“先读 spec 文件头的索引，再按需读正文”；③`sunshine-update-spec` skill 加一句“更新正文时同步更新索引”；④教程写一句索引约定。依据：`scanSpecLayers`（`internal/task/context.go:512`）只列 spec 目录下的目录名、不读文件内容；注入预算按字节算（单文件 32768、总量 131072，`internal/hook/config.go`），不按内容算；`implement.jsonl` 存文件路径加行号，索引只是文件头上的几行，格式不变；工作流按路径引用 spec，路径不变。不变量：spec 三份文件路径不变、implement.jsonl / check.jsonl 格式不变、注入预算不变、工作流步骤不变、hook 机制不变；向后兼容——老项目没索引的 spec 照样用，agent 直接读正文，不用迁移。风险两处（流程风险非代码风险）：①索引漂移（改正文忘改索引），靠 3.3 `[required · once]` 更新步骤写死“同步索引”缓解；②索引质量（写得不好比没有更糟），工作流写明“拿不准就读正文”。第二步要新加解析器，context 组装、init、模板、update-spec 都要改，中等规模 Go 代码改动；第三步要多一个外部进程、MCP 接入、缓存增量更新和新故障模式，影响最大。

**验证**：已做——“## 索引”三份 spec 模板各 1 处在位；“规范索引”三份 workflow 模板各 1 处新增在位（native/tdd 另有 1 处是原有的 3.4 节“规范索引 Quality Check”，不是本次加的）；update-spec skill“同步更新索引表”1 处在位；go build/vet/gofmt/test 全过；init 实测新项目三份 spec 各含 1 处“## 索引”、workflow.md 含新增的规范索引指引。token 实测（tiktoken cl100k_base，奶茶商城 backend.md 实录 66 行）：全文 1742 token；索引节 257 token；做 R-004（优惠券）只读索引 + 优惠券一节 = 534 token，省 1208 token（69.3%）；再加数据模型一节 = 768 token，省 974 token（55.9%）。结论：单任务只读 1–2 节时，索引省 56–69% 的 spec token；任务要读的节越多，省得越少。

**续：自动维护与导入（2026-11-14，用户点名两件都要做）**：用户问“以后 spec 是不是不能随便乱写了、必须按规范写”，已答：散文还是随便写，索引只管导航不管内容；多的一条规矩是有索引就要同步维护（agent 在 3.3 里维护，人审）；索引可选，没索引的 spec 照样用。用户接着问能不能自动启动 skill 或命令自动格式化 spec 文档、不耗心智维护，以及有现成文档的人能不能调 skill/命令整理成带索引的规范化 spec。已定两件都要做：①CLI 命令 `sunshine spec index`（确定性、零 token）：读 spec 文件的 `##` 标题，自动生成/同步文件头上的索引节，保留旧索引里的描述（括号里的部分），新节只写标题；触发点是 3.3 更新规范后由 update-spec skill 指示 agent 跑一次，也可人随时手动跑、`--check` 只检查不同步不写（供 CI / pre-commit）。②Skill `sunshine-spec-import`（新文件 `templates/platforms/common/skills/spec-import.md`，装成各平台的 `sunshine-spec-import` skill）：agent 读用户现成的文档，理解内容后重组成 spec 格式并配索引；与已有的 `sunshine-spec-bootstrap`（从代码库分析起 spec）不同源（现成文档 vs 代码），不合并。两件都是新增，不动现有功能、不改 spec 格式、不影响已写好的 spec。

**两件验证（2026-11-14）**：`internal/specindex/` 新包（ParseSections/ParseIndex/BuildIndex/SyncContent/SyncFile/SyncDir/CheckFile/CheckDir，跳过 guides/）+ `internal/cli/spec.go`（`sunshine spec index`，`--check`/`--dry-run`/指定文件）+ root.go 注册；`autoSkillNames` 加 `spec-import`、`skillDescriptions` 加描述（未登记的 skill 名会报错，见 wrapSkillFrontmatter）；三个平台计数测试随之更新（reasonix skill 目录 13→14，E 路 codebuddy/copilot/droid 50/52/50→51/53/51，滚动轨道 16 平台各 +1）。命令实测（奶茶 backend.md 实录）：已同步时 `--check` 退出 0；加一节（限流与熔断）删一节（约束）后 `--check` 退出 1；同步后新节进索引、删节出索引、旧描述保留、正文不动、幂等；`--dry-run` 只显示不写盘；无索引节的文件自动插入。init 实测：新项目装上 `sunshine-spec-import`，update-spec skill 含 `sunshine spec index` 指令，新 spec 模板三份各含 1 处 `## 索引`。go build/vet/gofmt/test 全过（21 包）。附带修：spec.go 的 Long/Example 曾把 `\n` 写进反引号 raw string 导致帮助里出现字面 `\n`，已改真换行。

**发布 v0.5.11（2026-11-14，已发布，已验证）**：主提交一笔——规范索引层 + 自动维护与导入（本地 4ef595c → 远端 31e05d5，tree 7f86b5ed，父 b2e6d12，作者 linrong007，16 个文件，633 插入/10 删除）。tag v0.5.11 对象 77c0063 → 31e05d5。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.11，6 个资产非草稿，changelog 含主提交 31e05d5。发布 run 43608709168 success。验证：darwin arm64 checksum 对上（9b71cf69…）；Mach-O 64-bit arm64 确认（一键装的 linux/amd64 二进制因沙箱是 Linux，arm64 二进制报 Exec format error 属预期，checksum 与 file 已确认）；一键装装上 v0.5.11；已装二进制 version 0.5.11、`spec index` 命令在位、`upgrade --dry-run` 报已是最新。未验证：v0.5.10 二进制 upgrade --dry-run 未做（/tmp/sunshine-v0510 已被 tmpfs 清掉）。

## 四处工具发现（2026-11-14，已发布 v0.5.12）

背景：2026-11-13 s6-loop 真实运行（/tmp/scenes/s6-loop）发现的四处工具问题，用户 2026-11-14 点名修。另有两处 s6-loop 新发现（task start 无会话记录、init 不生成根 .gitignore）不在本次四处内，2026-11-16 用户说“动吧”后修完，见文末“两处 s6-loop 遗留”一节。

**① 归档/add-session 自动提交依赖 git 身份**：没设 git user.name/user.email 时 `git commit` 报 Author identity unknown，自动提交失败。两处：`internal/task/hooks.go` 的 autoCommitArchive（`git commit` 那一步；源任务已追踪时返回 false，归档操作按失败处理）；`internal/task/session.go:1099` 的 autoCommitWorkspace（`git commit` 那一步；失败返回 commitFailed，调用方 session.go:260 打印检查点并 `return 1`，add-session 退出 1——但会话记录已写盘）。修复：提交前查 git 身份（`git config user.name` / `git config user.email`，任一缺失即缺）；缺失时跳过自动提交并打印明确警告（风格同 printGitignoreWarning / printIndexLockWarning：说明缺了哪项、文件已写盘 git 未动、给修复方法——设身份，或在 `.sunshine/config.yaml` 设 session_auto_commit: false），不让整个操作失败：archive 返回 true，workspace 返回 commitBlocked（调用方 switch 里 commitBlocked 与 commitSkipped 同样落到 `return 0`）。新增 helper 放 hooks.go（与 runGit 同包，两处共用），签名用 io.Writer 以便两处直接调。

**② add-session 的 .gitattributes 未进提交**：EnsureGitattributes（hooks.go:325）会写/改项目根 .gitattributes（追加 journal merge=union 规则），由 session.go:258 的 add-session 流程调用（在 autoCommitWorkspace 之前）；但 safeTrellisPathsToAdd（hooks.go:144）列出的 stage 路径只有开发者 journal/index.md 与当前任务目录，不含 .gitattributes，导致它一直未提交。修复：safeTrellisPathsToAdd 末尾追加项目根 `.gitattributes`，条件是文件存在且含 journal merge 规则（复用 journalMergeUnionRe）；用户自己的 .gitattributes（无此规则）不动。调用方只有 autoCommitWorkspace（session.go:1114），改动范围就是 add-session 自动提交。archive 不调 EnsureGitattributes，不动 safeArchivePathsToAdd。

**③ task create 建出的 jsonl 是空文件、无 _example 行——核实结论：按设计，不改代码**。依据：Trellis 原版 task_store.py:578 注释原文“Create empty implement.jsonl / check.jsonl… They stay empty until the agent curates real entries during planning — a placeholder row would read as unresolved scaffolding to `task.py validate` and to PR preflight, so the curation instructions go to the console below instead of into the files”；task_context.py 亦说明旧版 _example 占位行会被 validate 当硬错误拒绝。Sunshine 的 internal/task/lifecycle.go:321 同样建空文件，与现版 Trellis 一致；internal/task/lifecycle.go:1240 与 internal/cli/task.go:132 确认 _example 是硬错误。把 _example 写回去会破坏 validate，不做。task create 的终端输出已说明 jsonl 格式（lifecycle.go:367 那段），不用再动。

**④ .sunshine/.developer 未被 gitignore、随提交进库**：模板 templates/sunshine/.gitignore 只有 `*.log` 与 `.runtime/` 两行；.developer 由 internal/scaffold/scaffold.go:165 写（开发者名），且 scaffold.go:64 的 hashExcluded 已把它排除在哈希清单外（说明它本就是本地文件，不应进库）。修复：模板加 `.developer` 一行。已有项目如何拿到：scaffold.go WalkDir 全量复制 templates/sunshine/（含 .gitignore）并哈希（.gitignore 不在 hashExcluded 里）；internal/update/hash.go:155 明言“.sunshine/.gitignore 不排除——scaffold 会哈希它，它是 init 模板的一部分，update 需要追踪它”——所以 `sunshine update` 会把新行同步进已有项目。注意：已追踪进库的 .developer 不会因进 .gitignore 自动脱离追踪，需手动 `git rm --cached .sunshine/.developer` 一次（写进汇报，不自动做）。

**验证（2026-11-14）**：go build/vet/gofmt/test 全量过（21 包）。临时目录实测（/tmp/fix1、/tmp/fix2，新二进制 /tmp/sunshine-fix1）：①无 git 身份时 add-session 打印“[警告] git 身份未设置（缺失：user.name、user.email），自动提交已跳过”并退出 0（会话已写盘）；archive 同样警告、退出 0、任务已移入 archive/2026-11/。②有身份时 add-session 的提交含 `.gitattributes`（基线无此文件，提交后 `git ls-files` 有它）。③task create 建出的 implement.jsonl/check.jsonl 各 0 字节；validate 报“0 条已整理条目”（2 个错误，退出 1），无 _example 相关错误——空文件是按设计的“未整理”状态。④新项目 init 后 `.sunshine/.gitignore` 含 `.developer`，`git check-ignore` 确认 `.sunshine/.developer` 被忽略；模拟旧项目（清单哈希改成旧内容）跑 `sunshine update --dry-run` 显示 `.sunshine/.gitignore` 待更新，实际 update 自动更新 1 个文件，.gitignore 补上 `.developer`。附带：实测中 /tmp（512M tmpfs）曾满导致测试链接失败，清掉旧二进制后恢复，与代码无关。遗留：已追踪进库的 .developer 需手动 `git rm --cached .sunshine/.developer` 一次（update 不同步已追踪文件）。

**发布 v0.5.12（2026-11-14，已发布，已验证）**：主提交一笔——四处工具发现修复（本地 ad9c649 → 远端 fb8edc6，tree 9aa2050，父 5140541，作者 linrong007，4 个文件，58 插入/0 删除）。tag v0.5.12 对象 223a438 → fb8edc6。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.12，6 个资产非草稿，changelog 含主提交 fb8edc6。发布 run 43612380096 success。验证：darwin arm64 checksum 对上；Mach-O 64-bit arm64 确认；一键装装上 v0.5.12；v0.5.11 二进制 upgrade --dry-run 查到 0.5.12；新版 spec index 命令在位。
**发布 v0.5.13（2026-11-16，已发布，已验证）**：主提交一笔——最少实现梯子（36 个 agent 定义文件）+ .sunshine/memory/ 长期记忆第一条（6 个文件：新模板 memory/index.md + 三份 workflow 模板 3.3 节 + update-spec skill 两份步骤 6），只改模板未动 Go 代码（本地 b135167 → 远端 b7429d7，tree 34a1d80，父 cb47675，作者 linrong007，43 个文件，355 插入/49 删除）。tag v0.5.13 对象 bd70685 → b7429d7。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.13，6 个资产非草稿，changelog 含主提交 b7429d7。发布 run 43787218976 success。验证：darwin arm64 checksum 23405a8b…e5d7b0 对上；Mach-O 64-bit arm64 确认；linux amd64 二进制报“Sunshine 版本 0.5.13”；一键装装上 v0.5.13；v0.5.12 二进制 upgrade --dry-run 查到 0.5.13。CI：一路 43787219007 success，另一路 43787194444 在“运行单元测试”步 failure（同提交另一路已过、本地 21 包全过，属已知抖动，非代码问题；token 无 Actions 重跑权限）。
**发布 v0.5.15（2026-11-16，已发布，已验证）**：主提交一笔——1.5 评审加两条同步核对（全量评审核对 prd.md 与需求列表.md 一致；只审变更时逐项核对父 prd [变更]、受影响子任务同步改、新文件进 jsonl、未受影响子任务未动），三份 workflow 模板 + PLAN.md 新一节（本地 eb264d8 → 远端 b6567f0，tree f324f07，父 ba769a8，作者 linrong007，4 个文件，23 插入/6 删除）。tag v0.5.15 对象 dc57ab7 → b6567f0。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.15，6 个资产非草稿，changelog 含主提交 b6567f0。发布 run 43799520053 success；持续集成 43799520025 success。验证：darwin arm64 checksum 88637403…52bc371 对上；Mach-O 64-bit arm64 确认；linux amd64 二进制报“Sunshine 版本 0.5.15”；一键装装上 v0.5.15；v0.5.14 二进制 upgrade --dry-run 查到 0.5.15。另：main 分支 CI run 43799407064 在“运行单元测试”步 failure（同提交的 v0.5.15 tag CI 已过、本地 21 包全过，属已知抖动；token 无 Actions 重跑权限）。
**发布 v0.5.14（2026-11-16，已发布，已验证）**：主提交一笔——两处 s6-loop 遗留（task start 降级补 `.runtime/` 目录一行、init 生成根 `.gitignore`）+ spec 索引自动同步（新 hook 事件 post-tool-use + SessionStart 兜底；索引是强制的，`.sunshine/spec/` 下全部文件都要带索引，无索引节也补一个）（本地 3133895 → 远端 41fe63c，tree 4f94eed，父 c46fe1d，作者 linrong007，8 个文件，251 插入/2 删除）。tag v0.5.14 对象 eece6a0 → 41fe63c。Release：https://github.com/linrong007/Sunshine/releases/tag/v0.5.14，6 个资产非草稿，changelog 含主提交 41fe63c。发布 run 43796272172 success；持续集成 43796272158 success。验证：darwin arm64 checksum 40cec20a…58042f6 对上；Mach-O 64-bit arm64 确认；linux amd64 二进制报“Sunshine 版本 0.5.14”；一键装装上 v0.5.14；v0.5.13 二进制 upgrade --dry-run 查到 0.5.14。

## 最少实现梯子（2026-11-16，已发布 v0.5.13）

背景：用户要 Sunshine 不过度编程（继 spec 索引层之后，Ponytail 一类 skill 的启发：省 token 靠少写代码）。已定方案：implement agent 加一段“最少实现”梯子，check agent 加一条超范围检查。只改 agent 定义模板，不改 Go 代码。用户 2026-11-16 点名“加上，就按这个做”。

**改了 36 个文件（290 插入/48 删除），不是任务初估的 38 个**：droid、dsh 两平台没有 agents 目录，实际是 channel 2 个 + 17 平台 × 2 个。implement 18 个（channel 1 + 15 个 .md/.agent.md + codex .toml + kiro .json），check 18 个（同构）。

**implement 梯子（原文，新增一节，放在“核心职责”之后、“禁止操作”之前）**：

> ## 最少实现
>
> 写任何代码之前，先顺着梯子往下走，能停就停：
>
> 1. PRD / spec 要求这个吗？——没有就跳过，不要写。
> 2. 代码库里已经有了吗？——复用，不要重写。
> 3. 标准库或已装依赖能做吗？——用现成的，不要自己造。
> 4. 能一行写完吗？——一行就够，不要展开。
> 5. 实在不行，才写最少能跑的代码；有逻辑就补一个小测试。
>
> 梯子不砍底线：信任边界的校验、数据丢失的处理、安全、无障碍，一律保留。

同时“实现功能”步骤加指针句：12 个标准 .md（含 kiro .json）把实现步骤里的“只做要求的事，不要过度设计”改成“先走最少实现梯子，能停就停；只做要求的事，不要过度设计”（只改第一处，代码标准节那句不动）；channel 模板把工作流第 3 步改成“先走最少实现梯子，能停就停；按规范与现有模式实现功能”；codex .toml 在规则列表加同款指针句、梯子以“最少实现：”小节放在规则之后；omp/pi/reasonix 三个无“只做要求的事”的文件，在工作规则末尾加“写代码前先走最少实现梯子，能停就停。”

**check 超范围检查（原文）**：核心职责在“对照规范检查”之后加一项“**查超范围代码**——diff 里有没有实现 PRD / spec 之外功能的代码？机械性的多余代码直接删；拿不准的记下来汇报，不要悄悄重写。”，后续项顺延编号；检查步骤清单加一项“是否有超出 PRD / spec 范围的代码”。变体：grok/kimi/zcode/opencode 是短版核心职责（对照规范检查在第 2 或第 3 项），按各自措辞（“不只汇报”/“typecheck 与 lint”）对齐；snow 的步骤 2 原无清单，在句后补一项；omp/pi/reasonix 无“对照规范检查”项，插在“对照任务 PRD/产物评审”之后，评审重点末尾加“超出 PRD / spec 范围的代码。”；codex .toml 在检查清单末尾加同款项；channel 模板插在“对照规范评审”之后，工作流第 3 步清单加“超范围代码——实现 PRD / spec 之外功能的代码：机械性的直接删，拿不准的记录并汇报”。

**验证（2026-11-16）**：改前脚本校验全部锚点命中数（--check 模式，先校验再写盘；中途发现 opencode check 是短版措辞，已纠正）。改后 grep 核对：36 个文件全部在位（implement 梯子节 16/16、指针句 16/16、check 查超范围 16/16、步骤清单 12 个平台文件 + omp/pi/reasonix 评审重点各 1、codex 2 处、kiro 2 处）；codex .toml 与 kiro .json 均能解析，kiro 改后仍是 indent=2、UTF-8 原样、末尾换行。go build/vet/gofmt/test 全量过（21 包）。新二进制 /tmp/sunshine-ladder 在 /tmp/ladder-test init 新项目，装上的 `.claude/agents/sunshine-implement.md` 含梯子与指针句（行 86）、`sunshine-check.md` 含查超范围项与清单项（行 66）。已随 v0.5.13 发布（主提交 b7429d7，发布记录见“四处工具发现”节末）。

## 长期记忆第一条：.sunshine/memory/ Markdown 文件（2026-11-16，已发布 v0.5.13）

背景：2026-11-16 调研 GitHub 长期记忆方案（报告见 ~/workspace/research_notes/long-term-memory-github-survey-20261116-0118/report.md，另有一页中文版）。结论是不嵌入任何系统，只借三件套；先做第一条 Markdown 记忆文件，第二条 SQLite + FTS5 索引等文件多了再说，第三条渐进式披露与双时间戳是第一条的做法、不用单独立项。

**与 spec 的边界（已定，不冲突）**：两者回答不同的问题。spec（`.sunshine/spec/`）是“这个项目现在应该怎么做”——规定性、当前有效、agent 必须遵守；memory（`.sunshine/memory/`）是“以前发生过什么、为什么这样定”——描述性、历史性、只作参考。教训的“规则部分”（以后怎么做）仍按老规矩在 3.3 写进对应 spec，不变；memory 记三样东西：①规则背后的故事（为什么定这条规则、当时踩了什么坑、考虑过哪些替代方案）；②不是规则的东西（用户偏好、跨项目教训、决策理由）；③已被取代的旧决策（标 superseded，不删除）。**不重复原则**：memory 不复述现行规则，提到规则时只写指向（spec 文件 + 节），不抄全文。**冲突时 spec 赢**：agent 读到 memory 与 spec 不一致，以 spec 为准；每条 memory 带来源与验证状态，标明是“过去上下文”。**写 memory 是可选的、挑着记**：只有“为什么”对将来决策有用时才写；拿不准就只写 spec，不写 memory。3.3 回写时：规则进 spec（老规矩不变），故事进 memory（新加，可选）。

**格式**：一条一文件，YAML 前置字段 + Markdown 正文，OKF 风格。目录：
```
.sunshine/memory/
  index.md            # 常读索引：标题 + 一行摘要 + 文件名；只有它常读
  2026-11-16-xxx.md   # 一条一文件，文件名用日期 + 短 slug
```
前置字段：`type`（decision | lesson | preference | fact）、`title`、`tags`、`created`、`valid_from`、`supersedes`（被取代条目的文件名，无则空）、`superseded_by`（取代它的文件名，无则空）、`source`（任务/会话 id）、`status`（draft | verified | superseded）、`spec`（关联的 spec 文件 + 节，如 `.sunshine/spec/backend.md#优惠券`，无则空）。正文写故事：发生了什么、为什么、考虑过什么、结论是什么。

**两条底线**：①记忆只是建议，带来源和验证状态；②一条记忆可以提议改 spec，但只有人或明确的工作流步骤能写 spec，记忆自己永远不写 spec。`mem` 命令后续可搜这些文件（现有的 `mem search` 已能搜 Markdown），索引（第二条）等文件多了再加。

**实施（2026-11-16）**：6 个文件。①新模板 `templates/sunshine/memory/index.md`（空索引：标题 + 一行摘要 + 文件名三列表头 + 边界说明一行）；scaffold 全目录 WalkDir 复制，新文件自动进 init 与哈希清单，无需另注册；普通 init 遇到已存在的 `.sunshine/` 会拒绝，不会覆盖。②三份 workflow 模板 3.3 节加 memory 行（native/tdd 同文、channel-driven 短版同义）。③update-spec skill 加“步骤 6：故事进 memory（可选）”（common skill 与 copilot prompt 两份同文，前置字段清单一致）。只改模板，不改 Go 代码。**注意**：`init -f` 会清掉整个 `.sunshine/`（含 tasks、spec、memory），这是既有行为（重新初始化），memory 与其他文件同等待遇；普通 init 与增量模式不碰已有文件。

**验证（2026-11-16）**：grep 核对 6 个文件全部在位（workflow 三份各 1、skill 两份步骤 6 各 1、index.md 模板在位）；go build/vet/gofmt/test 全量过（21 包）。新二进制 /tmp/sunshine-mem 在 /tmp/mem-test init 新项目：`.sunshine/memory/index.md` 在位、内容与模板一致；`.sunshine/workflow.md` 第 647 行有 memory 行；update-spec skill 第 151 行有步骤 6；梯子也在（Claude implement 含“最少实现”2 处、check 含“查超范围代码”1 处，合并验证通过）。不覆盖验证：填一条测试条目后普通 init 不动它（增量模式）；`init -f` 后 index.md 回到空模板、测试条目消失，tasks 同样被清——证实 -f 是既有全清行为。已随 v0.5.13 发布（主提交 b7429d7，发布记录见“四处工具发现”节末）。

## 两处 s6-loop 遗留（2026-11-16，已发布 v0.5.14）

2026-11-13 s6-loop 发现、2026-11-15 摆给用户三选一未答、2026-11-16 用户说“动吧”后实施（未再追问选项，按最稳妥的取舍做）。

**① task start 无会话身份时不写 session 文件、不建 .runtime/**：`internal/task/lifecycle.go` 的 Start 降级分支（无 SUNSHINE_CONTEXT_ID）：状态照翻、`.current-task` 照写、面包屑照打，但不写 session 文件、不调 AddSession，s6-loop 那轮 `.sunshine/.runtime/` 根本没建。修复取三选一里的 (b)：降级分支里只加一行 `_ = os.MkdirAll(.sunshine/.runtime, 0o755)`，保证目录存在（serve 与 hook 按该目录读活动流与会话票据）；不取 (a) 自动补 add-session 记录——那是补造不存在的会话记录；不取 (c) 保持原样——用户已说动手修。

**② init 不生成根 .gitignore**：只生成 `.sunshine/.gitignore`；奶茶那轮根 `.gitignore` 只有 `node_modules/` 一行、后端子任务后补，教程 §12.6 实录写明“否则 `node_modules/` 会进工作区”。修复：scaffold 加 `writeRootGitignore`（Go 常量，不进模板 WalkDir），init 末尾无条件调，只在文件不存在时写，已有内容绝不覆盖、`-f` 也不覆盖（与 writeDesignTemplates 同规矩）。内容取舍：Sunshine 自身生成的文件都在 `.sunshine/` 里（由 `.sunshine/.gitignore` 管）、根目录没有 Sunshine 自有项，所以“只写 Sunshine 自有项”会是空文件、修不了真实问题；按真实踩坑写三项通用项：`node_modules/`（奶茶那轮差点进库）、`.env`（防密钥进库）、`.DS_Store`（macOS 通用）。Trellis 只写 `.trellis/.gitignore`、不生成根 `.gitignore`，本次是超出 Trellis 的改进。

**验证（2026-11-16）**：go build/vet/gofmt/test 全量过（21 包）。新二进制 /tmp/sunshine-s6fix 在 /tmp/s6fix-test：init 后根 `.gitignore` 在位（三项 + 说明行）；追加自定义行后 `init -f`，自定义行与三项都在，未覆盖。降级 start 实测（`env -u SUNSHINE_CONTEXT_ID task start test-task --allow-empty-context`）：状态 planning → in_progress（降级），`.sunshine/.runtime/` 已建，`.runtime/sessions/` 不存在（未补造 session 文件），`.current-task` 已写。已随 v0.5.14 发布（主提交 41fe63c，发布记录见“四处工具发现”节末）。

## spec 索引自动同步（2026-11-16，已发布 v0.5.14）

**起因**：用户 2026-11-16 指出，设计阶段增补原型时 spec、prd、task 都会改；spec 改了之后索引要靠 agent 记得跑 `sunshine spec index`，忘了就不同步。用户定：做成自动的。

**现状核实**：`sunshine hook` 现有 6 个事件（session-start、workflow-state、subagent-context、subagent-stop、shell-session、statusline），没有 PostToolUse/Stop 事件；Claude settings.json 注册了 PreToolUse（Task/Agent → subagent-context）、SubagentStop、SessionStart、UserPromptSubmit（→ workflow-state），没有 PostToolUse。`sunshine spec index` 是确定性命令（按 `##` 标题重生成索引，只改索引节、正文不动、反复跑结果相同）；`specindex.SyncContent` 在文件有 `##` 节但无索引节时会**插入**一个索引节——手动命令与自动 hook 都这样（索引是强制的，见下）。

**方案**：两层，PostToolUse 即时同步为主，SessionStart 兜底为辅。
- 主：新 hook 事件 `post-tool-use`（`internal/hook/post_tool_use.go`，`RunPostToolUse`）。Claude settings.json 加 PostToolUse（matcher `Edit|Write|Bash`，timeout 10）。hook 解析 stdin JSON 里的 tool_name 与 tool_input：Edit/Write 取 file_path，只当文件在 `<root>/.sunshine/spec/` 下且以 .md 结尾时处理（单文件同步）；Bash 无法从命令串可靠判断改了哪个文件，改走整目录同步（`SyncDir`，只写真正不同步的文件）。索引是强制的（2026-11-16 用户定：为省 token，`.sunshine/spec/` 下全部文件都要带索引；先按“只同步已有索引”实施，实施后用户反问“强制不好吗”，即定改强制）：同步用完整 `SyncFile`/`SyncDir`，无索引节的文件也补一个，与手动命令行为一致；任何失败都静默返回 nil，绝不阻断 agent（hook 铁律）。同 turn 内改 spec 后立即派子代理也能读到新鲜索引（Stop hook 做不到这点，turn 没结束它不触发）。用户 2026-11-16 补一句“所有有关 spec 的修改，涉及到索引的，都要自动 spec index”——即触发面要全：Edit/Write 之外，Bash 改 spec（sed、python、heredoc）也要覆盖，故 matcher 加 Bash。
- 辅：`RunSessionStart` 里顺手跑一遍 `specindex.SyncDir`（完整同步，无索引节也补），覆盖平台无 PostToolUse、编辑发生在 agent 之外、hook 没触发的情况；保证注入的上下文读到的索引是新鲜的。
- 其他平台（Cursor/Codex 等）模板里没有 post-edit 事件，不加；靠 SessionStart 兜底 + 手动命令。workflow 3.3 与 update-spec skill 的手动步骤保留作兜底（命令是幂等的，已同步时只打印“无需改动”）。

**实施（2026-11-16）**：8 个文件。①新 hook 事件 `post-tool-use`（`internal/hook/post_tool_use.go` + 测试）：Edit/Write 按 file_path 单文件同步；Bash 走整目录同步（命令串判断不可靠）；非 Edit/Write/Bash、坏 JSON、缺 file_path 一律静默返回 nil。②`internal/cli/hook.go` 注册事件并更新帮助。③`RunSessionStart` 开头加 `SyncDir` 兜底。④Claude settings.json 加 PostToolUse（matcher `Edit|Write|Bash`，timeout 10）。只改 Go 代码与 Claude 模板，未动 workflow/skill 文本（手动步骤留作兜底）。实施后改强制（用户定，见方案）：hook 与兜底由 `SyncFileIfIndexed`/`SyncDirIfIndexed` 换成完整 `SyncFile`/`SyncDir`（无索引节也补一个），`HasIndex`/`SyncFileIfIndexed`/`SyncDirIfIndexed` 三个 helper 随之删除（只存在于未提交的工作树里，未进过任何版本）；hook 测试 `TestPostToolUse_EditSpecWithoutIndexUntouched` 改名 `TestPostToolUse_EditSpecWithoutIndexGetsIndex`，断言改为补索引。索引“可选”的说法只出现在刚写的两处注释里，已同步改掉；workflow/skill 文本里没有“索引可选”的说法，无需改。

**验证（2026-11-16）**：go build/vet/gofmt/test 全量过（21 包）；新测试 8 个全过（specindex 3 个 + hook 5 个）。新二进制 /tmp/sunshine-autoindex 在 /tmp/autoindex-test 端到端实测：Edit 改 backend.md 加一节后调 hook，索引表里出现新节；Bash 改 frontend.md 加一节后调 hook，索引同步；无索引节的 extra.md 加节后调 hook，原样不动；system.md（有索引）新节进索引；改非 spec 文件不触发；坏输入退出 0。SessionStart 兜底实测：把 backend 索引行删掉变旧，跑 session-start 后索引行回来。**改强制后重验**：go 全量 21 包全过；hook 测试 5 个全过（含改名的 GetsIndex；3 个 specindex 测试随 helper 删除）；新二进制 /tmp/sunshine-mandindex 在 /tmp/mandindex-test 实测：backend.md 删索引节后加“优惠券结算”一节、调 post-tool-use，索引被补上且含新节；frontend.md 删索引节后跑 session-start，索引回来；hook 退出 0。已随 v0.5.14 发布（主提交 41fe63c，发布记录见“四处工具发现”节末）。

## 1.5 评审加两条同步核对（2026-11-16，已发布 v0.5.15）

**起因**：用户拿一个全流程 demo（奶茶商城后台，sunshine-demo8）验证：设计做完、没详审没拆解，`.sunshine/tasks/` 里只有一个父任务 `11-16-milktea-admin`（planning，children 为空），没有拆解——符合预期。但顺手发现一处不一致：需求列表 R-004 是 7 列含成本价（变更记录写了“按追加需求增加成本价列，表格由 6 列扩为 7 列”），父任务 prd.md 的 R-004 验收标准还写着 6 列；原型表头已是 7 列含成本价；prd.md 全文 0 处“成本价”、0 处“变更”。即变更只进了需求列表与原型，没进父任务。

**根因**：prd.md 与需求列表.md 是两份手写文件，没有自动同步机制，全靠 agent 同一轮两边都改；追加类变更（先 6 列、后加成本价）最容易漏一边。流程文字上早有要求（循环里“答案立即记进 prd.md，该条写进需求列表”；变更流程“先在 prd.md 记一笔 [变更] 再改 doc/”），但没有一个关口去验。

**用户追问的场景**：设计人员反场（开发中途需求变更）第一件事改需求列表与原型；如果父任务与已拆出的子任务不做记录/变更，设计人员退场后开发人员没法做变更——要不要重走评审、重新拆解？答：不用。变更流程本来就是针对性回炉（只改变化的部分）+ 1.5 只审变更的部分 + 子任务原地改（planning 直接改、in_progress 暂停回炉不停状态、已归档不 reopen 而是新建子任务），新增需求走迷你一轮 1.1 → 1.2 → 1.4 → 1.5 挂新子任务；不重走全量评审、不重新拆解。但这套成立的前提是任务侧同步真的发生了——demo 证明它会被漏掉，所以要在 1.5 的两个关口上加可核对的清单，让关口去抓，而不是再写一遍“要同步”。

**方案**：只改三份 workflow 模板（native/tdd/channel-driven）的 1.5 节，两处（两句原文三份完全相同，脚本断言各命中 1 次后追加）：
- 全量评审（设计阶段）：`需求评审` 段“确认后才往下走。”后加——“呈报前先逐条核对：父任务 `prd.md` 与 `doc/design/需求列表.md` 一致（每条需求的描述、验收标准、对应原型页；需求列表的变更记录与 `prd.md` 里的 `[变更]` 对得上）。不一致先补齐，不带矛盾进评审。”
- 只审变更（设计人员反场）：`需求变更` 段“回到 1.5 只评审变更的部分，评审通过再动手。”后加——“只审变更时逐项核对：①父任务 `prd.md` 已记 `[变更 YYYY-MM-DD]`（改了什么、为什么、谁定的）；②受影响的子任务 `prd.md` / `design.md` / `implement.md` 已同步改，范围与清单已更新（`set-scope` / `add-context`）；③新文件已进 jsonl；④未受影响的子任务未动。设计人员反场先改 `doc/` 也可以，但退场前任务侧必须补齐——缺一项就不通过，开发不许动手。”
- 顺序说明：变更流程原有的“prd [变更] → doc/ → 子任务”仍是推荐顺序；新加的只是关口核对——反场先改 doc/ 不算违规，但退场前任务侧必须补齐，关口按清单验。

**实施（2026-11-16）**：3 个文件（只改 workflow 模板，不动 Go 代码、不动 skill、不动教程 §12.6——§12.6 正文是实录不可改写）。demo 本身用户说不用管，未动 demo 文件。

**验证（2026-11-16）**：grep 两句在三份模板各 1 处，A/B 两句三份 md5 一致；go build/vet/gofmt/test 全量过（21 包）。已随 v0.5.15 发布（主提交 b6567f0，发布记录见“四处工具发现”节末）。

### 设计先行单父任务（2026-11-16，已随 v0.5.16 发布）

**起因**：用户拿服装商城后台 demo9（`~/workspace/user/files/sunshine-demo9.zip`）验证发现：设计阶段 4 条需求（R-001 首页、R-002 商品列表、R-003 商品分类、R-004 库存管理），`.sunshine/tasks/` 里有 4 个文件夹（11-16-mall-admin-home/product-list/product-category/inventory），全部 `parent: None`、无父任务——agent 一条需求新建了一个任务，各走各的 1.1–1.5（商品列表那个停在它自己的 1.5 等 check 复核原型）。正确形状是设计阶段只有父任务 1 个，评审通过后 1.2 拆解才出现子任务。根因：工作流只写了“下一条”，没写“不要为每条需求新建任务、在同一个父任务里循环”；agent 默认习惯新活 = 新任务（尤其跨会话回来用户说“做下一个”时顺手 `task create`）；Sunshine 无硬门禁，`task create` 不检查已有设计先行父任务。demo9 本身的修正（建父任务、4 个任务挂子任务、走父任务 1.5 评审）已给用户，项目文件未动。

**实施（2026-11-16）**：两层，用户选了两层都做——
1. 三份 workflow 模板 1.1 各加两句（三份措辞与位置完全一致，零 Go 代码）：先定模式设计先行定义后加“设计先行只建一个父任务，循环在它里面跑。”；按需求循环段末尾加“不要为每条需求新建任务；新开会话回来继续做下一条需求时，先跑 `/sunshine:continue` 续接当前父任务，不要 `task create`。”
2. SessionStart hook 加一行提醒（Go 小改 + 测试）：当前任务 planning、children 为空、项目里有 `doc/design/需求列表.md` 时，`<current-state>` 多注一行“Design loop: 当前任务 planning、未拆解、已有 doc/design/需求列表.md——做下一条需求不要新建任务，先跑 `/sunshine:continue` 续接当前父任务。”。信号说明：planning + 未拆解 + 有需求列表即判定设计循环进行中；设计完成后已拆解的父任务（children 非空）不再提醒；项目里有需求列表但当前任务不是设计先行父任务时也会提醒一句，文案的建议（续接当前任务）仍成立，属可接受的误报面。

**验证（2026-11-16）**：grep 两句在三份模板各 1 处；go build/vet/gofmt/test 全量过（21 包）；hook 包新增 4 个测试全过（DesignLoopReminder / NoList / HasChildren / InProgress）。端到端实测：新二进制在 demo9 副本上跑 session-start，提醒行在位；无需求列表时提醒消失；新二进制 `init -y` 实测装上的 workflow.md 两句在位。实测教训：跑 session-start 端到端须设 `CLAUDE_PROJECT_DIR`，否则平台检测不到、当前任务解析为 none、提醒行不出现（第一次实测即因此误判，后设环境变量重验通过）。

**发布（2026-11-16）**：主提交本地 a9535d1 → 远端 **c69c60a**（tree `ec24822`、父 2657faa、作者 linrong007、6 个文件、75 插入/6 删除）。tag v0.5.16 对象 **9328376** → c69c60a。发布 run **43844544456** success；主推 CI 43844496049 success；tag 触发的另一路 CI 43844544497 在单元测试步 failure（同提交另一路已过、本地两次全过，属已知抖动，未重跑，token 无重跑权限）。Release https://github.com/linrong007/Sunshine/releases/tag/v0.5.16，6 资产非草稿，changelog 含 c69c60a。验证全过：darwin arm64 checksum `d6031032…64b72e`、Mach-O 64-bit arm64、linux 版报 0.5.16、一键装装上 0.5.16、v0.5.15 二进制 upgrade --dry-run 查到 0.5.16。

### spec-bootstrap 接手缺口：捕获表加三行 + frontend.md 加路由节（2026-11-16，实施中）

起因：用户问 spec-bootstrap 能否整出项目结构与 API 文档，又问接手存量项目的人最关心什么。核对结论：
- 接手人最关心：跑起来、改不炸（安全边界）、结构与数据流、约定与坑、验证与发布、出问题查哪找谁。
- spec-bootstrap 现有捕获表（repository-analysis.md“要捕获的内容”）覆盖包边界、运行时层、核心抽象（含路由）、数据流、错误处理、配置、测试；缺启动、部署、排查三行；frontend.md 模板有“技术栈”一节（占位待填实），但无路由节，路由表/守卫/懒加载无固定落点。
- API 文档不是 spec-bootstrap 的产物（skill 全文未提接口文档；spec-import 是反方向），想要接口文档须两步走：先 spec-bootstrap 落 spec，再单开任务按 spec 出 doc/ 下的接口文档（已补进教程 §12.2，未提交）。

实施（只改模板，不动 Go 代码）：
1. `templates/platforms/common/bundled-skills/sunshine-spec-bootstrap/references/repository-analysis.md`：捕获表加三行——启动（怎么跑起来：启动命令、依赖、环境、端口、数据库）、部署（怎么发布、回滚）、排查（日志、监控、出问题查哪）。
2. `templates/sunshine/spec/frontend.md`：加“路由”一节（路由表、守卫、懒加载）+ 索引表加一行，措辞与现有模板一致（占位式“按项目实际情况填写”）。

验证：新建演示存量项目（前后端 + 前端路由）实跑 spec-bootstrap 全流程——新二进制 init → 子代理按 skill 执行（GitNexus/ABCoder 无 MCP 时退回直接读源码）→ 独立核查产出的 spec 含结构/技术栈/路由/启动/部署/排查六项、每条有真实文件支撑、无占位符、index.md 与文件集合一致。

**验证结果（2026-11-16）**：演示项目 taskboard（Vue 3 + Express，`/tmp/specboot-verify/taskboard`，git 初始提交 be6d423），新二进制 init -y 装上新模板与新 skill（frontend.md 含路由节、skill 捕获表含新三行，grep 核实）。子代理按 skill 实跑：读 17 个产品文件，node --check 9 个 JS 文件全过，改写 system/frontend/backend.md、新建导航 index.md，未动产品代码，未提交。独立核查通过：占位 0 处；六项齐全——结构（workspaces + 前后端分层）、技术栈（前端 Vue 3.4.0 + vue-router 4.3.0 + Pinia 2.1.7 + Vite 5.2.0，后端 Express 4.19.0）、路由（5 条、4 条懒加载、beforeEach 守卫）、启动（5173/3000、环境变量）、部署（deploy.sh 四步 + 回滚）、排查（/api/health、console.error、已知坑 db.js 静默返回 []）；引用的完整文件路径全部存在。诚实记录的缺口：零测试文件、部署后前端 dist 由谁托管找不到答案（无 express.static、无 nginx 配置）、防抖缺失。另发现 skill 与模板不一致一处：skill 要求保留 index.md 导航文件，但 init 模板里没有它（只有各文件头上的 `## 索引` 节），子代理新建了一个导航版 index.md 顶上。

**修（2026-11-16）**：核对后定为改 skill 口径，不动模板。根因不是 init 漏了文件：Sunshine 的 index.md 是按包/层拆分的子目录索引（workflow 与 research agent 均指 `.sunshine/spec/<包名>/<分层>/index.md`），扁平根目录（system/frontend/backend.md）本就用各文件头上的 `## 索引` 节导航；skill 的“保留 index.md”一句没写清适用范围，子代理才在根目录新建了一个。改两处 skill 措辞（SKILL.md 完成标准、spec-writing.md 文件结构）：子目录各配 index.md、根目录不另设。另核：spec-index 机制对无 `##` 节的文件原样不动，CheckDir 也不报，旧版子代理建的根 index.md 即便留下也不会被同步改坏。验证：旧措辞 grep 0 处、新措辞两处在位；go build/vet/gofmt、全量 21 包测试全过；新二进制 init 实测 spec/ 仍只有三份文件、装上的 skill 是新措辞。

**发布（2026-11-16）**：主提交本地 4d699d4 → 远端 **1c3fa43**（tree `cbdbf74`、父 2be4ad8、作者 linrong007、7 个文件、73 插入/2 删除）。tag v0.5.17 对象 **ac34eac** → 1c3fa43。发布 run **43855462483** success；两路 CI 43855411927、43855462393 均 success（本次无抖动）。Release https://github.com/linrong007/Sunshine/releases/tag/v0.5.17，6 资产非草稿，changelog 含 1c3fa43。验证全过：darwin arm64 checksum `891a3d1a…4310e`、Mach-O 64-bit arm64、linux 版报 0.5.17、一键装装上 0.5.17、v0.5.16 二进制 upgrade --dry-run 查到 0.5.17。验证插曲：/tmp 曾满到 100%，首轮解包截断，清理旧验证目录后重验通过。

## DSH 功能补齐（2026-11-17，三档，用户已批准全排上）

背景：2026-11-17 对标 Claude 核查，DSH 核心闭环已 headless 实跑通过（dsh 0.2.2-rc.1，deepseek-flash，`~/dsh-e2e`），但 5 处缺失 + 模板口径问题。用户定全排上、按档做。

核查结论（已核实）：持平——任务全闭环、CLI、serve（DSH 项目烟雾测试通过）。缺失——hook 0/5 事件；子代理 0 定义；无斜杠面板；spec 索引手动；mem 无 DSH 适配器；Channel 只认 claude/codex（ValidProvider，runtime.go:74），channel-driven 模板 0 次提 DSH；原型强制 Task 派发但 DSH 无定义；tdd 模板 DSH 出现 0 次。可解性已核实：DSH 有 subagent/subagent_fork 工具（可传 prompt 派发两种子代理：继承对话/完全独立）；dsh-commands 斜杠注册表只在交互 UI 有、headless 没有；会话存 `~/.dsh/sessions/<项目>/session-<uuid>/session.v4.jsonl.zstd`（zstd 压 JSONL v4）；Go 尚无 zstd 直接依赖（仅 klauspost/cpuid 间接）。

第一档（只改模板与 skill 文字，不改 Go 代码）：
1. 派发规则授权 DSH：native/tdd workflow 的“inline 分支只属于 Codex”改为 DSH 也可走 subagent 工具派发（自包含 prompt，角色步骤写进 prompt；check 派发时附 skill 文件。仿 Kimi Code 借内置 agent 的写法）；planning-inline/in_progress-inline 分支加 DSH。
2. tdd 模板 inline 分组加 DSH（与 native 一致：`[codex-inline, Kilo, Antigravity, Devin, DeepSeek Harness]`）。
3. 原型走 subagent 工具派发（workflow 约 368 行）：DSH 上用 subagent 工具派 sunshine-prototype，附 skill 文件与需求列表路径；主会话仍不直接写原型。
4. spec 索引提醒：workflow 与 update-spec skill 加一句——DSH 上改完 spec 就跑 `sunshine spec index`（无 PostToolUse 兜底）。
5. DSH.md 同步更新（新授权的路 + 仍缺失的项）。
验收：go build/vet/gofmt、全量测试全过；模板措辞 grep 核对；DSH headless 实跑一次子代理派发（implement 或 prototype 二选一），产物落盘为准。

第二档（Go 代码）：
6. mem 适配器：新增 DSH 适配器（读 session.v4.jsonl.zstd，zstd 解压 + v4 JSONL 解析），注册进 registry，测试钉住；新增 zstd 解压依赖（如无更轻的办法）。
7. Channel DSH provider：ValidProvider 加 dsh；worker 用 `dsh --profile headless` 一次一任务 + `--session-id` 复用会话模拟常驻；channel-driven 模板补 DSH 路。
验收：新测试全过、全量测试全过；mem 在 `~/dsh-e2e` 的真实会话文件上实跑出结果；channel DSH worker 实跑一次 send/recv。

第三档（DSH 插件，TypeScript/Cordis）：
8. 调研先行：DSH 插件 API（事件面：会话开始/工具调用/消息变换；dsh-commands 注册；各 profile 的插件加载方式，headless 是否加载同款插件；安装方式 `dsh plugin add`），输出调研结论再定实现范围。
9. 实现：hook 等价物插件（session-start 注入、post-tool-use spec 索引同步）+ 斜杠命令注册（交互 UI）。
验收：插件在 web 与 headless 下的加载情况如实记录；实跑验证。风险：DSH 0.2.x RC，官方明示会有不兼容改动，插件需跟随维护。

不做：headless 下的斜杠命令面板（DSH 自己不提供）。

发版：本批不发版，等用户点选。

## Codex 对标 Claude Code（2026-11-17，用户定“对标 Claude Code”）

目标：Codex 与 Claude Code 持平。核查结论（已核实）：Claude 5 个 hook 事件（SessionStart、PreToolUse、SubagentStop、PostToolUse、UserPromptSubmit）；Codex 已有 4 个（SessionStart、SubagentStart、SubagentStop、UserPromptSubmit），缺 PostToolUse。子代理 4 个、命令 3 条、skill 10 个、mem 适配器、Channel worker，两边都有。config.toml（max_depth=1、不写 multi_agent_v2）与 model 键保留是逐字移植。

第一档（PostToolUse，Go + 模板）：
1. `internal/hook/post_tool_use.go`：加 `apply_patch` 分支——Codex 的文件编辑走 apply_patch，不走 Edit/Write；分支内走整目录同步（与 Bash 同理；SyncDir 只写不同步的文件）。
2. `templates/platforms/codex/hooks.json`：加 PostToolUse 条目，matcher `^(?:apply_patch|Bash)$`，调 `SUNSHINE_HOOK_PLATFORM=codex sunshine hook post-tool-use`，timeout 15。
3. 测试：apply_patch 改 spec 后索引同步；非 spec 的 apply_patch 不动 spec；Bash 行为不变。
依据：Codex 有 PostToolUse 事件（OpenAI hooks 文档，2026-11 核实；apply_patch 触发是 PR #18391 修的）。版本底线待 Mac 实测确认（桌面版内捆 CLI 版本未知）。
验收：go build/vet/gofmt、全量测试全过；用 Codex 格式的 PostToolUse stdin（tool_name=apply_patch）模拟跑 hook，索引同步为准；沙箱 init --codex 装出文件裸 `{{` 0 处。

第二档（init 检测用户级开关）：
4. `platforms.CodexHooksEnabled()`：读 `~/.codex/config.toml`，检 `[features]` 下 `hooks = true`（兼容旧名 `codex_hooks = true`）；文件不存在或未开启都算未开启。
5. `printPlatformSummary`：装了 codex 时，未开启打印现有 CodexHooksHint，已开启打印一行确认、不再重复提示。
不做：自动改写用户级配置（只提示不代写）；自动过 /hooks 评审（Codex 自己的审批界面，一次性，须手动）。
验收：有开关/无开关/无文件三种情形的单测；沙箱 init --codex 输出核对。

不做：桌面版 hook 触发实测（须用户 Mac）。
发版：本批不发版，等用户点选。（注：树上还有 DSH 一批未提交，两批同树，用户点选时一起定。）

## DSH 网页版 Harness skill 镜像（2026-11-17，用户已批准）

起因（demo11 实测，DeepSeek Harness 网页版，卖电脑商城后台）：会话报 skill "sunshine-brainstorm" unknown，文件明明在 `.agents/skills/` 里。已排除两个旧猜测：①会话启动时机——新开会话仍报 unknown；②缺 frontmatter——安装时 wrapSkillFrontmatter 已自动给 6 个单文件 skill 包上 name+description。最后一次验证：`sunshine-start`（在 `.dsh/skills/` 里）一次叫上、无报错。结论：**网页版 Harness 只发现 `.dsh/skills/`，不扫共享 `.agents/skills/`**；CLI dsh 两边都发现。Claude 那次同名报错是另一回事（同会话后装 skill、会话注册表是旧的），不用修。

修法（镜像，不搬家）：`init --dsh` 保持 `.agents/skills/` 原样（CLI 用），把 10 个工作流 skill（6 个单文件 + 4 个 bundled，含 references/ 子文件，共 44 个文件）原样再写一份到 `.dsh/skills/`，两处逐字节一致。
1. `internal/platforms/platforms.go`：Platform 加 `MirrorSkillsToCmdDir bool` 字段（只 dsh 置真）；buildFiles 在 skill 块（auto + bundled）之后、命令块之前，把 `files` 里 `SkillsDir+"/"` 前缀的条目按相对路径复制到 `CmdDir` 下。放命令块之前是关键：此时 `files` 里只有 skill 文件，镜像不会误抄命令；镜像键用 `CmdDir` 前缀，不会再命中 `SkillsDir` 前缀。
2. 注册表 dsh 条目加 `MirrorSkillsToCmdDir: true`。
3. `templates/platforms/dsh/DSH.md`：能力表与文件对照同步——网页版只认 `.dsh/skills/`，10 个工作流 skill 镜像在 `.dsh/skills/`，与共享根目录逐字节一致。
4. 测试：`platforms_new_test.go` 的 dsh 文件数 48 → 92（44 镜像 + 原 48）；dsh 子测试加断言——`.dsh/skills/sunshine-brainstorm/SKILL.md` 等 10 个 skill 存在且与 `.agents/skills/` 对应文件逐字节一致；其他平台文件数不变。
5. update/prune 走 Install/RenderTemplates，镜像自动进写出集，不会被当陈旧文件清掉；`writeTemplateFile` 内容相同跳过，重装不重复写。
不做：动 `.agents/skills/` 的位置（CLI 依赖它）；给 Claude 加提示（Claude 那次不用修）。
验收：go build/vet/gofmt、全量测试全过；沙箱 `init --dsh` 装出 92 个文件，`.dsh/skills/` 下 10 个 skill 全在且与 `.agents/skills/` 逐字节一致，裸 `{{` 0 处。
发版：本批不发版，等用户点选。

## v0.5.19 发布记录（2026-11-17，已发布）

内容：DSH 网页版 Harness skill 镜像（上一节）。主提交本地 `856d6fb` → 远端 `dbd8661e67f0451d0e95843fb5cc69b073aa9a62`，tree `927cbfdb8a9038d156e52bfc00a01aa7b69c03ac`，父提交 `733c1a3f7598dae8d45fb6298ac6a8ea5eaab948`（教程 §12.2 提交）。Tag v0.5.19，tag 对象 `904866e8190c77ad5cf3076dd9540bc6ad4066ea` → dbd8661（GitHub API 建）。发布 workflow（run 44016501658）与 CI 两路（44016501632、44016475659，同提交 push 与 tag 各一路）均 success。Release 非草稿，6 个资产（checksums.txt + darwin amd64/arm64、linux amd64/arm64、windows amd64）；changelog 含主提交 dbd8661 与 733c1a3。一键装脚本：`curl -fsSL https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh | sh`。

## Windows 一键安装 install.ps1（2026-11-17，用户定补）

起因：install.sh 只支持 macOS/Linux，Windows 只能手动下载 zip；用户问有没有类似的，定补一个 install.ps1。
1. 新文件 install.ps1（PowerShell 5.1+ 兼容）：用法 `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`；指定版本走 `$env:SUNSHINE_VERSION` 或 `-Version` 参数；装到指定目录走 `$env:SUNSHINE_INSTALL_DIR` 或 `-InstallDir` 参数；默认装到 `$env:LOCALAPPDATA\Sunshine\bin`，并把该目录追加进用户级 PATH（同时更新当前会话 `$env:Path`，不写系统 PATH、不用管理员）。
2. 流程与 install.sh 对齐：查最新版（先走 GitHub API releases/latest，失败再跟 releases/latest 跳转取 tag）→ 架构只认 AMD64（Release 只有 windows amd64 包，ARM64 报错明说）→ 下载 `sunshine_<ver>_windows_amd64.zip` + checksums.txt → SHA256 校验（Get-FileHash）→ Expand-Archive 解压 → 拷 sunshine.exe → 跑 `--version` 验证；TLS 1.2 先加上（老版 PS 5.1 默认可能更低）。
3. README.md 与 README.cn.md：前置条件里的 Windows 手动步骤改成一键命令，快速上手补 Windows 一行。
不做：ARM64 Windows 包（goreleaser 只出 amd64）；改 install.sh。
验收：本机无 PowerShell，Windows 真机实测待用户；curl 核对 v0.5.19 zip 与 checksums.txt 可达且校验和在位；脚本逐行核对（TLS 1.2、重定向、PATH 追加去重、临时目录清理）。
发版：本批不发版，等用户点选。

## v0.5.20 发布记录（2026-11-17，已发布）

内容：Windows 一键安装 install.ps1（上一节）。主提交本地 `8b1619a` → 远端 `6316f872ef37339a43a5b99f2347207184f6c0ec`，tree `5736d37ee828ae68a267fe7fa3cfd84f3caa1536`，父提交 `da2b8f9a68efa798dd7c9a90a68d18c4ccf7b82a`（PLAN.md 补 v0.5.19 发布记录）。Tag v0.5.20，tag 对象 `9f3bc098e70788b2b50cd85da171be02e3c9e662` → 6316f87（GitHub API 建）。发布 workflow（run 44019510738）与 CI 两路（44019510728、44019489057，同提交 push 与 tag 各一路）均 success。Release 非草稿，6 个资产（checksums.txt + darwin amd64/arm64、linux amd64/arm64、windows amd64）；changelog 含主提交 6316f87 与 da2b8f9。一键装：macOS/Linux 用 install.sh，Windows（PowerShell）用 `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`。

## 提问格式统一为选项式（2026-11-17，用户批准，未发版）

起因：demo12（卖书商城后台，DSH 网页版）里 R-001 提问只有一个自由输入框，用户不知道怎么答；用户定：以后每个问题都给选项，问询要全面、把所有边界都对齐。
1. 格式：每次只问一个问题，给出 2–4 个选项，其中一个标明是推荐项并写清取舍，外加一个“自己填写”的自由输入项；推荐项只是建议，不是默认选项，不替用户选。
2. 全面：每个需求和任务的问询要全面，把范围、边界、约束、验收标准都问清对齐了再往下走；全面不是一次全问完，还是一次一个问题，对照需求维度清单一轮一轮问到全部对齐为止，不要留含糊。skill 里已有的需求维度清单八个维度与“其余的进决策清单，下一轮接着问”机制不动，只把“问到全部对齐”写明确。
3. 改动：brainstorm.md 提问规则（步骤 4、“每个问题必须包含”列表、需求维度清单引言）3 处；三份 workflow 模板（native/tdd 各 3 处、channel-driven 2 处：引导列表/按需求循环/前后端基础 1.1）8 处；docs/tutorial.md §12.6 开场提示词 1 处；docs/tutorial.en.md 对应英文句 1 处。旧措辞“附上你的建议与取舍”在 templates/ 与 docs/ 规范正文里 0 残留（PLAN.md 历史记录里的 3 处不动）。
4. 顺带修正：DSH.md 里“网页版 Harness 只发现 `.dsh/skills/`”一句是错的（2026-11-17 调查：网页版设计上支持项目 skill，文件与 provider 已排除，疑似会话 cwd/托管端问题，待定位），能力表与文件对照共 3 处改成“网页版会话里 skill 工具报 unknown，文件与 provider 已排除，疑似会话 cwd/托管端问题，待定位；agent 直接读文件可绕过”。
验收：go build/vet/gofmt、全量测试 21 包全过；沙箱 `init --dsh` 与 `init`（默认）装出的 brainstorm skill 与 workflow.md 均含新措辞，旧措辞 0 处。
发版：本批不发版，跟下版走，等用户点选。

## init --dsh 检测 git 根劫持（2026-11-17，用户批准：只检测提醒，不自动建仓库）

起因：demo12（卖书商城后台，DSH 网页版）里 skill 工具报 unknown，文件、provider、cwd、版本、重启全排除后破案：demo12 里 `git rev-parse --show-toplevel` = `/Users/ethan`——家目录本身是个 git 仓库；DSH 按“最近一个含 .git 的祖先”定项目根，根被劫持到 ~，provider 扫的是 `~/.dsh/skills`，当然找不到。修复是在 demo12 里 `git init`，已确认 skill 恢复。
1. 新文件 internal/platforms/gitroot.go：GitRootHijack(dir) 往上找最近一个含 .git 的祖先（目录或 worktree 的 gitdir 文件都算），找到的根不是项目目录本身时返回 (根, true)；项目自己有 .git 或往上无 .git 时返回 ("", false)。DshGitRootHint(ancestor) 返回中文警告：点明劫持根、skill 报 unknown、修复命令（项目目录跑 git init，只新建空仓库不动上层仓库，重启 dsh web 并开新会话）。
2. internal/cli/init.go：printPlatformSummary 加 cwd 参数（3 处调用点同步改），新装 dsh 时调 GitRootHijack，劫持才打印警告；三种 init 模式（全新/-f/增量）共用该函数，全覆盖。只检测提醒，不自动建仓库：项目可能是有意放在父仓库里的，自动 git init 会打扰这种安排。
3. DSH.md 同步修正：此前未发版批次里写的“疑似会话 cwd/托管端问题，待定位”已证伪，3 处（能力表 2 行 + 正文 1 处）改成根因口径——项目根被上层目录的 .git 劫持，项目里跑 git init 即可。
4. 实施中抓到的 bug：第一版循环条件写成 parent != abs，到文件系统根时 Dir("/") 恒为 "/" 死循环——只在往上无 .git 时触发（单测 TestGitRootHijackNoGitAnywhere 跑了约 4 分钟无输出才发现），已改成“Dir(根)==根即停”，注释里写明禁区。附带：/tmp 曾被僵死的 go test 残留（459M go-build + 49M test-bin）占满 100%，构建报 no space，已清理。
验收：新单测 3 个全过（劫持/无 .git/提示文案）；go build/vet/gofmt 全过；全量测试 21 包全过；沙箱三场景：祖先有 .git 打印警告且根路径正确、无 .git 与项目自有 .git 均 0 警告。
发版：本批不发版，等用户点选（用户 14:42 问“需求发版吗”，答复：做完验证后可跟提问格式那批合在一起发一个版）。

## init 默认不勾选任何平台（2026-11-17，用户定）

起因：init 默认勾选 Claude Code 与 Cursor，用户定改成一个都不默认选，只有用户选择至少一项才继续。
1. Go 代码：internal/platforms/platforms.go 删掉 DefaultChecked 字段（22 处初始值一并删）与 DefaultSelected()（原来返回 [claude-code cursor]）；internal/cli/init.go 的 resolveSelection 改三路——显式 flag 照旧；-y 或非交互终端且无 flag 时报错（“请至少选择一个平台：例如 sunshine init --claude，或去掉 -y 后交互选择”/“未检测到交互终端，请用平台 flag 至少选择一个平台，例如 sunshine init --claude”，退出码 1、不生成 .sunshine）；交互多选传 nil 默认值（一个都不勾选），不选则打印“请至少选择一个平台（按 Ctrl+C 取消）”重选，直到选了至少一项。handler 里“未选择任何平台，初始化已取消”一行留作兜底（新流程下已到不了）。
2. 测试：platforms_test.go 的 core 结构与 22 条 want 映射去掉 DefaultChecked，删 TestDefaultSelected；init_test.go 里 -y 与非 TTY 两案改成断言报错含“至少选择一个平台”，交互两案断言多选收到的默认值为空，新增空选重选案（第一次空、第二次选 codex，确认问了两次）；init_projecttype_test.go 端到端 6 处调用各补 --claude（原来 -y 无 flag 全挂）。
3. 文档：教程中英文 §1 演示命令补 --claude --cursor（输出本身不变，沙箱重跑核对过逐行一致）；§1 说明与附录“选平台”“-y”行同步改成“默认一个都不勾选、不选重选、-y 须用 flag 至少选一个”；README 中英文命令表里的 `sunshine init -u YourName -y` 示例补 --claude --cursor。
验收：gofmt/build/vet 全过；全量测试 21 包全过；沙箱四场景：-y 无 flag 报错退出 1 且无 .sunshine、教程 §1 命令输出与教程记录逐行一致、--dsh -y 正常、非交互无 flag 报错退出 1。
发版：本批不发版，等用户点选（与提问格式、git 根检测两批同树未提交）。

## v0.5.21 发布记录（2026-11-17，已发布）

内容：三批合发——①init 平台多选默认一个都不勾选，只有用户选择至少一项才继续（-y/非交互无 flag 报错，交互不选重选；删 DefaultChecked 与 DefaultSelected）；②init --dsh 检测 git 根劫持（上层目录 .git 劫持项目根时打印警告与修复命令，只提醒不自动 git init）；③提问格式统一为选项式（每次只问一个问题，给 2–4 个选项，其中一个标明推荐项并写清取舍，外加“自己填写”，问到范围、边界、约束、验收标准全部对齐）。主提交本地 `845a43241310ea7e01170aeb3fe60a462e15ba33` → 远端 `fe592bc48179c3a264f2a888f637e2d738937f13`，tree `2beeee75678f924cdfeacabef864f6027b9604f9`，父提交 `aea6a25a5855f06aabc3f6a265cfb279445dfcfe`。Tag v0.5.21，tag 对象 `76fe8f50cd209eb8d039def8e65cd4a0f555c41b` → fe592bc（GitHub API 建，annotated tag）。发布 workflow（run 44035686689）与 CI 两路（44035627319 main、44035686508 v0.5.21，同提交 push 与 tag 各一路）均 success。Release（id 448270407）非草稿、非预发布，6 个资产（checksums.txt + darwin amd64/arm64、linux amd64/arm64、windows amd64）。一键装：macOS/Linux 用 `curl -fsSL https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh | sh`，Windows（PowerShell）用 `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`。

## init 警告改按项目状态查（2026-11-17，用户实测报障，未发版）

起因：v0.5.21 发版后用户在 Mac 上试，`sunshine init -u linrong` 没提示需要 git init，skill 还是得手动 git init 才能调用。沙箱复现确认：警告只接在 printPlatformSummary 里、只看本次安装列表——全新安装时警告正常，但增量重跑（DSH 已装、“没有需要安装的新平台” early return）与增量装别的平台（DSH 不在本次安装列表里）两种情形一次警告都不出。
1. internal/cli/init.go：新增 dshGitRootWarning(cwd)（项目里装了 DSH 才查 GitRootHijack，劫持返回 DshGitRootHint，否则 "")与 warnDshGitRootHijack(cwd)（有警告才打印）；printPlatformSummary 里的 dsh 分支删掉、签名改回不带 cwd；四处调用——全新（小结后、下一步前）、-f（小结后）、增量无新平台（early return 前）、增量装了新平台（小结后），单点按项目状态查，无重复打印。
2. 订正：上一批 PLAN 里“三种 init 模式全覆盖”不成立——增量两路当时根本没走到警告，见本节起因。
3. 测试：新增 internal/cli/init_gitroot_test.go——单测 dshGitRootWarning 四情形（装+劫持应警告、装+自有 .git 不警告、没装不警告、装+无 .git 不警告）；端到端 TestInitRerunWarnsDshGitRootHijack（三连跑：全新、增量重跑、增量装 codex，每次都应警告且含劫持根）。
验收：gofmt/build/vet 全过；全量测试 21 包全过；沙箱三场景：全新、增量重跑、增量装 codex 各 1 次警告，文案与劫持根路径正确。
发版：本批不发版，等用户点选。

## v0.5.22 发布记录（2026-11-17，已发布）

内容：修 init 警告只看本次安装列表的漏——改按项目状态查，项目里装了 DSH 就查 git 根劫持，增量重跑也提示。主提交本地 `1fe3f060d7ceb1678b229d0b1a378c5b016c812b` → 远端 `32555aef48b2e8659f8fe204168b1fadd6482f91`，tree `fc4a91d42c41d6959670bbbf92b7786bcfddebf8`，父提交 `280abc746c51da66f487647d1d2edf31bcfeb040`。Tag v0.5.22，tag 对象 `01767f8f0ba11ef7c8b42de3ceb08710b80095b4` → 32555ae（GitHub API 建，annotated tag）。发布 workflow（run 44038685723）与 CI 两路（44038642310 main、44038685746 v0.5.22，同提交 push 与 tag 各一路）均 success。Release（id 448288361）非草稿、非预发布，6 个资产（checksums.txt + darwin amd64/arm64、linux amd64/arm64、windows amd64）。一键装：macOS/Linux 用 `curl -fsSL https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh | sh`，Windows（PowerShell）用 `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`。

## 提问格式：开放式选择，不限单选多选（2026-11-17，用户定，未发版）

起因：demo11 提问框“导航模块范围”一题，推荐项与最小骨架同时被勾上、互相冲突，用户先定单选；第一版纯单选后用户指出选项 B 是在选项 A 基础上增加的（叠加关系）、有时候还得是多选，改成默认单选、叠加时多选；随后用户定稿：开放性的，不限制单选、多选，但一定是选择（推荐+其它+手动填充）。
1. 口径（定稿）：回到 v0.5.21 的开放式写法——每次只问一个问题，给 2–4 个选项（其中一个标明是推荐项并写清取舍），外加一个“自己填写”的自由输入项；不限制单选还是多选。单选、默认单选两批的措辞已全部撤回：templates/ 与 docs/ 与 v0.5.21（fe592bc）逐字节一致，diff 0 行。
2. 边界：DSH 提问框的勾选框是 DSH 自己的界面渲染，改不了；改的是 agent 在问题正文里列选项。DSH.md 里没有选项条数措辞，无需改。
验收：gofmt/build/vet 全过；全量测试 21 包全过。
发版：本批不发版，等用户点选。

## DSH 进子代理平台列表：task create 建 jsonl（2026-11-17，用户实测纠正，未发版）

起因：用户问 demo13（DSH）任务文件夹为什么没有 implement.jsonl/check.jsonl；我先答“DSH 没有子代理”是错的——用户随即贴图证明：DSH 主会话正在创建子智能体做原型（“创建子智能体 · Build product list prototype”）。核实：DSH.md 明确写 implement/check/research/prototype 默认走 subagent 工具派发；2026-11-17 上午沙箱第一档实测也证实 agent 真调 subagent 工具。旧说法源自 v0.5.17 时期的降级评估，早已过时。
1. 根因：internal/task/store.go 的 subagentConfigDirs 里一直漏了 .dsh——该列表从 Python task_store 抄来，DSH 接入时没同步。task create 只在 HasSubagentPlatform 为真时建两个空 jsonl，于是 DSH 项目的任务从没有这两个文件；派发 implement/check 子代理时 workflow 要求读 implement.jsonl 注入上下文，文件缺失只能走兜底（读 prd/spec、task context 列规范自挑），上下文不如已整理清单确定。
2. 修复：subagentConfigDirs 加 ".dsh"（一行，附注释）。影响面只在 task create（HasSubagentPlatform 仅此一处调用；task start 门禁与 validate 读不到文件时本就不设防/跳过）。已建的 DSH 任务不受影响（文件不存在仍不设防）；新建任务建出空 jsonl 后，1.4 须按 workflow 整理条目，否则 start 门禁拒绝（与 Claude 等平台一致）。
3. 测试：新增 internal/task/subagent_platform_test.go——HasSubagentPlatform 在 .dsh 下为真、无平台目录时为假；DSH 项目 task create 建出两个 jsonl；无子代理平台的项目不建。
验收：gofmt/build/vet 全过；新单测 3 个全过、全量测试 21 包全过；沙箱端到端：init --dsh 后 task create 建出两个 jsonl，对照 init --kilo（非子代理平台）建出 0 个。
发版：本批不发版，等用户点选（与开放式提问格式那批同树未提交）。

## v0.5.23 发布记录（2026-11-17，已发布）

内容：两批合发——①DSH 进子代理平台列表（subagentConfigDirs 加 .dsh，task create 建 implement/check.jsonl）；②提问格式定稿开放式（不限制单选、多选，但一定是选择：推荐+其它+手动填充；单选两批措辞已撤回，templates/ 与 docs/ 与 v0.5.21 一致）。主提交本地 `3f31fb9ba80a5db3fccb2f3a4a0f712e9df031f7` → 远端 `9baba91e01997d3c730467c922d74f94d69c44e5`，tree `35cc660834cfa3afd26835e2de3ba0a395bfd6dd`，父提交 `9415f3b8292bc6b662876425fa65f1619bf2a389`。Tag v0.5.23，tag 对象 `3fb27cc2e602eaffcc84d481b952cf10b6c2e840` → 9baba91（GitHub API 建，annotated tag）。发布 workflow（run 44042768373）与 CI 两路（44042768352 main push、44042705064 tag push，同提交 push 与 tag 各一路）均 success。Release（id 448316003）非草稿、非预发布，6 个资产（checksums.txt + darwin amd64/arm64、linux amd64/arm64、windows amd64）。一键装：macOS/Linux 用 `curl -fsSL https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh | sh`，Windows（PowerShell）用 `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`。

## 接入 WorkBuddy：复用 --codebuddy，不新增平台（2026-11-17，未发版）

结论：不新增平台条目。WorkBuddy 应用实际跑的是内置的 CodeBuddy Code CLI，项目级配置读 `.codebuddy/`，与独立 CLI 完全一致；Sunshine 现有的 `init --codebuddy` 安装形态可直接被 WorkBuddy 应用内 CLI 加载。WorkBuddy 用户用 `--codebuddy`。
1. 核查证据（决定性问题 a：项目级配置目录是 .codebuddy/）：
   - 静态（npm 包 @tencent-ai/codebuddy-code v2.184.0，沙箱安装于 ~/workbuddy-npm，dist/codebuddy.js 35MB）：项目配置目录由 getProjectHomeDir 解析——$CODEBUDDY_PROJECT_CONFIG_DIR，否则产品 dataLayout.configDir.projectDirName，否则默认 `.codebuddy`；resolveProductIdentity 里 userDirName 与 projectDirName 默认值都是 ".codebuddy"；五个产品配置文件（product.json 等）均无 dataLayout，默认值生效。".workbuddy" 在 bundle 里 7 处，全是用户级 home（settings.json、models.json、logs、failover），项目级 0 处；isWorkBuddyProduct 的 .workbuddy 分支只出现在用户级路径，不碰项目目录。
   - 行为（沙箱实测）：项目里同时放 .codebuddy/settings.json 与 .workbuddy/settings.json（各注册一个 SessionStart hook 写不同 marker），两种模式各跑一次（独立 CLI；CODEBUDDY_HOST=workbuddy-desktop 模拟应用拉起，均配隔离 CODEBUDDY_CONFIG_DIR）：两次都只有 .codebuddy 的 marker 出现，.workbuddy 的一次都没触发。
   - (b) WORKBUDDY_CONFIG_DIR / CODEBUDDY_CONFIG_DIR 只影响用户级 home（settings、models、mods、会话、日志），不影响项目目录（行为测试：CODEBUDDY_CONFIG_DIR 指向隔离 home 时项目 .codebuddy/settings.json 照样加载）。
   - (c) settings.json（hooks）经行为测试证实加载；agents/skills/commands 同样挂在项目配置目录下（代码：getProjectHomeDir()+agents、getProjectSkillsDir、命令提示写明 ${projectDirName}/commands/）。
   - (d) 应用内 CLI 可直接调：/Applications/WorkBuddy.app/Contents/Resources/app.asar.unpacked/cli/bin/codebuddy（agent-avatar 2026-08-28 真机验证），或 npm 的 codebuddy；不需要应用先启动（smz2009 的 workbuddy-cli 包装直接无 GUI 拉起）。无登录时会话在模型调用前报“Authentication required”退出，但 SessionStart hook 在此之前已触发（见验收）。
   - 来源：agent-avatar connectors/workbuddy/README.md（2026-08-28 真机验证，CLI v2.115.0）、smz2009/agents-platform mechanism-probing 文档（WorkBuddy AI 5.5.2）、codync.dev/docs/agents、openagents PR #654、treerouter 指南。注意：两来源对应用侧 home 名字说法不一（agent-avatar 说 ~/.workbuddy，smz2009 说 ~/.workbuddy-ai），可能版本不同；不影响本结论（两者都是用户级）。
2. 实现（复用，不新增条目）：代码 0 改动。只补文档 4 处：README.md 与 README.cn.md 各在 init 小节后加一行“WorkBuddy 用户请用 --codebuddy”；教程中英文附录平台 flag 表在 --codebuddy 后标注（WorkBuddy 用户用它）。注册表仍为 22 个平台（platforms_test.go 的 22 断言不动）。
3. 验收：gofmt/build/vet 全过；全量测试 21 包全过；沙箱 init --codebuddy：.codebuddy/ 清单齐全（4 agents、2 命令、settings.json、10+ skills）、裸 {{ 0 处、settings.json 合法 JSON、hook 全指 sunshine hook（SessionStart/PreToolUse/UserPromptSubmit）；CLI 烟囱测试：真实 sunshine 二进制在 PATH 上、CODEBUDDY_HOST=workbuddy-desktop + 隔离 CODEBUDDY_CONFIG_DIR，在已安装项目里跑 CLI -p——SessionStart hook 真执行了（证据：.sunshine/.runtime/update-check-codebuddy_<session-id>.marker，session_start.go 写出，时间与本次运行一致），随后因无登录报 Authentication required 退出（预期内）。
4. 未验证：WorkBuddy 桌面应用是否拾取安装——沙箱没有该应用，只能等用户在 Mac 上实测（应用内建新会话，看 hook 是否触发、/sunshine:continue 是否可用）。另：mem 适配器与 Channel 不做（本次只做平台安装层）。
发版：本批不发版，等用户点选。

## Codex 两处小修：HasHooks 改 true 与 init 代写开关（2026-11-17，未发版）

1. HasHooks 改 true：internal/platforms/platforms.go 的 codex 条目 `HasHooks: false` → `true`（hooks.json 自 v0.5.18 已装 5 个事件，false 与现实不符）；platforms_test.go 的 codex 期望元组同步改。附带发现：HasHooks 不只被测试读——模板占位符 `{{#HAS_HOOKS}}` 条件块按它渲染（platforms.go:1197），翻转后 Codex 走“有钩子”分支；模板里实际用该条件块的地方只有 bundled-skills.md 的说明文字（未用作条件），安装内容不受影响。TestResolvePlaceholders 的 Codex 断言同步改成“有钩子”，并补 DSH 作“无钩子”分支。
2. init 确认后代写开关：internal/platforms/platforms.go 新增 `EnableCodexHooks() error`（CodexHooksEnabled 旁边）：行级编辑 `~/.codex/config.toml`，保留其他内容与注释；文件不存在则连 `~/.codex/` 目录一起创建；无 `[features]` 节则末尾追加；已有 `hooks = false` 改 true；已有旧名 `codex_hooks = false` 也一并改 true；已是 true 的行不动；写失败返回 error。internal/cli/init.go：`printPlatformSummary` 加 `assumeYes` 参数（三处调用传 `initYes || !stdinIsTTY()`），codex 分支改成——已开启打印确认；未开启时交互问一句（survey Confirm，默认 Yes），确认后调 EnableCodexHooks，成功打印“已开启 Codex hooks（~/.codex/config.toml），仍需在 Codex 里运行一次 /hooks 审批”，失败打印错误 + 原提示且不让 init 失败；答 no 打印原提示。提问函数 `promptEnableCodexHooks` 沿用 survey（promptStatusline 同款，测试可替换）。提示文案原文（survey 渲染，默认 Yes 显示为 `(Y/n)`）：“? 是否帮你开启 Codex hooks？（将写入 ~/.codex/config.toml 的 [features] hooks = true） (Y/n)”。`-y`/非交互视为已确认，直接写。
3. 文档最小改动：docs/tutorial.en.md 里那段 `init --codex -y` 的实录输出按沙箱实测改成新行（旧提示行换成“↳ 已开启 Codex hooks……”），中英翻译句同步改；templates/platforms/codex/config.toml 顶部注释把“只提示，不代写”改成问一句、确认后代写（/hooks 评审仍须用户跑）。中文教程 tutorial.md 里没有这句提示，未改。
4. 验收：gofmt/build/vet 全过；全量测试 21 包全过；EnableCodexHooks 表驱动单测 8 例 + 目录被占错误例全过（含幂等断言）。沙箱 `init --codex`（隔离 HOME）：①已开启时打印确认、文件 sha 不变；②未开启 + `-y` 时写进 `[features] hooks = true`；③交互 PTY 喂 `y` 时写进、喂 `n` 时不写只打印原提示（`~/.codex` 未创建）；④hooks.json 与模板逐字节一致。交互验证踩坑：survey 发 `ESC[6n` 查光标位置，`script` 直喂输入会卡死（无终端模拟器回包），改用 Python PTY 驱动（收到查询回 `ESC[24;1R`，等提示出现再按键）才跑通。只改 init，未碰 `sunshine update`。
5. /hooks 自动批不在本次范围（用户已定不做）：Postman 前车之鉴（2026-11-17 向用户汇报过：安装器代写 trusted_hash 遭 HN 曝光）+ trusted_hash 哈希机制未文档化（每次 hooks.json 一变就得重批）+ 收益仅一次点击。init 只代写开关，评审仍由用户在 Codex 里点 Allow all。
发版：本批不发版，等用户点选。

## v0.5.24 发布记录（2026-11-17，已发布）

- 内容：两批合发——①Codex 两处小修：注册表 HasHooks 改 true、init 确认后代写 [features] hooks=true（-y 直接写，交互默认 Yes 问一句；写失败不让 init 失败；/hooks 自动批不在本次范围）；②WorkBuddy 接入：复用 --codebuddy，只补文档，代码 0 改动。
- 主提交：本地 eda4afefa7272c57df18ab147f668b1b27aed454 → 远端 aeb2033fdf4e131d74f509833708501262a54a9c，tree d9c025a2e66e7796e106f0f05e6b9e0fcea8350e，父提交 04fdcbb34f86ad6a556b768e6288b1445f45655b，作者 linrong007。
- tag：v0.5.24，annotated tag，tag 对象 3337850df5b2ca4f99282ab339999e7c896e3dc2 → aeb2033。
- CI 与发布：发布 workflow run 44094820616、CI 两路 run 44094711785（main）与 44094820648（tag）均 completed success，无抖动。
- Release：id 448686724，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.24_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 一键装：macOS/Linux `curl -fsSL https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh | sh`；Windows `irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex`。

## spec 全认 + 模板市场简化（2026-11-18，用户定，未发版）

背景（2026-11-18 早上用户问清机制后定的方向）：
- spec 全认：不限于 system/frontend/backend 三份；.sunshine/spec/ 下所有 .md 都认。旧问题：平铺文件（api.md、ui.md 等）不在发现路径上——SessionStart 只列子目录 index.md、task context 只列分层名、工作流文字只点名 frontend/backend.md，不手动登记进 implement.jsonl 就不起作用；默认三份也是靠点名才没掉队；spec 模型（包/分层树）与默认模板形状（平铺三文件）拧巴。
- 适用标签：每个 spec 文件头写 YAML frontmatter `适用：前端/后端/通用`；缺了按文件名自动补（文件名含 frontend→前端，backend→后端，system→通用，其他→通用）；模板作者写了的以作者的为准，安装不改写。
- spec 地图：SessionStart 注入一张地图——spec/ 下全部 .md（递归），按适用分组，一行一个（路径 + 索引章节名）；前端任务读前端组+通用组，后端任务读后端组+通用组。jsonl 保留，1.4 挑文件改按适用规则办（不再靠 agent 猜）。
- 安装默认只补缺失：init --registry 默认只装没有的文件，已有的不动；--overwrite 全量替换（整目录清空再装，含用户自己写的文件，慎用）；--append 保留，与默认同义。
- 自动选：marketplace 里恰好只有一个 type=spec 的条目时，--template 可省，自动选中它；多个 spec 条目仍须指定；没有 spec 条目报错；skill 安装仍须显式 --template（自动选只针对 spec）。
- 查重：新命令 `sunshine spec check`，比对各文件索引章节，标题相同或内容相同的节列出来；安装后自动跑一遍，只报告、不阻止安装。
- registry.spec 留着：config.yaml 里 source+template 两行，update 靠它刷新；默认市场先不做（没有官方市场仓库可默认，仓库要转私有以后再说）；单 flag 直接指模板目录的写法已可用（直接下载模式，不走 index.json）。
- 仓库根语义：根 = 来源字符串指向的目录（可带子目录/分支）；index.json 必须在根下，条目 path 相对根写；直接模式取根/marketplace/specs/（若有）否则根本身；子目录不得越出仓库。

实施：
1. specindex：加适用标签解析与自动补。SyncContent/SyncFile 同步时：文件无 frontmatter 适用行则按文件名推断补在文件最前面（`---\n适用：X\n---` + 空行 + 原内容）；已有标签不动。索引同步只插标题后/替换索引节，不得动 frontmatter。SessionStart 兜底同步同样补标签（覆盖 update 刷新进来的文件）。单测：四种文件名推断 + 已有标签不动 + frontmatter 与索引共存。
2. 默认模板加标签：templates/sunshine/spec/system.md→通用、frontend.md→前端、backend.md→后端（frontmatter 写死）。
3. SessionStart 地图：internal/hook/session_start.go 的 collectSpecIndexPaths 改成扫 spec/ 下全部 .md（递归；跳过 guides/ 下文件，但 guides/index.md 保留在列，与现状一致；monorepo spec_scope 过滤只对子目录生效，平铺文件恒列出）。注入文案把 "Available indexes" 改成按适用分组的地图（通用组/前端组/后端组，每行 `路径 — 章节1、章节2`，无索引节只列路径）。checkLegacySpec 若把平铺当 legacy 警告，改掉/删掉（平铺现在是正经形态）。
4. `task context --mode packages`：text 输出改成同一张地图（按适用分组）；JSON 保留 specLayers（分层名照旧算），新增 specFiles（路径+适用）。internal/task/context.go 的 scanSpecLayers 不再是唯一输出。
5. 工作流模板三份（templates/workflows/*/workflow.md）：开头 spec 组织一节（“按包与分层组织”）改成新模型（一叠文件 + 适用标签 + 地图）；1.4“放什么”规范文件一节改成适用规则（前端任务取适用=前端+通用，后端取适用=后端+通用，都沾的都取；只放本任务用得上的）；“如何发现相关规范”改成读 SessionStart 地图 + `task context --mode packages`；原型派发那节“（如 .sunshine/spec/frontend.md 里的设计约束）”改成“适用=前端的 spec 文件里的设计约束”；1.4 示例命令的 backend.md 路径保留作示例。
6. registry 默认策略：internal/cli/registry.go 的 resolveRegistryStrategy 默认返回 StrategyAppend（只补缺失）；--append 保留；--overwrite 保持；--overwrite/--append 不再强制要求 --template（校验移到目标解析后，自动选能补上时放行）。copyWithStrategy 的模板级 skip（目标目录已存在则整个跳过）在默认路径下不再触发（append 走 copyMissing）。
7. 自动选：internal/registry/install.go 的 ResolveTarget/ResolveTargetWithProbe/Install：marketplace 模式且 templateID=="" 时，恰好一个 spec 条目则自动选中；多个 spec 条目报错列出 id；0 个 spec 条目报错“该市场没有 spec 模板”。交互选单（resolveInteractiveTemplate）不动（交互本来就要挑）。单测覆盖三情形。
8. 安装后收尾：installRegistryTemplates 成功后按顺序做——对新装文件跑 specindex.SyncDir（补索引+补标签）→ 跑查重打印重叠章节警告（不失败、不阻止）→ 记哈希 → 写 registry.spec（哈希必须在同步之后记，否则同步改了文件哈希就 stale）。失败回退（内置空白模板）不变。
9. `sunshine spec check`：internal/cli/spec.go 加 check 子命令；specindex 加 CheckDuplicates：扫 spec/ 全部 .md，取各文件索引章节（无索引节的按 ## 标题现算），标题相同（跨文件）或同一标题下节正文哈希相同，列出文件对+章节；无重叠打印“未发现重复”。单测：标题撞车、内容相同、标题不同内容不同三例。
10. 文档：docs/tutorial.md 与 docs/tutorial.en.md 模板市场一节通俗改写/新增（两种模式、仓库根语义、用法：新项目/已有项目/单 flag/--overwrite 注意、配置示例 config.yaml registry.spec 两行、市场目录 tree + index.json 四字段、skill 一节、适用标签与地图一句话）；internal/registry/marketplace.md 同步更新（默认 append、自动选、适用标签、spec check、仓库根语义）。README 暂不动。
11. 验收：gofmt/build/vet 全过；全量测试 21 包全过（跑前后清 /tmp/sunshine-test-bin-* 与 /tmp/go-build*）；沙箱 e2e：①新项目 init --registry --template（spec 文件带标签+索引，默认三份不生成）；②单 spec 模板市场不给 --template 自动选；③多 spec 模板不给 --template 报错列出 id；④已有项目再跑默认只补缺失（删一文件再跑只补它）；⑤--overwrite 全换（含用户文件被清）；⑥spec check 检出重复标题；⑦skill 安装落点 .agents/skills/<id> 且默认 spec 照常生成；⑧单 flag 直接指模板目录；⑨SessionStart 注入含地图（平铺文件在列、按适用分组）；⑩grep 残留（“按包与分层组织”、旧 skip 默认文案）。

## v0.5.25 发布记录（2026-11-18，已发布）

- 内容：spec 全认 + 模板市场简化：适用标签、规范地图、默认只补缺失、自动选、spec check、官方市场 `marketplace/`（6 条目）；另含一处改名：`templates/sunshine/spec/frontend.md` 的 `## 约束` → `## 前端约束`（marketplace 镜像与教程同步），新二进制装市场无重复提醒，`spec check` 退出 0。
- 主提交：本地 2ab86bfd3fceabd26dc95821ebc106b98fd147ea → 远端 bd44a6280e4324df3c7555c8d5c4538554b15b24，tree 46328c09c8660dfd7f74108a4d280e4d0d04aaeb，父提交 c482f0cf0dde1d890eaa74fd3cc518ce1fdc5ba6，作者 linrong007。33 个文件，+4164/-225。
- tag：v0.5.25，annotated tag，tag 对象 90a57a3d916ba898e5abff3fbfaa348e2be7d6ff → bd44a62。
- CI 与发布：发布 workflow run 44223122480、CI（tag）run 44223122485、CI（main）run 44223060926 均 completed success。
- Release：id 449448302，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.25_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.25（linux/amd64）；darwin_arm64 包 checksum OK，Mach-O arm64；0.5.24 二进制 `upgrade --dry-run` 查到 0.5.25（初次报 GitHub 匿名接口 403 限流，10 分钟后重试通过）。

## 局域网 http 修复（2026-11-18，用户定修，已随 v0.5.26 发布）

背景：用户问局域网 git 服务器能不能当市场来源。实测：SSH 局域网代码支持（未实测）；局域网 plain http 走不通。根因：`buildGitURL`（internal/registry/parse.go）自建域名统一拼 `https://%s/%s.git`，`gitURLInput` 无 scheme 字段，`ParseRegistrySource` 里 `httpsUnknownRe` 虽匹配 `https?`，但 scheme 未保留——用户写 `http://192.168.1.10:3000/team/market.git`，克隆时被改成 `https://…`，对只讲 http 的服务器报 `tls: first record does not look like a TLS handshake`（本地 http git 服务器实测，git http-backend + python http.server CGI，端口 18080）。自建来源 `PreferGit=true`，探测走 git 克隆，所以失败点在克隆，不在 RawBaseURL。

实施：
1. `gitURLInput` 加 scheme 字段（"http"/"https"，默认 "https"）；`ParseRegistrySource` 从原始来源取 scheme（仅 `httpsUnknownRe` 匹配到的自建 http/https URL；giget 前缀与公开域名不走这条，仍强制 https）。
2. `buildGitURL`：host 非空时按 scheme 拼（`%s://%s/%s.git`）；公开域名分支不动。
3. `RawBaseURL`：自建来源的 `https://gitlab.com` 替换改按 scheme（`scheme://host`），与 GitURL 一致（探测走克隆时用不上，HTTP 回退路径要一致）。
4. 单测（parse）：`http://192.168.1.10:3000/team/market.git` → GitURL 为 http；`https://` 同 → https；`https://github.com/org/repo` → `https://github.com/org/repo.git` 不变；`gh:` 前缀不变。
5. 沙箱 e2e：本地 http git 服务器（git http-backend CGI：cgi-bin/git 脚本设 GIT_PROJECT_ROOT 与 GIT_HTTP_EXPORT_ALL 后 exec git-http-backend，python3 -m http.server --cgi 18080）跑 `init --registry http://127.0.0.1:18080/cgi-bin/git/team-market.git --template team-spec` 成功装 spec；注意 /tmp 只有 512M，跑全量测试前后清 /tmp/sunshine-test-bin-* 与 /tmp/go-build*，且不要用 pkill -f 杀服务器（命令行含匹配串会杀掉自己的 shell，改用 curl 探活 + 按 PID 杀）。
6. 教程：中英文模板市场局域网小节改写——http:// 局域网可用（删“走不通”句，改写为 http/https 均可）；internal/registry/marketplace.md 同步。README 不动。
7. 验收：gofmt/build/vet 全过；全量测试 21 包全过；沙箱 e2e（上条）；grep 残留（"走不通"）。

不在本次范围：自签证书 https 仍无跳过校验选项；git:// 协议仍不支持。

实施与验证（2026-11-18，已实施并验证通过，已随 v0.5.26 发布）：发版协调人按上节 7 步实施；主会话独立验证——diff 与方案一致（parse.go +16/-2，source_test.go +30，marketplace.md + 教程中英文）；gofmt/build/vet 全过；全量测试 21 包全过；沙箱 e2e：本地 http git 服务器（git http-backend CGI + python3 -m http.server --cgi 18080），`init --registry http://127.0.0.1:18080/cgi-bin/git/team-market.git --template team-spec` 成功装 team-spec（2 个文件），registry.spec 记 http 来源，默认三份未生成，适用标签自动补；grep 残留“走不通”与英文 “doesn't work yet” 在 docs/ 与 internal/ 均为 0 处。e2e 复验时沙箱 CGI 踩坑两处（已记 AGENTS.md）：Python CGI 子进程 setuid(nobody) 跑脚本，脚本须全局可执行、目录链须可穿越（umask 0007 建的 770 会报 PermissionError，curl 只看状态码是假阳性，须看 body 大小/内容）；裸仓库属主 root 会触发 git dubious ownership，须用 nobody 身份建库（runuser -u nobody 复制源码后 init/commit/clone --bare），沙箱里 chown 改不了属主。工作树另有模板市场用法补记（教程 §8 中英文，约 30 行），与本批一起随 v0.5.26 发布。

发版：已随 v0.5.26 发布（见下节）。

## v0.5.26 发布记录（2026-11-18，已发布）

- 内容：局域网 http 修复 + 模板市场用法补记 + v0.5.25 发布记录。http 修复：`gitURLInput` 加 scheme（默认 https），仅自建 http/https URL 取值；`buildGitURL` 与 `RawBaseURL` 按 scheme 拼；单测 4 例；沙箱本地 http git 服务器实测 init 装 team-spec 成功；教程中英文局域网小节与 marketplace.md 同步（http/https 均可）。教程 §8 补记模板市场全部用法（两种模式、交互选单、0 spec 报错、skill 也记 registry.spec、装失败回退、自己建一个市场）。
- 主提交：本地 66eef67b9e8447485348665a476311001a755c13 → 远端 c95fac4f40834d61077a571876a04e2629b81444，tree 53f571b9ebe6d2eddf4b8003ce3e4e76d86c64ae，父提交 bd44a6280e4324df3c7555c8d5c4538554b15b24，作者 linrong007。6 个文件，+124/-12。
- tag：v0.5.26，annotated tag，tag 对象 d0a631f676b4ff055c468ced5023cc0a845d0c69 → c95fac4。
- CI 与发布：发布 workflow run 44227240921、CI（tag）run 44227241116、CI（main）run 44227193150 均 completed success。
- Release：id 449470432，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.26_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.26（linux/amd64）；darwin_arm64 包 checksum OK，Mach-O arm64（macOS 二进制在 Linux 沙箱上不可执行，Exec format error 属预期，只核 checksum 与 file）；0.5.25 二进制 `upgrade --dry-run` 查到 0.5.26（一次通过）。


## 原型提速三处（2026-11-19，用户定做，已随 v0.5.27 发布）

背景：用户问设计阶段产出原型会不会过度编程，并指出产出原型很慢。定：mock/JS 该写就写，标准是够用为止，不是能省则省（“最少演示”——静态占位、不写 JS——用户已否掉，提得过了）。慢的根因三处（已核模板原文）：①每轮派发把整个 `doc/prototype/` 目录给子代理，保一致要读已有页，页越多输入越大；②一页高保真带 mock + JS，输出本身几千上万 token；③一轮一页串行，6 个需求就是 6 次冷启动。修法只砍浪费、不砍保真度。

实施：
1. 派发收窄：三份 workflow.md（native/tdd/channel-driven-subagent-dispatch）的原型派发段落 + 18 个 prototype agent 文件的上下文节，改成只给本轮只读文件（`doc/prototype/index.html` 导航入口 + 共用样式/mock 文件 + 本轮要做/改的那一页，如已有），不要给整个 `doc/prototype/` 目录，不要读其它已有页。agent 文件里的三处措辞同步改（派发消息/派发提示、上下文列表、工作流第 1 步）。
2. 共用强制：agent 原型要求节，“共享样式、mock 数据抽成共用文件，各页引用”改成必须抽共用：新页不得重复写已有共用文件里的样式、mock 数据或导航代码；共用文件里没有的才在本页写。
3. 单页 400 行上限：agent 原型要求节新增一条——每页 HTML 不超过 400 行（`wc -l` 为准，含本页内联样式与脚本，不含引用的共用文件）；超了必须拆（重复块抽共用组件，或次要区块拆子页），不砍需求写明的功能与交互，不把该写的 mock/JS 换成静态占位。400 行有实据：教程 §12.6 实录四页为 72/145/215/86 行，mock.js 17 行，均在上限内。
4. 不动：docs/tutorial.md 与 .en.md 的 §12.6 是 s6-loop 真实实录，不改写（其中“输入三样……已有原型目录”是当时的派发方式，属历史记录）；不改 Go 代码，只改模板文字；已装好的项目跑 update 才生效。18 个 agent 文件：源文件 templates/sunshine/agents/prototype.md（channel 运行时，措辞“派发消息”）+ 17 个平台副本（措辞“派发提示”；含 codex .toml 与 kiro .json 两种格式，改其内嵌正文）；平台副本互不相同（claude、codebuddy、qoder、trae 四份逐字节相同，其余各异），逐文件按各自原文改，不许用一份覆盖另一份。

验收：gofmt/build/vet 全过；全量测试 21 包全过（跑前后清 /tmp/sunshine-test-bin-* 与 /tmp/go-build*）；grep 残留：templates/ 内“已有原型目录”为 0 处（docs/ 教程实录中文 2 处 + 英文 1 处保留，不改）；新二进制 `grep -a -c` 核三处新文案在位（只读文件、共用强制、400 行）；逐文件核对 21 个文件（3 workflow + 18 agent）改完且格式未破（toml/json 须过解析）。

发版：已随 v0.5.27 发布（见下节）。


## v0.5.27 发布记录（2026-11-19，已发布）

- 内容：原型提速三处——派发收窄 + 共用强制 + 单页 400 行上限。只改模板文字，不改 Go 代码；教程 §12.6 实录未动。沙箱实测（DSH headless，3 页图书借阅管理）：总 10.3 分钟，约 3.4 分钟/页，三页 177/385/252 行均 ≤400，派发收窄实证（R-003 派发点名 8 个文件，三轮子代理只读 index + 共用文件）；墙钟基本没变（实测共用文件本身涨到 1620 行成大头，第 3 轮只省约 25%，早先按教程实录算的 70% 已订正），并行先不做，用户 11-19 定保持一轮一页。
- 主提交：本地 2f63bd80fdf305caad526adcc97fe4b8f2981841 → 远端 4ff850fcc0516bc12d154e6012c335e52501f349，tree 821c3a6cf6f80d2de5455aba247c90e221b07391，父提交 781c6dcf78a3f3f9087bc1ef3e7b94e823e75ff5，作者 linrong007。22 个文件，+105/-74。
- tag：v0.5.27，annotated tag，tag 对象 af234b8b8848c0d4f6fc1cefbdda3c3da8dee0fe → 4ff850f。
- CI 与发布：发布 workflow run 44458856999、CI（tag）run 44458856898、CI（main）run 44458824686 均 completed success。
- Release：id 450884046，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.27_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.27（linux/amd64）；darwin_arm64 包 checksum OK，Mach-O arm64（macOS 二进制在 Linux 沙箱上不可执行，Exec format error 属预期，只核 checksum 与 file）；0.5.27 二进制 `upgrade --dry-run` 报已是最新。实测中 /tmp 周期清理会把 12MB 的 darwin 包删掉（两次下载后都消失），以后大文件验 checksum 改放 ~/workspace/scratch/。


## 单页上限 400→500 行，个别情况允许超（2026-11-19，用户定，已随 v0.5.28 发布）

背景：用户问 400 行会不会有点少。实据：沙箱实测最复杂一页（借阅登记页）385 行，离上限只差 15 行；再复杂一点的表单页、数据密集表页就会撞线；拆子页能过，但演示一页变多页会碎。400 行只算页面本身（样式与 mock 在共用文件里不占），撞线通常是这页自己的结构加交互确实多。

实施：18 个 prototype agent 文件（源文件 + 17 个平台副本，含 codex .toml 与 kiro .json）原型要求节同一条，改成“单页 500 行上限：每页 HTML 不超过 500 行（`wc -l` 为准，含本页内联样式与脚本，不含引用的共用文件）；超了必须拆——重复块抽成共用组件，或次要区块拆成子页；不砍需求里写明的功能与交互，不把该写的 mock/JS 换成静态占位。个别情况允许超：单页确实需要更多行时（如大表单、数据密集的表），可超上限，但汇报里要写明超了多少行和原因。”500 行有实据：实测三页 177/385/252 行全在 500 内还有余量。workflow 三份不动（不含行数），PLAN.md 旧节与 v0.5.27 记录里的 400 行是历史记录，不改。

验收：gofmt/build/vet 全过；全量测试 21 包全过（跑前后清 /tmp/sunshine-test-bin-* 与 /tmp/go-build*）；grep 残留：templates/ 内“400 行”为 0 处；新二进制 `grep -a -c` 核“500 行上限”在位；逐文件核对 18 个文件改完且格式未破（toml/json 须过解析）。

发版：已随 v0.5.28 发布（见下节）。


## v0.5.28 发布记录（2026-11-19，已发布）

- 内容：单页上限 400→500 行，个别情况允许超（单页确实需要更多行时可超，汇报写明超了多少行和原因）。18 个 prototype agent 文件同一条；workflow 三份不动；PLAN.md 旧节与 v0.5.27 记录里的 400 行是历史记录，不改。
- 主提交：本地 e229a159d7af02ac1081c956cd7e4603aab8931a → 远端 a06d9bd76e4df6dbbd34f091d2e208b32e9e4d71，tree 1543f7dfbc135cba64c504051a3f7d62ee0ba6f4，父提交 d62afa2b15bf2c96787e2b2e74d97f13107c384f，作者 linrong007。19 个文件，+29/-18。
- tag：v0.5.28，annotated tag，tag 对象 b403239303658ed1e87788537ef01c07563a0986 → a06d9bd。
- CI 与发布：发布 workflow run 44467847386、CI（tag）run 44467847541、CI（main）run 44467816459 均 completed success。
- Release：id 450933957，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.28_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.28（linux/amd64）；darwin_arm64 包 checksum OK，Mach-O arm64（macOS 二进制在 Linux 沙箱上不可执行，Exec format error 属预期，只核 checksum 与 file）；新装二进制 `grep -a -c` 核“500 行上限”与“个别情况允许超”各 15 处、“400 行上限”0 处。遗留：`upgrade --dry-run` 两次均报 GitHub API 403 rate limit exceeded（沙箱出口 IP 共享、未认证请求限额已满，与发布本身无关；v0.5.27 同命令曾通过），沙箱内无法复验，用户 Mac 上可直接跑 `sunshine upgrade --dry-run` 验证。


## CSS 和 mock 拆分（2026-11-19，用户定，已随 v0.5.29 发布）

背景：三处提速发完 v0.5.27 后实测墙钟基本没动；用户问还有什么地方可以加快速度。结论：一轮一页的前提下墙钟大头已无处可砍（并行是唯一大改，用户定不做），剩下的是省 token。共用文件本身成了输入大头：沙箱实测共用文件 1620 行（prototype.css 1079、mock.js 314、nav.js 122、ui.js 105），第 3 轮新派发读 1644 行、旧派发约 2206 行，只省约 25%。

实施：共用文件拆开，都放在 `doc/prototype/assets/` 下——样式拆三件：`tokens.css`（设计 token：颜色、字体、间距、圆角）+ `components.css`（组件样式）+ `components-index.md`（组件索引，一行一个类名加一句话说明，新增组件样式时同步加一行），每轮只读 tokens 和组件索引，`components.css` 只在新增或改组件样式时读相关部分，不读全文；mock 拆两件：`mock-common.js`（跨页共用的实体数据）+ `mock-Rxxx.js`（本页数据，Rxxx 为需求编号），每轮只读 mock-common 和本轮的 mock 文件，不读其它页的 mock 文件。nav.js、ui.js 不动（122/105 行，仍整份读）。改动：18 个 prototype agent 文件（上下文第 3 项 + 原型要求节新增拆分一条）+ 3 份 workflow 派发段落（只读文件清单改成拆分后的清单）；只改模板文字，不改 Go 代码；教程 §12.6 实录不改。

测算（非实测）：按沙箱实测文件量算，第 3 轮派发输入约从 1644 行降到约 480 行（index 24 + tokens 约 50 + 组件索引约 30 + mock-common 约 50 + nav 122 + ui 105 + 本轮 mock 约 100），省约 70%；tokens/索引/mock-common 的行数是估算，实测以真跑为准。沙箱重跑需 DeepSeek Key（沙箱无存储的 Key），等用户定。

后续：标准 UI 套件暂不做，保持开放，日后有可能从模板市场带进来（不放进内置模板）。

验收：gofmt/build/vet 全过；全量测试 21 包全过（跑前后清 /tmp/sunshine-test-bin-* 与 /tmp/go-build*）；grep 残留：templates/ 内“共用样式/mock 文件”为 0 处；新二进制 `grep -a -c` 核新文案在位（components-index.md、mock-common.js、mock-Rxxx.js）；逐文件核对 21 个文件改完且格式未破（toml/json 须过解析）。

发版：已随 v0.5.29 发布（见下节）。


## 检查代理节食（2026-11-19，用户定，已随 v0.5.29 发布）

背景：用户转述 Trellis 开发一大小程序花 60 元 token 只完成 1/10；用户要求从架构上找优化，即使推倒重来。先核了注入链：每子任务实现、检查各一次子代理注入，上限 128KB（jsonl 整理条目 + prd/design/implement.md 三份产物各上限 64KB）；SessionStart 只注一次；原型已收窄。

修正（重要）：汇报时曾说“缓存顺序调整能让检查那轮命中缓存、省约一半注入量”，核查后是错的——实现与检查是两个不同的 agent 文件（不同的 system prompt），API 请求从第一个 token 就分叉，检查不可能命中实现的缓存；顺序调整只帮同一角色跨子任务共享系统提示词加模板头那几 KB，省的很少。缓存顺序不做。真正的省法是少读，而不是指望缓存。

实施（检查节食）：检查真正需要的是 diff + 验收标准 + 相关规范节，不是三份产物全文。
- hook：GetCheckContext 不再走 getTaskContext 全量，改成 check.jsonl 精选条目（保留）+ prd.md 只取验收标准节（标题含“验收标准”的 ## / ### 节；没有则取标题含“需求”的节；都没有则全文兜底，仍受产物上限约束）+ 一行未注入说明（design.md / implement.md 未注入，需要判断实现方式是否符合设计时再按需读）。finish（[finish] 检查）用同一套瘦上下文。
- agent 文件：templates/sunshine/agents/check.md 上下文节改成节食顺序（check.jsonl → git diff → prd.md 验收标准节 → 相关规范节；design.md / implement.md 按需读）。
- 平台副本（2026-11-19 补）：18 个平台 check 文件（claude/codebuddy/codex/copilot/cursor/droid/gemini/grok/kimi/kiro/omp/opencode/pi/qoder/reasonix/snow/trae/zcode，含 codex .toml 与 kiro .json）+ check skill 的上下文段落也改成节食口径，共 40 处替换（Python 脚本逐处断言恰好一次）；此前只改了 hook 与 channel 运行时那一份，平台副本仍写“读 prd/design/implement.md 全文”、Snow 还明说 hook 注入只是面包屑要重读，拉取式平台上节食会被 agent 自己重读抵掉——已补齐。验证：旧措辞残留 0 处；kiro JSON 与 codex TOML 校验通过；gofmt/build/vet 全过，全量测试 21 包全过；新二进制 grep 确认嵌了节食文本（“检查节食”16 处、Snow 新行 1 处）。
- workflow：native 与 tdd 的 2.2 节“对照 prd.md、有则对照 design.md、implement.md”改成节食口径；channel-driven 的 2.2 节 spawn 命令去掉 `--file design.md` / `--file implement.md`（检查工作线程按需再读），`--file prd.md` 保留——channel 路径上 prd.md 仍是全文注入（assembleSpawnContext 直接拼全文，无按节切分），是本批已知缺口，不在这批补。
- 预算上限不动（单文件 32KB、单产物 64KB、总 128KB），节食后自然远低于上限；diff 不注入（检查自己跑 git diff 拿，注了也是同样的 token 数）。

验收：gofmt/build/vet 全过；全量测试 21 包全过；新单测钉住 GetCheckContext（只含验收标准节、不含 design/implement.md 全文、无验收标准节时取需求节、都没有时全文兜底）；hook 实测：用 demo14 量级的任务目录跑 `sunshine hook subagent-context`，检查注入字节数前后对比。实测结果（2026-11-19）：测试夹具 prd 10420 + design 18017 + implement 21617 字节，检查注入旧 51285 字节 → 新 1460 字节，少约 97%；新注入含验收标准一/二与 check.jsonl 两条规范节，不含目标/需求/参考/design/implement 全文；gofmt/build/vet 全过，全量测试 21 包全过（cli 22.1s、update 23.5s，跑前后清 /tmp）。

发版：已随 v0.5.29 发布（见下节）。

后续：常驻 worker（推倒版）用户 2026-11-19 定暂时不考虑；检查换便宜模型是待验证的实验，不在这批。


## v0.5.29 发布记录（2026-11-19，已发布）

- 内容：原型共用拆分与检查代理节食。原型样式与 mock 拆共用文件（样式拆 tokens.css + components.css + 组件索引，mock 拆 mock-common.js + 按页 mock-Rxxx.js，每轮只读 tokens、索引、mock-common 与本轮 mock）；检查代理节食（检查只拿 check.jsonl 精选条目 + prd.md 验收标准节，design.md/implement.md 不注入、按需再读，hook 实测检查注入 51285 → 1460 字节，少约 97%；18 个平台 check 文件 + check skill 同步改成节食口径）。44 个文件，+345/-130。
- 主提交：本地 7f2532626df70f8f07591775a8746f1f12877fce → 远端 36d05f0e0ac35367fa664f4e00d1dad79328dc6e，tree 90994fe783ba697c73daa48d54effb5350478c55，父提交 6873ba42a588ad4e4bee9256dab9ed8d4e9b4520，作者 linrong007。44 个文件，+345/-130。
- tag：v0.5.29，annotated tag，tag 对象 5815e88a959eaf296b40bff7643331aa46f1b8e7 → 36d05f0。
- CI 与发布：发布 workflow run 44521558386、CI（tag）run 44521558495、CI（main）run 44521445354 均 completed success。
- Release：id 451299884，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.29_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.29（linux/amd64），`sunshine version` 报 0.5.29；darwin_arm64 包 checksum b37ad3964a9c1515b9ac1c19d858e7456c6c498efd8d9e41044d442c0dd5ed64 与 checksums.txt 一致；`upgrade --dry-run` 这次通过（“已是最新版本（当前 0.5.29，最新 0.5.29），无需升级。”），v0.5.28 时沙箱曾两次报 403 rate limit。


## 工具输出压缩（模板措辞第一档）（2026-11-19，用户定，已随 v0.5.30 发布）

背景：用户 2026-11-19 问还有没有省 token 的方案；第二轮调研出四条（工具输出压缩、子任务边界压缩、符号级检索、思考预算上限），另有两条不适合（Batch API、输出压缩）。用户定先做工具输出压缩，模板措辞这步先行，hook 改写命令那档后定。

实施（第一档，模板措辞）：只改模板文字，不改 Go 代码。
- implement/check agent 文件新增“## 命令输出要精简”节：测试 / lint / 类型检查只看失败项与摘要，通过的不要贴全文；长输出加 `| head -n 100`；`git log` 用 `--oneline`；汇报只写结论与失败项，不要把命令输出全文贴进汇报。
- check 另加一条：`git diff` 先 `--stat` 看改动规模，再按文件 `git diff -- <文件>` 读具体改动，不要一上来就全量 `git diff`；复查只读改过的文件。check 工作流步骤 1 的全量 `git diff` 同步改成 `--stat` + 按文件。
- 范围（41 个文件）：canonical implement.md/check.md + 18 个平台 implement/check 副本（claude/codebuddy/codex/copilot/cursor/droid/gemini/grok/kimi/kiro/omp/opencode/pi/qoder/reasonix/snow/trae/zcode，含 codex .toml 与 kiro .json，按转义原文替换）+ check skill + native/tdd 两份 workflow 的 2.1/2.2 验证行（12 处，加“（输出要精简：只看失败项与摘要）”）。
- 落点：标准 .md 副本插在“## 汇报格式”前；omp/pi/reasonix implement 无汇报格式节、附在文件末尾，check 插在“## 输出”前；codex implement 插在“完成前，总结：”前，check 插在“输出格式：”前；kiro 按 JSON 转义原文插在“## 汇报格式”前。
- check skill：步骤 1 加 `git diff --stat HEAD`，步骤 3 加只看失败项与摘要。

不做（本批）：hook 改写命令（第二档，RTK 式 PreToolUse 改写，需设计自带压缩器还是只改写精简 flag，等第一档验证后再定）；子任务边界压缩、符号级检索、思考预算上限（下一版候选，用户未定）。

效果预期（实话）：模板措辞靠 agent 自觉，效果不确定，不承诺具体数字；实证有效的数（RTK 独立基准每会话实省约 20-43%）是 hook 改写那档测出来的，不是本档的。量法：固定任务跑前后对比（待定；沙箱 DSH 无 Key，需用户定）。

验收：gofmt/build/vet 全过；全量测试 21 包全过；新二进制 grep 确认新文案在位；各文件措辞逐处断言恰好一次。

验证（2026-11-19）：41 个模板文件 + PLAN.md，+372/-32；kiro JSON 与 codex TOML 解析通过；新节“命令输出要精简”每文件恰好 1 处（38 个文件）；旧全量 `git diff` 代码块与旧步骤 1 行残留 0 处；工作流括号 12 处（native/tdd 各 6）；`git diff --stat` 模板内 38 处；gofmt 无输出、build/vet 全过；全量测试 21 包全过；新二进制“命令输出要精简”36 处、“输出要精简：只看失败项与摘要”12 处、“git diff --stat”36 处——二进制比模板少 2 处是链接器去重（claude 与 codebuddy 的 implement/check 两对文件逐字节相同，v0.5.27 已有同类去重记录）。

发版：已随 v0.5.30 发布（见下节）。


## v0.5.30 发布记录（2026-11-19，已发布）

- 内容：工具输出压缩（模板措辞第一档）。implement/check agent 文件新增“命令输出要精简”节（测试 / lint / 类型检查只看失败项与摘要，通过的不要贴全文；长输出加 `| head -n 100`；`git log` 用 `--oneline`；汇报不贴命令输出全文；check 的 `git diff` 先 `--stat` 再按文件）；check skill 与 native/tdd 两份 workflow 同步。41 个模板文件，只改模板文字，不改 Go 代码。
- 主提交：本地 39c793cd8f014ec3feca97aae4d308881a00570f → 远端 348e43c6cb7b294d5aa85f071d1aea55f6a4d450，tree c457cbc3c700b3b106780a71d18bdf5c641dc887，父提交 0123d7a2a5bf089403edfb419c9deedced197c59，作者 linrong007。42 个文件，+374/-32。
- tag：v0.5.30，annotated tag，tag 对象 6108fc3c2e89d740feeffbcbe5288132e3f1f1ac → 348e43c。
- CI 与发布：发布 workflow run 44542343736、CI（tag）run 44542343712、CI（main）run 44542296040 均 completed success。
- Release：id 451451858，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.30_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.30（linux/amd64），`sunshine version` 报 0.5.30；darwin_arm64 包 checksum d50052533458d1b87ab2580e03758e3d2188bdd33ed0df6f8d24d1a7c941ba18 与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.30，最新 0.5.30），无需升级。”）。


## 注入顺序与会话去重（2026-11-19，用户定，已随 v0.5.31 发布）

背景：GitHub 第三轮调研（2026-11-19）出 5 条，用户定 1、2 号一起做。1 号注入顺序（cachesplit 残余版）、2 号会话去重（sqz 去重半截）。

### 1 号：注入顺序（稳定在前、易变在后）

现状：四个子代理提示词 builder（`internal/hook/subagent.go` 的 buildImplementPrompt / buildCheckPrompt / buildFinishPrompt / buildResearchPrompt）都是“固定头 + context（稳定）+ originalPrompt（易变，含派发文本）+ Workflow / Constraints（固定、稳定）”；Codex 的 buildCodexSubagentContext 是“Active task（易变）+ Curated Context（稳定）”。易变内容夹在中间，把其后的稳定段也拖出缓存前缀。

改法：纯重排，字节不变。四个 builder 改成“固定头 + context + Workflow / Constraints + Your Task（originalPrompt）”；Codex 改成“Curated Context + Active task”。agent 定义文件本身已跨子任务逐字节稳定，不动。

收益预期（实话）：未在 Sunshine 上实测；cachesplit 的 34% 是代理改写 system prompt 测出来的，不是这个改动的数。同一角色跨子任务的前缀更长，显式断点（Anthropic / Bedrock / Vertex）与隐式最长前缀缓存（DeepSeek 等）都受益于更长的一致前缀。改动本身零成本。

不做：SessionStart 重排（那是会话前缀、一次注入，收益是另一回事，另立项）；channel spawn 组装不动（supervisor 层，另立项）。

### 2 号：会话去重（PostToolUse 换引用，Claude Code 首发）

机制已核 Claude Code 官方 hooks 文档（2026-11-19）：PostToolUse 可返回 `hookSpecificOutput.updatedToolOutput` 替换工具结果，值须匹配工具输出形状；Bash 是 `{stdout, stderr, interrupted, isImage}`；PostToolUseFailure 不能改写，失败原样过（正好，失败项本来就要让 agent 看到）。sqz 本体用的是 PreToolUse 改写命令（`sqz compress --cmd *`），那是另一条路，本批不用。本批只做去重半截，不做格式化那半截（格式化是 RTK 那档的活，别重复造）。

- 去重表：`.sunshine/.runtime/dedup-<session>.jsonl`，一行一条 `{seq, tool, key, hash}`，seq 为行内序号（截断后仍单调）；上限 1000 行，超了截断留最近（activity.go 同款追加 + 截断）。只记哈希，不存原文（原文已在会话上下文里）。
- 键：Bash = 去空白后的命令串；Read = file_path + offset + limit（缺省为空）。
- 命中条件（全满足才替换）：同一会话（无 session_id 不去重，跨会话去重是错的——新会话里模型没有原文）；同一键；输出哈希完全相同；输出不小于引用（太短的换引用反而更长）；Bash 要求成功（stderr 为空；tool_response 有 exitCode 字段时要求为 0）。
- 替换文本：`[Sunshine: 与本会话第 N 次工具调用的输出相同，已在上下文中，不再重复]`（N = 首次记录的 seq）。Bash 替换后 stdout 为该行、stderr 置空、interrupted / isImage 原样；Read 按实际 tool_response 形状替换，形状不符不替换、原样放过。
- 平台门：v1 只做 Claude（updatedToolOutput 是 Claude Code 的契约）；其它平台不替换（Codex 等的 PostToolUse 输出替换契约未核实，另立项）。
- 注册：Claude `templates/platforms/claude/settings.json` 的 PostToolUse matcher 加 Read（`Edit|Write|Bash|Read`），否则 Read 重读根本不触发 hook。Codex 不动。
- 并发：PostToolUse 对并行工具调用并发触发；追加用 O_APPEND 单行写，读-查-写竞态的失败模式是良性的（漏一次去重或重复记一条），不阻塞。
- 铁律：任何失败静默返回 nil，绝不阻断会话。

收益预期（实话）：sqz 的 24.7% 是它全套（格式化 + 去重）的累计数，不是本批的数；本批只做去重，收益集中在吵闹会话（重复命令输出、改后重读），安静会话省得少。量法：固定任务跑前后对比（待定）+ 去重表可计数命中次数。

残留风险：Read 的 tool_response 实际形状以真机为准（字符串或对象两种都接，形状不符时不替换、原样放过）；Codex 等平台的输出替换契约未核实，v1 不做。

### 验收

gofmt / build / vet 全过；全量测试 21 包全过；新单测（提示词顺序断言 + 去重命中 / 未命中 / 无会话 / 小输出 / 非 Claude 平台）；hook 端到端实测（subagent-context 提示词顺序 + post-tool-use 替换输出）；新二进制 grep 确认新文案在位。

发版：已随 v0.5.31 发布（见下节）。

## 窄版 RTK（PreToolUse 改写命令）

### 背景

用户 2026-11-19 定：省 token 是目标，5 号（计量 + 护栏）不是一回事——计量只给 visibility、护栏只防跑飞，正常任务一个 token 都不省。1、2 号已做完（注入顺序 + 会话去重，已随 v0.5.31 发布），剩下最大的一档是 RTK 式 PreToolUse 改写命令：独立基准每会话实省约 20-43%。用户点选“先做窄版 RTK”。

### 契约（2026-11-19 核 Claude Code 官方 hooks 文档）

- PreToolUse 输入：`tool_name`、`tool_input`、`tool_use_id`；Bash 的 tool_input 字段是 `command`（string）、`description`（可选）、`timeout`（可选，毫秒）、`run_in_background`（可选）。
- 输出走 `hookSpecificOutput`：`updatedInput` 替换整个 tool_input 对象——必须把原字段原样带上、只改 `command`，漏掉 description/timeout 它们就没了；`permissionDecision` 与 updatedInput 搭配：`"allow"` 跳过权限提示（deny/ask 规则仍照常评估，always_ask 的工具与网络路径读不受影响），`"ask"` 把改写后的命令拿给用户确认，`"defer"` 只用于 `-p` 非交互模式；**不带 permissionDecision 时 auto 模式可能直接拒绝改写后的调用**（服务端审的是原输入，不是改写后的），所以必须带。
- 多个 PreToolUse hook 意见不一致时优先级 deny > defer > ask > allow。
- 2.1.300 起 updatedInput 按工具 schema 校验：未知字段丢弃、不合法的输入直接拒绝调用——改写后的 command 必须是合法 Bash 输入。
- `additionalContext` 是附加给 Claude 的说明文字，随工具结果一起进上下文。

### 设计（窄版，v1 只做 Claude）

新增 `sunshine hook pre-tool-use`（`internal/hook/pre_tool_use.go`）+ `sunshine hook compress`（`internal/hook/compress.go`，复用 sqz 的包一层模式：`sqz compress --cmd *`），注册进 Claude `templates/platforms/claude/settings.json` 的 PreToolUse，matcher `Bash`，timeout 10 秒。Codex 的 PreToolUse 改写契约未核实、DSH 有无对应事件未核实，都另立项，本批不动。

改写只做白名单，命中才返回 JSON（不命中时无输出、退出 0，等于没意见）；任何解析失败、平台不是 Claude、工具不是 Bash、命令为空时都原样放过（fail open）。

#### 直接加 flag（不改命令结构，不动退出码）

- `git log`（无 `--oneline` / `--format` / `-p` / `--patch` / `--stat` / `--name-only` / `--name-status` / `--graph` 时）→ 在 `log` 后插 `--oneline`。
- `git diff`（无 `--stat` / `--name-only` / `--name-status` / `--numstat` / `-p` / `--patch` / `--check` 时）→ 在 `diff` 后插 `--stat`。agent 要看某文件再跑 `git diff -- <文件>`。

#### 包一层（测试 / lint / 构建类吵闹命令）

白名单命令头：`go test` / `go vet` / `go build`、`npm test` / `npm run test` / `npx tsc` / `tsc` / `eslint`、`cargo test` / `cargo clippy`、`pytest` / `python -m pytest`、`make test` / `make check`。命中时把该段命令改写为 `sunshine hook compress --timeout <原 timeout> -- '<原命令>'`（timeout 原没有就不带；二进制用 `os.Executable()` 的绝对路径，不依赖 PATH）。

`compress` 用 `bash -c` 跑原命令（stdout/stderr 分开抓），压缩后分别写回自己的 stdout/stderr，**以原命令的退出码退出**——退出码是底线，绝不能像 `| head` 那样把失败掩盖成成功。

压缩规则（每条流独立算）：
- ≤200 行且 ≤12000 字节：原样通过，不加标记。
- 超了：保留首 10 行 + 失败特征行 + 末 80 行，总保留 ≤200 行；连续重复行折叠（同一行连续出现记 `（×N）`）；行首加一条截断说明，注明原行数、保留行数与绕过办法（命令后加 `| cat` 即不包一层）。

失败特征行（大小写不敏感子串）：FAIL / `--- FAIL` / FAILED / Error / `error:` / panic / exception / ✗ / ✘ / `not ok`。

#### 不改写的情形

- 命令含管道 `|`：agent 已在自己管输出，不插手（这也是绕过办法）。
- 复合命令（`&&` / `||` / `;` / 换行）按段切分：用引号感知的切分器（跟踪单双引号与反斜杠转义），每段独立判断是否命中白名单、命中只改写该段；引号不平衡时整条命令不动。
- 已含 `sunshine hook compress`：不重复包（幂等）。
- `git log` / `git diff` 已带等价 flag：不动。
- 后台命令（`run_in_background: true`）照常改写：压缩后的输出进 BashOutput 文件，机制不变。

#### 权限决定

改写时返回 `permissionDecision: "allow"` + 全量 `updatedInput` + 一句 `additionalContext`（说明改了什么，例如“Sunshine 已将 git log 改为 --oneline 以节省 token”）。理由：改写是原命令的严格收窄——同一条命令、更少的输出，不增加任何新能力；deny/ask 规则仍照常评估；用户本来就要跑原命令。`allow` 跳过的只是对改写后命令的重复提示，不是绕过规则。

### 风险与实话

- 20-43% 是 RTK 那套在别人会话上的独立基准数，不是 Sunshine 的。v0.5.30 的模板措辞已经让 agent 自觉做同样的事（git log --oneline、只看失败项、截断），hook 挣的是 agent 没照做的那部分——这部分有多大，**没量过**，本批固定任务跑前后对比就是来量这个的。
- 改写是唯一会动 agent 实际执行命令的一档，改错的风险真实存在：白名单 + 引号感知切分 + 失败原样放过是三道防线；`git diff → --stat` 是有意的语义收窄（agent 要看细节再跑单文件），不是无损改写，写进文档。
- wrapper 会缓冲整段输出再一次性打印，长测试跑完前界面看不到实时输出（模型侧本来就是等跑完才看到结果，无影响）。

### 验收

gofmt / build / vet 全过；全量测试 21 包全过；新单测：两条 flag 改写（命中 / 已带 flag 不动 / 管道不动 / 引号不平衡不动 / 复合命令只改命中段）、包一层（白名单命中 / 已包过不动 / 含管道不动 / 非 Claude 不动）、compress（小输出原样过、长输出截断、失败行保留、重复行折叠、退出码原样、stderr 分开）；端到端实测：新二进制跑 pre-tool-use（git log / git diff / go test 三例）+ compress 真跑；固定任务前后对比（同一组命令：原命令输出字节 vs 改写后输出字节，退出码一致）；新二进制 grep 确认新文案在位。

### 实测发现

- cobra 会把 `--` 从位置参数里剥掉：`sunshine hook compress -- 'cmd'` 到 Run 时 args 只有 `['cmd']`，第一版按“args 里有 `--`”解析，包一层后静默退出 0、无输出；单测直调 RunCompress 时 `--` 还在所以全过，是端到端实测（固定任务前后对比）抓到的。修复：`--timeout` 注册成 hookCmd 的 cobra flag，RunCompress 只收位置参数 + timeoutMs。
- 固定任务前后对比（25 提交仓库 + 120 行未提交 diff + 458 行 go test -v 输出）：git log 3340→441 字节（-86.8%）、git diff 8850→115 字节（-98.7%）、go test -v 30517→4660 字节（-84.7%，FAIL 行保留、退出码 1 不变）、go test（小输出）2157→2157 原样过、go vet 0→0。退出码全部一致。

发版：已随 v0.5.31 发布（见下节）。

## v0.5.31 发布记录（2026-11-19，已发布）

- 内容：注入顺序、会话去重与窄版 RTK（省 token 三档）。1 号：四个子代理提示词 builder + Codex 上下文纯重排（稳定在前、易变在后），字节不变；2 号：PostToolUse 会话去重（`.sunshine/.runtime/dedup-<session>.jsonl`，只记哈希，上限 1000 行），Claude 首发，PostToolUse matcher 加 Read；窄版 RTK：PreToolUse 改写 Bash 命令（git log 加 --oneline、git diff 加 --stat、测试/lint/构建类包一层 `sunshine hook compress` 压缩输出，退出码原样），Claude 首发。
- 主提交：本地 7ffe7abed9d92af5d3107a707fb8828d4c7f146f → 远端 983378313c194167e66a4a51cae880614ab6ee51，tree d4b006b23be420e9a8a5ce8c745b54ab46795fab，父提交 081acfefa5ad99d5e10548126e8f1f34f1e7e25d，作者 linrong007。12 个文件，+1701/-37。
- tag：v0.5.31，annotated tag，tag 对象 c9b02b3c072aa2b8bd65ee9ef22a9a617a01a734 → 9833783。
- CI 与发布：发布 workflow run 44553231021、CI（tag）run 44553230855、CI（main）run 44553165161 均 completed success。
- Release：id 451527893，非草稿、非预发布，6 个资产在位（checksums.txt + sunshine_0.5.31_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.31（linux/amd64），`sunshine version` 报 0.5.31；darwin_arm64 包 checksum cc3084a36691cf598ed3b204a23103974a0667d0c7d49db01f6a364f69841ffa、linux_amd64 包 checksum f5dad5b2ca62b7eba066afefbbb80e5196cb4ce506797f176bf1c83729a87e59，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.31，最新 0.5.31），无需升级。”）。发布包二进制 grep 确认“已包一层压缩输出”1 处、“pre-tool-use”5 处、“输出过长”1 处在位。

## squeez 增量机制（模糊去重 + Read/Grep 限流 + compact 后重注）

### 背景

用户 2026-11-19 点选“先做 squeez 增量机制”。第四轮调研结论：没有碾压已做三档（注入顺序、会话去重、窄版 RTK）的新机制，增量在三件可拆开做的事。squeez（https://github.com/claudioemmanuel/squeez，★237，Apache 2.0，Rust 单二进制），源码读于 v1.49.3（HEAD f2c6184），clone 在 `~/workspace/scratch/squeez-src`。

注意 squeez 有两条去重线不要混：PostToolUse 跨调用去重（`context/redundancy.rs`）与 Bash wrap 管道内的行级 dedup / log-template（`strategies/`）。本批三件分属两条线：模糊去重走 PostToolUse、log-template 折叠与限流注入走 PreToolUse/压缩管道、compact 后重注走 SessionStart。

### 数字实话

- 91.2% 是 46 场景 × 5 次合成 benchmark（chars/4 计，229,443 → 20,249 token）；cl100k_base 真实 BPE 重算 22 个 fixture 得 83.5%（112,557 → 18,590）；**无真实会话端到端对照**。
- v1.48.25–v1.49.1 统计 bug：finalize 把整小时共享会话日志的所有事件算进一个会话，单会话节省约高估 8 倍（已修，issue #284，commit 9f2377a）。
- 无法压缩：Agent/Task 返回值、skills 与斜杠命令文件、用户顶层 prompt（README "What squeez CANNOT compress" 一节）。
- 支持列表无 DSH、Cursor；本批 v1 只做 Claude。

### 设计

#### 1. 模糊去重（PostToolUse，复用 dedup.go）

- 比较单位：整段工具输出（全部行 join 成一个字符串）；归一化只有空白折叠（连续空白等价），不去时间戳、不做语义归一化。
- 素描：空白 token 的连续三元组（trigram），每组 FNV-1a-64 哈希，取最小的 96 个（bottom-k MinHash，SHINGLE_K=96），排序去重；Jaccard 在两个有序去重集合上算。Go 用标准库 `hash/fnv.New64a()`，无外部依赖。
- 记录：dedupRecord 加可选 `sketch` 字段（≤96 个 u64，只在输出 ≥6 行时记；只记素描不存原文——sqz 旧版把短凭证行存进本地库的安全坑不要踩）。精确去重逻辑与现有字段不动；旧行无 sketch，模糊查找跳过。
- 命中条件（全满足）：同会话 + 同键（dedupKey 现状）+ 输出 ≥6 行（MIN_LINES_FUZZY=6）+ 长度比 min/max ≥ 0.80（LENGTH_RATIO_GUARD）+ trigram Jaccard ≥ 0.85（similarity_threshold 默认值）+ 只查最近 16 次调用（RECENT_WINDOW=16）+ 净省 ≥24 token（约 96 字符，net_win_min_tokens=24）。
- 替换文本：`[Sunshine: 与本会话第 N 次工具调用的输出约 X% 相似，已在上下文中，不再重复，如需确认请重读]`（X 为 Jaccard 百分比取整；N 为首次记录的 seq）。
- 安全护栏（squeez A2 等价实现）：Read 模糊命中须文件 mtime 与记录时一致（记录时 stat 文件记 mtime，命中时再 stat 比对；取不到 mtime 时 Read 只允许精确命中）；Bash 不受此限（同命令键重跑）；MCP 工具结果不做模糊去重。
- 误判兜底：相似度不够高不替换、任何解析失败原样放过、输出比引用短不换；精确命中路径先跑，模糊只在精确未命中时跑。

#### 2. log-template 折叠（compress.go，Bash 输出管道内）

- 位置：compress.go 压缩路径，在现有首 10 + 失败行 + 末 80 行截断之前先折叠（squeez 管道顺序 smart_filter → dedup → log_template → relevance-truncation）；失败特征行永不折叠（原样保留）。
- 推导：整行按空白分词，逐 token 判变量；`key=value` 只把值换成 `{}`、键保留；变量四类：①纯数字形（仅数字 + `.:-/T+` 分隔符，如 12:30:45、2026-06-21T10:30:45、192.168.0.1、3.14）；②数字+单位（1ms、200ms、5s、3.2KB、50%、1.5x，后缀 ≤4 字母/%）；③hex/UUID（长度 ≥8、≥2 个数字、全 hex 或 `-`）；④复合 ID（带数字且含 `-`/`_` 分隔符，如 req-1；版本号 v1.24.0、1.2.3 因无 `-`/`_` 被保留）。纯单词（含 ERROR/INFO）是结构，不替换。
- 阈值：模板出现次数 ≥3 才折叠（log_template_min=3）；只有含 `{}` 槽位的模板才折叠，无变量的逐字重复行留给现有连续重复折叠。
- 标记：`[×N] <模板>  (e.g. <首行原文样本>)`，样本为首次出现行原文，trim 后超 100 字符截断加 `…`；折叠行发在首次出现位置保序，其余出现直接丢弃；其余 N−1 行的变量取值丢弃。
- 范围：只在 Bash 输出管道（compress.go）内做，不碰 Read/Grep 结果（squeez 同款限定）。

#### 3. Read/Grep 限流注入（PreToolUse）

- Read 缺 limit 时注入 `limit: 800`；Grep 缺 head_limit 时注入 `head_limit: 100`（read_max_lines=800、grep_max_results=100）；只补不存在的键，永不覆盖调用方显式值，不设上限（传 5000 也原样放过）；Glob 不注入（squeez 名义上有、实际返回 None）。（2026-11-20 变更：Read 默认值由 300 改为 800——用户担心 300 行读漏长文件后文，改大到 800 折中；已随 v0.5.33 发布，见文末 v0.5.33 发布记录。）
- 方式：改 tool_input 字段、返回全量 updatedInput + permissionDecision "allow"，口径与 RTK 一致；任何解析失败原样放过。
- 注册：Claude settings.json 的 PreToolUse matcher 由 `Bash` 改为 `Bash|Read|Grep`；RunPreToolUse 按 tool_name 分发（Bash 走现有改写，Read/Grep 走限流注入）。

#### 4. compact 后状态重注（SessionStart，source=compact 才注）

- 触发判断：SessionStart 输入 `source` 字段（官方文档 5 个取值：startup 新会话、resume、clear、compact、fork）；只在 `source == "compact"` 时重注，其余一律不注；取不到 source 时静默不注。
- 指针内容（≤4000 字符，MAX_SUMMARY_CHARS=4000）：当前任务（会话指针文件里的 current_task + 任务标题/状态）+ 下一步（任务状态与最近 activity.jsonl 事件推导）+ 关键约束（取自任务 prd.md 的验收标准节，最多 3 条，每条 ≤80 字符；取不到就空着）；不在指针里贴全文、不贴 jsonl 原文。
- 指针不落盘：重注时直接从磁盘读现有状态拼（会话指针、task.json、activity.jsonl），不新增写盘文件、不新增常驻进程；复用 session_start.go 已有的任务解析 helper，不另造一套。
- 与现有注入不重复：指针以 `<compact-resume>` 块追加在现有 SessionStart 上下文之后，不改现有 workflow 概览/当前任务/spec 地图。
- clear 的处理：source=clear 时现有注入（workflow 概览 + 当前任务 + spec 地图）已足够重定向，不另加指针块（与任务书“compact/clear 后才重注”措辞的偏差：clear 是用户主动 fresh start，现有注入已含当前任务，再贴指针是重复；如主会话要改，改这一句）。
- compact 后去重表失效：SessionStart 在 source=compact 时删本会话的 dedup-<session>.jsonl（compact 前调用的输出模型已看不见，旧引用会指到不存在的内容；删表后精确去重重建，代价是 compact 后第一次重读不省；删失败静默）。

### 不做项

- Bash 规则超集（squeez 41 条规则）：窄版 RTK 已覆盖本批需要的，规则数不扩。
- agent prompt 超阈值时压缩：PreToolUse 拦截 Agent/Task 的 prompt，token 估算（chars/4）超 agent_prompt_max_tokens（默认 2000）时先跑 markdown 压缩，仍超则头部截断到预算并追加截断提示；另有并发派生上限（默认 5 个在途，超限直接 deny）。
- SessionStart 注入上一会话摘要：squeez init 把上一会话终结写 `~/.claude/squeez/memory/` 的 Summary，新会话 banner 印最近 3 条摘要行 + 最新一条最多 3 条 Next steps。
- 可逆取回（内容寻址 blob）：大输出压缩前原文存 `~/.claude/squeez/blobs/<16 位 hex FNV-1a 哈希>`，凭 key 调 squeez_retrieve 取回，存前有敏感内容护栏（含疑似凭证则拒存、改发原文）。
- 其它平台（DSH、Cursor 等）不动；MCP 工具结果不做模糊去重（squeez 同款禁令）。

### 风险与实话

- squeez 的 91.2%/83.5% 是合成 benchmark，无真实会话端到端对照；本批收益以固定任务前后对比 + 模糊命中计数实测为准，不承诺整会话百分比。
- 模糊匹配误判风险真实存在：阈值 0.85 + 长度比 0.80 + 净省 24 token 三道护栏 + Read 的 mtime 护栏；误判最坏情况是模型看到引用后需重读（替换文本里写明“如需确认请重读”），不丢数据（原文仍在磁盘文件里，Bash 输出可重跑）。
- log-template 折叠丢弃 N−1 行的变量取值：只折叠 ≥3 次出现的模板，且失败行永不折叠；折叠后保留一条原文样本。
- compact 检测不到（source 字段缺失或平台不传）时静默不注，不报错、不补注。
- dedup 表加 sketch 字段后单行变长（≤96 个 u64，约 2KB/行）；表上限仍 1000 行，模糊只查最近 16 条，文件体积可控；旧行无 sketch，模糊查找跳过。

### 验收

gofmt / build / vet 全过；全量测试 21 包全过；新单测：模糊命中/未命中/阈值边界（0.85 上下）/长度比护栏/净省门槛/Read mtime 变化时只精确命中；log-template 折叠（四类变量/key=value/≥3 才折/失败行不折/样本截断）；限流注入（Read 缺 limit 注入 300、已有 limit 不动、Grep 缺 head_limit 注入 100、已有不动、非 Claude 不动、Glob 不注入）；重注（source=compact 注入在位、startup/resume/clear/fork 不注入、source 缺失不注入、指针 ≤4000 字符、compact 后 dedup 表已清）；hook 端到端实测（pre-tool-use stdin 真实输入、post-tool-use 模糊命中、session-start compact 后重注在位）；固定任务前后对比（同一组命令/读取：改写前输出字节 vs 改写后输出字节，退出码一致；模糊去重单独给命中前后字节数）；新二进制 grep 确认新文案在位。

### 发版前 bug 检查修复（2026-11-20）

发版前扫了一遍 squeez 三件与相关 hook 改动：发版前必须修的未发现；修了 2 处建议修：
- 验收标准小数行被切错（compact_resume.go）：`1.5 倍性能提升` 被当成序号 `1.` 切成 `- 5 倍性能提升`；修为小数点后跟数字的不算序号（`1. ` 是序号，`1.5` 是小数），`、` 不用改。新增 TestCompactCriterionLine_Decimal，端到端复核指针块已无错切。
- 模糊去重只验最佳候选（dedup_fuzzy.go）：最佳候选长度比不足时直接返回未命中，不试次优；改为按 Jaccard 从高到低依次验长度比与净省，首个全过者命中；顺带把长度随候选一起带上，不再按 seq 二次查表（并发下 seq 可能重复）。新增 TestFuzzy_SecondCandidate（手工素描定 Jaccard：最佳 0.905 长度比不足，次优 0.852 命中）。
- 去重表无锁读-改-写并发竞态：失败模式良性（漏一次去重、compact 后一条旧引用复活），本批之前就存在，接受为已知限制，不加文件锁（hook 绝不能阻塞）。
- 另 8 条小问题（重复计算、折叠顺序交互、Read 限流截断是否标明等）暂不修；全量测试 21 包全过，hook 包 128 单测全过。


发版：已随 v0.5.32 发布（见文末 v0.5.32 发布记录）。

### 发版前 bug 检查（全仓，2026-11-20）

用户 2026-11-20 要求发版前再扫 bug：squeez 专查 + 全仓四队（hook / CLI 与任务系统 / 平台与 registry / serve 与 channel 与 mem）。squeez 三件结果见上一节；全仓结论：发版前必须修 12 条，建议修 33 条，可忽略的小问题约 46 条。只做检查，未改文件；squeez 的 2 处修复已另行提交推送（c581170）。用户 2026-11-20 定 12 条全修，已修完并随 v0.5.32 发布（修复记录见本节末）。

#### 发版前必须修（12 条）

- A1 spec 索引同步非原子 read-modify-write（specindex/index.go:177,228）：两次快速 Edit 或 SessionStart SyncDir 并发，hook 基于旧内容写回，覆盖 agent 改动，spec 内容静默丢失。修：临时文件 + rename 原子替换，或写盘前比对 mtime/内容重算，或同文件加锁。
- A2 单个坏 spec 文件 abort 整个规范地图/全量同步（specindex/applicability.go:127-170，index.go:228-237）：spec/ 下任一 .md 是悬空软链接或权限 000，SessionStart 规范地图整个为空，SyncDir 中断。修：单文件读失败跳过继续，最后汇总。
- A3 任务指针 `..` 逃出 tasks 目录仍被当成当前任务（hook/hook.go:356-391，active_task.go:284）：会话指针 current_task 写 `../spec`，session-start 输出 `Current task: .sunshine/spec` 未判 STALE，提示注入可利用。修：收容收紧到 `.sunshine/tasks/`。
- B1 add-subtask 可建父子环与自环（task/lifecycle.go:1012,795，context.go:272）：环内任务文本 list/context 报“没有任务”；环挂根下后 list/context 无限递归，20 秒写 396MB 输出直至栈溢出。修：建链前拒绝 parent==child 与成环；printTask/printTree 加 visited 集合。
- B2 归档任务仍可被 start/add-subtask/create --parent/set-* 操作（task/lifecycle.go:505,1012,123,909）：按归档路径传参时成功，归档任务复活为当前任务、归档文件被改写、父子图悬空。修：所有写任务命令统一加 IsWithinTasksDir 守卫（与 Archive/Rename 一致）；Start 另加状态校验。
- B3 init -f 无确认、无备份直接 RemoveAll(.sunshine/)（scaffold/scaffold.go:101）：已初始化项目一次误操作丢全部任务、journal、memory，且中途失败旧数据已丢。修：交互确认并明示，或先备份到 .sunshine/.backup-<时间戳>/，或 -f 只重建骨架保留 tasks/workspace/memory。
- C1 增量 init 的 registry 安装失败回退清空手写规范（cli/registry.go:60-70，scaffold/registry.go:24-56）：安装探测成功后失败时，内置空白模板无条件覆盖手写 system.md/frontend.md/backend.md，内容永久丢失，还记了空白模板哈希。修：WriteDefaultSpec 只写不存在的文件，或回退前备份 spec/。
- C2 GitLab 子群组仓库解析错误（registry/parse.go:303-316，workflow/registry.go:293-306）：三段以上路径（group/sub/repo）被切成 repo="group/sub"，拼出错误 GitURL。修：两处解析器共用“仓库路径可多段”逻辑。
- C3 公开 GitLab 浏览 URL（/-/tree/）解析错误（registry/parse.go:48-66，workflow/registry.go:118-140）：`/-/tree/main/marketplace` 被当成 subdir，RawBaseURL 走偏。修：tree 正则加 (?:/-)?，两处同步。
- D1 activity.jsonl 并发截断丢事件（hook/activity.go:49-97）：文件超 1000 行后并发 subagent-stop，truncate 读全文件→写 .tmp→Rename，窗口期内追加丢失；实测 200 并发丢 100 条。修：append+truncate 临界区加锁文件，或取消原地截断（只追加，读侧取尾部）。
- D2 CLI spawn 的 --file "**" glob 静默丢深层文件（cli/channel_spawn.go:305-380，channel/supervisor/context.go:39-130）：CLI 用 filepath.Glob 预展开（不支持 **），supervisor 永远看不到原始 pattern；另 CLI 预组装读盘失败即致 spawn 整体失败（supervisor 本是 warn+跳过），且 CLI 组装的 SystemPrompt 在生产路径被丢弃。修：CLI 不预展开、不预读，原始 pattern 原样传 supervisor。
- D3 DSH 会话列表任一坏行导致整个会话被跳过（mem/adapters/dsh.go:56-64,120-145）：session.v4.jsonl.zstd 解压后一行无法解析，整个会话从 mem list/search 消失。修：只有打不开/解压失败才跳过，坏行只记警告。

#### 建议修（33 条，简列）

hook（A5–A15）：A5 spec 索引同步沿软链接写到 spec 外（post_tool_use.go:78，index.go:177）；A6 PostToolUse Bash 分支每次全量 SyncDir（post_tool_use.go:87）；A7 shell-session 身份判定只查顶层字段（shell_session.go:66）；A8 ResolveProjectRootMulti 不查环境变量（hook.go:273）；A9 statusline 只用进程 cwd 找根（statusline.go:29）；A10 isLoneLineTag 只验标签后半行（workflow_state.go:194）；A11 materializeDirectory 超 20 文件静默丢弃（subagent.go:564）；A12 compress 输出无界缓冲、超时不彻底（compress.go:74-76）；A13 handleCodexSubagentStart 找根不查环境变量（subagent.go:170）；A14 spec_scope 写单个包名时静默全量扫描（session_start.go:194）；A15 ReadActivityEvents 单行超限丢弃其后全部事件（activity.go:102-116）。
CLI 与任务系统（B4–B13）：B4 .current-task 与会话文件写操作全程无锁（task/active.go）；B5 create --force 换父后旧父 children 残留（lifecycle.go:123）；B6 Archive 先置 completed 再解子女链接（lifecycle.go:1771）；B7 Archive 缺少 create 文案承诺的空标题/空描述门禁（lifecycle.go:1771）；B8 ParseRecord 要求 24 字段全必填（record.go:247）；B9 开发者名未净化拼进工作区路径（hook/hook.go:403）；B10 task list --status 按原始 status 过滤与 displayStatus 不一致（lifecycle.go:746,800）；B11 spec index <文件> 不校验须在项目内（cli/spec.go）；B12 upgrade verifyNewVersion 无超时、回滚非原子（upgrade.go:668）；B13 Start 不校验任务状态（lifecycle.go:505）。
平台与 registry（C4–C10）：C4 来源带 .git 又带 #ref 拼出双重 .git 后缀（registry/parse.go:166-180）；C5 spec 模板 path 指向单个文件时 .sunshine/spec 被写成普通文件（registry/download.go:357-372）；C6 append 策略同名文件静默跳过却照常计数记哈希（registry/download.go:74-93）；C7 --overwrite“先备后换”不是原子（download.go:111-141）；C8 适用标签按文件名推断不认中文名（specindex/applicability.go:36-46）；C9 registry:/spec: 行带行尾注释时读不到、写出重复节（config/registry.go:60-68,150-222）；C10 外部市场模板内容不经处理成为 agent 指令上下文（设计级缺口，registry/download.go）。
serve/channel/mem（D4–D9）：D4 Claude 适配器只认 string 的 user 内容、只认数组的 assistant 内容（mem/adapters/claude.go:143-190）；D5 serve 每轮询全量扫描 mem 会话（serve/handlers_misc.go，app.js:619,627）；D6 serve 三处每轮全量读 channel 事件且一处重复读（handlers_channels.go，handlers_misc.go）；D7 channel 锁陈旧窃取无年龄兜底、supervisor done/error 丢失无重试（channel/store/lock.go:124-151，supervisor/run.go:368-390）；D8 BuildChildIndex 的 parent_id 成环死循环耗尽内存（mem/context.go:79-100）；D9 serve /api/health 在配了 registry.spec 时触发出站拉取（serve/handlers_misc.go）。

#### 可忽略的小问题（约 46 条）
hook 19 条、CLI 与任务系统 7 条、平台与 registry 11 条、serve/channel/mem 9 条，详见本次检查汇报（会话记录）。另核查通过两点：hook fail-open 铁律未发现违反；serve 只读口径属实（无写文件调用、路由全 GET、只绑 127.0.0.1）。

#### 修复记录（2026-11-20，四队并行修，改动留工作树待发版）

- A1：SyncFile 写盘改「同目录临时文件（os.CreateTemp）+ Sync + rename」原子替换并保留原文件权限；另加写前内容比对（读盘后文件已被改则重读重算，最多 5 次，仍在变报错放弃）。未用 mtime 退路（同目录 rename 无跨设备约束）。改 internal/specindex/index.go；测试 TestSyncFile_AtomicNoOverwrite。
- A2：CollectSpecMap 的 WalkDir 回调与单文件 ReadFile 失败改为跳过继续（仅 specDir 根目录读不了才返回真错误）；SyncDir 单文件失败记下继续、最后汇总返回带跳过清单的错误；连带 SpecFiles 的 walk 回调同样跳过坏条目（否则悬空链接会在列文件阶段先中断 SyncDir）。改 internal/specindex/applicability.go、index.go；测试 TestCollectSpecMap_SkipsBadFile、TestSyncDir_SkipsBadFile（用悬空软链接触发，沙箱 root 下权限 000 仍可读）。
- A3：ResolveTaskRef 收容从「仓库内」收紧到「.sunshine/tasks/ 内（含 archive/）」；.sunshine 为符号链接时按其实址的 tasks/ 判定并映射回词法路径（#567 例外保留，真实符号链接项目实测通过）。active_task.go 未改：activeFromRef 本就以 ResolveTaskRef=="" 判 Stale，收紧后自动生效。改 internal/hook/hook.go；测试 TestResolveTaskRef_EscapeTasksRejected、TestSessionStart_EscapeTaskPointerIsStale。复核：指针 ../spec 时修复前输出 Current task: .sunshine/spec 并把 spec 的假 task.json/prd.md 当任务注入，修复后判 STALE POINTER。
- B1：linkSubtask 建链前拒绝自环、沿新父的父链上溯拒绝成环（报错写明祖先关系）；List 的 printTask 与 Context 的 printTree 各加 visited 集合（回边打印「已访问，跳过」警告），并加兜底遍：无根可达的环内任务逐个补打，不再被藏掉。改 internal/task/lifecycle.go、context.go；测试 TestLinkSubtask_SelfCycleRejected、TestLinkSubtask_CycleRejected、TestListContext_HandmadeCycle_B1。复核：add-subtask B A 与 A A 均 exit 1；手改自环后 list/context 立即返回（241 字节，修复前 20 秒写 396MB）。
- B2：新增统一守卫 requireActiveTaskDir（基于既有 IsWithinTasksDir，与 Archive/Rename 同口径），覆盖 Start、linkSubtask（父子两侧）、Create --parent、setField、SetMeta、AddContext；Start 另加状态校验，非 planning/in_progress 拒绝并报错说明当前状态。口径落实：Validate 全程只读不写盘，按「只给写路径加」未加守卫；Start 状态校验仅在 task.json 可读时生效，缺失/损坏仍走原有告警路径。改 internal/task/lifecycle.go；测试 TestStart_ArchivedTaskRejected、TestStart_CompletedStatusRejected、TestWriteOps_ArchivedTaskRejected。复核：按归档路径 start/set-branch 均 exit 1，.current-task 未被写入。
- B3：Force 分支删前先把旧 .sunshine/ 整体 os.Rename 备份到同级 .sunshine.backup-YYYYMMDD-HHMMSS（同秒冲突加 -1、-2 后缀）并打印备份路径，备份失败则中止且旧数据不动；保持非交互，无确认提问；-f 帮助文案改写明会先备份。改 internal/scaffold/scaffold.go、internal/cli/init.go；测试 TestInitForce_BackupsExisting、TestInitForceHelpMentionsBackup_B3。复核：init -f 后备份目录含原任务与归档任务的 task.json，新 .sunshine/ 正常重建。
- C1：WriteDefaultSpec 改成只补缺失——已存在文件原样保留、不覆盖、且不进返回清单（调用方据返回值记哈希，手写内容不再被记成模板哈希的问题随之消失）；全新 init 照常全写。internal/cli/registry.go 未改（其回退分支原本就拿返回值记哈希）；回退不写 registry.spec 的既有行为保持。改 internal/scaffold/registry.go；测试 TestWriteDefaultSpec_KeepsExisting、TestRegistryInstallFailureFallback_C1_KeepsExisting（端到端：假市场模板 path 不存在触发回退，三份 MUST-STAY 手写 spec 逐字保留）。
- C2：GitLab 裸 HTTPS 路径（公开 gitlab.com 与自建 host）全段视为仓库路径；tree 形式按 /tree/ 或 /-/tree/ 之前的段定仓库、其后为 subdir；两包同构实现，未抽共用函数（两包结构体/错误类型不同，且 workflow 不能反向依赖 registry，按既有惯例各实现一份）。**与任务书的出入**：子群组「全段为仓库」只施于 GitLab 裸 HTTPS 与 tree 形式；giget 前缀（gitlab:...）与 SSH 形式仍保持两段式——这两类自带显式 subdir 语义且被既有单测钉住（如 ssh://git@localhost:2222/git/registry/marketplace → repo=git/registry、subdir=marketplace），全改会挂单测且 subdir 无从表达；任务书实测的两个出错例子都是 HTTPS 形式，已全部修复。改 internal/registry/parse.go、internal/workflow/registry.go；测试两包各一份 TestParseGitLabSubgroup。复核：git.corp.com/group/sub/repo 修复前 GitURL=.../group/sub.git，修复后 Repo=group/sub/repo、GitURL=.../group/sub/repo.git。
- C3：两处 tree 正则（registry 的 httpsTreeRe、workflow 的 githubTreeRe）都加 (?:/-)?，与 gitlabTreeRe 一致；附带 GitHub 的 /-/tree/ 也随同一正则一并修好（已加断言）。改同 C2 两文件；测试两包各一份 TestNormalizeGitLabDashTree。复核：gitlab.com/org/repo/-/tree/main/marketplace 修复后 Repo=org/repo、Subdir=marketplace、Ref=main，与自建 GitLab 行为一致。
- D1：AppendActivityEvent 的 append+truncate 整个临界区加锁（进程内 sync.Mutex + 跨进程 O_EXCL 锁文件、内写 pid、只删自己 pid 的锁、陈旧锁窃取），拿不到锁时 fail-open 只追加不截断；截断 tmp 名加 pid 后缀。**与任务书的出入**：锁等待上限取 2 秒（重试间隔 10ms）而非几十毫秒——几十毫秒装不下 200 并发的排队（每临界区含一次读/重写 1000 行），会大量走 fail-open 反而可能丢；2 秒仍属短超时且超时即 fail-open。陈旧判定用锁文件年龄 >5s（不可解析内容 >1s）而非 pid 存活探测（hook 包无现成 pidAlive，加平台文件会扩范围）。改 internal/hook/activity.go；测试 TestAppendActivityEvent_ConcurrentNoLoss。复核：进程内 200 并发 200/200 不丢；实编译二进制跨进程实测（预置 999 行、xargs -P 20 并发 200 次 subagent-stop）200/200、丢 0 条（修复前丢 100 条），.runtime/ 下无残留锁/tmp。
- D2：parseSpawnFlags 按后端真实分流条件 channelWorkerRuntime==nil 区分路径——生产路径不预展开、不预读，原始 --file/--jsonl pattern 原样写入 opts.Files/JSONLManifests 透传 supervisor；仅注入 runtime 的 API/测试路径仍走 assembleSpawnContext，其报错只作用于该路径。**与任务书的出入**：internal/channel/supervisor/context.go 与 internal/cli/channel_real_backend.go 未改代码——核查后 supervisor 的 expandGlob 本已支持 **、spawnViaSupervisor 本已转发 opts.Files/JSONLManifests，病根只在 CLI 预展开覆盖了原始值。改 internal/cli/channel_spawn.go；测试 TestSpawnFileGlob_DeepPreserved（src/**/*.go 展开含 src/a/b/deep.go；--file "src2/*" 命中目录时生产路径不报错、supervisor 跳过目录）。
- D3：dshSessionMeta 只在读不到 session 头（打不开/解压失败/无头）时返回 nil，坏行只带 warnings 返回会话；dshSessions 只按 info==nil 跳过，并把 warnings 汇总上抛（此前恒返回 nil warnings）；与 claude/codex/pi 的列表逻辑对齐。改 internal/mem/adapters/dsh.go；测试 TestDSHSessions_BadLineKeepsSession；既有 TestDshCorruptFile（坏 zstd 仍跳过）、TestDshNoSessionHeader（无头仍跳过）仍过。

全量验证（协调人统一跑）：gofmt -l 全仓为空；go build ./...、go vet ./... 通过；TMPDIR=/var/tmp go test ./... -count=1 全量 21 个有测试的包全过；上列回归测试 20 个点名复跑全过，D1 并发测试连跑 3 遍不 flaky；协调人另用实编译二进制独立复核 B1（成环/自环 exit 1）、B2（归档 start/set-branch exit 1）、B3（备份目录存在且含全部 task.json、新 .sunshine 已重建）。

## v0.5.32 发布记录（2026-11-20，已发布）

- 内容：squeez 增量机制（模糊去重、Read/Grep 限流注入、compact 后状态重注）+ 发版前 bug 检查修复三批：squeez 专查 2 处（验收标准小数行错切、模糊去重次优候选）、全仓 12 条必须修（spec 同步原子化、坏文件跳过、任务指针收容、子任务成环、归档守卫、init -f 先备份、init 回退保手写规范、GitLab 解析两处、activity 并发锁、spawn glob 透传、DSH 坏行不丢会话）。
- 主提交：squeez 本地 fee09cc129b6901a97bc70455293d7ea4a8411c5 → 远端 ab23f3d1786b766b695f1a5018d7e9c8d7bba42e，tree ce9bdaf537965e9d286f8a207653241b61e2ccaf，15 个文件 +1542/-25；squeez 修复本地 7e05dab6cc90c77177bb6d053b8400a58d1b88a0 → 远端 c581170a5e27ea0fc3a7c729a82fc63c0c3c4ed9，tree 8cfc13a568a8280f0ecb675d0081de608b9eabd5，5 个文件 +99/-33；全仓修复本地 865926e3f693f2b7128d88df3e3082a141bf7a13 → 远端 f90251f84cf2faf70ffd323053bece002b570884，tree 21d3a149aad1711d8a5968ed53313b5c71a83d8c，27 个文件 +1638/-55（tag 指向此提交）。
- tag：v0.5.32，annotated tag，tag 对象 37f3f4c9a24163ac4fba01f12fea482f9a256b94 → f90251f。
- CI 与发布：发布 workflow run 44642749412、CI（tag）run 44642749359、CI（main）run 44642710675 均 completed success。
- Release：id 452091364，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.32_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.32（linux/amd64），`sunshine version` 报 0.5.32；darwin_arm64 包 checksum 992b3c36b22c93fa753dbd7e8f40999be763d35777af4e2af8e5edac713e1f3f、linux_amd64 包 checksum a65c189a144b68d086545feb2f8b9cb013f8bb09b2ecb1a022004541ff412a74，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.32，最新 0.5.32），无需升级。”）；发布包二进制含模糊去重引用文案（“已在上下文中，不再重复”2 处）。
- 已知遗留：C2 的 SSH 与 giget 前缀形式 GitLab 子群组仍按两段式解析（被既有单测的 subdir 语义钉住，HTTPS 形式已全修）；②建议修 33 条、③小问题约 46 条未修（清单见上节）；本地验证时 TestChannelWaitAll 在连跑两遍全量下抖过一次（单独连跑 10 次与整包复跑均过，负载时序抖动，与 v0.5.6 的 CI 抖动同类）。

## v0.5.33 发布记录（2026-11-20，已发布）

- 内容：Read 默认限流 300 改 800（用户担心 300 行读漏长文件后文；只改默认值常量与提示文案，Grep head_limit 100 不变，只补缺失、永不覆盖显式值的口径不变）。
- 主提交：本地 89fea433bd2b467f08b033fe1aa1e6699fbf58e7 → 远端 d85795f197c12decb9b9a7c47d5afa722ce37cfc，tree 41d5ee3d2e4a3a665a0d555e9a303d34eeed3899，父提交 392b2b5656772a493688fc0bf77ba0a7df98767a，3 个文件 +10/-10。
- tag：v0.5.33，annotated tag，tag 对象 65fd08b7f70c32e30b1e0d07e0673b94add8552f → d85795f。
- CI 与发布：发布 workflow run 44647404957、CI（tag）run 44647404894、CI（main）run 44646343060 均 completed success。
- Release：id 452121201，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.33_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.33（linux/amd64），`sunshine version` 报 0.5.33；darwin_arm64 包 checksum 25d1721bfe0c5c0114a04ecea92156e77fe6725139dd64f11055a3d81ba6655a、linux_amd64 包 checksum af11b41c744da4b1c7ec969aba0772328756d67cb3a4ab88fdadac507d40821a，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.33，最新 0.5.33），无需升级。”）；发布包二进制 grep 确认 “limit=800” 1 处在位；发版前 pre-tool-use 端到端实测注入 limit=800、全量测试 21 包全过。

## 原型代理自检封顶（2026-11-20，已随 v0.5.34 发布）

- 起因：用户在 Mac 上用 dsh-context 插件抓到一个 DSH 原型子代理会话的真实分项（R-001 导航页，deepseek-v4.1-flash）：31 步、42 次工具调用、总流量 858.4k（累计口径，缓存命中 95.97%），其中大量步数花在自写 node 截图像素分析脚本（sx-verify 1–4 等）。核查确认原型代理模板里没有任何自检/截图要求——该行为是模型即兴发挥，模板既不要求也不禁止，全凭模型。
- 决定（用户 2026-11-20 批准）：给自检同时定下限与上限，写进原型代理定义，全平台统一。
- 改动：`templates/sunshine/agents/prototype.md`（channel 源）+ 17 份平台副本（claude/codebuddy/codex/copilot/cursor/gemini/grok/kimi/kiro/omp/opencode/pi/qoder/snow/trae/zcode/droid）共 18 个文件，在「原型要求」的“单页 500 行上限”条后各加一条：「渲染自检最多一轮：交付前可以截一张图看一眼渲染结果，只看图、不量像素，不许自写像素测量或分析脚本；看到大错（白屏、布局塌陷、明显溢出）改一次就停，改完仍有问题就带着问题交付、在汇报里写明，不再循环检查；没有截图条件时直接跳过，不为自检装工具或写脚本」。kiro 的 JSON 与 codex 的 TOML 按各自格式同文插入（JSON 除插入外零变化，已逐字核对）。纯模板措辞，不动代码。
- 验证：18 个文件各恰好 1 处新条；`go build` 过、全量测试 21 包全过；新二进制在临时项目 `init -y --claude --codex --kiro`，装出的三份原型代理文件均含新条。
- 发版：已随 v0.5.34 发布，见文末 v0.5.34 发布记录。

## v0.5.34 发布记录（2026-11-20，已发布）

- 内容：原型代理渲染自检封顶（「原型要求」加一条：最多一轮、只看图不量像素、不许自写像素测量/分析脚本、大错改一次就停、无截图条件跳过），源模板 + 17 份平台副本共 18 个模板文件；起因与决定见上「原型代理自检封顶」节。纯模板措辞，不动代码。
- 主提交：本地 222e49bc79f8550c3c67f196ed23c7e6792393a5 → 远端 5c8242d341d19d00d18f0259a9be6aa49d241a0a，tree ee93b25ff70498051e3e85d6540d020a248af015，父提交 74a1f5b863565e59403be1e3650aa55ec041fd89，19 个文件 +26/-1（含 PLAN.md 本节）。
- tag：v0.5.34，annotated tag，tag 对象 e87d7558812279feb42bf9710f1e308ce887cc8c → 5c8242d。
- CI 与发布：发布 workflow run 44661082024、CI（tag）run 44661081886、CI（main）run 44661049994 均 completed success。
- Release：id 452197015，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.34_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.34（linux/amd64），`sunshine version` 报 0.5.34；darwin_arm64 包 checksum 75288be8ef50fe2cca86999c9099186f35eef701b781affe8825a1ba1db18715、linux_amd64 包 checksum 70f94803a3b0041856b7fecd2e217fee5b4cca046084d768221d3197e78828bc，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.34，最新 0.5.34），无需升级。”）；发布包二进制 grep 确认 “渲染自检最多一轮” 在位；发版前全量测试 21 包全过、临时项目 init 实测 claude/codex/kiro 装出文件含新条。

## 需求边界三件套（2026-11-20，已实现，待发版）

- 起因：用户担心拆解出的需求边界不够细（上传允许什么类型/多大、配置是本地文件还是网络配置等），问细度在哪一阶段定、能不能做到。核查现状：细粒度澄清的主战场是设计阶段每条需求的澄清轮；拆解是机械转换、照单全收；子任务开工只处理 `[待定]`，中途发现缺失是兜底。brainstorm 八维清单是维度级不是条目级，具体项靠 agent 临场判断，会漏。用户又转来另一方案的产出（EARS 规格改写后汇成的待确认清单：背景 → 问题 → 建议选项 → 不确认的影响挂 REQ 编号，分必须确认/次要，基准文档缺失时数值标“不冻结”暂按正文执行），问可借鉴处。用户定口径：需求分业务与技术两块，两块边界都要问出来定下来、每条挂影响到的需求与模块，清零才进编码，功能才做得全。
- 改动（三件，纯模板措辞）：
  1. **功能型必问项**（事前防漏）：brainstorm 的「需求维度清单」节后新增「功能型必问项」节——八个功能型（文件上传、配置项、列表与查询、登录与权限、导入导出、通知、金额与判定、状态流转）各列必问条目，只收用户能拍板的业务边界；命中功能型就并入决策清单逐项问。末尾定口径：碰到偏技术的实现选择不当场定、不跳过，记 `[待定]` 写进 prd。改 2 个文件（`templates/platforms/common/skills/brainstorm.md`、copilot 的 `brainstorm.prompt.md`）。
  2. **缺失项扫描**（事后找茬）：workflow 1.5「需求评审」段后新增一段——呈报前把 prd 与需求列表当待挑错的稿子逐条过，专找四类缺失（判定规则/术语未定义、写了“可配置/灵活/适当”没定口径、文档内部自相矛盾、引用了不存在的文档或数据）；每条写清背景、问题、建议选项、不确认影响哪条 R-xxx 或哪个模块，汇成待确认清单；影响核心功能验收的当场问完记回 prd 才算材料齐，其余转 `[待定]`；基准文档缺失时相关数值标“不冻结”并写明暂行口径。
  3. **技术待定分流**：workflow 1.2「拆解只做一次」段后新增一段——拆解时收齐 `[待定]` 技术项：全局性的归技术选型与脚手架子任务随选型在它的 1.1 定；单条需求局部的写进对应子任务 prd，开工先问答或先写 design.md 定再动手；`[待定]` 不许原样带进实现靠猜。
  - 第 2、3 件改 6 个文件（`templates/workflows/` 三份 + `marketplace/workflows/` 三份镜像，两处锚点在六份里均唯一且原文一致）。
- 验证：8 个文件共 +58 行、0 删除；全量测试 21 包全过；新二进制临时项目 `init -y --claude` 实测：装出的 `.claude/skills/sunshine-brainstorm/SKILL.md` 含「功能型必问项」，`.sunshine/workflow.md` 含「缺失项扫描」「技术待定分流」各一处。
- 附带发现（未处理）：`marketplace/workflows/` 镜像有两处旧漂移（原型派发段仍是派发收窄前的旧文案、lint/类型检查缺“输出要精简”后缀），是此前批次未同步镜像所致，与本批无关，待用户定是否做一次镜像全量同步。
- 发版：已随 v0.5.35 发布，见文末 v0.5.35 发布记录。

## 找茬子代理 sunshine-audit（2026-11-20，用户批准）

起因：三件套的缺失项扫描是主会话自查，与写稿的是同一个 agent、盲区重合；用户问“有找茬的机制吗”后点选把扫描独立成子代理。做法：新增第五个子代理 `sunshine-audit`，评审前派一次，只读、只出清单。

- 代理定义：源 `templates/sunshine/agents/audit.md`（channel 运行时）+ 平台副本 17 份（md 13 平台、codex TOML 只读沙箱、kiro JSON 只读工具集、copilot agent.md、droid），共 18 个新文件。各平台 tools/permission 一律收成只读（Read/Glob/Grep 等价物），正文统一：递归防护、只读 prd.md 与需求列表两份文档、四类缺失（没定义/没口径/自相矛盾/引用不存在）、功能型必问项逐条核对（与 brainstorm 同一份八类清单）、必须确认/可延后两级、禁止改文档与直接问用户、待确认清单汇报格式（背景/问题/建议选项/影响 R-xxx）。
- 注册接线：`internal/platforms/platforms.go` 的 agentNames 加 audit（md/pull/skill 副本平台自动覆盖），codex、kiro、copilot、droid 四个显式清单各加一条；reasonix 只有 implement/check 两代理、DSH 靠自包含 prompt 派发，均不加文件，与现状口径一致。平台安装计数断言同步更新（E 组三平台各 +1；新平台组各 +1、kimi +2 含 skill 副本）。
- hook：`internal/hook/subagent.go` 加 AgentAudit 并进 agentsAll；Claude 与 Codex 两条路径都按 prototype 口径处理——只记 subagent_started 活动事件、不注入上下文、不要求有当前任务。serve 的 nativeAgentRoles 与 app.js 角色表/热力行同步加 audit。
- workflow：三份 workflow 模板与三份 marketplace 镜像的「缺失项扫描」段改为派 sunshine-audit 专扫，平台不支持派发或派发失败时主会话按同一口径自查兜底。
- 验证：全量测试 21 包全过；新二进制 init 实测（claude/codex/kiro/copilot 四平台）装出四种格式的 audit 文件且无裸 {{ 残留，.sunshine/agents/audit.md 同步装出；hook 端到端实测 subagent-context 对 sunshine-audit 退出 0、不注入、activity.jsonl 记下 agent=audit 的 started 事件。

发版：已随 v0.5.35 发布，见文末 v0.5.35 发布记录。

## EARS 打散吸收（2026-11-20，用户批准）

起因：用户转来《AI 编程实践第 17 节：使用 EARS 实现需求重写》并要求综合评估。评估结论：EARS（Mavin 2009，Rolls-Royce 出身）诊断与机制成立，最值钱的是“改写逼出待确认”；但文章零实测数据、只覆盖需求入口一环、全量重写有第二份文档与中文评审负担。用户在“单独工具 / 单独环节 / 打散”三选中定打散：不做独立 Skill、不加新环节、不产出重写文档，把零件放进已有位置。

- 写侧：brainstorm（common + copilot prompt 两份）在「功能型必问项」后加「验收标准写法（规则型需求）」节——判定/状态流转/异常处理的验收标准按“当……时 / 如果…… / 在……状态时，系统应……”句式写，一条一个动作，响应可衡量，不写测不出来的词，页面展示型不套；问答记进 prd 时就按此格式。workflow 六份（3 模板 + 3 镜像）的按需求循环段在需求列表写入处同步补一句。
- 查侧：sunshine-audit 18 份定义统一加——①「登录与权限」必问项加细（登录失败处理：错误文案、失败次数、锁定阈值与时长、计数窗口、解锁方式；账号禁用/失效表现；会话超时与多处登录），brainstorm 表格行同步加细；②新增「再做两遍深查」段：默套句式（对规则型需求在心里套句式，触发/响应/数字填不出算缺失，只做内部检查不产出改写文档）与可测性（留模糊词、一 条多动作、说不清怎么验证的都算缺失）；③汇报格式加推断标注（文档未提、凭推断怀疑的条目标明“（推断，文档未提及）”）。
- 有意不取：全量 EARS 重写留档（第二份需求文档、双份真相）、独立 EARS Skill（靠人记得调）、非功能需求套句式（作者自己也说单独放）。
- 验证：全量测试 21 包全过；init 实测装出的 audit 含默套句式与计数窗口、brainstorm 含验收标准写法节、workflow 含句式句；18 份 audit 定义均含默套段。

发版：已随 v0.5.35 发布，见文末 v0.5.35 发布记录。

## v0.5.35 发布记录（2026-11-20，已发布）

- 内容：三部分同批。①需求边界三件套：brainstorm 加「功能型必问项」（八类，只收业务边界，技术选择记 [待定]）、workflow 1.5 加「缺失项扫描」、1.2 加「技术待定分流」。②找茬子代理 sunshine-audit：新增第五个子代理（channel 源 + 17 份平台副本共 18 个新文件，工具一律只读；platforms 注册、hook 只记活动不注入、serve 角色同步；平台安装计数断言同步），缺失项扫描改为派它专扫、派不了主会话自查兜底。③EARS 打散吸收：规则型需求验收标准句式化（brainstorm 加「验收标准写法」节、workflow 六份同步），audit 加登录必问项细化、「再做两遍深查」（默套句式、可测性）、推断标注。共 33 个文件 +1606/-16。
- 主提交：本地 302a4d52f46071d58bf05a8f165092df62acf7f9 → 远端 155e445915a7bc19c4cdc5e2b672e36cb6c56835，tree ce54cc7b4fa71e9b8bef0c94e15a559ab4998396，父提交 63cab7264763cc9a7710cddad220f6dedd5b9cd2。
- tag：v0.5.35，annotated tag，tag 对象 bd04f07ac620f914fbbb382e01dd676881c04923 → 155e445。
- CI 与发布：发布 workflow run 44687555584、CI（tag）run 44687555480、CI（main）run 44687496430 均 completed success。
- Release：id 452343442，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.35_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.35（linux/amd64），`sunshine version` 报 0.5.35；darwin_arm64 包 checksum c2489ebfcef0bd077f1cba2570e174e418e9b3865547e63b9fccaebfd21c69f9、linux_amd64 包 checksum 69617bdfe2c7c5dd8cc67765c9b3db0b009acfb64f4cadbbde775a214854445f，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.35，最新 0.5.35），无需升级。”）；发布包二进制 grep 确认 “默套句式” 12 处、“sunshine-audit” 40 处在位；发版前全量测试 21 包全过、init 实测四平台装出 audit 文件、hook 端到端记下 audit started 事件。

## 证据化检查 + 风险两档 + EARS 配图补遗（2026-11-20，已随 v0.5.36 发布）

### 起因

- 用户拿中睿 Deerpowers SDD 方案与 Sunshine 对比后，点选把可借鉴项落进 Sunshine，并纠正一处：Sunshine 默认走 native 工作流、TDD 不是默认流，借鉴项不能只落在 TDD 工作流里，要全工作流生效。
- 三个借鉴点定版：①“判”的机器化只取轻型版——不自建规则母题校验器（那是按对方交付领域攒的、通用项目不适用）、不加硬拦截（与“门可 waive、无硬锁”的既定口径冲突），取“证据化检查”：验收标准编号 + check 逐条对账附证据 + 实跑项目自带测试/lint 拿工具链原始结果当判据。②风险分级只取两档版（标准/轻量），治“小改动走全流程太重”。③模型路由不做（平台支持不齐、DSH 上模型切换无效、无质量实测数据）。
- 同日用户又发来 EARS 文章的落盘示例配图要求细看：9 张内容大图已逐张放大转录核对。图里比正文多出的可吸收点集中在落盘示例的待确认问题（5 项，正文实例一只列 3 项）：校验时机（状态校验在密码验证之前还是之后，在后可能泄露账号有效性）、提示时效（用户可见反馈的响应时限）。另两处次级点：敏感操作留审计日志（总览图文件/权限样例）、需求的负边界（总结图“不做什么”）。总览图另有一套六句式字母简写与正文六型冲突，系配图自创，不采；图中数字（≤500ms 之类）是示例值不是口径，不采。

### 一、证据化检查

- 现状：check skill 的步骤 3 已要求跑 lint、类型检查与测试；但 19 份 check 代理定义（channel 源 `templates/sunshine/agents/check.md` + 18 份平台副本）的“运行验证”只写 lint 与类型检查、没提测试；汇报格式只有验证结果数行，没有逐条对账 prd 验收标准；prd 验收标准无编号，对账没有锚点。
- 改动 1（编号）：brainstorm 两份（common skill + copilot prompt）的「验收标准写法（规则型需求）」节补编号要求——验收标准逐条编号（AC-1、AC-2……，按节内出现顺序），编号随问答记进 prd 时就带上；workflow 六份（三份源 + 三份 marketplace 镜像）在 1.1 的验收标准句式句后同步补编号一句。
- 改动 2（check 代理 19 份）：运行验证补“实跑项目测试命令”；新增「验收标准对账」段——汇报前对照 prd 验收标准节逐条对账：每条给编号、结论（已实现 / 未实现 / 超范围 / 未验证）与证据（测试名、命令输出摘要或 `file:line`，至少其一）；给不出证据的只能标“未验证”，不许写“已实现”；存在未实现或未验证条目时，汇总不得写全部通过。汇报格式加「验收标准对账」表段，验证结果加“测试”一行。
- 改动 3（check skill）：`templates/platforms/common/skills/check.md` 的步骤 6 补同样的对账报告要求（inline 平台走 skill，是它们的主入口），与代理口径一致。
- 改动 4（workflow 六份）：2.2 质量检查段补主会话侧一句——收到 check 汇报先看对账表，有未实现/未验证条目不算检查通过，按现有修复循环处理。
- 不动：check 的自行修复权限与汇报主体结构、检查节食的注入内容（验收标准节已在注入内，编号随之进入）。

### 二、风险两档

- 存储：复用 task.json 的 `meta` 扩展位——拆解建轻量档子任务时 `task create ... --meta risk=light`，事后改档用 `task set-meta <任务> risk light|standard`；不新增第 25 个字段（24 字段与 Trellis schema 对齐是既定约束），标准档不写、缺省即标准。本部分无 Go 代码改动。
- 拆解标档（workflow 六份 1.2，在「技术待定分流」后加「风险标档」段）：逐个子任务定档。轻量档判据：纯展示、文案、样式、配置默认值这类不改数据、不改判定逻辑、无外部副作用的改动。强制标准档：金额与判定、登录与权限、删除或覆盖数据、数据迁移、对外接口契约变更——沾一条就不许轻量。标档结果随子任务清单在呈报确认时一并给用户看，用户可改档。
- 开发路由（workflow 六份的 in_progress 派发段）：激活子任务前先读 task.json 的 `meta.risk`；轻量档由主会话直接实现（不派 implement 子代理），check 照常派发，回写 spec → 提交 → 归档的顺序不变；标准档走现有派发流程。防滥用两条：档位只在拆解时定，主会话开发时不得临场改档、不得以“改动少”为由跳过派发（现有“不得以任务小、改动少为由改走 inline”的规则补明：唯一例外是拆解时已标轻量档的任务）；轻量档实现中发现实际动了数据或判定逻辑，立即停手，改档为标准并按标准档流程补走。
- inline 平台（Codex inline、DSH 自包含派发等）本来就不派 implement，轻量档对它们无差别，workflow 不单独写。

### 三、EARS 配图补遗（纯模板措辞）

- brainstorm 两份：「登录与权限」必问项补“校验时机：状态/权限校验在身份验证之前还是之后执行（在后可能泄露账号或资源是否有效）”与“敏感操作（删除、改权限、金额变更）留不留审计日志、记哪些字段、留多久”；「功能型必问项」节后补跨类一条「时机、顺序与时效」：多步校验/处理问清先后顺序与触发时机，用户在等的反馈（提示、通知、结果）问清响应时限，没有口径的记 [待定]。
- audit 18 份：登录必问项细化行补“校验时机”；「再做两遍深查」段补一句——再查时机与顺序（多步处理没写先后与时机、用户可见反馈没写响应时限的算缺失）与负边界（只写了做什么、没写明确不做什么的，标出来供确认）。

### 验证口径

- 全量测试 21 包全过；grep 核对新措辞在各目标文件的在位份数（check 代理 19、audit 18、brainstorm 2、workflow 6、check skill 1）；新二进制临时项目 init 实测装出的 check 代理含对账段、brainstorm 含校验时机；task CLI 实测 `--meta risk=light` 落盘与 set-meta 改档。
- 发版：已随 v0.5.36 发布，见文末 v0.5.36 发布记录。

- 落地结果（2026-11-20）：三部分全部实现，共 47 个文件 +409/-65（含 PLAN.md 本节）。证据化检查：brainstorm 2 份与 workflow 6 份补 AC 编号；check 代理 19 份加「验收标准对账」段并把运行验证补齐测试；check skill 1 份补对账报告要求；workflow 6 份加「对账把关」。风险两档：workflow 6 份加 1.2「风险标档」与开发阶段「风险档路由」，存储复用 task.json 的 meta（`--meta risk=light` / `set-meta`），无 Go 代码改动。EARS 配图补遗：brainstorm 2 份补校验时机、审计留痕与跨类「时机、顺序与时效」；audit 18 份补校验时机、审计日志口径与深查「时机、顺序与负边界」。全量测试 21 包全过；新二进制临时项目 init 实测装出的 check 含对账段、audit 含负边界、workflow 含标档/路由/对账把关；task CLI 实测 `--meta risk=light` 落盘与 `set-meta` 改档均正确。

## v0.5.36 发布记录（2026-11-20，已发布）

- 内容：三部分同批。①证据化检查：验收标准逐条编号 AC-1……（brainstorm 2 份 + workflow 6 份）；check 代理 19 份加「验收标准对账」段（逐条结论 已实现/未实现/超范围/未验证 + 证据，无证据只能标未验证，有未实现/未验证不许写全部通过）且运行验证补齐实跑项目测试；check skill 同步对账要求；workflow 6 份加主会话「对账把关」。②风险两档：workflow 6 份加 1.2「风险标档」（轻量判据 + 五类强制标准）与开发阶段「风险档路由」（轻量档主会话直做不派 implement、check 照跑、档位只在拆解定），存储复用 task.json 的 meta.risk，无 Go 代码改动。③EARS 配图补遗：brainstorm 2 份补校验时机、审计留痕与跨类「时机、顺序与时效」；audit 18 份补校验时机、审计日志口径与深查「时机、顺序与负边界」。共 47 个文件 +411/-65。
- 主提交：本地 f522706a8f52df00805808fbd3eec57d6b751a18 → 远端 a7e7d50dfcfcdba4419460230b7310a4bd3dc4e2，tree 008b43e88f5e011ddf0e91a303f2cc1b10c470f8，父提交 9c9c779bb7e962f790cb0dce328dde57889a43ec。
- tag：v0.5.36，annotated tag，tag 对象 e247392c34252e8af22c40244f076ecb18736e60 → a7e7d50。
- CI 与发布：发布 workflow run 44691572288、CI（tag）run 44691572253、CI（main）run 44691536611 均 completed success。
- Release：id 452367868，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.36_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装（无参）装上 v0.5.36（linux/amd64），`sunshine version` 报 0.5.36；darwin_arm64 包 checksum 5386f62b68d5ce402b8cdf9872c1aa5a9f49ba4be3a618c5602d566fa8afbed3、linux_amd64 包 checksum 6ef8c853176970f6ffccf8d54512494f224db5bc9b483c689206ea7ef9bcbead，均与 checksums.txt 一致；`upgrade --dry-run` 通过（“已是最新版本（当前 0.5.36，最新 0.5.36），无需升级。”）；发布包二进制 grep 确认 “验收标准对账” 50 处、“风险标档” 4 处在位；发版前全量测试 21 包全过、init 实测装出新内容、task CLI 实测 meta 标档与改档正确。

## CodeGraph 浅集成（2026-11-20，已随 v0.5.37 发布）

- 背景与定调：对比网易 CodeWave 的 CodeMap 后，用户定调不自建代码地图、只借入现成工具，并拍板不用测、就用 CodeGraph（colbymchenry/codegraph，MIT，tree-sitter 确定性解析建图、零 LLM token，文件监听自动同步，MCP 工具与 CLI 孪生命令双通道）。范围收敛为**只针对存量项目**：核心用途是存量入场摸底与整理 spec（spec-bootstrap、spec 与代码对齐时 research 查图核对），图建好后该项目的开发阶段顺带复用（implement 改符号前查调用方、check 查波及面）；纯新建项目不装不建。用户特别交代教程里要维护好这一块。
- 口径（两层缺什么补什么）：init 时项目里已有代码才触发；①工具层：PATH 上没有 codegraph 就跑官方安装脚本装上（装到用户机器、不打进 Sunshine 发布包，失败退 npm 全局装，再失败只警告给手动命令）；装了跳过。②项目层：没有 .codegraph/ 就 `codegraph init --yes` 建图；有就跳过。③接线层：`codegraph install --target=<映射> --yes` 把 MCP 写进各平台配置，仅在刚装上工具、刚建图或本次 init 新装了平台时执行（该命令幂等）。全程失败不阻断 init（外部工具不得拖垮主流程）；`--no-codegraph` 与环境变量 SUNSHINE_NO_CODEGRAPH=1 可关闭；测试桩二进制路径含 sunshine-test-bin 时自动跳过，免得端到端测试触发真安装。
- 平台映射（CodeGraph target 名已按其仓库 installer/targets 核对）：claude-code→claude、cursor→cursor、codex→codex、kiro→kiro、gemini→gemini、qoder→qoder、pi→pi、opencode→opencode、dsh→dsh、grok→grok、kimi→kimi、antigravity→antigravity、copilot→copilot-cli、droid→droid，共 14 个；其余 8 个平台（zcode、trae、omp、snow、reasonix、kilo、devin、codebuddy）无对应 target，靠 CLI 孪生命令 + 模板规矩覆盖。注意 dsh 的接线是全局级（写 $DSH_HOME 的 cordis patch），是 CodeGraph 自己的设计。
- 实现：新增 internal/codegraph 包（HasExistingCode 存量检测：init 写任何文件之前判定，清单文件或源码扩展名命中即算，跳过 .git/node_modules/vendor 等，带遍历上限；TargetsFor 映射；Provision 决策与执行，Runner 可注入以便单测）。init.go 三条分支（新建 / -f / 增量）在平台安装后调用，增量分支无新平台时也调用以便补建图。
- 模板规矩：workflow 6 份（及 marketplace 镜像）加「代码地图（CodeGraph，仅存量项目）」小节（何时有图、整理 spec 与 research 先查图、implement 改符号前查 callers、check 用 affected/impact 出波及面证据、无图跳过）；research / implement / check 三种代理的全部副本各加一段对应的查图规矩（MCP 与 CLI 两种写法并列）；spec-bootstrap 相关 skill 同步一句。
- 教程：docs/tutorial.md 与 docs/tutorial-en.md 各加一节专讲（只针对存量、何时装、两层口径、整理 spec 怎么用、开发阶段复用、关闭办法、图与升级归 CodeGraph 自己管）。
- 验证口径：全量测试；新包单测（存量检测、映射、Provision 决策表用假 Runner）；init 端到端用桩 codegraph（PATH 注入、记录调用）验四态（空项目不触发、存量+无工具走安装、存量+有工具只建图、全就绪全跳过）；沙箱真装 CodeGraph 后对夹具存量项目跑真 init 并用 codegraph explore 查一次证明图可用。
- 发版：已随 v0.5.37 发布，见文末 v0.5.37 发布记录。
- 落地结果（2026-11-20）：已实现并验证。新增 internal/codegraph 包（存量检测 / 平台映射 / Provision 三层供给，Runner 可注入）与 init 三分支接入（--no-codegraph 与 SUNSHINE_NO_CODEGRAPH 关闭、测试桩路径自动跳过）；模板 64 处（research/implement/check 代理 56 份、workflow 6 份、spec-bootstrap 与 check 两份 skill）；教程中英文各加一节（§12.2 内）。共 70 个文件。全量测试全过；新增单测覆盖存量检测、映射与 Provision 决策表，init 端到端桩测四态全过（存量接线建图、空项目不触发、--no-codegraph 跳过、二次 init 零调用）。沙箱真装实测：全新环境跑 init，自动经官方脚本装上 CodeGraph、接线 claude、建图成功，`codegraph callers Charge` 准确返回 CreateOrder（order.go:7），二次 init 报已就绪跳过，空项目 init 零代码地图输出。

## v0.5.37 发布记录（2026-11-20，已发布）

- 内容：CodeGraph 浅集成，只针对存量项目。新增 internal/codegraph 包并接入 init 三个分支：init 写文件前判定存量，三层供给缺什么补什么（工具缺了跑官方脚本装、失败退 npm、再失败只警告；项目没图就建；接线只在刚装工具/刚建图/本次新装平台时补，14 个平台有 CodeGraph target 映射），全程失败不阻断 init，--no-codegraph 与 SUNSHINE_NO_CODEGRAPH=1 可关闭，测试桩二进制自动跳过。模板 64 处：research/implement/check 代理 56 份各加查图段、workflow 6 份加「代码地图（CodeGraph，仅存量项目）」小节、spec-bootstrap 与 check 两份 skill 同步；教程中英文 §12.2 各加 CodeGraph 专节。共 71 个文件 +997/-11。
- 主提交：本地 e58af94397db4b212fc10cf0f9388130818dcd0f → 远端 ec2b35f9a27ec2f08007614773b5709b14d3e4e3，tree 11487612dc369d4af3c2d9575895872b7125a2b2，父提交 6ad25b6046defd397ac82ccd6c617043c3f3a4f8。
- tag：v0.5.37，annotated tag，tag 对象 ec1c057eeb06ec098b48dead6cd6865503245374 → ec2b35f。
- CI 与发布：发布 workflow run 44698031004、CI（tag）run 44698031142、CI（main）run 44697995236 均 completed success。
- Release：id 452406960，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.37_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装装上 v0.5.37，`sunshine version` 报 0.5.37，`init --help` 含 --no-codegraph；darwin_arm64 包 checksum 8f8910f065c6e8b2fb933700a046c0ce79b63d00bbe74aca750505e01dce19cd、linux_amd64 包 checksum 7c19a9cd7bc67c43d7f40ff5ec4dd5416b623e63ae2a7f6179f624a2a81b9259，均与 checksums.txt 一致；`upgrade --dry-run` 通过；发布包二进制含“代码地图”67 处；发版前全量测试全过、桩测四态全过、沙箱真装 CodeGraph 端到端实测通过（自动安装、接线、建图、callers 查询准确、二次 init 跳过、空项目不触发）。

## 云端平台路线评估（2026-11-20，存档，未立项）

起因：用户看网易 CodeWave SDD 方案后问 Sunshine 能否包装成 Web、并点选评估其中的云端平台档（在网页里直接开发，平台托管执行）。结论：技术上能做，但现在做是亏账——不放大核心资产，反而逐条抵消现有优势；建议分步演进，平台档推迟到团队付费意愿被验证之后。本节只是评估存档，不是立项决定；团队方向（团队中枢 / 桌面指挥台 / MCP）仍未定。

### 形态定义

- 云端平台档：用户在浏览器里提需求、看代码、验收；平台在云端沙箱里跑 agent 写代码、跑测试、预览与发布。参照物是 CodeWave 的纯云端模式（L4）：它自带 Web 云端 + 自有 IDE，执行端只有云端与自家 IDE，不接 Claude Code / Codex / DSH 等第三方工具（PPT 原文核对：无 MCP、无开放 API 字眼，Codex 仅作客户案例对照组出现）。
- 与控制台档的区别：控制台只是给现有 CLI 换个 Web 壳（扩展 sunshine serve），执行仍在用户本机与既有工具里；平台档是换芯，Sunshine 自己长出 agent 运行时。

### 可行性与唯一现实路线

- 唯一现实路线：云端沙箱里跑现成 CLI 的无头模式（Claude Code headless、Codex exec 等），把用户本机环境整体搬上云；自研 agent 运行时等于重写一个 Claude Code，不在讨论范围。
- 有利条件：Sunshine 状态全走文件（任务、spec、会话断点可续），天生适合云端托管与恢复；工作流、模板体系可大半搬进沙箱镜像。
- 可复用资产：工作流 / spec / 任务系统 / 模板规矩（核心）、Channel 协同层（部分）、serve（仅 UI 经验）。
- 作废资产：22 个平台的适配层（hook、子代理、命令各平台一份）是当前工程量最大的一块，平台里 Sunshine 自己就是宿主，这块被绕过而非复用。

### 三笔账

1. 负担质变：从「发一个二进制」变成「养一个在线服务」——跑用户代码即跑不可信代码，沙箱隔离、防滥用（挖矿、攻击跳板）、数据备份、可用性与故障响应都是持续投入；token 成本是死结（平台垫付则毛利被模型价格牵着走，用户自带 Key 则门槛与体验打折）。以当前单人 + AI 的开发节奏，平台建设期以月计，期间 CLI 主线停摆。
2. 定位反转：现有卖点是装进现有工具、不锁定、代码不出本机，平台三条全反；目标客户从「已在用 Claude Code 的开发者」变成「愿意把整个开发搬进新平台的企业」，而企业对代码上云最谨慎，最后多半要私有化部署，又回到卖软件加部署的老成本结构。与已定的仓库私有化 + license 验证（卖工具路线）是两套商业模式，需要二选一或明确先后。
3. 竞争格局：对面是网易 CodeWave、腾讯 CodeBuddy、字节 Trae 等成建制团队，正面竞争无胜算；差异化仍只能靠 SDD 规矩本身，而规矩不需要平台也能交付——平台化不放大核心优势。

### 平台档的真实好处（备查）

- 工作流可硬执行：各平台 hook 能力参差的问题（DSH 降级、Cursor 缺 post-tool-use hook 等）全部消失。
- 门槛低：打开浏览器就能用，不用装一堆 CLI；团队协作与 Skill 资产跨项目沉淀更容易做。
- 商业模式变订阅制，收入可预测；license 吊销、版本碎片问题天然不存在。

### 建议路径（分步演进，每步独立有价值、不锁死）

1. 控制台档：扩展 serve 成可操作的 Web 控制台（项目、任务看板、活动流、点按钮触发 init / 建任务 / 归档），验证 Web 入口是否真的有人要。
2. 端云一体（Sunshine 版）：Web 里点一下，在用户本机拉起 Claude Code / Codex 会话干活——壳在浏览器、执行在本地、代码不出机。
3. 仅当前两步证明团队付费意愿后，再评估执行上云（平台档）。平台档的核心风险（代码上云、自研运行时、防滥用）全在第三步才出现，没必要现在全押。

### 后续调研与用户定调（2026-11-20 补记）

- 同日做了现成宿主外壳调研（报告在 workspace/research_notes/cloud-agent-host-shells-20261120-0814/，另有同名调研页）：云上没有可白标的「Pi 云端版」；现成壳只有 OpenHands（核心 MIT 免费，多租户/RBAC/计费在付费 enterprise 层）；其余为「免费引擎（OpenCode / Pi / CLI 本身）+ 沙箱（Sprites / E2B 等按量收费）+ 自建薄壳」。OpenCode 拼法对 Sunshine 复用率最高（已有 OpenCode 平台适配、skills 格式同源），但薄壳开发量与沙箱费用仍在。
- **用户定调：收费就不行。** 上述路线凡涉及付费（企业许可、沙箱月费/按量、自建多租户计费体系）均被否，云端平台方向就此搁置，不立项。零费用的唯一形态是开源组件跑在自有硬件上自托管——那属于控制台档 / 团队中枢方向（见上文建议路径第 1、2 步），不是对外 SaaS 平台；将来若重启讨论，以此为前提，不再重提收费路线。


## 镜像一键安装（文档，2026-11-20）

- 起因：用户在 Mac 上 `sunshine upgrade` 下载 v0.5.37 发布包超时（GitHub 发布下载走独立 CDN，网页能开不等于下载通）。install.sh 内部下载地址写死 github.com、无镜像开关，一键安装与 upgrade 同病。
- 临时方案（已实测）：经公共镜像代理 ghfast.top 拉脚本，再用 sed 把脚本内 `https://github.com/` 全部改走同一镜像后执行——版本查询、下载、checksums 校验、落盘均为原脚本流程；沙箱端到端实测装上 0.5.37，文档里的命令逐字提取复跑亦通过。备用镜像 gh-proxy.com（同批实测存活；12 个候选中仅此两个返回正确的官方 checksums.txt，其余或连不上或被限流/拒绝）。
- 已维护进文档（用户指示）：README.cn.md / README.md 快速上手各加「GitHub 下载不了时：镜像一键安装」一节（命令 + 原理 + 换备用镜像 + 第三方代理提示）；教程中英文 §1 各加一段（中文版附命令与回指 README，英文版以引用块附命令）。
- 边界：镜像站为第三方公共代理、状态多变，只作直连失败时的备选；包仍按 checksums.txt 校验。产品化方向（upgrade / install.sh 内置候选镜像名单、直连失败自动逐个试）已向用户提出，未点选，未立项。
- 另记体积实测（同日用户问）：v0.5.37 二进制 35.5MB（goreleaser 已带 -s -w；不剥为 51.5MB），tar.gz 下载包 12.7–13.7MB；大头是 Go 静态运行时与重依赖（modernc.org/sqlite、go-git 加密栈），全部平台模板仅 1.7MB；UPX 不采用（误报与启动解压代价），维持现状。


## 重跑 init 沿用已有开发者名（2026-11-20，用户点选，已实现待发版）

- 问题：init 定开发者名的顺序原为「-u > git user.name > developer」，不读项目里已记的 `.sunshine/.developer`，且每次 init 都按本次解析结果重写该文件——git 配置与首次名字不一致时，重跑 init（例如为存量项目补 CodeGraph）会把开发者名悄悄改掉。
- 修复：resolveUser 改为「-u > 已有 .sunshine/.developer > git user.name > developer」；-u 显式指定仍最高。-f 路径在 scaffold 备份/重建之前解析，同样能读到旧名字。增量重跑路径本来就不碰 .developer，无变化。-u 的帮助文案已同步。
- 验证：新增 TestResolveUserPrefersExistingDeveloper（沿用、-u 覆盖、无文件不误读三态）；全量 22 包测试全过。
- 备查：目前没有独立的 `sunshine codegraph` 命令，CodeGraph 供给只挂在 init 上；手动触发即重跑 init，或直接用 codegraph 自己的 install/init 命令。是否加独立命令待用户定。


## sunshine codegraph 命令（2026-11-20，用户点选，已实现待发版）

- 新增 `sunshine codegraph install` / `sunshine codegraph status` 两个子命令，给 CodeGraph 供给一个随时手动触发的入口（此前只挂在 init 上，无独立命令）。install 复用 codegraph.Provision：手动执行不受存量判定限制、每次重新接线（codegraph.Options 新增 Force 字段），平台取当前目录已装平台（installedPlatformIDs，口径与 init 的 filterUninstalled 一致）；非 Sunshine 项目目录也能用（只建图不接线）。status 只读：工具路径、.codegraph/ 是否已建、存量检测、可接线平台与 target。SUNSHINE_NO_CODEGRAPH=1 时 install 同样停用并明示。
- 验证：新增 TestProvisionForceWires（全就绪 + Force 只接线一次）、TestInstalledPlatformIDs（注册顺序）；全量 22 包全过；沙箱夹具实测全通（status 报告未装/未建 → install 经官方脚本装工具、接线 claude、建图 → status 全绿）。
- 文档同步：教程中英文 §12.2 各加「手动触发」一段、附录手动干预速查各加一行；附录 -u 优先级行同步为新顺序（-u > 已有 .developer > git user.name > developer，中英文）。
- 与上一节的 .developer 修复（3bd0c3c）同批进下一版，均未推送。


## 开始纪律：规划阶段只读本项目（2026-11-20，用户点选，已实现待发版）

- 起因：用户贴出 DSH 在 demo16 的开局记录——流程没走偏，但 agent 开工前把兄弟目录 demo14 的设计产物（需求列表、导航 prd、原型、共用 CSS、概要设计）整套读了一遍才问第一个问题。定性：共性问题（agent 即兴「找参考」，与平台机制无关），DSH 放大（无 hook 注入、pull 式开局靠满盘摸索，容易滑向漫游）。
- 改动（纯模板措辞）：①三份 workflow 模板（native / tdd / channel-driven）在第 1 阶段 1.0 之前各加「开始纪律（第 1 阶段全程适用）」小节：只读本项目（任务文件、spec、doc、用户点名文件），不读其它项目/历史 demo 的设计产物作本项目依据（除非用户点名）；开局按 start 清单加载、workflow 按节读、第一个问题前不做项目外检索。②start 入口源文件 templates/platforms/common/commands/start.md 加「开局纪律」节（全平台同一源，DSH 镜像 skill 同步生效）：步骤 1–3 之外不漫游、不读他项目产物、workflow 按节读、完成分流前不做额外探索性阅读。
- 未动：marketplace/workflows 镜像（同步与否仍待用户定，与既有两处旧漂移一并搁置）；sunshine-meta 的 local-architecture/workflow.md 是架构参考文档、非 workflow 本体，不在同步范围。
- 验证：全量 22 包测试全过；grep 核对含「1.0 创建任务」的 workflow 模板共 3 份、均已加节，start.md 已加节。
- 边界（已向用户说明）：措辞是降概率、不是硬拦截，Sunshine 无硬门禁；与自检封顶、派发节食同类修法。

## 长期记忆索引注入（2026-11-20，用户点选，已实现待发版）

- 起因：`.sunshine/memory/` 写有入口（3.3 可选沉淀）、读无机制——hook 零引用，靠 agent 想起来才翻，写了没人看。
- 改动：SessionStart 注入「长期记忆索引」节（规范地图之后）：只注入 index.md 数据行，上限 4000 字节按行截断并注明去向，无条目不注入；`task context --mode packages` 同步装配（hook.MemoryIndexText 共用）。注入文案写明故事档案定位、冲突以规范为准。
- 验证：新增 5 个测试（解析/空索引/无文件/截断/端到端），全量 22 包过；临时项目真跑 session-start 与 task context 两面均对。本地提交 577a06e。

## EARS 深度逼问：audit 新模式（2026-11-20，用户点选，已实现待发版）

- 起因：用户贴《AI 编程实践第 17 节》EARS 实例截图（多表处理工具的必须确认清单），问能否只对当前 PRD 跑一次 EARS。评估结论：打散部分不回撤（写侧句式节管出生质量、audit 粗扫管每次评审、证据化对账管事后验），新增的是第四层「按需深挖」；单开阶段已否（第二事实源漂移、逼全需求硬套句式、成本方向与节食相反）。
- 形态：不新建代理、不新建阶段——给 sunshine-audit 加「深度逼问」模式，派发注明才启用，替代当轮普通扫描（覆盖不叠加）。逐条需求把正文规则（判定/算法/配置/阈值/状态流转）与验收标准套 EARS 句式逼问未定点，每条标跨需求影响；只出必须确认/可延后清单，不产出改写全文；已定过的不重问。
- 触发（条件自动，非纯点名）：①设计先行父任务 1.5 评审且需求含规则型内容（金额判定/登录权限/状态流转/删除覆盖/对外契约沾一条）；②开发先行任务或子任务 1.5 激活前属标准档（未标 risk=light）。轻量档不触发；用户可点名补跑、可豁免（记进 prd）。任务级深挖另读相关 spec：已沉淀全局规则视为已定，新定下的全局规则当场回写 spec。
- 改动（纯模板措辞，21 文件）：audit 代理 18 份（channel 源 + 17 平台副本，含 codex toml / kiro json）各加「深度逼问模式」节；三份 workflow 的 1.5 缺失项扫描段后各加「深度逼问（条件触发，替代当轮扫描）」段。
- 未动：marketplace/workflows 镜像（待用户定，与既有漂移一并搁置）；brainstorm 写侧与 check 对账不回撤、不改动。
- 验证：全量 22 包测试全过；grep 核对 18 份 audit 均含新节、三份 workflow 均含触发段，kiro json 解析正常。真实 PRD 试跑未做（用户手头 prd 不便发来），产出质量待真实项目验证。

- 排版微调（同日晚，用户点选）：audit 汇报格式对齐 EARS 实例截图——「必须确认」标题加注（影响核心功能验收），每条加可选「附带问题」行（主问题带出的次级确认，没有就不写）。同轮已定：prd 本体不改 EARS 问卷格式——契约与问卷不缝合，未决选项不进实现上下文，全量 EARS 化即已否的硬套。

## v0.5.38 发布记录（2026-11-21，已发布）

- 内容：六批打包。①重跑 init 沿用已有 .developer：开发者名解析改为 -u > 已有 .sunshine/.developer > git user.name > developer，重跑 init 不再被不一致的 git 配置顶掉名字（3bd0c3c）。②新增 `sunshine codegraph install / status` 命令：手动装工具、接线、建图与只读查状态，非 Sunshine 目录也能建图（b80bdb9）。③开始纪律：三份 workflow 在 1.0 前加小节、start 入口加开局纪律——规划阶段只读本项目、开局不全盘扫描（b852329）。④长期记忆索引注入：SessionStart 与 task context --mode packages 装配 .sunshine/memory/index.md（仅索引、上限 4000 字节按行截断、无条目不注入）（577a06e）。⑤EARS 深度逼问：audit 新增模式（派发注明启用、替代当轮粗扫），条件自动触发（设计先行父任务 1.5 评审含规则型内容、开发先行标准档任务激活前），任务级深挖读 spec 且新全局规则当场回写（f6da893）。⑥audit 清单排版对齐 EARS 实例：必须确认标题加注（影响核心功能验收）、每条加可选附带问题行（1eff59a）。共 35 个文件 +631/-26。
- 主提交：本地 1eff59a79775d5649acb9b04a2cc9dc6af12b9a2 → 远端 662907833681079af0e9b974a7d332866cb97aa3，tree 064336f3f6ecee6579c98a53780de9681e517821，父提交 7b3c0c8a77061f405fb83ab3e1e86634da9c364b。
- tag：v0.5.38，annotated tag，tag 对象 2677190bc163bbede5f98ea89f5ed4967ad3bb53 → 6629078。
- CI 与发布：发布 workflow run 44769332920、CI（tag）run 44769333208、CI（main）run 44769254925 均 completed success。
- Release：id 452902282，非草稿，6 个资产在位（checksums.txt + sunshine_0.5.38_darwin_amd64/arm64.tar.gz、linux_amd64/arm64.tar.gz、windows_amd64.zip）。
- 实测：一键装装上 v0.5.38（`sunshine version` 报 0.5.38）；linux_amd64 包 checksum 9fa48beb9813e9596ad27996133497b69335e1f82ef89d0faf0b796be4cbd03f 与 checksums.txt 一致；`upgrade --dry-run` 报已是最新；`codegraph --help` 正常；发布包 init 装出的 audit 代理含「深度逼问模式」、workflow 含「深度逼问（条件触发」段；发版前全量 22 包测试全过。遗留：EARS 深度逼问的真实 PRD 试跑未做，产出质量待真实项目验证。

## 2026-11-21 下载镜像回退产品化（用户点选，未发版）

- 背景：11-20 用户 upgrade 下载超时，实测公共镜像（ghfast.top、gh-proxy.com）可当搬运通道，当时只落了文档里的手动 sed 命令；用户问 upgrade/update 是否同样受影响并点选三入口全做。
- 新增 internal/mirror：候选顺序 = 环境变量 SUNSHINE_DOWNLOAD_MIRROR → 直连 → 内置 ghfast.top / gh-proxy.com；Rewrite 只改写 github.com / api.github.com / raw.githubusercontent.com 三主机，其余（自建实例、本地、SSH 形态）原样。
- upgrade：Client 加 Bases/UsedBase，getJSON 与 download 按候选逐个试；404（发布不存在）是确定性结论立即返回不试镜像；下载走镜像时 Run 打印明示。update 的版本检查经 LatestVersion 自动继承。
- update/registry：cloneRegistry 在 github.com HTTPS + 网络类错误时逐镜像整轮重试（SSH/认证/不存在不重试）；probeIndexHTTP 同口径（404/认证不重试）。
- install.sh：版本查询（latest 跳转 + API 两法）与包下载都按通道循环；包与 checksums 必须同通道；校验失败直接报错不换通道重试（防完整性降级）。
- 验证：mirror/upgrade/registry 新单测（假传输层直连失败走镜像、404 短路、retryable 判定）；全量 22 包过；install.sh 用 curl 垫片封直连实测——指定版本与 latest 两条路都经 ghfast 装上 v0.5.38、校验通过、提示文案正确。
- 文档：README 中英文 + 教程中英文的镜像节改为「已内置回退」，手动命令简化为只经镜像取脚本（sed 改址不再需要）。
- 同批前置：市场 workflow 镜像全量同步已单独提交 23b2811（本地，未推送）。

## v0.5.39 发布记录（2026-11-21）

- 内容：下载镜像回退产品化（internal/mirror + upgrade/update/install.sh 三入口，本地 7a132ef）+ 市场 workflow 三份镜像全量同步（本地 23b2811）。push-initial 按全树推送，两笔本地提交合成为一笔远端提交。
- 远端主提交 7c7ab54a65eb33326a4592069691a6c6b3a08bf4，tree a75eacee3c9a7a08bc68085865dddc174cb79b49，父 8f1cc732a181bfcfb15a8c8c414958e1301fc28b。tag v0.5.39 对象 9dfe46f426b815f7e8e8e1487d2bbc17843f3fe7。
- run：发布 44852636910、CI(tag) 44852636908、CI(main) 44852604516，均 success。Release id 453367542，6 资产齐全；linux_amd64 sha256 750d2beb80955342d571b00d323c6838dd261c03e95d4afa563946658b05a432 与 checksums.txt 一致。
- 实测：线上 install.sh 直连装上 0.5.39；封直连（curl 垫片）后版本查询与下载均经 ghfast 回退装上 0.5.39、提示文案正确（中途 /tmp 垫片被清、一度假象直连成功，已复测澄清）；upgrade --dry-run 报已是最新；发布包 init 装出的 audit 含深度逼问节、workflow 含同步内容。校验包在 ~/workspace/scratch/release-0539/。

## 2026-11-21 交付文档七份占位（用户点选，未发版）

- 起因：用户指出概要设计口径上「按需」但默认在生成（init 铺文件 + workflow/brainstorm「三件套」措辞）；讨论后用户定：常规交付全套七份都占位——概要设计、详细设计、数据库设计、接口文档、测试报告、用户手册、部署说明。
- init：产品设计类型新增六份占位模板（templates/design/{detailed,database,api,test-report,manual,deployment}.md → doc/delivery/），概要设计保留 doc/design/ 原路径并改为占位头（点名时可提前填）；占位只有标题、章节骨架与「交付时经用户确认后生成、交付前不读不填」声明。init 帮助与完成提示文案同步。
- workflow×3 + brainstorm：删「三件套」与「循环结束后按需生成交付客户的文档」，改为「交付文档（只占位，项目收尾时经确认生成）」节——全部子任务归档、父任务集成复核通过后列清单请用户确认，点头哪份生成哪份（依据 prd/需求列表/spec/check 对账/代码事实），点名提前生成不受限；拆解依据改为「概要设计已有实质内容（非占位）才参考」；集成复核行加收尾处理占位一步。市场镜像三份已再同步（diff 空）。
- 验证：全量 22 包过；新二进制 init --type 产品设计实测 doc/ 共 10 份文件、占位头在位、装出的 workflow 三件套 0 处、新交付文档节在位。
- 教程 §12 的概要设计示例是「开场点名要这份文档」的场景，与点名提前生成口径一致，未动。

## 2026-11-21 补记：交付占位统一到 doc/delivery/（用户点选，未发版）

- 用户问七份为何分两个文件夹；原因为实现便利（概要设计老路径、serve 写死引用），非产品逻辑。用户定统一：七份全进 doc/delivery/，doc/design/ 只留需求列表。
- 已改：scaffold 映射、serve 活动流与设计文档面板的概要设计路径、init 完成提示、templates.go 与 scaffold 注释、workflow×3 交付文档节、brainstorm 路径、两处测试断言、教程中英文各 3 处路径；市场镜像再同步。老路径 doc/design/概要设计.md 全仓残留 0（PLAN 历史记录除外）。老项目已生成的旧路径文件不动（init 只补缺失）。
- 验证：全量 22 包过；init 实测 doc/delivery/ 七份齐、doc/design/ 只剩需求列表。

## v0.5.40 发布记录（2026-11-21）

- 内容：交付文档七份占位（本地 2a5a10d）+ 统一到 doc/delivery/（本地 53ac2d2）。push-initial 全树推送，两笔本地提交合成为一笔远端提交。
- 远端主提交 8c32e611e356b5676ee4e88e40cbfb6b099b21a7，tree 7c0e87fca0ac717de18d64160078648c0a5f5e22，父 3b783c375d45d49fef56073771081243fd44baa8。tag v0.5.40 对象 85b412cd1f03483122d114149fc972bc16a1e7d9。
- run：发布 44865089128、CI(tag) 44865089065、CI(main) 44865071383，均 success。Release id 453432927，6 资产齐全；linux_amd64 sha256 9d354782cd506d0bff8fc4041b2d63675c8487026fabe10b2d798c89f9a6b7a5 与 checksums.txt 一致。
- 实测：线上 install.sh 装上 0.5.40、upgrade --dry-run 报已是最新；发布包 init --type 产品设计生成 doc/delivery/ 七份齐、doc/design/ 只剩需求列表。校验包在 ~/workspace/scratch/release-0540/。

## 2026-11-21 原型超限处置改梯度（用户点选，未发版）

- 起因：用户在 DSH 实跑中看到 prototype 子代理对 548 行页面（超限 48 行）按模板「超了必须拆」写了 478 行共用列表引擎并重写整页，理由是后续页面预计复用——处置成本远超超限量，且为未做页面预抽。
- 改法（模板措辞，18 份 prototype 代理同步）：超限先在汇报写明行数与原因、带着超限交付（不砍功能、不换静态占位的护栏保留）；只有同一块逻辑已在两个及以上页面真实重复出现，才抽共用组件或拆子页；不为还没做的页面预抽共用件。
- 验证：旧措辞残留 0、新措辞 18 份齐、kiro JSON 合法、全量测试全过。

## 2026-11-21 防派发漂移：措辞强化 + 两道留痕门（用户点选，未发版）

- 起因：用户在 DSH 实跑发现主会话有时不派子代理自己干（实证为原型：主会话自写图书列表页并自行重构）。DSH 无 hook 不能强制，定两类措施：强化措辞 + 留痕门（不依赖 hook 的流程自洽检查）。
- workflow×3（市场镜像已同步）：①原型派发段收窄小修例外（只限单处单文件的文字/颜色/间距，不新增区块、不改结构、不抽共用件）并加派发前自检（拿不准算大改）；②1.5 评审加原型产出留痕门：每条需求的原型页须有 sunshine-prototype 产出汇报（页数、mock 数、路径），主会话自写的页视为未完成、补派复核或重做才算过；③派发默认段加自检句（派发指实际发出调用，没派发就动手立即停手补派；channel 版按 spawn 口径）；④对账把关加 check 落盘与归档门。
- check 角色 20 份（代理 19 + check skill 1）：对账段写完同时写入当前任务目录 check.md（覆盖更新）。workflow 侧：汇报有对账段、盘上没 check.md 同样视为检查未完成；归档前先验 check.md，缺了补派落盘后再归档。
- 验证：全量测试全过，kiro JSON 合法，各锚点计数核对无误。
- 备注：截图另见 nav-prototype 子代理 3.6M tok / 26 分钟（导航页，步数膨胀），与本次防漂移无关，已单独向用户指出。

## 2026-11-21 原型验证总量封顶（用户点选，未发版）

- 实证：用户给出 nav-prototype 子代理全程记录（3.6M tok / 26 分钟）：生产约 8 步（写 mock/nav.js/两页），其余 30 多步是验证螺旋——无头 Chrome 沙箱挂死重试、截图成功但模型无图像输入读不了、改名「静态检查」继续 DOM 转储/探针、明知禁令仍写 PNG 像素采样脚本、探针连锁无收口。v0.5.34 的「渲染自检最多一轮」按名目封顶被改名绕过。
- 改法（prototype 代理 18 份）：封顶改为按总量——最后一个文件写完后一切验证动作（行数语法自查、截图、DOM 转储、控制台/错误探针、像素检查等，叫什么名目都算）总共不超过 5 步工具调用；截图打不开或看不了图试一次即跳过；大错只许改一轮、改完不再复验、带问题交付；预算用完立即收尾，不为验证装工具或另写脚本。原「只看图不量像素、不许像素脚本」保留。
- workflow native/tdd 的 DSH 原型派发句补：prompt 注明该环境模型读不了图片、截图自检直接跳过（市场镜像已同步）。channel 版派发段无 DSH 专属句，靠代理定义内「看不了图试一次即跳过」兜底，未加。
- 验证：旧句残留 0、新封顶 18 份齐、kiro JSON 合法、全量测试全过。

## v0.5.41 发布记录（2026-11-21）

- 内容四批：原型超限梯度（c79a7e2）、防派发漂移留痕门（7f11b39）、原型验证总量封顶（568b5a5）、封顶补澄清（3c0c874）。push-initial 全树推送，四笔本地提交合成为一笔远端提交。
- 远端主提交 262e113edd8c2d93a64c081b25c9ab76009f05c7，tree c19bd3a45b222b0d4011409d7937e1c6d9418897，父 7c92a5c4a00f7b990828a5c952fc61a60a7e9196。tag v0.5.41 对象 d1ae433d6ac5fc9041e7951764eaa1e7601147f7。
- run：发布 44877667961、CI(tag) 44877667973、CI(main) 44877650617，均 success。Release id 453503651，6 资产齐全；linux_amd64 sha256 ea47326187ae96eee52257eebcc0114fd17b187dd2954151a7904476c91dfe92 与 checksums.txt 一致。
- 实测：线上 install.sh 装上 0.5.41、upgrade --dry-run 报已是最新；发布包 init 装出的 prototype 含验证总量封顶与超限梯度、workflow 含原型产出留痕门、check 含 check.md 落盘。校验包在 ~/workspace/scratch/release-0541/。

## 2026-11-21 提问轮次边界（共性修复，用户点选，未发版）

- 实证：用户给运动鞋商城开场「只做第一个需求：导航菜单」，DSH 里的 agent 连问 15 题——导航壳在第 7 问已够动手，后 8 问（商品状态、出入库审核/作废/负库存、品牌分类维护等）是后续需求细节被提前一锅烩。用户问是否共性，定性为「轮次边界失守」一类 bug：完整性清单（功能型必问项、缺失项四类）没有范围反制原则，勤快模型会把检查表当整个产品的问卷。
- 改法（总则 + 三落点）：①brainstorm 提问规则加「提问边界（总则）」：只问当前轮次/当前任务范围内的决策，范围外记 [待定] 挂到对应需求/子任务名下，清单是当前条目的检查表不是产品问卷；②workflow×3 设计循环：本轮只问清该条细节，列表先列全条目，其余条目未定项标 [待定] 不展开，共用项首次需要的那轮定；③workflow×3 技术待定分流：开工问答只处理本子任务 [待定]，不顺带澄清其它子任务，新发现的范围外问题挂账；④audit 18 份：必须确认加范围标准——只限挡住当前评审对象交付的，后续需求/其它子任务细节归可延后并注明挂账去向。需求变更处已有「只问 delta」同原则，未动。市场镜像已同步。
- 验证：全量测试全过，kiro JSON 合法，各锚点计数无误。

## 2026-11-21 防跳过子代理三件套 + 全角色扩展（用户点选，未发版）

- 背景：v0.5.41 措辞在 DSH+DeepSeek 全新项目上仍未拦住主会话自写原型（「I'll do this inline (small)」）。复盘通道效力：该轮被遵守的规矩（提问格式、问答记 prd）都经 skill 渠道送达；被绕过的派发只躺在 60KB workflow.md 里。用户追加要求：DSH 上其它子代理（implement/check/research/audit）的活也不能跳过。
- 三件套：①`.sunshine/AGENTS.md` 铁律加两条——「子代理的活不代干」（主会话直接做只限 workflow 写明情形或用户明确要求、须在汇报说明依据、模型不得自批例外）与「原型只出自子代理」（主会话对 doc/prototype/ 只读）。AGENTS.md 每会话注入、仅 559 字节，是最高显著度通道。②例外改用户授权：workflow×3 原型小修例外加「用户明确要求主会话直接改时才可用，模型不得自批」；派发默认段删掉 DSH 的自批 inline 分支（原「改动太小不值得派发」是 implement/check 在 DSH 上最大的自批口子），inline 只属 Codex，DSH 只有用户明确要求才可 inline。③新增 sunshine-prototype skill（common/skills/prototype.md，注册进 autoSkillNames + skillDescriptions，全平台写出、DSH 镜像到 .dsh/skills；reasonix 无 prototype 子代理且同名会与其子代理 skill 混淆，用 ExcludePrototypeSkill 排除）：停手规则 + 填空派发骨架（需求条目/本轮页面/spec 约束/汇报落盘四项）+ 收汇报核对。汇报落盘全角色铺开：prototype 子代理 18 份汇报写入任务目录 prototype-<需求编号>.md，workflow 1.5 留痕门改验落盘文件；audit 18 份清单落盘 audit.md，1.5 缺失项扫描无落盘视为未完成；check 已有 check.md 不动。DSH.md 口径与计数（11 个工作流 skill）同步。
- 验证：全量测试全过（各平台文件数 +1、DSH +2、reasonix 不变，计数断言已更新）；沙箱实装 DSH 项目核对：.agents/skills 与 .dsh/skills 均有 sunshine-prototype/SKILL.md，AGENTS.md 与 workflow.md 新句都在。kiro JSON、codex toml 合法。
- 限度：仍非机制锁（DSH 无 hook）；本批打的是通道显著度、自批例外与落盘回执三个实测失效点。DSH 会话记录事后核验未做。

## v0.5.42 发布记录（2026-11-21）

- 内容：提问轮次边界（brainstorm 总则 + 设计循环/子任务开工/audit 分类三落点）+ 防跳过子代理三件套（AGENTS 铁律、例外改用户授权并删 DSH 自批 inline 分支、新增 sunshine-prototype skill、prototype/audit 汇报落盘与 1.5 门改验落盘文件）。
- 远端主提交 `e4757e19eed53e1ac81863272fc3e8fe44ca3c38`，tree `10b7c5cea57b204633f5098ac4c419253caa697d`，父 `33b176d852ffde7d960ebdaf074123b8e093ec1f`。tag v0.5.42 对象 `529024b75ac8382ede59b290110929dfad2a971e`。
- 波折：首打 tag（指向 33b176d）的 CI 两路失败于 gofmt 检查（platforms.go 新增字段后注释对齐未跑 gofmt）；已删该 Release（id 453592433）与 tag，补 gofmt 提交（本地 1ebaca1）后重打 tag。正式版以重打为准。
- run：发布 44892091144、CI(tag) 44892091163、CI(main) 44892078330，均 success。Release id 453596119，6 资产齐全；linux_amd64 sha256 `bc30fc1dce5b06d462cd4fd5f8bfabd0ca0c3a834726a27a16d144cce8fb7df5` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.42、upgrade --dry-run 报已是最新；发布包 init 的 DSH 项目含 sunshine-prototype skill（.agents/skills 与 .dsh/skills 两处）、AGENTS.md 铁律与 workflow 新口径。校验包在 `~/workspace/scratch/release-0542/`。

## 2026-11-21 还原 DSH inline 分支（用户点选，未发版）

- 真因修正：运动鞋项目「不按工作流」的真因是新项目没先跑 git init——DSH 按最近 .git 祖先定项目根、项目 skill 未被发现，agent 只能读文件即兴发挥（与 demo11/12 同根；init 已有针对性警告与修复步骤，属漏看）。与 v0.5.41/0.5.42 的模板改动无关。
- 逐件复核 v0.5.42 三件套后，用户定：只还原一件——派发默认段恢复 DSH 的 inline 分支（「改动太小不值得派发，或用户要求 inline」），DSH.md 两句口径同步恢复；这是原有设计取舍（DSH 派发拼自包含 prompt 成本高），当初删除它依据的是误诊。其余保留：AGENTS 铁律、sunshine-prototype skill、汇报落盘、原型小修用户授权（书籍商城 skill 正常加载时仍有自批实证）、提问轮次边界。
- 验证：全量测试全过，市场镜像已同步。

## v0.5.43 发布记录（2026-11-21）

- 内容：只还原 DSH 的 inline 分支（派发默认段与 DSH.md 口径恢复到 v0.5.41 的写法）；v0.5.42 其余内容（提问轮次边界、AGENTS 铁律、sunshine-prototype skill、汇报落盘、原型小修用户授权）全部保留。缘由见上一节（真因是新项目未 git init 致 skill 未发现）。
- 远端主提交 `f3c322d2ea355b3fdf241615cdff2da99ca5aec2`，tree `c60c902ec40f7565c58f867afb01a3b22dc0826e`，父 `b1f9036e61f61d529fb2bbeb45541cd356d2eb78`。tag v0.5.43 对象 `bb7fd74093aaaf728afda8a8f83850e136a5aef9`。
- run：发布 44894021773、CI(tag) 44894021810、CI(main) 44894003465，均 success。Release id 453608621，6 资产齐全；linux_amd64 sha256 `f4349f15ef02823e410f656b6301139fc143d1cba97675a1fcb6a75a7e0e3381` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.43、upgrade --dry-run 报已是最新；发布包 init 的 DSH 项目 workflow.md 含还原后的 DSH inline 分支、DSH.md 口径已恢复，sunshine-prototype skill 与 AGENTS 铁律仍在。校验包在 `~/workspace/scratch/release-0543/`。

## 2026-11-21 主会话交付验收封顶（用户点选，未发版）

- 背景：demo19（服装后台）R-001 实跑，prototype 子代理侧全部正常（派发自包含、汇报落盘 prototype-R001.md、写后验证仅 wc 与 node --check 两条命令）；但主会话在子代理交付后逐项复核内部接线（导航链接、data-page 键、tab 键、hash、快捷入口，约 6 轮命令），并尝试无头 Chrome 截图验证（失败、重试、挂起、取消）——验证螺旋从子代理转移到主会话。现行验证封顶只约束子代理，未约束主会话的验收深度；且 DSH 上模型无图像输入，截图本就白截（书籍商城已实证）。
- 改法：workflow×3 在派发协议处加「交付验收封顶」通用口径——主会话验收任何子代理/工作线程的交付只做三件事：读落盘汇报、按汇报核对产物清单、抽查一个产物；不逐项复核内部接线与细节（那是实现阶段 check 的对账活），不做渲染/截图验证（DSH 明确跳过）；发现问题记下来退回对应子代理修，没有落盘汇报的交付视为未完成。市场镜像同步。
- 验证：全量测试全过；本地构建二进制 init 的 DSH 项目 `.sunshine/workflow.md` 含「交付验收封顶」段（恰 1 处），市场镜像与模板逐字节一致。

## v0.5.44 发布记录（2026-11-21）

- 内容：主会话交付验收封顶——workflow×3 派发协议加「交付验收封顶」段（验收只做读落盘汇报、按汇报核对产物清单、抽查一个产物；不逐项复核内部接线与细节、不做渲染/截图验证；发现问题退回对应子代理修；无落盘汇报视为未完成），市场镜像同步。缘由见上一节（demo19 主会话验收膨胀实证）。
- 远端主提交 `df314f24f5ecc1511620a13b6cb2d0aac5d5864e`，tree `f736f3dbf92435327738ddaa5d6a07429c5396dc`，父 `df6975edf3a88d1ceb18a446c64ce50bba3113bb`。tag v0.5.44 对象 `bec1388619bc6a7760da08156764edf93cbc15c4`。
- run：发布 44896648311、CI(tag) 44896648425、CI(main) 44896627336，均 success。Release id 453626383，6 资产齐全；linux_amd64 sha256 `4134a0edc5ffb34300436a132730e34ccad6c084f2dcdf303456cd33f9e31375` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.44、upgrade --dry-run 报已是最新；发布包 init 的 DSH 项目 workflow.md 含「交付验收封顶」（恰 1 处）。校验包在 `~/workspace/scratch/release-0544/`。

## 2026-11-21 提问走结构化工具（用户点选，未发版）

- 背景：mall-admin 新项目实跑，agent 把问题写成纯文本、用户只能打字回字母；用户指出此前在 DSH 都是弹窗点选。查证：DSH 有原生 `ask_user_question` 工具（官方工具目录在册），客户端渲染为提问弹窗，无弹窗的会话才退化成普通消息提问——是否弹窗取决于 agent 是否调用工具。此前助理把「DSH 勾选框渲染改不了」过度推广成「DSH 无可点选控件」，已向用户更正、MEMORY 已订正。根因：brainstorm 措辞只规定问题内容与格式，从未要求用结构化提问工具，弹窗与否全凭模型当轮自觉。
- 改法（用户点选，全平台适用）：brainstorm「提问规则」加「提问走结构化工具」明文——平台提供结构化提问工具时必须用工具提问，不许纯文本让用户打字回（Claude Code 用 AskUserQuestion、DSH 用 ask_user_question、其它平台有等价工具同理；格式不变；确实没有工具的平台才用纯文本）。落点：common brainstorm + copilot prompt 变体；workflow×3 按需求循环段加一句指针（见 brainstorm 提问规则），市场镜像同步。
- 验证：全量测试全过；本地构建 init 实测：DSH 项目 .agents/skills 与 .dsh/skills 两处 brainstorm 均含新段、workflow.md 含指针句，Claude 项目 brainstorm 含新段；市场镜像与模板逐字节一致。

## v0.5.45 发布记录（2026-11-21）

- 内容：提问走结构化工具——brainstorm 两副本「提问规则」加「提问走结构化工具」明文（平台提供结构化提问工具时必须用工具提问：Claude Code 用 AskUserQuestion、DeepSeek Harness 用 ask_user_question、其它平台有等价工具的同理；确实没有工具才用纯文本），workflow×3 按需求循环加指针句，市场镜像同步。缘由见上一节（mall-admin 纯文本提问实证）。
- 远端主提交 `645e393f0555a063446153d927653290c7241146`，tree `4e3f61422a42f7a0092eeb335f3b628471a13cba`，父 `778eb777dae84c0f385232c18f44dd39944f7236`。tag v0.5.45 对象 `e678804f7645ef9387159d675d2b5b66d05de15d`。
- run：发布 44907655541、CI(tag) 44907655529、CI(main) 44907632878，均 success。Release id 453702178，6 资产齐全；linux_amd64 sha256 `597defc7fe4dca956f888446e1553e10215c414bc753365a73967d69a0fbe80a` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.45、upgrade --dry-run 报已是最新；发布包 init 的 DSH 项目 brainstorm（.agents/skills 与 .dsh/skills 两处）含「提问走结构化工具」、workflow.md 含指针句。校验包在 `~/workspace/scratch/release-0545/`。

## 2026-11-21 1.5 提问口径补齐（用户点选，随 v0.5.46 发布）

- 背景：v0.5.45 发布后用户在 mall-admin 实跑 1.5 评审：audit 出完清单，主会话呈报「必须确认」时仍是纯文本列选项、没弹 ask_user_question。查 workflow 1.5：缺失项扫描与深度逼问两段只写「当场逐条问完」，没写提问格式、也没指向 brainstorm——v0.5.45 的结构化工具明文只落在 brainstorm 与设计循环，1.5 呈报不在覆盖内（该项目当时也尚未 update 到 0.5.45 模板）。
- 改法（用户点选）：workflow×3 的 1.5 缺失项扫描段在「当场逐条问完」后加全口径（向用户提问按 brainstorm 提问规则：一次只问一个问题、给 2–4 个选项加「自己填写」；平台提供结构化提问工具时必须用工具提问，不用纯文本列选项）；深度逼问段加指针句（提问口径同扫描）。市场镜像逐字节同步。
- 验证：全量测试全过；本地构建 init 实测 DSH 项目 workflow.md 两处新口径在位；发布包复验见发布记录。

## v0.5.46 发布记录（2026-11-21）

- 内容：1.5 提问口径补齐——workflow×3 缺失项扫描段在「当场逐条问完」后加全口径（向用户提问按 brainstorm 提问规则：一次只问一个问题、给 2–4 个选项加「自己填写」；平台提供结构化提问工具时必须用工具提问，不用纯文本列选项），深度逼问段加指针句（提问口径同扫描），市场镜像逐字节同步。缘由见上一节（mall-admin 1.5 呈报仍纯文本实证）。
- 远端主提交 `6263a518233b6e9a6d11a5bae9c9df9e81f60553`，tree `4ceb801615b791daf9a86d21a17c7c840839acbb`，父 `7bf513fb468fe1e096813783da1e2f2d1541c0b3`。tag v0.5.46 对象 `89689ad16ac3f5ca9349aba46a586e03bd8b2cc9`。
- run：发布 44910889390、CI(tag) 44910889419、CI(main) 44910868819，均 success。Release id 453727413，6 资产齐全；linux_amd64 sha256 `aaff99fce7447426a6170e2d630dd42edd836e1d88a35e976437891a5ca0a6cf` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.46、upgrade --dry-run 报已是最新；发布包 init 的 DSH 项目 workflow.md 两处 1.5 新口径与 brainstorm「提问走结构化工具」均在位。校验包在 `~/workspace/scratch/release-0546/`。

## 2026-11-21 audit 业务+技术双角度逼问到极限（用户点选，未发版）

- 背景：用户问 audit 在哪个阶段做、能否全审出来；答复后用户要求「从业务和技术角度逼问到极限」。现状核查：audit 业务侧已深（四类缺失+功能型必问项八类+默套句式+可测性+负边界+深度逼问逐条），技术侧基本空白（模板明文不读 spec/不看实现；技术未定项只靠 1.2 技术待定分流被动接收）。
- 方案（用户点选推荐方案）：①audit 加「技术型必问项」八类（选型与外部依赖、接口契约、数据口径、并发与一致性、性能与规模、安全、异常与恢复、环境约束），与功能型并列，普通扫描轻核、深度逼问逐条逼；只标未定点、不替用户选型。②深度逼问改收敛式：逐轮深查直到一整轮无新增才停、上限 4 轮、汇报注明轮数（普通扫描的「再做两遍」不动）。③技术项分阶段分级：设计先行父任务评审时一律可延后（标影响、挂技术选型子任务转 [待定]，守「设计阶段不问技术栈」）；开发先行/子任务开工前挡住实现的进必须确认。
- 落点：audit 模板 18 份（md/toml/json/copilot 变体）+ workflow×3 1.5 口径同步（扫描段轻核一句、深度逼问段技术维度+收敛轮次+设计阶段挂账一句），市场镜像同步。
- 验证：全量测试全过；kiro JSON 与 codex TOML 改后解析正常；本地构建 init 实测 Claude/Codex/Kiro 的 audit 文件含技术型必问项与收敛式深查、DSH workflow 含收敛口径；市场镜像逐字节一致。

## v0.5.47 发布记录（2026-11-21）

- 内容：audit 双角度逼问到极限——audit 18 份加「技术型必问项」八类（选型与外部依赖、接口契约、数据口径、并发与一致性、性能与规模、安全、异常与恢复、环境约束；只标未定、不替选型），深度逼问改收敛式（逐轮深查直到一整轮无新增、上限 4 轮、汇报注明轮数），技术项分阶段分级（设计先行一律可延后挂技术选型子任务，开发先行/子任务开工前挡实现的进必须确认）；workflow×3 1.5 同步（扫描轻核句、深度逼问技术维度+收敛+设计阶段挂账），市场镜像同步。缘由见上一节。
- 远端主提交 `50fe48a56416ab26c507f97f7603f3cf84080511`，tree `745c7335f311216fc6ecb2c288b172672eb0d41f`，父 `50727e64bff4da77e46a6e1d2a02ff93b1749f40`。tag v0.5.47 对象 `6991ce92e8b838215bdedd0ad9cd17d09f1819c4`。
- run：发布 44917001935、CI(tag) 44917001824、CI(main) 44916978128，均 success。Release id 453770602，6 资产齐全；linux_amd64 sha256 `7ff490d87a6e89920ceed596995267dc564cf28a6787dc76e11d4c8040697b26` 与 checksums.txt 一致。
- 实测：发布包二进制报 0.5.47、upgrade --dry-run 报已是最新；发布包 init 的 Claude 项目 audit 含技术型必问项与收敛式深查、workflow 含收敛口径。校验包在 `~/workspace/scratch/release-0547/`。
