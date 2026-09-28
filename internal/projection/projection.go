// Package projection renders the Learner Model as human-readable Markdown.
// Output depends only on records, curriculum state and resolver data, so a
// rebuild reproduces the same bytes; the home page's due reviews and Git
// reminder are the exceptions and follow the clock and the Git state.
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
	"github.com/hedykan/learning-system/internal/gitx"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/mdblock"
	"github.com/hedykan/learning-system/internal/policy"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/tags"
)

// Inputs is everything a projection needs besides the model.
type Inputs struct {
	Model      *learner.Model
	Root       string
	Lang       string // interface language of fixed text (CR-2026-023)
	Resolver   *learner.VaultResolver
	Active     string
	Titles     map[string]string
	Positions  map[string]curriculum.Position
	DetourLogs map[string][]curriculum.DetourLogEntry
	Outlines   map[string]curriculum.Outline
	Resources  map[string]curriculum.ResourceSet
	Statuses   map[string][]curriculum.NodeStatus
	Archived   map[string]bool
	Library    []string // imported, non-archived curricula in id order
	Recent     []RecentSession
	Today      string
	Next       *policy.Action
	Git        *GitReminder // nil when history is safely committed
}

// GitReminder describes Vault changes not yet saved to Git (CR-2026-024).
type GitReminder struct {
	Uncommitted int
	LastCommit  string // ISO time of HEAD, "" when never committed
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
	in := Inputs{Model: m, Root: root, Resolver: learner.NewResolver(root), Titles: map[string]string{},
		Positions: map[string]curriculum.Position{}, DetourLogs: map[string][]curriculum.DetourLogEntry{},
		Outlines: map[string]curriculum.Outline{}, Resources: map[string]curriculum.ResourceSet{}, Statuses: map[string][]curriculum.NodeStatus{}, Archived: map[string]bool{}}
	cfg, err := config.Load(root)
	if err != nil {
		return in, err
	}
	in.Active = cfg.Curriculum.Active
	in.Lang = i18n.Normalize(cfg.Language)
	if cfg.Git.Enabled && gitx.IsRepo(root) && gitx.LoadAuto(root).Result != "committed" {
		if n, err := gitx.Uncommitted(root); err == nil && n > 0 {
			in.Git = &GitReminder{Uncommitted: n, LastCommit: gitx.LastCommit(root)}
		}
	}
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
			in.Titles[id] = curriculum.ArchivedTitle(root, id) + in.t("（已归档）")
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
		if set, err := curriculum.LoadResourceSet(root, id); err == nil {
			in.Resources[id] = set
		}
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
			Position: in.Positions[in.Active], ActiveSession: active, Today: in.Today, Lang: in.Lang,
			OutlineStatus: in.Outlines[in.Active].Status}
		if n, ok := curriculum.NextNode(in.Outlines[in.Active], in.Statuses[in.Active], in.Positions[in.Active]); ok {
			ctx.NextNode = &n
		}
		next := policy.Next(m, ctx)
		node := next.Node
		if node == "" {
			node = in.Positions[in.Active].Node
		}
		if res, err := curriculum.NodeResources(root, in.Active, node, in.Lang); err == nil && len(res) > 0 {
			next.Resources = res
		}
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
	orphans := findOrphans(root, in)
	owned := func(rel string, body string) {
		existing, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		keep := mdblock.PreservedUser(string(existing), strings.Contains(string(existing), "generated_by: learn"))
		if strings.TrimSpace(keep) == "" {
			keep = orphans.userBlocks[rel]
		}
		rendered[rel] = body + "\n" + mdblock.UserSectionIn(in.Lang, keep)
	}
	owned("README.md", renderHome(in))
	for id := range in.Titles {
		if in.Archived[id] {
			continue
		}
		owned(CurriculumIndexFile(id, in.Titles[id]), renderCurriculum(in, id))
		progress, err := curriculum.ProgressMarkdown(root, id, in.Model.NodesWithEvidence(id), in.Lang)
		if err != nil {
			return nil, err
		}
		rendered[CurriculumProgressFile(id, in.Lang)] = progress
	}
	if !in.Model.Empty() {
		for _, c := range in.Model.ConceptList() {
			owned(ConceptFile(in.Model, c.ID), renderConcept(in, c))
		}
		owned(OverviewFile(in.Lang), renderOverview(in))
		for _, q := range in.Model.QuestionList() {
			owned(QuestionFile(in.Model, q.ID), renderQuestion(in, q))
		}
	}
	for id := range in.Model.Sessions {
		rel := "Sessions/" + id + ".md"
		existing, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		header := fmt.Sprintf("---\nid: %s\ngenerated_by: learn\n%s---\n\n", id, tags.Lines(tags.Session)) + i18n.F(in.Lang, "# 学习记录 %s\n", sessionLabel(id))
		text, _ := tags.Ensure(mdblock.UpsertIn(in.Lang, string(existing), "analysis", renderSessionAnalysis(in, id), header), tags.Session)
		rendered[rel] = text
	}
	paths := make([]string, 0, len(rendered))
	for p := range rendered {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]File, 0, len(paths)+len(orphans.remove))
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
	for _, rel := range orphans.remove {
		if _, kept := rendered[rel]; !kept {
			files = append(files, File{Path: rel, Action: "remove"})
		}
	}
	return files, nil
}

