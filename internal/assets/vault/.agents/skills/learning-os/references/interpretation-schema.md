# Interpretation schema

All arrays are optional, but a record must contain at least one item. Unknown fields are rejected.

```json
{
  "schema": "learning-os/interpretation@1",
  "curriculum": "<active curriculum id>",
  "concepts": [
    {"id": "tail-latency", "label": "尾延迟", "aliases": ["p99 延迟"]},
    {"id": "replication-lag", "label": "复制延迟", "source_ref": {"node": "6.2"},
     "textbook_points": {"pages": [210, 212], "points": ["follower 异步应用 leader 的变更，因此读到的可能是旧值", "滞后通常很短，但在负载或故障时可能达到数秒甚至数分钟"]},
     "related": [{"concept": "tail-latency", "type": "contrast", "note": "都关乎读到结果的时效"}]}
  ],
  "events": [
    {"id": "e1", "type": "prediction", "concept": "tail-latency", "summary": "...", "evidence": [{"turn": "t0010", "quote": "exact learner words"}]}
  ],
  "cognitive_changes": [
    {"id": "c1", "concept": "tail-latency", "old_model": "...", "trigger": "...", "trigger_turn": "t0011", "new_model": "...",
     "old_evidence": [{"turn": "t0010", "quote": "..."}], "new_evidence": [{"turn": "t0012", "quote": "..."}]}
  ],
  "state_updates": [
    {"id": "u1", "concept": "tail-latency", "state": "developing", "capabilities": ["explained"], "summary": "...",
     "evidence": [{"turn": "t0012", "quote": "..."}], "open_questions": ["..."]}
  ],
  "strategy_attempts": [
    {"id": "s1", "strategy": "counterexample", "situation": "misconception", "concept": "tail-latency", "action_turn": "t0011",
     "expected_change": "...", "outcome": "effective", "linked": ["c1"], "evidence": [{"turn": "t0012", "quote": "..."}],
     "replaces": "s0", "switch_reason": "..."}
  ],
  "pattern_observations": [
    {"id": "p1", "pattern": "concrete-before-abstract", "description": "first time only", "preferred_strategy": "concrete_example",
     "situation": "misconception", "stance": "supports", "summary": "...", "linked": ["c1"], "evidence": [{"turn": "t0012", "quote": "..."}]}
  ],
  "review_results": [
    {"id": "r1", "concept": "tail-latency", "outcome": "recalled", "action_turn": "t0020", "evidence": [{"turn": "t0021", "quote": "..."}]}
  ],
  "no_related": ["concept-id"],
  "questions": [
    {"id": "why-ask-if-unchanged", "question": "内容没变为什么还要问一次", "concept": "negotiated-cache", "node": "3.2", "evidence": [{"turn": "t0027", "quote": "..."}]}
  ],
  "question_resolutions": [
    {"question": "why-ask-if-unchanged", "summary": "...", "evidence": [{"turn": "t0040", "quote": "..."}]}
  ],
  "retractions": [{"ref": "session-...:e3", "reason": "..."}],
  "progress_decision": {"decision": "stay", "reason": "..."}
}
```

## Values

| Field | Allowed values |
| --- | --- |
| event `type` | question, prediction, attempt, misconception, correction, insight, understanding_revision, prerequisite_gap, retrieval, application, transfer, connection, learner_proposed_method |
| `state` | developing, fragile, stable |
| `capabilities` | recognized, explained, predicted, retrieved, applied, transferred |
| `strategy` | concrete_example, analogy, diagram, counterexample, prediction_probe, learner_action, direct_explanation, prerequisite_repair, retrieval_practice |
| `situation` | new_concept, misconception, prerequisite_gap, retrieval, transfer |
| `outcome` | effective, inconclusive, ineffective |
| `decision` | stay, advance, detour (end records only) |

## Rules the Runtime enforces

- Local IDs match `^[a-z][a-z0-9-]{0,31}$` and are unique within the session. Refer to another session's item as `<session-id>:<id>`, and to another session's turn as `<session-id>#t0012`.
- `turn` must be a learner turn; `action_turn` must be an assistant turn in this session; strategy evidence must come after its action turn.
- Cognitive change: every new-evidence turn comes after every old-evidence turn, with `trigger_turn` in between. An `understanding_revision` event is derived automatically.
- `developing` needs explained, predicted, or applied evidence.
- `fragile` needs a misconception event or an ineffective attempt on the concept.
- `stable` needs an earlier developing state plus retrieved or transferred evidence from a later session than the first explanation.
- `strategy_switch` is derived from `replaces`; do not submit it as an event.
- `textbook_points`: 1–5 points of 4–120 characters, with where you read them: `pages: [start, end]` for a PDF, or a `locator` for any material (`{"kind": "anchor", "value": "#replication-lag"}`, `{"kind": "time", "value": "3/05:20-12:00"}`, `{"kind": "page", "value": "42-45"}` for a paper book). Give one of the two, inside the concept's outline entry. A record may carry only concepts with textbook points. Submitting new points replaces the current version; the old one stays in the note's history.
- `related`: other concept IDs (existing or declared in the same record), with an optional `type` and a note of at most 40 characters. Never relate a concept to itself.
  - `prerequisite`: this concept needs the other one first (replication lag → replication).
  - `part_of`: this concept is a part of the other one (ETag → cache validation).
  - `applies_to`: this concept applies the other one (CDN revalidation → conditional requests).
  - `contrast`: the two are easy to confuse (fresh cache hit ↔ revalidation).
  - `related` (default): any other real connection.
  Directed types are declared on the dependent side; the graph draws an arrow from it. Restating a pair in a later record replaces its type, direction and note. Prerequisites must not form a cycle.
- `review_results.outcome`: `recalled`, `partial`, or `forgotten`; `action_turn` is your question in this session and evidence comes after it.
- `no_related`: concepts you checked and found unrelated to every existing concept; the end-of-session check stops asking about them until a relation is added.
- A record may carry only concepts with textbook points or relations, or only `no_related`.
- `questions`: global kebab-case `id`, `question` of 4–120 characters, optional existing `concept`, optional `node` that exists in the confirmed outline, learner evidence required. `question_resolutions` closes a question once, with learner evidence.
- A pattern becomes `supported` with more support than contradiction and either support from two curricula across at least two sessions, or support from at least three sessions.
