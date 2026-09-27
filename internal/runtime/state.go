package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
)

const SchemaVersion = 1

type ActiveSession struct {
	ID              string `json:"id"`
	StartedAt       string `json:"started_at"`
	Conversation    string `json:"conversation"`
	Kind            string `json:"kind,omitempty"`
	Depth           string `json:"depth,omitempty"`
	BaselineSkipped bool   `json:"baseline_skipped,omitempty"`
	Domain          string `json:"domain,omitempty"`
	Curriculum      string `json:"curriculum,omitempty"`
	StartingChapter string `json:"starting_chapter,omitempty"`
	StartingSection string `json:"starting_section,omitempty"`
	StartingConcept string `json:"starting_concept,omitempty"`
}

type State struct {
	SchemaVersion int            `json:"schema_version"`
	ActiveSession *ActiveSession `json:"active_session"`
	LastSession   string         `json:"last_session,omitempty"`
	UpdatedAt     string         `json:"updated_at"`
}

func NewState(now time.Time) State {
	return State{SchemaVersion: SchemaVersion, UpdatedAt: now.UTC().Format(time.RFC3339)}
}

func Load(root string) (State, error) {
	path := filepath.Join(root, ".learning", "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read runtime state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse runtime state: %w", err)
	}
	if state.SchemaVersion != SchemaVersion {
		return State{}, fmt.Errorf("unsupported state schema version %d", state.SchemaVersion)
	}
	return state, nil
}

func Save(root string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state: %w", err)
	}
	data = append(data, '\n')
	return fsutil.WriteFileAtomic(filepath.Join(root, ".learning", "state.json"), data, 0o644)
}
