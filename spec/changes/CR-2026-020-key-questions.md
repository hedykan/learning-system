---
id: CR-2026-020
title: "学习者关键问题成为一等学习对象"
status: implemented
target_version: v0.1.6
created: 2026-09-28
---

# 变更摘要

学习者提出的关键问题作为独立对象保存，有开放与已解决两种状态，生成独立的 Obsidian 笔记，并进入 `learn next` 的后续教学。

## 动机

v0.1.5 场景 13 未通过：学习者问“既然 304 表示没变，为什么还非得问这一次”，Agent 认真回答了，但记录里没有任何问题事件，问题也无法影响后续教学。

## 目标行为

- Interpretation Record 新增 `questions`：`id`（全局 kebab-case，与概念 ID 同规则）、`question`（问题的简述，最多 120 字）、可选 `concept`（已有概念）、可选 `node`（预计在哪个目录条目回答）、`evidence`（学习者原话，必填）。
- 新增 `question_resolutions`：`question`（问题 ID）、`summary`、`evidence`（学习者能回答时的原话）。一个问题只能解决一次。
- 提交时若给了 `node`，必须存在于已确认的目录。
- Learner Model 保存问题的状态、证据、提出与解决的 Session。
- 投影：每个问题生成独立笔记（v0.1.6 起以问题本身命名，见 CR-2026-022）（带 `learning/question/open|resolved` 标签）；概念笔记列出相关问题；学习者总览与首页列出开放问题。
- Learning Policy：
  - 新规则 R2b（位于 R2 之后、R3 之前）：存在开放问题，其 `node` 等于当前位置，或其 `concept` 为当前概念，返回 `address_question`。
  - `learn next` 输出新增 `open_questions`（最多 3 个）。

## 数据与兼容性影响

schema 保持 `@1`，新增字段可选。v0.1.4 起不再预建的 `Questions/` 目录在有问题时按需创建。

## 验收条件

1. 带证据的问题被接受并生成问题笔记；无证据或引用未知概念、未知目录条目的问题被拒收。
2. 解决后状态变为 resolved，笔记与首页同步更新；重复解决被拒收。
3. 位置进入问题的 `node` 时，`learn next` 返回 `address_question`。
4. 实测复验场景 13 通过。

## 决策

2026-09-28 接受，进入 v0.1.6。
