package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

// ProposalSpec is a suggested outline change (CR-2026-041), as carried by
// an Interpretation Record.
type ProposalSpec struct {
	ID            string
	Action        string
	Node          string
	Title         string
	Why           string
	Prerequisites []string
	Reason        string
}

// Decision is the learner's answer to a proposal.
type Decision struct {
	Decision string `yaml:"decision" json:"decision"` // accepted or rejected
	At       string `yaml:"at" json:"at"`
	Reason   string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Version  int    `yaml:"outline_version,omitempty" json:"outline_version,omitempty"`
}

// allowed lists the actions each curriculum type accepts (CR-2026-038).
var allowed = map[string]map[string]bool{
	SourceAligned: {"skip": true, "mark_known": true},
	Synthesized:   {"skip": true, "mark_known": true, "insert": true, "remove": true, "retitle": true},
}

// CheckProposal validates a proposal against the curriculum as it is now.
func CheckProposal(root, id string, p ProposalSpec) error {
	o, err := LoadOutline(root, id)
	if err != nil {
		return err
	}
	what := "curriculum proposal " + p.ID
	if o.Status != "confirmed" {
		return fmt.Errorf("%s: the outline of %s is not confirmed yet; change the draft directly", what, id)
	}
	if !allowed[Synthesized][p.Action] {
		return fmt.Errorf("%s: action must be skip, mark_known, insert, remove or retitle", what)
	}
	if !allowed[o.TypeOf()][p.Action] {
		return fmt.Errorf("%s: a %s curriculum follows its material, so %s is not allowed (only skip and mark_known)", what, o.TypeOf(), p.Action)
	}
	_, exists := o.Find(p.Node)
	switch p.Action {
	case "insert":
		if exists {
			return fmt.Errorf("%s: entry %s already exists; choose a free id (e.g. a sub-entry %s.1)", what, p.Node, p.Node)
		}
		next, err := applyProposal(o, p)
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		return next.Validate()
	case "remove":
		if !exists {
			return fmt.Errorf("%s: outline has no entry %s", what, p.Node)
		}
		if o.hasChildren(p.Node) {
			return fmt.Errorf("%s: entry %s has sub-entries; remove those first", what, p.Node)
		}
		entries, err := LoadProgress(root, id)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Node == p.Node && e.Kind == "completed" {
				return fmt.Errorf("%s: entry %s is already completed; keep it", what, p.Node)
			}
		}
	default:
		if !exists {
			return fmt.Errorf("%s: outline has no entry %s", what, p.Node)
		}
	}
	return nil
}

// applyProposal returns the outline with an insert, remove or retitle done.
func applyProposal(o Outline, p ProposalSpec) (Outline, error) {
	next := o
	next.Nodes = append([]Node(nil), o.Nodes...)
	switch p.Action {
	case "insert":
		next.Nodes = append(next.Nodes, Node{ID: p.Node, Title: p.Title, Why: p.Why, Prerequisites: p.Prerequisites})
		sort.SliceStable(next.Nodes, func(i, j int) bool { return lessID(next.Nodes[i].ID, next.Nodes[j].ID) })
	case "remove":
		kept := next.Nodes[:0]
		for _, n := range next.Nodes {
			if n.ID == p.Node {
				continue
			}
			var pre []string
			for _, q := range n.Prerequisites {
				if q != p.Node {
					pre = append(pre, q)
				}
			}
			n.Prerequisites = pre
			kept = append(kept, n)
		}
		next.Nodes = kept
	case "retitle":
		for i := range next.Nodes {
			if next.Nodes[i].ID == p.Node {
				next.Nodes[i].Title = p.Title
			}
		}
	}
	return next, nil
}

func decisionsPath(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "proposals.yaml")
}

// LoadDecisions reads the learner's decisions on proposals.
func LoadDecisions(root, id string) (map[string]Decision, error) {
	out := map[string]Decision{}
	data, err := os.ReadFile(decisionsPath(root, id))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return out, yaml.Unmarshal(data, &out)
}

