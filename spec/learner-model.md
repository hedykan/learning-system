# Learner Model 与知识沉淀

**产品地位：** core  
**交付状态：** 具体版本能力由变更单和版本基线决定

## 目标

知识沉淀服务于长期 Learner Model 和未来教学决策，不以生成更多 Markdown 为完成条件。系统必须能够回答：用户怎样理解、理解如何变化、判断依据是什么，以及这些历史为什么导致当前的 Next Best Learning Action。

## 规范管线

```text
Raw Learning History
        ↓ evidence reference
Validated Interpretation Record
        ↓ projection
Learner Model + Cognitive History
        ↓ Learning Policy
Next Best Learning Action
        ↓
New Raw Learning History
```

### Raw Learning History

Conversation 保存实际发生的角色、时间、原文和上下文，保持 append-only。Raw 层不预先断言一段话是 prediction、insight 或 misconception。

### Validated Interpretation Record

Agent 可以提出 Learning Event、认知变化和状态更新候选；Runtime 只接受能够定位到原始证据且通过 Schema 校验的候选。解释记录可被后续记录修正，但不能改写引用的原始证据。

### Learner Model 与 Cognitive History

Learner Model 是当前最佳解释；Cognitive History 保存模型为何以及何时发生修订。过去的误解被修正后保留历史状态并标记结果，不直接删除。

Learner Model 不等于 Concept State 集合，更不能退化成 Mastery Tracker。除了概念层面的当前状态，它还包含跨课程 Learning Pattern，以及特定教学策略在特定情境下产生何种认知变化的 Strategy Evidence。

### Obsidian Projection

Concept、Question、Misconception、Insight、Session 和索引 Markdown 是人类可读投影。投影可从规范数据重建；Runtime 必须保护明确标记的用户手写区域。

## 三个正交维度

任何沉淀记录不得用一个 `type` 混合以下维度：

1. **Artifact layer**：raw history、interpretation record、learner model、projection。
2. **Provenance**：learner、source、assistant、runtime。
3. **Epistemic status**：observed、inferred、unverified、superseded。

学习者的一句话可以同时是 learner provenance 的 observed evidence，并被解释记录引用为 learner understanding；两者不是互斥类型。

## 证据约束

- 每项 Learner State、Learning Event 和 Cognitive Change 必须引用稳定 Conversation、turn ID 和短原文。
- 学习者原话与 AI 综合解释分区展示；AI 不以第一人称冒充学习者总结“我的理解”。
- “我懂了”是 observed evidence，不足以单独产生稳定理解判断。
- 定性状态与证据能力分开：状态描述当前判断，recognized、explained、retrieved、applied、transferred 描述判断依据。
- 后续反例可以把当前状态从 stable 修订为 fragile，同时保留此前判断及其证据。

## 定性状态

