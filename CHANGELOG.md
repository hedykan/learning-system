# Changelog

## v0.2.4 — 2026-09-30

- 复习课可以插进学习课：学习课或练习课进行中，`learn session start --kind review` 会挂起当前的课，复习课结束或中止后自动恢复，两节课的对话和记录分开保存。`learn status` 与 `learn vaults` 显示被挂起的课。老学习库无需迁移。
- 修复：同一时刻开始的两节课（例如固定 `LEARN_NOW` 时中止后马上重开）会共用 ID，后一节覆盖前一节的对话。

## v0.2.3 — 2026-09-29

- 内嵌 Git：学习库的提交、状态与历史，以及代码项目按提交读取，改用纯 Go 的 go-git，不再需要安装 `git` 命令，为手机端铺路。仓库仍是标准 Git，任何 Git 客户端都能继续使用。
- 没有配置 Git 身份时不再提交失败，作者回退为 `Learning OS <learn@localhost>`；新学习库的默认分支为 `main`。

## v0.2.2 — 2026-09-29

- `learn vaults [目录] [--json]`：列出一个文件夹及其下两层中的学习库，给出每个库的课程、当前课程、进行中的 Session 与今天到期的复习数；配合 `--vault <路径>`，可以在一个总文件夹里管理多个学习库。

## v0.2.1 — 2026-09-28

入学：先了解学习者，再设计课程。

- 学习目标卡：要达成什么、用在哪里、已有基础、时间与深度、怎样算学会、具体关注点，每项都附学习者原话；从摸底评估写入，或用 `learn curriculum goal set`；从目标建立的课程必须先有目标卡才能提交大纲。
- 入学诊断：摸底可在建课之前完成，评估中的发现标注概念 ID；`learn curriculum outline review` 据此提示每个条目学习者可能已经会了什么，由学习者决定是否跳过。
- 草案评审：逐条列出为什么学、先修、服务的关注点、证据与资料；目标里的关注点都有条目覆盖、每个叶子条目都写了为什么学，目标课程才能确认。
- 资料选择理由：`learn source attach --why --serves`，教材首页与评审中显示为什么选这份资料。

## v0.2.0 — 2026-09-28

Curriculum Builder：课程本身成为可构建、有来源、会随学习者演化的对象。

- 课程类型：`source_aligned`（跟随一份资料）与 `synthesized`（从学习目标组织），类型决定课程能怎样调整。
- 从学习目标建课：`learn curriculum import --goal "<目标>"`；Agent 研究方向、提交带“为什么学”、先修与概念的大纲，再给每个条目挂上可靠资料；没有资料的条目明确标为 AI 综合，不能提交教材要点。
- 先修：大纲条目的先修必须存在且无环；`learn next` 只推荐先修已完成的条目。
- 有证据的课程调整：解读记录中的 `curriculum_proposals`（跳过、已掌握、插入、删除、改名），经学习者同意后 `learn curriculum accept` 才生效；被替换的大纲保留在版本历史中。
- 跨课程复用：条目涉及的概念若在其他课程中已稳定，`learn next` 建议快速检验（`quick_check`）而不是从头讲。

## v0.1.9 — 2026-09-28

- 更多资料格式：EPUB（按书内目录生成草稿，拒绝 DRM 版本）、HTML、Word、Jupyter、LaTeX、reStructuredText、AsciiDoc、Org，以及混合这些格式的文件夹；都可以用 `learn source read` 按标题锚点或章节读取原文。
- 代码项目：`--kind code` 以链接方式导入 Git 项目，按提交读取文件，凭据、依赖、构建目录与二进制文件不可读；`learn source refresh` 记录新提交，旧记录仍指向原来的提交。
- 网址：`learn curriculum import <url>`（可加 `--sitemap`、`--prefix`）抓取网页或整本在线书的快照，之后离线学习；遵守 robots.txt 并限速；在 Agent 沙箱内连不上网时给出明确说明。
- 发布包：macOS 与 Windows 改为直接提供二进制（不再打 tar.gz、zip，包里本来只有这一个文件）；Linux 仍提供 AppImage 与 tar.gz。

## v0.1.8 — 2026-09-28

