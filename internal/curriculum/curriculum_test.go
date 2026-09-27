package curriculum_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/vault"
)

func TestImportDryRunThenCommitAndRejectDuplicate(t *testing.T) {
	root := initVault(t)
	source := filepath.Join(t.TempDir(), "course.md")
	if err := os.WriteFile(source, []byte("# Course\n\nLesson one."), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	plan, err := curriculum.Import(root, curriculum.ImportOptions{
		SourcePath: source, ID: "my-course", DryRun: true, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DryRun || plan.Kind != "markdown" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(root, "Sources", "my-course")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote source: %v", err)
	}
	_, err = curriculum.Import(root, curriculum.ImportOptions{
		SourcePath: source, ID: "my-course", Title: "My Course", Confirmed: true,
		Activate: true, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Curriculum.Active != "my-course" {
		t.Fatalf("active = %q", cfg.Curriculum.Active)
	}
	if _, err := os.Stat(filepath.Join(root, "Sources", "my-course", "original", "course.md")); err != nil {
		t.Fatal(err)
	}
	_, err = curriculum.Import(root, curriculum.ImportOptions{
		SourcePath: source, ID: "copy-course", Confirmed: true, Now: now,
	})
	if !errors.Is(err, curriculum.ErrAlreadyImported) {
		t.Fatalf("duplicate error = %v; want ErrAlreadyImported", err)
	}
}

func TestPositionRoundTrip(t *testing.T) {
	root := initVault(t)
	source := filepath.Join(t.TempDir(), "course.txt")
	if err := os.WriteFile(source, []byte("lesson"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: source, ID: "course", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	want := curriculum.Position{Book: "course", Chapter: "2", Section: "2.3", CurrentConcept: "limits"}
	if err := curriculum.SavePosition(root, "course", want); err != nil {
		t.Fatal(err)
	}
	got, err := curriculum.LoadPosition(root, "course")
	if err != nil {
		t.Fatal(err)
	}
	if got.Book != want.Book || got.Chapter != want.Chapter || got.Section != want.Section || got.CurrentConcept != want.CurrentConcept {
		t.Fatalf("position = %+v; want %+v", got, want)
	}
}

func initVault(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	return root
}
