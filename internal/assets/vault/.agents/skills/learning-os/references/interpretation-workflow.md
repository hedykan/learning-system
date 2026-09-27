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

## Writing a good record

1. Cite learner turns only. Quote 4–200 characters copied from the turn text; whitespace differences are tolerated, paraphrase is not.
2. Keep claims small. One event per observable behaviour. Do not infer stable understanding from "I get it"; that is `recognized` at most.
3. Leave out `source_ref` for concepts from the current outline node; the Runtime anchors them to it. Give an explicit `source_ref` only for a concept from another part of the book. Reuse concept IDs. Check `learn state --json` first; a new concept whose label or alias collides with an existing one is rejected.
4. Name the old model in the learner's terms and the new model as the learner now states it. The trigger is what you did in between (`trigger_turn` is your assistant turn).
5. When you corrected an error during teaching, record it: a `misconception` event for the wrong idea and a `correction` event (or a cognitive change) for the fix. Do not reduce a corrected mistake to an `application` event.
6. Judge strategies honestly. `effective` must link the change or positive event it produced; otherwise use `inconclusive` or `ineffective` with a reason.
7. Link concepts that the book or the learner's reasoning actually connects (prerequisite, contrast, part of, same mechanism) with `related`. Two or three meaningful links per concept are better than many weak ones; they become the Obsidian graph.
8. Record a learning pattern only when the same way of learning appears again. Add `contradicts` observations when the learner breaks the pattern; they matter as much as support.
9. When an earlier judgment was wrong, add a `retraction` with its global ID (`<session-id>:<local-id>`) and a reason.

## Backfilling older sessions

Sessions recorded before v0.1.2 have turn IDs derived from order. Use `learn session turns --session <id> --json`, then `learn session annotate <id> --analysis-file -` to add a record without reopening the session.

## After submitting

Run `learn next --json`. Follow its action unless the learner's current turn makes it clearly wrong; if you depart from it, the next record should show why.
