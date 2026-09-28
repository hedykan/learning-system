# Resources

A curriculum's material is its primary resource; other material can be attached to outline entries. Every resource declares `capabilities` (`learn source list --json`); decide how to read it from them, not from the file name.

| Capability | What to do |
| --- | --- |
| `extractable` | Read with `learn source read <source> <kind> <value> --json`; the text comes back as Markdown, with any `images` to look at. |
| `needs_vision` | Read the pages yourself; for scanned pages use vision (see [curriculum outline](curriculum-outline.md)). |
| `external` | There is no file. The learner brings the content: what they watched or read, a photo of the page, a transcript. |
| `structured` | `learn source outline <source> --json` gives headings for a draft outline. |

## Locators

A locator says where in a resource something is: `{"resource": "<id>", "kind": "<kind>", "value": "<value>"}`. Omit `resource` for the curriculum's own material.

| Kind | Value | For |
| --- | --- | --- |
| `page` | `42-45` | PDF, paper books |
| `anchor` | `#replication-lag` (heading slug) | Markdown |
| `file` | `ch05.md#L10-40` | lines of a text or Markdown file |
| `time` | `3/05:20-48:00` (episode/start-end) | video and audio |
| `chapter` | `ch05.xhtml#sec2` | e-books |
| `text` | free text | anything else, e.g. “讲义第二部分” |

Outline entries and textbook points take locators; child entries and points must lie inside their entry when positions can be compared.

## Video courses

- Attach each entry's lecture segment (`time`) so `learn next` can tell the learner what to watch.
- Before the learner watches: ask for a prediction about the segment's key idea.
- After: ask them to explain it without notes first (retrieval), then catch misconceptions, then practice with exercises (application), then a variant they have not seen (transfer).
- When the learner says “看完第 3 讲”, map it to the entries attached to that lecture and mark them with `learn curriculum complete`.
- Formulas and proofs on the board are rarely in transcripts: ask for a screenshot and read it with vision.
- When no material covers what you explain, say that it is a general account that may differ from their course. Submit `textbook_points` only after the learner has shown you the page or segment.

## Paper books

Ask for a photo of the contents page to build the outline with `page` locators, and photos of the pages you teach from. Do not teach a section from memory as if it were the book.
