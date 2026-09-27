package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/gitx"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/mdblock"
	"github.com/hedykan/learning-system/internal/model"
	"github.com/hedykan/learning-system/internal/projection"
	"github.com/hedykan/learning-system/internal/record"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
)

type StartResult struct {
	ID           string `json:"id"`
	Conversation string `json:"conversation"`
	StartedAt    string `json:"started_at"`
	Kind         string `json:"kind"`
	Depth        string `json:"depth,omitempty"`
}

type EndResult struct {
	ID           string `json:"id"`
	Conversation string `json:"conversation"`
	Session      string `json:"session"`
	Events       int    `json:"events"`
	Kind         string `json:"kind"`
	Termination  string `json:"termination"`
	Assessment   string `json:"assessment,omitempty"`
	Record       string `json:"record,omitempty"`
	Projections  string `json:"projections,omitempty"`
	Git          string `json:"git"`
}

type StartOptions struct {
	Domain       string
	Kind         string
	Depth        string
	SkipBaseline bool
}

type EndOptions struct {
	Assessment       *assessment.Report
	Analysis         *record.Record
	NoAnalysis       bool
	NoAnalysisReason string
}

var allowedKinds = map[string]bool{"baseline": true, "lesson": true, "review": true, "practice": true}
var allowedDepths = map[string]bool{"quick": true, "standard": true, "deep": true}

func Start(root string, opts StartOptions, now time.Time) (StartResult, error) {
	state, err := runtimeState.Load(root)
	if err != nil {
		return StartResult{}, err
	}
	if state.ActiveSession != nil {
		return StartResult{}, fmt.Errorf("session %s is already active", state.ActiveSession.ID)
	}
	id := "session-" + now.UTC().Format("20060102-150405.000000000")
	rel := filepath.ToSlash(filepath.Join("Conversations", id+".md"))
	cfg, err := config.Load(root)
	if err != nil {
		return StartResult{}, err
	}
	status, err := assessment.CurrentStatus(root, cfg.Curriculum.Active)
	if err != nil {
		return StartResult{}, err
	}
	kind := opts.Kind
	if kind == "" {
		if cfg.Curriculum.Active != "" && status.State != "assessed" {
			kind = "baseline"
		} else {
			kind = "lesson"
		}
	}
	if !allowedKinds[kind] {
		return StartResult{}, fmt.Errorf("invalid session kind %q", kind)
	}
	if opts.SkipBaseline && kind != "lesson" {
		return StartResult{}, fmt.Errorf("--skip-baseline is only valid for lesson sessions")
	}
	if kind == "baseline" && cfg.Curriculum.Active == "" {
		return StartResult{}, fmt.Errorf("baseline session requires an active curriculum")
	}
	if kind == "lesson" && cfg.Curriculum.Active != "" && status.State != "assessed" && !opts.SkipBaseline {
		return StartResult{}, fmt.Errorf("curriculum has no baseline assessment; start a baseline or pass --skip-baseline explicitly")
	}
	depth := opts.Depth
	if kind == "baseline" {
		if depth == "" {
			depth = "standard"
		}
		if !allowedDepths[depth] {
			return StartResult{}, fmt.Errorf("invalid baseline depth %q", depth)
		}
	} else if depth != "" {
		return StartResult{}, fmt.Errorf("--depth is only valid for baseline sessions")
	}
	active := &runtimeState.ActiveSession{
		ID: id, StartedAt: now.UTC().Format(time.RFC3339Nano), Conversation: rel,
		Kind: kind, Depth: depth, BaselineSkipped: opts.SkipBaseline,
		Domain: opts.Domain, Curriculum: cfg.Curriculum.Active,
	}
	if active.Curriculum != "" {
		position, err := curriculum.LoadPosition(root, active.Curriculum)
		if err != nil {
			return StartResult{}, err
		}
		active.StartingChapter = position.Chapter
		active.StartingSection = position.Section
		active.StartingConcept = position.CurrentConcept
	}
	content := fmt.Sprintf("---\nid: %s\nstarted_at: %s\nkind: %s\ndepth: %q\nbaseline_skipped: %t\ndomain: %q\ncurriculum: %q\n---\n\n# Raw Conversation\n", id, active.StartedAt, active.Kind, active.Depth, active.BaselineSkipped, opts.Domain, active.Curriculum)
	if err := fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
		return StartResult{}, err
	}
	state.ActiveSession = active
	state.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	if err := runtimeState.Save(root, state); err != nil {
		return StartResult{}, err
	}
	return StartResult{ID: id, Conversation: rel, StartedAt: active.StartedAt, Kind: active.Kind, Depth: active.Depth}, nil
}

