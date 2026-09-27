---
id: CR-2026-013
title: "Obsidian 知识图谱：概念互链与状态标签"
status: implemented
target_version: v0.1.5
created: 2026-09-26
---

# 变更摘要

Agent 在解读中标注相关概念，程序在概念笔记中生成双向的“相关概念”链接，并为每张笔记写入标签和别名，让 Obsidian 图谱呈现概念网络并可按理解状态着色。

## 目标行为

- Interpretation Record 的 `concepts[]` 新增可选 `related`：`[{"concept": "<id>", "note": "<关系说明，可选，最多 40 字>"}]`。被引用的概念必须已存在或在同一记录中声明；不能与自身相关。
- 关系是无向的，两张笔记都显示；同一对概念重复提交时保留第一次的说明。
- 概念笔记新增“## 相关概念”，列出双链与说明。
- 概念笔记 frontmatter 新增 `aliases`（概念别名，便于 Obsidian 按别名链接）与 `tags`：`learning/state/<状态>`、`learning/curriculum/<教材 id>`。
- README 说明如何在 Obsidian 图谱设置中按标签分组着色。

## 验收条件

1. 提交相关概念后，两张概念笔记都出现互相的双链。
2. 引用未知概念或自身的关系被拒收。
3. 概念笔记的 tags 随状态变化而更新，aliases 与概念别名一致。

## 补充：程序兜底（2026-09-27）

v0.1.5 实测中 Agent 两次都没有提交 `related`。按“语义交给 Agent、程序只做固定流程”的原则增加兜底：

- `session end`（baseline 除外）时，若本 Session 涉及的概念没有任何相关概念、也未被声明为 `no_related`，且同一教材中存在其他概念，则拒绝结束并列出可关联的已有概念；记录不会写入，Session 保持活动。
- 记录新增顶层 `no_related`：Agent 检查后认为与所有已有概念都不相关的概念 ID。声明后该概念不再被拦，直到它有了相关概念。
- 概念所属教材按其 `source_ref` 或其出现过的学习 Session 判断。
- `--no-analysis` 结束不做此检查。

## 决策

2026-09-26 接受，进入 v0.1.5。2026-09-27 按用户要求加入程序兜底。
