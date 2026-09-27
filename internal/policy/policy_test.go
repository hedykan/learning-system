package policy_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/channelwill/learning-os/internal/conversation"
	"github.com/channelwill/learning-os/internal/curriculum"
	"github.com/channelwill/learning-os/internal/learner"
	"github.com/channelwill/learning-os/internal/policy"
	"github.com/channelwill/learning-os/internal/record"
)

// Every session uses the same script: assistant asks, learner answers twice.
type script struct{}

func (script) Turn(session, turn string) (conversation.Turn, error) {
	n, ok := conversation.Ordinal(turn)
	texts := []string{"assistant|请预测结果", "user|我猜平均值更重要", "assistant|看这个反例", "user|原来尾部慢请求更关键"}
	if !ok || n > len(texts) {
		return conversation.Turn{}, fmt.Errorf("no turn %s", turn)
	}
	role, text, _ := strings.Cut(texts[n-1], "|")
	return conversation.Turn{ID: turn, Ordinal: n, Role: role, Text: text}, nil
}

func sid(i int) string { return fmt.Sprintf("session-202609%02d-100000.000000000", i) }

func ev(turn string) []record.Evidence {
	q := map[string]string{"t0002": "平均值更重要", "t0004": "尾部慢请求更关键"}[turn]
	return []record.Evidence{{Turn: turn, Quote: q}}
}

var tail = record.Concept{ID: "tail", Label: "尾延迟", SourceRef: &record.SourceRef{Chapter: "1"}}

func build(t *testing.T, recs ...record.Record) *learner.Model {
	t.Helper()
	var envs []record.Envelope
	for i, r := range recs {
		r.Schema = record.Schema
		if r.Curriculum == "" {
			r.Curriculum = "ddia"
		}
		envs = append(envs, record.Envelope{Session: sid(i + 1), Seq: 1, Kind: "checkpoint", SubmittedAt: sid(i + 1), Record: r})
	}
	m, err := learner.Replay(envs, script{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func ctx(active string) policy.Context {
	return policy.Context{Curriculum: "ddia", Assessed: true, ActiveSession: active,
		Position: curriculum.Position{Chapter: "1", CurrentConcept: "尾延迟"}}
}

func misconception(id string) record.Record {
	return record.Record{Concepts: []record.Concept{tail},
		Events: []record.Event{{ID: id, Type: "misconception", Concept: "tail", Summary: "只看平均值", Evidence: ev("t0002")}}}
}

func attempt(id, strategy, outcome string, withChange bool) record.Record {
	r := record.Record{Concepts: []record.Concept{tail}}
	a := record.StrategyAttempt{ID: id, Strategy: strategy, Situation: "misconception", Concept: "tail", ActionTurn: "t0003",
		ExpectedChange: "修正", Outcome: outcome, Evidence: ev("t0004")}
	if withChange {
		r.CognitiveChanges = []record.CognitiveChange{{ID: "c" + id, Concept: "tail", OldModel: "平均", Trigger: "反例", TriggerTurn: "t0003",
			NewModel: "尾部", OldEvidence: ev("t0002"), NewEvidence: ev("t0004")}}
		a.Linked = []string{"c" + id}
	} else {
		a.Reason = "没有变化"
	}
	r.StrategyAttempts = []record.StrategyAttempt{a}
	return r
}

func TestRules(t *testing.T) {
	empty := learner.New()
	c := ctx("")
	c.Assessed = false
	if got := policy.Next(empty, c); got.Rule != "R0-no-baseline" {
		t.Fatalf("R0: %+v", got)
	}
	c.BaselineSkipped = true
	if got := policy.Next(empty, c); got.Rule == "R0-no-baseline" {
		t.Fatalf("explicit skip ignored: %+v", got)
	}
	if got := policy.Next(empty, ctx("")); got.Rule != "R5-unobserved" || got.Strategy != "prediction_probe" || got.HistoryUsed {
		t.Fatalf("R5: %+v", got)
	}
	c = ctx("")
	c.Position.CurrentConcept = ""
	if got := policy.Next(empty, c); got.Rule != "R6-continue" || got.CurriculumRelation != "mainline" {
		t.Fatalf("R6: %+v", got)
	}
	m := build(t, misconception("e1"))
	if got := policy.Next(m, ctx(sid(2))); got.Rule != "R2-open-misconception" || got.Strategy != "counterexample" || !got.HistoryUsed {
		t.Fatalf("R2: %+v", got)
	}
	developing := record.Record{Concepts: []record.Concept{tail}, StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail",
		State: "developing", Capabilities: []string{"explained"}, Summary: "能解释", Evidence: ev("t0004")}}}
	if got := policy.Next(build(t, developing), ctx("")); got.Action != "application_probe" || got.Rule != "R4-missing-transfer" {
		t.Fatalf("R4: %+v", got)
	}
	fragile := attempt("s1", "analogy", "ineffective", false)
	fragile.StateUpdates = []record.StateUpdate{{ID: "u1", Concept: "tail", State: "fragile", Capabilities: []string{"explained"}, Summary: "动摇", Evidence: ev("t0004")}}
	if got := policy.Next(build(t, fragile), ctx("")); got.Rule != "R3-fragile" || got.Strategy != "retrieval_practice" {
		t.Fatalf("R3: %+v", got)
	}
}

func TestDetourReturn(t *testing.T) {
	tx := record.Concept{ID: "transaction", Label: "事务", SourceRef: &record.SourceRef{Chapter: "7"}}
	m := build(t, record.Record{Concepts: []record.Concept{tx, tail}, StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "transaction",
		State: "developing", Capabilities: []string{"explained"}, Summary: "能解释", Evidence: ev("t0004")}}})
	c := ctx("")
	c.Position.Detour = &curriculum.Detour{Topic: "事务", Concept: "transaction", ReturnTo: curriculum.ReturnPoint{Chapter: "1", Concept: "尾延迟"}}
	got := policy.Next(m, c)
	if got.Rule != "R1-detour-resolved" || got.Concept != "tail" || got.CurriculumRelation != "return" {
		t.Fatalf("R1: %+v", got)
	}
}

