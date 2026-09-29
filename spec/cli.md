# CLI 契约

所有命令支持 `--help`；查询类命令支持 `--json`，便于 AI Agent 稳定消费。错误写入 stderr，并返回非零退出码。

## 安装

```bash
go install ./cmd/learn
learn version
```

安装目标为 Go 标准的 `GOBIN`，未设置时使用 `GOPATH/bin`。该目录需要在 `PATH` 中。

发布：推送 `v*` 标签时，GitHub Actions（`.github/workflows/release.yml`）运行测试并用 `scripts/release.sh` 打包 macOS（x64、arm64，裸二进制）、Windows（x86、x64、arm64，`.exe`）与 Linux（x86、x64、arm64、armv7，AppImage 与 tar.gz），连同 `SHA256SUMS.txt` 上传到同名 Release，说明取自 CHANGELOG 对应段落。打包前校验 `learn version` 与标签一致。手动运行工作流并填入已有标签可补发旧版本。

## Vault

```bash
learn init [path] [--language zh|en]
learn status [--json]
learn config show [--json]
learn config set language <zh|en>
```

`init` 可重复执行，不覆盖任何已有用户文件。

界面语言（v0.1.7）：`.learning/config.yaml` 的 `language` 决定生成页面的固定文字与固定页面名（`学习者总览.md` / `Learner overview.md`，`学习进度.md` / `Progress.md`），缺省为 `zh`。`config set language` 保存后立即重建页面，旧语言的固定页面被删除，手写区迁移到新页面。已有 Session 文件的标题保持创建时的语言，其解读区按新语言重建。命令行输出始终为英文。`status --json` 输出 `language`。命令从当前目录向上发现 `.learning/schema-version`，也可用 `--vault` 显式指定。

## Curriculum

```bash
learn curriculum
learn curriculum list [--json]
learn curriculum import <path> --id <id> [--title <title>] [--copy|--link] [--activate] [--dry-run] [--yes] [--json]
learn curriculum show <id> [--json]
learn curriculum activate <id>
learn curriculum position [--json]
learn curriculum position set [--node <id>] [--chapter <value>] [--section <value>] [--concept <value>]
learn curriculum position reset
learn curriculum outline [show] [id] [--json]                                                          # v0.1.3
learn curriculum outline set [id] --file <path|-> [--dry-run] [--json]                                  # v0.1.3
learn curriculum outline confirm [id]                                                                  # v0.1.3
learn curriculum complete <node-id> --reason <text> [--json]                                           # v0.1.3
learn curriculum skip <node-id> --reason <text> [--json]                                               # v0.1.3
learn curriculum deactivate                                                                             # v0.1.4
learn curriculum remove <id> [--dry-run] [--reason <text> --yes] [--json]                               # v0.1.4
learn curriculum archives [--json]                                                                      # v0.1.4
learn curriculum restore <archive-id>                                                                   # v0.1.4
learn curriculum purge <archive-id> --yes --confirm <archive-id>                                        # v0.1.4
learn curriculum detour start --topic <topic> --reason <reason> --return-condition <condition> [--json]   # v0.1.2
learn curriculum detour end --outcome <completed|abandoned> --learned <text> [--json]                     # v0.1.2
```

对非交互 Agent，正式导入必须带 `--yes`；推荐先调用 `--dry-run --json`。`--copy` 为默认模式。

## Session

```bash
learn session start [--domain <domain>] [--kind <baseline|lesson|review|practice>] [--depth <quick|standard|deep>] [--skip-baseline] [--json]
learn session append --role <user|assistant|system> [--text <content>] [--json]
learn session turns [--session <id>] [--role <user|assistant|system>] [--json]                          # v0.1.2
learn session checkpoint --analysis-file <path|-> [--json]                                               # v0.1.2
learn session end [--analysis-file <path|->] [--assessment-file <path|->] [--no-analysis --reason <reason>] [--json]
learn session annotate <session-id> --analysis-file <path|-> [--json]                                    # v0.1.2
learn session abort --reason <reason> [--json]
```

