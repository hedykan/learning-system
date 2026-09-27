// Package projection renders the Learner Model as human-readable Markdown.
// Output depends only on records, curriculum state and resolver data, never
// on the clock, so a rebuild reproduces the same bytes.
package projection

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/clock"
	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/mdblock"
	"github.com/hedykan/learning-system/internal/policy"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
)

// Inputs is everything a projection needs besides the model.
type Inputs struct {
	Model      *learner.Model
	Resolver   *learner.VaultResolver
	Active     string
	Titles     map[string]string
	Positions  map[string]curriculum.Position
	DetourLogs map[string][]curriculum.DetourLogEntry
	Outlines   map[string]curriculum.Outline
	Statuses   map[string][]curriculum.NodeStatus
	Archived   map[string]bool
	Library    []string // imported, non-archived curricula in id order
	Recent     []RecentSession
	Today      string
	Next       *policy.Action
}

// RecentSession is one entry of the home page's recent sessions.
type RecentSession struct {
	ID         string
	Kind       string
	Curriculum string
	HasSession bool
}

// Gather loads curriculum context for every curriculum the model touches.
func Gather(root string, m *learner.Model) (Inputs, error) {
	in := Inputs{Model: m, Resolver: learner.NewResolver(root), Titles: map[string]string{},
		Positions: map[string]curriculum.Position{}, DetourLogs: map[string][]curriculum.DetourLogEntry{},
		Outlines: map[string]curriculum.Outline{}, Statuses: map[string][]curriculum.NodeStatus{}, Archived: map[string]bool{}}
	cfg, err := config.Load(root)
	if err != nil {
		return in, err
	}
	in.Active = cfg.Curriculum.Active
	in.Today = clock.Date(clock.Now())
	ids := map[string]bool{}
	for _, s := range m.Sessions {
		if s.Curriculum != "" {
			ids[s.Curriculum] = true
		}
	}
	for _, c := range m.Concepts {
		for _, r := range c.SourceRefs {
			ids[r.Curriculum] = true
		}
	}
	if in.Active != "" {
		ids[in.Active] = true
	}
	library, err := curriculum.List(root)
	if err != nil {
		return in, err
	}
	for _, man := range library {
		ids[man.ID] = true
		in.Library = append(in.Library, man.ID)
	}
	sort.Strings(in.Library)
	for id := range ids {
		manifest, err := curriculum.LoadManifest(root, id)
		if err != nil {
			in.Titles[id] = curriculum.ArchivedTitle(root, id) + "（已归档）"
			in.Archived[id] = true
			continue
		}
		in.Titles[id] = manifest.Title
		if pos, err := curriculum.LoadPosition(root, id); err == nil {
			in.Positions[id] = pos
		}
		logs, err := curriculum.LoadDetourLog(root, id)
		if err != nil {
			return in, err
		}
		in.DetourLogs[id] = logs
		outline, err := curriculum.LoadOutline(root, id)
		if err != nil {
			return in, err
		}
		entries, err := curriculum.LoadProgress(root, id)
		if err != nil {
			return in, err
		}
		in.Outlines[id] = outline
		in.Statuses[id] = curriculum.Statuses(outline, entries, in.Positions[id], m.NodesWithEvidence(id))
	}
	recent, err := recentSessions(root, 5)
	if err != nil {
		return in, err
	}
	in.Recent = recent
	if in.Active != "" {
		status, err := assessment.CurrentStatus(root, in.Active)
		if err != nil {
			return in, err
		}
		state, err := runtimeState.Load(root)
		if err != nil {
			return in, err
		}
		active := ""
		if state.ActiveSession != nil {
			active = state.ActiveSession.ID
		}
		skipped, err := conversation.BaselineSkipped(root, in.Active)
		if err != nil {
			return in, err
		}
		ctx := policy.Context{Curriculum: in.Active, Assessed: status.State == "assessed", BaselineSkipped: skipped,
			Position: in.Positions[in.Active], ActiveSession: active, Today: in.Today}
		if n, ok := curriculum.NextNode(in.Outlines[in.Active], in.Statuses[in.Active], in.Positions[in.Active]); ok {
			ctx.NextNode = &n
		}
		next := policy.Next(m, ctx)
		in.Next = &next
	}
	return in, nil
}

