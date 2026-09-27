---
id: CR-2026-006
title: "学习者可见话术与学习对话模型配置"
status: implemented
target_version: v0.1.3
created: 2026-09-25
---

# 变更摘要

学习对话中，学习者看到的每一句话（包括调用工具前的进度提示）只谈学习内容；Vault 自带 Codex 项目配置，学习时使用较快的模型设置。

## 动机

2026-09-25 的 Codex 实测中，学习者会看到“我会使用 learning-os 技能读取你的学习状态”“进度已保存”“自动 Git 暂存因权限限制未完成”等出戏语句。CR-2026-003 的规则写在 Skill 中，无法约束宿主在调用工具前自动产生的进度提示。三组模型对比显示 gpt-5.6-sol 低思考兼顾响应速度与教学连贯性。

## 目标行为

- Vault `AGENTS.md` 新增 “Learner-facing voice” 规则，列出禁止提及的内容和示例句；`CLAUDE.md` 与 Skill 同步引用。
- 进度提示只能是关于学习者想法的一句话，或者不发；同一要点不在提示和正文中重复。
- Git 等记账失败不向学习者报告。
- 继续学习时从上一个未答完的问题接续。
- 纯控制语句（如“继续学习”“今天先到这里”）不作为学习轮次记录。
- `learn init` 写入 `.codex/config.toml`（gpt-5.6-sol、低思考），仅在 Codex 信任该目录时生效；`agent update` 不覆盖此文件。

## 数据与兼容性影响

只改变 Agent 资产；旧 Vault 通过 `learn agent update` 获得新规则，Codex 配置需手动添加或由新 `init` 生成。

## 验收条件

1. 升级后的 Vault 在 Codex 四轮实测中，学习者可见消息不含技能名、命令、保存、记录或 Git 字样。
2. 在受信任 Vault 中，Codex 会话实际使用 gpt-5.6-sol 与低思考。

## 决策

2026-09-25 按用户要求实施；Agent 资产与测试已更新，真实 Vault 已添加 Codex 项目配置。随 v0.1.3 发布。
