package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// Tools dispatches the model's tool calls against one vault. PendingUser holds
// a learner message that arrived before any session started; it is recorded as
// soon as a session opens.
type Tools struct {
	Vault       string
	PendingUser string
}

type toolDef struct {
	Name  string
	Desc  string
	Props [][2]string // ordered (name, JSON type); empty means no parameters
	Run   func(t *Tools, a map[string]any) toolResult
}

func str(a map[string]any, key string) string {
	s, _ := a[key].(string)
	return s
}

func num(a map[string]any, key string, fallback int) int {
	switch v := a[key].(type) {
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// toolSessionStart starts a lesson or a baseline; reviews belong to the review
// screen, which starts them itself.
func toolSessionStart(t *Tools, a map[string]any) toolResult {
	kind := str(a, "kind")
	if kind == "review" {
		return toolResult{false, "reviews run only on the app's review screen; start a lesson (kind lesson) or a baseline here, and teach new material"}
	}
	if kind == "" {
		kind = "lesson"
	}
	args := []string{"session", "start", "--kind", kind}
	if b, _ := a["skip_baseline"].(bool); b {
		args = append(args, "--skip-baseline")
	}
	if d := str(a, "depth"); d != "" {
		args = append(args, "--depth", d)
	}
	r := runCLI(t.Vault, "", append(args, "--json")...)
	if r.OK && t.PendingUser != "" {
		turn := appendTurn(t.Vault, "user", t.PendingUser)
		t.PendingUser = ""
		r.Output += fmt.Sprintf("\n(The learner's message was recorded as turn %s.)", turn)
	}
	return r
}

func toolSessionEnd(t *Tools, a map[string]any) toolResult {
	var extra []string
	var stdin string
	if rec, ok := a["record"]; ok && rec != nil {
		extra, stdin = []string{"--analysis-file", "-"}, obj(rec)
	} else if asm, ok := a["assessment"]; ok && asm != nil {
		extra, stdin = []string{"--assessment-file", "-"}, obj(asm)
	} else {
		reason := str(a, "reason")
		if reason == "" {
			reason = "nothing to record"
		}
		extra = []string{"--no-analysis", "--reason", reason}
	}
	args := append([]string{"session", "end"}, extra...)
	return runCLI(t.Vault, stdin, append(args, "--json")...)
}

// toolSourceAdd imports a textbook: a file path is copied in; a URL is
// snapshotted page by page (a whole site can take minutes).
func toolSourceAdd(t *Tools, a map[string]any) toolResult {
	src := str(a, "path_or_url")
	if src == "" {
		return toolResult{false, "path_or_url is required"}
	}
	title := str(a, "title")
	if title == "" {
		title = str(a, "id")
	}
	args := []string{"source", "add", src, "--id", str(a, "id"), "--title", title, "--json"}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		args = append(args, "--sitemap")
		if p := str(a, "prefix"); p != "" {
			args = append(args, "--prefix", p)
		}
		args = append(args, "--max-pages", strconv.Itoa(min(num(a, "max_pages", 200), 500)))
	}
	return runCLI(t.Vault, "", args...)
}

// defs is the tool registry in declaration order (stable schemas are friendly
// to the provider's prompt cache).
func (t *Tools) defs() []toolDef {
	return []toolDef{
		{"learn_status", "Vault status: active curriculum, position, outline, active session, baseline.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "status", "--json") }},
		{"learn_next", "The recommended next learning action, with the entry's resources.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "next", "--json") }},
		{"learn_review", "Concepts due for spaced review today and in the next 7 days.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "review", "--json") }},
		{"concept_show", "One concept with its cognitive history.", [][2]string{{"concept", "string"}},
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "state", "concept", str(a, "concept"), "--json") }},
		{"outline_show", "Show the outline with statuses.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "curriculum", "outline", "show", "--json") }},
		{"position_set", "Move the position to an outline entry.", [][2]string{{"node", "string"}},
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "curriculum", "position", "set", "--node", str(a, "node")) }},
		{"curriculum_complete", "Mark an outline entry completed.", [][2]string{{"node", "string"}, {"reason", "string"}},
			func(t *Tools, a map[string]any) toolResult {
				return learn(t.Vault, 6000, "curriculum", "complete", str(a, "node"), "--reason", str(a, "reason"), "--json")
			}},
		{"session_start", "Start a session. kind is lesson or baseline; reviews run on the review screen.",
			[][2]string{{"kind", "string"}, {"skip_baseline", "boolean"}, {"depth", "string"}}, toolSessionStart},
		{"session_turns", "List the recorded turns of the active session with their ids.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "session", "turns", "--json") }},
		{"source_outline", "Headings of a source.", [][2]string{{"source", "string"}},
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "source", "outline", str(a, "source"), "--json") }},
		{"source_list", "List the materials stored in the vault.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "source", "list", "--json") }},
		{"source_add", "Import a textbook into the vault: a file path (PDF, EPUB, Markdown), or a website URL snapshotted page by page (prefix limits the paths, max_pages how many). A whole site takes minutes: warn the learner.",
			[][2]string{{"path_or_url", "string"}, {"id", "string"}, {"title", "string"}, {"prefix", "string"}, {"max_pages", "integer"}}, toolSourceAdd},
		{"source_attach", "Attach a part of a source to an outline entry (kind anchor/file/chapter/page/time/text).",
			[][2]string{{"node", "string"}, {"source", "string"}, {"kind", "string"}, {"value", "string"}, {"why", "string"}},
			func(t *Tools, a map[string]any) toolResult {
				why := str(a, "why")
				if len([]rune(why)) > 120 {
					why = string([]rune(why)[:120])
				}
				return learn(t.Vault, 6000, "source", "attach", str(a, "node"), str(a, "source"), str(a, "kind"), str(a, "value"), "--why", why, "--json")
			}},
		{"source_read", "Read a source at a locator (kind anchor/file/chapter/page/time/text).",
			[][2]string{{"source", "string"}, {"kind", "string"}, {"value", "string"}},
			func(t *Tools, a map[string]any) toolResult {
				return learn(t.Vault, 6000, "source", "read", str(a, "source"), str(a, "kind"), str(a, "value"), "--json")
			}},
		{"record_checkpoint", "Submit an interpretation record for the active session.", [][2]string{{"record", "object"}},
			func(t *Tools, a map[string]any) toolResult {
				return runCLI(t.Vault, obj(a["record"]), "session", "checkpoint", "--analysis-file", "-", "--json")
			}},
		{"session_end", "End the session with a final record, a baseline assessment, or a reason.",
			[][2]string{{"record", "object"}, {"assessment", "object"}, {"reason", "string"}}, toolSessionEnd},
		{"curriculum_create_goal", "Create and activate a new course from a learning goal. Only when an [app] instruction says the learner asked for a new course. id is short lowercase kebab-case.",
			[][2]string{{"goal", "string"}, {"id", "string"}, {"title", "string"}},
			func(t *Tools, a map[string]any) toolResult {
				return learn(t.Vault, 6000, "curriculum", "import", "--goal", str(a, "goal"), "--id", str(a, "id"), "--title", str(a, "title"), "--yes", "--activate", "--json")
			}},
		{"goal_set", "Set the goal card from the intake interview: {outcome: {text, evidence}, context?, background?, constraints?, success_criteria?, focus?: [{id, text, evidence}]}; evidence is the learner's exact words in this session.",
			[][2]string{{"goal_card", "object"}},
			func(t *Tools, a map[string]any) toolResult {
				return runCLI(t.Vault, obj(a["goal_card"]), "curriculum", "goal", "set", "--file", "-", "--json")
			}},
		{"goal_show", "Show the goal card of the active course.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 6000, "curriculum", "goal", "show", "--json") }},
		{"outline_set", "Submit a draft outline {\"nodes\": [...]}, each entry with why, prerequisites, concepts, serves.",
			[][2]string{{"outline", "object"}},
			func(t *Tools, a map[string]any) toolResult {
				return runCLI(t.Vault, obj(a["outline"]), "curriculum", "outline", "set", "--file", "-", "--json")
			}},
		{"outline_review", "Review the draft: entries, goal coverage, likely known, ready_to_confirm.", nil,
			func(t *Tools, a map[string]any) toolResult { return learn(t.Vault, 20000, "curriculum", "outline", "review", "--json") }},
	}
}

// Schema renders the registry as OpenAI-style function tools.
func (t *Tools) Schema() []map[string]any {
	out := []map[string]any{}
	for _, d := range t.defs() {
		props := map[string]any{}
		for _, p := range d.Props {
			props[p[0]] = map[string]any{"type": p[1]}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        d.Name,
				"description": d.Desc,
				"parameters":  map[string]any{"type": "object", "properties": props},
			},
		})
	}
	return out
}

// Run dispatches one call; an unknown tool fails softly so the model can
// recover.
func (t *Tools) Run(name string, args map[string]any) toolResult {
	for _, d := range t.defs() {
		if d.Name == name {
			return d.Run(t, args)
		}
	}
	return toolResult{false, "tool error: unknown tool " + name}
}
