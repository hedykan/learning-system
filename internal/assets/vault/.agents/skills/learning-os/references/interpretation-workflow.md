# Interpretation workflow

An Interpretation Record states what the conversation showed about the learner. The Runtime accepts it only if every claim quotes the learner's own turn.

## When to checkpoint

Submit a checkpoint whenever one of these happened since the last one:

- the learner revised a model (old model, trigger, new model);
- a misconception, prerequisite gap, retrieval, application, or transfer became visible;
- a teaching strategy clearly worked, clearly failed, or was replaced;
- the learner showed a way of learning that also appeared in another course.

```bash
printf '%s' "$RECORD_JSON" | learn session checkpoint --analysis-file - --json
```

`accepted` means the record, learner model, and notes were updated. `unchanged` means an identical record already exists. Submit only new items; repeating an item with the same `id` and the same content is harmless, but reusing an `id` for different content is rejected.

## Choosing the event type

Label by what the learner did, not by how impressive it sounded:

| Type | Use when | Not when |
| --- | --- | --- |
| `application` | A new problem of the same type and domain as one already taught (another max-age timing, another hit-rate calculation). | The problem is only reworded or the numbers changed; that is still application, not transfer. |
| `transfer` | The learner maps the idea onto a clearly different domain or problem type on their own (cache staleness → an inventory dashboard synced hourly). | The scenario is the same kind as in the lesson (another stale CSS file). |
| `learner_proposed_method` | The learner invents a solution, derivation, or design before being told, even if it matches the book (“compare a fingerprint of the file”). | The learner restates a method you just explained. |
| `insight` | The learner spontaneously states a distinction or rule that was not just explained (“更快就更可能旧”). | The learner summarizes or paraphrases what you just taught. |
| `retrieval` | Without a prompt, the learner uses knowledge from an earlier session. Cite both the current turn and the turn in the earlier session where it was learned (`<session-id>#tNNNN`, find it with `learn session turns --session <id> --json`). | You asked a review question; record that as a `review_results` item plus retrieval. |
| `connection` | The learner links two concepts they both already know. | The link is really a use of old knowledge in a new problem; that is retrieval or application. |
| `question` in `questions` | The learner asks something that changes or extends the direction of study. See “Key questions”. | A clarification such as “什么意思？” |

A remark about how the learner learns (“我每次要看具体例子才懂”) is a `pattern_observations` item, never an event.

## Key questions

When the learner asks a question that points beyond the current explanation (“既然没变为什么还要问？”), save it as a first-class question:

```json
{"questions": [{"id": "why-ask-if-unchanged", "question": "内容没变为什么还要问一次", "concept": "negotiated-cache", "node": "3.2",
  "evidence": [{"turn": "t0027", "quote": "为什么还非得去问这一次"}]}]}
```

Give `node` when the book answers it in a later section; `learn next` will bring it back there (`address_question`). When the learner can answer it, close it with `question_resolutions` and their own words as evidence. `learn next` lists open questions in `open_questions`; weave them in when relevant.

## Downgrading a stable concept

If a concept is currently `stable` and this record shows a misconception on it or a `partial`/`forgotten` review, add a `state_update` to `fragile` in the same record. The Runtime rejects the record otherwise. The earlier stable judgment stays in history.

## Writing a good record

1. Cite learner turns only. Quote 4–200 characters copied from the turn text; whitespace differences are tolerated, paraphrase is not.
2. Keep claims small. One event per observable behaviour. Do not infer stable understanding from "I get it"; that is `recognized` at most.
3. Leave out `source_ref` for concepts from the current outline node; the Runtime anchors them to it. Give an explicit `source_ref` only for a concept from another part of the book. Reuse concept IDs. Check `learn state --json` first; a new concept whose label or alias collides with an existing one is rejected.
4. Name the old model in the learner's terms and the new model as the learner now states it. The trigger is what you did in between (`trigger_turn` is your assistant turn).
5. When you corrected an error during teaching, record it: a `misconception` event for the wrong idea and a `correction` event (or a cognitive change) for the fix. Do not reduce a corrected mistake to an `application` event.
6. Judge strategies honestly. `effective` must link the change or positive event it produced; otherwise use `inconclusive` or `ineffective` with a reason.
7. Link concepts that the book or the learner's reasoning actually connects with `related`, choosing the most specific `type` (prerequisite, part_of, applies_to, contrast, related). Two or three meaningful links per concept are better than many weak ones; they become the Obsidian graph. Links to concepts of another textbook are the most valuable ones: add them whenever the learner connects the two books.
8. Record a learning pattern only when the same way of learning appears again. Add `contradicts` observations when the learner breaks the pattern; they matter as much as support.
9. When an earlier judgment was wrong, add a `retraction` with its global ID (`<session-id>:<local-id>`) and a reason.

## Backfilling older sessions

Sessions recorded before v0.1.2 have turn IDs derived from order. Use `learn session turns --session <id> --json`, then `learn session annotate <id> --analysis-file -` to add a record without reopening the session.

## After submitting

Run `learn next --json`. Follow its action unless the learner's current turn makes it clearly wrong; if you depart from it, the next record should show why.