// AppendResult identifies the raw turn that was written.
type AppendResult struct {
	Turn string `json:"turn"`
	Role string `json:"role"`
}

func Append(root, role, content string, now time.Time) (AppendResult, error) {
	if role != "user" && role != "assistant" && role != "system" {
		return AppendResult{}, fmt.Errorf("invalid role %q", role)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return AppendResult{}, fmt.Errorf("conversation content is empty")
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		return AppendResult{}, err
	}
	if state.ActiveSession == nil {
		return AppendResult{}, fmt.Errorf("no active session; run 'learn session start' first")
	}
	path := filepath.Join(root, filepath.FromSlash(state.ActiveSession.Conversation))
	conv, err := conversation.Load(path)
	if err != nil {
		return AppendResult{}, err
	}
	ordinal := len(conv.Turns) + 1
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return AppendResult{}, fmt.Errorf("open active conversation: %w", err)
	}
	if _, err := f.WriteString(conversation.FormatEntry(ordinal, role, content, now)); err != nil {
		f.Close()
		return AppendResult{}, fmt.Errorf("append conversation: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return AppendResult{}, fmt.Errorf("sync conversation: %w", err)
	}
	if err := f.Close(); err != nil {
		return AppendResult{}, fmt.Errorf("close conversation: %w", err)
	}
	return AppendResult{Turn: conversation.TurnID(ordinal), Role: role}, nil
}

func End(ctx context.Context, root string, provider model.Provider, opts EndOptions, now time.Time) (EndResult, error) {
	state, err := runtimeState.Load(root)
	if err != nil {
		return EndResult{}, err
	}
	if state.ActiveSession == nil {
		return EndResult{}, fmt.Errorf("no active session")
	}
	active := state.ActiveSession
	conversationPath := filepath.Join(root, filepath.FromSlash(active.Conversation))
	conv, err := os.ReadFile(conversationPath)
	if err != nil {
		return EndResult{}, fmt.Errorf("read conversation: %w", err)
	}
	if active.Kind == "" {
		active.Kind = "lesson"
	}
	if opts.NoAnalysis && opts.Analysis != nil {
		return EndResult{}, fmt.Errorf("--no-analysis cannot be combined with --analysis-file")
	}
	if opts.NoAnalysis && strings.TrimSpace(opts.NoAnalysisReason) == "" {
		return EndResult{}, fmt.Errorf("--no-analysis requires --reason")
	}
	if active.Kind == "baseline" {
		if err := assessment.Validate(opts.Assessment, string(conv), active.Curriculum, active.Depth); err != nil {
			return EndResult{}, err
		}
	}
	hasRecords, err := learner.HasRecords(root, active.ID)
	if err != nil {
		return EndResult{}, err
	}
	if active.Kind != "baseline" && opts.Analysis == nil && !hasRecords && !opts.NoAnalysis {
		return EndResult{}, fmt.Errorf("%s session needs --analysis-file, an earlier checkpoint, or --no-analysis --reason", active.Kind)
	}
	result := EndResult{ID: active.ID, Conversation: active.Conversation, Kind: active.Kind, Termination: "completed", Git: "disabled"}
	var legacy *model.SessionAnalysis
	if opts.Analysis != nil {
		if err := anchorConcepts(root, active.Curriculum, opts.Analysis); err != nil {
			return EndResult{}, err
		}
		check := func(m *learner.Model) error { return relationGate(m, active) }
		if active.Kind == "baseline" {
			check = nil
		}
		submitted, err := learner.SubmitChecked(root, active.ID, "end", opts.Analysis, now, check)
		if err != nil {
			return EndResult{}, fmt.Errorf("validate interpretation record: %w", err)
		}
		result.Record = submitted.Record
		hasRecords = true
	}
	if opts.Analysis == nil && hasRecords && active.Kind != "baseline" {
		m, _, err := learner.Load(root)
		if err != nil {
			return EndResult{}, err
		}
		if err := relationGate(m, active); err != nil {
			return EndResult{}, err
		}
	}
	if !hasRecords {
		legacy, err = provider.AnalyzeSession(ctx, model.SessionAnalysisInput{
			SessionID: active.ID, Conversation: string(conv), Curriculum: active.Curriculum,
			Chapter: active.StartingChapter, Section: active.StartingSection, Concept: active.StartingConcept,
		})
		if err != nil {
			return EndResult{}, fmt.Errorf("analyze session: %w", err)
		}
		if err := model.Validate(legacy); err != nil {
			return EndResult{}, fmt.Errorf("validate session analysis: %w", err)
		}
		result.Events = len(legacy.Events)
	}
	result.Session, err = writeSessionFile(root, active, legacy, "completed", opts.NoAnalysisReason, now)
	if err != nil {
		return EndResult{}, err
	}
	if active.Kind == "baseline" {
		result.Assessment, err = assessment.Save(root, active.ID, *opts.Assessment, now)
		if err != nil {
			return EndResult{}, err
		}
	}
	if active.Curriculum != "" && active.Kind != "baseline" {
		if err := curriculum.RecordSession(root, active.Curriculum, active.ID, now); err != nil {
			return EndResult{}, err
		}
	}
	state.ActiveSession = nil
	state.LastSession = active.ID
	state.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	if err := runtimeState.Save(root, state); err != nil {
		return EndResult{}, err
	}
	result.Projections = refreshStatus(root)
	return commitResult(root, result, "learning: complete "+active.ID)
}

