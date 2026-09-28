# Interpretation Record 与 Learning Policy 规范

**状态：** frozen（v0.1.2）  
**冻结日期：** 2026-09-24  
**来源：** [CR-2026-004](./changes/CR-2026-004-evidence-derived-learner-model.md)

本文件冻结 v0.1.2 的数据格式、校验规则、派生规则和 Learning Policy 规则。产品原则见 [Learner Model 与知识沉淀](./learner-model.md)，命令见 [CLI 契约](./cli.md)。

## 1. 分层与存储

```text
Conversations/<session>.md                      Raw，append-only，事实
.learning/interpretations/<session>/NNNN.json   Interpretation Record，append-only，Git 跟踪
.learning/model/learner-model.json              派生缓存，可删除重建，Git 忽略
Concepts/ Questions/ Sessions/ Profile/ Curriculum/<id>/<书名>.md   Markdown 投影，可重建，Git 跟踪；文件名使用学习者语言（CR-2026-022）
Curriculum/<id>/detours.yaml                    Curriculum State 的 Detour 日志，append-only
```

原则：**Model can change. History cannot.** Learner Model 永远等于“按时间回放全部已接受 Interpretation Record”的结果。修正旧判断只能追加新记录或 retraction，不能改写旧记录。

`.learning/state.json` 保持 schema 1；v0.1.1 Vault 无需迁移，新目录按需创建。

## 2. Turn ID

- Conversation 中每个 `## ` 条目按出现顺序编号，从 `t0001` 开始。因为 Conversation append-only，序号即稳定 ID。
- v0.1.2 起新条目格式：

  ```markdown
  ## t0012 — 2026-09-24T09:05:06.123Z — user

  > 原文

  ^t0012
  ```

  `^t0012` 是 Obsidian 块引用锚点，投影用 `[[Conversations/<session>#^t0012]]` 链接证据。
- 旧格式 `## <timestamp> — <role>` 仍可解析，按序号推导 turn ID，但没有块锚点，投影只链接到文件。
- 解析时若新格式条目的显式 ID 与序号不一致，Conversation 视为损坏，所有写命令拒绝执行。

## 3. Evidence Reference

```json
{"turn": "t0012", "quote": "不能只看平均值"}
```

- 同一记录内 `turn` 默认指当前 Session；跨 Session 引用写作 `session-20260924-090212.000000000#t0012`。
- 校验：turn 存在；角色为 `user`；`quote` 去除首尾空白并折叠连续空白后，是该 turn 原文的子串；长度 4 至 200 个字符。
- 助手或系统 turn 只能作为 `trigger_turn` 或 `action_turn`，不能作为学习者能力证据。

## 4. Interpretation Record Schema

Agent 通过 `session checkpoint`、`session end` 或 `session annotate` 提交。未知字段一律拒绝。

```json
{
  "schema": "learning-os/interpretation@1",
  "curriculum": "ddia-2e-zh",
  "concepts": [
    {"id": "tail-latency", "label": "尾延迟", "aliases": ["p99 延迟"],
     "source_ref": {"chapter": "1", "section": "描述性能"}}
  ],
  "events": [
    {"id": "e1", "type": "prediction", "concept": "tail-latency",
     "summary": "预测平均延迟更低的系统更能扛住流量",
     "evidence": [{"turn": "t0010", "quote": "B 平均更快"}]}
  ],
  "cognitive_changes": [
    {"id": "c1", "concept": "tail-latency",
     "old_model": "平均延迟低即性能好",
     "trigger": "1% 请求等待 8 秒意味着每分钟数千次慢请求",
     "trigger_turn": "t0011",
     "new_model": "需要同时观察 p99 等尾部指标",
     "old_evidence": [{"turn": "t0010", "quote": "B 平均更快"}],
     "new_evidence": [{"turn": "t0012", "quote": "不能只看平均值"}]}
  ],
  "state_updates": [
    {"id": "u1", "concept": "tail-latency", "state": "developing",
     "capabilities": ["explained"],
     "summary": "能解释为什么平均值会掩盖慢请求，尚无迁移证据",
     "evidence": [{"turn": "t0012", "quote": "不能只看平均值"}],
     "open_questions": ["p99 在分片系统中如何放大"]}
  ],
  "strategy_attempts": [
    {"id": "s1", "strategy": "counterexample", "situation": "misconception",
     "concept": "tail-latency", "action_turn": "t0011",
     "expected_change": "学习者放弃只看平均值",
     "outcome": "effective", "linked": ["c1"],
     "evidence": [{"turn": "t0012", "quote": "不能只看平均值"}]}
  ],
  "pattern_observations": [
    {"id": "p1", "pattern": "concrete-before-abstract", "stance": "supports",
     "summary": "具体场景和预测失败后才形成抽象指标",
     "linked": ["c1"],
     "evidence": [{"turn": "t0012", "quote": "不能只看平均值"}]}
  ],
  "retractions": [
    {"ref": "session-20260924-084000.000000000:e3", "reason": "误把复述当成解释"}
  ],
  "progress_decision": {
    "decision": "stay",
    "reason": "尾延迟尚无迁移证据"
  }
}
```

