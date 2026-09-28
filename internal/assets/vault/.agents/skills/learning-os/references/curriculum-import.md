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

The default is `copy`. Use `--link` only when the user asks to keep the original outside the Vault. Supported files: Markdown, plain text, PDF, EPUB, HTML, Word (`.docx`), Jupyter (`.ipynb`), LaTeX, reStructuredText, AsciiDoc, Org, and folders of these (read in natural order, so `ch2` comes before `ch10`). Formats with headings get a draft outline automatically; still check it against the real table of contents and remove non-content entries (tags, index pages, site home) before confirming. A DRM-protected EPUB is refused: ask for a DRM-free copy.

## Web pages and online books

`learn curriculum import <url> --id <id> --title "<name>" --yes` stores a snapshot of one page; add `--sitemap` (or `--sitemap-url <sitemap>`) to take a whole online book, and `--prefix <path>` only when the sitemap mixes languages or editions; look at the sitemap's paths first, since a language's own sitemap often uses root paths such as `/ch6/`. Learning then reads only the snapshot; run `learn source refresh <id>` later to take a new one. Before fetching a site, check whether its source is published as Markdown (often a GitHub repository): cloning that and importing the folder is cleaner. When the command reports that it cannot connect, tell the learner plainly, in one sentence, that the Agent sandbox blocks network access and that they can run the same command in a normal terminal (or allow network access in `.codex/config.toml`); do not retry silently. Never fetch sites that forbid copying.

## Code projects

`learn curriculum import <repo> --kind code --id <id> --yes` links a Git project at its current commit; nothing is copied. Build the outline yourself (modules, or one request's path through the code). Read files with `learn source read <id> file src/raft.go#L120-180`. Credentials, dependency and build folders, and binary files cannot be read, by design. After the learner pulls new commits, `learn source refresh <id>` records the new revision. The Runtime does not extract PDF text; read the pages with your own tools.
