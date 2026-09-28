// Package mdblock manages marked regions inside Markdown files so the Runtime
// can rewrite its own content while preserving what the learner wrote.
package mdblock

import "strings"

const userHeading = "## 手写笔记\n\n"

// userHeadings are the learner-area headings per interface language.
var userHeadings = map[string]string{"zh": userHeading, "en": "## My notes\n\n"}

func headingFor(lang string) string {
	if h, ok := userHeadings[lang]; ok {
		return h
	}
	return userHeadings["en"]
}

func markers(name string) (string, string) {
	return "<!-- learn:" + name + ":begin -->", "<!-- learn:" + name + ":end -->"
}

// Block wraps inner content in named markers.
func Block(name, inner string) string {
	begin, end := markers(name)
	inner = strings.Trim(inner, "\n")
	if inner == "" {
		return begin + "\n" + end + "\n"
	}
	return begin + "\n" + inner + "\n" + end + "\n"
}

// Extract returns the content between a block's markers.
func Extract(text, name string) (string, bool) {
	begin, end := markers(name)
	i := strings.Index(text, begin)
	if i < 0 {
		return "", false
	}
	rest := text[i+len(begin):]
	j := strings.Index(rest, end)
	if j < 0 {
		return "", false
	}
	return strings.Trim(rest[:j], "\n"), true
}

// Replace swaps a block's inner content, reporting whether it existed.
func Replace(text, name, inner string) (string, bool) {
	begin, end := markers(name)
	i := strings.Index(text, begin)
	if i < 0 {
		return text, false
	}
	j := strings.Index(text[i:], end)
	if j < 0 {
		return text, false
	}
	after := text[i+j+len(end):]
	after = strings.TrimPrefix(after, "\n")
	return text[:i] + Block(name, inner) + after, true
}

// UserSection renders the preserved learner area.
func UserSection(inner string) string { return UserSectionIn("zh", inner) }

// UserSectionIn renders the learner area with the heading of lang.
func UserSectionIn(lang, inner string) string { return headingFor(lang) + Block("user", inner) }

// PreservedUser returns what must survive a rewrite: the marked user block,
// or, for a file the Runtime does not own, its entire existing content.
func PreservedUser(existing string, runtimeOwned bool) string {
	if inner, ok := Extract(existing, "user"); ok {
		return inner
	}
	if runtimeOwned {
		return ""
	}
	return strings.TrimSpace(existing)
}

// Upsert places a named block in text: replace it, insert it before the user
// section, or append it; an empty text becomes header + block + user section.
func Upsert(text, name, inner, header string) string {
	return UpsertIn("zh", text, name, inner, header)
}

// UpsertIn is Upsert with the learner-area heading of lang for new files; an
// existing file keeps whichever heading it already has.
func UpsertIn(lang, text, name, inner, header string) string {
	if strings.TrimSpace(text) == "" {
		return header + "\n" + Block(name, inner) + "\n" + UserSectionIn(lang, "")
	}
	if replaced, ok := Replace(text, name, inner); ok {
		return replaced
	}
	for _, h := range userHeadings {
		if i := strings.Index(text, h); i >= 0 {
			return text[:i] + Block(name, inner) + "\n" + text[i:]
		}
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "\n" + Block(name, inner)
}