type orphanSet struct {
	userBlocks map[string]string // expected path -> user block carried over
	remove     []string
}

// findOrphans locates generated notes whose file name no longer matches the
// naming rules (for example v0.1.6 notes named by internal IDs). Their user
// blocks move to the expected file; only files marked generated_by: learn
// are ever removed.
func findOrphans(root string, in Inputs) orphanSet {
	o := orphanSet{userBlocks: map[string]string{}}
	adopt := func(rel, expected string) {
		if rel == expected || expected == "" {
			return
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !strings.Contains(string(data), "generated_by: learn") {
			return
		}
		if block, ok := mdblock.Extract(string(data), "user"); ok && strings.TrimSpace(block) != "" && o.userBlocks[expected] == "" {
			o.userBlocks[expected] = block
		}
		o.remove = append(o.remove, rel)
	}
	frontValue := func(data, key string) string {
		if !strings.HasPrefix(data, "---\n") {
			return ""
		}
		rest := data[len("---\n"):]
		end := strings.Index(rest, "\n---\n")
		if end < 0 {
			return ""
		}
		for _, line := range strings.Split(rest[:end], "\n") {
			if strings.HasPrefix(line, key+": ") {
				return strings.TrimSpace(strings.TrimPrefix(line, key+": "))
			}
		}
		return ""
	}
	scan := func(dir string, expected func(data string) string) {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			rel := dir + "/" + e.Name()
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				continue
			}
			adopt(rel, expected(string(data)))
		}
	}
	m := in.Model
	scan("Concepts", func(d string) string {
		if id := frontValue(d, "concept"); m.Concepts[id] != nil {
			return ConceptFile(m, id)
		}
		return ""
	})
	scan("Questions", func(d string) string {
		if id := frontValue(d, "question"); m.Questions[id] != nil {
			return QuestionFile(m, id)
		}
		return ""
	})
	scan("Profile", func(d string) string {
		if frontValue(d, "projection") == "learner-overview" {
			return OverviewFile(in.Lang)
		}
		return ""
	})
	for id := range in.Titles {
		if in.Archived[id] {
			continue
		}
		dir := "Curriculum/" + id
		scan(dir, func(d string) string {
			switch {
			case frontValue(d, "projection") == "curriculum-index":
				return CurriculumIndexFile(id, in.Titles[id])
			case curriculum.IsProgressPage(d):
				return CurriculumProgressFile(id, in.Lang)
			}
			return ""
		})
	}
	sort.Strings(o.remove)
	return o
}

// Write applies a plan, skipping unchanged files.
func Write(root string, files []File) error {
	for _, f := range files {
		if f.Action == "unchanged" {
			continue
		}
		if f.Action == "remove" {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(f.Path))); err != nil && !os.IsNotExist(err) {
				return err
			}
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
	var data []byte
	var err error
	for _, rel := range overviewCandidates() {
		if data, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			break
		}
	}
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
