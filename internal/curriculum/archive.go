package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/fsutil"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"gopkg.in/yaml.v3"
)

// Archive describes a curriculum moved out of the active learning space.
// History (Conversations, Sessions, records, Git) is never moved or deleted.
type Archive struct {
	ArchiveID  string   `yaml:"archive_id" json:"archive_id"`
	ID         string   `yaml:"id" json:"id"`
	Title      string   `yaml:"title" json:"title"`
	SHA256     string   `yaml:"sha256" json:"sha256"`
	Reason     string   `yaml:"reason" json:"reason"`
	ArchivedAt string   `yaml:"archived_at" json:"archived_at"`
	Sessions   []string `yaml:"sessions,omitempty" json:"sessions,omitempty"`
}

// RemovalPlan is what `curriculum remove --dry-run` reports.
type RemovalPlan struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Source     string   `json:"source"`
	Curriculum string   `json:"curriculum"`
	Active     bool     `json:"active"`
	Sessions   []string `json:"sessions"`
	ArchiveID  string   `json:"archive_id"`
	Blocked    string   `json:"blocked,omitempty"`
}

func archiveRoot(root string) string { return filepath.Join(root, ".learning", "archive") }

// PlanRemoval lists what an archive would move and why it may be refused.
func PlanRemoval(root, id string, now time.Time) (RemovalPlan, error) {
	m, err := LoadManifest(root, id)
	if err != nil {
		return RemovalPlan{}, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return RemovalPlan{}, err
	}
	sessions, err := sessionsFor(root, id)
	if err != nil {
		return RemovalPlan{}, err
	}
	plan := RemovalPlan{ID: id, Title: m.Title, Source: "Sources/" + id, Curriculum: "Curriculum/" + id,
		Active: cfg.Curriculum.Active == id, Sessions: sessions,
		ArchiveID: id + "-" + now.UTC().Format("20060102T150405Z")}
	state, err := runtimeState.Load(root)
	if err != nil {
		return RemovalPlan{}, err
	}
	if state.ActiveSession != nil && state.ActiveSession.Curriculum == id {
		plan.Blocked = fmt.Sprintf("session %s is active on this curriculum; end or abort it first", state.ActiveSession.ID)
	}
	return plan, nil
}

