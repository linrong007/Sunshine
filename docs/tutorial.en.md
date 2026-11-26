# Sunshine tutorial: init once, then work in the agent UI

The main flow in one line: **run `sunshine init` once in the terminal, then work entirely in the agent UI (Claude Code first, Codex / Cursor the same idea) in plain language: you state the requirement, and the agent follows `.sunshine/workflow.md` — questions first, a written plan, and implementation only after the 1.5 review passes with your confirmation (design-first splits first, then one sub-task at a time); each task runs implement, check, spec write-back, commit, and archive end to end, and you appear only to answer questions, confirm the review and the commit plan, and accept the result.**

How to read this tutorial: section 1 walks the main flow end to end; section 2 covers the two modes and the review; the capability sections after that (specs, memory, token saving, collaboration, maintenance) are there to look up as needed; section 9 is the record of six real scenarios, and the appendix is the command reference. Command transcripts live in section 3 and section 9; the rest is flow and capabilities.

The demo project is the same throughout: **refund reason required** — add a mandatory reason field to the refund flow. The example is small; the workflow is real.

## Contents

- [1. Five minutes in: walk the main flow](#1-five-minutes-in-walk-the-main-flow)
- [2. Two modes: design-first and development-first](#2-two-modes-design-first-and-development-first)
- [3. Under the hood: what the agent actually ran](#3-under-the-hood-what-the-agent-actually-ran)
- [4. Specs: the project contract](#4-specs-the-project-contract)
- [5. Memory: mem and long-term memory](#5-memory-mem-and-long-term-memory)
- [6. Saving tokens: the three-piece set](#6-saving-tokens-the-three-piece-set)
- [7. Multi-agent collaboration](#7-multi-agent-collaboration)
- [8. Maintenance: update and upgrade](#8-maintenance-update-and-upgrade)
- [9. Scenario records: from day one to daily work](#9-scenario-records-from-day-one-to-daily-work)
  - [9.1 Starting a new project from zero (development-first): a clothing-store admin system](#91-starting-a-new-project-from-zero-development-first-a-clothing-store-admin-system)
  - [9.2 Adopting an existing project: a clothing-store admin system](#92-adopting-an-existing-project-a-clothing-store-admin-system)
  - [9.3 Requirement changes: stock 0 no longer means off shelf](#93-requirement-changes-stock-0-no-longer-means-off-shelf)
  - [9.4 Refactoring a legacy module: pin the behavior first, then change the code](#94-refactoring-a-legacy-module-pin-the-behavior-first-then-change-the-code)
  - [9.5 Fixing a bug that keeps coming back: find how it was fixed last time](#95-fixing-a-bug-that-keeps-coming-back-find-how-it-was-fixed-last-time)
  - [9.6 Design-first: requirements → design documents → breakdown → development](#96-design-first-requirements--design-documents--breakdown--development)
- [Appendix: command reference](#appendix-command-reference)

## 1. Five minutes in: walk the main flow

The main flow has five steps: init once → state the requirement → the agent asks questions and writes a plan → implementation after the review passes → you accept and it archives. Here it is step by step, on a small change: adding a mandatory reason field to the refund flow.

### 1.1 Run once in the terminal: init

Install Sunshine first (see [README.md](../README.md#installation)), then confirm it. On Windows, the current one-line install runs in PowerShell (the counterpart of the Unix curl one-liner):

```powershell
irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex
```

To install a specific version, set `$env:SUNSHINE_VERSION = 'vX.Y.Z'` first and run the same line; the script installs into `%LOCALAPPDATA%\Sunshine\bin` by default and adds that directory to the user PATH (Windows x64 only). The old way still works: download the windows zip from the [Releases](https://github.com/linrong007/Sunshine/releases) page, unzip it, and put `sunshine.exe` on your PATH; with Go installed, `go install github.com/linrong007/Sunshine/cmd/sunshine@latest` works too.

> If github.com is unreachable on your network and downloads time out: the install script, `sunshine upgrade` and `sunshine update` all fall back to public mirror proxies automatically — only fetching the script itself needs a manual mirror hop:

> ```sh
> curl -fsSL "https://ghfast.top/https://raw.githubusercontent.com/linrong007/Sunshine/main/install.sh" | sh
> ```

```sh
sunshine version
# Sunshine 版本 0.2.1

sunshine --help
# Sunshine 把规范、任务与项目记忆持久化进仓库，
# 让 AI 编程助手按团队的工程标准工作，而不是每次从零猜测。
#
# 先用 sunshine init 初始化当前项目，再让 AI 助手按
# .sunshine/workflow.md 推进任务。
# ……
```

(The help text says, in English: "Sunshine persists specs, tasks, and project memory into the repository, so the AI programming assistant works to the team's engineering standards instead of guessing from scratch each time. First run `sunshine init` to initialize the current project, then let the AI assistant drive tasks following `.sunshine/workflow.md`.")

Now initialize the demo project — the only terminal command you need before switching to the agent UI:

```sh
mkdir demo && cd demo
git init
sunshine init -u 演示者 -y --claude --cursor
```

Real output:

```text
正在配置 Claude Code……
正在配置 Cursor……
初始化完成，开发者：演示者
项目类型：全栈（默认）
已安装平台：
  - Claude Code
  - Cursor
下一步：
  1. 在 agent 界面里打开项目，直接用自然语言提第一个需求；
  2. 规范已按默认模板生成在 .sunshine/spec/（已生成 system.md、frontend.md、backend.md；有团队规范时再补充），工作流按 .sunshine/workflow.md 自动推进，一般不用改。
```

(`-u` is the developer name; `-y` skips all prompts and uses the defaults. In English: "Configuring Claude Code… / Configuring Cursor… / Initialization complete, developer: 演示者 / Project type: 全栈 (default) / Installed platforms: Claude Code, Cursor / Next steps: 1. Open the project in the agent UI and state your first requirement in plain language; 2. The specs have been generated in .sunshine/spec/ from the default templates (generated: system.md, frontend.md, backend.md; add your team's specs if you have them); the workflow runs automatically following .sunshine/workflow.md and normally needs no changes.")

The demo directory is empty, so nothing is detected; with `-y` skipping the prompt it defaults to fullstack, which is why the output above says `项目类型：全栈（默认）` ("Project type: fullstack (default)"). `init` requires at least one platform (none checked by default) and generates the Chinese `.sunshine/` skeleton with the selected platforms' configs.

**Interactive mode (no `-y`) asks five things in order** — the demo above skipped them all with `-y`:

1. **Platforms**: a multi-select, nothing checked by default; at least one must be picked to continue (passing a flag such as `--claude` skips the question).
2. **Statusline confirmation**: asked only when Claude Code is selected — whether to install the statusline (shows the current task and more in the UI); when Codex is selected, a separate question asks whether init may write its hooks switch for you.
3. **Spec template**: start from scratch with the default templates, or pick a ready-made template from a template marketplace (a custom source can be entered too). A marketplace template becomes this project's spec and is recorded in `.sunshine/config.yaml` under `registry.spec`, so `sunshine update` keeps refreshing it afterwards (section 4); passing `--registry` / `--template` skips this menu.
4. **Project type**: detection first — init reads `package.json` dependencies and common config files to decide frontend / backend / fullstack, and a detected type is used directly, with the type and its source printed in the output; only when nothing is detected does a menu ask. The four types generate different things: frontend generates `system.md` + `frontend.md`, backend generates `system.md` + `backend.md`, fullstack generates all three; **product design** never takes part in detection and can only be chosen by hand (in the menu, or `--type 产品设计`) — it generates the three specs plus, under the project's `doc/`, the requirements list and prototype placeholders and the seven delivery-document placeholders under `doc/delivery/`, for design-first projects. With a marketplace template there is no detection and no type question (the template is the spec); with `-y` and nothing detected, the default is fullstack.
5. **CodeGraph**: this one comes after the files are written, and only when the tool is not installed yet and this is the first init — whether to install the code-map tool (about 58 MB). On a yes it installs on the spot, and a project that already has code gets its graph built too; a new project gets the tool only. A no is recorded and never asked again (details in the "Code map: CodeGraph for existing projects" section below).

From here on, close the terminal and ask for your first requirement in the agent UI (section 1.2). All init operations — the interactive questions, the common options, the three modes, project-type detection, and what gets installed — are in the [appendix: command reference](#appendix-command-reference) at the end.

### 1.2 Open the agent UI: ask in plain language

Open the project in Claude Code (Codex and Cursor work the same way — the agent there reads the same `.sunshine/` files). The agent already knows the project conventions from `AGENTS.md`, `spec/`, and `workflow.md`; you just describe what you want.

> **Example prompt** (an illustration of what to say, not a real session transcript):
>
> Add a required reason field to the refund flow: when a user submits a refund, the reason must not be empty or the submission is rejected. The reason input is a dropdown — quality issue / wrong size / other — and choosing "other" reveals a text box. On the backend, `POST /refunds` requires `reason`; a missing reason returns 400 with error code `REASON_REQUIRED`. Please follow the Sunshine workflow.

Note: the prompt must name the workflow explicitly ("follow the Sunshine workflow", plus the mode if there is one, e.g. development-first). The steps after that — ask first, create the task and write the PRD on confirmation, review, then implement — do not need restating: they are all in workflow.md, and the agent follows them in order once it has read it. A bare requirement on its own is not enough — in a real run the agent jumped straight to coding and skipped task creation and requirement confirmation — but the problem there was the unnamed workflow, not the unrestated steps.

The designer and the developer are not two workflows — they are two phases of one workflow: the designer does 1.1 (clarify requirements), the design deliverables, 1.5 (review), and 1.2 (breakdown); the developer does phase 2 (implement and check) and phase 3 (write back and commit). They share one workflow.md, one task system, and one PRD, and the handoff runs through the requirements list and the task structure. So the prompt does not need to name a role — naming the workflow and the mode is enough, and the role follows the phase: while the task is in the design phase, the agent is the designer; once it moves into development, the agent is the developer. If the mode is unclear, the agent asks first in 1.1. The only thing to state explicitly is the scope boundary: design only, no code, or the design is done and only development remains.

You do not need to mention commands. The prompt names the workflow explicitly because a bare requirement on its own lets the agent jump straight to coding. Once the flow is named, the agent follows it: if the request is big enough to be a task, `workflow.md` requires it to ask for your agreement before creating one — it will not silently create a task directory. It may also ask clarifying questions first (for example, whether "other" needs a minimum text length), and it will tell you what it is about to do before doing it.

Before the work starts, the demo project's conventions need to live in the specs — either you add them, or you ask the agent to ("Put the refund conventions into the spec files"). Either way they end up as three short additions:

`.sunshine/spec/system.md` — the general principle:

```text
## 演示项目约定

- 退款原因必填：用户提交退款时原因字段不能为空，否则拒绝提交。
```

(`system.md` is the spec the agent itself maintains in the background; see [section 4](#how-specs-are-maintained).)

`.sunshine/spec/backend.md` — the API contract:

```text
## 退款接口

- `POST /refunds`：`reason` 必填，缺失返回 400，错误码 `REASON_REQUIRED`。
```

`.sunshine/spec/frontend.md` — the form behavior:

```text
## 退款表单

- 原因输入框为下拉选择（质量问题 / 尺寸不符 / 其他），选“其他”时出现文本框。
```

(In English: system — "Demo project convention: the refund reason is required — submitting a refund with an empty reason is rejected"; backend — "`POST /refunds`: `reason` is required, missing it returns 400 with error code `REASON_REQUIRED`"; frontend — "The reason input is a dropdown (quality issue / wrong size / other); choosing 'other' reveals a text box.")

Once the specs say it, every later task follows it automatically — that is the point of writing specs down instead of repeating the rule in every prompt.

### 1.3 The agent works through workflow.md

This is what the agent does after you give it the request. You watch it happen in the UI; you do not type any of this.

**Phase 1 — planning.** The agent creates the task directory `.sunshine/tasks/11-10-给退款原因加必填校验/` and writes `prd.md` from a template:

```markdown
# 给退款原因加必填校验

## 目标

退款时原因字段必填

## 需求

- 待定

## 验收标准

- [ ] 待定
```

("Target: the refund reason field is required when refunding.") Then it fills in the real requirements and acceptance criteria. A light task can stop at the PRD; a complex task also gets `design.md` (technical design) and `implement.md` (execution plan) before anything is implemented. It also organizes the context lists `implement.jsonl` / `check.jsonl` — the spec and research files the sub-agents will read. For a complex task it asks you to review the plan before starting implementation; only then does it move the task to `in_progress`.

There are two modes, decided at the start of planning: design-first, where the product design documents (requirements list, outline design, high-fidelity prototype — whatever the project needs, if any) land in the project's `doc/` folder first and the work is then split into sub-tasks, or direct development, where the task goes straight from the PRD to implementation. The design documents live in `doc/`, not in `.sunshine`; `.sunshine` only keeps the index (the task record, `prd.md`, and the `implement.jsonl` / `check.jsonl` lists) pointing at them. Section 9.6 walks the design-first mode end to end.

**Phase 2 — execution.** The agent dispatches sub-agents: `sunshine-implement` writes the code, then `sunshine-check` verifies it. The platform hooks inject the context automatically — the `implement.jsonl` / `check.jsonl` entries, plus `prd.md`, `design.md`, and `implement.md` if they exist — so each sub-agent starts with the right specs in its prompt. The dispatch prompt always opens with `Active task: <task path>`. The fixed order in `workflow.md` is:

```text
sunshine-implement -> sunshine-check -> sunshine-update-spec -> commit (step 3.4) -> /sunshine:finish-work
```

(`sunshine-implement` → `sunshine-check` → `sunshine-update-spec` → commit (step 3.4) → `/sunshine:finish-work`.)

**Phase 3 — wrap-up.** If the same problem needed repeated fixes, the agent loads the `sunshine-break-loop` skill and records the root cause. It then loads `sunshine-update-spec` and asks whether this task produced anything worth writing back into `.sunshine/spec/` — a new pattern, a pitfall, a technical decision — and updates the specs if so, even if the answer is "nothing to update" (it still has to make that judgement). It commits the code changes, and finally runs `/sunshine:finish-work`, which archives the task and records the session in the developer journal.

Your part during all of this: read what the agent says it is doing, answer its questions, and ask it to change course when it goes the wrong way. The commands it runs are listed in the next section — for your understanding, not for you to type.

### 1.4 You review and accept

The agent does the work; you decide whether it is done. When it says the task is finished, review it in the UI the same way you would review a colleague's change:

- Read the diff the agent shows you (or run `git diff` yourself) — the refund form change, the backend validation, and nothing else. `workflow.md` has an explicit rule for this: only change the affected parts; untouched code and styles stay untouched.
- Check it against the acceptance criteria in `prd.md` — the criteria you agreed in phase 1 are the definition of done, not the agent's summary.
- Run the thing yourself if it matters: submit a refund with an empty reason and confirm it is rejected; pick "other" and confirm the text box appears.

If something is off, say so in plain language and the agent fixes it — it is still the same task, still the same conversation:

> **Example prompt** (an illustration of what to say, not a real session transcript):
>
> The reason dropdown works, but choosing "other" doesn't show the text box yet. Fix that, and keep the backend validation as it is.

When you accept, the commit has already happened in workflow step 3.4 — the agent batches the code changes into commits before `/sunshine:finish-work` runs, and the finish command itself does no code committing (it only archives the task and records the journal). So "accept" means: the diff is right, the criteria are met, and the commits are already in your branch. Push when you are ready, the same as any other work.

## 2. Two modes: design-first and development-first

When the requirement needs exploring first, or a design has to be shown to people, go design-first; when the requirement is clear and the change is small, go development-first. Say which one in the opening prompt; if you don't, the agent asks first. Both orders are fixed — they do not change with how the opening prompt is worded.

### Design-first

The order: create one parent task, never start it → the per-requirement loop (1.1: one requirement per round — ask it clear, record it in the requirements list, dispatch a prototype sub-agent to build or update its page (page files carry the requirement ID, e.g. `R-001-product-list.html`, using the requirement's name verbatim — no invented shorthand); the design phase never asks about the tech stack) → at the end of the loop, ask only "shall we enter the review", never about splitting → split (1.2, one sub-task per requirement, the first being stack choice and scaffolding) only after the review (1.5) passes → each sub-task runs implement → check → spec write-back → commit → archive in turn, and the parent task closes with a final integration review.

The review examines the design package: prd, requirements list, design documents, and prototype must all line up. A requirement change does not re-run the whole flow: edit the design documents and the affected prototype pages first, record a [变更] in the parent prd, and the review covers only the change. Record: 9.6.

### Development-first

The order: clarify the requirement (1.1, one question at a time, each answer recorded in prd.md at once) → write the planning products (a light task has only prd.md; a complex one adds design.md and implement.md) → split only if the scope is too large → set up context (1.4) → review (1.5) → task start → development.

The review sits right before start: first check the planning products are self-consistent, then dispatch an audit scan, ask every must-confirm question on the spot and record the answers in prd — implementation starts only after you confirm. A single task starts right after its review; with sub-tasks, each one passes its own review before its own start, and the parent task is never reviewed as a whole and never started. Record: 9.1.

### The audit in the review, and risk tiers

Besides the main session's own check, the 1.5 review dispatches a read-only audit sub-agent to scan: it only reports items that were never confirmed and changes no files, and its findings land in the task directory's `audit.md` — no such file, and the review counts as not done. Answers go back into prd.md; anything undecided is marked `[待定]`.

Each audit item also carries a kind: business decision (money and judgement rules, business rules, permissions, external commitments, whether data gets deleted) or technical. When the review wraps up, every item gets an outcome, written back into audit.md as its disposition: what you decided on the spot is recorded as "user-decided", a suggested option you picked is "user-accepted", and anything else stays `[待定]`; conclusions copied into prd.md carry the same source tag. An item you were never asked about may not be written up as a conclusion — business decisions above all: without a source from you it does not count as decided, and when development reaches one it must stop and ask you, never silently follow the audit's suggestion.

Tasks carry one of two risk tiers. A light-tier task runs the flow as is; a standard-tier task switches the review to deep interrogation — round after round of questions from both the business and the technical angle (the technical side has eight must-ask categories: stack and dependencies, interface contracts, data definitions, concurrency and consistency, performance and scale, security, failure recovery, environment constraints), until a round adds nothing new, at most 4 rounds. The tier is set on each sub-task at split time (on the task itself for a single task), chosen by blast radius and uncertainty.

A light-tier task takes the **fast lane**: display-only changes, copy, styling, config defaults — small stuff like that (in dev-first the tier can be set when the task is created; saying "small change" works too). In the fast lane the requirement is clarified in one line into the PRD and work starts — no context assembly, no review, the main session edits directly; but check is still dispatched as usual, and verification slips still apply. If halfway through it turns out to touch standard-tier ground (data, judgement logic), it upgrades on the spot and the skipped steps are made up.

### Switching workflow templates

There are three built-in workflow templates. You usually switch by asking the agent in the UI ("Switch the workflow to TDD for the next task"), and the agent runs the command; running it yourself in the terminal works the same way:

```sh
sunshine workflow --list
#
# 可用的工作流模板：
#
#   native（内置） — 原生 Sunshine 工作流
#     默认的 Sunshine 规划 / 执行 / 收尾工作流，含原生子代理与 inline 平台分支
#   tdd（内置） — TDD 工作流
#     用一次一个红灯 / 绿灯 / 重构行为切片驱动第 2 阶段的 Sunshine 工作流变体
#   channel-driven-subagent-dispatch（内置） — Channel 驱动的子代理派发
#     主会话通过 sunshine channel 协调 implement/check 工作线程的工作流变体
```

("Available workflow templates: `native` (built-in) — the native Sunshine workflow: the default plan / execute / wrap-up workflow with native sub-agents and inline platform branches; `tdd` (built-in) — TDD workflow: a variant that drives phase 2 with one red / green / refactor behavior slice at a time; `channel-driven-subagent-dispatch` (built-in) — channel-driven sub-agent dispatch: a variant where the main session coordinates implement/check worker threads through `sunshine channel`.")

Switch to the TDD workflow:

```sh
sunshine workflow --template tdd
#   ✓ 已把 .sunshine/workflow.md 换成 "tdd"
```

("Switched `.sunshine/workflow.md` to `tdd`.")

Switching back to native stops you first — because `workflow.md` is now the tdd version, which no longer matches the hash manifest, the CLI treats it as "locally modified":

```sh
sunshine workflow --template native
# 错误：.sunshine/workflow.md 已有本地修改，请加 --force 覆盖，或用 --create-new 写 .sunshine/workflow.md.new

sunshine workflow --template native --force
#   ✓ 已把 .sunshine/workflow.md 换成 "native"

sunshine workflow --template native --create-new
#   + 已写入 .sunshine/workflow.md.new（workflow.md 未改动）
```

("Error: `.sunshine/workflow.md` has local modifications; add `--force` to overwrite, or use `--create-new` to write `.sunshine/workflow.md.new`. / Switched `.sunshine/workflow.md` to `native`. / Wrote `.sunshine/workflow.md.new` (`workflow.md` untouched).")

The rule: unmodified files are overwritten directly; modified or unknown-origin files ask interactively (overwrite / write a `.new` copy / skip), and in a non-interactive terminal you use `--force` to overwrite or `--create-new` to write a copy. `--create-new` only writes `.sunshine/workflow.md.new` and leaves the active `workflow.md` alone. The same three-state logic is what `update` uses in the next section — a switched workflow shows up there under "你已修改" ("modified by you").

## 3. Under the hood: what the agent actually ran

These are the commands the agent actually ran for the demo task, with real outputs captured by running the equivalent commands. You are reading them to understand the mechanism; in normal use you never type them.

### Creating the task

```sh
sunshine task create "给退款原因加必填校验" --description "退款时原因字段必填" --priority P1
```

Real output (excerpt):

```text
警告：无法解析仓库默认分支（未配 remote 或离线），base_branch 暂记为当前检出分支 "master"，可用 --base-branch 覆盖。
已创建任务：11-10-给退款原因加必填校验

下一步：
  - 在 prd.md 里写清需求与验收标准
  - 轻量任务：只有 PRD 也可以
  - 复杂任务：在 sunshine task start 前补充 design.md 与 implement.md
  ……

.sunshine/tasks/11-10-给退款原因加必填校验
```

(The warning says the default branch could not be resolved — no remote or offline — so `base_branch` is recorded as the current checkout branch `master` for now, overridable with `--base-branch`. The "next steps" say: write requirements and acceptance criteria in `prd.md`; a light task can have only a PRD; a complex task should add `design.md` and `implement.md` before `task start`; organize `implement.jsonl` / `check.jsonl` — one JSON object per line, `{"file": "<path>", "reason": "<reason>"}`, spec/research docs only.)

Give the task name a word that tells it apart — put the distinguishing word from the title into the name itself rather than differing from a sibling by one root word. When a new name is too close to an existing task (one contains the other, or they differ only by generic words like management or list), `task create` warns you on the spot and names the look-alike; heed it and rename, so references stay unambiguous later. And when you refer to something by a short spoken name ("change the order detail page"), the agent checks it against the task list and the requirements list first; if several candidates match, it lists them and asks you to pick — it never guesses.

`create` prepends the `MM-DD-` date to the task name automatically — you never type the date yourself. `--description` is required; omitting it fails immediately:

```text
错误：--description 必填
请传入 --description "<该任务交付什么>"（纯空白不算）：空描述的任务会在归档时被拒绝。
```

("Error: `--description` is required. Pass `--description "<what this task delivers>"` (blank does not count): a task with an empty description is refused at archive time.")

`create` writes `task.json` with 24 fields in a fixed order (shown here as archived — `status` becomes `completed` and `completedAt` is filled in at archive time):

```json
{
  "id": "给退款原因加必填校验",
  "title": "给退款原因加必填校验",
  "description": "退款时原因字段必填",
  "status": "completed",
  "priority": "P1",
  "creator": "演示者",
  "assignee": "演示者",
  "createdAt": "2026-11-10",
  "completedAt": "2026-11-10",
  "branch": "master",
  "base_branch": "master",
  "children": [],
  "parent": null,
  "meta": {}
  ……（其余字段：name、dev_type、scope、package、worktree_path、commit、pr_url、subtasks、relatedFiles、notes）
}
```

### Organizing the context lists

The agent registers the files the sub-agents need — `implement` for files to read while implementing, `check` for files to check against while verifying:

```sh
sunshine task add-context 11-10-给退款原因加必填校验 implement src/Refund.vue "实现退款原因必填校验"
# 已添加文件：src/Refund.vue

sunshine task add-context 11-10-给退款原因加必填校验 check .sunshine/spec/backend.md "退款接口契约"
# 已添加文件：.sunshine/spec/backend.md

sunshine task list-context 11-10-给退款原因加必填校验
# [implement.jsonl] 1. src/Refund.vue → 实现退款原因必填校验
# [check.jsonl]     1. .sunshine/spec/backend.md → 退款接口契约

sunshine task validate 11-10-给退款原因加必填校验
# === 校验上下文文件 ===
# 目标目录：/tmp/rewrite-en/demo/.sunshine/tasks/11-10-给退款原因加必填校验
#
#   implement.jsonl：✓（1 条）
#   check.jsonl：✓（1 条）
#
# ✓ 全部校验通过
```

("已添加文件" = "file added"; "上下文文件" = "context files"; "校验上下文文件" = "validating context files"; "全部校验通过" = "all checks passed".) The lists are stored as one JSON object per line, for example `{"file":"src/Refund.vue","reason":"实现退款原因必填校验"}`.

If the context lists are still empty, `start` refuses outright and tells the agent what to do:

```text
错误：implement.jsonl 和 check.jsonl 没有已整理条目
子代理（implement/check）将得不到任何规范上下文。
  整理：sunshine task add-context 11-10-给退款页加返回按钮 implement <路径> "<原因>"
  检查：sunshine task validate 11-10-给退款页加返回按钮
  确实要空着：重新运行 start 并加 --allow-empty-context
```

("Error: `implement.jsonl` and `check.jsonl` have no organized entries; the implement/check sub-agents will get no spec context. Organize with… / check with… / if you really want it empty, re-run start with `--allow-empty-context`.")

### Starting, viewing, finishing

```sh
sunshine task start 11-10-给退款原因加必填校验
```

Real output:

```text
ℹ 会话身份不可用；本次不持久化当前任务指针（降级模式），AI 仍可按对话上下文继续。
提示：在能提供会话身份的 AI 会话内运行，或先设置 SUNSHINE_CONTEXT_ID 再执行 sunshine task start。
✓ 状态：planning → in_progress（降级）
✓ 已记录分支：master（降级）
警告："master" 也是该任务的 base_branch，PR 不能指向自身分支，归档时会拒绝。
切出分支后运行：sunshine task set-branch <任务> <功能分支>
```

("Session identity unavailable; the current-task pointer is not persisted this time (degraded mode), the AI can still continue from the conversation context. Tip: run inside an AI session that provides a session identity, or set `SUNSHINE_CONTEXT_ID` before running `sunshine task start`. Status: `planning` → `in_progress` (degraded). Recorded branch: `master` (degraded). Warning: `master` is also the task's `base_branch`; a PR cannot point at its own branch, archiving will refuse. After checking out a branch, run: `sunshine task set-branch <task> <feature branch>`.")

That "degraded mode" is a property of running the command in a plain terminal, where there is no session identity — `workflow.md` itself says `create` points at the new task automatically "when a session identity is available". Inside the agent session the identity exists, so the pointer is persisted normally. Either way, `.sunshine/.current-task` is still written, so the current task can be found:

```sh
sunshine task current
# .sunshine/tasks/11-10-给退款原因加必填校验

sunshine task list
# 全部活动任务：
#
#   - 11-10-给退款原因加必填校验/ (in_progress) [演示者] <- 当前任务
#
# 总计：1 个任务

sunshine task context
# ========================================
# 会话上下文
# ========================================
#
# ## 开发者
# 名字：演示者
#
# ……
# ## 当前任务
# 路径：.sunshine/tasks/11-10-给退款原因加必填校验
# 来源：file:.sunshine/.current-task
# 名字：给退款原因加必填校验
# 状态：in_progress
# 创建：2026-11-10
# 描述：退款时原因字段必填
# ……
```

("全部活动任务" = "all active tasks"; "总计：1 个任务" = "total: 1 task"; "会话上下文" = "session context"; "开发者" = "developer"; "当前任务" = "current task"; "来源" = "source".)

### Archiving and recording the session

At the end of the work the agent runs `/sunshine:finish-work`, which archives the task and records the session in the journal. Archiving is strict about branches — in the demo project the task never went through a PR, so the plain archive is refused, and that is real behavior, not a mistake:

```text
警告：记录的分支 "master" 已不存在（可能已合并删除）。
错误：拒绝归档 "11-10-给退款原因加必填校验"：branch 与 base_branch 都是 "master"，PR 不能指向自身分支，这份元数据无法描述已合并的工作。
请修复其中一个字段：
  sunshine task set-branch 11-10-给退款原因加必填校验 <功能分支>
  sunshine task set-base-branch 11-10-给退款原因加必填校验 <目标分支>
  sunshine task archive 11-10-给退款原因加必填校验 --skip-branch-validation   # 仅当该任务从未走 PR
未归档：.sunshine/tasks/11-10-给退款原因加必填校验 未动。
```

("Warning: the recorded branch `master` no longer exists (may have been merged and deleted). Error: refusing to archive …: branch and base_branch are both `master`; a PR cannot point at its own branch, this metadata cannot describe merged work. Fix one of the fields: … / … / … # only if the task never went through a PR. Not archived: … untouched.")

In a real project the agent would `set-branch` to the feature branch first. The demo task never went through a PR, so it skips the check:

```sh
sunshine task archive 11-10-给退款原因加必填校验 --skip-branch-validation
# 警告：记录的分支 "master" 已不存在（可能已合并删除）。
# 已归档：11-10-给退款原因加必填校验 -> archive/2026-11/
# [OK] 已自动提交：chore(task): archive 11-10-给退款原因加必填校验

sunshine task list-archive
# 已归档任务：
#
# [2026-11] - 1 个任务
#   - 11-10-给退款原因加必填校验/
```

("Archived: … -> `archive/2026-11/`; auto-committed: `chore(task): archive …`; archived tasks: `[2026-11]` - 1 task.") The archive path is `.sunshine/tasks/archive/<YYYY-MM>/`, and archiving auto-commits by default (while `session_auto_commit` is `true`).

The session record goes into `.sunshine/workspace/<your name>/journal-1.md`, with the auto-generated section of `index.md` updated at the same time:

```sh
sunshine task add-session --title "退款原因必填校验实现" --summary "加了原因下拉框与必填校验"
```

Real output (excerpt):

```text
[OK] 会话已追加到 journal-1.md
……
[OK] 会话 1 添加成功！
[OK] 已自动提交：chore: record journal（更新 journal-1.md 与 index.md）
```

("Session appended to `journal-1.md`; session 1 added successfully; auto-committed: `chore: record journal`, updating `journal-1.md` and `index.md`.")

One environment note, recorded as it happened: the auto-commit needs a configured git identity. In the first demo run the sandbox had none, so archiving and `add-session` printed `[警告] 自动提交失败：Author identity unknown` while still writing the files, and re-running the exact same `add-session` command resumed from the commit step instead of appending a second session. Set `git config user.email` / `user.name` once and the `[OK] 已自动提交` lines above are what you get.

### Verification slips: don't archive foundation tasks yet

Some tasks are done, and their checks pass, but whether they are actually right only becomes clear once a later feature plugs into them. Foundations are the classic case: you build the login base first (sessions, tokens, interception), but there is no login page yet, so items like "session survives a refresh after login" and "anonymous users get intercepted" simply cannot be verified. Tasks like this used to be archived as soon as their checks passed, and the problems only surfaced during integration — long after the task was closed.

The check reconciliation now has one more verdict: **pending integration verification**. If what needs verifying is not in this task but belongs to a later task's deliverable, it gets this verdict, with two things spelled out: what it is waiting for, and what counts as passing. When the main session wraps up, it opens a **verification slip** from those items, and the task moves into a "verifying" state instead of being archived:

```sh
sunshine task verify-open 11-26-地基脚手架 --item "登录后会话保持" --standard "真实登录后刷新仍保持登录" --verifier 11-26-登录页
# 已立单：地基脚手架 转待验（verifying），新增 1 条（共 1 条未销）。单子见 verify.md。
```

("Slip opened: the foundation task moved to verifying, 1 new item, 1 open in total. See verify.md.") The slip lives in the task directory (`verify.md`, generated by the command — don't hand-edit it); `task list` shows the task with a `[待验：未销 n 条]` tag, and every new session reminds you. Archiving is refused by the command itself, with no way around it:

```text
错误：拒绝归档 11-26-地基脚手架：待验单未清零
  - 本任务待验单未销：V-1 登录后会话保持（待验）
```

("Error: refusing to archive — the verification slip is not cleared: V-1, session persistence after login, still pending.") Once the login page is built, its check verifies the item along the way (integration-level verification: actually run the flow end to end and use the real output as evidence, not "the code looks right"), and the main session signs it off from the check report:

```sh
sunshine task verify-sign 11-26-地基脚手架 V-1 --result 验通 --by 11-26-登录页 --evidence "真实登录后刷新仍保持登录，集成测试通过"
```

A sign-off has only three outcomes: passed, failed, or still unverifiable. Failed means the foundation task goes back to work in place (it was never archived, so `task start` just resumes it) and gets verified again after the fix; still unverifiable means the item stays open. Once the slip is fully signed, archive the foundation task first and the verifying task right after. If a requirement change kills the feature, its item can be waived with your approval (`verify-waive`, a reason is required) — that also closes it.

Both flows work the same way: in design-first, who-waits-for-whom is planned during review and decomposition; in dev-first there is no pre-assignment, so whichever new task can verify an item gets it when its context is assembled. A task with nothing downstream to verify it may not open a slip at all — its acceptance must include an end-to-end path it can run on the spot.

### Stopping mid-list: the checklist progress is not lost

Design-first sub-tasks run one at a time, and the progress lives on disk if you get pulled away mid-list or close the session and come back tomorrow. A reopened session shows an "unfinished checklists" line in its opening state; and when you resume with `/sunshine:continue`, the agent reports the checklist first — for each parent task: how many sub-tasks were split out, how many are archived, in progress, or awaiting verification, and which one is next — and asks whether to continue the current one or move to the next, proceeding only on your say-so. It reports; it never silently resumes — side-tracked work is neither quietly picked up nor forgotten.

## 4. Specs: the project contract

This section covers spec once, completely: where it comes from, how it is dug out of code, what tooling that takes, how it is maintained day to day, how the agent reads it, and where templates come from.

### Where specs come from: requirements and code, organized in

A spec is not a document someone writes from a blank page — it is organized in from two sources:

- **The requirements line**: conventions and decisions clarified while working a requirement land in spec on the spot; rules settled mid-task are written back at the end (see "How specs are maintained" below). The source is the Q&A record and your confirmation.
- **The code line**: conventions that are already facts in an existing codebase (layering, naming, interface contracts, error handling…) and that nobody can state one by one are dug out of the code by `sunshine-spec-bootstrap` — operations in the next subsection of this section, record in 9.2.

The code line works "graph first, text second": when the project has a `.codegraph/`, the bootstrap first queries the graph with CodeGraph (`codegraph explore` / `codegraph context`) to map symbols, call relations, and module structure — the graph comes from deterministic parsing, no model involved, cheap and complete; then it reads the actual source the graph points at, verifies it, and distills the patterns that genuinely repeat into spec entries. The graph is navigation, not authority: any convention that goes into spec must be pointable to a real file or a repeating pattern in the source — if it cannot be pointed at, it is not written. Without CodeGraph it falls back to language tooling and direct source reading; the flow is the same, just slower.

One principle governs both lines: write only what has a source — no generic advice, no placeholder text; the templates are a starting point, and spec files can be deleted, renamed, split, or added when the repository needs it. A finished spec carries its head index and applicability tags (the next two subsections), so the agent reads only the files and sections a task needs — that is what makes spec usable as a "contract": written from evidence, and cheap to read.

### When the conventions are unknown: dig them out of the code with sunshine-spec-bootstrap

The requirements-line route assumes the conventions are known — the three lines were in the README and people remembered them. The following is an example (written from the skill's documentation, not a real run): the more common case is a project that has run for three years with no written conventions at all, everything living in the code and nobody able to state it. The scenario here is a Go backend monolith, a dozen packages, a README with a single startup command.

This time it does not go through creating a task, writing a PRD, and implement/check. It uses the `sunshine-spec-bootstrap` skill (installed with the platform configuration, not referenced by the workflow), one agent running the whole loop in a single pass:

Example prompt (an example, not a real session record):

> Nobody has written conventions for this project; they all live in the code. Please use sunshine-spec-bootstrap to analyze the codebase and produce a usable spec.

The agent first reads the current spec tree (all templates), then analyzes the repository: top-level structure, package list, build scripts; when the project has a `.codegraph/` (built by init for existing projects — see the next subsection) it queries the graph first with CodeGraph (`codegraph explore` / `codegraph context`) to map symbols, call relations, and module structure, then uses GitNexus for execution flow, module clusters, and dependency centers, and ABCoder for exact signatures and types; finally it reads representative source and test files directly, and only turns a finding into a rule after confirming it — the graph supplies leads, and nothing is written down before it is verified in the source. The findings it digs out of the code would look like this (example): on package boundaries, `internal/order` owns orders and `internal/payment` owns payments, payment is only called by order and never the other way; errors are uniformly wrapped with errors.Wrap plus an error code, converted to `{ code, msg }` at the HTTP layer; configuration is config.yaml plus environment overrides; tests are all table-driven.

Then it writes the spec by package and layer: template sections that do not apply — `frontend.md`, say, when there is no frontend — are deleted; each package gets a section, each rule carries a real file path and an example; important local patterns the templates miss (the error-code convention, for example) get a new file of their own. After writing it verifies three things: the spec is internally consistent, there is no placeholder text, and `index.md` matches the final file set.

The difference from the route above is one line: conventions known, ask and land them in spec, people tell the agent; conventions unknown, run spec-bootstrap and let the agent discover them from the code.

One more thing: spec-bootstrap does not produce an API reference directly. Its product is the coding guidance in `.sunshine/spec/` — structure, routes, and data flow are all there, but not as a human-readable endpoint list. For an API document, go in two steps: run spec-bootstrap to land the spec first, then a separate task to produce the document from the spec, placed in the project's `doc/` directory, not in `.sunshine`; the agent writing the document reads the spec instead of re-reading the whole codebase, and when interfaces change later, the document gets updated alongside the spec write-back at step 3.3.

Example prompt (an example, not a real session record):

> Please follow the sunshine workflow: first use sunshine-spec-bootstrap to land this project's structure and conventions in spec, then produce an API reference from the spec and place it in doc/, verifying every endpoint against the code.

No need to name every area to dig into in the opening prompt: spec-bootstrap's repository analysis is full coverage by default — runtime layers (CLI, backend, frontend, worker, shared libraries), deployment (build, deploy steps, rollback), startup, configuration, data flow, error handling, testing, and troubleshooting are all in the default dimension table, and anything with evidence in the code lands in the spec. So the two-step prompt above is the most practical opening line for a legacy project. The only limit is what the code actually contains: nothing is invented without evidence, and template files for layers the repository does not have are deleted outright.

### Code map: CodeGraph for existing projects

The hard part of an existing project is not writing new code; it is helping the agent find the right old code: who calls a function, what a change touches. Grepping file by file is slow and easy to get wrong. For existing projects Sunshine borrows a ready-made code map — CodeGraph (an open-source tool that parses code into a graph of symbols and call relations; building the graph is deterministic parsing, no LLM and no tokens, and it auto-syncs when files change).

**When it is installed**: installing the tool is a one-time, machine-wide thing, so **every project's first `sunshine init` asks**, new or existing; building the graph is only for projects that already have code. Each of the three layers fills in only what is missing:

1. No CodeGraph on the machine — installing a tool touches your machine and the network, so init never does it unasked: interactive mode asks first (**and only once**: a no is recorded in `.sunshine/config.yaml` and later init runs never ask again; run `sunshine codegraph install` whenever you change your mind). On a yes, Sunshine downloads the bundle itself (about 58 MB, with a percentage progress display; installed onto your machine, not into Sunshine's packages) over the same mirror stations as `sunshine upgrade` but in the opposite order — **mirrors first** (the bundle is big and a direct connection is routinely slow): mirrors first, moving to the next channel whenever one fails or is still unfinished after 4 minutes, direct last, and the official script only if every channel fails; with `-y` or a non-interactive terminal it does not install at all and just prints a hint. Once the tool is installed, no project ever asks again;
2. The project already has code but no `.codegraph/` — build the graph (it only writes inside the project and can be deleted any time); a brand-new project has no code and skips this step — re-running init after code exists fills it in automatically;
3. Wire the queries into the platforms selected in this init (MCP where the platform supports it, equivalent CLI queries elsewhere).

Any failed step only warns and prints the manual command; it never blocks init. To opt out: `sunshine init --no-codegraph`, or set `SUNSHINE_NO_CODEGRAPH=1`. Installing the tool and building the graph can take a few minutes: the bundle download shows a live percentage and MB count, and CodeGraph's own output streams to the terminal while the graph builds, so you can watch how far each has got. If you do not want to wait, Ctrl+C cancels only the code map part — the rest of init still completes, and you can fill the gap later with `sunshine codegraph install`.

**When `.codegraph/` is generated**: exactly at step 2 above — during init, once the tool is in place (already installed, or installed with your consent at the first init), the project has code, and there is no `.codegraph/` yet, it is built on the spot; a manual `sunshine codegraph install` does the same. If it already exists it is never rebuilt — CodeGraph itself keeps it in sync incrementally as files change, and Sunshine does not build it twice. To rebuild from scratch, delete `.codegraph/` and run init or `sunshine codegraph install` again. A brand-new project has no code and gets no graph (the tool is still asked about and installed at its first init); once code exists, re-running init fills the graph in automatically — no manual command needed.

**Manual trigger**: no need to wait for init — `sunshine codegraph install` installs the tool, wires it, and builds the graph at any time (manual runs are not gated on the existing-code check and re-wire on every run); `sunshine codegraph status` only reports state and changes nothing.

**How it is used**: you do not query it by hand — the workflow and subagents do. When landing a spec for an existing project (spec-bootstrap) or doing research, they query the graph first to map symbols, call relations and structure, then read files; before changing an existing function they check its callers; during checks they use the graph for affected tests and change impact as evidence for the acceptance reconciliation.

Once wired, the agent gains a set of graph queries (MCP tools on platforms that support them, equivalent CLI commands elsewhere — both ask the same graph): search symbols by name, view a symbol's definition and signature, find callers and callees, compute the impact of a change, and outline a file's structure. You do not need to remember any of these — ask "who calls this function" in the agent UI and it queries the graph itself. To look yourself, run `codegraph explore <question>` in the project.

Status is checkable at any time (real output; here the tool is installed and the graph built):

```text
$ sunshine codegraph status
代码地图（CodeGraph）状态：
  工具：已安装（/usr/bin/codegraph）
  项目图：已建（.codegraph/）
  存量代码：检测到（init 会供给代码地图，未装工具时先问）
  已装平台：无（非 Sunshine 项目或尚未 init）
```

Run inside an initialized project, the last line lists the installed platforms and their wiring targets; whichever line reports "not installed / not built", one `sunshine codegraph install` fills the gap.

**Boundary**: the graph is CodeGraph's own — it syncs itself on file changes, upgrades with `codegraph upgrade`, and can be removed with `codegraph uninstall` (wiring) and `codegraph uninit` (the graph). Sunshine only installs, builds, and writes the rules into the workflow and subagents.

### How specs are maintained

Specs are not a document you write once and forget. `.sunshine/spec/system.md` states the rule at the top:

```text
# 系统规范（自主沉淀总原则）

本文件是 Sunshine 规范自维护的总原则。AI 助手在后台自主、静默地维护 `.sunshine/spec/`，人只负责审阅成果。

## 总原则

- 技术选型、分层架构、数据库规约等确定后，立即写入 `.sunshine/spec/` 对应文件，不等人手动整理。
- 规范一经写入，后续所有任务自动遵守。
- 发现规范与现实不符（代码已改、规范未改），以现实为准并同步修正规范。
```

("System spec — the general principle of autonomous spec maintenance. The AI assistant maintains `.sunshine/spec/` autonomously and silently in the background; a human only reviews the result. / General principles: once a technical choice, layering, or database convention is settled, write it into the matching file under `.sunshine/spec/` immediately, don't wait for manual tidying. Once a spec is written, all later tasks follow it automatically. If a spec disagrees with reality (the code changed, the spec didn't), reality wins and the spec is corrected.")

In practice that means two things:

- **During a task**, the agent writes what it learned back into the specs (workflow step 3.3, `sunshine-update-spec`) — for the demo task, the `REASON_REQUIRED` error code and the "other reveals a text box" rule are exactly the kind of conventions that get written down so the next refund task does not have to rediscover them.
- **Between tasks**, you can also just tell the agent to update a spec ("Record in the backend spec that all list endpoints paginate with cursor-based pagination") and it edits the file; you review the diff like any other change.

Specs are user data: `sunshine update` never touches `.sunshine/spec/` — in the scan output in [section 8](#8-maintenance-update-and-upgrade) it appears under "用户数据（保持不动）", "user data (kept untouched)". The one exception is a spec installed from the template marketplace, which is hash-tracked and refreshed by `update` — see [section 8](#8-maintenance-update-and-upgrade).

### The template marketplace: install specs from a registry

A template marketplace is a repository of reusable specs: specs other people already wrote (how interfaces are shaped, what the UI rules are) that you can reuse instead of writing your own from scratch. The official marketplace is `gh:linrong007/Sunshine/marketplace`. It holds one full-stack spec (`sunshine-fullstack` — the three default files `system.md`, `frontend.md`, `backend.md`), two skills (`go-sdk-author`, `mem-recall`), and three workflows (`native`, `tdd`, `channel-driven-subagent-dispatch`).

To start a new project from a marketplace spec, add `--registry` when you init:

```sh
mkdir demo3 && cd demo3 && git init
sunshine init -u 演示者 -y --claude --registry gh:linrong007/Sunshine/marketplace --template sunshine-fullstack
```

Real output:

```text
正在配置 Claude Code……
已安装模板 "sunshine-fullstack"（.sunshine/spec），共 3 个文件。
已记录 registry.spec：gh:linrong007/Sunshine/marketplace（模板 sunshine-fullstack），sunshine update 会持续刷新。
初始化完成，开发者：演示者
已安装平台：
  - Claude Code
下一步：
  1. 在 agent 界面里打开项目，直接用自然语言提第一个需求；
  2. 规范已从模板市场装进 .sunshine/spec/，工作流按 .sunshine/workflow.md 自动推进，一般不用改。
```

("Setting up Claude Code… / Installed template `sunshine-fullstack` (`.sunshine/spec`), 3 files in total. / Recorded `registry.spec`: `gh:linrong007/Sunshine/marketplace` (template `sunshine-fullstack`); `sunshine update` will keep refreshing it. / Initialization complete…")

After install, `.sunshine/spec/` holds the template's three files, and the source is recorded in `config.yaml`:

```yaml
registry:
  spec:
    source: gh:linrong007/Sunshine/marketplace
    template: sunshine-fullstack
```

After install, a duplicate check runs: if a section title shows up in more than one file, or the same section has identical text, it prints a heads-up line — it only reports, it never blocks the install. To see the exact places, run `sunshine spec check` (below).

Common variations:

- **Marketplace only, no template**: when a marketplace holds exactly one spec template, `--template` can be left out and it is picked automatically. The official marketplace has only one spec (`sunshine-fullstack`), so the command above works the same without `--template sunshine-fullstack`. With several spec templates you must name one; omitting it (or naming a wrong id) errors out and lists the available ids. If a marketplace has no spec template at all, it errors with "该市场没有 spec 模板" (this marketplace has no spec templates).
- **Without `-y`, a menu appears**: in an interactive terminal with no `--template` given, it asks you to pick first (start from scratch / pick one from the marketplace / enter a custom source); with `--registry` the menu lists that marketplace's spec templates directly. See the init reference in the appendix.
- **Run it again, only missing files are added**: running the same command in a project that already has the template only installs files that are missing — existing files are left alone, including ones you wrote yourself. Delete one file and re-run, and only that file comes back.
- **Replace everything**: `--overwrite` empties `.sunshine/spec/` and installs fresh, including deleting files you wrote yourself — only use it when you mean it.
- **A failed install doesn't stop init**: if a template can't be pulled (say the entry points at a directory that doesn't exist), init carries on with the built-in blank specs and prints a retry command to run again later.
- **Install a skill**: same command, different id (e.g. `--template go-sdk-author`). The skill lands in `.agents/skills/<id>/`, and the three default spec files are still generated as usual. The skill install is also recorded in `registry.spec` (the field is still called spec), and `update` refreshes it the same way.

- **Workflows don't go through this command**: install a workflow template with `sunshine workflow --template tdd` (see section 2).
- **Point straight at a template directory**: the source can also be one template's directory, no marketplace and no `--template` needed — the whole directory is installed into `.sunshine/spec/` as-is:

```sh
sunshine init -u 演示者 -y --claude --registry gh:linrong007/Sunshine/marketplace/specs/sunshine-fullstack
```

A remote marketplace works the same way — swap `gh:owner/repo` for a full git URL, an HTTP(S) URL, or a `git@` address. A local marketplace is a local git repository path, and it must point at the level that contains `index.json`: the official marketplace lives in the `marketplace/` subdirectory of the repository root, so the source has to be written up to `…/marketplace`, not the repository root.

Before installing, Sunshine looks for `index.json` under the source root, and there are two modes:

- **Marketplace mode**: the root has an `index.json` listing the templates one by one (see "Build your own marketplace" below). Then `--template` must name an id (unless auto-pick applies); `workflow` entries can't be installed this way — it tells you to use `sunshine workflow --template` instead.
- **Direct download mode**: the root has no `index.json`, so the whole source is one spec and goes straight into `.sunshine/spec/`. Then `--template` must not be passed — passing it errors out. If the root has a `marketplace/specs/` directory it is used, otherwise the root itself.

The repository root is the directory the source string points at, and every `path` in `index.json` is relative to it.

### Every spec counts: applicability tags and the spec map

Every `.md` file under `.sunshine/spec/` counts as a spec, not just the default three: `api.md` or `ui.md` pulled from a marketplace, or notes you wrote yourself — all of them. Each file carries one line at the top, `适用：前端 / 后端 / 通用` ("Applies to: frontend / backend / general"), saying who should read it:

```md
---
适用：后端
---

# 后端规范
```

If that line is missing, the first sync after install (`sunshine spec index`) fills it in from the file name: names containing `frontend` are frontend, `backend` are backend, `system` and anything else are general (so `ui.md` and `api.md` both land in the general group — to make one frontend, change its line to `适用：前端` by hand). If the template author already wrote the line, the author's value wins and install never rewrites it.

Every time a session starts, Sunshine injects a spec map: all files under `spec/`, grouped by applicability, one line each (path + the section names from its index). Frontend work reads the frontend group plus the general group; backend work reads the backend group plus the general group — and only the files this task actually needs. `sunshine task context --mode packages` shows the same map.

To check whether specs repeat each other (say two files each describe the login endpoint), run:

```sh
sunshine spec check
```

A section title appearing in more than one file is listed; sections whose text is fully identical are additionally marked "内容相同" (identical content). It only reports and never edits files; it exits 1 when duplicates are found and prints "未发现重复" (no duplicates found) when there are none.

### Local paths and the LAN

A marketplace doesn't have to live online: a local git repository works as a `--registry` source too.

- **Local paths**: absolute paths, relative paths (starting with `./` or `../`), and `file://` URLs all work; both the repository root and a subdirectory work (e.g. `--registry /path/to/Sunshine/marketplace`). With a subdirectory you don't need to work out where the repository root is: Sunshine walks up from the directory you wrote and takes the first directory containing `.git` as the repository root, with the rest as the subdirectory.

- **Commit first**: a local path is still fetched by cloning, so only committed content is visible. Files you added to the marketplace but haven't committed yet can't be pulled — you'll get a "template not found" error. One `git commit` is enough; no push needed.
- **LAN shared folders**: a mounted folder on a NAS or shared drive works as a local path (same rule: commit first).
- **LAN git servers go over SSH**:

```sh
sunshine init -u 演示者 -y --claude --registry git@192.168.1.10:team/market.git --template sunshine-fullstack
# Or with a port: ssh://git@192.168.1.10:2222/team/market.git
```

Authentication uses ssh-agent or a key in `~/.ssh`. This one is verified against the code only — it hasn't been tested on a real LAN yet.

- **Plain http:// and https:// both work on a LAN**: writing `http://192.168.1.10/team/market.git` or `https://192.168.1.10:3000/team/market.git` clones over the protocol you wrote — it is no longer rewritten to `https://` (tested against a local http git server). The `git://` protocol is still not supported — it errors with `不支持的 provider "git"` ("unsupported provider "git""). Self-signed https still has no skip-verification option.

### Build your own marketplace

A marketplace is just a git repository, laid out like this:

```text
marketplace/
├── index.json
├── specs/       # one spec template per subdirectory
├── skills/      # one skill template per subdirectory
└── workflows/   # one workflow template per subdirectory
```

`index.json` lists every entry with at least four fields (id, type, name, path — path relative to the marketplace root; tags optional):

```json
{"id": "sunshine-fullstack", "type": "spec", "name": "Sunshine 默认全栈规范", "path": "specs/sunshine-fullstack"}
```

Commit once after writing it (no push needed), then use a local path or a git address as the `--registry` source. A team marketplace can live in a private repository.

### The index layer: read the index first, then jump

Every spec file carries an index table at its head (section names + a one-line summary each). The agent reads the index first and jumps only to the sections this task needs, instead of pouring the whole file into context — measured on one backend spec: 1742 tokens for the full text, 534 tokens for the index plus the one relevant section. The index is mandatory: files installed by init or pulled from a marketplace carry one; a file you drop into `spec/` by hand gets one automatically at the next session start, or right away with `sunshine spec index`. Only the main session edits specs; sub-agents read them.

### Adopting an existing project: the order is reversed

An existing project lands its conventions before any feature work: `sunshine init` installs the skeleton (supplying CodeGraph along the way) → the conventions go into spec (Q&A when known, spec-bootstrap when unknown) → then feature work starts in one of section 2's modes. init only adds files and never touches existing code; the acceptance bar is one line — after init, `git status` shows no modified old files. The end-to-end record is 9.2.

## 5. Memory: mem and long-term memory

### mem: search the raw sessions

`sunshine mem` searches the agent session records stored on this machine (across platforms), for "how was this fixed last time, what was said then". It is purely manual and read-only, searches the current project by default, and in practice is one command — `sunshine mem search "refund"`; more usage is in the appendix, and the record is 9.5. It searches raw text, not conclusions — conclusions are distilled into the thing below. One boundary to keep clear: it only covers this machine — if the last session ran on a different computer, the search comes up empty; what carries across computers is long-term memory and the spec, which live inside the project and travel with the repo.

### Long-term memory: project lessons

**What it is for**: keeping the pitfalls this project hit, and how its settled practices came to be, for future sessions. A new session or task starts out already knowing these lessons — no re-stepping on the same rake, no re-arguing what was already decided. It is the project's institutional memory.

**What is stored**: one file per item in `.sunshine/memory/` (title, summary, body), with `index.md` as the directory. The split with spec: anything that can be stated as a rule goes into spec (that is "what the rule is now"); the story behind the rule — why it was decided this way, what went wrong at the time, which alternatives were considered — goes into long-term memory (that is "why the rule is this"). When the two conflict, spec wins: long-term memory is a story archive, not a second rulebook.

**When it is written**: at a task's closing step, 3.3 (update the specs), the agent judges in passing whether this task produced a story worth keeping for the future; if so it writes one item and adds a line to `index.md`; if not, it writes nothing — and when in doubt it writes spec only. Nothing is scraped automatically: nobody moves session transcripts in, and mem's raw-text search never turns into long-term memory on its own.

**When it is read**: at the start of every session, the long-term memory index (titles and summaries only, capped at 4000 bytes, truncated by line beyond that) is injected into context automatically, so the agent knows these lessons exist and reads the body of whichever item is relevant; the same index rides along when context is assembled for sub-agents. You never have to remind it "remember last time's pitfall" — it already knows at the start.

## 6. Saving tokens: the three-piece set

Every sub-agent round re-reads its context: what gets injected, and how large tool outputs are, decides the token bill. Three compressions are in place, each measured (all figures are fixed-task byte measurements, not whole-session numbers):

- **check on a diet**: the check sub-agent is injected only with the selected check.jsonl entries + the acceptance-criteria section of prd, not the whole bundle — fixture-measured injection fell from 51285 bytes to 1460 bytes (about -97%, and that is the check round's figure).
- **narrow RTK rewriting**: a few high-frequency commands have their output slimmed before it reaches the agent, whitelist only: `git log` -86.8%, `git diff` -98.7%, `go test -v` -84.7% (fixed-task measurements).
- **squeez compression**: tool output gets fuzzy de-duplication and throttling, with repeats replaced by a reference: same-template logs -98.4%, a repeated output's second appearance -65.3%.

One clarification: metering and guardrails (limits, blocks) do not themselves save a single token — they only stop runaway work. The three above are the actual compression.

## 7. Multi-agent collaboration

One task is not done by a single agent end to end: the main session owns clarifying the requirement, the review, and acceptance, while the actual coding, checking, research, prototyping, and review scanning are dispatched to dedicated sub-agents (implement / check / research / prototype / audit). The split is the workflow's internal business — you do not need to manage it: you appear only to answer questions, confirm the review, and accept the result, and every sub-agent's output lands back in the main session for you to see.

### serve: watch progress

`sunshine serve` starts a read-only web page on this machine (`127.0.0.1:17890`) showing the current project's task progress, sub-task completion, and sub-agent start/stop events, refreshed by polling — no agent session needs to stay open. It only reads files and never talks to an agent, so it can only show what has already been written to disk; with no `.sunshine/` it refuses to start. For a second project at the same time, add `--port`.

### channel: how agents coordinate

When several workers run in parallel (one implementing, one checking), they pass messages, wait on events, and queue work through channels, driven by the main session underneath — that is what the `channel-driven` workflow template does. You meet this concept only when choosing a workflow; the command details are the agent's job — the manual-inspection commands are in the appendix. Two preconditions are worth knowing: spawning a real worker needs a logged-in claude / codex on this machine, and a wait timing out is a normal result, not a failure.

## 8. Maintenance: update and upgrade

### `update`: refresh the template files in the project

When a new Sunshine version ships new templates, you refresh the project with `update`. First look at the plan with `--dry-run` — it prints the plan and writes nothing:

```sh
sunshine update --dry-run
```

Real output (excerpt; run with the v0.2.1 binary, project on 0.2.1):

```text
Sunshine 更新
══════════════
项目版本：0.2.1
CLI 版本：0.2.1
最新版：0.2.1


正在扫描变更……
  未变更文件（将跳过）：
    ○ .claude/agents/sunshine-check.md
    ○ .claude/agents/sunshine-implement.md
    ○ .claude/agents/sunshine-research.md
    ○ .claude/commands/sunshine/continue.md
    ○ .claude/commands/sunshine/finish-work.md
    ……另有 102 个

  用户数据（保持不动）：
    ○ .sunshine/workspace/
    ○ .sunshine/tasks/
    ○ .sunshine/spec/
    ○ .sunshine/.developer/

✓ 已是最新，无需更新。
```

("Scanning changes… Unchanged files (will be skipped): … / User data (kept untouched): `.sunshine/workspace/`, `.sunshine/tasks/`, `.sunshine/spec/`, `.sunshine/.developer/` / Already up to date, nothing to update.")

Three states at a glance: unchanged template files are updated automatically; files you modified ask one by one (overwrite / skip / write a `.new` copy, default skip); user data (specs, tasks, workspace, developer identity) is never touched. Switch the workflow to tdd first ([section 2](#switching-workflow-templates)) and the same scan shows the third state in action:

```text
  你已修改（需要你决定）：
    ? .sunshine/workflow.md

  用户数据（保持不动）：
    ○ .sunshine/workspace/
    ○ .sunshine/tasks/
    ○ .sunshine/spec/
    ○ .sunshine/.developer/

[试运行] 未做任何修改。
```

("Modified by you (needs your decision): `.sunshine/workflow.md` / User data (kept untouched): … / [Dry run] No changes made.")

To actually run it, drop `--dry-run`. Before writing, it takes a full backup — verified by running it for real on the tdd-switched project (non-interactive, so the conflict file was skipped by default):

```text
非交互环境，跳过统一确认（冲突文件默认跳过）。

已创建备份：.sunshine/.backups/2026-11-10T01-16-37/

--- 逐个确认冲突 ---
  ○ 已跳过：.sunshine/workflow.md

--- 汇总 ---
  已跳过：1 个文件
  备份：.sunshine/.backups/2026-11-10T01-16-37/

✓ 更新完成！（0.2.1 → 0.2.1）
```

("Non-interactive environment, skipping the unified confirmation (conflicting files are skipped by default). / Backup created: `.sunshine/.backups/2026-11-10T01-16-37/` / … skipped: `.sunshine/workflow.md` / … / Update complete! (0.2.1 → 0.2.1).") On success it updates `.sunshine/.version`.

Who runs `update`? Usually you, in the terminal, when you know a new version is out — or you ask the agent to ("Check whether the project templates need updating") and it runs `update --dry-run` and shows you the plan. Either way, nothing is overwritten silently.

### After the marketplace template changes, `update` keeps refreshing it

Edit the template in the marketplace, commit, and run `update --dry-run` again (excerpt):

```text
正在扫描变更……
  模板已更新（将自动更新）：
    ↑ .sunshine/spec/system.md

  未变更文件（将跳过）：
    ○ .claude/agents/sunshine-check.md
    ……

  你已修改（需要你决定）：
    ? .sunshine/config.yaml

  用户数据（保持不动）：
    ○ .sunshine/workspace/
    ○ .sunshine/tasks/
    ○ .sunshine/spec/
    ○ .sunshine/.developer/

[试运行] 未做任何修改。
```

("Scanning changes… / Template updated (will be updated automatically): `.sunshine/spec/system.md` / Unchanged files (will be skipped): … / Modified by you (needs your decision): `.sunshine/config.yaml` / User data (kept untouched): … / [Dry run] No changes made.")

`system.md` is updated automatically because its local copy was never edited; if you had edited it locally, it would go through the same "modified by you" confirmation. `config.yaml` shows as "modified by you" because `registry.spec` was written into it when the template was installed — that is expected.

### upgrade: update the CLI itself

`upgrade` upgrades the CLI itself (`update` upgrades the template files inside a project — two different things). Check the plan first with `--dry-run`; nothing is written. This is a command you run yourself in the terminal — it is about your installation, not about a task, so the agent has no reason to run it.

When already on the latest version (output from a local build with the version injected as 0.2.1 via ldflags, matching the current Release):

```sh
sunshine upgrade --dry-run
# 已是最新版本（当前 0.2.1，最新 0.2.1），无需升级。
```

("Already on the latest version (current 0.2.1, latest 0.2.1), no upgrade needed.")

When a new version exists (output from the default local build, whose version is 0.1.0, against the same latest 0.2.1):

```text
正在升级 Sunshine：0.1.0 → 0.2.1（通道 latest）
资产：sunshine_0.2.1_linux_amd64.tar.gz
下载地址：https://github.com/linrong007/Sunshine/releases/download/v0.2.1/sunshine_0.2.1_linux_amd64.tar.gz
备份：将把当前二进制备份为 /tmp/rewrite-en/sunshine.bak
[试运行] 未做任何修改。
```

("Upgrading Sunshine: 0.1.0 → 0.2.1 (channel latest); asset: `sunshine_0.2.1_linux_amd64.tar.gz`; download URL: …; backup: the current binary will be backed up as `/tmp/rewrite-en/sunshine.bak`; [dry run] no changes made.")

A real upgrade (drop `--dry-run`) follows the same flow: check the latest version → pick the asset for the current OS and architecture → download → verify sha256 against `checksums.txt` → back up the current binary (a `.bak` next to it) → replace → run the new `--version` to verify, rolling back to the backup and reporting an error on failure. You can also pin a version or channel: `sunshine upgrade --tag 0.2.1`, `sunshine upgrade --tag beta`.

## 9. Scenario records: from day one to daily work

Sections 1–11 cover the main flow; this section walks through six real scenarios, each one a complete end-to-end run:

| Scenario | When it applies | The point |
| --- | --- | --- |
| 1. Starting a new project from zero (development-first) | A new product, service, package, or internal tool | Empty directory, `init`, first task grows from nothing |
| 2. Adopting an existing project | The repo exists, but the conventions live mostly in people's heads | `init` touches no existing code; land the conventions in spec first |
| 3. Requirement changes | A requirement changes halfway through | The same change run twice, development-first and design-first: only the changed part goes back through the loop; archived tasks stay archived |
| 4. Refactoring a legacy module | The code runs, but it is no longer safe to change | Pin the behavior with tests first, then refactor, tests green throughout |
| 5. Fixing a bug that keeps coming back | The same class of problem appears again and again | Check long-term memory and the last session first, then create the task and file the lesson in both the spec and long-term memory |
| 6. Design-first product work | Design comes before development | Design documents land in `doc/`; split into sub-tasks, then developers take them one by one |

### 9.1 Starting a new project from zero (development-first): a clothing-store admin system

An empty directory, nothing in it — the cleanest starting point. Create an empty directory and run `git init` once (fix the project root first: the agent finding files and platforms discovering skills both depend on it), and leave the rest to Claude Code: `init`, creating the task, writing the PRD — all of it runs underneath, and you type no commands. An empty directory has no detectable type, so it defaults to full-stack and all three specs (system.md, frontend.md, backend.md) are generated — a new project starts with a default set of specs, and the team adjusts them later, which beats starting with nothing.

Open the project in Claude Code and state the first requirement like this (same for Codex and Cursor) — development-first, with the opening landing directly on the first requirement's code:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow, development-first. I'm building a clothing-store admin system; the first requirement is the product list: it shows each product's name, category, price, stock, and on/off-shelf status, and can search by name.

Here is how the session continues (illustrative dialogue):

> **Claude Code:** First, the technical route. A: a static front end first, product data hard-coded in the page — get the list and search working, and connect a backend in its own later task; B: front and back end together, interfaces and database in this task. I recommend A — the first requirement should validate the fields and interactions; small plate, fast acceptance.
>
> **You:** A.
>
> **Claude Code:** Five columns for the list — name, category, price, stock, status. Enough?
>
> **You:** Enough.
>
> **Claude Code:** Search by name only, or name plus category?
>
> **You:** Name only — keep it simple first.
>
> **Claude Code:** Here is the planning summary, confirm and I start:
>
> - Goal: a clothing-store admin system; the first task delivers the product list (static front end, data hard-coded).
> - In scope: the product list page (five columns: name, category, price, stock, status) + search by name; no backend, no create/edit/delete.
> - Acceptance: open the page — all 6 sample products shown; searching "down" leaves only products with "down" in the name; a row with stock 0 shows status "off shelf" with the stock in red.
>
> **You:** Confirmed, go.

On confirmation the agent creates the task and writes the PRD out in full — each answer you gave was written into it on the spot, and the planning summary is what got written down; the job is small, no breakdown, one task end to end. Next, context curation (1.4): the specs implementation and checking need to read are listed into implement.jsonl and check.jsonl. Then the review (1.5):

> **You:** The planning is done — please follow the Sunshine workflow and review it, and give me the final planning summary to confirm.
>
> **Claude Code:** Sure. The review has two steps: first the check — the PRD verified line by line, goal, scope, and acceptance criteria all line up; then the audit scan — nothing that has to be decided now. The final planning summary is the one you just confirmed: one task for the product list, its three acceptance criteria all in the PRD. Your nod, and I start.
>
> **You:** Confirmed.

Only on your nod does the agent run `task start` and enter implementation. The back half — five steps, none skipped — all happens in the conversation:

**Implement.** The main session does not write code itself; it dispatches an implement sub-agent, which reads exactly the PRD and the implement.jsonl just curated. When it finishes, it reports back and asks you to try it yourself:

> **Claude Code:** Code done — a single file, `index.html`: a search box on top, the five-column table below. Six products are written into the page: down jacket, down vest, hoodie, jeans, dress, and a clearance hoodie (stock 0). Open it and try the search.
>
> **You:** Opened it — the list looks right, and searching "down" works.

**Check.** Next, a check sub-agent verifies against the PRD's acceptance criteria one by one, with evidence, not just a verbal pass. This time it catches one:

> **Claude Code:** check is done — two pass, one doesn't:
>
> - Five columns complete ✓
> - Searching "down" leaves 2 products ✓
> - The stock-0 line ✗: the clearance hoodie's status reads "Off-Shelf" where the acceptance says "Off shelf", and the stock number isn't red.
>
> I'll have implement fix those two spots, then check re-verifies.
>
> **Claude Code:** Re-checked: "Off shelf" matches now, and the stock is red. All three pass.

**Spec write-back.** With checking done, the conventions settled this round are written back into the spec, so the next task follows them without asking again:

> **Claude Code:** Spec write-back: two lines into `.sunshine/spec/frontend.md` —
>
> ```markdown
> - The product list has five fixed columns: name, category, price, stock, status.
> - Stock 0: the status shows "Off shelf" and the stock number is red.
> ```
>
> The order and inventory pages later on will follow the same rules.

**Commit.** A plan first, the commit only on your confirmation; files outside this task are not touched:

> **Claude Code:** Commit plan: one commit with just `index.html` and the spec just written back, message "feat: product list page (static)". There is also a file in the working tree you edited earlier — not part of this task, I won't touch it. Confirm the commit?
>
> **You:** Confirmed.

**Archive.** After the commit, finish-work wraps up, and the agent states plainly where the task ended up:

> **Claude Code:** Committed. The task is archived — you'll find it under `.sunshine/tasks/archive/` — and a session entry is logged: what was done, which commit, and the acceptance results are all in there. This task is closed.

This case runs the whole development-first flow: questions (answers recorded on the spot) → PRD → context curation → review → implement → check → spec write-back → commit → archive. Afterwards each requirement (order list, stock adjustments, connecting the backend…) gets its own task and repeats this line.

### 9.2 Adopting an existing project: a clothing-store admin system

This admin system is already live, written by someone else, and you have just taken it over. Products, Orders, Inventory, and Customers all have pages and APIs; the trouble is that the conventions were never written down: two are in the README, some lived only in the original author's head, and the rest are in the code. Adopting an existing project runs in reverse — do not touch features first; land the conventions in the spec first, or the agent can only guess when it writes new code. And do not reverse the order either: init first, then speak in the agent UI; state a requirement before init and the agent has no workflow to follow.

**Step one: init — put the skeleton in.** Run `sunshine init` once in the project directory (in a terminal, or have the agent run it): it only adds files and does not change a single line of existing code. It recognizes a full-stack project (pages and APIs both), generates the three spec templates, and — because the project already has code — the terminal also asks whether to install CodeGraph; on a yes it installs the tool and builds the graph (`.codegraph/`), which the digging below will use. The acceptance check is one line: `git status` shows only additions, no modified old files:

```text
?? .claude/
?? .sunshine/
?? .codegraph/
```

**Step two: land the known conventions — the workflow route, Q&A.** With init done, open the agent UI (Claude Code; same for Codex and Cursor) and make the first prompt name the workflow:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow. I've just taken over this clothing-store admin system; it's already running. Don't touch any features yet — help me land the conventions I know into the spec first.

The README has two conventions; the agent confirms them with you, and on your nod writes them down:

> **Claude Code:** The README lists two conventions — let me confirm they still hold. One: APIs uniformly return `{code, data}`. Two: prices are stored in cents and divided by 100 for display.
>
> **You:** They hold, that's how it's always been done. One more that was never written down: the admin has a single administrator role, no permission split.
>
> **Claude Code:** Noted, all three.

This step is itself a complete task, run on the same line as 9.1: create the task, write the PRD, review, implement writes the three conventions into the spec (the first two into `backend.md`, the role one into `system.md`), check verifies them against the code one by one (the return format looked at endpoint by endpoint, the price unit confirmed), then commit and archive.

**Step three: land what nobody can state — dig it out of the code with spec-bootstrap.** The remaining conventions (how lists page, when stock is deducted, how files are layered) are ones nobody can fully state; the agent has to find them in the code itself. This step's prompt is different — it does not name the workflow; it names the skill itself:

Example prompt (an example, not a record of a real session):

> Plenty of this project's conventions are ones nobody can state — they're all in the code. Please use sunshine-spec-bootstrap to analyze the codebase, dig out the remaining conventions, and land them in the spec.

Why no workflow in this sentence: spec-bootstrap is not a workflow task. It is a standalone skill — a single agent runs it in one pass, no task created, no PRD written. And you do not need to list what to dig for: by default it looks at everything — structure, APIs, data flow, error handling, tests — and lands whatever the code gives evidence for; without evidence, it writes nothing.

Its process is fixed: first it queries the CodeGraph graph to map the four modules' structure and who calls whom; then it reads representative source files and verifies each finding (the graph only gives leads — nothing is written down before it is verified in the source); finally it writes the conventions into the spec, each with its source file, and self-checks at the end (internally consistent, no placeholder text, the index matches). When it finishes, it reports:

> **Claude Code:** Digging done — 6 more landed, 9 in the spec together with step two's 3. The main ones:
>
> - List APIs uniformly page at 20 per page (source: all four list endpoints under `src/routes/`);
> - Page files live by module under `public/`, APIs by module under `src/routes/`.
>
> One I couldn't settle, so I didn't write it down: when stock is deducted, two places in the code disagree — `src/routes/orders.js` deducts at order time, `src/routes/pay.js` looks like payment time. Which one rules?
>
> **You:** Payment time rules. The order-time one is the old way; I'll open a task to fix it later.
>
> **Claude Code:** Written down as payment-time, then; the order-time spot is marked "contradicts the convention, to be fixed" — from now on, check will flag it when it sees it.

The landed spec sits in `.sunshine/spec/`: three files, an index at the head of each, and every convention traceable to its source. This step, too, writes not one line of business code.

**Step four: a small trial task, to prove the spec works.** Landing it on disk is not enough — the next task has to actually follow it. You give a real small job:

> **You:** Please follow the Sunshine workflow. Add category filtering to the product list.
>
> **Claude Code:** Following the workflow, then. You don't need to explain anything this time — implement read the spec and wrote the code by it: return format `{code, data}`, 20 per page, all from the spec; check verified against the spec, format and paging both right. All acceptance items pass; committed and archived.

This task's flow is exactly 9.1's — questions → PRD → review → implement → check → commit → archive — so it is not spelled out again here.

Only when the trial runs right is the adoption complete. Three criteria: init touched no old files; the conventions are in the spec, each with a source; the trial task followed the spec and got it right. From then on, the conventions no longer live in the original author's head — people leave, the spec stays, and every new task works by it.

### 9.3 Requirement changes: stock 0 no longer means off shelf

Halfway through a project, a requirement changes — utterly common. The change itself is the same in both halves below: out-of-stock sizes get restocked all the time, so stock 0 should no longer show "Off shelf" — show "Temporarily out of stock" instead, with a "Notify me" button. But where the change lands depends on the project's mode: development-first lands it on the task and PRD; design-first lands it back in the design pack first. One pass each — and both follow the same change rules: ask only about the delta, fix the blast radius first, update the record before touching code, and review only the changed part.

#### Development-first: the change lands on the task and the PRD

On the development-first clothing-store admin: the product list is done and archived (9.1), the order list is in progress. Operations brings the change to you, and you say in Claude Code:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow. A requirement change: products with stock 0 should no longer show "Off shelf" — show "Temporarily out of stock", and add a "Notify me" button.

Here is how the session continues (illustrative dialogue).

**First, pin down what changed.** The agent goes back to clarification, but only asks about the delta — settled things are not re-asked:

> **Claude Code:** Only the changed parts to confirm — two questions. One: what does "Notify me" do when clicked? A: just register it, the button turns into "Registered"; B: actually send a notification. Two: only the product list changes, or everywhere stock is shown, orders included?
>
> **You:** A — register only, for now. Product list only.

**Then fix the blast radius.** Before anything else, the agent walks the task list and classifies each task, so nothing is missed and nothing is changed for nothing:

> **Claude Code:** Blast radius checked: the product-list task is archived and it is exactly what displays this — affected. The order-list task is in progress; I read its PRD, it doesn't display stock status — not affected, no need to stop. Only the product list changes.

**Then update the record, before any code.** With the radius fixed, the agent still does not touch code — it fixes the sources of truth first:

> **Claude Code:** Record first, then: a new rework task, with a `[Change 2026-11-23]` entry in its PRD — what changed, why, and who decided, all written down; the spec line "stock 0 shows Off shelf" is updated to the new wording at the same time, "Notify me registers only, no notification" goes in with it, and the rework task's context is curated (the updated spec and the product-list code listed into implement.jsonl / check.jsonl). The archived product-list task is not reopened — it records the history as it was; touching it would scramble that.

**Review only the change.** The review (1.5) does not re-review the whole project; it walks the change checklist item by item:

> **You:** The change is done — please follow the Sunshine workflow and review it, reviewing only the changed part.
>
> **Claude Code:** Reviewing only the change, then — all four items check out: the `[Change]` entry is recorded; the spec is updated; the rework task's acceptance criteria and context are ready; the unaffected order task is untouched. Shall I start?
>
> **You:** Go.

**Only then, hands on code.** The rest follows 9.1's line: implement → check → spec write-back → commit → archive. check verifies against the new wording: stock 0 shows "Temporarily out of stock", the stock number still red, "Notify me" turning into "Registered" when clicked. All pass; committed and archived; the rework task is closed.

#### Design-first: the change lands back in the design pack first

Now the same admin system on a design-first project: the design pack has passed review and been broken down, and the sub-tasks are being developed one by one — the product-list sub-task is archived, the order sub-task is in progress, the inventory sub-task has not started. The same change comes in, with the same opening line:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow. A requirement change: products with stock 0 should no longer show "Off shelf" — show "Temporarily out of stock", and add a "Notify me" button.

Here is how the session continues (illustrative dialogue).

**Asking about the delta and fixing the blast radius are the same in both modes.** The same two questions, and the same answers: register only, and only the places that display stock status. The radius, checked against the parent task's sub-task list: the product sub-task is archived — affected; the inventory sub-task hasn't started — affected (it displays stock status too); the order sub-task is in progress and doesn't display stock status — not affected. What differs is below.

**Change the design pack first — and bring the affected sub-tasks in line before the review.** In design-first, the source of truth is the design pack, so the change lands there first, with everything updated before the review:

> **Claude Code:** Sources of truth first, no code: the product-list entry in the requirements list gets the new wording in its description and acceptance criteria, and the parent task's PRD gets a `[Change 2026-11-23]` entry; a prototype sub-agent updates the prototype — the product-list page's status text becomes "Temporarily out of stock", with the "Notify me" button added; the spec line is updated in step. And the inventory sub-task, which hasn't started: its PRD is edited now — acceptance criteria switched to the new wording, and the new prototype page added to its context list with add-context (the list can only grow, never shrink) — so it starts out right.

**The review covers only the change, item by item.** This time 1.5 checks whether the change has fully landed; one missing item fails it, and development may not start:

> **You:** The change is done — please follow the Sunshine workflow and review it, reviewing only the changed part.
>
> **Claude Code:** Reviewing only the change, then — four items: the parent PRD's `[Change]` entry is recorded; requirements list, prototype, and spec are all updated; the affected inventory sub-task's PRD is in sync and the new file is in its context list; the unaffected order sub-task is untouched. All four pass — the change can land.
>
> **You:** Good.

**Only after the review do the tasks move.** What remains is routing by state:

> **Claude Code:** Two things on the task side. One: the product sub-task is archived — not reopened; a new rework sub-task goes under the parent, its description naming the original task and the reason for the change, and it changes that one spot following the new prototype. Two: the order sub-task is not affected and keeps going. One rule worth stating: an in-progress task that *is* affected stops its current work, goes back through the loop, and then continues, status unchanged — not needed this time, since it isn't affected.

**The rework sub-task runs its own line.** Like every sub-task: its own review → implement → check (against the new prototype and the new wording) → spec write-back → commit → archive. Once it is archived, the change has fully landed.

**Where the two modes differ.** The common ground first: both ask only about the delta, both fix the blast radius first, both record a `[Change]`, both review only the changed part, neither reopens archived work, neither stops unaffected work, and an affected in-progress task pauses, loops back, and continues. The single difference is where the change lands first — development-first has no design pack, so the change goes straight onto the task and PRD; design-first has one, so the change lands in the design pack first (requirements list + prototype), the review reviews the design pack, and affected sub-tasks are brought in line before that review. A brand-new requirement runs the same play in miniature: a small round of clarify → break down → curate context → review, and the new task hangs off the same structure.

### 9.4 Refactoring a legacy module: pin the behavior first, then change the code

Same clothing-store admin (the one adopted into Sunshine in 9.2). This time the target is the old order-amount code: the algorithm originally lived only in the orders API, `src/routes/orders.js`; later the product page needed an estimated price and the inventory page needed stock values, and each copied the code over — three copies, each edited separately, and one of them has already produced a wrong amount once. The refactor's goal is one shared function all three call; the rule is to pin the current behavior down first, then touch the code, and the money must not change by a single cent.

Say it in Claude Code — this time, name the TDD template:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow (the TDD template this time). Refactor the order-amount code in the clothing-store admin: one copy in the orders API, one copied into the product page, one into the inventory page, and the three no longer agree. Pin the current behavior first, then refactor; the amounts must not change by a cent.

Here is how the session continues (illustrative dialogue).

**First, two questions: what gets pinned, and where the boundary is.**

> **Claude Code:** Two things to confirm. One: if the three copies disagree, what then? A: unify on the orders copy outright; B: pin each one as it actually behaves, extract the shared function without changing any result, and list the disagreements for you to decide. I recommend B — a refactor does not change behavior; changing the rule is a separate matter. Two: the boundary — just the amount calculation, or stock valuation too? The valuation uses a variant of the same algorithm.
>
> **You:** B. And the boundary stops at the amount calculation; leave valuation alone this time.

**The PRD's acceptance rests on a baseline.** The agent creates the task and writes the PRD: first a batch of characterization tests pins all three copies' actual outputs as the baseline (four scenario families: plain orders, threshold discounts, shipping fees, partial refunds); the refactor's acceptance is a single line — after the refactor, the same batch matches the baseline item by item, and only one implementation remains across the three call sites. Context is curated (implement.jsonl lists the three copies and the spec's price convention: stored in cents, divided by 100 for display).

**The review checks whether the test list is complete.** At the review (1.5), beyond checking the PRD, the audit scan finds one unpinned scenario family: orders where a coupon and a threshold discount apply together. It goes into the baseline list, you confirm, and only then `task start`.

**Implementation comes in two steps; step one only pins.** implement writes the characterization tests and produces the baseline — changing no business code in this step. Midway, it stops and reports something:

> **Claude Code:** Baseline pinned, all four scenario families covered. But one disagreement was already there: for the same threshold-discount order, the orders copy computes 89700 cents and the product-page copy 89699 — one cent apart, a rounding difference (one sums then rounds, the other rounds per item). A refactor can't settle that; it's a rules disagreement. Which one rules?
>
> **You:** The orders one — sum first, then round.
>
> **Claude Code:** Recorded in the PRD as a confirmed rules correction, and the baseline pins the orders rule; everything else is pinned exactly as it behaves — nothing quietly changed under cover of the refactor.

**Step two is the refactor itself.** With the baseline complete, implement extracts the shared function `src/lib/money.js` and switches the three call sites to it one by one, running the full batch after each switch. When all three are switched, the tests match the baseline item by item (except the one spot you confirmed as a correction, which the PRD records).

**Check reconciles three things.** check does not grade whether the code looks nice; it reconciles: one, the full characterization batch compared against the baseline item by item, results attached; two, a repo-wide grep — the amount algorithm exists only in `money.js`, the other two sites are calls; three, against the spec — prices in cents, divided by 100 for display, and the shared function follows both.

**One anti-copy convention is written back.** This lesson is worth landing in words, or in two years there will be three copies again:

> **Claude Code:** Spec write-back: one line added to `backend.md` — order amounts must go through the shared function in `src/lib/money.js`; copying the algorithm into a page or an endpoint is forbidden; the rule is sum first, then round. If anyone copies it again, check will flag it against this line.

**Commit and archive.** The commit plan still comes to you first: one commit (the shared function + the three switches + the characterization tests + that spec line); on your confirmation it commits, archives, and logs.

The refactor rules in three sentences: pin the behavior before touching the code — the baseline is the contract; when the baseline exposes a disagreement, stop and let a human decide — never quietly change the rules under cover of a refactor; and when the refactor is done, write back one convention that stops the next copy.

### 9.5 Fixing a bug that keeps coming back: find how it was fixed last time

When the same class of problem keeps appearing, the expensive part is investigating from zero every time. Same clothing-store admin: after a successful payment, the stock was deducted twice — this happened once last month, was fixed, and now it is back. Do not rush into the code: first dig out how it went last time — how it was diagnosed, what was fixed, and what was missed.

For finding last time, name memory (the project's long-term memory): it lives inside the project and travels with the repo — another computer, another platform, still there. mem is a different thing: it only searches session transcripts stored on this machine, so if the last fix happened on a different computer, it finds nothing — and finding nothing does not mean it was never fixed. That is why the opening line names memory, not mem.

Say it in Claude Code:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow. Fix an old bug: in the clothing-store admin, stock is deducted twice after a successful payment. It happened last month, was fixed, and it's back. Check memory (the project's long-term memory) first — how was it fixed last time, and what was missed?

Here is how the session continues (illustrative dialogue).

**One memory entry picks the thread straight up.** At the start of a new session, the project long-term memory's index is injected automatically, and the agent spots the entry at once and opens it — a story written when the last task wrapped up, with the root cause, the fix, and the miss all inside:

> **Claude Code:** Found it — the memory entry has the whole story. Last time, the diagnosis was a user submitting the order twice, and the order entry point deducted twice; the fix added a duplicate guard in `src/routes/orders.js`. The root cause was written down plainly even then — "deduction has no idempotency; any entry point that fires twice will deduct again" — but only the order entry point was fixed, the payment one untouched. And the crucial part: the rule went into this story, never into the spec — and the sub-agents doing the work read the spec, so as far as they were concerned, the rule was never set.
>
> **You:** This time it's the payment side: the payment platform retried its callback once, and it deducted again.
>
> **Claude Code:** Same class, different door, then: the root cause survived and came in the other way. This time we fix the class, not just the payment entry point — both entry points get closed off together. Confirm?

(mem never appears in this section: it only digs up this machine's session transcripts. Memory alone picked the thread up this time; only if memory had no such entry, and it happened to be the same machine, would mem be the fallback for the original transcript — in that order, not the reverse.)

**The task's acceptance is written for the class.** On confirmation: task created, PRD written — root cause stated plainly (deduction has no idempotency check); the acceptance is not merely "payment callbacks no longer double-deduct", but: one order, from any entry point, triggered any number of times, deducts exactly once. The memory entry is listed into the context, so implement does not re-derive it. At the review, the audit scan walks the deduction entry points once more and confirms there are exactly two (order placement, payment callback); the inventory page's manual adjustment is a human operation, not automatic deduction, and is explicitly out of scope.

**Implementation closes the class off.** implement extracts a shared deduction function, `src/lib/stock.js`: the order number is the idempotency key — already deducted means return immediately, no second deduction; both entry points are switched to call it. The duplicate guard added at the order entry point last time is deleted in passing — the rule now lives in exactly one place, so the two can never disagree again.

**Check reproduces it for you.** check verifies the acceptance item by item, and re-runs the reproduction: the same order's payment callback fired twice — stock down once; the same order submitted twice — also once; a repo-wide grep shows deduction living only in the shared function, both entry points calling it.

**Write-back lands in two places — last time only one was used.** This time the rule and the story are filed separately:

> **Claude Code:** Write-back, two places. One, the rule into the spec: a line in `backend.md` — stock deduction must be idempotent (keyed by order number; repeated triggers deduct once), must go through `src/lib/stock.js`, and entry points must not write their own deduction; that is a hard rule every task follows. Two, the story into long-term memory: why it happened again and how the two entry points were closed off, as a new entry with a line in the index — so the next occurrence is visible right at session start. Last time only the story was filed and the rule never was: enough to remember the incident, not enough to govern the sub-agents doing the work.

**Commit and archive.** The usual rule: commit plan for your confirmation first, then commit, archive, and log.

A recurring bug, in three sentences: check memory for the last time first — it travels with the project, there on any computer and any platform, with mem only a fallback for this machine's transcripts; fix the class, not just the entry point you happened to hit; and file the lesson in two places — the rule into the spec to govern the work, the story into long-term memory for the why — because filing only one of them is how it comes back.

### 9.6 Design-first: requirements → design documents → breakdown → development

The difference from 9.1 is the mode — 9.1 is a new project developed directly; this scenario designs a new project thoroughly first, then develops it. Same clothing-store admin system. The design output lands in the project's `doc/` (requirements list and prototypes live there); `.sunshine` holds only indexes and pointers.

Open the empty project in Claude Code (after `git init` and a `sunshine init` run), and open like this:

Example prompt (an example, not a record of a real session):

> Please follow the Sunshine workflow, design-first, design only, no development. Build a clothing-store admin system — start with the first requirement's prototype: the navigation menu, in the prototype's index.html.

Here is how the session continues (illustrative dialogue).

**One parent task, then the design loop.** The agent creates only the parent task and does not start it (the design phase has no "start developing"). Then the requirements loop runs, one requirement at a time: clarify → record into the PRD on the spot → write into the requirements list → dispatch a prototype sub-agent for its page. The first requirement you named is the navigation menu; before building a menu, the agent asks about the top-level modules — a menu first has to know what it lists:

> **Claude Code:** Before the navigation, the top-level modules. My suggestion is A: Products, Orders, Inventory, Customers — a clothing admin is these four; say now if you want changes.
>
> **You:** A it is.

**R-001, the navigation menu.** Modules settled, it clarifies the navigation form (left sidebar, content area switching on click), records into the PRD, lands R-001 in the requirements list, and dispatches prototype for `index.html` (the navigation shell + entry points to the four modules). The sub-agent files its report in the parent task's directory (`prototype-R001.md`: one new page, its mock data), you open it, click through, and confirm.

**R-002, product management.** Two rounds of clarification: what the list shows (name, category, price, stock, status); what operations exist (add, edit, on/off shelf; no delete — off-shelf only, your call). R-002 lands, prototype builds the product list page and the edit dialog, you confirm.

**R-003, order management.** Clarify the status flow (pending payment → pending shipment → shipped → completed, plus refunding) and the refund rule (an administrator confirms manually before any refund). R-003 lands, prototype builds the order list page and the order detail page, you confirm.

**R-004 inventory, R-005 customers.** The same loop once each: inventory is about low-stock warnings (below threshold, shown red) and stock in/out records; customers are a list plus each customer's order history. Five requirements done, and the design output all sits in `doc/`:

```text
doc/design/requirements list.md   # R-001 to R-005, each with acceptance criteria
doc/prototype/index.html           # the navigation shell
doc/prototype/R-001-product list page.html, R-003-order list page.html, R-003-order detail page.html …
```

**The wrap-up asks only about entering the review.** The design loop stops, and the agent does not ask about breakdown, let alone development — it asks one thing:

> **Claude Code:** All five requirements are done; the requirements list and prototypes are in doc/. Is this batch enough — shall we enter the review?
>
> **You:** The design is done. Please follow the Sunshine workflow and review it — and give me the final planning summary to confirm.

**The 1.5 review: the design pack is what gets reviewed.** That sentence of yours is the door into the review. The agent first does the review itself: a line-by-line check (requirements list, PRD, and prototypes all line up; every requirement has its prototype report on file — a missing one means that requirement is not done); then an audit scan — this design pack contains rule-type content (order status flow, refunds), so it goes to deep interrogation, which forces out two must-confirms:

> **Claude Code:** Two open points from the review that must be settled now. One: when an order is cancelled, is the deducted stock restored? Two: does a pending-payment order hold stock — deducted at order time, or only at payment time?
>
> **You:** Cancel restores it. Deduct at payment time; an order alone holds nothing.
>
> **Claude Code:** Both recorded back into the PRD. Finally, the final planning summary, presented for your confirmation: the goal is the clothing-store admin, the scope is the five requirements, the breakdown preview is six sub-tasks (selection first), and acceptance runs requirement by requirement against the list. One clarification: this summary is the review's presentation material — the review itself was the checking and interrogation just now. Your confirmation of the summary is what passes the review.
>
> **You:** Confirmed — review passed.

**Only after the review, the breakdown.** Breakdown (1.2) makes one sub-task per requirement, with a "technical selection and scaffolding" sub-task in front: the selection items (front/back-end stack, database, cache) are marked `[To be decided]` in the PRD, left for that sub-task's own 1.1 — breakdown does not write back into the spec. The agent presents the list for your confirmation: 6 sub-tasks with their risk tiers (the order-status one is standard tier, the rest light), all in planning, none starting on its own.

**Development takes over, one sub-task at a time.** Starting with the first, and the next starts only when the previous one is fully done. Selection and scaffolding goes first: its own 1.1 settles the selection with you (Node + Express, SQLite for now, your confirmation), its own 1.5 passes, `task start`, a runnable skeleton goes up (page opens, APIs answer, database connected), check, spec write-back (the settled selection is only now written into system / frontend / backend), commit, archive. Then the R-001 navigation sub-task runs the same line: its own review → implement → check → write-back → commit → archive. The remaining four follow one by one.

**The parent task wraps up.** With all six sub-tasks archived, the parent does a final integration review: the five modules clicked through together, each requirement signed off against the requirements list. Last, the delivery documents: the seven placeholders generated at init sit in `doc/delivery/`; the agent lists them and asks which to generate — you pick the high-level design and the deployment guide, it generates those two, and the rest stay placeholders. The parent task archives; the project is closed.

The whole design-first line is this: requirements loop (clarify → record in the PRD → requirements list → prototype) → wrap-up asks about the review → 1.5 reviews the design pack → breakdown → sub-tasks developed one by one → the parent's integration wrap-up. Its biggest difference from development-first: the review comes before the breakdown, and what it reviews is the design pack, not one task's PRD.

## Appendix: command reference

That is the whole mainline: you ran `init` once, asked for the refund validation in plain language in the agent UI, the agent planned, implemented, checked, wrote the lessons back into the spec, and finished with `/sunshine:finish-work`; you reviewed the diff and accepted; the task is archived and the session is in the journal.

Everything else in Sunshine is something you reach for when you step in manually. These are the only commands you ever need to type yourself — every one of them was run for real earlier in this tutorial (the one-line descriptions of `uninstall` / `ablate` / `restore` come from their own `--help`, not from a run here).

Everything not listed here — `task create/start/archive`, `task add-context/validate`, `channel create/send/spawn/wait`, `task add-session` — is the agent's job, driven by `workflow.md` (see section 3). If you find yourself typing those by hand every day, something is wrong: open the agent UI and let it drive.

### init reference

#### What init asks (without `-y`, four questions in order)

1. **Pick platforms** ("请选择要配置的 AI 编程平台", "Select the AI programming platforms to configure"): a multi-select of all 22 platforms, with none checked by default; selecting none re-prompts until at least one is picked (Ctrl+C cancels).
2. **Statusline confirmation** (only when Claude Code is selected): "是否为 Claude Code 安装状态栏……？" ("Install the statusline for Claude Code…?"), default no; pass `--with-statusline` to install it directly.
3. **Pick a spec template** ("请选择规范模板", "Select a spec template"): "从零开始" (from scratch) by default, or "自定义" (enter a marketplace source and pick from it); with `--registry` the picker lists that marketplace's spec templates, and the picked one is installed instead of the blank specs.
4. **Pick a project type** (only when nothing is detected): "检测不到项目类型，请选择" ("Project type not detected, please choose"): frontend / backend / fullstack / product design, default fullstack. Picking product design means the project starts design-first: it generates the full spec set — `system.md`, `frontend.md`, and `backend.md` — plus the `doc/` design templates (requirements list, outline design, prototype).

`-y` skips all four questions and uses the defaults; passing the matching flag (a platform flag, `--type`, `--template`) skips that question too.

Project-type detection reads `package.json` dependencies and common frontend/backend config files (`vite.config.ts`, `go.mod`, etc.) and generates only the matching specs — a frontend project gets `system.md` and `frontend.md`, a backend project gets `system.md` and `backend.md`, a fullstack project gets all three. Product design is never detected; pick it manually (interactive question or `--type 产品设计`). With `--registry` (specs from the template marketplace) there is no detection and no question.

#### Common options

| Option | What it does |
| --- | --- |
| `-u, --user <name>` | Developer name, written to `.sunshine/.developer`; priority: `-u` > existing `.developer` > git user.name > `developer` (re-running init without `-u` keeps the recorded name); new tasks default their creator and assignee to it |
| `-y, --yes` | Skip all prompts: a platform flag is required to pick at least one platform, default fullstack, default blank specs, no statusline |
| `-f, --force` | Rebuild the skeleton and platform files even when already initialized (see "Three modes" below) |
| `--type <前端/后端/全栈/产品设计>` | Specify the project type explicitly (前端=frontend, 后端=backend, 全栈=fullstack, 产品设计=product design), skipping detection and the question |
| `--registry <source>` | Template marketplace source: `gh:owner/repo`, a full git URL, an HTTP(S) URL, a `git@` address, or a local git repository path |
| `--template <id>` | Marketplace entry id, must be used with `--registry`; when omitted, a marketplace source errors out under `-y` and shows the picker interactively |
| `--overwrite` / `--append` | Overwrite the target if it exists / only fill in missing files; only with `--template` |
| `--with-statusline` | Install the statusline for Claude Code; only effective when Claude Code is selected |
| `--<platform>` | Install only the named platform: `--claude` `--cursor` `--codex` `--kiro` `--gemini` `--qoder` `--pi` `--zcode` `--trae` `--omp` `--snow` `--opencode` `--dsh` `--reasonix` `--grok` `--kimi` `--kilo` `--antigravity` `--devin` `--codebuddy` (WorkBuddy users: use this one) `--copilot` `--droid` (22 in total; `--windsurf` is a deprecated alias for `--devin` and is converted automatically with a warning) |

#### Three modes

- **Fresh init**: no `.sunshine/` in the directory yet; generates the skeleton and platform configs following the flow above.
- **Rebuild** (`-f`): rebuilds the skeleton and platform files even when already initialized.
- **Incremental**: running `init` again without `-f` in an initialized project only adds the newly selected platforms (and can also install a marketplace template with `--registry`), leaving existing platform files and the `.sunshine/` skeleton untouched. For example, `sunshine init --codex -y` adds only Codex.

Note: `init` refuses to run in your home directory (set `SUNSHINE_ALLOW_HOMEDIR=1` to bypass).

#### What init installs

What `init` just installed — 115 files, in two groups:

- **For the agent to read — `.sunshine/` (17 files):** `workflow.md` (714 lines, the full development workflow the agent follows), `spec/` with three spec templates (`system.md`, `backend.md`, `frontend.md`), `config.yaml` (project config with Chinese comments), `AGENTS.md`, and `agents/` (five agent definitions). `AGENTS.md` opens with:
  ```text
  # AGENTS.md（Sunshine）

  本文件面向在本仓库工作的 AI 编程助手。

  ## 工作方式

  - 先读 `.sunshine/spec/` 与 `.sunshine/workflow.md`，再动手。
  ```
  ("This file is for the AI programming assistant working in this repository. / How to work: first read `.sunshine/spec/` and `.sunshine/workflow.md`, then start.")
- **For the platforms to load — `.claude/` and `.cursor/` (98 files):** skills, commands (`/sunshine:continue`, `/sunshine:finish-work`), sub-agents (`sunshine-implement`, `sunshine-check`, `sunshine-research`), and hook configs. These are what make the agent know Sunshine's workflow at all.

Check what is configured:

```sh
sunshine platforms
# 已配置的平台：
#   Claude Code (claude-code) — .claude
#   Cursor (cursor) — .cursor
```

("Configured platforms: …") By default `init` installs Claude Code and Cursor. To add Codex later, run `init --codex -y` again in the same project — it only adds the new platform and leaves existing files untouched: There are 22 supported platforms in total; install a specific one with `sunshine init --<platform> -y`, e.g. `sunshine init --gemini -y`. With no platform flag, `init` still installs only Claude Code and Cursor.

```text
正在配置 Codex……
已安装平台：
  - Codex
    ↳ 已开启 Codex hooks（~/.codex/config.toml），仍需在 Codex 里运行一次 /hooks 审批
```

("Configuring Codex… / Installed platforms: Codex / Note: `init` now writes `features.hooks = true` into `~/.codex/config.toml` for you (Codex 0.129+; older versions use `codex_hooks = true`) after asking (default yes; `-y` writes it directly); you still need one `/hooks` approval run inside Codex, otherwise `.codex/hooks.json` does not take effect.")

### Manual-intervention cheat sheet

| When | Command | What it does |
| --- | --- | --- |
| Where did we get to? | `sunshine task list` / `sunshine task current` / `sunshine task context` | Show active tasks, the current task, and the full session context |
| Check a verification slip | `sunshine task verify <task>` | Who still owes integration verification and how far sign-off has got; the same family covers opening (`verify-open`), signing (`verify-sign`), waiving (`verify-waive`) and reassigning (`verify-assign`) |
| Check the next step | `sunshine task list-archive` | Show archived tasks by month |
| Switch how the agent works | `sunshine workflow --list` / `sunshine workflow --template tdd` | List or switch workflow templates ([section 2](#switching-workflow-templates)) |
| Refresh project templates | `sunshine update --dry-run` (drop `--dry-run` to apply) | Sync template files; user data is never touched ([section 8](#8-maintenance-update-and-upgrade)) |
| Install a spec from a marketplace | `sunshine init --registry <source> --template <id>` | Install a spec template, refreshed by `update` afterwards ([section 8](#8-maintenance-update-and-upgrade)); in interactive mode, omit `--template` to pick from a list |
| All init operations | `sunshine init --help` | Platforms, statusline, template picker, project type, three modes — see the init reference in this appendix |
| Look back | `sunshine mem search "退款"` / `sunshine mem list` | Search local session history, read-only ([section 5](#5-memory-mem-and-long-term-memory)) |
| Multi-agent work | `sunshine channel list` / `sunshine channel messages <name>` | Inspect channels and their event timelines ([section 7](#7-multi-agent-collaboration)) |
| Update the CLI itself | `sunshine upgrade --dry-run` (drop `--dry-run` to apply) | Upgrade the `sunshine` binary ([section 8](#8-maintenance-update-and-upgrade)) |
| See what is configured | `sunshine platforms` | Show the platforms configured for this project |
| Install / build the code map manually | `sunshine codegraph install` / `sunshine codegraph status` | init asks about the tool on first run and builds graphs only for existing projects; a manual run is not gated on that check and re-wires on every run |
| Remove Sunshine from a project | `sunshine uninstall` / `sunshine ablate` / `sunshine restore` | Delete all Sunshine files, temporarily remove them (reversible), or restore exactly what `ablate` saved |
