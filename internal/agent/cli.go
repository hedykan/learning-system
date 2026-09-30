// Package agent is the tutor driver layer shared by every frontend: prompt
// assembly, tool definitions and dispatch, model calls (streaming), and the
// conversation loop. It drives the learning core in-process through the CLI
// command tree, so the behavior is exactly the CLI's (including Git commits),
// with none of the subprocess overhead or output truncation surprises.
package agent

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hedykan/learning-system/internal/app"
)

// toolResult mirrors the Python devserver's {"ok": bool, "output": str}.
type toolResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

// runCLI executes a learn command in-process. stdout and stderr merge into one
// string, and a command error is appended (the Python server matched error text
// like "needs --analysis-file", so keep it in the output).
func runCLI(vault, stdin string, args ...string) toolResult {
	var out bytes.Buffer
	a := app.New()
	a.Out, a.Err, a.In = &out, &out, strings.NewReader(stdin)
	cmd := a.RootCommand()
	cmd.SetArgs(append([]string{"--vault", vault}, args...))
	err := cmd.Execute()
	s := strings.TrimSpace(out.String())
	if err != nil {
		if s != "" {
			s += "\n"
		}
		s += err.Error()
	}
	return toolResult{OK: err == nil, Output: s}
}

// learn runs a command and caps the output for the model's context. keep <= 0
// means no cap; the default cap guards the model from huge payloads.
func learn(vault string, keep int, args ...string) toolResult {
	r := runCLI(vault, "", args...)
	if keep > 0 && len(r.Output) > keep {
		r.Output = r.Output[len(r.Output)-keep:]
	}
	return r
}

// learnJSON runs a --json command and parses the full output. It feeds the
// app's screens, where truncation would corrupt the payload (a long outline
// review once broke the draft card), so it never caps.
func learnJSON(vault string, args ...string) map[string]any {
	r := runCLI(vault, "", append(args, "--json")...)
	if !r.OK {
		return nil
	}
	var d map[string]any
	if json.Unmarshal([]byte(r.Output), &d) != nil {
		return nil
	}
	return d
}

// learnJSONList is learnJSON for commands that answer with a JSON array.
func learnJSONList(vault string, args ...string) []any {
	r := runCLI(vault, "", append(args, "--json")...)
	if !r.OK {
		return nil
	}
	var d []any
	if json.Unmarshal([]byte(r.Output), &d) != nil {
		return nil
	}
	return d
}

// obj renders a tool argument for stdin: strings pass through, anything else
// becomes JSON.
func obj(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func activeSession(vault string) map[string]any {
	st := learnJSON(vault, "status")
	if st == nil {
		return nil
	}
	s, _ := st["active_session"].(map[string]any)
	return s
}

func reviewing(vault string) bool {
	return activeSession(vault)["kind"] == "review"
}

// appendTurn records a conversation turn and returns its id ("" on failure).
func appendTurn(vault, role, text string) string {
	r := runCLI(vault, text, "session", "append", "--role", role, "--json")
	var d map[string]any
	if json.Unmarshal([]byte(r.Output), &d) != nil {
		return ""
	}
	s, _ := d["turn"].(string)
	return s
}