func sessionsFor(root, id string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Conversations"))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read conversations: %w", err)
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		conv, err := conversation.Load(filepath.Join(root, "Conversations", e.Name()))
		if err == nil && conv.Curriculum == id {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Remove archives a curriculum: its Source and Curriculum directories move
// under .learning/archive with same-filesystem renames, and a matching active
// curriculum is cleared. Conversations, Sessions and records stay in place.
func Remove(root, id, reason string, now time.Time) (Archive, error) {
	if strings.TrimSpace(reason) == "" {
		return Archive{}, fmt.Errorf("--reason is required")
	}
	plan, err := PlanRemoval(root, id, now)
	if err != nil {
		return Archive{}, err
	}
	if plan.Blocked != "" {
		return Archive{}, fmt.Errorf("cannot remove %s: %s", id, plan.Blocked)
	}
	m, _ := LoadManifest(root, id)
	dir := filepath.Join(archiveRoot(root), plan.ArchiveID)
	if _, err := os.Stat(dir); err == nil {
		return Archive{}, fmt.Errorf("archive %s already exists", plan.ArchiveID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Archive{}, fmt.Errorf("create archive: %w", err)
	}
	// Large originals stay out of Git in the archive, as they did in Sources.
	if _, err := fsutil.WriteFileIfAbsent(filepath.Join(archiveRoot(root), ".gitignore"), []byte("*/source/original/\n"), 0o644); err != nil {
		return Archive{}, err
	}
	a := Archive{ArchiveID: plan.ArchiveID, ID: id, Title: m.Title, SHA256: m.SHA256, Reason: reason,
		ArchivedAt: now.UTC().Format(time.RFC3339), Sessions: plan.Sessions}
	data, err := yaml.Marshal(a)
	if err != nil {
		return Archive{}, err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "archive.yaml"), data, 0o644); err != nil {
		return Archive{}, err
	}
	if err := os.Rename(filepath.Join(root, "Sources", id), filepath.Join(dir, "source")); err != nil {
		os.RemoveAll(dir)
		return Archive{}, fmt.Errorf("archive source: %w", err)
	}
	if err := os.Rename(filepath.Join(root, "Curriculum", id), filepath.Join(dir, "curriculum")); err != nil {
		if back := os.Rename(filepath.Join(dir, "source"), filepath.Join(root, "Sources", id)); back != nil {
			return Archive{}, fmt.Errorf("archive curriculum: %w; restoring source also failed: %v", err, back)
		}
		os.RemoveAll(dir)
		return Archive{}, fmt.Errorf("archive curriculum: %w", err)
	}
	if plan.Active {
		cfg, err := config.Load(root)
		if err != nil {
			return a, err
		}
		cfg.Curriculum.Active = ""
		if err := config.Save(root, cfg); err != nil {
			return a, err
		}
	}
	return a, nil
}

// ListArchives returns archives in id order.
func ListArchives(root string) ([]Archive, error) {
	entries, err := os.ReadDir(archiveRoot(root))
	if os.IsNotExist(err) {
		return []Archive{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read archives: %w", err)
	}
	out := []Archive{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		a, err := loadArchive(root, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func loadArchive(root, archiveID string) (Archive, error) {
	if strings.ContainsAny(archiveID, `/\`) || archiveID == "" || archiveID == "." || archiveID == ".." {
		return Archive{}, fmt.Errorf("invalid archive id %q", archiveID)
	}
	data, err := os.ReadFile(filepath.Join(archiveRoot(root), archiveID, "archive.yaml"))
	if err != nil {
		return Archive{}, fmt.Errorf("archive %s: %w", archiveID, err)
	}
	var a Archive
	if err := yaml.Unmarshal(data, &a); err != nil {
		return Archive{}, fmt.Errorf("parse archive %s: %w", archiveID, err)
	}
	return a, nil
}

// ArchivedTitle returns the title of the latest archive of id, or id.
func ArchivedTitle(root, id string) string {
	archives, err := ListArchives(root)
	if err != nil {
		return id
	}
	title := id
	for _, a := range archives {
		if a.ID == id {
			title = a.Title
		}
	}
	return title
}

// Restore moves an archive back. It refuses when the id is in use again.
func Restore(root, archiveID string) (Archive, error) {
	a, err := loadArchive(root, archiveID)
	if err != nil {
		return Archive{}, err
	}
	for _, p := range []string{filepath.Join(root, "Sources", a.ID), filepath.Join(root, "Curriculum", a.ID)} {
		if _, err := os.Stat(p); err == nil {
			return Archive{}, fmt.Errorf("cannot restore: %s already exists; remove or rename it first", p)
		}
	}
	dir := filepath.Join(archiveRoot(root), archiveID)
	if err := os.Rename(filepath.Join(dir, "source"), filepath.Join(root, "Sources", a.ID)); err != nil {
		return Archive{}, fmt.Errorf("restore source: %w", err)
	}
	if err := os.Rename(filepath.Join(dir, "curriculum"), filepath.Join(root, "Curriculum", a.ID)); err != nil {
		if back := os.Rename(filepath.Join(root, "Sources", a.ID), filepath.Join(dir, "source")); back != nil {
			return Archive{}, fmt.Errorf("restore curriculum: %w; moving source back also failed: %v", err, back)
		}
		return Archive{}, fmt.Errorf("restore curriculum: %w", err)
	}
	return a, os.RemoveAll(dir)
}

// Purge permanently deletes an archive. The caller must repeat the archive
// id as confirmation.
func Purge(root, archiveID, confirmation string) (Archive, error) {
	a, err := loadArchive(root, archiveID)
	if err != nil {
		return Archive{}, err
	}
	if confirmation != archiveID {
		return Archive{}, fmt.Errorf("purge permanently deletes %s; repeat its archive id with --confirm to proceed", archiveID)
	}
	return a, os.RemoveAll(filepath.Join(archiveRoot(root), archiveID))
}

// Deactivate clears the active curriculum unless a session is using it.
func Deactivate(root string) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	if cfg.Curriculum.Active == "" {
		return "", fmt.Errorf("no active curriculum")
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		return "", err
	}
	if state.ActiveSession != nil && state.ActiveSession.Curriculum == cfg.Curriculum.Active {
		return "", fmt.Errorf("session %s is active on %s; end or abort it first", state.ActiveSession.ID, cfg.Curriculum.Active)
	}
	id := cfg.Curriculum.Active
	cfg.Curriculum.Active = ""
	return id, config.Save(root, cfg)
}

// archivedMatch returns the archive whose content hash equals hash.
func archivedMatch(root, hash string) string {
	archives, err := ListArchives(root)
	if err != nil {
		return ""
	}
	for _, a := range archives {
		if a.SHA256 == hash {
			return a.ArchiveID
		}
	}
	return ""
}
