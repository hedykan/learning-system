---
id: CR-2026-038
title: "课程类型：Source-aligned 与 Synthesized"
status: implemented
target_version: v0.2
created: 2026-09-28
---

# 变更摘要

大纲增加 `type`：`source_aligned`（忠于一份资料的章节与顺序）或 `synthesized`（由 Agent 从学习目标组织、按先修依赖安排）。类型决定课程能怎样被调整：前者只能跳过或标记“已掌握”，后者还能插入、删除和改名条目。

## 动机

来自 [Curriculum Builder](../proposals/curriculum-builder.md)：有成熟教材时，Learner Model 决定“怎么学”，不能随意改变“学什么”；没有教材时，Learner Model 可以更深地参与课程设计。两种课程需要不同的规则，而“谁能改什么”是固定规则，应由程序强制。

## 目标行为

- `outline.yaml` 新增 `type`；缺省为 `source_aligned`，现有课程不变。以学习目标建立的课程（CR-2026-040）为 `synthesized`。
- 课程调整建议（CR-2026-041）按类型限制：`source_aligned` 只接受 `skip`、`mark_known`；`synthesized` 另接受 `insert`、`remove`、`retitle`。
- `status --json`、`curriculum outline show`、教材首页显示类型。

## 数据与兼容性影响

纯新增字段；旧大纲读作 `source_aligned`。

## 验收条件

1. 旧课程为 `source_aligned`，重建结果不变。
2. `source_aligned` 课程的 `insert` 建议被拒收并说明原因。

## 决策

2026-09-28 接受，排入 v0.2。

2026-09-28 实现：`outline.yaml` 的 `type`，缺省 source_aligned；目标课程固定为 synthesized；`outline show`、状态中可见；调整建议按类型限制。