- 三段学习流程：资料搜集 → 学习 → 巩固。`learn next` 输出 `stage`，Vault 首页按三段组织。
- Obsidian 图谱整理：每篇生成文档带固定的英文层级标签（`learning/knowledge`、`evidence`、`process`、`nav`），可用 `tag:#learning/...` 筛选；新学习库自带只显示知识与手写笔记的默认图谱配置（已有配置不动）；`agent update` 为旧原始对话补标签，对话内容不变。
- 知识笔记的证据链接收敛为每个 Session 一个；概念笔记不再链接学习记录与教材首页。
- 概念关系类型：先修、组成、应用、易混、相关；有方向的关系只在依赖方写链接，图谱显示箭头；先修不能成环；跨教材的联系单独列出并加标签。
- 通用定位：教材要点与目录条目可用页码、锚点、章节、文件行号、视频时间点或自由文本定位；页码写法与以前完全一致。
- 资料接口：`learn source add/list/outline/read/attach/detach/check`；Markdown 可按标题锚点读取原文；文件夹教材按自然顺序（ch2 在 ch10 前）生成目录。
- 一门课程可以挂多份资料，例如教材加视频课；`learn next` 与 `status` 列出当前小节的资料。
- 没有文件的资料：`learn curriculum import --external` 支持视频课、纸质书、线下课程。
- 话术：对话里的数学公式用终端可读的符号，不输出 LaTeX 源码。
- 发布流程：推送版本标签时 GitHub Actions 自动测试并打包 macOS（x64、arm64）、Windows（x86、x64、arm64）与 Linux（x86、x64、arm64、armv7；AppImage 与 tar.gz），上传到 Release；可手动补发旧版本。
- README：仓库已公开，删除私有仓库的拉取配置，新增下载二进制的安装方式。

## v0.1.7 — 2026-09-28

- 界面语言：`learn init --language`、`learn config set language <zh|en>`；生成页面的固定文字、固定页面名（`Learner overview.md`、`Progress.md`）、手写区标题与策略理由跟随设置，缺省中文且与 v0.1.6 输出一致；切换时自动迁移页面并保留手写区。Skill 按学习者使用的语言自动设置。
- 学习记录未提交提醒：记录自动提交结果；`status --json` 新增 `git_uncommitted`、`git_auto_commit`、`language`；自动提交失败时首页顶部提示在沙箱外运行 `learn commit`，提交成功后提醒自动消失。Session 结束时 Agent 可提一句尚未保存。Skill 中写死的中文页面名与“结束时绝不提 Git”的旧规则已同步修正。
- 规划 [v0.1.8 资料接口](spec/versions/v0.1.8.md)：通用定位、资料适配器、多资料课程、外部资料、文字类格式、代码项目、网址快照、Agent 组织课程、数学公式写法（CR-2026-025 至 033）。

## v0.1.6 — 2026-09-28

- 新增 [核心验证场景](spec/validation-scenarios.md)：17 个场景作为每个版本发布前的真实 Agent 验收标准。
- 模型修订兜底：stable 概念出现误解或 partial、forgotten 复习时，记录必须同时降级，否则拒收；旧判断保留在历史中。
- 完成当前所在小节时，位置自动推进到下一个未完成条目，修复绕行返回点错位与学完后位置停留在上一节的问题。
- 学习模式在同一本教材 3 个 Session 支持时即可被采信。
- 学习者关键问题成为一等学习对象：`questions` 与 `question_resolutions`，独立的 `Questions/<id>.md` 笔记，首页与总览列出开放问题，策略规则 R2b 在相关小节优先回答，`learn next` 输出 `open_questions`。
- Skill 新增学习事件判别表（application、transfer、learner_proposed_method、insight、retrieval、connection）与稳定概念降级规则。
- 话术：一个学习轮次只回复一次，对学习者的评价只出现在最后的回复中，进度提示最多一句不带评价的中性过渡。
- 笔记以学习者的语言命名：`Concepts/<概念名称>.md`、`Questions/<问题>.md`、`Profile/学习者总览.md`、`Curriculum/<id>/<书名>.md`、`学习进度.md`；Session 标题与小标题改为中文；重建时自动迁移旧命名笔记并保留手写区。
- README 改写为面向学习者的产品介绍，并补充 Obsidian 使用说明。

## v0.1.5 — 2026-09-26

- 艾宾浩斯式间隔复习：按 1、2、4、7、15、30、60 天排期；Agent 提交 `review_results`；新增 `learn review` 与策略规则 R3b；首页显示“今天该复习”，概念笔记显示复习计划，学习者总览显示复习日程。
- Obsidian 知识图谱：Agent 用 `related` 标注相关概念，概念笔记生成双向“相关概念”链接，并写入 aliases 与 `learning/state/*`、`learning/curriculum/*` 标签。
- 结束学习前的相关概念兜底：本次涉及的概念若没有任何相关概念，`session end` 拒绝并列出可关联的概念；Agent 标注 `related` 或声明 `no_related` 后才能结束。
- 扫描版教材：Skill 规定先文字提取、再用模型视觉读页面图像，仍读不出则请学习者换用文字版，不凭记忆教学。
- Windows：新增 Windows amd64/arm64 构建与 `make test-windows-build`，补充 CRLF 解析测试。

