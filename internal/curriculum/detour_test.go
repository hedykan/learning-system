package curriculum_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/vault"
)

func TestDetourStartEndRestoresReturnPoint(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	if _, err := vault.Init(root, now); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(src, []byte("# DDIA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: src, ID: "ddia", Activate: true, Confirmed: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := curriculum.SavePosition(root, "ddia", curriculum.Position{Chapter: "1", Section: "负载", CurrentConcept: "吞吐量"}); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.StartDetour(root, "ddia", curriculum.DetourStart{Topic: "事务", Reason: "锁边界不清"}, now); err == nil {
		t.Fatal("detour without return condition accepted")
	}
	d, err := curriculum.StartDetour(root, "ddia", curriculum.DetourStart{Topic: "事务", Concept: "transaction", Reason: "锁边界不清", ReturnCondition: "能解释并发扣减"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "d1" || d.ReturnTo.Concept != "吞吐量" {
		t.Fatalf("detour = %+v", d)
	}
	if _, err := curriculum.StartDetour(root, "ddia", curriculum.DetourStart{Topic: "x", Reason: "y", ReturnCondition: "z"}, now); err == nil {
		t.Fatal("second open detour accepted")
	}
	if err := curriculum.SavePosition(root, "ddia", curriculum.Position{Chapter: "7", Section: "事务", CurrentConcept: "ACID", Detour: &d}); err != nil {
		t.Fatal(err)
	}
	entry, err := curriculum.EndDetour(root, "ddia", "completed", "理解了同一事务内读改写", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if entry.Learned == "" || entry.ReturnCondition != "能解释并发扣减" {
		t.Fatalf("entry = %+v", entry)
	}
	pos, err := curriculum.LoadPosition(root, "ddia")
	if err != nil {
		t.Fatal(err)
	}
	if pos.Detour != nil || pos.Chapter != "1" || pos.CurrentConcept != "吞吐量" {
		t.Fatalf("position not restored: %+v", pos)
	}
	log, err := curriculum.LoadDetourLog(root, "ddia")
	if err != nil || len(log) != 1 || log[0].Reason != "锁边界不清" {
		t.Fatalf("log = %+v err=%v", log, err)
	}
	if _, err := curriculum.EndDetour(root, "ddia", "completed", "x", now); err == nil {
		t.Fatal("ending without open detour accepted")
	}
}