func TestStrategyFromEvidenceAndExclusion(t *testing.T) {
	m := build(t, attempt("s1", "analogy", "effective", true), attempt("s2", "analogy", "effective", true), misconception("e9"))
	got := policy.Next(m, ctx(sid(3)))
	if got.Strategy != "analogy" || !got.HistoryUsed {
		t.Fatalf("score choice: %+v", got)
	}
	m = build(t, attempt("s1", "counterexample", "ineffective", false), attempt("s2", "counterexample", "ineffective", false), misconception("e9"))
	got = policy.Next(m, ctx(sid(3)))
	if got.Strategy == "counterexample" || len(got.Evidence) < 3 {
		t.Fatalf("exclusion: %+v", got)
	}
}

func TestSupportedPatternWins(t *testing.T) {
	obs := func(r record.Record, id string) record.Record {
		r.PatternObservations = []record.PatternObservation{{ID: id, Pattern: "concrete-first", Description: "先具体后抽象",
			PreferredStrategy: "learner_action", Situation: "misconception", Stance: "supports", Summary: "x",
			Linked: []string{"c" + r.StrategyAttempts[0].ID}, Evidence: ev("t0004")}}
		return r
	}
	a := obs(attempt("s1", "analogy", "effective", true), "p1")
	b := obs(attempt("s2", "analogy", "effective", true), "p2")
	b.Curriculum = "calculus"
	m := build(t, a, b, misconception("e9"))
	if got := policy.Next(m, ctx(sid(3))); got.Strategy != "learner_action" {
		t.Fatalf("pattern: %+v", got)
	}
}