### 4.1 通用规则

- `schema` 必须为 `learning-os/interpretation@1`；`curriculum` 必须等于 Session 的 Curriculum（无 Curriculum 时为空串）。
- event、change、state update、strategy attempt、pattern observation 都必须带局部 `id`，匹配 `^[a-z][a-z0-9-]{0,31}$`，在同一 Session 的全部记录内唯一；全局引用为 `<session-id>:<local-id>`。重复提交 ID 与内容都相同的条目会被跳过，ID 相同内容不同则拒绝。
- `linked` 只能引用同一 Session 内已存在或本记录中的 event、change。
- 所有数组可省略；整条记录至少包含一个 event、change、state update、strategy attempt、pattern observation、retraction 或 progress decision。
- 任意一项校验失败，整条记录拒绝，已有记录、模型和投影均不变。

### 4.2 Concept

- `id` 为全局 kebab-case slug，跨 Curriculum 共享；首次出现必须在 `concepts` 声明 `label`。
- 新声明的 `label` 或 `aliases` 与其他已有 Concept 的 label/alias 大小写不敏感重名时拒绝，要求复用已有 ID，防止重复概念。
- 已存在 Concept 可再次声明以追加 alias 或 `source_ref`，不能修改 label。
- v0.1.6：记录可带 `questions`（全局 kebab-case ID、4–120 字、可选已有 `concept` 与已确认目录中的 `node`、必须有学习者原话）与 `question_resolutions`（每个问题只能解决一次）。`learn next` 输出最多 3 个 `open_questions`（CR-2026-020）。
- v0.1.6：提交时（不含回放），若一个应用前为 `stable` 的 Concept 在本记录中出现 `misconception` 事件或 `partial`、`forgotten` 复习结果，且应用后仍为 `stable`，记录被拒收（CR-2026-016）。
- v0.1.6：Learning Pattern 在支持多于反例，且“两本教材、至少两个 Session”或“至少三个 Session”支持时为 `supported`（CR-2026-018）。
- v0.1.5：Concept 可带 `related`（无向关系，说明最多 40 字，不能指向自身）；记录可带 `review_results`（`recalled`、`partial`、`forgotten`，出题轮次为助手轮次，证据在其后）。排期由记录推导：间隔 1、2、4、7、15、30、60 天，首次 developing 或 stable 进入第 0 档，想起升档、部分想起保持、忘记或变为 fragile 回到第 0 档（CR-2026-012、CR-2026-013）。
- v0.1.4：Concept 可带 `textbook_points`（`pages: [start, end]`，1–5 条、每条 4–120 字符）。新版本取代当前版本，旧版本保留为历史；与当前版本完全相同时不追加。checkpoint 与 end 提交时，若目录已确认且该 Concept 的目录条目有页码，要点页码必须落在该范围内。只含带要点的 Concept 的记录不算空记录（CR-2026-011）。
- v0.1.3：`source_ref` 可带 `node`。checkpoint 与 end 提交时，新 Concept 若省略 `source_ref`，Runtime 以当前位置补齐；`source_ref` 与当前章节相同但缺 `node` 时补上 node。Detour 期间与 annotate 不做补齐。补齐发生在计算记录哈希之前。

