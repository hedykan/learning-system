# Curriculum outline

The outline ties every lesson to the real textbook. Without a confirmed outline the position is only a guess, and a guess from memory can come from a different edition.

## Build the outline

1. Read the table of contents in the imported source (`Sources/<id>/original/`). Use your own PDF or document reading tools. Put any extracted text or scratch files under `.learning/tmp/`, never elsewhere in the Vault.
2. Write the outline as JSON or YAML: a `nodes` list in reading order. Each node has `id` (`1`, `1.2`, `1.2.3`), `title` exactly as printed, and `pages: [start, end]` using the PDF page numbers you actually read (for other material give a `locator` instead, see [resources](resources.md); Markdown books already get a draft from their headings). A section's pages are its own range, found from where its heading appears in the text; if you cannot locate a section's start, omit its `pages` instead of copying the chapter's range. Include chapters and the sections you can see; skip front matter, indexes, and parts like “I 数据系统基础”.
3. Validate without writing:

   ```bash
   learn curriculum outline set --file .learning/tmp/outline.json --dry-run --json
   ```

4. Save it with the same command without `--dry-run`, then show the learner the chapter list in plain words and ask whether it matches their book.
5. Only after the learner agrees, run `learn curriculum outline confirm`. Confirming also means this file really is the book they want to study.

Markdown sources get a draft outline automatically at import; review and confirm it the same way.

## Scanned or unreadable pages

Read in this order and never fall back to memory:

1. Extract the page text with your tools.
2. If a page yields no usable text (scanned images, garbled symbols), render that page to an image under `.learning/tmp/` and read it with your own vision.
3. When the outline or points came from page images, tell the learner “这部分是从扫描页识别的，请帮忙核对” before confirming.
4. If the pages still cannot be read reliably, stop and tell the learner plainly that this copy is a scanned or damaged file, and suggest getting a text-based PDF or EPUB of the same edition. Do not teach that part from memory.

## Keep the position on the outline

- Set the position with `learn curriculum position set --node <id> [--concept "<idea within the section>"]`. After confirmation, chapter and section labels come from the outline and free-text labels are rejected.
- `learn status --json` reports `position_verified` and `uncovered` entries. If the position is not verified, rebuild or confirm the outline and move the position to the matching node before teaching.
- For each uncovered entry, ask the learner whether to study it now or skip it, then record the choice. Never skip silently.
- `partial` entries were taught but never finished. Ask whether to finish them now, mark them complete (if the main ideas were covered), or skip them. `learn next` returns to an earlier partial entry before moving on.

## Teach from the text

Before teaching a node, read its pages, using the same scanned-page order as above. Follow the book's order and examples; use the learner's own work only as extra scenarios. If what you remember differs from the text, the text wins.

## Textbook points

After reading a node's pages, add `textbook_points` to the concepts you taught from it: up to five short lines, in the book's own terms, with the pages you read (see [interpretation schema](interpretation-schema.md)). The pages must lie inside the node's page range. Never write points from memory or for pages you did not read; leave the field out instead. Points describe the book, not the learner.

## Mark progress

Curriculum progress is separate from understanding.

```bash
learn curriculum complete <node-id> --reason "<what the learner can now do>"
learn curriculum skip <node-id> --reason "<why the learner chose to skip>"
```

Complete a section when its main ideas were covered in the lesson. Before leaving a section you taught, either complete it or tell the learner it stays partly studied. `learn curriculum complete` prints the next unfinished entry (`next_node`); move the position there, never to a section chosen from memory. Setting the position on a completed entry is a review and is reported as such. Completing a section does not make its concepts stable; the learner model handles that separately.
