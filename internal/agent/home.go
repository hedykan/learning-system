package agent

import (
	"fmt"
	"sort"
	"strings"
)

type homeConcept struct {
	List   map[string]any
	Detail map[string]any
}

type homeInput struct {
	Active   string
	Courses  []map[string]any
	Concepts []homeConcept
	Review   map[string]any
	Overview map[string]any
}

// Home is the cross-course growth summary rendered by the islands home page.
func (s *Server) Home() map[string]any {
	courseView := s.Courses()
	courses := homeMaps(courseView["courses"])
	state := learnJSON(s.Vault, "state")
	concepts := []homeConcept{}
	for _, raw := range asSlice(state["concepts"]) {
		m := mmap(raw)
		st, ok := stateMap[mstr(m, "state")]
		if !ok {
			continue
		}
		list := map[string]any{"id": mstr(m, "id"), "label": mstr(m, "label"), "state": st}
		concepts = append(concepts, homeConcept{List: list, Detail: learnJSON(s.Vault, "state", "concept", mstr(m, "id"))})
	}
	return buildHome(homeInput{
		Active: mstr(courseView, "active"), Courses: courses, Concepts: concepts,
		Review: s.Due(), Overview: s.Overview(),
	})
}

func homeMaps(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, mmap(row))
		}
		return out
	default:
		return []map[string]any{}
	}
}

