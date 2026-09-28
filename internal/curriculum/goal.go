package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

// GoalItem is one field of a goal card: what the Agent understood, and the
// learner's own words it rests on (CR-2026-043).
type GoalItem struct {
	Text     string `yaml:"text" json:"text"`
	Evidence string `yaml:"evidence" json:"evidence"`
}

// FocusItem is a concrete point the learner cares about, which outline
// entries can serve (CR-2026-045).
type FocusItem struct {
	ID       string `yaml:"id" json:"id"`
	Text     string `yaml:"text" json:"text"`
	Evidence string `yaml:"evidence" json:"evidence"`
}

// GoalCard is the structured learning goal of a curriculum.
type GoalCard struct {
	Outcome         *GoalItem   `yaml:"outcome" json:"outcome"`
	Context         *GoalItem   `yaml:"context,omitempty" json:"context,omitempty"`
	Background      *GoalItem   `yaml:"background,omitempty" json:"background,omitempty"`
	Constraints     *GoalItem   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	SuccessCriteria *GoalItem   `yaml:"success_criteria,omitempty" json:"success_criteria,omitempty"`
	Focus           []FocusItem `yaml:"focus,omitempty" json:"focus,omitempty"`
}

// StoredGoal is a goal card with where it came from.
type StoredGoal struct {
	GoalCard `yaml:",inline"`
	Version  int    `yaml:"version" json:"version"`
	Session  string `yaml:"session" json:"session"`
	Source   string `yaml:"source" json:"source"` // assessment or set
	SetAt    string `yaml:"set_at" json:"set_at"`
}

var focusID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Items lists the card's fields with their names, for evidence checks.
func (g GoalCard) Items() map[string]GoalItem {
	out := map[string]GoalItem{}
	for name, it := range map[string]*GoalItem{"outcome": g.Outcome, "context": g.Context, "background": g.Background,
		"constraints": g.Constraints, "success_criteria": g.SuccessCriteria} {
		if it != nil {
			out[name] = *it
		}
	}
	for _, f := range g.Focus {
		out["focus "+f.ID] = GoalItem{Text: f.Text, Evidence: f.Evidence}
	}
	return out
}

// Check validates the card's shape; evidence is checked against the
// conversation by the caller.
func (g GoalCard) Check() error {
	if g.Outcome == nil || strings.TrimSpace(g.Outcome.Text) == "" {
		return fmt.Errorf("goal card needs an outcome: what the learner wants to be able to do")
	}
	seen := map[string]bool{}
	for _, f := range g.Focus {
		if !focusID.MatchString(f.ID) {
			return fmt.Errorf("goal card focus id %q must be kebab-case", f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("goal card focus id %q is duplicated", f.ID)
		}
		seen[f.ID] = true
	}
	for name, it := range g.Items() {
		if n := utf8.RuneCountInString(strings.TrimSpace(it.Text)); n < 2 || n > 200 {
			return fmt.Errorf("goal card %s must be 2-200 characters", name)
		}
		if strings.TrimSpace(it.Evidence) == "" {
			return fmt.Errorf("goal card %s needs the learner's words as evidence", name)
		}
	}
	return nil
}

// HasFocus reports whether the card names a focus id.
func (g GoalCard) HasFocus(id string) bool {
	for _, f := range g.Focus {
		if f.ID == id {
			return true
		}
	}
	return false
}

func goalPath(root, id string) string { return filepath.Join(root, "Curriculum", id, "goal.yaml") }

// LoadGoal reads a curriculum's goal card; ok is false when it has none.
func LoadGoal(root, id string) (StoredGoal, bool, error) {
	data, err := os.ReadFile(goalPath(root, id))
	if os.IsNotExist(err) {
		return StoredGoal{}, false, nil
	}
	if err != nil {
		return StoredGoal{}, false, err
	}
	var g StoredGoal
	if err := yaml.Unmarshal(data, &g); err != nil {
		return StoredGoal{}, false, fmt.Errorf("parse goal card of %s: %w", id, err)
	}
	return g, true, nil
}

// SaveGoal stores a checked goal card, keeping the previous one in
// goal-history.
func SaveGoal(root, id string, card GoalCard, source, session string, now time.Time) (StoredGoal, error) {
	if err := card.Check(); err != nil {
		return StoredGoal{}, err
	}
	next := StoredGoal{GoalCard: card, Version: 1, Session: session, Source: source, SetAt: now.UTC().Format(time.RFC3339)}
	if prev, ok, err := LoadGoal(root, id); err != nil {
		return next, err
	} else if ok {
		next.Version = prev.Version + 1
		data, err := yaml.Marshal(prev)
		if err != nil {
			return next, err
		}
		hist := filepath.Join(root, "Curriculum", id, "goal-history", fmt.Sprintf("%04d.yaml", prev.Version))
		if err := fsutil.WriteFileAtomic(hist, data, 0o644); err != nil {
			return next, err
		}
	}
	data, err := yaml.Marshal(next)
	if err != nil {
		return next, err
	}
	return next, fsutil.WriteFileAtomic(goalPath(root, id), data, 0o644)
}

// checkServes rejects serves that name no focus of the goal card.
func checkServes(root, id string, o Outline) error {
	var goal StoredGoal
	var ok bool
	for _, n := range o.Nodes {
		if len(n.Serves) == 0 {
			continue
		}
		if goal.Outcome == nil {
			g, found, err := LoadGoal(root, id)
			if err != nil {
				return err
			}
			goal, ok = g, found
		}
		for _, f := range n.Serves {
			if !ok || !goal.HasFocus(f) {
				return fmt.Errorf("outline entry %s serves %q, which is not a focus of the goal card", n.ID, f)
			}
		}
	}
	return nil
}

// Coverage reports goal card focus points no entry serves.
func Coverage(o Outline, goal GoalCard) []FocusItem {
	served := map[string]bool{}
	for _, n := range o.Nodes {
		for _, f := range n.Serves {
			served[f] = true
		}
	}
	var out []FocusItem
	for _, f := range goal.Focus {
		if !served[f.ID] {
			out = append(out, f)
		}
	}
	return out
}

// ReadyToConfirm gates goal curricula: a goal card, a why for every leaf
// entry, and every focus point served (CR-2026-045).
func ReadyToConfirm(root, id string, o Outline) error {
	m, err := LoadManifest(root, id)
	if err != nil || m.Kind != "goal" {
		return nil
	}
	goal, ok, err := LoadGoal(root, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("curriculum %s has no goal card; run the intake interview first", id)
	}
	var missing []string
	for _, n := range o.Nodes {
		if !o.hasChildren(n.ID) && strings.TrimSpace(n.Why) == "" {
			missing = append(missing, "entry "+n.ID+" has no why")
		}
	}
	for _, f := range Coverage(o, goal.GoalCard) {
		missing = append(missing, "no entry serves focus "+f.ID+" ("+f.Text+")")
	}
	if len(missing) > 0 {
		return fmt.Errorf("the outline is not ready to confirm: %s; see learn curriculum outline review", strings.Join(missing, "; "))
	}
	return nil
}
