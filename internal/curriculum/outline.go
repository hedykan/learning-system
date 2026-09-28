package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/source"
	"github.com/hedykan/learning-system/internal/tags"
	"gopkg.in/yaml.v3"
)

// Node is one outline entry, identified by a dotted number such as "2.3".
type Node struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Pages []int  `yaml:"pages,omitempty" json:"pages,omitempty"`
	// Locator places the entry in its resource when pages do not apply
	// (CR-2026-025); give it or pages, not both.
	Locator *locator.Locator `yaml:"locator,omitempty" json:"locator,omitempty"`
}

// Where is the entry's position as a locator, from pages or the locator.
func (n Node) Where() (locator.Locator, bool) {
	switch {
	case n.Locator != nil:
		return *n.Locator, true
	case len(n.Pages) == 2:
		return locator.FromPages(n.Pages), true
	}
	return locator.Locator{}, false
}

// Outline is the confirmed or draft table of contents of a curriculum.
type Outline struct {
	Version int    `yaml:"version" json:"version"`
	Status  string `yaml:"status" json:"status"` // missing, draft, confirmed
	Nodes   []Node `yaml:"nodes" json:"nodes"`
}

var nodeIDPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

func outlinePath(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "outline.yaml")
}

// LoadOutline reads outline.yaml; the v0.1 `chapters: []` placeholder and a
// missing file both load as an empty outline with status "missing".
func LoadOutline(root, id string) (Outline, error) {
	data, err := os.ReadFile(outlinePath(root, id))
	if os.IsNotExist(err) {
		return Outline{Version: 2, Status: "missing"}, nil
	}
	if err != nil {
		return Outline{}, fmt.Errorf("read outline: %w", err)
	}
	var o Outline
	if err := yaml.Unmarshal(data, &o); err != nil {
		return Outline{}, fmt.Errorf("parse outline: %w", err)
	}
	if len(o.Nodes) == 0 {
		return Outline{Version: 2, Status: "missing"}, nil
	}
	return o, nil
}

func saveOutline(root, id string, o Outline) error {
	o.Version = 2
	data, err := yaml.Marshal(o)
	if err != nil {
		return fmt.Errorf("encode outline: %w", err)
	}
	return fsutil.WriteFileAtomic(outlinePath(root, id), data, 0o644)
}

