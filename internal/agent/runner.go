package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrCancelled means the learner left or tapped stop: the page stopped
// listening, so the driver gives up on the model call.
var ErrCancelled = errors.New("cancelled")

// CallRecord notes one tool call for callers that inspect what happened.
type CallRecord struct {
	Name string
	Args map[string]any
	OK   bool
}

// Emit streams one progress event to the page; a returned error aborts the run.
type Emit func(ev map[string]any) error

// repair fills gaps in a saved conversation: every tool call needs its result
// before the next model call, or the API refuses the whole conversation. A stop
// between the two leaves a gap, which gets a "not run" result.
func repair(history []Message) []Message {
	for i := 0; i < len(history); {
		m := history[i]
		calls, _ := m["tool_calls"].([]any)
		if m["role"] != "assistant" || len(calls) == 0 {
			i++
			continue
		}
		answered := map[string]bool{}
		j := i + 1
		for j < len(history) && history[j]["role"] == "tool" {
			if id, ok := history[j]["tool_call_id"].(string); ok {
				answered[id] = true
			}
			j++
		}
		var missing []Message
		for _, c := range calls {
			tc, _ := c.(map[string]any)
			id, _ := tc["id"].(string)
			if !answered[id] {
				missing = append(missing, Message{
					"role": "tool", "tool_call_id": id,
					"content": `{"ok":false,"output":"not run: the learner stopped this reply"}`,
				})
			}
		}
		history = append(history[:j], append(missing, history[j:]...)...)
		i = j + len(missing)
	}
	return history
}

// RunAgent lets the model work with tools until it replies. When recordReply
// is set and a session is active, the reply is recorded as a conversation turn.
func RunAgent(ctx context.Context, tools *Tools, history []Message, emit Emit, recordReply bool) (string, []CallRecord, error) {
	logs := filepath.Join(tools.Vault, ".learning", "tmp", "app-logs")
	_ = os.MkdirAll(logs, 0o755)
	log, _ := os.Create(filepath.Join(logs, time.Now().Format("20060102-150405")+".jsonl"))
	defer log.Close()
	writeLog := func(v map[string]any) {
		if log != nil {
			b, _ := json.Marshal(v)
			fmt.Fprintln(log, string(b))
		}
	}
	// Always give a saved conversation today's documents, or the model has to
	// guess formats it was never shown.
	if len(history) > 0 && history[0]["role"] == "system" {
		history[0]["content"] = SystemPrompt(tools.Vault)
	}
	history = repair(history)
	reply, calls, err := run(ctx, tools, history, emit, recordReply, writeLog)
	if errors.Is(err, ErrCancelled) {
		repair(history) // the caller saves the history: leave it valid for the next reply
	}
	return reply, calls, err
}

func run(ctx context.Context, tools *Tools, history []Message, emit Emit, recordReply bool, writeLog func(map[string]any)) (string, []CallRecord, error) {
	var calls []CallRecord
	fails := map[string]int{}
	for range 30 {
		model := ActiveModel()
		answer, err := chatCompletion(ctx, model, history, tools.Schema(), func() error {
			return emit(map[string]any{"type": "alive"})
		})
		if err != nil {
			return "", calls, err
		}
		keep := Message{"role": "assistant", "content": answer.Content}
		if answer.Reasoning != "" {
			keep["reasoning_content"] = answer.Reasoning
		}
		if len(answer.ToolCalls) > 0 {
			var tcs []any
			for _, tc := range answer.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id": tc.ID, "type": "function",
					"function": map[string]any{"name": tc.Name, "arguments": tc.Arguments},
				})
			}
			keep["tool_calls"] = tcs
		}
		history = append(history, keep)
		if len(answer.ToolCalls) == 0 {
			if recordReply {
				if s := activeSession(tools.Vault); s != nil {
					appendTurn(tools.Vault, "assistant", answer.Content)
				}
			}
			writeLog(map[string]any{"reply": answer.Content, "usage": answer.Usage})
			return answer.Content, calls, nil
		}
		for _, tc := range answer.ToolCalls {
			if err := emit(map[string]any{"type": "tool", "name": tc.Name}); err != nil {
				return "", calls, err
			}
			var args map[string]any
			var result toolResult
			if json.Unmarshal([]byte(tc.Arguments), &args) != nil {
				args = map[string]any{}
				result = toolResult{false, "tool error: bad arguments"}
			} else {
				result = tools.Run(tc.Name, args)
			}
			calls = append(calls, CallRecord{tc.Name, args, result.OK})
			if result.OK {
				fails[tc.Name] = 0
			} else {
				fails[tc.Name]++
			}
			writeLog(map[string]any{"tool": tc.Name, "args": args, "ok": result.OK, "output": truncate(result.Output, 400)})
			b, _ := json.Marshal(result)
			history = append(history, Message{"role": "tool", "tool_call_id": tc.ID, "content": string(b)})
		}
		if maxFail(fails) >= 4 {
			// the same step keeps failing: stop instead of guessing on
			writeLog(map[string]any{"stopped": "repeated tool failures", "fails": fails})
			return "（这一步连着出错了几次，我先停下。换个说法再试一次，或者告诉我哪里不对。）", calls, nil
		}
	}
	return "（这一步想得太久了，换个说法再试一次吧。）", calls, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func maxFail(fails map[string]int) int {
	m := 0
	for _, v := range fails {
		if v > m {
			m = v
		}
	}
	return m
}
