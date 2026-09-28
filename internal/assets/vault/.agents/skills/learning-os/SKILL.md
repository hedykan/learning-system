---
name: learning-os
description: Operate a Personal Learning OS Vault when the user wants to import material, start or continue learning, establish a baseline, inspect progress, finish a session, or review prior understanding.
---

# Learning OS

Use `learn` as the only writer for runtime-managed state.

Keep every Runtime operation backstage and follow the "Learner-facing voice" rules in the Vault `AGENTS.md`. The learner sees only teaching: no mention of this skill, `learn`, sessions, records, saving, or Git, not even in progress updates before tool calls. Surface the system only when a failure stops the lesson, a user choice or destructive confirmation is required, or the learner asks about recording or status.

## Begin

1. Run `learn status --json` from the Vault. Its `language` is the interface language of generated pages (`zh` or `en`). If the learner is clearly using the other supported language, run `learn config set language <zh|en>` once, silently; for any other language use `en`. Also switch when the learner asks for pages in another language.
2. If an active session exists and the user wants to continue, resume it: find the last assistant question with `learn session turns --json` and pick up exactly there, restating it briefly if the learner has not answered it yet.
3. If the active curriculum reports `position_verified: false`, any `uncovered` entries, or any `partial` entries, read [curriculum outline](references/curriculum-outline.md) and fix that first: confirm an outline with the learner, move the position to the right node, and ask what to do with uncovered and partly studied entries.
4. If the active curriculum reports `baseline.state = not_assessed`, read [baseline workflow](references/baseline-workflow.md). Establish the baseline before a lesson unless the user explicitly skips it.
5. Read [learning policy](references/learning-policy.md) and [session workflow](references/session-workflow.md) before teaching or reviewing.
6. Run `learn next --json` and let its action, concept, strategy, curriculum relation, and next node shape the first teaching move. It encodes what earlier sessions showed about this learner. When it returns `review_due`, follow [review workflow](references/review-workflow.md) first.
7. Before teaching a node, read it in the source. `learn next --json` and `learn status --json` list the node's `resources` (the book's pages, an attached video segment...). Teach what the material says, in its order. How to read each kind of material is in [resources](references/resources.md).

## Record understanding

The Learner Model grows only from Interpretation Records you submit and the Runtime validates. Read [interpretation workflow](references/interpretation-workflow.md) and [interpretation schema](references/interpretation-schema.md) before the first checkpoint of a session. A rejected record changes nothing; fix the cited evidence and resubmit.

## Import material

Read [curriculum import](references/curriculum-import.md) when the user provides a textbook or course path, a video course, a paper book or a class, or wants to stop studying, remove, or restore a book. Complete a dry run before the real import, then build the outline with [curriculum outline](references/curriculum-outline.md).

## Safety boundary

Read [data boundaries](references/data-boundaries.md) before repairing files, changing generated state, or handling a failed command. A command is complete only when its JSON or exit status confirms success.
