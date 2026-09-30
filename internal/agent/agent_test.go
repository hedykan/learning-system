package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairFillsMissingToolResults(t *testing.T) {
	history := []Message{
		{"role": "user", "content": "hi"},
		{"role": "assistant", "content": "", "tool_calls": []any{
			map[string]any{"id": "c1", "type": "function", "function": map[string]any{"name": "learn_status", "arguments": "{}"}},
			map[string]any{"id": "c2", "type": "function", "function": map[string]any{"name": "learn_next", "arguments": "{}"}},
		}},
		{"role": "tool", "tool_call_id": "c1", "content": `{"ok":true}`},
	}
	history = repair(history)
	if len(history) != 4 {
		t.Fatalf("expected one filled result, got %d messages", len(history))
	}
	filled := history[3]
	if filled["role"] != "tool" || filled["tool_call_id"] != "c2" {
		t.Fatalf("filled message = %#v", filled)
	}
	if !strings.Contains(filled["content"].(string), "not run") {
		t.Fatalf("filled content = %#v", filled["content"])
	}
}

func TestSchemaListsEveryToolOnce(t *testing.T) {
	tools := &Tools{Vault: t.TempDir()}
	schema := tools.Schema()
	if len(schema) != len(tools.defs()) {
		t.Fatalf("schema len %d != defs len %d", len(schema), len(tools.defs()))
	}
	seen := map[string]bool{}
	for i, d := range tools.defs() {
		if seen[d.Name] {
			t.Fatalf("duplicate tool %q", d.Name)
		}
		seen[d.Name] = true
		got := schema[i]["function"].(map[string]any)["name"]
		if got != d.Name {
			t.Fatalf("schema[%d] = %v, want %q", i, got, d.Name)
		}
	}
	for _, want := range []string{"learn_status", "source_add", "record_checkpoint", "outline_review", "goal_set"} {
		if !seen[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
}

func TestToolDispatchAgainstFreshVault(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if r := runCLI(root, "", "init", root); !r.OK {
		t.Fatalf("init: %s", r.Output)
	}
	tools := &Tools{Vault: root}
	r := tools.Run("learn_status", map[string]any{})
	if !r.OK {
		t.Fatalf("learn_status: %s", r.Output)
	}
	var st map[string]any
	if err := json.Unmarshal([]byte(r.Output), &st); err != nil {
		t.Fatalf("learn_status not JSON: %v\n%s", err, r.Output)
	}
	if r := tools.Run("no_such_tool", map[string]any{}); r.OK {
		t.Fatal("unknown tool should fail softly")
	}
}

func TestSessionStartRecordsPendingUser(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if r := runCLI(root, "", "init", root); !r.OK {
		t.Fatalf("init: %s", r.Output)
	}
	tools := &Tools{Vault: root, PendingUser: "我想学 OpenGL"}
	r := tools.Run("session_start", map[string]any{"kind": "lesson", "skip_baseline": true})
	if !r.OK {
		t.Fatalf("session_start: %s", r.Output)
	}
	if tools.PendingUser != "" {
		t.Fatal("pending user should be consumed")
	}
	if !strings.Contains(r.Output, "recorded as turn") {
		t.Fatalf("output should mention the recorded turn: %s", r.Output)
	}
}

func TestSystemPromptIncludesVaultDocs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("VAULT RULES"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := SystemPrompt(root)
	if !strings.Contains(p, "大肥鱼老师") || !strings.Contains(p, "VAULT RULES") {
		t.Fatalf("prompt missing app text or vault rules: %d bytes", len(p))
	}
}