### 4.3 Learning Event

`type` 允许值：

```text
question prediction attempt misconception correction insight
understanding_revision prerequisite_gap retrieval application
transfer connection learner_proposed_method
```

- 每个 event 至少一条 learner evidence。
- `strategy_switch` 不由 Agent 直接提交，由 Runtime 从 strategy attempt 的 `replaces` 派生（见 4.6）。
- v0.1.1 的 `policy_failure` 由 `outcome: ineffective` 的 strategy attempt 取代，只在读取旧 Session 时兼容显示。

### 4.4 Cognitive Change

- `old_evidence` 和 `new_evidence` 各至少一条；`new_evidence` 的 turn 必须晚于全部 `old_evidence`。
- `trigger_turn` 必须位于两者之间，可以是助手 turn。
- 自动派生一个 `understanding_revision` 事件用于索引，不需要重复提交。

### 4.5 State Update

`state` 允许值：`unobserved`、`developing`、`fragile`、`stable`。  
`capabilities` 允许值：`recognized`、`explained`、`predicted`、`retrieved`、`applied`、`transferred`。

| 目标状态 | Runtime 最低证据要求 |
| --- | --- |
| `developing` | 至少一个 `explained`、`predicted` 或 `applied`；仅 `recognized` 不够 |
| `fragile` | 同一记录或此前记录中该 Concept 有未撤回的 misconception 事件或 `ineffective` 的 strategy attempt（失败的 retrieval 以 situation 为 `retrieval` 的 ineffective attempt 记录） |
| `stable` | 该 Concept 已有 `developing` 历史，且本次有 `retrieved` 或 `transferred` 证据，证据所在 Session 晚于该 Concept 首次 `explained` 证据所在 Session |
| `unobserved` | 只能由 retraction 回退产生，不能直接提交 |

- 状态可以下降，例如 `stable` 到 `fragile`；旧状态保留在 Cognitive History。
- “我懂了”一类自我声明只能标记 `recognized`。
- v0.1.1 的 `solid` 从未持久化到状态文件，只存在于 Skill 文案。v0.1.2 不接受 `solid`，Skill 文案统一改为 `stable`。

### 4.6 Strategy Attempt

`strategy` 允许值：

```text
concrete_example analogy diagram counterexample prediction_probe
learner_action direct_explanation prerequisite_repair retrieval_practice
```

`situation` 允许值：`new_concept`、`misconception`、`prerequisite_gap`、`retrieval`、`transfer`。  
`outcome` 允许值：`effective`、`inconclusive`、`ineffective`。

- `action_turn` 必须是助手 turn；`evidence` 的 turn 必须晚于 `action_turn`。
- `effective` 必须 `linked` 到至少一个 cognitive change，或 insight、correction、understanding_revision、retrieval、application、transfer 事件。
- `inconclusive` 与 `ineffective` 必须提供 `reason`。
- 可选 `replaces`（前一 attempt 的局部或全局 ID）与 `switch_reason`；存在时 Runtime 派生 `strategy_switch` 事件，连接前后策略与后续认知变化。
- 结果是可修正观察，不代表因果证明。

### 4.7 Pattern Observation

- `pattern` 为 kebab-case slug。首次出现时必须提供 `description`，可选 `preferred_strategy`（取 4.6 的 strategy 值）与 `situation`（为空表示适用于所有情境）；之后的观察不能修改这些定义。
- `stance`：`supports` 或 `contradicts`；两者都必须带 learner evidence，并至少 `linked` 一个 change 或 event。
- 派生状态：
  - `candidate`：默认。
  - `supported`：supports 来自至少 2 个 Session 且至少 2 个 Curriculum，并且 supports 多于 contradicts。
  - `contested`：contradicts 数量不少于 supports。