// relationGate refuses to end a session while concepts it touched have no
// related concept. The Agent decides relations; the Runtime only insists
// that the question was considered (CR-2026-013 addendum).
func relationGate(m *learner.Model, active *runtimeState.ActiveSession) error {
	missing, candidates := m.UnlinkedIn(active.ID, active.Curriculum)
	if len(missing) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("before ending, link the concepts from this session: add \"related\" to each concept below, or list it in \"no_related\" if none of the existing concepts is related.\n")
	for _, id := range missing {
		var opts []string
		for _, o := range candidates[id] {
			opts = append(opts, fmt.Sprintf("%s（%s）", o, m.Concepts[o].Label))
		}
		fmt.Fprintf(&b, "- %s（%s）: existing concepts %s\n", id, m.Concepts[id].Label, strings.Join(opts, "、"))
	}
	return fmt.Errorf("%s", strings.TrimRight(b.String(), "\n"))
}

// CheckpointResult reports a mid-session interpretation submission.
type CheckpointResult struct {
	Session     string `json:"session"`
	Status      string `json:"status"`
	Record      string `json:"record,omitempty"`
	Seq         int    `json:"seq"`
	Projections string `json:"projections"`
	Git         string `json:"git"`
}

// Checkpoint validates and stores an incremental record for the active
// session, then refreshes the model and projections. The session stays open.
func Checkpoint(root string, rec *record.Record, now time.Time) (CheckpointResult, error) {
	state, err := runtimeState.Load(root)
	if err != nil {
		return CheckpointResult{}, err
	}
	if state.ActiveSession == nil {
		return CheckpointResult{}, fmt.Errorf("no active session")
	}
	if err := anchorConcepts(root, state.ActiveSession.Curriculum, rec); err != nil {
		return CheckpointResult{}, err
	}
	return submitAndRefresh(root, state.ActiveSession.ID, "checkpoint", rec, now)
}

// anchorConcepts ties concepts to the current outline entry: a new concept
// without source_ref gets the current position, and a source_ref in the
// current chapter without a node gets the node. Detour concepts are left
// alone so prerequisites are not filed under the mainline chapter. It also
// checks that textbook points cite pages inside the concept's outline entry.
func anchorConcepts(root, curriculumID string, rec *record.Record) error {
	if curriculumID == "" || len(rec.Concepts) == 0 {
		return nil
	}
	pos, err := curriculum.LoadPosition(root, curriculumID)
	if err != nil {
		return err
	}
	outline, err := curriculum.LoadOutline(root, curriculumID)
	if err != nil {
		return err
	}
	m, _, err := learner.Load(root)
	if err != nil {
		return err
	}
	anchor := pos.Detour == nil && (pos.Node != "" || pos.Chapter != "")
	chapterNode := ""
	if pos.Node != "" {
		chapterNode = strings.Split(pos.Node, ".")[0]
	}
	for i := range rec.Concepts {
		c := &rec.Concepts[i]
		if anchor {
			switch {
			case c.SourceRef == nil:
				if _, exists := m.Concepts[c.ID]; !exists {
					c.SourceRef = &record.SourceRef{Node: pos.Node, Chapter: pos.Chapter, Section: pos.Section}
				}
			case c.SourceRef.Node == "" && pos.Node != "" && learner.SameChapter(c.SourceRef.Chapter, pos.Chapter):
				c.SourceRef.Node = chapterNode
				if c.SourceRef.Section == "" || c.SourceRef.Section == pos.Section || learner.SameChapter(c.SourceRef.Section, pos.Section) {
					c.SourceRef.Node = pos.Node
				}
			}
		}
		if err := checkPointsInNode(outline, curriculumID, m, c); err != nil {
			return err
		}
	}
	return nil
}

