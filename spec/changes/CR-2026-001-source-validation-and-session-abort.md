---
id: CR-2026-001
title: "Source 生命周期与 Session 中止"
status: implemented
target_version: v0.1.4
created: 2026-09-24
---

# 变更摘要

为教材预检失败提供正式、可审计、可恢复的处理路径，避免 Agent 只能结束一个未发生学习的 Session，或让不可靠 Source 保持激活状态；同时允许用户安全移除并重新导入错误教材。

## 动机

真实导入测试发现：PDF 文件可能可读且哈希正常，但内容并非用户期望的原教材，或关键数学符号损坏。v0.1 可在验证前激活 Curriculum 和启动 Session，却没有拒绝 Source、停用 Curriculum 或中止 Session 的命令。

## 当前行为

- 导入可直接 `--activate`，没有 `pending/verified/rejected` 状态。
- Session 只能 `end`，没有 `abort`。
- Curriculum 只能切换到另一个 ID，不能 deactivate。
- Position 可以逐字段设置，但没有明确的 reset 操作。
- Source/Curriculum 没有 remove、restore 或同 ID reimport 流程。

## 目标行为

- Source import 默认进入 `pending`，内容核验后执行 `source verify` 或 `source reject --reason`。
- 未验证 Source 默认不能激活；显式 override 必须留下审计记录。
- `learn session abort --reason <reason>` 保留 Conversation，生成 `aborted` Session，不运行认知分析、不推进 Curriculum，并清除 active session。
- `learn curriculum deactivate` 和 `learn curriculum position reset` 提供明确恢复操作。
- `learn curriculum remove <id> --dry-run --json` 列出将受影响的 Source、Curriculum、active 状态和历史引用，不写入任何内容。
- `learn curriculum remove <id> --yes` 默认执行可恢复归档：移动 Source 与当前 Curriculum 到 Runtime 管理的归档区，清除匹配的 active curriculum，但保留 Conversation、Session 和 Git 历史。
- active Session 引用目标 Curriculum 时拒绝 remove，必须先完成或 abort Session。
- `learn curriculum restore <archive-id>` 恢复归档；恢复遇到同 ID 时拒绝覆盖。
- 归档后允许使用原 ID 重新 import；内容哈希去重默认不扫描 Archived Source，但输出历史匹配提示。
- 永久删除使用独立的 `purge` 操作，并要求二次显式确认；不属于普通 remove 的副作用。
- `status --json` 显示 Source verification 与 Session termination 状态。

## 数据与兼容性影响

现有 v0.1 Source 迁移为 `unknown`，不自动认定 verified。Session schema 增加 `completed/aborted` termination 状态，保留 schema version migration。归档清单保存原 ID、哈希、归档原因、时间和历史引用，不保存新的教材副本。

## 风险

- 验证状态不能被误解为版权、真实性或安全性的权威认证。
- abort 必须保留已发生的原始 Conversation，不能伪装成从未启动。
- rejected Source 的原件删除必须是独立、显式且可恢复的操作，不属于本变更默认行为。
- remove 不能级联删除 Conversation、Session、Learner State 证据或 Git 历史。
- 大型 Source 的归档必须使用同一文件系统 rename，避免复制中断产生半删除状态。

## 验收条件

1. pending Source 在无 override 时不能 activate。
2. reject 会保存原因和证据时间，不修改 Source 原件。
3. abort 后 active session 为空，Conversation 保留，Learner/Curriculum State 不前进。
4. abort 生成可读的审计 Session，并能安全 Git commit。
5. deactivate 后 status 明确显示没有当前 Curriculum。
6. remove dry-run 准确列出影响且零写入；remove 默认可恢复并清除匹配的 active curriculum。
7. 被历史 Session 引用的 Source 可以归档，但历史链接仍能解释其归档状态，且历史文件不被删除。
8. active Session 正引用目标时 remove 明确拒绝。
9. restore 和同 ID reimport 都不静默覆盖任何活动文件。
10. purge 与 remove 分离，且 purge 需要二次显式确认。
11. v0.1 Vault migration 后所有既有文件仍可读。

## 决策

部分实现并拆分（2026-09-26）：

- `session abort` 与 `curriculum position reset` 已在 v0.1.1 实现。
- Source 内容核验并入 CR-2026-005 的目录确认流程，随 v0.1.3 评审。
- 其余 remove、restore、purge、deactivate 留待 v0.1.4 评审。

2026-09-26 接受剩余部分进入 v0.1.4，范围调整如下：

- 不再引入 `pending/verified/rejected` 状态：内容核验已由 v0.1.3 的目录确认承担，目录未确认的教材在 `status` 中已明确标出。
- v0.1.4 交付：`curriculum deactivate`、`curriculum remove`（默认可恢复归档，带 dry-run）、`curriculum restore`、同 ID 重新导入、独立的 `curriculum purge`（二次确认）。
- 验收条件 1、2 与上述调整不再适用，其余验收条件保持。
