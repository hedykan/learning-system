package learner_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/record"
)

// fakeResolver holds scripted conversations: session -> turns ("role|text").
type fakeResolver map[string][]string

func (f fakeResolver) Turn(session, turn string) (conversation.Turn, error) {
	n, ok := conversation.Ordinal(turn)
	turns := f[session]
	if !ok || n > len(turns) {
		return conversation.Turn{}, fmt.Errorf("turn %s missing in %s", turn, session)
	}
	role, text, _ := strings.Cut(turns[n-1], "|")
	return conversation.Turn{ID: turn, Ordinal: n, Role: role, Text: text}, nil
}

const s1, s2 = "session-20260901-100000.000000000", "session-20260905-100000.000000000"

var convs = fakeResolver{
	s1: {
		"assistant|两个系统哪个更能扛住？",
		"user|B 平均更快，所以 B 更好",
		"assistant|1% 请求等 8 秒意味着什么？",
		"user|不能只看平均值，还要看最慢的那批请求",
		"user|我懂了，原来是这样",
	},
	s2: {
		"assistant|还记得上次的尾延迟吗？换到分片系统呢？",
		"user|扇出请求时整体延迟取决于最慢的分片",
	},
}

func env(session string, seq int, kind string, rec record.Record) record.Envelope {
	rec.Schema = record.Schema
	if rec.Curriculum == "" {
		rec.Curriculum = "ddia"
	}
	return record.Envelope{Session: session, Seq: seq, Kind: kind, SubmittedAt: fmt.Sprintf("%s-%02d", session, seq), Record: rec}
}

func ev(turn, quote string) []record.Evidence { return []record.Evidence{{Turn: turn, Quote: quote}} }

var concept = record.Concept{ID: "tail-latency", Label: "尾延迟", Aliases: []string{"p99"}, SourceRef: &record.SourceRef{Chapter: "1"}}

func revision() record.Record {
	return record.Record{
		Concepts: []record.Concept{concept},
		Events:   []record.Event{{ID: "e1", Type: "misconception", Concept: "tail-latency", Summary: "只看平均值", Evidence: ev("t0002", "B 平均更快")}},
		CognitiveChanges: []record.CognitiveChange{{ID: "c1", Concept: "tail-latency", OldModel: "平均值低即好", Trigger: "慢请求规模",
			TriggerTurn: "t0003", NewModel: "要看尾部", OldEvidence: ev("t0002", "B 平均更快"), NewEvidence: ev("t0004", "不能只看平均值")}},
		StrategyAttempts: []record.StrategyAttempt{{ID: "s1", Strategy: "counterexample", Situation: "misconception", Concept: "tail-latency",
			ActionTurn: "t0003", ExpectedChange: "放弃只看平均值", Outcome: "effective", Linked: []string{"c1"}, Evidence: ev("t0004", "不能只看平均值")}},
		StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail-latency", State: "developing", Capabilities: []string{"explained"},
			Summary: "能解释平均值掩盖慢请求", Evidence: ev("t0004", "还要看最慢的那批请求")}},
	}
}

func replay(t *testing.T, envs ...record.Envelope) (*learner.Model, error) {
	t.Helper()
	return learner.Replay(envs, convs)
}

func TestRevisionRecordBuildsHistory(t *testing.T) {
	m, err := replay(t, env(s1, 1, "checkpoint", revision()))
	if err != nil {
		t.Fatal(err)
	}
	c := m.Concepts["tail-latency"]
	if c.State() != "developing" || len(m.Changes) != 1 {
		t.Fatalf("state=%s changes=%d", c.State(), len(m.Changes))
	}
	if got := m.UnresolvedMisconceptions("tail-latency"); len(got) != 0 {
		t.Fatalf("misconception should be resolved by the revision: %+v", got)
	}
	var derived bool
	for _, e := range m.Events {
		derived = derived || (e.Type == "understanding_revision" && e.DerivedBy == s1+":c1")
	}
	if !derived {
		t.Fatal("understanding_revision not derived from change")
	}
}

