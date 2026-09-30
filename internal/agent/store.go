package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// One tutor conversation per course and screen: a session belongs to one
// curriculum, and the review screen talks in its own review session.
func statePath(vault, cur, screen string) string {
	if cur == "" {
		st := learnJSON(vault, "status")
		cur, _ = st["current_learning"].(string)
		if cur == "" {
			cur = "none"
		}
	}
	name := "app-state-" + cur
	if screen == "review" {
		name += "-review"
	}
	return filepath.Join(vault, ".learning", "tmp", name+".json")
}

func loadHistory(vault, cur, screen string) []Message {
	if b, err := os.ReadFile(statePath(vault, cur, screen)); err == nil {
		var h []Message
		if json.Unmarshal(b, &h) == nil && len(h) > 0 {
			return h
		}
	}
	return []Message{{"role": "system", "content": SystemPrompt(vault)}}
}

func saveHistory(vault, cur, screen string, h []Message) {
	b, _ := json.Marshal(h)
	_ = os.WriteFile(statePath(vault, cur, screen), b, 0o644)
}

func forgetHistory(vault, cur, screen string) {
	_ = os.Remove(statePath(vault, cur, screen))
}

// migrateLegacyState renames the pre-per-course state file, if one is around.
func migrateLegacyState(vault string) {
	old := filepath.Join(vault, ".learning", "tmp", "app-state.json")
	if _, err := os.Stat(old); err == nil {
		_ = os.Rename(old, statePath(vault, "", "learn"))
	}
}
