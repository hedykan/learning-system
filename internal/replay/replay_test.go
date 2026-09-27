// Package replay holds the v0.1.2 core acceptance test: after a realistic
// history of 12 sessions across two curricula, the Learning Policy must act
// differently than it would without history, and say which history it used.
package replay_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/model"
	"github.com/hedykan/learning-system/internal/policy"
	"github.com/hedykan/learning-system/internal/record"
	"github.com/hedykan/learning-system/internal/session"
	"github.com/hedykan/learning-system/internal/vault"
)

// Each session follows the same shape: the assistant asks, the learner gives
// a first model, the assistant challenges it, the learner revises.
func turns(i int, label string) []struct{ role, text string } {
	return []struct{ role, text string }{
		{"assistant", fmt.Sprintf("第 %d 课：请先预测「%s」会怎样。", i, label)},
		{"user", fmt.Sprintf("我猜%s就是最直观的那样，第%d次的想法", label, i)},
		{"assistant", fmt.Sprintf("看一个具体场景 %d，结果和你的预测一致吗？", i)},
		{"user", fmt.Sprintf("原来%s要在具体场景里才看得清，第%d次修正", label, i)},
	}
}

func ev(turn, quote string) []record.Evidence { return []record.Evidence{{Turn: turn, Quote: quote}} }

type lesson struct {
	curr, chapter, concept, label string
	build                         func(i int, first, revised string) record.Record
}

const pattern = "concrete-before-abstract"

// revision is the recurring "misconception → concrete example → revision" loop.
func revision(concept string, withSwitch, observe bool) func(int, string, string) record.Record {
	return func(i int, first, revised string) record.Record {
		r := record.Record{
			Events: []record.Event{{ID: "e1", Type: "misconception", Concept: concept, Summary: "用直观印象代替定义", Evidence: ev("t0002", first)}},
			CognitiveChanges: []record.CognitiveChange{{ID: "c1", Concept: concept, OldModel: "直观印象", Trigger: "具体场景与预测冲突",
				TriggerTurn: "t0003", NewModel: "在场景中重建定义", OldEvidence: ev("t0002", first), NewEvidence: ev("t0004", revised)}},
			StrategyAttempts: []record.StrategyAttempt{{ID: "s2", Strategy: "concrete_example", Situation: "misconception", Concept: concept,
				ActionTurn: "t0003", ExpectedChange: "预测失败后重建定义", Outcome: "effective", Linked: []string{"c1"}, Evidence: ev("t0004", revised)}},
			StateUpdates: []record.StateUpdate{{ID: "u1", Concept: concept, State: "developing", Capabilities: []string{"explained"},
				Summary: "能在具体场景中解释", Evidence: ev("t0004", revised)}},
		}
		if withSwitch {
			r.StrategyAttempts = append([]record.StrategyAttempt{{ID: "s1", Strategy: "counterexample", Situation: "misconception", Concept: concept,
				ActionTurn: "t0003", ExpectedChange: "反例触发冲突", Outcome: "ineffective", Reason: "抽象反例没有引起冲突", Evidence: ev("t0004", revised)}},
				r.StrategyAttempts...)
			r.StrategyAttempts[1].Replaces, r.StrategyAttempts[1].SwitchReason = "s1", "改用学习者熟悉的具体场景"
		}
		if observe {
			r.PatternObservations = []record.PatternObservation{{ID: "p1", Pattern: pattern, Description: "抽象定义难以直接接受；具体场景中预测失败后，学习者能重新抽象",
				PreferredStrategy: "concrete_example", Situation: "misconception", Stance: "supports", Summary: "具体场景促成重新抽象",
				Linked: []string{"c1"}, Evidence: ev("t0004", revised)}}
		}
		return r
	}
}

func developing(concept string) func(int, string, string) record.Record {
	return func(i int, first, revised string) record.Record {
		return record.Record{
			Events: []record.Event{{ID: "e1", Type: "prediction", Concept: concept, Summary: "给出预测", Evidence: ev("t0002", first)}},
			StateUpdates: []record.StateUpdate{{ID: "u1", Concept: concept, State: "developing", Capabilities: []string{"predicted", "applied"},
				Summary: "能预测并应用，尚未解释", Evidence: ev("t0004", revised)}},
		}
	}
}

