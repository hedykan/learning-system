---
id: CR-2026-016
title: "模型修订兜底：稳定概念出现反证时必须降级"
status: implemented
target_version: v0.1.6
created: 2026-09-28
---

# 变更摘要

提交记录时，如果一个当前为 `stable` 的概念在这条记录里出现误解事件，或复习结果为 `partial`、`forgotten`，而记录没有把它降级，程序拒收并要求补一条降级的状态更新。旧的 stable 判断与证据保留在历史中。

## 动机

v0.1.5 核心验证场景 15 未通过：学习者在已 stable 的“强缓存”上复发最初的误解，Agent 记录了误解事件和 partial 复习结果，却没有降级，概念仍显示 stable。

## 目标行为

- 适用于 checkpoint、end 与 annotate。
- 判定：概念在本条记录应用前为 `stable`，本条记录含该概念的 `misconception` 事件，或 `partial`、`forgotten` 的 `review_results`，且应用后仍为 `stable`，则拒收。
- 错误信息列出概念与触发原因，提示补一条 `fragile`（或 `developing`）状态更新。
- 只检查新提交的记录；回放历史记录时不做此检查，已有 Vault 不受影响。

## 验收条件

1. 上述情况被拒收且零写入，补上 `fragile` 后被接受，历史中 stable 条目仍在。
2. 非 stable 概念的误解不受影响。
3. 已有记录回放不因本规则失败。

## 决策

2026-09-28 接受，进入 v0.1.6。
