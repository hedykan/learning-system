---
id: CR-2026-042
title: "跨课程复用已掌握的概念"
status: implemented
target_version: v0.2
created: 2026-09-28
---

# 变更摘要

条目声明了预计涉及的概念（CR-2026-039）时，如果这些概念在任何一门课程中都已是 stable，`learn next` 不再从头讲，而是建议一次快速检索验证（`quick_check`），并说明这些概念是在哪门课程掌握的。通过后，Agent 可以提出 `mark_known` 调整建议。

## 动机

同样学 Kubernetes，不同基础的人路径不同，差别主要来自已经会什么。概念本来就跨课程共享，程序可以确定性地发现“这一节你已经会了”，而不是凭 AI 的感觉。

## 目标行为

- 规则 R6b-known-elsewhere：下一个条目的 `concepts` 全部存在且为 stable 时，动作为 `quick_check`，策略 `retrieval_practice`，阶段 `learn`，理由列出概念与掌握它们的课程；只要有一个概念不是 stable，就按原规则继续。
- 教材首页在该条目后标“已在其他课程掌握”。
- Skill：quick_check 用一两个问题验证；答得好就提出 `mark_known`，答不好就正常学习（并按规则降级）。

## 验收条件

1. 条目概念全部 stable 时 `next` 为 quick_check 并给出来源课程；有一个不是 stable 时不触发。
2. 教材首页显示标注。

## 决策

2026-09-28 接受，排入 v0.2。

2026-09-28 实现：规则 R6b-known-elsewhere 与动作 `quick_check`；教材首页标“已在其他课程掌握”。