// checkPointsInNode rejects textbook points whose pages fall outside the
// concept's outline entry when a confirmed outline gives that entry pages.
func checkPointsInNode(o curriculum.Outline, curriculumID string, m *learner.Model, c *record.Concept) error {
	tp := c.TextbookPoints
	if tp == nil || o.Status != "confirmed" || len(tp.Pages) != 2 {
		return nil
	}
	node := ""
	if c.SourceRef != nil {
		node = c.SourceRef.Node
	}
	if existing, ok := m.Concepts[c.ID]; node == "" && ok {
		for _, r := range existing.SourceRefs {
			if r.Curriculum == curriculumID && r.Node != "" {
				node = r.Node
				break
			}
		}
	}
	n, ok := o.Find(node)
	if !ok || len(n.Pages) != 2 {
		return nil
	}
	if tp.Pages[0] < n.Pages[0] || tp.Pages[1] > n.Pages[1] {
		return fmt.Errorf("concept %s textbook_points pages %d-%d fall outside outline entry %s (pages %d-%d)", c.ID, tp.Pages[0], tp.Pages[1], n.ID, n.Pages[0], n.Pages[1])
	}
	return nil
}

// Annotate adds a record to a session that is no longer active, e.g. to
// backfill v0.1.1 conversations. It never changes the curriculum position.
func Annotate(root, sessionID string, rec *record.Record, now time.Time) (CheckpointResult, error) {
	state, err := runtimeState.Load(root)
	if err != nil {
		return CheckpointResult{}, err
	}
	if state.ActiveSession != nil && state.ActiveSession.ID == sessionID {
		return CheckpointResult{}, fmt.Errorf("session %s is active; use checkpoint instead", sessionID)
	}
	if _, err := os.Stat(conversation.Path(root, sessionID)); err != nil {
		return CheckpointResult{}, fmt.Errorf("session %s has no conversation", sessionID)
	}
	return submitAndRefresh(root, sessionID, "annotate", rec, now)
}

func submitAndRefresh(root, sessionID, kind string, rec *record.Record, now time.Time) (CheckpointResult, error) {
	submitted, err := learner.Submit(root, sessionID, kind, rec, now)
	if err != nil {
		return CheckpointResult{}, fmt.Errorf("validate interpretation record: %w", err)
	}
	result := CheckpointResult{Session: sessionID, Status: submitted.Status, Record: submitted.Record, Seq: submitted.Seq, Git: "disabled"}
	if submitted.Status == "unchanged" {
		result.Projections = "unchanged"
		return result, nil
	}
	if err := Refresh(root); err != nil {
		result.Projections = "failed"
		return result, fmt.Errorf("record %s was stored, but refreshing the learner model failed (run 'learn model rebuild'): %w", submitted.Record, err)
	}
	result.Projections = "updated"
	cfg, err := config.Load(root)
	if err != nil {
		return result, err
	}
	if cfg.Git.Enabled {
		if err := gitx.CommitAll(root, fmt.Sprintf("learning: %s %s", kind, sessionID)); err != nil {
			result.Git = "failed: " + err.Error()
		} else {
			result.Git = "committed"
		}
	}
	return result, nil
}

// Refresh replays all records, rewrites the model cache and projections.
func Refresh(root string) error {
	m, _, err := learner.Load(root)
	if err != nil {
		return err
	}
	if !m.Empty() {
		if err := learner.SaveCache(root, m); err != nil {
			return err
		}
	}
	_, err = projection.Refresh(root, m)
	return err
}

func refreshStatus(root string) string {
	if err := Refresh(root); err != nil {
		return "failed: " + err.Error()
	}
	return "updated"
}

// Turns lists the raw turns of a session (the active one by default).
func Turns(root, sessionID, role string) (string, []conversation.Turn, error) {
	if sessionID == "" {
		state, err := runtimeState.Load(root)
		if err != nil {
			return "", nil, err
		}
		if state.ActiveSession == nil {
			return "", nil, fmt.Errorf("no active session; pass --session")
		}
		sessionID = state.ActiveSession.ID
	}
	conv, err := conversation.Load(conversation.Path(root, sessionID))
	if err != nil {
		return "", nil, err
	}
	turns := []conversation.Turn{}
	for _, t := range conv.Turns {
		if role == "" || t.Role == role {
			turns = append(turns, t)
		}
	}
	return sessionID, turns, nil
}

