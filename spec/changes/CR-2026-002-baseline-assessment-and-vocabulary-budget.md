---
id: CR-2026-002
title: "首次学习摸底、Guided Socratic Policy 与术语预算"
status: implemented
target_version: v0.1.1
created: 2026-09-24
---

# 变更摘要

首次进入教材或新领域时，先完成轻量 Baseline Assessment，再推荐起点并进入正式授课。教学阶段采用 Guided Socratic Policy、例子优先和术语预算，避免在未知基础上一次堆叠大量名词。

## 动机

真实试用中，Agent 在第一轮同时引入数据库复制、缓存、搜索索引、流处理和批处理。学习者能够判断电商系统偏数据密集型，但随后明确询问“缓存和搜索索引是什么”，说明 Agent 尚未识别基础词汇缺口就推进了教材位置。

## 当前行为

- 首次学习直接设置 Curriculum Position 并开始讲解。
- 一个回合可引入任意数量的新术语。
- 简短小测发生在讲解之后，无法建立未受提示影响的初始证据。
- 用户暴露先修缺口后，没有明确的策略切换与 return point。

## 目标行为

### 首次摸底

- 新 Curriculum 没有 Learner State 时，默认先建议 Baseline Assessment；用户可以明确跳过。
- Assessment 先询问学习目标、相关经验和熟悉场景，再逐项测试关键先修概念。
- 问题一次一个，不先讲答案；“不知道”是有效证据，不视为失败。
- 默认 5–8 个自适应问题，支持 quick、standard、deep 三种深度。
- 覆盖术语识别、自己的话解释、场景判断、简单预测和信心校准，而不是只问自我评分。
- Assessment 期间不推进 Curriculum Position。
- 完成后输出定性基线：已有证据、未知概念、可能误解、先修缺口和推荐起点；用户确认后才设置位置。

### 教学策略

- 从学习者熟悉的具体场景出发，再命名抽象概念。
- 默认遵循 `elicitation → prediction → reasoning → challenge → revision → transfer`；每一步必须引用本 Session 中可观察的回答。
- 不把苏格拉底式教学理解为“只问不教”：学习者缺少基础术语或无法形成初始模型时，先给最小直接解释和例子，再恢复提问。
- 默认每个教学回合最多引入 1–2 个新术语；超过预算时拆分回合。
- 每个新术语至少包含一个具体例子、与相邻概念的边界，以及一个轻量理解检查。
- 用户询问基础术语含义时，记录 `prerequisite_gap`，暂停继续引入同层新术语。
- 补齐缺口后用明确 return point 回到教材主线。
- 一轮没有可观察认知变化时切换表示方式，不重复同样的术语解释。

## CLI 与数据

- Session 增加 `kind: baseline|lesson|review|practice`。
- 建议入口：`learn session start --kind baseline --depth standard`。
- Baseline Conversation 仍然 append-only；结束后生成 Assessment artifact 和 Learner State 候选。
- `status --json` 显示某 Curriculum 是否已有 baseline、日期和证据范围。
- Lesson Session 必须记录它采用的 baseline 或用户显式 skip。

## 数据与兼容性影响

现有 Session 缺少 kind 时迁移为 `lesson`。现有 Curriculum 没有 baseline 时显示 `not_assessed`，不自动推断用户是初学者或熟练者。

## 验收条件

1. 新 Curriculum 首次开始时会建议 baseline，不会先推进 Position。
2. 用户可以显式 skip，且 skip 被记录但不伪造 Learner State。
3. Standard baseline 一次只问一个问题，并在结束前不泄露后续答案。
4. 仅凭“我懂了”不能形成 solid 状态。
5. Lesson 默认每轮不超过两个未建立的新术语。
6. 用户询问基础术语时产生 prerequisite gap，并停止继续堆叠名词。
7. Assessment 结束后先展示推荐起点，只有用户确认才写入 Position。
8. Baseline、Lesson 与 Review 的历史可以分别查询。
9. Agent 不能在只完成一次识别题后直接判定概念已掌握；至少需要解释或预测证据，solid 状态还需要延迟检索或迁移证据。
10. 每次 Session 可以解释下一教学动作对应了哪条对话证据和 Guided Socratic 阶段。

## 决策

已接受进入 v0.1.1，作为该版本核心能力。真实网络 Model Provider 仍可后续接入；v0.1.1 允许宿主 Agent 提交结构化、带原始对话证据的 Baseline Assessment，由 Runtime 校验并落盘。