func TestEvidenceRules(t *testing.T) {
	cases := map[string]func(*record.Record){
		"assistant turn":   func(r *record.Record) { r.Events[0].Evidence = ev("t0001", "哪个更能扛住") },
		"missing turn":     func(r *record.Record) { r.Events[0].Evidence = ev("t0099", "B 平均更快") },
		"foreign quote":    func(r *record.Record) { r.Events[0].Evidence = ev("t0002", "A 平均更快") },
		"no evidence":      func(r *record.Record) { r.Events[0].Evidence = nil },
		"unknown concept":  func(r *record.Record) { r.Concepts = nil },
		"derived switch":   func(r *record.Record) { r.Events[0].Type = "strategy_switch" },
		"reversed change":  func(r *record.Record) { r.CognitiveChanges[0].NewEvidence = ev("t0002", "B 平均更快") },
		"trigger outside":  func(r *record.Record) { r.CognitiveChanges[0].TriggerTurn = "t0005" },
		"user action turn": func(r *record.Record) { r.StrategyAttempts[0].ActionTurn = "t0002" },
		"effective unlinked": func(r *record.Record) {
			r.StrategyAttempts[0].Linked = nil
		},
		"ineffective without reason": func(r *record.Record) {
			r.StrategyAttempts[0].Outcome = "ineffective"
		},
		"recognition only developing": func(r *record.Record) {
			r.StateUpdates[0].Capabilities = []string{"recognized"}
			r.StateUpdates[0].Evidence = ev("t0005", "我懂了，原来")
		},
		"solid": func(r *record.Record) { r.StateUpdates[0].State = "solid" },
		"stable same session": func(r *record.Record) {
			r.StateUpdates = append(r.StateUpdates, record.StateUpdate{ID: "u2", Concept: "tail-latency", State: "stable",
				Capabilities: []string{"retrieved"}, Summary: "x", Evidence: ev("t0004", "不能只看平均值")})
		},
		"progress in checkpoint": func(r *record.Record) {
			r.ProgressDecision = &record.ProgressDecision{Decision: "stay", Reason: "x"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := revision()
			mutate(&rec)
			_, err := replay(t, env(s1, 1, "checkpoint", rec))
			if err == nil {
				t.Fatal("expected rejection")
			}
			t.Log(err)
		})
	}
}

func TestDuplicateConceptNameRejected(t *testing.T) {
	rec := revision()
	rec.Concepts = append(rec.Concepts, record.Concept{ID: "p99-latency", Label: "P99"})
	if _, err := replay(t, env(s1, 1, "checkpoint", rec)); err == nil || !strings.Contains(err.Error(), "tail-latency") {
		t.Fatalf("err = %v", err)
	}
}

func TestResubmittedItemsAreIdempotentButConflictsFail(t *testing.T) {
	first := env(s1, 1, "checkpoint", revision())
	if _, err := replay(t, first, env(s1, 2, "checkpoint", revision())); err != nil {
		t.Fatalf("identical items should be skipped: %v", err)
	}
	changed := revision()
	changed.Events[0].Summary = "别的说法"
	if _, err := replay(t, first, env(s1, 2, "checkpoint", changed)); err == nil {
		t.Fatal("same id with different content accepted")
	}
}

func stableLater() record.Record {
	return record.Record{StateUpdates: []record.StateUpdate{{ID: "u2", Concept: "tail-latency", State: "stable",
		Capabilities: []string{"transferred"}, Summary: "迁移到分片扇出", Evidence: ev("t0002", "取决于最慢的分片")}}}
}

func TestStableThenCounterexampleToFragileKeepsHistory(t *testing.T) {
	later := stableLater()
	m, err := replay(t, env(s1, 1, "checkpoint", revision()), env(s2, 1, "checkpoint", later))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Concepts["tail-latency"].State(); got != "stable" {
		t.Fatalf("state = %s", got)
	}
	later.Events = []record.Event{{ID: "e9", Type: "misconception", Concept: "tail-latency", Summary: "扇出时误判", Evidence: ev("t0002", "取决于最慢的分片")}}
	later.StateUpdates = append(later.StateUpdates, record.StateUpdate{ID: "u3", Concept: "tail-latency", State: "fragile",
		Capabilities: []string{"explained"}, Summary: "反例下动摇", Evidence: ev("t0002", "取决于最慢的分片")})
	m, err = replay(t, env(s1, 1, "checkpoint", revision()), env(s2, 1, "checkpoint", later))
	if err != nil {
		t.Fatal(err)
	}
	c := m.Concepts["tail-latency"]
	if c.State() != "fragile" || len(c.History) != 3 || c.History[1].State != "stable" {
		t.Fatalf("history = %+v", c.History)
	}
}

