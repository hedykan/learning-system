---
id: CR-2026-018
title: "学习模式在单本教材内也可被采信"
status: implemented
target_version: v0.1.6
created: 2026-09-28
---

# 变更摘要

学习模式的“supported”门槛从“至少 2 个 Session 且 2 本教材”放宽为：满足其一即可——2 本教材各有支持且至少 2 个 Session，或同一本教材中至少 3 个 Session 支持；两种情况都要求支持多于反例。

## 动机

核心验证场景 16 的定义是“多个 Session 都显示同一模式”，并不要求跨教材。v0.1.5 只学一本书时，模式最多停在 candidate，无法进入策略个性化。

## 验收条件

1. 同一教材 2 个 Session 支持仍为 candidate，3 个 Session 支持为 supported。
2. 跨 2 本教材、2 个 Session 支持仍为 supported。
3. 反例不少于支持时为 contested。

## 决策

2026-09-28 接受，进入 v0.1.6。
