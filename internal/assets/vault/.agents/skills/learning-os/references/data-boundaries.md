# Data boundaries

- `Sources/` holds immutable source versions and metadata, not learning progress.
- `Curriculum/` holds the learning route: the outline, the current position, and the append-only `progress.yaml`. `学习进度.md` and the book home page (named after the book) are generated from them.
- `.learning/tmp/` is the only place for scratch files; Git ignores it.
- `README.md` at the Vault root is the learner's generated home page; only its 手写笔记 block belongs to the learner.
- `.learning/archive/` holds removed books. Change it only through `learn curriculum remove|restore|purge`.
- `.learning/interpretations/` holds accepted Interpretation Records. They are append-only; correct an earlier judgment with a `retraction`, never by editing a record.
- `Concepts/`, `Questions/`, `Profile/学习者总览.md`, each book's home page, and the analysis block in `Sessions/` are projections rebuilt from records. Notes are named after concept labels, questions, and book titles in the learner's language, so write labels and questions in that language; internal IDs stay in the notes' aliases. Only the "手写笔记" block survives a rebuild; never write learner notes elsewhere in those files. `learn model rebuild` restores them.
- `Conversations/` is append-only raw evidence.
- `Sessions/` holds interpretations that can be regenerated.
- `.learning/` is Runtime state; change it through `learn` commands.

Model output is a proposal. Accept a state change only after the Runtime validates and commits it. Preserve existing files when initialization reports them as already present.
