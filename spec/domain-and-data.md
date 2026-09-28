# 领域与数据边界

规范词汇见根目录 [CONTEXT.md](../CONTEXT.md)。

## 所有权

| 数据 | 所有者 | 可变性 | 默认 Git 策略 |
| --- | --- | --- | --- |
| Source 原件 | 用户 | 同一版本只读 | 大文件默认忽略 |
| Source manifest | Runtime | 新版本追加 | 跟踪 |
| Conversation | 用户事实记录 | append-only | 跟踪 |
| Session | Runtime 的解释 | 可重建 | 跟踪 |
| Curriculum | 用户选择的学习主线 | 显式修改 | 跟踪 |
| Curriculum State | Runtime | 会话结束时原子更新 | 跟踪 |
| Interpretation Record | Agent 提交、Runtime 校验 | append-only，可 retraction | 跟踪 |
| Learner Model | Runtime 回放派生 | 可删除重建 | 忽略（`learner-model.json`） |
| Learner State 投影 | Runtime | 证据驱动更新，保留用户区 | 跟踪 |
| `.learning` 状态 | Runtime | 命令管理 | 选择性跟踪 |

## Vault 布局

```text
Vault/
├── Conversations/
├── Sessions/
├── Sources/
│   └── <source-id>/
│       ├── manifest.yaml
│       ├── original/<file>
│       └── extracted/
├── README.md             # v0.1.4 生成的学习首页
├── Questions/            # v0.1.6 学习者关键问题笔记，按需创建
├── Concepts/
├── Curriculum/
│   └── <curriculum-id>/
│       ├── book.md
│       ├── outline.yaml
│       ├── current-position.md
│       ├── 学习进度.md       # 由 progress.yaml 生成（v0.1.6 前为 progress.md）
│       ├── progress.yaml     # v0.1.3 完成、跳过与 Session 记录，append-only
│       ├── <书名>.md         # 教材首页投影（v0.1.6 前为 index.md）
│       └── detours.yaml      # v0.1.2 Detour 日志
├── Profile/
├── AGENTS.md
├── CLAUDE.md
├── .agents/skills/learning-os/
├── .claude/skills/learning-os/
├── .learning/
│   ├── config.yaml
│   ├── state.json
│   ├── schema-version
│   ├── tmp/                                     # v0.1.3 Agent 临时文件，Git 忽略
│   ├── archive/<archive-id>/                    # v0.1.4 归档的 Source 与 Curriculum
│   ├── interpretations/<session-id>/NNNN.json   # v0.1.2
│   └── model/learner-model.json                 # v0.1.2，派生缓存
└── .git/
```

v0.1.4 起 `init` 不再创建 `Questions/`、`Ideas/`、`Hypotheses/`、`Misconceptions/`、`Insights/`，`agent update` 会删除其中完全为空的目录。

## 教材导入

教材导入分为 Source 注册和 Curriculum 激活：

1. 解析输入路径并确认可访问。
2. 计算 SHA-256，识别重复内容。
3. 生成只包含计划变更的 dry-run 结果。
4. 复制到 Vault 或显式登记外部链接。
5. 写入 manifest；同 ID 不静默覆盖。
6. 为 Markdown/Text 建立可读内容；PDF v0.1 保存原件和元数据，无法可靠提取时明确报告。
7. 创建 Curriculum 初始文件并可选择激活。

默认使用 `copy`，保证可迁移。`link` 模式保存用户给出的路径，路径失效时状态必须可诊断。原件更新产生新版本，不覆写旧版本。

## 学习进度

Source 不保存学习程度。Curriculum State 保存教材位置、完成项、detour 和 return point；Learner State 保存概念证据；Session 与 Conversation 保存来源链。Review 从这些数据推导，不回写 Source。

## 会话完整性

- `session start` 创建唯一 ID、开始时间、Conversation 目标和起始 Curriculum State。
- `session append` 追加带角色和时间的原始内容，不提供普通覆盖接口。
- `session end` 先分析并校验所有候选，再原子写入派生状态。
- 分析失败保留活跃会话和原始 Conversation，不改变既有状态。
- Git 失败不回滚已安全写入的数据，但必须返回清晰错误。
