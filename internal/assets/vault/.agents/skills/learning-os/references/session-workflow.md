# Session workflow

1. Run `learn session start` before the first learning exchange. Confirm its returned kind; do not teach lesson content inside a baseline.
2. Append each learning turn with a quoted heredoc and keep the returned turn ID:

   ```bash
   learn session append --role user --json <<'TURN'
   <learner text, verbatim>
   TURN
   ```

   The quoted delimiter keeps quotes, `$`, and backticks literal. Alternatively write the text to `.learning/tmp/` and pass `--file`. Never put scratch files anywhere else in the Vault. Never run `append`, `checkpoint`, or `end` with `-` unless content is actually piped in. Append the learner's words verbatim; evidence quotes must match them. After an interruption, `learn session turns --json` lists every turn ID.
3. Teach the current outline node from its pages. Use the active curriculum position, baseline, and learner evidence under the [learning policy](learning-policy.md). Treat a claim of understanding as evidence, then probe explanation, prediction, variation, retrieval, or transfer.
4. After each concept reaches a natural pause (a revision, a failed or successful probe, a strategy change), submit a checkpoint as described in the [interpretation workflow](interpretation-workflow.md), then run `learn next --json` before the next teaching move.
5. Before ending, check the concepts touched in this session against the existing ones (`learn state --json`) and add `related` links where the book or the learner's reasoning connects them. The Obsidian graph is built only from these links. `session end` is refused while a touched concept has no relation; the error lists the existing concepts. Link it, or put its ID in `no_related` when none of them is truly related.
6. End with an interpretation record that carries the progress decision:

   ```bash
   printf '%s' "$RECORD_JSON" | learn session end --analysis-file - --json
   ```

   Submit records the same way (`--analysis-file - <<'JSON' ... JSON`, or a file under `.learning/tmp/`). A lesson, review, or practice cannot end without a record or earlier checkpoint. Use `--no-analysis --reason "<why>"` only when nothing interpretable happened. Ending never moves the curriculum position. When the decision is `advance`, first `learn curriculum complete <node>` for the finished section, then `learn curriculum position set --node <next_node from that output>`.
7. Close with the learning outcome and the next useful step, in teaching language only. Do not say that progress was saved or recorded, and do not mention permission problems. The one exception is the Git reminder in the Vault `AGENTS.md`: when `learn status --json` shows `git_auto_commit: failed` with `git_uncommitted` above 0, add that single sentence after the summary.
8. Append only learning content. A pure control phrase such as “继续学习” or “今天先到这里” is not a learning turn unless it carries an answer.

If the interaction is interrupted, leave the active session recoverable. Never reconstruct missing verbatim turns and present them as raw history.
