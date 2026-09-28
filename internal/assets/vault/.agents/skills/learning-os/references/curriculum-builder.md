# Curriculum Builder

A curriculum is either `source_aligned` (it follows one material: the book decides *what* is learned, the learner model decides *how*) or `synthesized` (you organized it from a learning goal: prerequisites decide the order, and the learner model may reshape it — always with the learner's consent).

## From a learning goal

Use this when the learner wants to learn a direction and has no textbook ("我想搞懂向量数据库").

1. `learn curriculum import --goal "<goal in the learner's words>" --id <id> --title "<title>" --dry-run --json`, then with `--yes --activate`.
2. **Intake** — one baseline session before any outline ([baseline workflow](baseline-workflow.md)):
   - Interview (3–5 questions, one at a time): what they want to be able to do, where they will use it, what they already know, how much time and how deep, how they will know they have learned it, and the concrete points they care about.
   - Diagnose (2–5 scenario questions) the prerequisites and key concepts the goal implies. Tag findings with `concepts` ids.
   - End with an assessment that carries `goal_card` (see [assessment schema](assessment-schema.md)). If the learner refuses a baseline, still ask the interview questions in a lesson session and store the card with `learn curriculum goal set --file -`. The outline is refused until a goal card exists.
3. **Research** from the goal card: turn `context` and each `focus` into questions, and search for sources that answer them — official documentation, original papers, well-known textbooks and courses, the authors' own writing; prefer sources for the learner's own stack (an Elasticsearch user gets Elastic's docs). Use web search if you have it; otherwise say so and rely on what the learner provides.
4. **Draft** with `learn curriculum outline set`: every entry has `why` (≤120 characters), `prerequisites`, `concepts` (reusing ids from the intake and `learn state --json`) and `serves` (the goal focus ids it serves). Leave out or mark what the intake showed the learner already knows. Keep it small: 4–10 leaf entries.
5. **Review** with `learn curriculum outline review --json`: walk the learner through each entry in one line (why, and what they already know there), ask them to add, drop or reorder, and resubmit the draft after changes. `ready_to_confirm` must be true: every leaf has a `why` and every focus point is served.
6. Confirm only after the learner agrees (`learn curriculum outline confirm`).
7. **Attach sources** to every leaf: `learn source add <url|file> --id <id>`, then `learn source attach <node> <source> <kind> <value> --why "<why this source>" --serves <focus-id>`. `learn source check --json` must report no `unsourced` entries before you rely on them; when nothing suitable exists, say so instead of attaching an unrelated source.
8. When `learn next` says `unsourced: true`, tell the learner in one sentence that this part has no source yet and your explanation is a general account; do not submit `textbook_points` there (they are refused).

## A goal and a textbook together

When the learner gives both a goal and a material ("我想能做数据库选型，手上有 DDIA"), run the intake first (interview and diagnosis, ending with a `goal_card`) and add `concepts` to the book's outline entries so the intake evidence reaches them. Then:

- If the material, read in its own order, already serves the goal, import it as usual (source-aligned). Use the goal to decide what to emphasize; `learn curriculum outline review --json` lists `likely_known` entries — propose `skip` or `mark_known` for them (and for parts the goal does not need), with the learner's words as evidence, and let the learner decide.
- Otherwise build a goal curriculum and make the material its main resource: `learn source add <material> --id <id>`, then attach the relevant chapters or pages to each entry. Find other sources only for entries the material does not cover, and tell the learner which entries those are.

Say in one sentence which of the two you chose and why.

## Changing a curriculum

Never edit a confirmed outline behind the learner's back. When the learner's words show the curriculum should change, put a proposal in the record, with their words as evidence:

```json
"curriculum_proposals": [{"id": "known-cosine", "action": "mark_known", "node": "1",
  "reason": "学习者在工作中已熟练使用余弦相似度", "evidence": [{"turn": "t0004", "quote": "余弦相似度很熟了"}]}]
```

| action | when | allowed in |
| --- | --- | --- |
| `skip` | the learner does not want this part | both types |
| `mark_known` | the learner already masters it (evidence, ideally a quick check) | both types |
| `insert` | a missing topic the learner needs; give a free `node` id (e.g. `2.3.1`), `title`, `why`, `prerequisites` | synthesized |
| `remove` | a topic that turned out irrelevant (not completed, no sub-entries) | synthesized |
| `retitle` | the title misleads | synthesized |

Then ask the learner plainly whether to apply it. Only after they agree run `learn curriculum accept <id>`; if they decline, `learn curriculum reject <id> --reason "<why>"`. `learn next` lists undecided `pending_proposals`; raise them at a natural pause, not mid-explanation. Earlier outlines stay in `learn curriculum outline history`.

## Already known elsewhere

When `learn next` returns `quick_check`, the entry's concepts are already stable from another curriculum. Ask one or two questions that need the concept, not its definition. If the learner answers well, propose `mark_known` with that evidence; if not, teach the entry normally and downgrade the concept as the interpretation rules require.
