# 产品需求

## 愿景

持续建立一个关于用户如何学习、理解和思考的个人 Learner Model，并让过去的学习证据真正改变未来 AI 的教学方式。

Personal Learning OS 的核心不是让 AI 单次回答得更好，也不是自动生成知识笔记。系统必须长期保留用户原来如何理解、哪里理解错误、什么证据促成了变化、当前理解处于什么状态，以及能否在新场景中检索、应用和迁移。

## 核心闭环

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

闭环成立的必要条件是：下一次教学动作能够说明它使用了哪些历史证据和当前 Learner State。只保存历史、生成摘要或建立知识库而不改变未来教学，不构成完整闭环。

## 长期竞争力

1. **长期 Learner Model**：持续形成有证据、可修正的认知状态，而不是一次会话内的临时印象。
2. **可追溯 Cognitive History**：保留原始学习过程、认知事件、模型修订及其证据；AI 判断可以更新，事实历史不能被改写。
3. **过去学习改变未来教学**：Learner Model 必须参与决定下一步是解释、提问、测试、补先修知识、切换策略、复习还是继续教材。

普通 AI Tutor 主要优化当前回答，普通 AI Notes 主要建立用户的知识库；Personal Learning OS 建立关于用户学习过程的模型。Obsidian、Markdown、Git、Go CLI 和 Knowledge Graph 是实现与可移植手段，不是产品壁垒。

## 成功标准

系统能够用明确证据回答：

1. 我目前如何理解某个概念？
2. 我曾经在哪里理解错误？
3. 理解何时、因为什么发生变化？
4. 哪段原始 Conversation 支持这个判断？
5. 当前教材位置和完成进度是什么？
6. 为什么下一步采用这种教学或复习方式？

## 原则

- Local-first：没有云数据库也能管理 Vault。
- User-owned：长期数据使用 Markdown、YAML、普通目录和 Git。
- Evidence-first：Conversation 是 append-only 的事实，解释可以重算。
- Runtime commits：模型提出结构化候选，Go 校验并原子落盘。
- Model is defeasible：Learner Model 是当前最佳解释，可以被后续证据修正；“掌握”不是永久事实或无依据分数。
- History changes policy：每次教学策略选择都应能够追溯到 Curriculum、Learner Model 或 Cognitive History。
- Curriculum fidelity：选择教材后默认沿教材主线前进，绕行必须有原因和返回点。
- Curriculum constrains policy：Learner Model 决定怎么学，Curriculum 决定主线学什么；Next Best Learning Action 必须同时满足学习者需要和课程约束。
- Guided Socratic：默认先提取学习者现有模型，再用预测、理由、反例、修正和迁移推动认知变化；先修缺口使用直接支架教学，不把苏格拉底式教学退化为连续反问。
- Model beyond mastery：Concept State 只是 Learner Model 的一部分；系统还要积累跨课程 Learning Pattern 与 Strategy Evidence。
- Policy learns：系统不仅更新用户哪里会或不会，也根据后续 Cognitive Change 更新“什么策略在什么情境下可能有效”的证据。
- Portable agent use：Agent 通过 CLI 和生成的 Skill 工作，不绑定特定厂商。
- Agent for meaning, Runtime for rules（2026-09-26 确认）：凡是需要语义理解的工作（解读学习者、读原书目录与原文、写教材要点、出题、判断回答、判断概念之间的关系）都由 Agent 完成；程序只做固定化的事情：存储、校验、状态推导、确定性规则、Markdown 投影，以及通过 Skill 下发的提示词流程。程序不内置模型调用。
- Multi-platform（2026-09-26 确认）：Runtime 与 Vault 必须在 macOS、Linux、Windows 上行为一致，不依赖任何单一平台的系统框架或宿主专有能力。
- Obsidian as the graph：知识图谱由 Obsidian 呈现；程序负责生成双链与标签，不自建图谱可视化。

## v0.1 不包含

GUI、移动端、Obsidian 插件、云同步、服务端、守护进程、向量数据库、RAG 基础设施、知识图谱可视化、复杂推荐、多模型路由、自动间隔重复、OCR、音视频和图像理解。