func segments(id string) []int {
	parts := strings.Split(id, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

func lessID(a, b string) bool {
	x, y := segments(a), segments(b)
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

func parentID(id string) string {
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[:i]
	}
	return ""
}

// Validate checks IDs, hierarchy, document order and page ranges.
func (o Outline) Validate() error {
	if len(o.Nodes) == 0 {
		return fmt.Errorf("outline has no entries")
	}
	seen := map[string]Node{}
	prev, prevStart := "", 0
	for i, n := range o.Nodes {
		where := fmt.Sprintf("outline entry %d (%s)", i+1, n.ID)
		if !nodeIDPattern.MatchString(n.ID) {
			return fmt.Errorf("%s: id must be dotted numbers such as 2 or 2.3", where)
		}
		if _, dup := seen[n.ID]; dup {
			return fmt.Errorf("%s: duplicate id", where)
		}
		if strings.TrimSpace(n.Title) == "" {
			return fmt.Errorf("%s: title is required", where)
		}
		if prev != "" && !lessID(prev, n.ID) {
			return fmt.Errorf("%s: entries must follow document order (after %s)", where, prev)
		}
		parent, hasParent := seen[parentID(n.ID)]
		if parentID(n.ID) != "" && !hasParent {
			return fmt.Errorf("%s: parent %s must appear first", where, parentID(n.ID))
		}
		if n.Locator != nil {
			if len(n.Pages) != 0 {
				return fmt.Errorf("%s: give pages or locator, not both", where)
			}
			if err := n.Locator.Validate(); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			if outer, ok := parent.Where(); hasParent && ok {
				if inside, comparable := n.Locator.Within(outer); comparable && !inside {
					return fmt.Errorf("%s: locator %s falls outside parent %s", where, n.Locator.Value, parent.ID)
				}
			}
		}
		if len(n.Pages) != 0 {
			if len(n.Pages) != 2 || n.Pages[0] < 1 || n.Pages[0] > n.Pages[1] {
				return fmt.Errorf("%s: pages must be [start, end] with 1 <= start <= end", where)
			}
			if n.Pages[0] < prevStart {
				return fmt.Errorf("%s: pages start before the previous entry", where)
			}
			if hasParent && len(parent.Pages) == 2 && (n.Pages[0] < parent.Pages[0] || n.Pages[1] > parent.Pages[1]) {
				return fmt.Errorf("%s: pages fall outside parent %s", where, parent.ID)
			}
			prevStart = n.Pages[0]
		}
		seen[n.ID] = n
		prev = n.ID
	}
	return nil
}

// Find returns a node by ID.
func (o Outline) Find(id string) (Node, bool) {
	for _, n := range o.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// Label renders "2.3 标题".
func (n Node) Label() string { return n.ID + " " + n.Title }

// ChapterOf returns the top-level node that contains id.
func (o Outline) ChapterOf(id string) (Node, bool) {
	return o.Find(strings.Split(id, ".")[0])
}

func (o Outline) hasChildren(id string) bool {
	for _, n := range o.Nodes {
		if parentID(n.ID) == id {
			return true
		}
	}
	return false
}

// SetOutline validates a submitted outline and stores it as a draft.
func SetOutline(root, id string, data []byte, dryRun bool) (Outline, error) {
	if _, err := LoadManifest(root, id); err != nil {
		return Outline{}, err
	}
	var in Outline
	if err := yaml.Unmarshal(data, &in); err != nil {
		return Outline{}, fmt.Errorf("parse outline: %w", err)
	}
	in.Version, in.Status = 2, "draft"
	if err := in.Validate(); err != nil {
		return Outline{}, err
	}
	// Entry locators point into the curriculum's own material; the adapter
	// knows which kinds make sense (pages for PDF, anchors for Markdown...).
	if primary, err := Resolve(root, id); err == nil {
		for _, n := range in.Nodes {
			if n.Locator == nil {
				continue
			}
			if n.Locator.Resource != "" && n.Locator.Resource != id {
				return Outline{}, fmt.Errorf("outline entry %s: locators point into the curriculum's own material; attach other resources with `learn source attach`", n.ID)
			}
			if err := primary.Validate(*n.Locator); err != nil {
				return Outline{}, fmt.Errorf("outline entry %s: %w", n.ID, err)
			}
		}
	}
	if dryRun {
		return in, nil
	}
	if err := saveOutline(root, id, in); err != nil {
		return Outline{}, err
	}
	return in, RenderProgress(root, id)
}

// ConfirmOutline marks a draft outline as confirmed by the learner.
func ConfirmOutline(root, id string) (Outline, error) {
	o, err := LoadOutline(root, id)
	if err != nil {
		return Outline{}, err
	}
	if o.Status == "missing" {
		return Outline{}, fmt.Errorf("curriculum %s has no outline to confirm", id)
	}
	if err := o.Validate(); err != nil {
		return Outline{}, err
	}
	o.Status = "confirmed"
	if err := saveOutline(root, id, o); err != nil {
		return Outline{}, err
	}
	return o, RenderProgress(root, id)
}

// MarkdownOutline derives draft outline entries from level 1 and 2 headings
// of a Markdown file or folder; other formats yield no entries.
func MarkdownOutline(path string) ([]Node, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	a, err := source.Detect(path, info)
	if err != nil || !a.Capabilities().Structured {
		return nil, nil
	}
	sections, err := a.Outline(path)
	if err == source.ErrNoStructure {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return NodesFromSections(sections, filepath.Base(path)), nil
}

// NodesFromSections numbers headings as chapters (level 1) and sections
// (level 2); sections before any chapter go under a chapter named after
// their file.
func NodesFromSections(sections []source.Section, fallback string) []Node {
	var nodes []Node
	chapter, section := 0, 0
	for _, s := range sections {
		if s.Level == 1 {
			chapter, section = chapter+1, 0
			nodes = append(nodes, Node{ID: strconv.Itoa(chapter), Title: s.Title})
			continue
		}
		if chapter == 0 {
			chapter = 1
			name := fallback
			if s.File != "" {
				name = filepath.Base(s.File)
			}
			nodes = append(nodes, Node{ID: "1", Title: name})
		}
		section++
		nodes = append(nodes, Node{ID: fmt.Sprintf("%d.%d", chapter, section), Title: s.Title})
	}
	return nodes
}

// ProgressEntry is one append-only curriculum progress event.
type ProgressEntry struct {
	Kind    string `yaml:"kind" json:"kind"` // completed, skipped, session, legacy
	Node    string `yaml:"node,omitempty" json:"node,omitempty"`
	Title   string `yaml:"title,omitempty" json:"title,omitempty"`
	Reason  string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Session string `yaml:"session,omitempty" json:"session,omitempty"`
	At      string `yaml:"at" json:"at"`
	Text    string `yaml:"text,omitempty" json:"text,omitempty"`
}

func progressPath(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "progress.yaml")
}

// LoadProgress reads progress.yaml. On first use it adopts an older
// progress.md as a legacy entry so no history is lost.
func LoadProgress(root, id string) ([]ProgressEntry, error) {
	data, err := os.ReadFile(progressPath(root, id))
	if err == nil {
		var entries []ProgressEntry
		if err := yaml.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("parse progress: %w", err)
		}
		return entries, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read progress: %w", err)
	}
	old, err := os.ReadFile(filepath.Join(root, "Curriculum", id, "progress.md"))
	if err != nil || strings.Contains(string(old), "generated_by: learn") {
		return nil, nil
	}
	var kept []string
	for _, line := range strings.Split(string(old), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "# ") || t == "No completed steps yet." {
			continue
		}
		kept = append(kept, t)
	}
	if len(kept) == 0 {
		return nil, nil
	}
	return []ProgressEntry{{Kind: "legacy", Text: strings.Join(kept, "\n")}}, nil
}

func appendProgress(root, id string, entry ProgressEntry) error {
	entries, err := LoadProgress(root, id)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(append(entries, entry))
	if err != nil {
		return fmt.Errorf("encode progress: %w", err)
	}
	if err := fsutil.WriteFileAtomic(progressPath(root, id), data, 0o644); err != nil {
		return err
	}
	return RenderProgress(root, id)
}

// Mark records a completed or skipped outline entry.
func Mark(root, id, node, kind, reason, session string, now time.Time) (ProgressEntry, error) {
	if kind != "completed" && kind != "skipped" {
		return ProgressEntry{}, fmt.Errorf("unknown progress kind %q", kind)
	}
	if strings.TrimSpace(reason) == "" {
		return ProgressEntry{}, fmt.Errorf("--reason is required")
	}
	o, err := LoadOutline(root, id)
	if err != nil {
		return ProgressEntry{}, err
	}
	n, ok := o.Find(node)
	if !ok {
		return ProgressEntry{}, fmt.Errorf("outline of %s has no entry %q", id, node)
	}
	entry := ProgressEntry{Kind: kind, Node: n.ID, Title: n.Title, Reason: reason, Session: session, At: now.UTC().Format(time.RFC3339Nano)}
	return entry, appendProgress(root, id, entry)
}

// AdvanceIfCurrent moves the position to the next unfinished entry when the
// entry just marked is the current position and no detour is open. It
// returns the new node, or nil when the position did not move (CR-2026-017).
func AdvanceIfCurrent(root, id, marked string) (*Node, error) {
	pos, err := LoadPosition(root, id)
	if err != nil {
		return nil, err
	}
	if pos.Detour != nil || pos.Node != marked {
		return nil, nil
	}
	o, err := LoadOutline(root, id)
	if err != nil {
		return nil, err
	}
	entries, err := LoadProgress(root, id)
	if err != nil {
		return nil, err
	}
	next, ok := NextNode(o, Statuses(o, entries, pos, nil), pos)
	if !ok || next.ID == pos.Node {
		return nil, nil
	}
	moved, err := PositionAtNode(o, pos, next.ID)
	if err != nil {
		return nil, err
	}
	moved.CurrentConcept = ""
	if err := SavePosition(root, id, moved); err != nil {
		return nil, err
	}
	return &next, RenderProgress(root, id)
}

// RecordSession logs a finished session at the current position.
func RecordSession(root, id, session string, now time.Time) error {
	pos, err := LoadPosition(root, id)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(strings.Join([]string{pos.Chapter, pos.Section}, " / "))
	return appendProgress(root, id, ProgressEntry{Kind: "session", Node: pos.Node, Title: strings.Trim(title, " /"), Session: session, At: now.UTC().Format(time.RFC3339Nano)})
}

// NodeStatus is the derived curriculum status of one outline entry.
type NodeStatus struct {
	Node
	Depth  int    `json:"depth"`
	Status string `json:"status"` // completed, skipped, in_progress, partial, uncovered, not_started
}

// SessionNodes returns the outline entries where a session ended.
func SessionNodes(entries []ProgressEntry) map[string]bool {
	out := map[string]bool{}
	for _, e := range entries {
		if e.Kind == "session" && e.Node != "" {
			out[e.Node] = true
		}
	}
	return out
}

// Statuses derives each entry's status from progress, the position and the
// entries that already have learning evidence (touched).
func Statuses(o Outline, entries []ProgressEntry, pos Position, touched map[string]bool) []NodeStatus {
	studied := SessionNodes(entries)
	for id := range touched {
		studied[id] = true
	}
	partial := func(id string) bool {
		for s := range studied {
			if s == id || strings.HasPrefix(s, id+".") {
				return true
			}
		}
		return false
	}
	recorded := map[string]string{}
	for _, e := range entries {
		if e.Kind == "completed" || e.Kind == "skipped" {
			recorded[e.Node] = e.Kind
		}
	}
	current := pos.Node
	if _, ok := o.Find(current); !ok {
		current = ""
	}
	var covered func(id string) string
	covered = func(id string) string {
		if k := recorded[id]; k != "" {
			return k
		}
		if !o.hasChildren(id) {
			return ""
		}
		kind := "completed"
		for _, n := range o.Nodes {
			if parentID(n.ID) != id {
				continue
			}
			c := covered(n.ID)
			if c == "" {
				return ""
			}
			if c == "skipped" {
				kind = "skipped"
			}
		}
		return kind
	}
	out := make([]NodeStatus, 0, len(o.Nodes))
	for _, n := range o.Nodes {
		st := NodeStatus{Node: n, Depth: len(segments(n.ID))}
		inherited := ""
		for p := parentID(n.ID); p != "" && inherited == ""; p = parentID(p) {
			inherited = recorded[p]
		}
		switch {
		case covered(n.ID) != "":
			st.Status = covered(n.ID)
		case inherited != "":
			st.Status = inherited
		case current != "" && (n.ID == current || strings.HasPrefix(current, n.ID+".")):
			st.Status = "in_progress"
		case partial(n.ID):
			st.Status = "partial"
		case current != "" && lessID(n.ID, current):
			st.Status = "uncovered"
		default:
			st.Status = "not_started"
		}
		out = append(out, st)
	}
	return out
}

// Uncovered lists the topmost entries before the position that were never
// completed or skipped.
func Uncovered(statuses []NodeStatus) []NodeStatus {
	var out []NodeStatus
	flagged := map[string]bool{}
	for _, s := range statuses {
		if s.Status != "uncovered" {
			continue
		}
		flagged[s.ID] = true
		if !flagged[parentID(s.ID)] {
			out = append(out, s)
		}
	}
	return out
}

// Partial lists the topmost partially studied entries.
func Partial(statuses []NodeStatus) []NodeStatus {
	var out []NodeStatus
	flagged := map[string]bool{}
	for _, s := range statuses {
		if s.Status != "partial" {
			continue
		}
		flagged[s.ID] = true
		if !flagged[parentID(s.ID)] {
			out = append(out, s)
		}
	}
	return out
}

// NextNode returns the first partially studied leaf before the position, or
// else the first leaf from the position onward (the current entry included)
// that is neither completed nor skipped.
func NextNode(o Outline, statuses []NodeStatus, pos Position) (Node, bool) {
	for _, s := range statuses {
		if s.Status == "partial" && !o.hasChildren(s.ID) && pos.Node != "" && lessID(s.ID, pos.Node) {
			return s.Node, true
		}
	}
	for _, s := range statuses {
		if o.hasChildren(s.ID) || s.Status == "completed" || s.Status == "skipped" {
			continue
		}
		if pos.Node != "" && lessID(s.ID, pos.Node) {
			continue
		}
		return s.Node, true
	}
	return Node{}, false
}

// StatusLabel is the human label of each entry status.
var StatusLabel = map[string]string{"completed": "✅ 已完成", "skipped": "↷ 已跳过", "in_progress": "▶ 进行中", "partial": "◐ 学过一部分", "uncovered": "⚠ 未覆盖", "not_started": "○ 未开始"}

// ProgressFile is the generated progress page inside a curriculum folder.
func ProgressFile(lang string) string { return i18n.T(lang, "学习进度") + ".md" }

// IsProgressPage recognizes a generated progress page in any language.
func IsProgressPage(data string) bool {
	for _, l := range i18n.Languages {
		if strings.Contains(data, "\n"+i18n.T(l, "# 学习进度 — ")) {
			return true
		}
	}
	return false
}

// RenderProgress regenerates the progress page using session evidence only; the
// full projection refresh re-renders it with concept evidence as well.
func RenderProgress(root, id string) error {
	lang := i18n.Normalize("")
	if cfg, err := config.Load(root); err == nil {
		lang = i18n.Normalize(cfg.Language)
	}
	content, err := ProgressMarkdown(root, id, nil, lang)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(root, "Curriculum", id, ProgressFile(lang)), []byte(content), 0o644)
}