func TestRetractionFallsBackToPreviousState(t *testing.T) {
	retract := record.Record{Retractions: []record.Retraction{{Ref: s2 + ":u2", Reason: "证据不足"}}}
	m, err := replay(t, env(s1, 1, "checkpoint", revision()), env(s2, 1, "checkpoint", stableLater()), env(s2, 2, "checkpoint", retract))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Concepts["tail-latency"].State(); got != "developing" {
		t.Fatalf("state after retraction = %s", got)
	}
	retractAll := record.Record{Retractions: []record.Retraction{{Ref: s1 + ":u1", Reason: "误判"}}}
	m, err = replay(t, env(s1, 1, "checkpoint", revision()), env(s1, 2, "checkpoint", retractAll))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Concepts["tail-latency"].State(); got != "unobserved" {
		t.Fatalf("state = %s", got)
	}
	bad := record.Record{Retractions: []record.Retraction{{Ref: s1 + ":c1#revision", Reason: "x"}}}
	if _, err := replay(t, env(s1, 1, "checkpoint", revision()), env(s1, 2, "checkpoint", bad)); err == nil {
		t.Fatal("retracting a derived event should fail")
	}
}

func TestStrategySwitchDerived(t *testing.T) {
	rec := revision()
	rec.StrategyAttempts[0].Outcome = "ineffective"
	rec.StrategyAttempts[0].Reason = "学习者仍坚持平均值"
	rec.StrategyAttempts[0].Linked = nil
	rec.StrategyAttempts = append(rec.StrategyAttempts, record.StrategyAttempt{ID: "s2", Strategy: "concrete_example", Situation: "misconception",
		Concept: "tail-latency", ActionTurn: "t0003", ExpectedChange: "看到慢请求", Outcome: "effective", Linked: []string{"c1"},
		Evidence: ev("t0004", "不能只看平均值"), Replaces: "s1", SwitchReason: "反例没有触发冲突"})
	m, err := replay(t, env(s1, 1, "checkpoint", rec))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range m.Events {
		found = found || (e.Type == "strategy_switch" && strings.Contains(e.Summary, "counterexample → concrete_example"))
	}
	if !found {
		t.Fatal("strategy_switch not derived")
	}
}

func TestPatternNeedsTwoCurricula(t *testing.T) {
	obs := func(id, turn, quote string) record.Record {
		return record.Record{PatternObservations: []record.PatternObservation{{ID: id, Pattern: "concrete-before-abstract",
			Description: "先具体后抽象", PreferredStrategy: "concrete_example", Stance: "supports", Summary: "x",
			Linked: []string{s1 + ":c1"}, Evidence: ev(turn, quote)}}}
	}
	sameCurr := obs("p2", "t0002", "取决于最慢的分片")
	m, err := replay(t, env(s1, 1, "checkpoint", revision()), env(s1, 2, "checkpoint", obs("p1", "t0004", "不能只看平均值")), env(s2, 1, "checkpoint", sameCurr))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Patterns["concrete-before-abstract"].Status(); got != "candidate" {
		t.Fatalf("two sessions in one curriculum should stay candidate, got %s", got)
	}
	otherCurr := obs("p2", "t0002", "取决于最慢的分片")
	otherCurr.Curriculum = "calculus"
	m, err = replay(t, env(s1, 1, "checkpoint", revision()), env(s1, 2, "checkpoint", obs("p1", "t0004", "不能只看平均值")), env(s2, 1, "checkpoint", otherCurr))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Patterns["concrete-before-abstract"].Status(); got != "supported" {
		t.Fatalf("status = %s", got)
	}
}