候选规范状态为 `unobserved`、`developing`、`fragile`、`stable`。状态不是线性分数，也不随时间永久单调上升；它是特定概念在当前证据下的可修正判断。v0.1.1 Skill 文案中的 `solid` 从未持久化，v0.1.2 起统一使用 `stable`。各状态的最低证据门槛见 [Interpretation Record 规范](./interpretation-record.md#45-state-update)。

## Curriculum Fidelity

Learner Model 决定如何学习，Curriculum 决定主线学习什么：

```text
Next Best Learning Action
=
Learner-optimal Action
∩
Curriculum-compatible Action
```

- Learning Policy 不得仅凭 Learner Model 任意重组整套教材。
- 正常动作应保持在当前 Curriculum 路径上，或明确推动 Curriculum 中的当前目标。
- 为修复 prerequisite 临时偏离主线时，必须建立 Detour，记录偏离原因、学习内容、return point 和返回条件。
- Detour 完成或被证明不再必要时，Next Best Learning Action 应优先回到记录的 return point。
- 永久改变教材顺序、跳过大段内容或更换主线属于显式 Curriculum 决策，不是 Learning Policy 的隐式副作用。

## 跨课程 Learning Pattern

长期 Learner Model 应逐渐识别用户通常如何形成和修正理解，例如：

```text
抽象定义理解困难
→ 具体场景
→ 用户预测
→ 反例产生冲突
→ 重新抽象
→ 理解形成
```

Learning Pattern 必须由多个独立情境支持；单次表现只能形成候选。跨课程模式需要来自至少两个 Curriculum 的证据，并保留不符合该模式的反例，避免把用户固化成“视觉型学习者”等不可证伪标签。

Learner Model 至少应允许表达：

- 用户通常如何从具体经验走向抽象模型；
- 经常出现的误解形成和修正模式；
- 哪类问题、解释、反例或行动更容易促成变化；
- 什么情境更容易产生 retrieval、application 或 transfer；
- 用户提问、预测和推理方式随时间如何变化。

## Learning Policy 的证据闭环

Learning Policy 本身也必须从结果中积累证据：

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

每次可评估的策略尝试应记录策略、使用情境、预期变化、实际 Learning Event 或 Cognitive Change、证据引用和定性结果。结果使用 `effective`、`inconclusive` 或 `ineffective` 等可修正判断，不声称一次相邻发生就证明因果关系。

`strategy_switch` 同时是 Learning Event 和策略反馈机制：它必须指明前一策略为何没有产生预期变化、切换到什么策略，以及后续是否出现新证据。

## 教学闭环约束

- Learning Policy 读取 Learner Model、Cognitive History、Curriculum State 和当前会话情境。
- Next Best Learning Action 必须包含动作、目标概念、选择理由、证据和与 Curriculum 的关系。
- 同一策略连续无效时，历史应促使策略切换，而不是重复同一种解释。
- Curriculum Progress 与 Learner State 独立：完成章节不等于形成稳定理解；结束 Session 也不自动推进教材位置。

## 面向用户的核心投影

- Concept：学习者原话、当前解释、外部知识、证据、问题、连接和修订历史。
- Session：起点、事件、认知变化、受影响概念、开放问题、进度决策和下一次验证。
- Question / Hypothesis：保持 open、resolved、unverified 或 superseded 等生命周期状态。
- Learner overview：当前薄弱点、稳定理解、待验证判断、有效教学策略和推荐下一步。
- Curriculum index：教材位置与理解状态并排展示，但不互相替代。

v0.1.2 只生成支撑核心闭环所需的最小投影。有明确证据和独立生命周期价值时才创建单独的 Question、Hypothesis、Misconception 或 Insight 文件；否则保留在 Interpretation Record、Concept 或 Session 中，避免用文件数量冒充知识沉淀。

## v0.1.2 范围纪律

v0.1.2 不把开发重心放在 Knowledge Graph 可视化、Dashboard、OCR、自动复习调度、大量自动笔记或复杂 Mastery 分数。最小导航索引和证据链接属于可用性基础，不扩展成分析型 Dashboard。

本版本的首要验收问题是：

> 同一个用户连续学习 10～20 个 Session 后，系统是否比 Session 1 更理解这个用户，并因此做出不同且有历史证据支持、同时保持 Curriculum Fidelity 的教学动作？

验收必须比较无历史与有历史两种情境下的动作选择，并证明差异来自跨 Session Learner Model 或 Strategy Evidence，而不是当前一轮对话中的临时上下文。

## 非目标

- 生成教材百科摘要或章节摘要。每个概念允许的“教材要点”只有几句话，必须来自读过的原文页（见 CR-2026-011）。
- 用无测量依据的百分比表示掌握程度。
- 把 AI 推断写成客观事实。
- 为了图谱完整而自动制造概念连接。
- 把 Learner Model 简化成 Concept mastery 状态表。
- 用单次策略成功推导固定“学习风格”或强因果结论。
- 用技术载体代替产品闭环；Obsidian、Git、CLI 和图谱本身不是完成标准。