新教材在没有 Assessment 时，无参数 `session start` 默认开始 `standard` baseline；显式跳过必须使用 `--kind lesson --skip-baseline`。Baseline 结束时必须通过 `--assessment-file` 提交带学习者原话证据的 JSON，且摸底期间禁止推进 Curriculum Position。未提供 `--text` 时 `append` 从 stdin 读取，以避免复杂内容的 shell 转义问题。

`abort` 保留原始 Conversation 和中止原因，但不会推进教材进度或生成掌握证据。

### v0.1.2 Session 行为

- `append` 可用 `--text`、`--file` 或管道输入；标准输入是终端且没有其他来源时立即报错（v0.1.3）。
- `append --json` 返回 `{"turn": "t0012", "role": "user"}`，Agent 用它构造证据引用。
- `turns` 列出 turn ID、角色和原文，默认当前 Session，用于中断恢复。
- `checkpoint` 提交增量 Interpretation Record，校验后立即更新 Learner Model 与投影；Session 保持活动。重复提交同一记录返回 `unchanged`。
- lesson、review、practice 的 `end` 必须满足以下之一：带 `--analysis-file`、此前至少成功 checkpoint 一次，或显式 `--no-analysis --reason`。否则拒绝结束，Session 保持活动。
- baseline 的 `end` 仍要求 `--assessment-file`；可同时提交 `--analysis-file`。
- `annotate` 为已结束或已中止的 Session 追加 Interpretation Record，用于回填 v0.1.1 的旧对话；它不改变 Curriculum Position。
- Record 格式、校验与幂等规则见 [Interpretation Record 规范](./interpretation-record.md)。

### 目录与完成记录（v0.1.3）

- 目录为 `nodes` 列表，条目含 `id`（如 `2.3`）、`title` 与可选 `pages: [start, end]`，按阅读顺序排列。`outline set` 校验 ID 唯一、父条目先出现、顺序递增、页码合法，结果为 `draft`；学习者确认后 `outline confirm` 置为 `confirmed`。Markdown Source 导入时自动生成草稿目录。
- 目录确认后，`position set` 必须用 `--node`，章节与小节标题由目录填充；自由文本的 `--chapter`、`--section` 被拒绝。`--concept` 仍可自由填写。
- `complete`、`skip` 向 `progress.yaml` 追加记录，baseline 期间拒绝。`学习进度.md` 与以书名命名的教材首页由 Runtime 生成，显示每个条目的已完成、已跳过、进行中、未覆盖或未开始。
- `status --json` 新增 `outline`、`position_verified`、`uncovered` 与 `git_last_commit`。

### 自动推进（v0.1.6）

`complete` 或 `skip` 标记的正是当前位置、且不在 Detour 中时，位置自动移到下一个未完成条目，输出与 JSON 的 `moved_to` 报告新位置；全书最后一节完成时位置不变（CR-2026-017）。

### 状态与首页（v0.1.4）

- 目录条目新增状态 `partial`（学过一部分）：有 Session 在该条目结束，或有带证据的 Concept 挂在该条目上，且未完成、未跳过、不是当前位置。`status --json` 新增 `partial` 列表。
- `init`、导入、激活、目录与位置变更、完成与跳过、Detour、checkpoint、Session 结束与中止、`model rebuild` 都会刷新根目录 `README.md` 首页；没有学习记录时首页也会生成。
- `agent update` 预演列出 v0.1 遗留的空目录（Questions、Ideas、Hypotheses、Misconceptions、Insights），`--yes` 时只删除空目录，含用户文件的目录保留。

### 教材生命周期（v0.1.4）

- `remove` 默认可恢复：把 `Sources/<id>` 与 `Curriculum/<id>` 移到 `.learning/archive/<archive-id>/`，清除匹配的激活教材；Conversation、Session、记录与 Git 历史不动。活动 Session 使用该教材时拒绝。`--dry-run` 零写入。
- `restore` 在同 ID 已被占用时拒绝。归档后可用原 ID 重新导入，导入计划的 `archived_match` 提示内容相同的归档。
- `purge` 永久删除归档，需要 `--yes` 与重复一次归档 ID 的 `--confirm`。

