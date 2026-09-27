# Personal Learning OS

## 产品现状、核心目标与下一阶段计划

**当前版本：** v0.1.1  
**项目阶段：** 可本地使用的早期版本  
**当前重点：** 从可靠记录学习过程，走向长期 Learner Model

---

## 1. 产品是什么

Personal Learning OS 是一个本地优先、用户拥有数据的个人学习系统。

它的核心不是让 AI 在某一次对话中“讲得更好”，也不是自动生成大量笔记，而是：

> 持续建立一个关于用户如何学习、理解和思考的个人模型，并让过去的学习真正影响未来 AI 的教学方式。

普通 AI Tutor 主要优化当前回答，普通 AI Notes 主要建立用户的知识库；Personal Learning OS 希望建立的是：

> 关于用户学习过程的长期 Learner Model。

Obsidian、Markdown、Git、Go CLI 和未来的 Knowledge Graph 都是实现这一目标的技术手段，而不是产品本身的核心壁垒。

---

## 2. 核心闭环

```text
Raw Learning History
        ↓
Learning Events
        ↓
Learner Model
        ↓
Learning Policy
        ↓
Next Best Learning Action
        ↺
```

系统希望长期回答这些问题：

1. 用户现在如何理解某个概念？
2. 用户过去在哪些地方理解错误？
3. 什么问题、反例或实践促成了理解变化？
4. 哪段真实对话支持当前判断？
5. 用户只是听过、能解释，还是已经能够检索、应用和迁移？
6. 下一步为什么应该解释、提问、测试、补先修知识、复习或继续教材？

如果过去的学习没有改变未来的教学动作，那么即使生成了很多笔记，也没有完成 Learning OS 的核心闭环。

---

## 3. 当前已经完成的能力

### 3.1 本地运行与数据所有权

- 使用 Go 开发，可通过 `go install` 安装为本地 `learn` 命令。
- 数据保存在用户自己的普通目录中，不依赖云数据库。
- 使用 Markdown、YAML 和 JSON 保存内容，可以直接用 Obsidian 或编辑器查看。
- Vault 可以初始化 Git，用于追踪学习历史的变化。
- Codex 和 Claude 可以通过 Vault 中的 Rules 与 Skills 使用同一套学习流程。

### 3.2 教材管理

- 支持导入 PDF、Markdown、纯文本和目录。
- 支持复制到 Vault，或保留为外部链接。
- 正式导入前可以 dry-run，预览目标位置、内容哈希和重复情况。
- 支持教材列表、详情、激活和当前位置管理。
- 教材原件与学习进度分开保存，不会把学习状态写进 Source。

### 3.3 原始学习历史

- 每次学习会话生成独立 Conversation。
- 用户和 AI 的对话按时间追加，保持 append-only。
- 后续分析可以改变，但已经发生的原始对话不会被重新润色或覆盖。
- Session 结束后可以生成一份可追溯到原 Conversation 的解释文档。

### 3.4 首次摸底

- 新教材首次学习时，默认先进行 Baseline Assessment。
- 支持 quick、standard 和 deep 三种摸底深度。
- 摸底期间不会提前推进教材位置。
- 摸底结论必须引用学习者真实回答，不能使用 AI 自己的解释冒充证据。
- 摸底结果包括已有知识、先修缺口、可能误解、熟悉与陌生术语，以及推荐起点。
- 用户可以显式跳过摸底，但系统不会因此伪造学习者状态。

### 3.5 Guided Socratic Learning

当前 Agent Skill 默认使用以下教学循环：

```text
引出当前理解
    ↓
要求具体预测
    ↓
追问理由
    ↓
提供变化条件或反例
    ↓
让学习者修正模型
    ↓
在新场景中验证迁移
```

同时包含以下约束：

- 从具体场景出发，再命名抽象概念。
- 每轮默认最多引入两个尚未建立的新术语。
- “我懂了”只是一条证据，不等于掌握。
- 缺少基础概念时，允许先做最小直接解释，再恢复苏格拉底式探究。
- 同一种解释连续无效时，应该切换例子、反例、图示或先修路径。

### 3.6 会话恢复与安全操作

