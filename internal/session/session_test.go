package session_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/model"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/session"
	"github.com/hedykan/learning-system/internal/vault"
)

func TestAnalysisFailurePreservesActiveSessionAndRawConversation(t *testing.T) {
	root := initVault(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	started, err := session.Start(root, session.StartOptions{Domain: "calculus", Kind: "lesson", SkipBaseline: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(root, "user", "什么是极限？", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, err = session.End(context.Background(), root, model.FakeProvider{Err: errors.New("provider unavailable")}, session.EndOptions{NoAnalysis: true, NoAnalysisReason: "测试旧分析器"}, now.Add(2*time.Minute))
	if err == nil {
		t.Fatal("expected provider error")
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveSession == nil || state.ActiveSession.ID != started.ID {
		t.Fatalf("active session lost: %+v", state.ActiveSession)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(started.Conversation)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "什么是极限？") {
		t.Fatal("raw conversation content lost")
	}
	if _, err := os.Stat(filepath.Join(root, "Sessions", started.ID+".md")); !os.IsNotExist(err) {
		t.Fatalf("session file written after failed analysis: %v", err)
	}
}

func TestSessionLifecycleCommitsGit(t *testing.T) {
	root := initVault(t)
	runGit(t, root, "config", "user.email", "test@example.invalid")
	runGit(t, root, "config", "user.name", "Learning OS Test")
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	started, err := session.Start(root, session.StartOptions{Domain: "calculus", Kind: "lesson", SkipBaseline: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(root, "user", "极限一定要到达吗？", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(root, "assistant", "先考虑一个不断逼近的序列。", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{}, now.Add(3*time.Minute)); err == nil {
		t.Fatal("lesson ended without analysis or explicit --no-analysis")
	}
	result, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{NoAnalysis: true, NoAnalysisReason: "短会话"}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.Git != "committed" {
		t.Fatalf("git = %q", result.Git)
	}
	if result.Events != 1 {
		t.Fatalf("events = %d; want 1", result.Events)
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveSession != nil || state.LastSession != started.ID {
		t.Fatalf("unexpected state: %+v", state)
	}
	sessionData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(result.Session)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sessionData), "question") {
		t.Fatalf("session has no question event:\n%s", sessionData)
	}
	if got := runGit(t, root, "log", "-1", "--pretty=%s"); got != "learning: complete "+started.ID {
		t.Fatalf("commit = %q", got)
	}
}

func TestBaselineMustEndWithUserEvidenceThenDefaultsToLesson(t *testing.T) {
	root := initVault(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	source := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(source, []byte("# DDIA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{
		SourcePath: source, ID: "ddia", Title: "DDIA", Activate: true, Confirmed: true, Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	started, err := session.Start(root, session.StartOptions{}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if started.Kind != "baseline" || started.Depth != "standard" {
		t.Fatalf("started = %+v", started)
	}
	if _, err := session.Append(root, "assistant", "你熟悉缓存吗？", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	learnerAnswer := "我用过数据库，但不知道缓存是什么。"
	if _, err := session.Append(root, "user", learnerAnswer, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{}, now.Add(4*time.Minute)); err == nil {
		t.Fatal("baseline ended without an assessment")
	}
	state, err := runtimeState.Load(root)
	if err != nil || state.ActiveSession == nil {
		t.Fatalf("active baseline not preserved: state=%+v err=%v", state, err)
	}
	report := assessment.Report{
		Curriculum: "ddia", Depth: "standard",
		PrerequisiteGaps:     []assessment.Finding{{Claim: "尚未建立缓存概念", Evidence: learnerAnswer}},
		UnknownVocabulary:    []string{"缓存"},
		RecommendedEntry:     curriculum.Position{Book: "ddia", Chapter: "1", Section: "引言", CurrentConcept: "数据系统的基本职责"},
		RecommendationReason: "先建立数据系统构件的基础词汇。",
		NextProbe:            "订单数据为什么需要长期保存？",
	}
	ended, err := session.End(context.Background(), root, model.ConservativeProvider{}, session.EndOptions{Assessment: &report}, now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ended.Kind != "baseline" || ended.Assessment == "" {
		t.Fatalf("ended = %+v", ended)
	}
	status, err := assessment.CurrentStatus(root, "ddia")
	if err != nil || status.State != "assessed" {
		t.Fatalf("baseline status = %+v, err=%v", status, err)
	}
	next, err := session.Start(root, session.StartOptions{}, now.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if next.Kind != "lesson" {
		t.Fatalf("next kind = %q; want lesson", next.Kind)
	}
}

func TestAbortDoesNotAdvanceCurriculumProgress(t *testing.T) {
	root := initVault(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	source := filepath.Join(t.TempDir(), "book.md")
	if err := os.WriteFile(source, []byte("# Book"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{
		SourcePath: source, ID: "book", Activate: true, Confirmed: true, Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	progressPath := filepath.Join(root, "Curriculum", "book", "学习进度.md")
	before, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatal(err)
	}
	started, err := session.Start(root, session.StartOptions{Kind: "lesson", SkipBaseline: true}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Abort(root, "重新摸底", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.Termination != "aborted" || result.ID != started.ID {
		t.Fatalf("result = %+v", result)
	}
	after, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("progress changed on abort:\n%s", after)
	}
	state, err := runtimeState.Load(root)
	if err != nil || state.ActiveSession != nil {
		t.Fatalf("active session not cleared: state=%+v err=%v", state, err)
	}
}

func initVault(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Git.Enabled {
		t.Fatal("git should be enabled by default")
	}
	return root
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", root}, args...)
	out, err := exec.Command("git", cmdArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Sessions started at the same instant (a review right after its lesson, or
// a fixed LEARN_NOW) get distinct conversations.
func TestSameInstantStartsDoNotCollide(t *testing.T) {
	root := initVault(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	lesson, err := session.Start(root, session.StartOptions{Kind: "lesson", SkipBaseline: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(root, "user", "学习课的话", now); err != nil {
		t.Fatal(err)
	}
	review, err := session.Start(root, session.StartOptions{Kind: "review"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if review.ID == lesson.ID || review.Suspended != lesson.ID {
		t.Fatalf("lesson %+v, review %+v", lesson, review)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lesson.Conversation)))
	if err != nil || !strings.Contains(string(data), "学习课的话") {
		t.Fatalf("lesson conversation overwritten: %v\n%s", err, data)
	}
}
