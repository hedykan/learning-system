// Package tags holds the fixed layer tags of generated Vault files
// (CR-2026-035). Obsidian matches child tags when filtering by a parent, so
// `tag:#learning/knowledge` selects concepts and questions. Tags stay in
// English whatever the interface language, so saved filters keep working.
package tags

import "strings"

const (
	Concept      = "learning/knowledge/concept"
	Question     = "learning/knowledge/question"
	Conversation = "learning/evidence/conversation"
	Session      = "learning/process/session"
	Progress     = "learning/process/progress"
	Home         = "learning/nav/home"
	Overview     = "learning/nav/overview"
	Curriculum   = "learning/nav/curriculum"
)

// Lines renders a frontmatter tags list.
func Lines(tags ...string) string {
	var b strings.Builder
	b.WriteString("tags:\n")
	for _, t := range tags {
		b.WriteString("  - " + t + "\n")
	}
	return b.String()
}

// Ensure adds tag to the YAML frontmatter of text when it is missing,
// leaving everything else byte for byte. Text without frontmatter is
// returned unchanged.
func Ensure(text, tag string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return text, false
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return text, false
	}
	head := text[4 : 4+end+1] // frontmatter lines, each ending in \n
	if strings.Contains(head, "  - "+tag+"\n") {
		return text, false
	}
	rest := text[4+end+1:]
	if i := strings.Index(head, "tags:\n"); i == 0 || (i > 0 && head[i-1] == '\n') {
		insert := i + len("tags:\n")
		return "---\n" + head[:insert] + "  - " + tag + "\n" + head[insert:] + rest, true
	}
	return "---\n" + head + Lines(tag) + rest, true
}
