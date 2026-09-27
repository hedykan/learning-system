---
id: CR-2026-012
title: "艾宾浩斯式间隔复习"
status: implemented
target_version: v0.1.5
created: 2026-09-26
---

# 变更摘要

程序按固定间隔为每个概念排复习日期，列出到期概念并在首页显示；Agent 负责出回忆题、判断回答并提交复习结果。

## 分工

- 程序：根据记录回放计算每个概念的复习档位与下次日期，提供 `learn review`，在 `learn next`、首页、概念笔记和学习者总览中显示。
- Agent：出题、判断回答是想起、部分想起还是忘记，提交 `review_results`。

## 目标行为

- 间隔档位：1、2、4、7、15、30、60 天。
- 概念第一次达到 `developing` 或 `stable` 时进入第 0 档，下次复习为该状态记录日期加 1 天。
- 复习结果 `recalled`：升一档；`partial`：档位不变；`forgotten`：回到第 0 档。下次日期为复习当天加上对应档位的间隔。
- 概念被更新为 `fragile` 时回到第 0 档。
- 日期按学习者本地日历日计算。
- Interpretation Record 新增 `review_results`：`id`、`concept`、`outcome`、`action_turn`（助手出题的轮次）、`evidence`（其后的学习者原话）。
- Learning Policy 新增规则 R3b（位于 R3 之后、R4 之前）：存在到期概念时返回 `review_due`，概念为最早到期者，situation 为 `retrieval`。
- `learn review [--json]` 列出今天到期与未来 7 天的概念，以及各自档位和最近一次结果。
- 首页新增“今天该复习”，标注计算日期；概念笔记新增“复习计划”；学习者总览新增复习日程表。
- 测试用环境变量 `LEARN_NOW`（RFC3339）可覆盖当前时间，仅用于验收与自动化测试。

## 数据与兼容性影响

Interpretation Record schema 保持 `@1`，`review_results` 为可选新增字段；排期完全由记录推导，不新增存储。首页的“今天该复习”依赖当天日期，其余投影仍与时间无关。

## 验收条件

1. 概念达到 developing 的次日出现在到期列表中，当天不出现。
2. 连续想起时间隔依次为 1、2、4、7……天；忘记后回到 1 天；变为 fragile 后回到 1 天。
3. 非法复习结果（未知概念、未知结果、出题轮次不是助手、证据早于出题）被拒收。
4. 有到期概念时 `learn next` 返回 `review_due`；本 Session 已复习过的概念不再到期。
5. 首页、概念笔记、学习者总览正确显示复习信息。

## 决策

2026-09-26 接受，进入 v0.1.5。
