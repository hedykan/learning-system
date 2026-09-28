package projection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/learner"
)

// relationLabel names a relation from the concept that holds the entry.
var relationLabel = map[string]map[string]string{
	"out": {"prerequisite": "先修", "part_of": "属于", "applies_to": "应用了"},
	"":    {"contrast": "易混", "related": "相关"},
	"in":  {"prerequisite": "以它为先修", "part_of": "是它的一部分", "applies_to": "应用了它"},
}

// curriculaOf lists the curricula a concept comes from.
func curriculaOf(c *learner.Concept) map[string]bool {
	out := map[string]bool{}
	for _, r := range c.SourceRefs {
		out[r.Curriculum] = true
	}
	return out
}

// splitRelations separates relations to concepts of another textbook.
func splitRelations(m *learner.Model, c *learner.Concept) (local, cross []learner.RelatedEntry) {
	mine := curriculaOf(c)
	for _, r := range c.Related {
		other := m.Concepts[r.Concept]
		isCross := false
		if other != nil && len(mine) > 0 {
			theirs := curriculaOf(other)
			isCross = len(theirs) > 0
			for id := range theirs {
				if mine[id] {
					isCross = false
				}
			}
		}
		if isCross {
			cross = append(cross, r)
		} else {
			local = append(local, r)
		}
	}
	return local, cross
}

// renderRelations writes outgoing and undirected relations as links, and
// incoming directed ones as plain text, so graph arrows mean something.
func (in Inputs) renderRelations(b *strings.Builder, heading string, rels []learner.RelatedEntry, withBook bool) {
	if len(rels) == 0 {
		return
	}
	m := in.Model
	b.WriteString(heading)
	book := func(id string) string {
		if !withBook {
			return ""
		}
		var titles []string
		for cur := range curriculaOf(m.Concepts[id]) {
			titles = append(titles, in.Titles[cur])
		}
		if len(titles) == 0 {
			return ""
		}
		return fmt.Sprintf(in.t("（%s）"), strings.Join(sortedStrings(titles), in.t("、")))
	}
	var incoming []string
	for _, r := range rels {
		if r.Direction == "in" {
			label := r.Concept
			if c := m.Concepts[r.Concept]; c != nil {
				label = c.Label
			}
			incoming = append(incoming, fmt.Sprintf(in.t("%s（%s）"), label+book(r.Concept), in.t(relationLabel["in"][r.Kind()])))
			continue
		}
		line := fmt.Sprintf("- %s%s%s%s", in.t(relationLabel[r.Direction][r.Kind()]), in.t("："), conceptLink(m, r.Concept), book(r.Concept))
		if r.Note != "" {
			line += in.t("：") + r.Note
		}
		b.WriteString(line + "\n")
	}
	if len(incoming) > 0 {
		fmt.Fprintf(b, in.t("\n被这些概念引用：%s\n"), strings.Join(incoming, in.t("、")))
	}
}

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}
