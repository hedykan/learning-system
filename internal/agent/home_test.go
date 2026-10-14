package agent

import "testing"

func TestBuildHome(t *testing.T) {
	t.Run("empty vault", func(t *testing.T) {
		got := buildHome(homeInput{Review: map[string]any{}, Overview: map[string]any{}})
		if mmap(got["stats"])["courses"] != 0 || len(got["courses"].([]map[string]any)) != 0 || len(got["moments"].([]map[string]any)) != 0 {
			t.Fatalf("unexpected empty home: %#v", got)
		}
	})

	t.Run("one course two concepts", func(t *testing.T) {
		got := buildHome(homeInput{
			Active:  "math",
			Courses: []map[string]any{{"id": "math", "title": "数学", "meta": "1 / 3 节"}},
			Concepts: []homeConcept{
				homeFixtureConcept("limit", "极限", "developing", "math", nil, nil),
				homeFixtureConcept("derivative", "导数", "stable", "math", nil, nil),
			},
			Review:   map[string]any{"due": []any{map[string]any{"concept": "limit"}}},
			Overview: map[string]any{"section": "2 导数", "node": "2"},
		})
		course := got["courses"].([]map[string]any)[0]
		if course["done"] != 1 || course["total"] != 3 || len(course["concepts"].([]map[string]any)) != 2 || course["active"] != true {
			t.Fatalf("unexpected course: %#v", course)
		}
		if mmap(got["today"])["due"] != 1 || mmap(got["today"])["active_title"] != "数学" {
			t.Fatalf("unexpected today: %#v", got["today"])
		}
	})

	t.Run("cross course bridges moments fixes and superseded", func(t *testing.T) {
		mis := map[string]any{"id": "m1", "type": "misconception", "curriculum": "ddia", "session": "s1", "at": "2026-10-12T09:00:00Z", "concept": "lag", "summary": "复制没有延迟", "evidence": []any{map[string]any{"quote": "主库成功就全成功"}}}
		fix := map[string]any{"id": "f1", "type": "correction", "curriculum": "ddia", "session": "s2", "at": "2026-10-13T09:00:00Z", "concept": "lag", "summary": "异步复制有延迟"}
		transfer := map[string]any{"id": "t1", "type": "transfer", "curriculum": "crm", "session": "s3", "at": "2026-10-14T09:00:00Z", "concept": "lag", "summary": "迁移到会员同步"}
		supersededEvent := map[string]any{"id": "old", "type": "insight", "curriculum": "ddia", "at": "2026-10-15T09:00:00Z", "concept": "lag", "summary": "旧记录", "superseded": "new"}
		revision := map[string]any{"id": "r1", "type": "understanding_revision", "curriculum": "crm", "session": "s4", "at": "2026-10-14T10:00:00Z", "concept": "member", "summary": "按批次同步 → 按版本同步", "derived_by": "c1"}
		got := buildHome(homeInput{
			Active:  "crm",
			Courses: []map[string]any{{"id": "ddia", "title": "DDIA", "meta": "1 / 6 节"}, {"id": "crm", "title": "CRM", "meta": "1 / 4 节"}},
			Concepts: []homeConcept{
				homeFixtureConcept("lag", "复制延迟", "fragile", "ddia", []any{map[string]any{"concept": "member"}}, []any{mis, fix, transfer, supersededEvent}),
				{
					List: map[string]any{"id": "member", "label": "会员同步", "state": "developing"},
					Detail: map[string]any{"concept": map[string]any{"source_refs": []any{map[string]any{"curriculum": "crm"}}, "related": []any{map[string]any{"concept": "lag"}}},
						"events": []any{revision}, "cognitive_changes": []any{map[string]any{"id": "c1", "concept": "member", "session": "s4", "old_model": "按批次同步"}}},
				},
			},
			Review: map[string]any{"due": []any{}}, Overview: map[string]any{},
		})
		if len(got["bridges"].([]map[string]any)) != 2 {
			t.Fatalf("want related + transfer bridges, got %#v", got["bridges"])
		}
		moments := got["moments"].([]map[string]any)
		if len(moments) != 4 {
			t.Fatalf("superseded moment was not excluded: %#v", moments)
		}
		if moments[0]["old"] != "按批次同步" {
			t.Fatalf("revision old model missing: %#v", moments[0])
		}
		stats := mmap(got["stats"])
		if stats["misconceptions_fixed"] != 1 || stats["days"] != 3 {
			t.Fatalf("unexpected stats: %#v", stats)
		}
	})
}

func homeFixtureConcept(id, label, state, curriculum string, related, events []any) homeConcept {
	if related == nil {
		related = []any{}
	}
	if events == nil {
		events = []any{}
	}
	return homeConcept{
		List: map[string]any{"id": id, "label": label, "state": state},
		Detail: map[string]any{
			"concept": map[string]any{"source_refs": []any{map[string]any{"curriculum": curriculum}}, "related": related},
			"events":  events, "cognitive_changes": []any{},
		},
	}
}
