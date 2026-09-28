---
id: CR-2026-040
title: "从学习目标建立课程（取代 CR-2026-032）"
status: implemented
target_version: v0.2
created: 2026-09-28
---

# 变更摘要

`learn curriculum import --goal "<学习目标>"` 建立一门没有主资料的 synthesized 课程。Agent 研究这个方向，草拟带 `why`、先修和概念的大纲，学习者确认后，Agent 搜集可靠资料（`learn source add <url|文件>`）挂到条目上。没有任何资料的条目明确标为“AI 综合”。

## 动机

没有成型教材的方向（如“向量数据库”“某个新框架”）最容易变成零散问答。[Curriculum Builder](../proposals/curriculum-builder.md) 要求每个重要 Topic 保留来源、为什么学、先修与位置，避免 AI 凭空生成看似合理的课程。CR-2026-032 按此重写为本变更单。

## 目标行为

- `--goal` 需要 `--id` 与 `--title`；课程资料种类为 `goal`，没有文件，读取返回 `unsupported`（“挂上资料再读”），条目定位只接受 `text`。
- 大纲类型为 `synthesized`，状态为 missing，等待 Agent 提交。
- 条目有挂载资料才算有来源；`source check` 的 `unsourced` 列出没有来源的叶子条目；`learn next` 对这类条目输出 `unsourced: true`；教材首页在条目后标“（AI 综合，无原始资料）”，学习首页“资料”段显示未覆盖的条目数。
- 没有来源的条目，其概念不能提交教材要点（拒收并说明）。
- 程序不联网搜索，也不评判资料质量；Skill 规定来源优先级：官方文档、原始论文、知名教材与课程、作者本人的文章，其次才是博客。

## 数据与兼容性影响

纯新增资料种类；依赖 CR-2026-027 的挂载与 CR-2026-031 的网页快照。

## 验收条件

1. 目标课程可建立、提交大纲、确认、挂资料、学习。
2. 无来源条目在 `next`、首页、教材首页有标注；提交其教材要点被拒。
3. 真实 Agent 从一个学习目标出发完成研究、建纲、确认与挂资料，并学完一节。

## 决策

2026-09-28 接受，排入 v0.2。

2026-09-28 实现：`learn curriculum import --goal`；资料种类 `goal`（无内容，只接受 text 定位）；未挂资料的叶子条目由 `source check` 报告，`next` 输出 `unsourced`，首页与教材首页标注，教材要点拒收；Skill 新增 `references/curriculum-builder.md`。
