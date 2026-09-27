# Changelog

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