- Session 支持 baseline、lesson、review 和 practice 类型。
- 错误或过时的 Session 可以安全中止，不会推进学习进度。
- Curriculum Position 可以显式重置。
- Agent Rules 与 Skills 可以先预演、再更新；旧文件会自动备份。
- 正常学习时，内部记录操作在后台静默执行，不打断教学对话。

---

## 4. 当前版本的真实限制

v0.1.1 已经完成可靠记录、首次摸底和基础教学流程，但还没有完成完整 Learner Model。

当前尚未实现：

- 从普通 Lesson 中可靠提取误解、洞察、理解修正、检索和迁移。
- 自动生成和维护长期 `Learner State`。
- 自动生成 Concept、Question、Hypothesis、Misconception 和 Insight 笔记。
- 把认知变化组织成可追溯的 Cognitive History。
- 让历史 Learner Model 系统性参与下一步教学决策。
- 为每个 Next Best Learning Action 保存选择理由和历史证据。
- 在 Obsidian 中生成课程首页、学习者概览和完整双向链接。
- 在长会话中进行阶段性 checkpoint。
- PDF 正文自动提取、扫描版 OCR 和自动章节识别。
- 自动间隔复习、遗忘模型和复习调度。
- Git 代码项目的 revision 级学习管理。
- Source 的完整验证、归档、恢复和永久删除生命周期。

目前 Session 中虽然已经预留 Learning Events 和 Cognitive Changes 结构，但内置分析器仍然非常保守，主要只能识别学习者提出的问题。因此当前系统不能声称已经拥有完整的自动认知分析能力。

---

## 5. v0.1.2 的核心目标

v0.1.2 的目标是：

> 把学习对话转化成可追溯的 Learner Model，并让这个模型决定下一步怎么教。

### 5.1 稳定的证据定位

每轮对话将拥有稳定 turn ID。任何学习事件或状态判断都必须引用：

```yaml
conversation: session-xxx
turn: turn-006
quote: "学习者的原始回答"
```

Runtime 会校验证据确实存在于对应的学习者回合中。

### 5.2 结构化 Learning Events

计划支持：

```text
question
prediction
attempt
misconception
correction
insight
understanding_revision
prerequisite_gap
retrieval
application
transfer
strategy_switch
```

AI 可以提出候选分析，但只有通过 Runtime 证据校验后才能进入长期状态。

### 5.3 Cognitive History

重点记录：

```text
Old Model
    ↓
Trigger
    ↓
New Model
```

例如，学习者原本只看平均延迟，在观察到最慢 1% 用户的实际数量后，修正为同时关注 p99 和尾延迟。系统保存的不只是最终定义，还包括理解如何发生变化。

### 5.4 长期 Learner Model

每个概念将保存：

- 学习者原话；
- 当前理解模型；
- AI 的解释与补充；
- 已发现和已修正的误解；
- explanation、retrieval、application 和 transfer 证据；
- 尚未解决的问题；
- 历史状态变化；
- 下一次应该验证什么。

候选状态为：

```text
unobserved
developing
fragile
stable
```

状态不是分数，也不是永久结论。新的反例或失败可以把 `stable` 修订为 `fragile`，同时保留此前判断的证据与历史。

Concept State 只是 Learner Model 的一部分，不能把系统退化成 Mastery Tracker。长期还需要发现：

- 用户通常怎样从具体经验形成抽象理解；
- 哪些误解模式会在不同领域重复出现；
- 什么解释、问题或反例在什么情境下更有效；
- 什么情况下更容易产生 retrieval、application 和 transfer；
- 用户的提问、预测和推理方式如何随时间变化。

跨课程模式比单一概念状态更有长期价值，但必须由多个独立情境支持，不能凭一次表现给用户贴固定“学习风格”标签。

### 5.5 Obsidian 知识投影

计划生成和维护支撑核心闭环所需的最小投影：

```text
Concepts/
Questions/
Hypotheses/
Misconceptions/
Insights/
Profile/learner-state.md
Sessions/
```

Concept Note 将明确区分：

```markdown
## 外部知识
## 学习者原话
## 当前理解模型
## AI 补充
## 误解与修正
## Evidence
## Open Questions
## History
```

