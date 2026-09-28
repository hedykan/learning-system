package projection

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
)

// Generated notes are named in the learner's language (CR-2026-022): concept
// labels, question text and book titles come from the Agent in that
// language; fixed pages use the projection language. Internal IDs never
// appear in file names, but stay in frontmatter aliases so old links resolve.
const (
	OverviewFile       = "Profile/学习者总览.md"
	legacyOverviewFile = "Profile/learner-state.md"
)

// unsafeName holds characters that are invalid in file names on some
// platform or that break Obsidian links.
const unsafeName = `/\:*?"<>|#^[]`

// noteName turns a title into a portable file name without extension.
func noteName(title string, max int) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(title) {
		switch {
		case strings.ContainsRune(unsafeName, r):
			b.WriteRune('-')
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	if utf8.RuneCountInString(name) > max {
		name = string([]rune(name)[:max])
	}
	return strings.Trim(name, " .-")
}

// uniqueNames assigns each id a unique name, resolving clashes by id order.
func uniqueNames(ids []string, title func(string) string, max int) map[string]string {
	sort.Strings(ids)
	out := map[string]string{}
	used := map[string]bool{}
	for _, id := range ids {
		name := noteName(title(id), max)
		if name == "" {
			name = id
		}
		if used[strings.ToLower(name)] {
			name = name + " (" + id + ")"
		}
		used[strings.ToLower(name)] = true
		out[id] = name
	}
	return out
}

var nameCache = struct {
	model     *learner.Model
	concepts  map[string]string
	questions map[string]string
}{}

func names(m *learner.Model) (map[string]string, map[string]string) {
	if nameCache.model == m && nameCache.concepts != nil {
		return nameCache.concepts, nameCache.questions
	}
	cids := make([]string, 0, len(m.Concepts))
	for id := range m.Concepts {
		cids = append(cids, id)
	}
	qids := make([]string, 0, len(m.Questions))
	for id := range m.Questions {
		qids = append(qids, id)
	}
	nameCache.model = m
	nameCache.concepts = uniqueNames(cids, func(id string) string { return m.Concepts[id].Label }, 80)
	nameCache.questions = uniqueNames(qids, func(id string) string { return m.Questions[id].Question }, 60)
	return nameCache.concepts, nameCache.questions
}

// ConceptFile is the Vault-relative path of a concept note.
func ConceptFile(m *learner.Model, id string) string {
	c, _ := names(m)
	return "Concepts/" + c[id] + ".md"
}

// QuestionFile is the Vault-relative path of a question note.
func QuestionFile(m *learner.Model, id string) string {
	_, q := names(m)
	return "Questions/" + q[id] + ".md"
}

// CurriculumIndexFile is the book's home page, named after the book.
func CurriculumIndexFile(id, title string) string {
	name := noteName(title, 80)
	if name == "" {
		name = id
	}
	return "Curriculum/" + id + "/" + name + ".md"
}

// CurriculumProgressFile is the generated progress page of a book.
func CurriculumProgressFile(id string) string {
	return "Curriculum/" + id + "/" + curriculum.ProgressFile
}

// link renders an Obsidian wikilink to a Vault-relative .md path.
func link(path, label string) string {
	return "[[" + strings.TrimSuffix(path, ".md") + "|" + label + "]]"
}
