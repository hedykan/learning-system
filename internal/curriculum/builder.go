package curriculum

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Curriculum types (CR-2026-038).
const (
	SourceAligned = "source_aligned"
	Synthesized   = "synthesized"
)

// TypeOf is the outline's type, source_aligned when unset.
func (o Outline) TypeOf() string {
	if o.Type == "" {
		return SourceAligned
	}
	return o.Type
}

var conceptIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validateTopics checks topic metadata: bounded why, prerequisites that
// exist, are not the entry's own ancestors or descendants and form no
// cycle, and concept ids in kebab-case.
func (o Outline) validateTopics() error {
	switch o.Type {
	case "", SourceAligned, Synthesized:
	default:
		return fmt.Errorf("outline type %q must be source_aligned or synthesized", o.Type)
	}
	byID := map[string]Node{}
	for _, n := range o.Nodes {
		byID[n.ID] = n
	}
	for _, n := range o.Nodes {
		where := "outline entry " + n.ID
		if utf8.RuneCountInString(n.Why) > 120 {
			return fmt.Errorf("%s: why must be at most 120 characters", where)
		}
		for _, p := range n.Prerequisites {
			if _, ok := byID[p]; !ok {
				return fmt.Errorf("%s: prerequisite %s is not in the outline", where, p)
			}
			if p == n.ID || strings.HasPrefix(n.ID, p+".") || strings.HasPrefix(p, n.ID+".") {
				return fmt.Errorf("%s: prerequisite %s is the entry itself, its parent or its child", where, p)
			}
		}
		for _, c := range n.Concepts {
			if !conceptIDPattern.MatchString(c) {
				return fmt.Errorf("%s: concept id %q must be kebab-case", where, c)
			}
		}
	}
	state := map[string]int{}
	var path []string
	var visit func(id string) error
	visit = func(id string) error {
		state[id] = 1
		path = append(path, id)
		for _, p := range byID[id].Prerequisites {
			switch state[p] {
			case 1:
				return fmt.Errorf("outline prerequisites form a cycle: %s → %s", strings.Join(path, " → "), p)
			case 0:
				if err := visit(p); err != nil {
					return err
				}
			}
		}
		path = path[:len(path)-1]
		state[id] = 2
		return nil
	}
	for _, n := range o.Nodes {
		if state[n.ID] == 0 {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// BlockedBy lists an entry's prerequisites that are neither completed nor
// skipped. A prerequisite with children counts as done when all its leaf
// entries are.
func BlockedBy(n Node, statuses []NodeStatus) []string {
	done := func(id string) bool {
		leaves, finished := 0, 0
		for _, s := range statuses {
			if s.ID != id && !strings.HasPrefix(s.ID, id+".") {
				continue
			}
			if hasChildStatus(statuses, s.ID) {
				continue
			}
			leaves++
			if s.Status == "completed" || s.Status == "skipped" {
				finished++
			}
		}
		return leaves > 0 && leaves == finished
	}
	var out []string
	for _, p := range n.Prerequisites {
		if !done(p) {
			out = append(out, p)
		}
	}
	return out
}

func hasChildStatus(statuses []NodeStatus, id string) bool {
	for _, s := range statuses {
		if strings.HasPrefix(s.ID, id+".") {
			return true
		}
	}
	return false
}