## v0.1.4 — 2026-09-26

- Vault 根目录生成 `README.md` 学习首页：正在学习、我的教材、学习者总览、最近学习、最近变化的概念与使用说明；没有学习记录时也会生成，已有的非生成 README 内容移入手写区。
- `init` 不再创建五个空的旧目录，`agent update` 安全删除其中的空目录。
- 目录条目新增“学过一部分”状态；`status` 报告 `partial`；`learn next` 推进时先回到之前学过一部分的条目。
- 笔记中的学习者原话显示该轮对话的实际时间。
- 概念笔记新增“教材要点”：Agent 读过原文后提交，校验页码范围，保留历史版本。
- 教材生命周期：`curriculum deactivate`、`remove`（可恢复归档）、`archives`、`restore`、`purge`（二次确认）；同 ID 可重新导入并提示内容相同的归档。

## v0.1.3 — 2026-09-26

- 教材目录：Markdown 导入自动生成草稿；PDF 由 Agent 读取原书目录后 `curriculum outline set`，学习者确认后 `outline confirm`。
- 目录确认后，教材位置只能用 `position set --node` 指向真实条目。
- 新增 `curriculum complete` 与 `curriculum skip`；进度改为 append-only 的 `progress.yaml`，`progress.md` 与教材首页显示逐节状态，并标出被越过的“未覆盖”条目。
- `status` 报告目录状态、位置是否已核对、未覆盖条目与最近一次 Git 提交。
- 新概念自动挂到当前目录条目；策略按目录编号判断章节范围，修复章节标题写法不同导致规则失效的问题。
- `learn next` 在沿教材继续时给出下一个未完成条目；新增 R4b：上次已应用或迁移但未稳定的概念，新 Session 先回忆一次。
- 学习者可见话术规则与 Vault 自带 Codex 配置（gpt-5.6-sol，低思考）。
- `session append --file`、终端输入检测、`.learning/tmp/` 临时目录、`learn commit` 补提交、概念笔记合并重叠引用。
- Agent Skill 新增目录流程、依据原文教学、完成记录与误解记录规则。

## v0.1.2 — 2026-09-24

- Conversation 每轮获得稳定 turn ID 与 Obsidian 块锚点；旧对话按顺序兼容，无需迁移。
- 新增 Interpretation Record：`session checkpoint`、`session end --analysis-file`、`session annotate`。每条判断都必须引用学习者原话，由 Runtime 校验。
- Learner Model 由全部记录回放生成，包含概念状态门槛、认知变化、策略证据、跨课程学习模式与撤回。
- 新增 `learn next` 确定性 Learning Policy，推荐动作附带理由、历史证据与教材主线关系。
- 新增 `curriculum detour start|end`，完整记录先修绕行与返回点。
- 自动生成概念笔记、学习者总览、教材学习首页与 Session 解读区块，保护手写笔记区；`learn model rebuild` 可逐字重建。
- `state` 与 `state concept` 读取 Learner Model；`status` 报告投影是否过期与当前绕行。
- lesson 结束必须提交解读、已有 checkpoint，或显式 `--no-analysis --reason`。
- Agent Skill 改用 `stable` 取代 `solid`，并加入记录提交与 `learn next` 流程。
- 12 Session、双教材回放验收：有历史的系统会做出不同且可追溯的教学选择。

## v0.1.1 — 2026-09-24

- 新教材默认先进行 quick、standard 或 deep Baseline Assessment。
- 增加 Session kind、显式 skip、position gate 和带原因的安全 abort。
- Baseline 只能用 Raw Conversation 中的学习者原话形成结构化 Assessment。
- `status` 增加 baseline 状态，完成摸底后才默认进入 lesson。
- Agent Skill 采用 Guided Socratic 循环、术语预算、先修缺口修复和 return point。
- 增加可预演、可备份的 `learn agent update`，用于升级已有 Vault。
- 常规学习记录改为后台静默执行，课程回复不再反复播报 Learning OS 操作。

## v0.1.0 — 2026-09-24

- 增加可通过 `go install` 安装的 `learn` CLI。
- 增加幂等 Vault 初始化、向上发现、Git 初始化和状态查询。
- 将 Codex/Claude Rules、Skills 和 references 嵌入单二进制。
- 增加教材 dry-run/import/list/show/activate 和位置管理。
- 明确 Source、Curriculum State 与 Learner State 的数据边界。
- 增加 append-only Session start/append/end 闭环和 Provider 抽象。
- 增加结构化 JSON 输出、原子关键文件写入和失败保护测试。
- 建立 spec、想法池、需求变更流程和分阶段开发计划。
