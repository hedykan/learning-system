package session_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/model"
	"github.com/hedykan/learning-system/internal/projection"
	"github.com/hedykan/learning-system/internal/record"
	"github.com/hedykan/learning-system/internal/session"
)

func TestCheckpointEndAndRebuild(t *testing.T) {
	root := initVault(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", now.Add(time.Hour).Format(time.RFC3339))
	src := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(src, []byte("# DDIA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: src, ID: "ddia", Title: "DDIA", Activate: true, Confirmed: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := curriculum.SavePosition(root, "ddia", curriculum.Position{Chapter: "1", Section: "描述性能", CurrentConcept: "尾延迟"}); err != nil {
		t.Fatal(err)
	}
	if _, err := assessment.Save(root, "session-20260901-000000.000000000", assessment.Report{Curriculum: "ddia", Depth: "quick"}, now); err != nil {
		t.Fatal(err)
	}
	started, err := session.Start(root, session.StartOptions{Kind: "lesson"}, now)
	if err != nil {
		t.Fatal(err)
	}
	turns := []struct{ role, text string }{
		{"assistant", "A 和 B 哪个更能扛住黑五？"},
		{"user", "B 平均更快，所以 B 更好"},
		{"assistant", "1% 请求要等 8 秒，每分钟会有多少人受影响？"},
		{"user", "不能只看平均值，还要看最慢的那批请求"},
	}
	for i, turn := range turns {
		got, err := session.Append(root, turn.role, turn.text, now.Add(time.Duration(i+1)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"t0001", "t0002", "t0003", "t0004"}[i]; got.Turn != want {
			t.Fatalf("turn = %s; want %s", got.Turn, want)
		}
	}
	ev := func(turn, quote string) []record.Evidence { return []record.Evidence{{Turn: turn, Quote: quote}} }
	rec := &record.Record{Schema: record.Schema, Curriculum: "ddia",
		Concepts: []record.Concept{{ID: "tail-latency", Label: "尾延迟", SourceRef: &record.SourceRef{Chapter: "1", Section: "描述性能"}}},
		Events:   []record.Event{{ID: "e1", Type: "prediction", Concept: "tail-latency", Summary: "预测平均更快的 B 更好", Evidence: ev("t0002", "B 平均更快")}},
		CognitiveChanges: []record.CognitiveChange{{ID: "c1", Concept: "tail-latency", OldModel: "平均延迟低即好", Trigger: "慢请求的绝对数量",
			TriggerTurn: "t0003", NewModel: "要同时看尾部延迟", OldEvidence: ev("t0002", "B 平均更快"), NewEvidence: ev("t0004", "不能只看平均值")}},
		StateUpdates: []record.StateUpdate{{ID: "u1", Concept: "tail-latency", State: "developing", Capabilities: []string{"explained"},
			Summary: "能解释平均值会掩盖慢请求", Evidence: ev("t0004", "还要看最慢的那批请求"), OpenQuestions: []string{"分片扇出时尾延迟如何放大"}}},
	}

	bad := *rec
	bad.Events = []record.Event{{ID: "e1", Type: "prediction", Concept: "tail-latency", Summary: "x", Evidence: ev("t0001", "哪个更能扛住")}}
	if _, err := session.Checkpoint(root, &bad, now.Add(5*time.Minute)); err == nil {
		t.Fatal("assistant evidence accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "Concepts", "tail-latency.md")); !os.IsNotExist(err) {
		t.Fatal("rejected record produced a projection")
	}

	res, err := session.Checkpoint(root, rec, now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "accepted" || res.Projections != "updated" {
		t.Fatalf("checkpoint = %+v", res)
	}
	again, err := session.Checkpoint(root, rec, now.Add(6*time.Minute))
	if err != nil || again.Status != "unchanged" {
		t.Fatalf("resubmit = %+v err=%v", again, err)
	}
	conceptPath := filepath.Join(root, "Concepts", "tail-latency.md")
	concept := read(t, conceptPath)
	for _, want := range []string{"形成中", "「不能只看平均值」", "#^t0004", "平均延迟低即好 → 触发：慢请求的绝对数量 → 要同时看尾部延迟", "分片扇出时尾延迟如何放大"} {
		if !strings.Contains(concept, want) {
			t.Fatalf("concept missing %q:\n%s", want, concept)
		}
	}
	if !strings.Contains(read(t, filepath.Join(root, "Sessions", started.ID+".md")), "learn:analysis:begin") {
		t.Fatal("active session has no analysis block")
	}
	overview := read(t, filepath.Join(root, "Profile", "learner-state.md"))
	if !strings.Contains(overview, "application_probe") {
		t.Fatalf("overview lacks next action:\n%s", overview)
	}

	// The learner writes notes; they must survive every rewrite.
	withNote := strings.Replace(concept, "<!-- learn:user:begin -->\n", "<!-- learn:user:begin -->\n我的比喻：尾延迟像排队最慢的那个人\n", 1)
	if err := os.WriteFile(conceptPath, []byte(withNote), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{
		Analysis: &record.Record{Schema: record.Schema, Curriculum: "ddia", ProgressDecision: &record.ProgressDecision{Decision: "stay", Reason: "还没有迁移证据"}},
	}, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	sessionDoc := read(t, filepath.Join(root, "Sessions", started.ID+".md"))
	for _, want := range []string{"termination: completed", "### 进度决策", "stay：还没有迁移证据", "## 手写笔记"} {
		if !strings.Contains(sessionDoc, want) {
			t.Fatalf("session doc missing %q:\n%s", want, sessionDoc)
		}
	}
	if strings.Contains(sessionDoc, "## Learning Events") {
		t.Fatal("legacy analysis rendered despite records")
	}
	pos, err := curriculum.LoadPosition(root, "ddia")
	if err != nil || pos.CurrentConcept != "尾延迟" {
		t.Fatalf("session end moved position: %+v", pos)
	}

	before := map[string]string{}
	for _, rel := range []string{"Concepts/tail-latency.md", "Profile/learner-state.md", "Curriculum/ddia/index.md"} {
		before[rel] = read(t, filepath.Join(root, rel))
	}
	if !strings.Contains(before["Concepts/tail-latency.md"], "我的比喻") {
		t.Fatal("user note lost on session end")
	}
	if err := os.Remove(filepath.Join(root, "Profile", "learner-state.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "Curriculum", "ddia", "index.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".learning", "model")); err != nil {
		t.Fatal(err)
	}
	m, envs, err := learner.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := projection.Staleness(root, m.Generation); got != "stale" {
		t.Fatalf("staleness = %s", got)
	}
	if err := session.Refresh(root); err != nil {
		t.Fatal(err)
	}
	for rel, want := range before {
		if got := read(t, filepath.Join(root, rel)); got != want {
			t.Fatalf("%s not reproduced by rebuild:\n--- got\n%s\n--- want\n%s", rel, got, want)
		}
	}
	if len(envs) != 2 || projection.Staleness(root, m.Generation) != "current" {
		t.Fatalf("envs=%d staleness=%s", len(envs), projection.Staleness(root, m.Generation))
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
