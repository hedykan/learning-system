# 核心验证场景

**产品地位：** core  
**确立日期：** 2026-09-28

这 17 个场景是 Personal Learning OS 的核心验证方向。每个版本发布前，都要用真实 Agent（当前为 Codex，gpt-5.6-sol，低思考）在一本教材上逐条复现，并核对记录、笔记与 `learn next` 输出。一个场景只有在“对话里识别了”并且“记录里存对了”时才算通过。

## 场景清单

| # | 场景 | 典型复现方式 | 系统必须识别或做到 | 主要落点 |
| --- | --- | --- | --- | --- |
| 1 | Misconception | 学习者对概念给出看似合理但错误的解释 | 识别误解并引用原话，不能只标“答错” | `misconception` 事件 |
| 2 | Socratic Cognitive Change | AI 不直接给答案，用连续问题让学习者自己修正 | 保存 Old Model → Trigger → New Model | `cognitive_changes` |
| 3 | Insight | 学习者自己说出关键区别或规律 | 识别 insight，并保存触发它的上下文 | `insight` 事件 |
| 4 | Understanding Revision | 学习者明确从旧模型改成新模型 | 保存前后理解，而不是只存最终正确答案 | 认知变化派生的 `understanding_revision` |
| 5 | Learner Proposed Method | 学习者自己提出解决方法或推导路径 | 标记为学习者产生的方法，而不是 AI 知识 | `learner_proposed_method` 事件 |
| 6 | Concept Connection | 学习者主动把当前知识和以前的概念连起来 | 建立真实的学习者连接 | `connection` 事件与 `related` |
| 7 | Spontaneous Retrieval | 没有提醒，学习者自己调用几天前学过的知识 | 记录 retrieval，并引用旧知识与当前使用场景 | 引用跨 Session 原话的 `retrieval` 事件 |
| 8 | False Understanding | 学习者说“懂了”，换一道变式马上不会 | 不能因为“懂了”就判 stable，要发现仍是 fragile | 状态门槛与 `recognized` |
| 9 | Policy Failure → Strategy Switch | 抽象解释两次仍不懂，换具体案例或反例后理解 | 识别原策略无效、策略切换与新策略效果 | `strategy_attempts` 与派生的 `strategy_switch` |
| 10 | Prerequisite Gap | 当前问题卡住，真正原因是先修知识缺失 | 找到最小先修知识，临时补完后返回原位置 | `prerequisite_gap` 事件与 Detour |
| 11 | Application | 学完概念后在相似问题中使用 | 记录 application，不能误判为 transfer | `application` 事件与 `applied` |
| 12 | Strong Transfer | 学习者主动把知识用于明显不同的新场景 | 记录 transfer，作为较强的理解证据 | `transfer` 事件与 `transferred` |
| 13 | Learner-generated Key Question | 学习者提出一个改变学习方向的关键问题 | 问题成为一等学习对象，并进入后续教学 | Question 对象（v0.1.6） |
| 14 | Curriculum Detour & Return | 为补先修知识暂时离开教材 | 保存绕行原因与返回点，补完必须回到主线 | Detour 与位置 |
| 15 | Learner Model Revision | 之前判为 stable，新证据表明其实不会 | stable → fragile，不篡改旧判断和旧证据 | 状态历史 |
| 16 | Cross-session Personal Pattern | 多个 Session 都显示“抽象不懂 → 具体案例后理解” | 从概念状态上升为长期学习模式 | Learning Pattern |
| 17 | Policy Personalization | 历史表明某种教学方式对该学习者有效 | 后续主动选择该策略，并说明为什么 | `learn next` 的策略与证据 |

## 验收方法

1. 用一本能完整学完的教材（推荐 Markdown，避免 PDF 读取干扰），在多个模拟日期下（`LEARN_NOW`）学完全书。
2. 由测试者扮演学习者，按场景设计回答，每个场景至少触发一次。
3. 学完后从派生的 Learner Model 汇总事件、认知变化、策略、复习、模式与状态历史，逐条判定通过、部分通过或未通过，并附证据。
4. 同时统计学习者可见话术：进度提示比例、提示与正文重复比例。

## 实测记录

### v0.1.5（2026-09-28，《HTTP 缓存入门》，41 轮，全书 8/8 完成）

| 结果 | 场景 |
| --- | --- |
| 通过 | 1、2、3、4、6、8、9、10、12、17 |
| 部分通过 | 7（被记成 connection，未引用旧 Session 原话）、11（2 次同类题误标为 transfer）、14（返回点错一节）、16（只记 1 次，停在 candidate） |
| 未通过 | 5（学习者推导的方法被记成 insight）、13（关键问题没有任何记录）、15（stable 概念复发误解后仍为 stable） |

话术：进度提示 68%，提示与正文句子重复 10%，提示与正文开头重复评价 7%。

对应修复见 [v0.1.6 基线](./versions/v0.1.6.md)。

### v0.1.6 复验（2026-09-28，接续上面的 Vault，另加《CDN 入门》，9 轮）

只复验 v0.1.5 未完全通过的场景：

| # | 结果 | 证据 |
| --- | --- | --- |
| 5 | 通过 | 学习者自己提出“先问香港节点再回源”，记为 `learner_proposed_method` |
| 7 | 未通过 | 学习者主动调出上一本书的强缓存知识，被记为 `insight`，证据只引用当前轮次 |
| 11 | 通过 | 同类路径题记为 `application` |
| 13 | 通过 | 问题存为一等对象并挂到 1.2，生成独立笔记，学完 1.2 后标为已解决 |
| 14 | 通过 | 完成 1.1 后位置自动推进到 1.2；全书学完后停在最后一节 |
| 15 | 通过 | stable 的强缓存复发误解后降为 fragile，历史保留 stable 条目；Agent 自行降级，兜底规则未被触发 |
| 16 | 通过 | “先具体后抽象”在两本教材、两个 Session 中得到支持，状态为 supported |

话术：提示与正文句子重复 0%，重复评价 0%；但每轮都以固定的“我看一下。”开头（进度提示 100%）。

遗留：场景 7 需要程序侧的检索提示，见 IDEA-024。
