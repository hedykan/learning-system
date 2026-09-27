# Curriculum import

1. Use the exact path supplied by the user; do not guess a path.
2. Preview with `learn curriculum import <path> --id <id> --dry-run --json`.
3. Check the detected kind, content hash, destination, duplicate status, and title.
4. If the plan matches the request, run the same command with `--yes`; add `--activate` when requested or clearly implied.
5. Run `learn curriculum show <id> --json` to verify the result.
6. Build and confirm the outline with the learner as described in [curriculum outline](curriculum-outline.md). Teaching starts only after the outline is confirmed.

## Stop, remove, or restore a book

- `learn curriculum deactivate` stops studying the active book without touching anything.
- To remove a book, first show the learner `learn curriculum remove <id> --dry-run --json`, then run it with `--reason "<why>" --yes` only after they agree. Removal archives the book under `.learning/archive/`; conversations, sessions, and notes stay.
- `learn curriculum archives --json` lists archives; `learn curriculum restore <archive-id>` brings one back.
- `learn curriculum purge` deletes an archive forever. Run it only when the learner explicitly asks for permanent deletion and has confirmed the archive id; it needs `--yes --confirm <archive-id>`.

The default is `copy`. Use `--link` only when the user asks to keep the original outside the Vault. The Runtime does not extract PDF text; read the source with your own tools.
