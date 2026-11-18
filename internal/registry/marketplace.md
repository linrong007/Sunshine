# Sunshine 模板市场（Marketplace）说明

本文档描述 `sunshine init --registry` 接受的模板仓库格式。
结构以实查为准（2026-11-09 核查官方模板仓库
`mindfold-ai/marketplace` 的 GitHub API 列表），不照抄上游
`marketplace/README.md`——该 README 称含 `commands/` 与 `agents/`
目录，实查并无此二目录。

## 仓库结构

```
<仓库根>/
├── index.json        # 模板索引（version 1），有它就是模板市场模式
├── specs/            # spec 模板，每子目录一个模板
├── skills/           # skill 模板，每子目录一个 skill
├── workflows/        # workflow 模板，每子目录一个工作流
└── README.md         # 可选
```

没有 `commands/`、`agents/` 目录。`index.json` 在仓库根
（不是 `marketplace/index.json`——那是 Trellis 单体仓库里
`marketplace/` 子目录的写法；用 `--registry` 指过去时，
来源要写到 `marketplace` 子目录，如 `gh:mindfold-ai/Trellis/marketplace`）。

## index.json

```json
{
  "version": 1,
  "templates": [
    {"id": "electron-fullstack", "type": "spec", "name": "...", "path": "specs/electron-fullstack"},
    {"id": "sunshine-meta", "type": "skill", "name": "...", "path": "skills/sunshine-meta"},
    {"id": "native", "type": "workflow", "name": "...", "path": "workflows/native/workflow.md"}
  ]
}
```

- `id`、`type`、`name`、`path` 为必填；`description`、`tags` 可选。
- `path` 相对 registry 根（index.json 所在目录）。
- 已确认的 11 个条目：3 spec（`electron-fullstack`、`nextjs-fullstack`、
  `cf-workers-fullstack`）、5 skill（`sunshine-meta`、
  `sunshine-spec-bootstrap`、`frontend-fullchain-optimization`、
  `mem-recall`、`go-sdk-author`）、
  3 workflow（`native`、`tdd`、`channel-driven-subagent-dispatch`）。
  上游 `trellis-spec-bootstarp` 是错拼，Sunshine 用
  `sunshine-spec-bootstrap`，不沿用。`mem-recall` 已收录（`sunshine mem`
  的用户入口）。`go-sdk-author` 已收录（Go SDK 编写 skill，
  对应上游的 `ts-sdk-author`，内容已按 Go 重写）。

## 安装位置

| type    | 安装目录                    |
|---------|-----------------------------|
| spec    | `.sunshine/spec`            |
| skill   | `.agents/skills/<id>`       |
| command | `.claude/commands/<id>`     |
| full    | 项目根                      |
| workflow| 不走 `--registry`，请用 `sunshine workflow --template <id>` |

## 两种模式

- **模板市场模式**（有 `index.json`）：用 `--template <id>` 指定条目；
  市场里恰好只有一个 spec 模板时 `--template` 可省，自动选中它；
  多个 spec 模板时必须指定，没有 spec 模板时报错。自动选只针对
  spec，skill 安装仍须显式 `--template`。
- **直接下载模式**（无 `index.json`）：整个 registry 内容装进
  `.sunshine/spec`；若 registry 根下存在 `marketplace/specs/`，
  以该目录为模板；否则以 registry 根本身为模板（私有 spec 仓库）。

## 来源写法

- `gh:owner/repo`，可带分支/子目录：`gh:owner/repo/marketplace#dev`
- 完整 git URL、`https://github.com/owner/repo`（含 `/tree/<分支>/<路径>`）
- SSH：`git@github.com:owner/repo`、`ssh://git@host:2222/owner/repo`
- 本地 git 仓库路径（Sunshine 补充）：`/path/to/repo`、`./repo`、`file:///path/repo`
  （可带子目录，如 `/path/to/repo/marketplace`）

## 本地路径与局域网

- 本地路径：绝对路径、`./` 与 `../` 开头的相对路径、`file://` 均可；
  带子目录时从该目录往上找第一个含 `.git` 的目录当仓库根
  （`splitLocalRepo`），剩余部分为 subdir。
- 本地路径同样走克隆：只认已提交内容，未提交文件拉不到（报模板
  不存在），须先 commit（不用 push）。
- 挂载的共享文件夹（NAS/共享盘）当本地路径用。
- 局域网 git 服务器走 SSH（`git@host:repo`、`ssh://git@host:port/repo`；
  认证先 ssh-agent，后 `~/.ssh` 密钥文件，见 `git.go` 的 `sshAuth`）——
  代码核对，未在局域网实测。
- 局域网 plain `http://` 目前走不通：解析接受（`httpsUnknownRe` 匹配
  `http://`），但 `buildGitURL` 一律拼成 `https://`，对着只听 http 的
  服务器报 TLS 错误（已实测）。`git://` 不支持（provider `git` 不在
  `rawURLPatterns` 里，报“不支持的 provider”）。自签证书的 https
  没有跳过校验的选项（`CloneOptions` 未配 TLS）。

## 冲突策略

默认 `append`（只补缺失文件，已有的不动）；`--overwrite` 整体替换
（清空目标目录再装，包括自己写的文件）；`--append` 与默认同义。
`--overwrite`/`--append` 不再要求 `--template`：校验移到目标解析后，
自动选能补上模板时放行；市场模式下目标仍无模板（多个 spec 未指定）
时报错。

## 官方市场

Sunshine 官方模板市场在仓库根的 `marketplace/` 目录
（`gh:linrong007/Sunshine/marketplace`），6 个条目：1 spec
（`sunshine-fullstack`，默认三份中文规范）、2 skill
（`go-sdk-author`、`mem-recall`）、3 workflow（`native`、`tdd`、
`channel-driven-subagent-dispatch`）。`marketplace/` 下的文件是
`templates/` 的镜像，改了源文件要同步一份过去
（见 `marketplace/README.md` 的同步规则）。

## 适用标签与规范地图

`.sunshine/spec/` 下所有 `.md` 都认，不限于默认三份。每个文件头有
YAML frontmatter `适用：前端 / 后端 / 通用`；缺了按文件名自动补
（`frontend`→前端，`backend`→后端，`system`→通用，其他→通用）；
模板作者写了的以作者的为准，安装不改写；补标签放在 `spec index`
同步里顺手做（`internal/specindex/applicability.go`）。

SessionStart 注入一张规范地图：spec/ 下全部 `.md`（递归），按适用
分组，一行一个（路径 + 索引章节名）；前端任务读前端组 + 通用组，
后端任务读后端组 + 通用组。`task context --mode packages` 输出
同一张地图（text），JSON 输出保留 `specLayers` 并新增 `specFiles`。

## 查重

`sunshine spec check` 比对各文件索引章节，标题相同或内容相同的节
列出来；安装后自动跑一遍，只报告、不阻止安装。无重叠打印
“未发现重复”并退出 0，发现重复退出 1（供 CI 用）。

## 持续刷新

装完后来源与模板记进 `.sunshine/config.yaml` 的 `registry.spec`，
`sunshine update` 按它重新拉取 spec 模板，本地改动走正常冲突流程。
