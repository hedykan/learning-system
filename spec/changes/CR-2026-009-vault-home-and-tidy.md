---
id: CR-2026-009
title: "Vault 首页 README.md 与清理空的旧目录"
status: implemented
target_version: v0.1.4
created: 2026-09-26
---

# 变更摘要

在 Vault 根目录生成 `README.md` 作为学习首页，打开 Vault 就能看到正在学什么、学到哪里、下一步做什么，并能跳到所有总览页面；同时不再创建、并安全清理 v0.1 遗留的空目录。

## 动机

2026-09-26 检查真实 Vault 与测试副本：根目录只有写给 Agent 的 `AGENTS.md`、`CLAUDE.md`。学习者总览、教材首页、进度页和 Session 解读分散在 `Profile/`、`Curriculum/<id>/`、`Sessions/` 下，学习者必须事先知道路径才能找到。

根目录还有 `Questions/`、`Ideas/`、`Hypotheses/`、`Misconceptions/`、`Insights/` 五个目录。它们由 v0.1 的 `init` 创建，v0.1.2 起 Runtime 不再写入，始终为空，只会干扰浏览。

## 当前行为

- `init` 不生成任何面向学习者的首页。
- 投影只在已有 Interpretation Record 时生成；刚初始化或刚导入教材的 Vault 没有任何可读总览。
- `init` 创建上述五个目录，任何命令都不会清理它们。

## 目标行为

### 首页 README.md

- Runtime 在 Vault 根目录生成 `README.md`，frontmatter 带 `generated_by: learn`，末尾保留“手写笔记”区。
- 内容依次为：
  - **正在学习**：当前教材标题与首页链接、当前位置（目录条目编号与标题）、目录是否已确认、`learn next` 推荐的下一步。
  - **我的教材**：每本教材一行，链接到教材首页和进度页，显示已完成、已跳过与总条目数。
  - **学习者总览**：链接到 `Profile/learner-state.md`，按形成中、脆弱、稳定统计概念数量，列出待修正误解的数量。
  - **最近学习**：最近 5 次 Session 的日期和链接。
  - **最近变化的概念**：最近 5 次概念状态更新，含新状态与链接。
  - **怎么用**：在本目录启动 Codex 或 Claude，说“继续学习”；导入教材时告诉它文件路径。
- 没有学习记录时首页仍然生成，各段显示明确的空状态和下一步操作，例如“还没有导入教材”“目录尚未确认”。
- 以下时机刷新首页：`init`、教材导入与激活、目录保存与确认、位置变更、完成与跳过、Detour、checkpoint、Session 结束与中止、`model rebuild`。
- 首页与其他投影一样可由 `model rebuild` 逐字重建，输出不依赖当前时间。
- 已存在且不含 `generated_by: learn` 的 `README.md`，其原有内容整体移入手写笔记区，不丢失任何文字。

### 清理空的旧目录

- `init` 不再创建 `Questions/`、`Ideas/`、`Hypotheses/`、`Misconceptions/`、`Insights/`。
- `learn agent update` 的预演列出其中完全为空的目录（忽略 `.DS_Store`）；`--yes` 时删除它们。含有任何其他文件的目录保持不动，并在结果中注明“保留：含用户文件”。

## 数据与兼容性影响

- 新增根目录 `README.md` 投影；不改变任何已有数据格式。
- 删除的只是空目录，Git 历史不受影响。
- 规范中的 Vault 布局同步移除这五个目录。

## 风险

- 用户可能已有自己的 `README.md`：通过移入手写区保证零丢失。
- 首页刷新点多，需保证与其他投影一致且不产生多余的 Git 噪音；内容不变时不重写文件。
- 删除目录前必须再次确认为空，避免竞态删除用户刚放入的文件。

## 验收条件

1. 新 `init` 的 Vault 根目录有 `README.md`，并且不存在上述五个目录。
2. 导入并确认目录、设置位置、完成一节、结束一次 Session 后，首页的正在学习、我的教材、最近学习与最近变化的概念均正确更新。
3. 删除首页后 `model rebuild` 逐字重建；手写区内容在重建后保持不变。
4. 已有的非生成 `README.md` 升级后原文完整出现在手写区。
5. `agent update --dry-run` 列出将删除的空目录且零写入；`--yes` 只删除空目录，含文件的目录保留并被注明。
6. 首页链接在 Obsidian 中全部可跳转：教材首页、进度页、学习者总览、Session 与概念笔记。

## 决策

2026-09-26 接受，进入 v0.1.4。
