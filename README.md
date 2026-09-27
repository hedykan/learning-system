# Personal Learning OS

Personal Learning OS 是一个本地优先的学习 Runtime。`learn` CLI 管理教材、学习位置、原始对话、Session 和 Git 历史；Markdown Vault 可以直接用 Obsidian、编辑器、Codex 或 Claude 打开。

当前版本：`v0.1.5`

## 安装

需要 Go 1.23 或更高版本，以及本机 Git。支持 macOS、Linux 和 Windows；Windows 上请确认 `git` 与 `go env GOPATH` 下的 `bin` 目录都在 PATH 中。

```bash
go install ./cmd/learn
learn version
```

如果找不到 `learn`，将 `go env GOBIN` 的结果加入 `PATH`；该值为空时使用 `$(go env GOPATH)/bin`。

## 快速开始

```bash
learn init ~/Learning
cd ~/Learning
learn status
```

`init` 会创建普通 Markdown Vault、初始化 Git，并写入 Codex/Claude 可发现的 Rules 和 Skills。这些模板编译在 `learn` 二进制中，安装后不依赖本源码仓库。重复初始化会保留已有文件。

## 导入教材

先预演，再正式导入：

```bash
learn curriculum import "/path/to/book.pdf" \
  --id my-book \
  --title "My Book" \
  --activate \
  --dry-run \
  --json

learn curriculum import "/path/to/book.pdf" \
  --id my-book \
  --title "My Book" \
  --activate \
  --yes
```

支持 Markdown、纯文本、PDF 和只含 Markdown/文本的目录。v0.1 管理 PDF 原件和元数据，不提取 PDF 正文。默认把教材复制进 Vault；明确需要外部引用时使用 `--link`。

设置教材位置：

```bash
learn curriculum position set \
  --chapter "2" \
  --section "2.3" \
  --concept "极限"
```

教材原件保存在 `Sources/`，学习进度保存在 `Curriculum/`；两者不会混在一起。

## 学习首页（v0.1.4）

Vault 根目录的 `README.md` 是自动生成的学习首页：正在学什么、学到哪一节、下一步做什么，以及每本教材、学习者总览、最近学习和最近变化的概念。首页末尾的「手写笔记」区属于你，重建时保留。

## 复习与知识图谱（v0.1.5）

- **间隔复习**：概念形成理解后按 1、2、4、7、15、30、60 天排期，答错回到第一档。`learn review` 查看今天到期的概念，首页也会显示；说“继续学习”时 Agent 会先安排到期复习。
- **知识图谱**：概念笔记之间有“相关概念”双链，Obsidian 的关系图谱会显示成一张网。在图谱设置的“分组”里按标签着色，例如 `tag:#learning/state/stable`、`tag:#learning/state/fragile`。

## 教材目录与进度（v0.1.3）

导入后先建立目录，学习位置才能和原书对上：

```bash
learn curriculum outline set --file outline.json --dry-run   # 预演
learn curriculum outline set --file outline.json             # 保存为草稿
learn curriculum outline confirm                             # 你确认后生效
learn curriculum position set --node 2.3                     # 位置只能指向目录条目
learn curriculum complete 2.3 --reason "能分析热点分片"       # 学完一节打勾
learn curriculum skip 1 --reason "暂时跳过"                   # 主动跳过
```

Markdown 教材会自动生成目录草稿；PDF 目录由 Agent 读原书后提交，你确认才生效。`Curriculum/<教材>/index.md` 和 `progress.md` 会显示每一节是已完成、已跳过、进行中、未开始，还是被越过的“未覆盖”。

## 学习 Session

```bash
learn session start
```

新教材第一次开始时会自动进入 `standard` 摸底。Agent 一次问一个问题，根据学习者的原话形成 Assessment；摸底完成前不会推进教材位置。之后的正式学习默认采用 Guided Socratic 循环：先引出当前理解，再预测、解释、质疑、修正和迁移；每轮最多引入两个尚未建立的新术语。

如果必须跳过摸底，可以显式运行：

```bash
learn session start --kind lesson --skip-baseline
```

Conversation 是 append-only 的事实记录，每轮都有稳定编号（`t0001`、`t0002`……）。`learn session abort --reason <原因>` 可以安全结束错误或过时的会话，不会推进教材进度。

## 学习记录与笔记（v0.1.2）

学完一个概念，Agent 会提交一条 Interpretation Record：学习事件、认知变化、概念状态、教学策略效果和学习模式。每条判断都必须引用你的原话，Runtime 校验不通过就整条拒收。

```bash
learn session checkpoint --analysis-file record.json   # 学习中途沉淀
learn session end --analysis-file record.json          # 结束并记录进度决策
learn session annotate <session-id> --analysis-file -  # 为旧 Session 回填
learn next                                             # 基于历史推荐下一步
learn state concept <concept-id>                       # 查看某个概念的认知历史
learn curriculum outline [set|confirm]
learn curriculum complete|skip <node>
learn curriculum deactivate|remove|archives|restore|purge
learn curriculum detour start|end                      # 记录补先修知识的绕行
learn model rebuild [--dry-run]                        # 从记录重建模型和笔记
```

记录被接受后，Obsidian 里会自动出现：

- `Concepts/<概念>.md`：你的原话、当前理解、误解与修正、策略记录、修订历史。
- `Profile/learner-state.md`：各概念状态、待修正误解、有效策略、学习模式和推荐下一步。
- `Curriculum/<教材>/index.md`：教材位置与理解状态并列展示。
- `Sessions/<session>.md` 里的学习解读区块。

这些文件都能从记录重建。只有「手写笔记」区域属于你，重建时逐字保留。

## 使用 Codex 或 Claude 学习

```bash
cd ~/Learning
codex
# 或 claude
```

然后可以说：

> 继续学习。

或者：

> 我的教材在 `/path/to/book.pdf`，请预演导入，确认无误后激活它。

Agent 会根据 Vault 中的 Rules/Skill 调用 `learn`。v0.1 不能秘密读取宿主聊天历史，因此 Skill 要求 Agent 显式调用 `learn session append` 记录每个学习回合。

旧 Vault 升级到新教学规则：

```bash
learn agent update --dry-run
learn agent update --yes
```

正式更新会先备份被替换的 Rules/Skills。

## 常用命令

```text
learn init [path]
learn status [--json]
learn curriculum [--json]
learn curriculum list|import|show|activate|position
learn session start|append|turns|checkpoint|end|annotate|abort
learn curriculum detour start|end
learn next [--json]
learn state [concept <id>] [--json]
learn model rebuild [--dry-run]
learn review
learn commit
learn agent update
learn version
```

所有状态查询提供 `--json`，供 AI Agent 稳定读取。运行 `learn <command> --help` 查看完整参数。

## 开发

需求入口在 [`spec/`](./spec/README.md)，阶段计划在 [`development-plan/`](./development-plan/README.md)。新增产品想法先进入 `spec/ideas.md`；冻结版本的行为变更必须走 `spec/changes/`。

```bash
make test
make test-race
make vet
make build-all
```

## 边界

v0.1.2 完成“学习历史、认知变化、Learner Model、教学策略、下一步动作”的核心闭环。解读由 Agent 提交，Runtime 只做证据校验和确定性推导，不接入网络模型。PDF 提取、OCR、自动复习、Dashboard、知识图谱和 Source 删除归档仍是后续候选能力。
