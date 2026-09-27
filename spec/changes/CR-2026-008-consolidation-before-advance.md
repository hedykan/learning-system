---
id: CR-2026-008
title: "推进教材前的跨 Session 巩固规则"
status: implemented
target_version: v0.1.3
created: 2026-09-26
---

# 变更摘要

在 Learning Policy 中加入巩固规则：概念已有迁移证据但尚未稳定时，新 Session 开头先做一次回忆测试，再继续教材主线。

## 动机

实测中学习者当堂完成迁移后，`learn next` 立即给出 R6 `continue_curriculum`，Agent 随后把位置推进到下一节。按 CR-2026-004 的状态门槛，`stable` 需要后续 Session 的回忆或迁移证据；当前规则集中没有任何规则会主动安排这一步，概念会长期停在 `developing`。

## 目标行为

- 新增规则 R4b，位于 R4 之后、R5 之前：当前章节范围内存在 `developing` 概念，已有 `applied` 或 `transferred` 证据，最近一次证据来自当前 Session 之前的 Session，且当前 Session 还没有针对它的 retrieval 尝试时，返回 `retrieval_probe`，situation 为 `retrieval`。
- 同一概念在一个 Session 内最多触发一次 R4b，避免反复盘问。
- 该规则只调整教学动作，不阻止学习者或 Agent 显式推进教材。

## 数据与兼容性影响

仅改变 Learning Policy 输出；冻结规则表需新增 R4b，并更新回放验收夹具。

## 验收条件

1. 上一 Session 完成迁移的 `developing` 概念，在新 Session 首次调用 `learn next` 时得到 `retrieval_probe`。
2. 当前 Session 已对该概念做过 retrieval 尝试后，不再触发 R4b。
3. 回放验收仍然通过，并新增一条 R4b 场景断言。

## 决策

2026-09-26 接受，进入 [v0.1.3 基线](../versions/v0.1.3.md)。