func Abort(root, reason string, now time.Time) (EndResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return EndResult{}, fmt.Errorf("abort reason is required")
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		return EndResult{}, err
	}
	if state.ActiveSession == nil {
		return EndResult{}, fmt.Errorf("no active session")
	}
	active := state.ActiveSession
	if active.Kind == "" {
		active.Kind = "lesson"
	}
	if _, err := Append(root, "system", "Session aborted: "+reason, now); err != nil {
		return EndResult{}, err
	}
	hasRecords, err := learner.HasRecords(root, active.ID)
	if err != nil {
		return EndResult{}, err
	}
	var analysis *model.SessionAnalysis
	if !hasRecords {
		analysis = &model.SessionAnalysis{NextProbe: "Resolve the abort reason before starting another learning session."}
	}
	sessionRel, err := writeSessionFile(root, active, analysis, "aborted", reason, now)
	if err != nil {
		return EndResult{}, err
	}
	state.ActiveSession = nil
	state.LastSession = active.ID
	state.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	if err := runtimeState.Save(root, state); err != nil {
		return EndResult{}, err
	}
	result := EndResult{
		ID: active.ID, Conversation: active.Conversation, Session: sessionRel,
		Kind: active.Kind, Termination: "aborted", Git: "disabled",
	}
	result.Projections = refreshStatus(root)
	return commitResult(root, result, "learning: abort "+active.ID)
}

// writeSessionFile renders the Session header, the legacy v0.1 analysis when
// no Interpretation Record exists, and keeps the analysis and user blocks.
func writeSessionFile(root string, active *runtimeState.ActiveSession, legacy *model.SessionAnalysis, termination, reason string, now time.Time) (string, error) {
	rel := filepath.ToSlash(filepath.Join("Sessions", active.ID+".md"))
	path := filepath.Join(root, filepath.FromSlash(rel))
	existing, _ := os.ReadFile(path)
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %s\ndate: %s\nkind: %s\ntermination: %s\ndepth: %q\ndomain: %q\nsource_conversation: %q\ncurriculum: %q\ngenerated_by: learn\n---\n\n", active.ID, now.UTC().Format("2006-01-02"), active.Kind, termination, active.Depth, active.Domain, active.Conversation, active.Curriculum)
	fmt.Fprintf(&b, "# %s\n\n## Starting Point\n\nChapter: %s  \nSection: %s  \nConcept: %s\n\n", active.ID, valueOrNone(active.StartingChapter), valueOrNone(active.StartingSection), valueOrNone(active.StartingConcept))
	if termination == "aborted" {
		fmt.Fprintf(&b, "## Termination\n\nAborted: %s\n\n", reason)
	} else if reason != "" {
		fmt.Fprintf(&b, "## Termination\n\nEnded without interpretation: %s\n\n", reason)
	}
	if legacy != nil {
		renderLegacyAnalysis(&b, legacy)
		b.WriteString("\n")
	}
	if inner, ok := mdblock.Extract(string(existing), "analysis"); ok {
		b.WriteString(mdblock.Block("analysis", inner) + "\n")
	}
	b.WriteString(mdblock.UserSection(mdblock.PreservedUser(string(existing), true)))
	if err := fsutil.WriteFileAtomic(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

func renderLegacyAnalysis(b *strings.Builder, analysis *model.SessionAnalysis) {
	b.WriteString("## Learning Events\n\n")
	if len(analysis.Events) == 0 {
		b.WriteString("No validated learning events.\n\n")
	}
	for _, event := range analysis.Events {
		fmt.Fprintf(b, "- **%s**: %s\n  - Evidence: %s\n", event.Type, event.Summary, event.Evidence)
	}
	b.WriteString("\n## Cognitive Changes\n\n")
	if len(analysis.CognitiveChanges) == 0 {
		b.WriteString("No validated cognitive change.\n")
	}
	for _, change := range analysis.CognitiveChanges {
		fmt.Fprintf(b, "- %s → (%s) → %s\n", change.OldUnderstanding, change.Trigger, change.NewUnderstanding)
	}
	b.WriteString("\n## Open Loops\n\n")
	if len(analysis.OpenLoops) == 0 {
		b.WriteString("None recorded.\n")
	}
	for _, loop := range analysis.OpenLoops {
		fmt.Fprintf(b, "- %s\n", loop)
	}
	fmt.Fprintf(b, "\n## Next Probe\n\n%s\n", analysis.NextProbe)
}

func commitResult(root string, result EndResult, message string) (EndResult, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return result, err
	}
	if cfg.Git.Enabled {
		if err := gitx.CommitAll(root, message); err != nil {
			result.Git = "failed: " + err.Error()
		} else {
			result.Git = "committed"
		}
	}
	return result, nil
}

func valueOrNone(value string) string {
	if value == "" {
		return "Not set"
	}
	return value
}