## 资料（v0.1.8）

```bash
learn curriculum import --external --id <id> --title <name> [--url <url>] [--note <text>] [--dry-run|--yes] [--activate]
learn source add <path> --id <id> [--title <name>] [--link] [--json]
learn source add --external --id <id> --title <name> [--url <url>] [--note <text>] [--json]
learn source list [--json]
learn source outline <source> [--json]
learn source read <source> <kind> <value> [--json]
learn source attach <node> <source> <kind> <value> [--curriculum <id>] [--json]
learn source detach <node> <source> [--curriculum <id>]
learn source check [curriculum] [--json]
```

- 来源 ID：课程自身的资料用课程 ID；其他资料（`Sources/<id>/resource.yaml`）不会出现在课程列表中，可被多门课程共用。
- 每份资料声明 `capabilities`：`structured`（可出草稿目录）、`extractable`（`source read` 可取文字，统一为 Markdown）、`needs_vision`、`external`（没有文件）。读不了时 `source read --json` 返回 `status: unsupported` 与原因。
- 定位 `locator`：`{"resource", "kind", "value"}`，kind 为 `page`（`42-45`）、`anchor`（`#slug`）、`file`（`a.md#L10-40`）、`time`（`3/05:20-48:00`）、`chapter`、`text`。目录条目与 `textbook_points` 可用 `locator` 代替 `pages`；可比较的位置必须落在上级条目内。每种资料只接受适合它的定位：PDF 为 page，Markdown 为 anchor、file，外部资料为 page、time、text。
- `source attach` 只能挂到已确认大纲的条目上；挂载记录在 `Curriculum/<id>/resources.yaml`。`next --json` 的 `resources` 与 `status --json` 的 `node_resources` 列出当前小节的资料位置。
- `source check` 报告失效的定位，以及外部课程中没有任何资料位置的条目（`unsourced`）。
- 文件夹资料按自然顺序读取（`ch2` 在 `ch10` 前）。

## 内嵌 Git（v0.2.3）

所有 Git 操作由内嵌的 go-git 完成，不需要安装 `git` 命令。提交身份依次取：`GIT_AUTHOR_*` / `GIT_COMMITTER_*` 环境变量、学习库仓库配置、用户全局 Git 配置，最后回退为 `Learning OS <learn@localhost>`。新学习库默认分支为 `main`。go-git 不执行 Git 钩子。

## 多个学习库（v0.2.2）

```bash
learn vaults [dir] [--json]
learn --vault <path> <command> ...
```

`vaults` 列出 `dir`（缺省为当前目录）本身及其下两层中的学习库，不进入学习库内部继续查找，跳过隐藏目录。每个库输出路径、名称、界面语言、当前课程、课程列表（ID、标题、类型、资料种类、目录状态、已完成与总的叶子条目数，跳过计入已完成）、进行中的 Session 与今天到期的复习数；读取失败的库带 `error`。只读。选定库后，其余命令用全局参数 `--vault` 指定。

## 入学（v0.2.1）

```bash
learn curriculum goal show [id] [--json]
learn curriculum goal set [id] --file <path|-> [--json]
learn curriculum outline review [id] [--json]
learn source attach <node> <source> <kind> <value> [--why <text>] [--serves <focus-id>]
```

- 目标卡字段 `outcome`（必填）、`context`、`background`、`constraints`、`success_criteria`，各为 `{"text", "evidence"}`；`focus` 为 `{"id", "text", "evidence"}` 列表。`evidence` 必须是学习者原话摘录。来源：摸底评估的 `goal_card`，或 `goal set`（证据对照该课程进行中的 Session）。每次修改保存旧版本到 `goal-history/`。
- 评估发现可带 `concepts`；`outline review` 输出每个条目的 `hints`（`finding` 为 existing_knowledge、prerequisite_gap、possible_misconception，以及模型中的 `state`）、资料、`uncovered_focus`、`likely_known` 与 `ready_to_confirm`。提示不修改大纲。
- 大纲条目 `serves` 必须是目标卡中的关注点；目标课程 `outline confirm` 要求目标卡存在、每个叶子条目有 `why`、每个关注点至少被一个条目 `serves`，否则列出缺项。
- `source attach --why` 至多 120 字，`--serves` 必须是目标卡中的关注点；重新挂载同一位置时更新理由。