func TestConsolidationBeforeAdvance(t *testing.T) {
	transferred := record.Record{Concepts: []record.Concept{tail}, StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail",
		State: "developing", Capabilities: []string{"explained", "transferred"}, Summary: "迁移到批处理", Evidence: ev("t0004")}}}
	m := build(t, transferred)
	c := ctx(sid(2))
	c.Position.CurrentConcept = "应对负载增长"
	got := policy.Next(m, c)
	if got.Rule != "R4b-consolidate" || got.Action != "retrieval_probe" || got.Concept != "tail" || !got.HistoryUsed {
		t.Fatalf("R4b: %+v", got)
	}
	if same := policy.Next(m, ctx(sid(1))); same.Rule == "R4b-consolidate" {
		t.Fatalf("R4b fired in the session that produced the evidence: %+v", same)
	}
	retrieved := record.Record{StrategyAttempts: []record.StrategyAttempt{{ID: "s9", Strategy: "retrieval_practice", Situation: "retrieval",
		Concept: "tail", ActionTurn: "t0003", ExpectedChange: "独立回忆", Outcome: "effective", Linked: []string{"e9"}, Evidence: ev("t0004")}},
		Events: []record.Event{{ID: "e9", Type: "retrieval", Concept: "tail", Summary: "独立回忆", Evidence: ev("t0004")}}}
	m = build(t, transferred, retrieved)
	if got := policy.Next(m, c); got.Rule == "R4b-consolidate" {
		t.Fatalf("R4b fired twice in one session: %+v", got)
	}
}

func TestChapterScopeByNodeAndLabel(t *testing.T) {
	byNode := record.Concept{ID: "tail", Label: "尾延迟", SourceRef: &record.SourceRef{Node: "2.2", Chapter: "2 定义非功能性需求"}}
	m := build(t, record.Record{Concepts: []record.Concept{byNode},
		Events: []record.Event{{ID: "e1", Type: "misconception", Concept: "tail", Summary: "只看平均值", Evidence: ev("t0002")}}})
	c := policy.Context{Curriculum: "ddia", Assessed: true, Position: curriculum.Position{Node: "2.3", Chapter: "2 定义非功能性需求", CurrentConcept: "伸缩"}}
	if got := policy.Next(m, c); got.Rule != "R2-open-misconception" {
		t.Fatalf("node prefix scope: %+v", got)
	}
	c.Position = curriculum.Position{Node: "3.1", Chapter: "3 数据模型", CurrentConcept: "伸缩"}
	if got := policy.Next(m, c); got.Rule == "R2-open-misconception" {
		t.Fatalf("other chapter matched: %+v", got)
	}
	short := record.Concept{ID: "tail", Label: "尾延迟", SourceRef: &record.SourceRef{Chapter: "第1章"}}
	m = build(t, record.Record{Concepts: []record.Concept{short},
		Events: []record.Event{{ID: "e1", Type: "misconception", Concept: "tail", Summary: "只看平均值", Evidence: ev("t0002")}}})
	c.Position = curriculum.Position{Chapter: "第1章 可靠、可扩展与可维护的应用", CurrentConcept: "伸缩"}
	if got := policy.Next(m, c); got.Rule != "R2-open-misconception" {
		t.Fatalf("first-word chapter match: %+v", got)
	}
}

func TestContinueNamesNextOutlineNode(t *testing.T) {
	c := ctx("")
	c.Position.CurrentConcept = ""
	c.NextNode = &curriculum.Node{ID: "2.4", Title: "可维护性"}
	got := policy.Next(learner.New(), c)
	if got.Rule != "R6-continue" || got.Node != "2.4" || !strings.Contains(got.Reason, "可维护性") {
		t.Fatalf("R6 node: %+v", got)
	}
}

func TestReviewDueRule(t *testing.T) {
	developing := record.Record{Concepts: []record.Concept{tail}, StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail",
		State: "developing", Capabilities: []string{"explained"}, Summary: "能解释", Evidence: ev("t0004")}}}
	envs := []record.Envelope{{Session: sid(1), Seq: 1, Kind: "checkpoint", SubmittedAt: "2026-10-01T04:00:00Z",
		Record: func() record.Record { r := developing; r.Schema, r.Curriculum = record.Schema, "ddia"; return r }()}}
	m, err := learner.Replay(envs, script{})
	if err != nil {
		t.Fatal(err)
	}
	c := ctx(sid(2))
	c.Position.CurrentConcept = "别的概念"
	c.Today = "2026-10-01"
	if got := policy.Next(m, c); got.Rule == "R3b-review-due" {
		t.Fatalf("due on the learning day: %+v", got)
	}
	c.Today = "2026-10-03"
	if got := policy.Next(m, c); got.Rule != "R3b-review-due" || got.Action != "review_due" || got.Strategy != "retrieval_practice" {
		t.Fatalf("R3b: %+v", got)
	}
}
