// Package learner replays Interpretation Records into the Learner Model.
// Submission and rebuild share Apply, so a record is accepted only if the
// model replayed with it stays valid: Model can change, History cannot.
package learner

import (
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/locator"
)

var (
	EventTypes = set("question", "prediction", "attempt", "misconception", "correction", "insight",
		"understanding_revision", "prerequisite_gap", "retrieval", "application", "transfer",
		"connection", "learner_proposed_method")
	States       = set("developing", "fragile", "stable")
	Capabilities = set("recognized", "explained", "predicted", "retrieved", "applied", "transferred")
	Strategies   = set("concrete_example", "analogy", "diagram", "counterexample", "prediction_probe",
		"learner_action", "direct_explanation", "prerequisite_repair", "retrieval_practice")
	Situations     = set("new_concept", "misconception", "prerequisite_gap", "retrieval", "transfer")
	Outcomes       = set("effective", "inconclusive", "ineffective")
	Decisions      = set("stay", "advance", "detour")
	ReviewOutcomes = set("recalled", "partial", "forgotten")
	// positiveEvents can justify an "effective" strategy attempt.
	positiveEvents = set("insight", "correction", "understanding_revision", "retrieval", "application", "transfer")
)

func set(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}

type EvidenceRef struct {
	Session string `json:"session"`
	Turn    string `json:"turn"`
	Quote   string `json:"quote"`
}

type SourceRef struct {
	Curriculum string `json:"curriculum"`
	Node       string `json:"node,omitempty"`
	Chapter    string `json:"chapter,omitempty"`
	Section    string `json:"section,omitempty"`
}

type StateEntry struct {
	GID           string        `json:"id"`
	Order         int           `json:"order"`
	Session       string        `json:"session"`
	At            string        `json:"at"`
	State         string        `json:"state"`
	Capabilities  []string      `json:"capabilities"`
	Summary       string        `json:"summary"`
	Evidence      []EvidenceRef `json:"evidence"`
	OpenQuestions []string      `json:"open_questions,omitempty"`
	Superseded    string        `json:"superseded,omitempty"`
}

type Concept struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	Aliases    []string       `json:"aliases,omitempty"`
	SourceRefs []SourceRef    `json:"source_refs,omitempty"`
	Points     []*PointsEntry `json:"textbook_points,omitempty"`
	Related    []RelatedEntry `json:"related,omitempty"`
	NoRelated  bool           `json:"no_related,omitempty"`
	History    []*StateEntry  `json:"history"`
}

// PointsEntry is one version of a concept's textbook points; the last one
// is current and earlier ones remain as history.
type PointsEntry struct {
	Session string          `json:"session"`
	At      string          `json:"at"`
	Locator locator.Locator `json:"locator"`
	Points  []string        `json:"points"`
}

// RelatedEntry is one concept relation, stored on both ends. Type is empty
// for a plain related link. Directed types are "out" on the declaring
// concept and "in" on the other end.
type RelatedEntry struct {
	Concept   string `json:"concept"`
	Type      string `json:"type,omitempty"`
	Direction string `json:"direction,omitempty"`
	Note      string `json:"note,omitempty"`
}

// RelationTypes lists the allowed relation types; true marks directed ones.
var RelationTypes = map[string]bool{"related": false, "contrast": false, "prerequisite": true, "part_of": true, "applies_to": true}

// Kind is the relation type, "related" when unset.
func (r RelatedEntry) Kind() string {
	if r.Type == "" {
		return "related"
	}
	return r.Type
}

// QuestionItem is a learner-generated key question with its lifecycle.
type QuestionItem struct {
	ID         string          `json:"id"`
	Question   string          `json:"question"`
	Concept    string          `json:"concept,omitempty"`
	Node       string          `json:"node,omitempty"`
	Curriculum string          `json:"curriculum"`
	Session    string          `json:"session"`
	At         string          `json:"at"`
	Order      int             `json:"order"`
	Evidence   []EvidenceRef   `json:"evidence"`
	Status     string          `json:"status"` // open, resolved
	Resolution *QuestionAnswer `json:"resolution,omitempty"`
}

// QuestionAnswer records how and when a question was resolved.
type QuestionAnswer struct {
	Session  string        `json:"session"`
	At       string        `json:"at"`
	Summary  string        `json:"summary"`
	Evidence []EvidenceRef `json:"evidence"`
}

// Review is one spaced-review outcome.
type Review struct {
	GID      string        `json:"id"`
	Session  string        `json:"session"`
	At       string        `json:"at"`
	Order    int           `json:"order"`
	Concept  string        `json:"concept"`
	Outcome  string        `json:"outcome"`
	Evidence []EvidenceRef `json:"evidence"`
}

type Event struct {
	GID        string        `json:"id"`
	Session    string        `json:"session"`
	Curriculum string        `json:"curriculum"`
	At         string        `json:"at"`
	Order      int           `json:"order"`
	Type       string        `json:"type"`
	Concept    string        `json:"concept,omitempty"`
	Summary    string        `json:"summary"`
	Evidence   []EvidenceRef `json:"evidence"`
	DerivedBy  string        `json:"derived_by,omitempty"`
	Superseded string        `json:"superseded,omitempty"`
}

