package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/vault"
)

func TestInitIsIdempotentAndPreservesAgentRules(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Learning")
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	first, err := vault.Init(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Git != "initialized" {
		t.Fatalf("git = %q; want initialized", first.Git)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("custom rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := vault.Init(root, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if second.Git != "preserved" {
		t.Fatalf("git = %q; want preserved", second.Git)
	}
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "custom rules\n" {
		t.Fatalf("AGENTS.md overwritten: %q", got)
	}
	for _, rel := range []string{
		".learning/config.yaml", ".learning/state.json", ".learning/schema-version",
		".agents/skills/learning-os/SKILL.md", ".claude/skills/learning-os/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
}

func TestDiscoverWalksUpFromChild(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "Concepts", "calculus")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := vault.Discover(child)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("Discover() = %q; want %q", got, root)
	}
}

func TestUpdateAgentAssetsDryRunThenBacksUp(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(root, ".agents", "skills", "learning-os", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("custom skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := vault.UpdateAgentAssets(root, true, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) == 0 {
		t.Fatal("dry run found no changes")
	}
	data, err := os.ReadFile(skillPath)
	if err != nil || string(data) != "custom skill\n" {
		t.Fatalf("dry run changed skill: %q, err=%v", data, err)
	}
	result, err := vault.UpdateAgentAssets(root, false, true, time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupDir == "" {
		t.Fatal("update did not report backup directory")
	}
	backup, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(result.BackupDir), ".agents", "skills", "learning-os", "SKILL.md"))
	if err != nil || string(backup) != "custom skill\n" {
		t.Fatalf("backup = %q, err=%v", backup, err)
	}
	updated, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "baseline workflow") {
		t.Fatalf("updated skill does not contain baseline workflow:\n%s", updated)
	}
	if !strings.Contains(string(updated), "Keep every Runtime operation backstage") {
		t.Fatalf("updated skill does not require backstage Runtime operations:\n%s", updated)
	}
	if !strings.Contains(string(updated), "not even in progress updates before tool calls") {
		t.Fatalf("updated skill does not cover progress updates:\n%s", updated)
	}
	outlineRef, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "learning-os", "references", "curriculum-outline.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outlineRef), "text-based PDF") || !strings.Contains(string(outlineRef), "own vision") {
		t.Fatal("skill lacks the scanned-page reading order")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "learning-os", "references", "review-workflow.md")); err != nil {
		t.Fatal("skill lacks the review workflow")
	}
	rules, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rules), "One learning turn has exactly one reply") {
		t.Fatalf("updated rules lack the single-reply rule:\n%s", rules)
	}
	if !strings.Contains(string(rules), "## Learner-facing voice") {
		t.Fatalf("updated rules lack learner-facing voice:\n%s", rules)
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "learning-os", "references", "session-workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "Do not say that progress was saved") {
		t.Fatalf("session workflow exposes routine bookkeeping:\n%s", workflow)
	}
	exclude, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".learning/backups/") {
		t.Fatalf("agent backups are not Git-excluded:\n%s", exclude)
	}
}