AI 不会使用第一人称冒充学习者总结“我的理解”。Runtime 重新生成投影时，也必须保护用户手写内容。

只有具备明确证据和独立生命周期价值时，才创建单独的 Question、Hypothesis、Misconception 或 Insight 文件。v0.1.2 不以自动生成大量笔记、Dashboard 或 Knowledge Graph 可视化为目标。

### 5.6 Checkpoint 与幂等更新

长会话完成一个概念后即可 checkpoint，不必等整段 Session 结束才在 Obsidian 中看到沉淀结果。

checkpoint 和 end 必须满足：

- 重复执行不会生成重复笔记；
- 写入失败不会留下相互矛盾的半更新状态；
- 原始 Conversation 始终保留；
- 生成投影可以安全重建；
- 用户手写内容不会被覆盖。

### 5.7 Curriculum Fidelity

Learner Model 决定“怎么学”，Curriculum 决定“主线学什么”：

```text
Next Best Learning Action
=
Learner-optimal Action
∩
Curriculum-compatible Action
```

允许为了 prerequisite 临时偏离教材，但必须记录偏离原因、学习内容、return point 和返回条件。补齐缺口后应回到教材主线；AI 不能仅凭 Learner Model 随意重组整套教材。

### 5.8 Learning Policy 也要学习

系统不仅记录用户哪里不会，还要积累什么策略在什么情境下可能有效：

```text
Learner Model
↓
Choose Strategy
↓
Learning Action
↓
Observe Cognitive Change
↓
Evaluate Strategy Evidence
↓
Update Policy Evidence
```

每次策略尝试保存使用情境、预期变化、实际结果和证据。结果是可修正判断，不因一次成功就声称确定因果。`strategy_switch` 要连接前一策略、切换原因、新策略与后续认知变化。

### 5.9 Next Best Learning Action

Learner Model 将参与选择下一步应该：

- 继续解释；
- 要求学习者预测；
- 提供反例；
- 测试迁移；
- 补充先修知识；
- 切换教学方式；
- 复习旧概念；
- 返回教材主线。

每个动作都应记录目标、原因和证据，例如：

```yaml
action: transfer_probe
concept: tail-latency
reason: 已能解释 p99，但尚无跨场景迁移证据
evidence:
  - session-xxx/turn-006
```

动作还必须说明它与 Curriculum 的关系：保持主线、进入 prerequisite detour，或返回已记录的 return point。

### 5.10 v0.1.2 的唯一核心验收

> 同一个用户连续学习 10～20 个 Session 后，系统是否真的比 Session 1 更理解这个用户，并因此做出不同且有历史证据支持的教学动作？

验收需要比较无历史和有历史两种情况下的教学选择，并证明差异来自跨 Session Learner Model 或 Strategy Evidence，同时没有破坏 Curriculum Fidelity。

v0.1.2 暂不把重点放在 Knowledge Graph 可视化、Dashboard、OCR、自动复习系统、大量自动笔记或复杂 Mastery 分数。

---

## 6. 版本路线

| 版本 | 目标 | 状态 |
| --- | --- | --- |
| v0.1 | 建立本地 Vault、教材、Conversation、Session 和 CLI 基础闭环 | 已完成 |
| v0.1.1 | 增加首次摸底、证据校验、Guided Socratic 和安全恢复 | 已完成 |
| v0.1.2 | 从学习证据建立 Cognitive History 与长期 Learner Model，并驱动下一步教学 | 需求评审中 |
| 后续版本 | PDF/OCR、自动复习、项目学习、完整 Source 生命周期与更强分析能力 | 候选 |

---

## 7. 产品的长期竞争力

真正可能形成长期竞争力的不是某个模型、提示词或笔记模板，而是：

> 持续积累的个人 Cognitive History + Learner Model + Learning Policy。

随着使用时间增长，系统应该越来越了解：

- 用户容易在哪些地方形成误解；
- 哪种问题能促成真正的认知变化；
- 用户对哪些概念只能识别、哪些能够解释、哪些能够迁移；
- 什么教学方式对当前用户和当前概念更有效；
- 下一步怎样学习最有价值。

系统最终形成的不是一堆静态笔记，而是一份可追溯、可修正、持续参与未来教学的个人学习模型。