## Curriculum Builder（v0.2）

```bash
learn curriculum import --goal "<学习目标>" --id <id> --title <name> [--dry-run|--yes] [--activate]
learn curriculum proposals [id] [--all] [--json]
learn curriculum accept <proposal> [--curriculum <id>] [--json]
learn curriculum reject <proposal> --reason <why> [--curriculum <id>]
learn curriculum outline history [id] [--json]
```

- 大纲 `type`：`source_aligned`（缺省）或 `synthesized`（目标课程固定为此类型）。`outline show --json` 输出 `type`。
- 条目元数据：`why`（≤120 字）、`prerequisites`（其他条目 ID，不能是自身、祖先或后代，不能成环）、`concepts`（kebab-case 概念 ID）。
- 目标课程的主资料种类为 `goal`：没有内容，读取返回 `unsupported`；条目只接受 `text` 定位，且不算来源。没有挂载资料的叶子条目出现在 `source check` 的 `unsourced`、`next` 的 `unsourced: true`；这些条目的概念提交 `textbook_points` 会被拒收。
- `next --json` 新增 `blocked_by`（未完成的先修）、`pending_proposals`（待定的课程调整建议）；动作 `quick_check`（规则 R6b-known-elsewhere）表示条目涉及的概念都已在其他课程中稳定。
- `curriculum_proposals`：`skip`、`mark_known` 两类课程都可用；`insert`、`remove`、`retitle` 只用于 synthesized。提交时按当前大纲校验；`accept` 执行后大纲保持已确认，被替换的旧大纲存入 `Curriculum/<id>/outline-history/`，决定存入 `Curriculum/<id>/proposals.yaml`。同一建议只能决定一次。

## 更多资料（v0.1.9）

```bash
learn curriculum import <repo> --kind code --id <id> [--yes]
learn curriculum import <url> --id <id> [--sitemap | --sitemap-url <url>] [--prefix <path>] [--max-pages <n>] [--yes]
learn source add <repo> --kind code --id <id>
learn source add <url> --id <id> [--sitemap | --sitemap-url <url>] [--prefix <path>] [--max-pages <n>]
learn source refresh <source> [--json]
```

- 自动识别的格式：Markdown、文本、PDF、EPUB、HTML、DOCX、Jupyter、LaTeX、reStructuredText、AsciiDoc、Org，以及它们组成的文件夹。带 DRM 的 EPUB 在导入时拒绝。
- 定位：EPUB 用 `chapter`（`text/ch2.xhtml#s2`）；文件夹与网页快照可用 `chapter`（`0010-ch6.html#6-复制`）区分重复锚点；代码用 `file`，可带 `@<提交>`。
- 代码项目以链接方式导入，读取经 `git show`；依赖目录、构建目录、凭据类文件与二进制文件拒绝读取。挂载自动固定提交；代码类 `textbook_points` 必须带 `@<提交>`。
- 网址快照存于 `Sources/<id>/original/snapshot-NNNN/`（每页一个 HTML 与 `pages.yaml`）。robots.txt、页间至少 1 秒、至多 500 页由程序强制；dry run 不联网。连接失败时报错并说明 Agent 沙箱的联网设置。
- `source refresh`：代码项目记录新提交；网页重新抓取，内容相同则丢弃新快照。

## 三段流程与图谱（v0.1.8）