func buildHome(in homeInput) map[string]any {
	courseIDs, titles, order := map[string]bool{}, map[string]string{}, map[string]int{}
	for i, c := range in.Courses {
		id := mstr(c, "id")
		courseIDs[id], titles[id], order[id] = true, mstr(c, "title", "id"), i
	}

	owner := map[string]string{}
	byID := map[string]homeConcept{}
	validEvents := map[string][]map[string]any{}
	for _, hc := range in.Concepts {
		id := mstr(hc.List, "id")
		byID[id] = hc
		events := []map[string]any{}
		counts := map[string]int{}
		for _, raw := range asSlice(hc.Detail["events"]) {
			e := mmap(raw)
			if superseded(e["superseded"]) {
				continue
			}
			events = append(events, e)
			if cur := mstr(e, "curriculum"); courseIDs[cur] {
				counts[cur]++
			}
		}
		validEvents[id] = events
		best, bestN, bestOrder := "", 0, len(in.Courses)+1
		for cur, n := range counts {
			if n > bestN || (n == bestN && order[cur] < bestOrder) {
				best, bestN, bestOrder = cur, n, order[cur]
			}
		}
		if best == "" {
			concept := mmap(hc.Detail["concept"])
			if refs := asSlice(concept["source_refs"]); len(refs) > 0 {
				candidate := mstr(mmap(refs[0]), "curriculum")
				if courseIDs[candidate] {
					best = candidate
				}
			}
		}
		owner[id] = best
	}

	courseConcepts := map[string][]map[string]any{}
	unassigned := []map[string]any{}
	for _, hc := range in.Concepts {
		id := mstr(hc.List, "id")
		if cur := owner[id]; cur != "" {
			courseConcepts[cur] = append(courseConcepts[cur], hc.List)
		} else {
			unassigned = append(unassigned, hc.List)
		}
	}

	courses := []map[string]any{}
	for _, c := range in.Courses {
		id := mstr(c, "id")
		done, total := 0, 0
		fmt.Sscanf(mstr(c, "meta"), "%d / %d", &done, &total)
		concepts := courseConcepts[id]
		if concepts == nil {
			concepts = []map[string]any{}
		}
		courses = append(courses, map[string]any{
			"id": id, "title": titles[id], "meta": mstr(c, "meta"), "done": done, "total": total,
			"active": id == in.Active, "concepts": concepts,
		})
	}

	bridges := []map[string]any{}
	bridgeSeen := map[string]bool{}
	for id, hc := range byID {
		concept := mmap(hc.Detail["concept"])
		for _, raw := range asSlice(concept["related"]) {
			other := mstr(mmap(raw), "concept")
			if owner[id] == "" || owner[other] == "" || owner[id] == owner[other] {
				continue
			}
			a, b := id, other
			if a > b {
				a, b = b, a
			}
			key := "related:" + a + ":" + b
			if !bridgeSeen[key] {
				bridgeSeen[key] = true
				bridges = append(bridges, map[string]any{"a": a, "b": b, "kind": "related"})
			}
		}
		for _, e := range validEvents[id] {
			typ, cur := mstr(e, "type"), mstr(e, "curriculum")
			if (typ == "transfer" || typ == "application") && owner[id] != "" && cur != "" && cur != owner[id] && courseIDs[cur] {
				key := "transfer:" + id + ":" + cur
				if !bridgeSeen[key] {
					bridgeSeen[key] = true
					bridges = append(bridges, map[string]any{"a": id, "b": "", "course_b": cur, "kind": "transfer"})
				}
			}
		}
	}

	momentTypes := map[string]bool{"misconception": true, "prerequisite_gap": true, "correction": true, "insight": true, "understanding_revision": true, "transfer": true, "application": true}
	moments := []map[string]any{}
	days := map[string]bool{}
	fixed := 0
	for id, hc := range byID {
		events := validEvents[id]
		changes := asSlice(hc.Detail["cognitive_changes"])
		for i, e := range events {
			at := mstr(e, "at")
			if len(at) >= 10 {
				days[at[:10]] = true
			}
			if mstr(e, "type") == "misconception" {
				for _, later := range events[i+1:] {
					if typ := mstr(later, "type"); typ == "correction" || typ == "understanding_revision" {
						fixed++
						break
					}
				}
			}
			typ := mstr(e, "type")
			if !momentTypes[typ] {
				continue
			}
			cur := mstr(e, "curriculum")
			quote := ""
			for _, raw := range asSlice(e["evidence"]) {
				if quote = mstr(mmap(raw), "quote"); quote != "" {
					break
				}
			}
			old := ""
			if typ == "understanding_revision" {
				derived := mstr(e, "derived_by")
				for _, raw := range changes {
					ch := mmap(raw)
					if (derived != "" && mstr(ch, "id") == derived) || (mstr(ch, "concept") == id && mstr(ch, "session") == mstr(e, "session")) {
						old = mstr(ch, "old_model")
						break
					}
				}
				if old == "" {
					old = strings.TrimSpace(strings.SplitN(mstr(e, "summary"), "→", 2)[0])
				}
			}
			moments = append(moments, map[string]any{
				"type": typ, "at": at, "concept": id, "concept_label": mstr(hc.List, "label"),
				"course": cur, "course_title": titles[cur], "quote": quote, "summary": mstr(e, "summary"), "old": old,
			})
		}
	}
	sort.SliceStable(moments, func(i, j int) bool { return mstr(moments[i], "at") > mstr(moments[j], "at") })
	if len(moments) > 20 {
		moments = moments[:20]
	}

	byState := map[string]int{"fragile": 0, "developing": 0, "stable": 0}
	for _, hc := range in.Concepts {
		byState[mstr(hc.List, "state")]++
	}
	activeTitle := titles[in.Active]
	return map[string]any{
		"stats":   map[string]any{"courses": len(in.Courses), "concepts": len(in.Concepts), "by_state": byState, "misconceptions_fixed": fixed, "days": len(days)},
		"courses": courses, "unassigned": unassigned, "bridges": bridges, "moments": moments,
		"today": map[string]any{"due": len(asSlice(in.Review["due"])), "active_course": in.Active, "active_title": activeTitle,
			"section": in.Overview["section"], "node": in.Overview["node"]},
	}
}

func superseded(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case bool:
		return x
	case []any:
		return len(x) > 0
	default:
		return true
	}
}