func TestTextbookPoints(t *testing.T) {
	with := func(tp *record.TextbookPoints) record.Record {
		r := revision()
		c := concept
		c.TextbookPoints = tp
		r.Concepts = []record.Concept{c}
		return r
	}
	good := &record.TextbookPoints{Pages: []int{34, 36}, Points: []string{"响应时间是一个分布", "高百分位数决定最慢用户的体验"}}
	for name, tp := range map[string]*record.TextbookPoints{
		"no points":  {Pages: []int{34, 36}},
		"six points": {Pages: []int{34, 36}, Points: []string{"一二三四", "一二三四", "一二三四", "一二三四", "一二三四", "一二三四"}},
		"short":      {Pages: []int{34, 36}, Points: []string{"太短"}},
		"pages":      {Pages: []int{36, 34}, Points: []string{"响应时间是一个分布"}},
	} {
		if _, err := replay(t, env(s1, 1, "checkpoint", with(tp))); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	updated := record.Record{Concepts: []record.Concept{{ID: "tail-latency", TextbookPoints: &record.TextbookPoints{Pages: []int{34, 38}, Points: []string{"用百分位数而不是平均值描述响应时间"}}}}}
	m, err := replay(t, env(s1, 1, "checkpoint", with(good)), env(s1, 2, "checkpoint", with(good)), env(s2, 1, "checkpoint", updated))
	if err != nil {
		t.Fatal(err)
	}
	pts := m.Concepts["tail-latency"].Points
	if len(pts) != 2 || pts[1].Pages[1] != 38 || pts[0].Points[0] != "响应时间是一个分布" {
		t.Fatalf("points history = %+v", pts)
	}
}

func TestSpacedReviewSchedule(t *testing.T) {
	day := func(d int) string {
		return time.Date(2026, 10, d, 12, 0, 0, 0, time.Local).UTC().Format(time.RFC3339Nano)
	}
	at := func(e record.Envelope, d int) record.Envelope { e.SubmittedAt = day(d); return e }
	review := func(id, outcome string) record.Record {
		return record.Record{ReviewResults: []record.ReviewResult{{ID: id, Concept: "tail-latency", Outcome: outcome, ActionTurn: "t0003", Evidence: ev("t0004", "不能只看平均值")}}}
	}
	envs := []record.Envelope{at(env(s1, 1, "checkpoint", revision()), 1)}
	check := func(wantDue string, wantLevel int) {
		t.Helper()
		m, err := replay(t, envs...)
		if err != nil {
			t.Fatal(err)
		}
		s := m.ScheduleFor(m.Concepts["tail-latency"])
		if s.Due != wantDue || s.Level != wantLevel {
			t.Fatalf("schedule = %+v; want due %s level %d", s, wantDue, wantLevel)
		}
	}
	check("2026-10-02", 0)
	envs = append(envs, at(env(s1, 2, "checkpoint", review("r1", "recalled")), 2))
	check("2026-10-04", 1)
	envs = append(envs, at(env(s1, 3, "checkpoint", review("r2", "recalled")), 4))
	check("2026-10-08", 2)
	envs = append(envs, at(env(s1, 4, "checkpoint", review("r3", "partial")), 8))
	check("2026-10-12", 2)
	envs = append(envs, at(env(s1, 5, "checkpoint", review("r4", "forgotten")), 12))
	check("2026-10-13", 0)
	envs = append(envs, at(env(s1, 6, "checkpoint", review("r5", "recalled")), 13))
	check("2026-10-15", 1)
	fragile := record.Record{StateUpdates: []record.StateUpdate{{ID: "u9", Concept: "tail-latency", State: "fragile",
		Capabilities: []string{"explained"}, Summary: "又混淆了", Evidence: ev("t0004", "不能只看平均值")}}}
	envs = append(envs, at(env(s1, 7, "checkpoint", fragile), 14))
	check("2026-10-15", 0)
	for name, r := range map[string]record.ReviewResult{
		"outcome":     {ID: "x1", Concept: "tail-latency", Outcome: "maybe", ActionTurn: "t0003", Evidence: ev("t0004", "不能只看平均值")},
		"concept":     {ID: "x2", Concept: "nope", Outcome: "recalled", ActionTurn: "t0003", Evidence: ev("t0004", "不能只看平均值")},
		"user action": {ID: "x3", Concept: "tail-latency", Outcome: "recalled", ActionTurn: "t0002", Evidence: ev("t0004", "不能只看平均值")},
		"early":       {ID: "x4", Concept: "tail-latency", Outcome: "recalled", ActionTurn: "t0003", Evidence: ev("t0002", "B 平均更快")},
	} {
		bad := append(append([]record.Envelope{}, envs...), at(env(s1, 9, "checkpoint", record.Record{ReviewResults: []record.ReviewResult{r}}), 13))
		if _, err := replay(t, bad...); err == nil {
			t.Errorf("%s review accepted", name)
		}
	}
}

func TestConceptRelations(t *testing.T) {
	rec := revision()
	rec.Concepts = append(rec.Concepts, record.Concept{ID: "throughput", Label: "吞吐量", Related: []record.Relation{{Concept: "tail-latency", Note: "同为性能指标"}}})
	rec.Concepts[0].Related = []record.Relation{{Concept: "throughput", Note: "重复提交"}}
	m, err := replay(t, env(s1, 1, "checkpoint", rec))
	if err != nil {
		t.Fatal(err)
	}
	a, b := m.Concepts["tail-latency"].Related, m.Concepts["throughput"].Related
	if len(a) != 1 || len(b) != 1 || a[0].Concept != "throughput" || b[0].Concept != "tail-latency" || a[0].Note != "重复提交" {
		t.Fatalf("relations a=%+v b=%+v", a, b)
	}
	self := revision()
	self.Concepts[0].Related = []record.Relation{{Concept: "tail-latency"}}
	if _, err := replay(t, env(s1, 1, "checkpoint", self)); err == nil {
		t.Fatal("self relation accepted")
	}
	unknown := revision()
	unknown.Concepts[0].Related = []record.Relation{{Concept: "ghost"}}
	if _, err := replay(t, env(s1, 1, "checkpoint", unknown)); err == nil {
		t.Fatal("unknown relation accepted")
	}
}