var script = []lesson{
	{"ddia", "1", "tail-latency", "尾延迟", revision("tail-latency", false, true)},
	{"calculus", "1", "limit", "极限", revision("limit", true, true)},
	{"ddia", "1", "throughput", "吞吐量", revision("throughput", true, false)},
	{"calculus", "2", "derivative", "导数", revision("derivative", false, true)},
	{"ddia", "2", "data-model", "数据模型", developing("data-model")},
	{"ddia", "1", "tail-latency", "尾延迟", func(i int, first, revised string) record.Record {
		return record.Record{StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail-latency", State: "stable",
			Capabilities: []string{"transferred"}, Summary: "迁移到分片扇出场景", Evidence: ev("t0004", revised)}}}
	}},
	{"calculus", "2", "derivative", "导数", func(i int, first, revised string) record.Record {
		return record.Record{StrategyAttempts: []record.StrategyAttempt{{ID: "s1", Strategy: "analogy", Situation: "retrieval", Concept: "derivative",
			ActionTurn: "t0003", ExpectedChange: "用类比回忆", Outcome: "inconclusive", Reason: "回答含糊", Evidence: ev("t0004", revised)}}}
	}},
	{"ddia", "3", "storage-engine", "存储引擎", developing("storage-engine")},
	{"ddia", "1", "tail-latency", "尾延迟", func(i int, first, revised string) record.Record {
		return record.Record{
			Events: []record.Event{{ID: "e1", Type: "misconception", Concept: "tail-latency", Summary: "把尾延迟当成平均延迟的上限", Evidence: ev("t0002", first)}},
			StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail-latency", State: "fragile", Capabilities: []string{"explained"},
				Summary: "反例下理解动摇", Evidence: ev("t0004", revised)}},
		}
	}},
	{"calculus", "3", "integral", "积分", revision("integral", false, true)},
	{"ddia", "4", "encoding", "编码", func(i int, first, revised string) record.Record {
		r := developing("encoding")(i, first, revised)
		r.PatternObservations = []record.PatternObservation{{ID: "p1", Pattern: pattern, Stance: "contradicts",
			Summary: "这次直接读定义就理解了", Linked: []string{"e1"}, Evidence: ev("t0004", revised)}}
		return r
	}},
	{"ddia", "5", "replication-lag", "复制延迟", func(i int, first, revised string) record.Record {
		return record.Record{Events: []record.Event{{ID: "e1", Type: "misconception", Concept: "replication-lag",
			Summary: "以为副本总是实时一致", Evidence: ev("t0002", first)}}}
	}},
}

