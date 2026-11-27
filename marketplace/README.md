# Sunshine 模板市场

这里是 Sunshine 官方模板市场：`sunshine init --registry` 的来源。用法：

```bash
# 装 spec 模板（只有一个 spec 条目时 --template 可省，自动选）
sunshine init --registry gh:linrong007/Sunshine/marketplace --template sunshine-fullstack -y

# 装 skill 模板
sunshine init --registry gh:linrong007/Sunshine/marketplace --template go-sdk-author -y

# workflow 模板不走 --registry，用 sunshine workflow --template
sunshine workflow --template channel-driven-subagent-dispatch
```

## 目录

- `specs/`：spec 模板，一子目录一个（装进 `.sunshine/spec/`）
- `skills/`：skill 模板，一子目录一个（装进 `.agents/skills/<id>/`）
- `workflows/`：workflow 模板，一子目录一个（走 `sunshine workflow --template` 安装）

`index.json` 登记所有条目：id、type（spec / skill / workflow）、name、path（相对本目录）为必填。

## 同步规则

这里的文件是 `templates/` 下源文件的镜像：改了源文件，同步一份到这里。

- `specs/sunshine-fullstack/` ← `templates/sunshine/spec/`（三份默认规范，带适用标签）
- `skills/go-sdk-author/`、`skills/mem-recall/` ← `templates/marketplace/skills/`
- `workflows/*/` ← `templates/workflows/*/workflow.md`
