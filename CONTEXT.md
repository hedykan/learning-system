# Personal Learning OS

Personal Learning OS 将教材、真实学习对话和可演进的学习者状态组织为本地、可读、可追溯的数据。

## Language

**Vault**:
用户拥有的学习工作区，由普通目录、Markdown、运行时元数据和 Git 历史组成。
_Avoid_: 数据库、知识库

**Source**:
导入 Vault 的学习材料及其客观元数据，可以是文档、教材或代码项目；原件在一个版本内保持只读。
_Avoid_: 课程进度、学习状态

**Project Source**:
作为学习材料登记的代码项目，其可复现身份由 Source Revision 确定。
_Avoid_: Curriculum、项目副本

**Source Revision**:
Source 在特定时间的不可变版本；代码项目通常由 Git commit 标识，非 Git 材料由内容哈希标识。
_Avoid_: 当前工作区、学习进度

**Archived Source**:
已从活动学习空间移除但仍可恢复的 Source；它不再参与激活、去重或默认查询。
_Avoid_: 已永久删除、废弃 Session

**Curriculum**:
以某个 Source 为主线组织的学习路线，包括章节结构和教学顺序。
_Avoid_: 教材文件、学习记录

**Curriculum State**:
学习者在 Curriculum 中的当前位置、完成记录、临时绕行和返回点。
_Avoid_: Source 状态、教材状态

**Conversation**:
按发生顺序保存的人与 AI 的原始学习对话，是 append-only 的事实证据。
_Avoid_: Session 总结、聊天摘要

**Session**:
Runtime 对一次 Conversation 中学习活动的结构化解释。
_Avoid_: Conversation、聊天记录

**Learning Event**:
从 Conversation 中提取且能指向证据的学习行为或认知变化信号。
_Avoid_: 日志、消息

**Cognitive History**:
按时间保存的 Learning Event、Learner Model 修订及其证据链，描述理解如何变化而不改写原始事实。
_Avoid_: Conversation、聊天摘要、活动日志

**Learner Model**:
系统对用户如何理解、学习和迁移知识的可修正整体解释，包含当前 Learner State、跨课程 Learning Pattern 和 Strategy Evidence。
_Avoid_: 用户画像、Concept State 集合、永久事实、掌握分数

**Learner State**:
Learner Model 在某个概念或学习目标上的当前定性投影，包括误解、能力证据、信心和开放问题。
_Avoid_: Learner Model、掌握分数、永久事实

**Baseline Assessment**:
首次进入一个 Curriculum 或学习领域时，用少量、逐步的问题建立初始 Learner State 的诊断活动。
_Avoid_: 考试、正式授课、掌握度打分

**Vocabulary Budget**:
一次教学交互中允许引入的新术语数量上限，用于控制认知负荷并保证每个术语都有具体语境。
_Avoid_: 词汇表、课程目录

**Guided Socratic Policy**:
默认通过提取当前理解、要求预测、追问理由、提供反例、促成修正和验证迁移来教学；发现先修缺口时允许先做直接支架教学。
_Avoid_: 连续反问、拒绝解释、固定问答脚本

**Learning Policy**:
根据 Learner Model、Curriculum State 和当前情境选择教学动作的规则集合。
_Avoid_: 固定课程脚本、模型提示词

**Learning Pattern**:
由多个独立学习情境共同支持的跨概念或跨课程认知模式，描述用户通常如何形成、修正或迁移理解。
_Avoid_: 单次表现、学习风格标签、人格画像

**Strategy Evidence**:
一次教学策略在明确情境下与后续认知变化之间的可追溯观察，用于支持但不直接证明策略有效性。
_Avoid_: 策略评分、因果结论、偏好声明

**Next Best Learning Action**:
Learning Policy 当前推荐的单个教学动作，必须带有选择理由并可追溯到学习证据或课程约束。
_Avoid_: 完整教案、任意下一步、推荐内容列表

**Detour**:
为修复先修缺口或探索问题而暂离 Curriculum 主线的显式状态，包含返回点。
_Avoid_: 跳课、切换教材

**Review**:
基于历史证据重新检索、验证或修复某个概念的学习活动。
_Avoid_: 重读教材

**Agent Adapter**:
让特定 AI Agent 发现并遵循同一套 Learning OS 操作规则的薄入口文件。
_Avoid_: 独立规则副本