func TestTwelveSessionReplayChangesTheNextAction(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	base := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	if _, err := vault.Init(root, base); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ddia", "calculus"} {
		src := filepath.Join(t.TempDir(), id+".md")
		if err := os.WriteFile(src, []byte("# "+id), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: src, ID: id, Confirmed: true, Now: base}); err != nil {
			t.Fatal(err)
		}
		if _, err := assessment.Save(root, "session-20260801-000000.000000000", assessment.Report{Curriculum: id, Depth: "quick"}, base); err != nil {
			t.Fatal(err)
		}
	}
	declared := map[string]bool{}
	lastConcept := map[string]string{}
	var sessions []string
	for n, l := range script {
		i := n + 1
		now := base.Add(time.Duration(i) * 24 * time.Hour)
		if err := curriculum.Activate(root, l.curr); err != nil {
			t.Fatal(err)
		}
		if err := curriculum.SavePosition(root, l.curr, curriculum.Position{Chapter: l.chapter, CurrentConcept: l.label}); err != nil {
			t.Fatal(err)
		}
		started, err := session.Start(root, session.StartOptions{Kind: "lesson"}, now)
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, started.ID)
		ts := turns(i, l.label)
		for k, turn := range ts {
			if _, err := session.Append(root, turn.role, turn.text, now.Add(time.Duration(k+1)*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
		first := fmt.Sprintf("第%d次的想法", i)
		revised := fmt.Sprintf("第%d次修正", i)
		rec := l.build(i, first, revised)
		rec.Schema, rec.Curriculum = record.Schema, l.curr
		if !declared[l.concept] {
			c := record.Concept{ID: l.concept, Label: l.label, SourceRef: &record.SourceRef{Chapter: l.chapter}}
			if prev := lastConcept[l.curr]; prev != "" && i < len(script) {
				c.Related = []record.Relation{{Concept: prev, Note: "同一本书中相邻的概念"}}
			}
			rec.Concepts = []record.Concept{c}
			declared[l.concept] = true
			lastConcept[l.curr] = l.concept
		}
		if _, err := session.Checkpoint(root, &rec, now.Add(10*time.Minute)); err != nil {
			t.Fatalf("session %d checkpoint: %v", i, err)
		}
		if i < len(script) {
			if _, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{}, now.Add(20*time.Minute)); err != nil {
				t.Fatalf("session %d end: %v", i, err)
			}
		}
	}

	envs, err := record.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	resolver := learner.NewResolver(root)
	full, err := learner.Replay(envs, resolver)
	if err != nil {
		t.Fatal(err)
	}
	current := sessions[len(sessions)-1]
	var onlyCurrent []record.Envelope
	for _, env := range envs {
		if env.Session == current {
			onlyCurrent = append(onlyCurrent, env)
		}
	}
	control, err := learner.Replay(onlyCurrent, resolver)
	if err != nil {
		t.Fatal(err)
	}
	ctx := policy.Context{Curriculum: "ddia", Assessed: true, ActiveSession: current,
		Position: curriculum.Position{Chapter: "5", CurrentConcept: "复制延迟"}}

	without := policy.Next(control, ctx)
	with := policy.Next(full, ctx)
	t.Logf("without history: %+v", without)
	t.Logf("with history:    %+v", with)

	// 1. Control group uses the default rule and strategy.
	if without.Rule != "R2-open-misconception" || without.Strategy != policy.Defaults["misconception"] || without.HistoryUsed {
		t.Fatalf("control = %+v", without)
	}
	// 2. History changes the decision and names the earlier evidence.
	if with.Action == without.Action && with.Strategy == without.Strategy {
		t.Fatalf("history did not change the decision: %+v", with)
	}
	earlier := map[string]bool{}
	for _, gid := range with.Evidence {
		if s, _, _ := strings.Cut(gid, ":"); s != current {
			earlier[s] = true
		}
	}
	if !with.HistoryUsed || len(earlier) < 2 {
		t.Fatalf("decision must cite at least two earlier sessions: %+v", with)
	}
	// 3. Curriculum fidelity: stay on the mainline, in the current chapter.
	if with.CurriculumRelation != "mainline" || with.Concept != "replication-lag" {
		t.Fatalf("left the mainline: %+v", with)
	}
	// 4. The fixture exercised the hard cases.
	if got := full.Patterns[pattern].Status(); got != "supported" {
		t.Fatalf("pattern status = %s", got)
	}
	switched := false
	for _, e := range full.Events {
		switched = switched || e.Type == "strategy_switch"
	}
	if !switched {
		t.Fatal("fixture has no strategy_switch")
	}
	var states []string
	for _, h := range full.Concepts["tail-latency"].History {
		states = append(states, h.State)
	}
	if strings.Join(states, ">") != "developing>stable>fragile" {
		t.Fatalf("tail-latency history = %v", states)
	}
	// 5. R4b: data-model was applied in session 5; the new session consolidates it first.
	consolidate := policy.Next(full, policy.Context{Curriculum: "ddia", Assessed: true, ActiveSession: current,
		Position: curriculum.Position{Chapter: "2", CurrentConcept: "数据库选型"}})
	if consolidate.Rule != "R4b-consolidate" || consolidate.Concept != "data-model" {
		t.Fatalf("consolidation = %+v", consolidate)
	}
	if len(sessions) != 12 {
		t.Fatalf("sessions = %d", len(sessions))
	}
}
