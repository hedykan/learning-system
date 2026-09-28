---
id: CR-2026-024
title: "学习记录未提交到 Git 时提醒"
status: implemented
target_version: v0.1.7
created: 2026-09-28
---

# 变更摘要

学习 Vault 有未提交到 Git 的改动时，在学习首页和 `learn status` 中提醒学习者，并给出 `learn commit` 的用法；自动提交失败时不再完全静默。

## 动机

`learn` 在 checkpoint、annotate、结束或中止学习时自动提交 Vault，但 Codex 的沙箱禁止写 `.git`，每次自动提交都失败，只在命令结果中记一笔。实测 Vault 配置开启 Git，却一个提交都没有；学习者可能长期不知道学习记录没有版本保护。

## 目标行为

- `status --json` 新增 `git_uncommitted`（未提交的文件数）与 `git_auto_commit`（最近一次自动提交的结果：`committed`、`failed` 或 `none`），并保留 `git_last_commit`。
- 最近一次自动提交的结果保存在 `.learning/state.json` 之外的运行时文件中，不改变 state schema。
- 学习首页在有未提交改动时显示一段提醒：未提交改动数、最近一次提交时间（或“从未提交”），以及在沙箱外运行 `learn commit` 的说明。没有未提交改动时不显示。
- 首页提醒依赖当前 Git 状态，与“今天该复习”一样不属于逐字重建的范围。
- Git 未启用或 Vault 不是 Git 仓库时不显示提醒。
- Skill：自动提交失败仍不打断学习，也不在课程中播报；学习者询问或 Session 结束总结时，可以提一句“学习记录还没有保存到 Git”。

## 验收条件

1. 自动提交失败后，首页出现提醒，`status --json` 报告 `git_auto_commit: failed`。
2. 执行 `learn commit` 后，下一次刷新首页不再显示提醒。
3. Git 未启用时首页没有提醒。

## 决策

2026-09-28 接受，排入 v0.1.7。

2026-09-28 实现：提醒条件为“最近一次自动提交不是成功且有未提交改动”，避免成功提交前渲染的首页把提醒一并提交；结果变化时重建首页，成功时用 amend 并入同一提交。`git status` 使用 `--no-optional-locks`，沙箱内也能统计未提交数。
