package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ReviewQuestion is a generated retrieval question that has not been
// answered yet. Questions are kept separately from the model transcript so
// the app can restore them after a refresh without asking the model again.
type ReviewQuestion struct {
	Concept  string `json:"concept"`
	Label    string `json:"label"`
	Ask      string `json:"ask"`
	Recorded bool   `json:"recorded"`
}

type reviewQuestions struct {
	Course    string                    `json:"course"`
	Session   string                    `json:"session"`
	Questions map[string]ReviewQuestion `json:"questions"`
}

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

func reviewQuestionsPath(vault, cur string) string {
	if cur == "" {
		cur, _ = learnJSON(vault, "status")["current_learning"].(string)
		if cur == "" {
			cur = "none"
		}
	}
	return filepath.Join(vault, ".learning", "tmp", "app-review-questions-"+cur+".json")
}

func loadReviewQuestions(vault string) reviewQuestions {
	st := learnJSON(vault, "status")
	cur, _ := st["current_learning"].(string)
	active, _ := st["active_session"].(map[string]any)
	if active == nil || active["kind"] != "review" {
		return reviewQuestions{}
	}
	var out reviewQuestions
	b, err := os.ReadFile(reviewQuestionsPath(vault, cur))
	if err != nil || json.Unmarshal(b, &out) != nil || out.Course != cur || out.Session != mstr(active, "id") {
		return reviewQuestions{}
	}
	if out.Questions == nil {
		out.Questions = map[string]ReviewQuestion{}
	}
	return out
}

func saveReviewQuestion(vault string, q ReviewQuestion) {
	st := learnJSON(vault, "status")
	cur, _ := st["current_learning"].(string)
	active, _ := st["active_session"].(map[string]any)
	if active == nil || active["kind"] != "review" || q.Concept == "" || q.Ask == "" {
		return
	}
	set := loadReviewQuestions(vault)
	if set.Questions == nil {
		set = reviewQuestions{Course: cur, Session: mstr(active, "id"), Questions: map[string]ReviewQuestion{}}
	}
	set.Questions[q.Concept] = q
	b, _ := json.Marshal(set)
	_ = os.WriteFile(reviewQuestionsPath(vault, cur), b, 0o644)
}

func forgetReviewQuestion(vault, concept string) {
	set := loadReviewQuestions(vault)
	if len(set.Questions) == 0 {
		return
	}
	delete(set.Questions, concept)
	b, _ := json.Marshal(set)
	_ = os.WriteFile(reviewQuestionsPath(vault, set.Course), b, 0o644)
}

func forgetReviewQuestions(vault, cur string) {
	_ = os.Remove(reviewQuestionsPath(vault, cur))
}

// migrateLegacyState renames the pre-per-course state file, if one is around.
func migrateLegacyState(vault string) {
	old := filepath.Join(vault, ".learning", "tmp", "app-state.json")
	if _, err := os.Stat(old); err == nil {
		_ = os.Rename(old, statePath(vault, "", "learn"))
	}
}