- 反例永久保留并在投影中展示。只有 `supported` 模式参与 Learning Policy。

### 4.8 Retraction

- `ref` 指向任意已接受的 event、change、state update 或 attempt 全局 ID，必须附 `reason`。
- 被撤回项标记为 `superseded`，从派生模型中排除，但保留在历史和投影中。
- State update 被撤回后，该 Concept 状态回退到它之前最近一次未撤回的状态；没有则为 `unobserved`。

### 4.9 Progress Decision

- 仅 `session end` 的记录允许包含；`decision` 取 `stay`、`advance`、`detour`，必须有 `reason`。
- 它只记录判断，不修改 Curriculum Position；位置仍由 `curriculum position set` 或 `detour` 命令显式修改。
- Session 结束不自动产生任何 Concept 状态变化。

## 5. 幂等与原子性

- 记录规范化（字段排序、去空白）后计算 SHA-256。与该 Session 已有记录相同则返回 `unchanged`，不写任何文件。
- 相同局部 ID 但内容不同，拒绝。
- 写入顺序：校验，原子写入记录，回放生成 `learner-model.json`，重写受影响投影。
- checkpoint 与 annotate 在记录写入后若派生失败，命令返回非零；记录保留，`learn model rebuild` 可恢复。`session end` 此时已完成结束，改为在结果的 `projections` 字段报告失败。
- 投影 frontmatter 带 `model_generation`（全部记录哈希的摘要）；`learn status --json` 在投影落后时报告 `projections: stale`。

## 6. Detour

`Curriculum/<id>/current-position.md` 中的 `detour` 扩展为：

```yaml
detour:
  id: d1
  type: prerequisite
  topic: 数据库事务
  reason: 学习者不清楚锁与事务的边界
  return_condition: 能解释两笔并发扣减为何需要同一事务
  return_to: {chapter: "1", section: "描述负载", concept: "吞吐量"}
  started_session: session-...
```

- `detour start` 以当前位置作为 `return_to`；已有未结束 Detour 时拒绝。
- `detour end` 必须给出 `outcome`（`completed` 或 `abandoned`）与 `learned`，把位置恢复到 `return_to`，并向 `detours.yaml` 追加完整条目。
- Baseline Session 期间两者都被拒绝。

## 7. Learning Policy（`learn next`）

Runtime 以确定性规则计算单个 Next Best Learning Action，Agent 在 Session 开始和每次 checkpoint 后读取。

### 7.1 输出

```json
{
  "action": "transfer_probe",
  "concept": "tail-latency",
  "strategy": "concrete_example",
  "curriculum_relation": "mainline",
  "rule": "R4-missing-transfer",
  "reason": "能解释尾延迟，但尚无迁移证据",
  "evidence": ["session-...:c1", "session-...:s1"],
  "history_used": true
}
```

`action` 允许值：`return_to_mainline`、`repair_misconception`、`retrieval_probe`、`transfer_probe`、`application_probe`、`explain_probe`、`continue_curriculum`、`baseline`。  
`curriculum_relation` 允许值：`mainline`、`detour`、`return`。

### 7.2 规则（按顺序命中第一条）

| 规则 | 条件 | 动作 |
| --- | --- | --- |
| R0 | 当前 Curriculum 无 Assessment，且没有任何 Session 以 `--skip-baseline` 显式跳过 | `baseline` |
| R1 | 存在未结束 Detour，且 Detour 主题 Concept 已达 `developing` 以上 | `return_to_mainline`，relation `return` |
| R2 | 当前章节范围内有未被 correction 或 change 解决的 misconception | `repair_misconception` |
| R2b | 存在开放的学习者问题，其 `node` 等于当前位置，或其 `concept` 为当前概念（v0.1.6，CR-2026-020） | `address_question` |
| R3 | 当前章节范围内有 `fragile` Concept | `retrieval_probe` |
| R3b | 当前教材有按间隔复习计划到期（到期日不晚于今天）的 Concept，且本 Session 尚未复习它（v0.1.5，CR-2026-012） | `review_due` |
| R4 | 当前 Concept 为 `developing`，有 explained 但无 applied 或 transferred | `transfer_probe`；无 applied 时 `application_probe` |
| R4b | 当前章节范围内有 `developing` Concept，已有 applied 或 transferred 证据，最近一次状态来自更早的 Session，且当前 Session 尚无针对它的 retrieval 尝试或事件（v0.1.3，CR-2026-008） | `retrieval_probe` |
| R5 | 当前 Concept 为 `unobserved` 且位置已设置 | `explain_probe` |
| R6 | 其他 | `continue_curriculum`；有目录时附带下一个条目的 `node` 与 `node_title`：优先返回当前位置之前“学过一部分”的条目（v0.1.4），否则为之后第一个未完成条目（v0.1.3） |