- `next --json` 输出 `stage`：`consolidate`（复习到期、独立回忆）、`collect`（没有激活的课程、大纲未确认、全书学完）、`learn`（其余）。
- 所有生成文档带固定英文层级标签：`learning/knowledge/{concept,question}`、`learning/evidence/conversation`、`learning/process/{session,progress}`、`learning/nav/{home,overview,curriculum}`。`agent update` 为旧原始对话补标签，只改 frontmatter。
- `init` 与 `agent update` 在 `.obsidian/graph.json` 不存在时写入默认图谱配置：筛选 `-tag:#learning/evidence -tag:#learning/process -tag:#learning/nav -path:Sources`、关闭显示标签、按状态着色；已存在时不修改。
- 概念关系 `related[].type`：`prerequisite`、`part_of`、`applies_to`（有方向，声明在依赖方）、`contrast`、`related`（缺省）。一对概念只保存一条关系，后来的记录替换；先修成环时拒收。

## Detour（v0.1.2）

`detour start` 以当前位置作为 return point，写入 Detour 原因与返回条件；`detour end` 追加 Detour 日志并恢复到 return point。已有未结束 Detour 时拒绝再次 start；Baseline Session 期间两者都被拒绝。

## Learning Policy（v0.1.2）

```bash
learn next [--json]
```

按 [Learning Policy 规则](./interpretation-record.md#7-learning-policylearn-next) 输出单个 Next Best Learning Action，包含动作、目标概念、策略、与 Curriculum 的关系、命中规则、理由、证据 ID 和 `history_used`。该命令只读。

## Git（v0.1.3）

```bash
learn commit [--message <text>]
```

提交 Vault 中尚未提交的变更，没有变更时明确提示。用于宿主沙箱阻止自动提交的情况。

未提交提醒（v0.1.7）：每次自动提交的结果（`committed`、`failed`）记录在 Git 忽略的 `.learning/runtime/git.json`。`status --json` 输出 `git_uncommitted`（未提交的路径数）与 `git_auto_commit`（`committed`、`failed` 或 `none`）。Git 已启用、Vault 是 Git 仓库、有未提交改动、且最近一次自动提交不是成功时，首页顶部显示提醒与 `learn commit` 用法。提交结果变化时首页随即重建；成功时修正并入刚才的同一个提交，不会多出提交。

## 相关概念兜底（v0.1.5）

lesson、review、practice 的 `session end` 在本 Session 涉及的概念缺少相关概念时拒绝结束，错误信息列出同一教材的已有概念。Agent 在结束记录中补 `related`，或用 `no_related` 声明确实无关后再结束。`--no-analysis` 与 baseline 不做此检查。

## 复习（v0.1.5）

```bash
learn review [--json]
```

列出今天到期与未来 7 天的复习，含档位与最近结果。`LEARN_NOW`（RFC3339）环境变量可覆盖当前时间，仅用于验收与自动化测试。首页“今天该复习”一段依赖当天日期，其余投影与时间无关。

## Learner Model（v0.1.2）

```bash
learn model rebuild [--dry-run] [--json]
```

从全部 Interpretation Record 回放重建 `learner-model.json` 与 Markdown 投影，保留用户手写区。`--dry-run` 只报告将新增、修改的投影文件，零写入。

## Agent 资产

```bash
learn agent update --dry-run [--json]
learn agent update --yes [--json]
```

用于把旧 Vault 的 Rules 与 Skills 升级到当前二进制内嵌版本。正式更新前应先预演；被替换的文件会备份到 `.learning/backups/`，且备份目录写入本地 Git exclude。

## State

```bash
learn state [--json]
learn state concept <concept-id> [--json]      # v0.1.2
learn curriculum [--json]
```

没有数据时返回明确的空状态，而不是伪造掌握程度。v0.1.2 起 `state` 从 Learner Model 读取 Concept 状态、Learning Pattern 与 Strategy Evidence 摘要；`state concept` 返回单个 Concept 的当前状态、证据和完整 Cognitive History。`status --json` 增加 `projections` 字段（`current` 或 `stale`）和 `detour` 字段。

## 通用行为

- `--verbose` 输出诊断日志，但不写入 Markdown。
- `--json` 输出稳定 JSON，不混入日志。
- 路径可包含空格和非 ASCII 字符。
- 任何写命令使用锁和临时文件原子替换关键状态。
