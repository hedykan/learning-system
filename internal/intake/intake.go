// Package intake joins what is known about a learner before and while a
// curriculum is built (CR-2026-044, CR-2026-045): the goal card, the intake
// assessment and the Learner Model, entry by entry. It only reports; the
// Agent decides what to change and the learner agrees.
package intake

import (
	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
)

// Hint is what is already known about one concept of an entry.
type Hint struct {
	Concept string `json:"concept"`
	// Finding is existing_knowledge, prerequisite_gap or
	// possible_misconception when the intake assessment named the concept.
	Finding string `json:"finding,omitempty"`
	Claim   string `json:"claim,omitempty"`
	// State is the concept's Learner Model state, when it has evidence.
	State string `json:"state,omitempty"`
}

// Hints maps entry ids to what is known about their declared concepts;
// entries with nothing known are left out.
func Hints(o curriculum.Outline, report *assessment.Report, m *learner.Model) map[string][]Hint {
	type finding struct{ kind, claim string }
	found := map[string]finding{}
	if report != nil {
		for kind, list := range map[string][]assessment.Finding{
			"existing_knowledge": report.ExistingKnowledge, "prerequisite_gap": report.PrerequisiteGaps,
			"possible_misconception": report.PossibleMisconceptions,
		} {
			for _, f := range list {
				for _, c := range f.Concepts {
					// A gap or misconception outweighs claimed knowledge.
					if prev, ok := found[c]; !ok || prev.kind == "existing_knowledge" {
						found[c] = finding{kind, f.Claim}
					}
				}
			}
		}
	}
	out := map[string][]Hint{}
	for _, n := range o.Nodes {
		for _, c := range n.Concepts {
			h := Hint{Concept: c}
			if f, ok := found[c]; ok {
				h.Finding, h.Claim = f.kind, f.claim
			}
			if m != nil {
				if concept := m.Concepts[c]; concept != nil {
					h.State = concept.State()
				}
			}
			if h.Finding != "" || h.State != "" {
				out[n.ID] = append(out[n.ID], h)
			}
		}
	}
	return out
}

// LikelyKnown reports entries all of whose concepts the intake found as
// existing knowledge or the model holds as stable: candidates for skip or
// mark_known proposals, never applied automatically.
func LikelyKnown(o curriculum.Outline, hints map[string][]Hint) []string {
	var out []string
	for _, n := range o.Nodes {
		if len(n.Concepts) == 0 || len(hints[n.ID]) != len(n.Concepts) {
			continue
		}
		all := true
		for _, h := range hints[n.ID] {
			if h.Finding != "existing_knowledge" && h.State != "stable" {
				all = false
			}
			if h.Finding == "prerequisite_gap" || h.Finding == "possible_misconception" {
				all = false
			}
		}
		if all {
			out = append(out, n.ID)
		}
	}
	return out
}

// EntryReview is one entry as the learner reviews a draft.
type EntryReview struct {
	curriculum.Node
	Status    string                    `json:"status"`
	Hints     []Hint                    `json:"hints,omitempty"`
	Resources []curriculum.NodeResource `json:"resources,omitempty"`
	Unsourced bool                      `json:"unsourced,omitempty"`
}

// Review is the draft review of CR-2026-045.
type Review struct {
	Curriculum  string                 `json:"curriculum"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Goal        *curriculum.StoredGoal `json:"goal,omitempty"`
	Entries     []EntryReview          `json:"entries"`
	Uncovered   []curriculum.FocusItem `json:"uncovered_focus"`
	LikelyKnown []string               `json:"likely_known"`
	Ready       bool                   `json:"ready_to_confirm"`
	Blocking    string                 `json:"blocking,omitempty"`
}

// Build assembles the review of a curriculum's outline.
func Build(root, id, lang string) (Review, error) {
	o, err := curriculum.LoadOutline(root, id)
	if err != nil {
		return Review{}, err
	}
	rev := Review{Curriculum: id, Type: o.TypeOf(), Status: o.Status, Entries: []EntryReview{}, Uncovered: []curriculum.FocusItem{}, LikelyKnown: []string{}}
	if g, ok, err := curriculum.LoadGoal(root, id); err != nil {
		return rev, err
	} else if ok {
		rev.Goal = &g
		rev.Uncovered = append(rev.Uncovered, curriculum.Coverage(o, g.GoalCard)...)
	}
	report, err := assessment.Latest(root, id)
	if err != nil {
		return rev, err
	}
	m, _, err := learner.Load(root)
	if err != nil {
		return rev, err
	}
	hints := Hints(o, report, m)
	if known := LikelyKnown(o, hints); known != nil {
		rev.LikelyKnown = known
	}
	entries, err := curriculum.LoadProgress(root, id)
	if err != nil {
		return rev, err
	}
	pos, _ := curriculum.LoadPosition(root, id)
	unsourced := curriculum.Unsourced(root, id)
	for _, s := range curriculum.Statuses(o, entries, pos, m.NodesWithEvidence(id)) {
		res, err := curriculum.NodeResources(root, id, s.ID, lang)
		if err != nil {
			return rev, err
		}
		rev.Entries = append(rev.Entries, EntryReview{Node: s.Node, Status: s.Status, Hints: hints[s.ID], Resources: res, Unsourced: unsourced[s.ID]})
	}
	if o.Status == "draft" {
		if err := curriculum.ReadyToConfirm(root, id, o); err != nil {
			rev.Blocking = err.Error()
		} else {
			rev.Ready = true
		}
	}
	return rev, nil
}
