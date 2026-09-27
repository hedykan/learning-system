---
id: CR-2026-007
title: "Agent 实测暴露的可用性缺陷"
status: implemented
target_version: v0.1.3
created: 2026-09-26
---

# 变更摘要

修复 2026-09-25 Codex 实测中发现的、会让 Agent 卡住、绕路或留下脏数据的问题。

## 问题与目标行为

| 问题 | 目标行为 |
| --- | --- |
| `session append` 无管道输入时一直等待，Codex 调用被强制中断 | `append` 增加 `--file <path>`；标准输入是终端时立即报错并提示用法；Skill 统一使用带引号的 heredoc |
| Agent 为绕开转义在 Vault 根目录写临时文件，中断时会残留并被 Git 提交 | Skill 规定临时文件放在 `.learning/tmp/`，Runtime 创建该目录时写入忽略规则 |
| Codex 沙箱禁止写 `.git`，每次自动提交都失败，历史没有版本保护 | 新增 `learn commit [--message]`，供学习者在沙箱外补提交；`status --json` 报告 `git_last_commit`；文档说明如何在 Codex 中放开该目录 |
| 同一轮原话因引用片段重叠在概念笔记中出现两次 | 投影按轮次合并引用，只保留最长片段 |
| 学习者的错误被纠正后，记录里只有“能应用”，没有误解事件 | Interpretation workflow 增加规则：教学中纠正过的错误必须记录 misconception 与 correction |

## 数据与兼容性影响

不改变数据格式；`.learning/tmp/` 为新增的忽略目录。

## 验收条件

1. `session append` 在终端标准输入且无 `--text`、`--file` 时立即失败并给出用法。
2. `--file` 与 heredoc 两种方式写入的轮次内容与输入逐字一致。
3. `learn commit` 在有未提交变更时生成提交，没有变更时返回明确提示。
4. 概念笔记中同一轮次的重叠引用只出现一次。
5. Codex 实测中不再在 Vault 根目录出现临时文件。

## 决策

2026-09-26 接受，进入 [v0.1.3 基线](../versions/v0.1.3.md)。
