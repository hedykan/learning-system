---
id: CR-2026-005
title: "教材章节遵循：目录、位置校验、完成记录与依据原文教学"
status: implemented
target_version: v0.1.3
created: 2026-09-26
---

# 变更摘要

让学习历程真正跟随导入的教材：导入后建立经用户确认的目录，教材位置只能指向目录中的条目，学完一节可以显式标记完成，Agent 讲课前先读当前小节对应的原文页。

## 动机

2026-09-26 用 Codex 在 DDIA 第二版 Vault 副本上实测：

- `Curriculum/ddia-2e-zh/outline.yaml` 为空，Agent 无从查阅目录，凭记忆回答“本章后面还有什么”，列出的是第一版第 1 章结构。
- 摸底推荐入口时写下“第1章 可靠、可扩展与可维护的应用”，这是第一版标题；第二版第 1 章是“数据系统架构中的权衡”，可伸缩性在第 2 章“定义非功能性需求”。此后所有 Session 沿用错误位置，真正的第 1 章被跳过且没有任何记录。
- 学习者说“这一节学完了，进入下一节”后，Agent 只改了 `current_concept`，新概念名由 Agent 自拟；`last_completed` 仍为空，`progress.md` 只追加“某 Session 在记录的位置完成”。
- Interpretation Record 的 `source_ref.chapter` 写成“第1章”，与位置中的完整章节名不相等，导致 Learning Policy 的章节范围规则（R2、R3）对这些概念失效。

## 当前行为

- PDF 导入只保存原件与元数据，不产生目录。
- `curriculum position set` 接受任意字符串，不校验是否存在于教材。
- 没有“完成某节”或“跳过某节”的命令；`last_completed`、`next_textbook_step` 无写入路径。
- Skill 不要求讲课前阅读原文，教学内容可能来自模型记忆或其他版本。

## 目标行为

### 目录（Outline）

- `outline.yaml` 保存层级条目：`id`（如 `2`、`2.3`）、`title`、可选 `pages: [start, end]`。
- Markdown、文本和目录类 Source 在导入时由 Runtime 根据标题自动生成目录草稿。
- PDF 由 Agent 读取原书目录页后提交：`learn curriculum outline set <id> --file <path|-> [--dry-run]`。Runtime 校验 ID 唯一、层级连续、页码单调且不超出 Source 页数。
- 目录状态为 `draft` 或 `confirmed`。`learn curriculum outline confirm <id>` 需要学习者确认后执行；确认同时视为该 Source 的内容核验（吸收 CR-2026-001 中 `source verify` 的意图）。
- `learn curriculum outline show <id> [--json]` 输出目录与每个条目的完成状态。

### 位置校验

- 目录为 `confirmed` 时，`position set` 必须使用 `--node <id>`；章节、小节标题由 Runtime 从目录填充，拒绝不存在的条目。
- 目录缺失或为 `draft` 时，`position set` 仍接受自由文本，但 `status --json` 报告 `position_verified: false`，Skill 要求先建立目录。
- Interpretation Record 的 `source_ref` 增加可选 `node`；省略时 Runtime 以当前位置的 node 补齐。Learning Policy 的章节范围改为按 node 前缀匹配，不再比较标题字符串。

### 完成记录

- `learn curriculum complete <node-id> --reason <text>` 追加完成记录（时间、Session、理由），不影响 Learner State。
- `learn curriculum skip <node-id> --reason <text>` 记录主动跳过，区别于遗漏。
- 进度以结构化 `progress.yaml` 追加保存；`progress.md` 与教材首页由 Runtime 生成，显示每个条目的状态：已完成、已跳过、进行中、未开始。
- 位置越过尚未完成或跳过的条目时，教材首页标注“未覆盖”，`status --json` 列出这些条目。
- `learn next` 的 R6 `continue_curriculum` 给出目录中下一个未完成条目的 node 与标题。

### 依据原文教学

- Skill 要求 Agent 讲授某条目前，先阅读该条目 `pages` 范围内的原文，并按原书小节顺序推进；原文不可读时明确告知学习者，不以记忆替代。
- 摸底推荐入口必须引用目录 node。

### 既有 Vault 迁移

- 旧位置保持可读，标记 `position_verified: false`。
- Skill 指引 Agent 在下一次学习开始时：建立并确认目录，把当前位置映射到正确 node，并对位置之前从未学过的条目询问学习者是补学还是标记跳过。

## 数据与兼容性影响

- `outline.yaml` 从 `chapters: []` 升级为带 `status` 与层级条目的结构；空旧文件视为缺失。
- 新增 `progress.yaml`（append-only）；旧 `progress.md` 内容保留在生成文件的历史段落中。
- `current-position.md` 增加 `node` 字段；缺失时视为未校验。
- Interpretation Record schema 保持 `@1`，`source_ref.node` 为可选新增字段。

## 风险

- Agent 从 PDF 读目录可能出错；必须经过 dry-run 与学习者确认才能生效。
- 过严的位置校验会阻碍 Detour 和临时探索；Detour 仍按 CR-2026-004 记录，不要求 node。
- 完成标记可能被误读为“已掌握”；投影必须把教材进度与理解状态分列。
- 扫描版 PDF 无法读取目录时只能人工录入，OCR 仍不在范围内（IDEA-002）。

## 验收条件

1. Markdown Source 导入后自动生成与标题层级一致的目录草稿。
2. 非法目录（重复 ID、层级断裂、页码倒序或越界）被拒绝且零写入。
3. 目录确认后，`position set --node` 接受合法条目、拒绝不存在的条目，并自动填入标题。
4. `complete` 与 `skip` 追加记录，教材首页显示对应状态；位置越过的未处理条目被标注为“未覆盖”。
5. `learn next` 在 R6 时给出下一个未完成条目。
6. 省略 `source_ref` 的概念自动获得当前 node；R2、R3 能对这些概念生效。
7. 旧 Vault 升级后 `status --json` 显示 `position_verified: false`，所有旧文件可读。
8. Agent 测试：在 DDIA 第二版副本上，Agent 建立目录、把位置更正到第 2 章“可伸缩性”，并询问第 1 章是补学还是跳过；讲课前读取了对应页码。

## 决策

2026-09-26 接受，进入 [v0.1.3 基线](../versions/v0.1.3.md)。
