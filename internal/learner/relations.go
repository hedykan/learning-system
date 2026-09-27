package learner

import "sort"

// TouchedIn lists concepts that gained evidence in a session.
func (m *Model) TouchedIn(session string) []string {
	seen := map[string]bool{}
	for _, e := range m.Events {
		if e.Session == session && e.Concept != "" {
			seen[e.Concept] = true
		}
	}
	for _, ch := range m.Changes {
		if ch.Session == session {
			seen[ch.Concept] = true
		}
	}
	for _, a := range m.Attempts {
		if a.Session == session {
			seen[a.Concept] = true
		}
	}
	for _, r := range m.Reviews {
		if r.Session == session {
			seen[r.Concept] = true
		}
	}
	for _, c := range m.Concepts {
		for _, h := range c.History {
			if h.Session == session {
				seen[c.ID] = true
			}
		}
	}
	return sortedKeys(seen)
}

// UnlinkedIn lists concepts touched in a session that have no relation and
// were never declared unrelated, while other concepts of the same curriculum
// exist that they could be linked to. Candidates maps each to those others.
func (m *Model) UnlinkedIn(session, curriculum string) (missing []string, candidates map[string][]string) {
	candidates = map[string][]string{}
	studied := map[string]bool{}
	for sid, info := range m.Sessions {
		if info.Curriculum == curriculum {
			for _, id := range m.TouchedIn(sid) {
				studied[id] = true
			}
		}
	}
	inBook := func(c *Concept) bool { return c.InChapter(curriculum, "", "") || studied[c.ID] }
	var pool []string
	for _, c := range m.ConceptList() {
		if inBook(c) {
			pool = append(pool, c.ID)
		}
	}
	for _, id := range m.TouchedIn(session) {
		c := m.Concepts[id]
		if c == nil || len(c.Related) > 0 || c.NoRelated || !inBook(c) {
			continue
		}
		var others []string
		for _, o := range pool {
			if o != id {
				others = append(others, o)
			}
		}
		if len(others) == 0 {
			continue
		}
		sort.Strings(others)
		missing = append(missing, id)
		candidates[id] = others
	}
	return missing, candidates
}