func saveDecisions(root, id string, d map[string]Decision) error {
	data, err := yaml.Marshal(d)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(decisionsPath(root, id), data, 0o644)
}

// HistoryEntry is one earlier outline kept when a proposal changed it.
type HistoryEntry struct {
	Version  int     `yaml:"version" json:"version"`
	At       string  `yaml:"at" json:"at"`
	Proposal string  `yaml:"proposal" json:"proposal"`
	Reason   string  `yaml:"reason" json:"reason"`
	Outline  Outline `yaml:"outline" json:"outline"`
}

func historyDir(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "outline-history")
}

// OutlineHistory lists the outlines replaced by accepted proposals.
func OutlineHistory(root, id string) ([]HistoryEntry, error) {
	entries, err := os.ReadDir(historyDir(root, id))
	if os.IsNotExist(err) {
		return []HistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []HistoryEntry{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(historyDir(root, id), e.Name()))
		if err != nil {
			return nil, err
		}
		var h HistoryEntry
		if err := yaml.Unmarshal(data, &h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Accept applies a proposal the learner agreed to. skip and mark_known are
// recorded as progress; the other actions change the outline, whose previous
// version is kept in outline-history.
func Accept(root, id string, p ProposalSpec, session string, now time.Time) (Decision, error) {
	decisions, err := LoadDecisions(root, id)
	if err != nil {
		return Decision{}, err
	}
	if d, ok := decisions[p.ID]; ok {
		return d, fmt.Errorf("proposal %s was already %s", p.ID, d.Decision)
	}
	if err := CheckProposal(root, id, p); err != nil {
		return Decision{}, err
	}
	d := Decision{Decision: "accepted", At: now.UTC().Format(time.RFC3339)}
	switch p.Action {
	case "skip":
		if _, err := Mark(root, id, p.Node, "skipped", p.Reason, session, now); err != nil {
			return d, err
		}
	case "mark_known":
		if _, err := Mark(root, id, p.Node, "completed", "已掌握："+p.Reason, session, now); err != nil {
			return d, err
		}
	default:
		o, err := LoadOutline(root, id)
		if err != nil {
			return d, err
		}
		history, err := OutlineHistory(root, id)
		if err != nil {
			return d, err
		}
		d.Version = len(history) + 1
		h := HistoryEntry{Version: d.Version, At: d.At, Proposal: p.ID, Reason: p.Reason, Outline: o}
		data, err := yaml.Marshal(h)
		if err != nil {
			return d, err
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(historyDir(root, id), fmt.Sprintf("%04d.yaml", d.Version)), data, 0o644); err != nil {
			return d, err
		}
		next, err := applyProposal(o, p)
		if err != nil {
			return d, err
		}
		if err := saveOutline(root, id, next); err != nil {
			return d, err
		}
	}
	decisions[p.ID] = d
	if err := saveDecisions(root, id, decisions); err != nil {
		return d, err
	}
	if p.Action == "skip" || p.Action == "mark_known" {
		if _, err := AdvanceIfCurrent(root, id, p.Node); err != nil {
			return d, err
		}
	}
	return d, RenderProgress(root, id)
}

// Reject records that the learner declined a proposal.
func Reject(root, id, proposal, reason string, now time.Time) (Decision, error) {
	decisions, err := LoadDecisions(root, id)
	if err != nil {
		return Decision{}, err
	}
	if d, ok := decisions[proposal]; ok {
		return d, fmt.Errorf("proposal %s was already %s", proposal, d.Decision)
	}
	if strings.TrimSpace(reason) == "" {
		return Decision{}, fmt.Errorf("--reason is required")
	}
	d := Decision{Decision: "rejected", At: now.UTC().Format(time.RFC3339), Reason: reason}
	decisions[proposal] = d
	return d, saveDecisions(root, id, decisions)
}
