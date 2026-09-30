---
id: CR-2026-049
title: "复习课插入学习课"
status: implemented
target_version: v0.2.4
created: 2026-09-30
---

# 变更摘要

学习课（lesson）或练习课（practice）进行中，可以用 `learn session start --kind review` 开一节复习课：原来的课被挂起，复习课结束（end 或 abort）后自动恢复。两节课的对话和解读记录完全分开。

## 动机

手机原型里，学习页和老师对话时会开一节学习课。这时切到复习页答题，复习问答只能写进这节学习课：对话文件混进了复习内容，刷新后学习页也显示这段问答。根因是同一时间只能有一节进行中的课，`session start` 遇到进行中的课会直接拒绝。

在前端绕开需要每次先结束学习课、复习完再开新课：要等模型整理解读，一节课被切成两段，老师也接不上话头。

## 目标行为

- `.learning/state.json` 增加 `suspended_session`（结构同 `active_session`），最多一层。
- 学习课或练习课进行中，只有 `--kind review` 可以开新课；它把当前课移到 `suspended_session`。`start --json` 输出 `suspended`。
- 以下情况仍被拒绝：已有被挂起的课（复习课里不能再插课）、当前课是复习课或摸底（baseline）、新课不是复习课。
- 复习课 `end` 或 `abort` 后，被挂起的课恢复为进行中；输出 `resumed`。`append`、`checkpoint`、`turns` 始终作用于进行中的课，所以复习期间不会写进学习课。
- 被挂起的课不能 `annotate`。
- 同一时刻开始的两节课（复习紧接着学习课开始，或固定了 `LEARN_NOW`）不再共用 ID：ID 已被占用时顺延 1 纳秒，不会覆盖已有的对话。
- `learn status` 与 `learn vaults` 显示被挂起的课（JSON 字段 `suspended_session`）。
- `learn next` 的复习判断不变：复习结果推进复习计划，恢复后到期复习不再重复出现。

## 数据与兼容性影响

`state.json` 仍是 schema 1，新字段可省略，老学习库无需迁移。旧版本的 `learn` 读取带 `suspended_session` 的状态时会忽略它；复习期间不要混用旧版本。

## 风险

- 复习课结束时的自动提交包含学习课进行中的对话文件；这与学习课中途提交解读的行为相同。

## 验收条件

1. 学习课进行中开复习课：状态显示两节课；复习对话只写入复习课文件。
2. 复习课 end 与 abort 都恢复学习课，之后的轮次编号接着学习课往下。
3. 第二节学习课、复习中再开复习、摸底中开复习都被拒绝，并给出原因。
4. 同一时刻开始的学习课与复习课各有自己的对话文件。
5. 复习结果提交后，恢复的学习课里 `learn next` 不再给出同一概念的到期复习。

## 决策

2026-09-30 接受，排入 v0.2.4。

2026-09-30 实现：`runtime.State.Suspended` 与 `Resume`，`session.Start` 的 `interrupt` 规则；status、vaults、start、end、abort 输出被挂起或恢复的课；技能文档 `review-workflow.md`、`session-workflow.md` 说明用法。测试见 `internal/app/interrupt_test.go`。

2026-09-30 e2e（Codex gpt-5.6-sol low，`~/Documents/learning-os-e2e-runs/v0.2.4`）：学习课中学习者说“先切到复习页复习一下”，Agent 提交学习课记录后开复习课，学习课被挂起；复习结果只记在复习课；结束后学习课恢复，`learn next` 回到继续学习。第一次试跑发现 Codex 的登录 shell 用的是全局安装的旧版 `learn`，并在固定时钟下复现了 ID 冲突，据此加入 ID 顺延。前两轮 Agent 把同一条回复同时写进两节课，技能文档补充“同一段文字不进两节课”的切分规则后，第三轮完全分开；这一轮 Agent 还用 abort 撤回了一次过早开始的复习课，学习课正常恢复。
