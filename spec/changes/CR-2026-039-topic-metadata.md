---
id: CR-2026-039
title: "Topic 元数据：为什么学、先修、涉及的概念"
status: implemented
target_version: v0.2
created: 2026-09-28
---

# 变更摘要

大纲条目可带 `why`（为什么要学）、`prerequisites`（先修条目）与 `concepts`（预计涉及的概念 ID）。程序校验先修存在且无环；`learn next` 只推荐先修都已完成（或跳过）的条目。

## 目标行为

- `why`：至多 120 字；`prerequisites`：本大纲中已存在的其他条目 ID，不能是自身、祖先或后代，整体不能成环；`concepts`：kebab-case 概念 ID，可以是尚未出现的概念。
- `learn next`：按顺序找下一个未完成条目时，跳过先修未完成的条目；如果所有未完成条目都被挡住，返回第一个，并在 `blocked_by` 列出未完成的先修。
- 教材首页在条目下显示“为什么学”与先修。
- Skill：synthesized 课程的每个条目都写 `why` 与先修；source-aligned 课程可选。

## 数据与兼容性影响

纯新增字段。

## 验收条件

1. 先修指向不存在的条目、自身、祖先，或形成环时，大纲被拒收。
2. 先修未完成时 `learn next` 越过该条目；全部被挡时返回 `blocked_by`。

## 决策

2026-09-28 接受，排入 v0.2。

2026-09-28 实现：`why`、`prerequisites`、`concepts` 校验（存在、非自身/祖先/后代、无环、kebab-case）；`learn next` 越过先修未完成的条目，全部被挡时返回第一个并给出 `blocked_by`；教材首页显示为什么学与先修。