type Change struct {
	GID         string        `json:"id"`
	Session     string        `json:"session"`
	Curriculum  string        `json:"curriculum"`
	At          string        `json:"at"`
	Order       int           `json:"order"`
	Concept     string        `json:"concept"`
	OldModel    string        `json:"old_model"`
	Trigger     string        `json:"trigger"`
	TriggerTurn string        `json:"trigger_turn"`
	NewModel    string        `json:"new_model"`
	OldEvidence []EvidenceRef `json:"old_evidence"`
	NewEvidence []EvidenceRef `json:"new_evidence"`
	Superseded  string        `json:"superseded,omitempty"`
}

type Attempt struct {
	GID            string        `json:"id"`
	Session        string        `json:"session"`
	Curriculum     string        `json:"curriculum"`
	At             string        `json:"at"`
	Order          int           `json:"order"`
	Strategy       string        `json:"strategy"`
	Situation      string        `json:"situation"`
	Concept        string        `json:"concept"`
	ActionTurn     string        `json:"action_turn"`
	ExpectedChange string        `json:"expected_change"`
	Outcome        string        `json:"outcome"`
	Reason         string        `json:"reason,omitempty"`
	Linked         []string      `json:"linked,omitempty"`
	Evidence       []EvidenceRef `json:"evidence"`
	Replaces       string        `json:"replaces,omitempty"`
	SwitchReason   string        `json:"switch_reason,omitempty"`
	Superseded     string        `json:"superseded,omitempty"`
}

type Observation struct {
	GID        string        `json:"id"`
	Session    string        `json:"session"`
	Curriculum string        `json:"curriculum"`
	At         string        `json:"at"`
	Order      int           `json:"order"`
	Stance     string        `json:"stance"`
	Summary    string        `json:"summary"`
	Linked     []string      `json:"linked"`
	Evidence   []EvidenceRef `json:"evidence"`
}

type Pattern struct {
	ID                string         `json:"id"`
	Description       string         `json:"description"`
	PreferredStrategy string         `json:"preferred_strategy,omitempty"`
	Situation         string         `json:"situation,omitempty"`
	Observations      []*Observation `json:"observations"`
}

