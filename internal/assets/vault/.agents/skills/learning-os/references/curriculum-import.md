# Curriculum import

1. Use the exact path supplied by the user; do not guess a path.
2. Preview with `learn curriculum import <path> --id <id> --dry-run --json`.
3. Check the detected kind, content hash, destination, duplicate status, and title.
4. If the plan matches the request, run the same command with `--yes`; add `--activate` when requested or clearly implied.
5. Run `learn curriculum show <id> --json` to verify the result.
6. Build and confirm the outline with the learner as described in [curriculum outline](curriculum-outline.md). Teaching starts only after the outline is confirmed.

## Material without a file

For a video course, a paper book or a class, register the material itself: `learn curriculum import --external --id <id> --title "<name>" [--url <playlist>] [--note "<publisher, edition>"] --dry-run --json`, then the same with `--yes --activate`. Build the outline from the playlist, a photo of the contents page or the syllabus, with `locator`s instead of pages (see [resources](resources.md)). Never download or store videos.

## Extra material for a book

When the learner studies a book together with a video course, a paper or a second book, add it once with `learn source add <path> --id <id>` (or `--external --title ...`), then attach it to the outline entries it covers: `learn source attach <node> <id> <kind> <value>`. `learn source check --json` reports broken positions and entries without any material.

## Stop, remove, or restore a book

- `learn curriculum deactivate` stops studying the active book without touching anything.
- To remove a book, first show the learner `learn curriculum remove <id> --dry-run --json`, then run it with `--reason "<why>" --yes` only after they agree. Removal archives the book under `.learning/archive/`; conversations, sessions, and notes stay.
- `learn curriculum archives --json` lists archives; `learn curriculum restore <archive-id>` brings one back.
- `learn curriculum purge` deletes an archive forever. Run it only when the learner explicitly asks for permanent deletion and has confirmed the archive id; it needs `--yes --confirm <archive-id>`.

The default is `copy`. Use `--link` only when the user asks to keep the original outside the Vault. Supported files: Markdown, text, PDF, and folders of Markdown or text (read in natural order, so `ch2` comes before `ch10`). The Runtime does not extract PDF text; read the pages with your own tools.