// File is one rendered projection.
type File struct {
	Path    string `json:"path"`
	Action  string `json:"action"` // create, update, unchanged
	content string
}

// Plan renders every projection and compares it with the disk.
func Plan(root string, in Inputs) ([]File, error) {
	rendered := map[string]string{}
	owned := func(rel string, body string) error {
		existing, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		keep := mdblock.PreservedUser(string(existing), strings.Contains(string(existing), "generated_by: learn"))
		rendered[rel] = body + "\n" + mdblock.UserSection(keep)
		return nil
	}
	if err := owned("README.md", renderHome(in)); err != nil {
		return nil, err
	}
	for id := range in.Titles {
		if in.Archived[id] {
			continue
		}
		if err := owned("Curriculum/"+id+"/index.md", renderCurriculum(in, id)); err != nil {
			return nil, err
		}
		progress, err := curriculum.ProgressMarkdown(root, id, in.Model.NodesWithEvidence(id))
		if err != nil {
			return nil, err
		}
		rendered["Curriculum/"+id+"/progress.md"] = progress
	}
	if !in.Model.Empty() {
		for _, c := range in.Model.ConceptList() {
			if err := owned("Concepts/"+c.ID+".md", renderConcept(in, c)); err != nil {
				return nil, err
			}
		}
		if err := owned("Profile/learner-state.md", renderOverview(in)); err != nil {
			return nil, err
		}
	}
	for id := range in.Model.Sessions {
		rel := "Sessions/" + id + ".md"
		existing, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		header := fmt.Sprintf("---\nid: %s\ngenerated_by: learn\n---\n\n# %s\n", id, sessionLabel(id))
		rendered[rel] = mdblock.Upsert(string(existing), "analysis", renderSessionAnalysis(in, id), header)
	}
	paths := make([]string, 0, len(rendered))
	for p := range rendered {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]File, 0, len(paths))
	for _, rel := range paths {
		f := File{Path: rel, content: rendered[rel], Action: "create"}
		if existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			f.Action = "update"
			if string(existing) == f.content {
				f.Action = "unchanged"
			}
		}
		files = append(files, f)
	}
	return files, nil
}

// Write applies a plan, skipping unchanged files.
func Write(root string, files []File) error {
	for _, f := range files {
		if f.Action == "unchanged" {
			continue
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(f.Path)), []byte(f.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Refresh gathers inputs, renders and writes every projection.
func Refresh(root string, m *learner.Model) ([]File, error) {
	in, err := Gather(root, m)
	if err != nil {
		return nil, err
	}
	files, err := Plan(root, in)
	if err != nil {
		return nil, err
	}
	return files, Write(root, files)
}

func recentSessions(root string, limit int) ([]RecentSession, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Conversations"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read conversations: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "session-") && strings.HasSuffix(e.Name(), ".md") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > limit {
		ids = ids[:limit]
	}
	var out []RecentSession
	for _, id := range ids {
		r := RecentSession{ID: id}
		if conv, err := conversation.Load(conversation.Path(root, id)); err == nil {
			r.Kind, r.Curriculum = conv.Kind, conv.Curriculum
		}
		if _, err := os.Stat(filepath.Join(root, "Sessions", id+".md")); err == nil {
			r.HasSession = true
		}
		out = append(out, r)
	}
	return out, nil
}

// Staleness compares the overview's generation with the current records.
func Staleness(root, generation string) string {
	data, err := os.ReadFile(filepath.Join(root, "Profile", "learner-state.md"))
	if err != nil {
		if generation == "empty" {
			return "none"
		}
		return "stale"
	}
	if strings.Contains(string(data), "model_generation: "+generation+"\n") {
		return "current"
	}
	return "stale"
}
