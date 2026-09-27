---
id: CR-2026-004
title: "证据驱动的知识沉淀、Cognitive History 与 Learner Model"
status: implemented
target_version: v0.1.2
created: 2026-09-24
---

# 变更摘要

把已记录的 Raw Learning History 转换为经过 Runtime 校验的 Learning Events、Cognitive History 和 Learner State，并生成可在 Obsidian 中阅读和追溯的知识投影；这些状态继续驱动 Learning Policy 和 Next Best Learning Action。

## 动机

v0.1.1 已能保存 Conversation、Baseline Assessment 和基础 Session，但当前内置分析器主要识别问题，不会形成概念笔记、长期 Learner Model 或有效的认知变化记录。用户在 Obsidian 中看不到预期的学习沉淀，过去学习也尚未系统性影响未来教学。

## 目标行为

- Agent 提交结构化 Session Analysis，Runtime 校验每项证据来自指定 learner turn。
- 持久化可审计、可重算的 Interpretation Record。
- 更新 Concept、Question、Hypothesis、Misconception、Insight 和 Learner overview 投影。
- 记录 old model、trigger、new model 和证据，形成 Cognitive History。
- Learner Model 除 Concept State 外，还形成有多次独立证据支持的跨课程 Learning Pattern。
- Learning Policy 使用历史证据选择下一步教学动作，并记录策略、情境、预期变化、结果和 Strategy Evidence。
- Next Best Learning Action 必须同时满足 learner-optimal 与 curriculum-compatible；先修绕行记录原因、内容、return point 和返回条件。
- 长会话支持明确 checkpoint，使已完成概念无需等到整段会话结束才可见。
- Session 结束记录进度决策，但不因结束本身自动声称概念完成或掌握。
- 生成支撑核心闭环所需的最小 Obsidian 投影、导航索引和证据链接，不以笔记数量或 Dashboard 为目标。

## 数据与兼容性影响

- Conversation 保持 append-only，旧 Conversation 和 Session 可作为重建输入。
- 新增稳定 turn ID、Interpretation Record Schema 和投影版本。
- v0.1.1 的 `solid` 状态需要显式兼容迁移到候选规范词 `stable`，不得静默产生两套含义相同的状态。
- 生成文件必须区分 Runtime 区块与用户手写区块，升级和重建不得覆盖用户内容。
- 重复提交同一分析必须幂等；部分写入失败不得留下相互矛盾的模型与投影。

## 风险

- AI 过度推断学习者状态。
- 概念身份重复或错误合并。
- 频繁更新造成笔记噪音和 Git 历史膨胀。
- 重建投影时覆盖用户编辑。
- 把课程完成度错误等同于理解程度。
- 从单次行为推断固定学习风格，或把策略与认知变化的时间相邻误判为确定因果。
- 为追求个性化而脱离 Curriculum 主线或隐式重排教材。

## 验收条件

1. 每个状态与认知变化都能定位到 Conversation turn 和原文。
2. 无有效 learner evidence 的候选被 Runtime 拒绝且不改变已有状态。
3. 一次包含理解修正的 Session 会生成可读 Cognitive Change 和受影响 Concept 投影。
4. Concept 同时区分学习者原话、AI 解释和外部来源。
5. 后续反例能修订当前 Learner State，同时保留此前 Cognitive History。
6. Curriculum Progress 与 Learner State 分别更新，任何一方都不隐含另一方。
7. 最小 Obsidian 索引能够发现 Session、Concept、证据和下一步；没有独立生命周期价值的事件不会被扩张为大量单独文件。
8. checkpoint 和 end 重复执行具有幂等性，失败不会产生半更新状态。
9. 用户手写区块在投影重建和版本升级后保持不变。
10. Next Best Learning Action 能说明使用了哪些历史证据、策略规则，以及它如何兼容当前 Curriculum。
11. prerequisite Detour 完整保存偏离原因、学习内容、return point 和返回条件，并能回到教材主线。
12. Learning Policy 保存策略结果为可修正的 Strategy Evidence；`strategy_switch` 能关联前后策略和后续认知变化。
13. 跨课程 Learning Pattern 至少引用两个 Curriculum 中的独立证据，并保留反例或不确定性。
14. 在 10～20 个连续 Session 的回放验收中，有历史的系统相较 Session 1 选择了至少一个不同的教学动作；差异能够追溯到跨 Session Learner Model 或 Strategy Evidence，而不是当前回合上下文。

## 决策

2026-09-24 接受，进入 [v0.1.2 基线](../versions/v0.1.2.md)。Schema、CLI、策略记录结构、Learning Policy 规则和 12 Session 回放夹具已冻结于 [Interpretation Record 规范](../interpretation-record.md) 与 [CLI 契约](../cli.md)。

冻结时的关键取舍：

- 解释由 Agent 提交，Runtime 只做证据校验和确定性派生；v0.1.2 不接入网络模型。
- Interpretation Record 是规范数据；Learner Model 为回放结果，可删除重建。
- turn ID 由 append-only 序号推导，旧 Conversation 无需迁移。
- Learning Policy 为确定性规则，使 10～20 Session 验收可在测试中复现；验收夹具取 12 个 Session。
- 最小投影只含 Concept、Session、Learner overview 和 Curriculum index，不生成独立的 Question、Hypothesis、Misconception、Insight 文件。
- `solid` 从未持久化，直接由 `stable` 取代，无需数据迁移。
