# Data boundaries

- `Sources/` holds immutable source versions and metadata, not learning progress.
- `Curriculum/` holds the learning route: the outline, the current position, and the append-only `progress.yaml`. `progress.md` and `index.md` are generated from them.
- `.learning/tmp/` is the only place for scratch files; Git ignores it.
- `README.md` at the Vault root is the learner's generated home page; only its 手写笔记 block belongs to the learner.
- `.learning/archive/` holds removed books. Change it only through `learn curriculum remove|restore|purge`.
- `.learning/interpretations/` holds accepted Interpretation Records. They are append-only; correct an earlier judgment with a `retraction`, never by editing a record.
- `Concepts/`, `Profile/learner-state.md`, `Curriculum/<id>/index.md`, and the analysis block in `Sessions/` are projections rebuilt from records. Only the "手写笔记" block survives a rebuild; never write learner notes elsewhere in those files. `learn model rebuild` restores them.
- `Conversations/` is append-only raw evidence.
- `Sessions/` holds interpretations that can be regenerated.
- `.learning/` is Runtime state; change it through `learn` commands.

Model output is a proposal. Accept a state change only after the Runtime validates and commits it. Preserve existing files when initialization reports them as already present.
