# Curriculum Builder

A curriculum is either `source_aligned` (it follows one material: the book decides *what* is learned, the learner model decides *how*) or `synthesized` (you organized it from a learning goal: prerequisites decide the order, and the learner model may reshape it — always with the learner's consent).

## From a learning goal

Use this when the learner wants to learn a direction and has no textbook ("我想搞懂向量数据库").

1. Ask about their background and what they want to be able to do; one or two questions, not a survey. Check `learn state --json` for concepts they already hold.
2. Research the field before drafting: prefer official documentation, original papers, well-known textbooks and courses, and the authors' own writing; blog posts only to fill gaps. Use web search if you have it; otherwise rely on material the learner provides, and say so.
3. `learn curriculum import --goal "<goal in the learner's words>" --id <id> --title "<title>" --dry-run --json`, then with `--yes --activate`.
4. Submit the outline with `learn curriculum outline set`: every entry has `why` (why this is learned, ≤120 characters), `prerequisites` (entry ids to finish first) and `concepts` (expected concept ids, reusing existing ids from `learn state --json` where they match). Order entries so prerequisites come first. Keep it small: 4–10 leaf entries for a first version.
5. Show the learner the outline with the why of each part; confirm only after they agree (`learn curriculum outline confirm`).
6. Attach sources to every leaf entry: `learn source add <url|file> --id <id>` (a web page is snapshotted; see [curriculum import](curriculum-import.md)), then `learn source attach <node> <source> <kind> <value>`. `learn source check --json` must report no `unsourced` entries before you rely on them.
7. When `learn next` says `unsourced: true`, tell the learner in one sentence that this part has no source yet and your explanation is a general account; do not submit `textbook_points` there (they are refused).

## A goal and a textbook together

When the learner gives both a goal and a material ("我想能做数据库选型，手上有 DDIA"):

- If the material, read in its own order, already serves the goal, import it as usual (source-aligned). Use the goal to decide what to emphasize and which parts to propose skipping (`skip` proposals, with the learner's words as evidence).
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