type Progress struct {
	Session  string `json:"session"`
	At       string `json:"at"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type Retraction struct {
	Ref     string `json:"ref"`
	Reason  string `json:"reason"`
	Session string `json:"session"`
	At      string `json:"at"`
}

type SessionInfo struct {
	ID         string `json:"id"`
	Curriculum string `json:"curriculum"`
	Records    int    `json:"records"`
}

// Model is the replayed Learner Model plus Cognitive History.
type Model struct {
	Generation  string                   `json:"generation"`
	Sessions    map[string]*SessionInfo  `json:"sessions"`
	Concepts    map[string]*Concept      `json:"concepts"`
	Events      []*Event                 `json:"events"`
	Changes     []*Change                `json:"cognitive_changes"`
	Attempts    []*Attempt               `json:"strategy_attempts"`
	Patterns    map[string]*Pattern      `json:"patterns"`
	Progress    []*Progress              `json:"progress_decisions"`
	Reviews     []*Review                `json:"reviews"`
	Questions   map[string]*QuestionItem `json:"questions"`
	Retractions []*Retraction            `json:"retractions"`

	order    int
	items    map[string]any
	itemJSON map[string]string
}

func New() *Model {
	return &Model{
		Sessions: map[string]*SessionInfo{}, Concepts: map[string]*Concept{}, Patterns: map[string]*Pattern{},
		Events: []*Event{}, Changes: []*Change{}, Attempts: []*Attempt{}, Progress: []*Progress{}, Retractions: []*Retraction{}, Reviews: []*Review{},
		Questions: map[string]*QuestionItem{},
		items:     map[string]any{}, itemJSON: map[string]string{},
	}
}

// Empty reports whether no record has been accepted.
func (m *Model) Empty() bool { return len(m.Sessions) == 0 }

// Current returns the latest non-superseded state entry, or nil.
func (c *Concept) Current() *StateEntry {
	for i := len(c.History) - 1; i >= 0; i-- {
		if c.History[i].Superseded == "" {
			return c.History[i]
		}
	}
	return nil
}

// State returns the current qualitative state.
func (c *Concept) State() string {
	if cur := c.Current(); cur != nil {
		return cur.State
	}
	return "unobserved"
}

// Capabilities unions evidence capabilities across non-superseded history.
func (c *Concept) Capabilities() []string {
	seen := map[string]bool{}
	for _, entry := range c.History {
		if entry.Superseded == "" {
			for _, cap := range entry.Capabilities {
				seen[cap] = true
			}
		}
	}
	return sortedKeys(seen)
}

// InChapter reports whether the concept is anchored in a chapter of a
// curriculum. Outline node IDs match by prefix ("2.3" is in chapter "2");
// without nodes, chapter labels match exactly or by their first word, so
// "第1章" and "第1章 可靠、可扩展与可维护的应用" are the same chapter.
func (c *Concept) InChapter(curriculum, chapterNode, chapterLabel string) bool {
	for _, ref := range c.SourceRefs {
		if ref.Curriculum != curriculum {
			continue
		}
		if chapterNode != "" && ref.Node != "" {
			if ref.Node == chapterNode || strings.HasPrefix(ref.Node, chapterNode+".") {
				return true
			}
			continue
		}
		if chapterLabel == "" || SameChapter(ref.Chapter, chapterLabel) {
			return true
		}
	}
	return false
}

// SameChapter compares chapter labels exactly or by their first word.
func SameChapter(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	fa, fb := strings.Fields(a), strings.Fields(b)
	return fa[0] == fb[0]
}

// NodesWithEvidence returns outline entries of a curriculum that have a
// concept with at least one live state entry or event.
func (m *Model) NodesWithEvidence(curriculum string) map[string]bool {
	out := map[string]bool{}
	for _, c := range m.Concepts {
		live := c.Current() != nil
		for _, e := range m.Events {
			live = live || (e.Concept == c.ID && e.Superseded == "")
		}
		if !live {
			continue
		}
		for _, r := range c.SourceRefs {
			if r.Curriculum == curriculum && r.Node != "" {
				out[r.Node] = true
			}
		}
	}
	return out
}

// PatternStatus derives candidate, supported or contested.
func (p *Pattern) Status() string {
	supports, contradicts := 0, 0
	sessions, curricula := map[string]bool{}, map[string]bool{}
	for _, obs := range p.Observations {
		if obs.Stance == "supports" {
			supports++
			sessions[obs.Session] = true
			curricula[obs.Curriculum] = true
		} else {
			contradicts++
		}
	}
	switch {
	case contradicts > 0 && contradicts >= supports:
		return "contested"
	case supports > contradicts && ((len(sessions) >= 2 && len(curricula) >= 2) || len(sessions) >= 3):
		// Two curricula across two sessions, or three sessions in one
		// curriculum (CR-2026-018).
		return "supported"
	default:
		return "candidate"
	}
}

// QuestionList returns questions in the order they were asked.
func (m *Model) QuestionList() []*QuestionItem {
	out := make([]*QuestionItem, 0, len(m.Questions))
	for _, q := range m.Questions {
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// OpenQuestions returns open questions of a curriculum ("" for all).
func (m *Model) OpenQuestions(curriculum string) []*QuestionItem {
	var out []*QuestionItem
	for _, q := range m.QuestionList() {
		if q.Status == "open" && (curriculum == "" || q.Curriculum == curriculum) {
			out = append(out, q)
		}
	}
	return out
}

// ConceptList returns concepts sorted by ID.
func (m *Model) ConceptList() []*Concept {
	out := make([]*Concept, 0, len(m.Concepts))
	for _, id := range sortedKeys(m.Concepts) {
		out = append(out, m.Concepts[id])
	}
	return out
}

// PatternList returns patterns sorted by ID.
func (m *Model) PatternList() []*Pattern {
	out := make([]*Pattern, 0, len(m.Patterns))
	for _, id := range sortedKeys(m.Patterns) {
		out = append(out, m.Patterns[id])
	}
	return out
}

// FindConcept matches an ID, label or alias case-insensitively.
func (m *Model) FindConcept(key string) *Concept {
	if key == "" {
		return nil
	}
	if c, ok := m.Concepts[key]; ok {
		return c
	}
	k := fold(key)
	for _, c := range m.ConceptList() {
		if fold(c.Label) == k {
			return c
		}
		for _, a := range c.Aliases {
			if fold(a) == k {
				return c
			}
		}
	}
	return nil
}

// UnresolvedMisconceptions lists live misconception events on a concept with
// no later correction or understanding revision.
func (m *Model) UnresolvedMisconceptions(conceptID string) []*Event {
	var out []*Event
	for _, ev := range m.Events {
		if ev.Superseded != "" || ev.Concept != conceptID || ev.Type != "misconception" {
			continue
		}
		resolved := false
		for _, later := range m.Events {
			if later.Superseded == "" && later.Concept == conceptID && later.Order > ev.Order &&
				(later.Type == "correction" || later.Type == "understanding_revision") {
				resolved = true
				break
			}
		}
		if !resolved {
			out = append(out, ev)
		}
	}
	return out
}

// LiveAttempts returns non-superseded strategy attempts in order.
func (m *Model) LiveAttempts() []*Attempt {
	var out []*Attempt
	for _, a := range m.Attempts {
		if a.Superseded == "" {
			out = append(out, a)
		}
	}
	return out
}

// ChangesFor returns non-superseded cognitive changes on a concept.
func (m *Model) ChangesFor(conceptID string) []*Change {
	var out []*Change
	for _, ch := range m.Changes {
		if ch.Concept == conceptID {
			out = append(out, ch)
		}
	}
	return out
}

// EventsFor returns events on a concept, including superseded ones.
func (m *Model) EventsFor(conceptID string) []*Event {
	var out []*Event
	for _, ev := range m.Events {
		if ev.Concept == conceptID {
			out = append(out, ev)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