- “当前章节范围”：位置有目录 node 时，指 `source_ref.node` 以当前章节编号为前缀的 Concept；否则按章节标题完全相同或首个词相同匹配（v0.1.3）。Detour 中为 Detour 主题 Concept。
- Policy 不产生跨章节跳转；relation 只能是 `mainline`、`detour` 或 `return`。

### 7.3 策略选择

1. 若存在 `preferred_strategy` 不为空的 `supported` Pattern，且其 situation 与动作匹配，选用它。
2. 否则按该 situation 下全部 Curriculum 的 Strategy Evidence 打分：`effective` 加 1，`ineffective` 减 1，`inconclusive` 为 0，取最高分；平分时取最近一次 effective 的策略。
3. 同一 Concept 上最近连续 2 次 `ineffective` 的策略被排除。
4. 无任何证据时使用默认表：`new_concept` 用 `prediction_probe`，`misconception` 用 `counterexample`，`prerequisite_gap` 用 `direct_explanation`，`retrieval` 用 `retrieval_practice`，`transfer` 用 `concrete_example`。
5. `history_used` 为真，当且仅当动作或策略的选择引用了当前 Session 之外的证据；`evidence` 列出这些全局 ID。

## 8. 投影

| 文件 | 内容 |
| --- | --- |
| `Concepts/<概念名称>.md` | 教材来源、学习者原话（带证据链接）、当前解释（标注为 AI）、状态与能力证据、误解与修正、开放问题、修订历史、用户笔记区 |
| `Sessions/<session>.md` | 起点、事件、认知变化、受影响 Concept、策略尝试、进度决策、`learn next` 结果 |
| `Profile/学习者总览.md` | 各 Concept 状态、待验证判断、有效与无效策略、Learning Pattern（含反例）、推荐下一步 |
| `Curriculum/<id>/<书名>.md` | 教材位置、Detour 历史、本教材 Concept 的状态，两者并列不互相替代 |

- v0.1.2 不生成独立的 Questions、Hypotheses、Misconceptions、Insights 文件；这些内容保留在 Concept 和 Session 投影中。
- 用户手写区：`<!-- learn:user:begin -->` 与 `<!-- learn:user:end -->` 之间的内容在重建和升级时逐字保留；文件缺少标记时，Runtime 追加空白用户区而不删除任何已有文本。
- 投影使用相对路径和 Obsidian wikilink；AI 解释统一以第三人称书写，不以学习者口吻写“我的理解”。

## 9. 回放验收夹具

`internal/replay/replay_test.go` 以代码脚本构造 12 个 Session，覆盖两个 Curriculum（DDIA 与一本数学教材），每个 Session 都通过真实的 start、append、checkpoint、end 流程写入 Vault。验收测试：

1. **对照组**：只加载第 12 个 Session 的当前情境，`learn next` 选择默认规则与默认策略，`history_used: false`。
2. **实验组**：按顺序回放 12 个 Session 后在同一情境调用 `learn next`，动作或策略与对照组不同，`history_used: true`，且 `evidence` 至少引用 2 个更早 Session 的 ID。
3. 实验组的 `curriculum_relation` 不越出当前章节或已记录 Detour。
4. 夹具中包含至少一个跨两门教材的 `supported` Pattern、一次 `strategy_switch` 和一次 `stable` 被反例降为 `fragile`。