// ProgressMarkdown renders progress.md; touched adds entries with evidence.
func ProgressMarkdown(root, id string, touched map[string]bool, lang string) (string, error) {
	t := func(s string) string { return i18n.T(lang, s) }
	o, err := LoadOutline(root, id)
	if err != nil {
		return "", err
	}
	entries, err := LoadProgress(root, id)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(progressPath(root, id)); os.IsNotExist(statErr) && len(entries) > 0 {
		// Persist adopted legacy history before progress.md is regenerated.
		data, err := yaml.Marshal(entries)
		if err != nil {
			return "", fmt.Errorf("encode progress: %w", err)
		}
		if err := fsutil.WriteFileAtomic(progressPath(root, id), data, 0o644); err != nil {
			return "", err
		}
	}
	pos, err := LoadPosition(root, id)
	if err != nil {
		return "", err
	}
	title := id
	if m, err := LoadManifest(root, id); err == nil {
		title = m.Title
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\ngenerated_by: learn\ncurriculum: %s\n%s---\n\n%s%s\n\n", id, tags.Lines(tags.Progress), t("# 学习进度 — "), title)
	b.WriteString(t("> 由 Learning OS 根据目录与完成记录生成。教材进度与理解程度相互独立。\n\n## 目录\n\n"))
	if o.Status == "missing" {
		b.WriteString(t("尚未建立目录。\n"))
	} else {
		if o.Status == "draft" {
			b.WriteString(t("目录为草稿，尚未经学习者确认。\n\n"))
		}
		for _, s := range Statuses(o, entries, pos, touched) {
			fmt.Fprintf(&b, "%s- %s %s\n", strings.Repeat("  ", s.Depth-1), t(StatusLabel[s.Status]), s.Label())
		}
	}
	b.WriteString(t("\n## 记录\n\n"))
	if len(entries) == 0 {
		b.WriteString(t("暂无。\n"))
	}
	for _, e := range entries {
		date := e.At
		if len(date) >= 10 {
			date = date[:10]
		}
		switch e.Kind {
		case "completed", "skipped":
			verb := t(map[string]string{"completed": "完成", "skipped": "跳过"}[e.Kind])
			fmt.Fprintf(&b, t("- %s %s %s %s：%s\n"), date, verb, e.Node, e.Title, e.Reason)
		case "session":
			fmt.Fprintf(&b, t("- %s 结束学习 `%s`（位置：%s）\n"), date, e.Session, t(orUnset(e.Title)))
		case "legacy":
			fmt.Fprintf(&b, t("- 旧记录：%s\n"), strings.ReplaceAll(e.Text, "\n", t("；")))
		}
	}
	return b.String(), nil
}

func orUnset(s string) string {
	if strings.TrimSpace(s) == "" {
		return "未设置"
	}
	return s
}

// PositionVerified reports whether the position points at a node of a
// confirmed outline.
func PositionVerified(o Outline, pos Position) bool {
	if o.Status != "confirmed" || pos.Node == "" {
		return false
	}
	_, ok := o.Find(pos.Node)
	return ok
}

// PositionAtNode fills chapter and section labels from the outline.
func PositionAtNode(o Outline, pos Position, node string) (Position, error) {
	n, ok := o.Find(node)
	if !ok {
		return pos, fmt.Errorf("outline has no entry %q", node)
	}
	ch, _ := o.ChapterOf(node)
	pos.Node, pos.Chapter, pos.Section = n.ID, ch.Label(), ""
	if n.ID != ch.ID {
		pos.Section = n.Label()
	}
	return pos, nil
}
