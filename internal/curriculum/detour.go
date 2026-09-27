package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

// DetourStart describes a prerequisite detour away from the mainline.
type DetourStart struct {
	Topic           string
	Concept         string
	Reason          string
	ReturnCondition string
	Session         string
}

// DetourLogEntry is one completed detour in detours.yaml.
type DetourLogEntry struct {
	Detour  `yaml:",inline"`
	EndedAt string `yaml:"ended_at" json:"ended_at"`
	Outcome string `yaml:"outcome" json:"outcome"`
	Learned string `yaml:"learned" json:"learned"`
}

func detourLogPath(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "detours.yaml")
}

// LoadDetourLog reads completed detours.
func LoadDetourLog(root, id string) ([]DetourLogEntry, error) {
	data, err := os.ReadFile(detourLogPath(root, id))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read detour log: %w", err)
	}
	var entries []DetourLogEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse detour log: %w", err)
	}
	return entries, nil
}

// StartDetour records a detour using the current position as return point.
func StartDetour(root, id string, in DetourStart, now time.Time) (Detour, error) {
	for name, v := range map[string]string{"topic": in.Topic, "reason": in.Reason, "return condition": in.ReturnCondition} {
		if strings.TrimSpace(v) == "" {
			return Detour{}, fmt.Errorf("detour requires a %s", name)
		}
	}
	position, err := LoadPosition(root, id)
	if err != nil {
		return Detour{}, err
	}
	if position.Detour != nil {
		return Detour{}, fmt.Errorf("detour %q is still open; end it first", position.Detour.Topic)
	}
	log, err := LoadDetourLog(root, id)
	if err != nil {
		return Detour{}, err
	}
	d := Detour{
		ID: fmt.Sprintf("d%d", len(log)+1), Type: "prerequisite", Topic: in.Topic, Concept: in.Concept,
		Reason: in.Reason, ReturnCondition: in.ReturnCondition,
		ReturnTo:       ReturnPoint{Chapter: position.Chapter, Section: position.Section, Concept: position.CurrentConcept},
		StartedSession: in.Session, StartedAt: now.UTC().Format(time.RFC3339Nano),
	}
	position.Detour = &d
	return d, SavePosition(root, id, position)
}

// EndDetour closes the open detour, appends it to the log and restores the
// return point. The log is written first so a failure never loses history.
func EndDetour(root, id, outcome, learned string, now time.Time) (DetourLogEntry, error) {
	if outcome != "completed" && outcome != "abandoned" {
		return DetourLogEntry{}, fmt.Errorf("detour outcome must be completed or abandoned")
	}
	if strings.TrimSpace(learned) == "" {
		return DetourLogEntry{}, fmt.Errorf("detour end requires --learned")
	}
	position, err := LoadPosition(root, id)
	if err != nil {
		return DetourLogEntry{}, err
	}
	if position.Detour == nil {
		return DetourLogEntry{}, fmt.Errorf("no open detour")
	}
	log, err := LoadDetourLog(root, id)
	if err != nil {
		return DetourLogEntry{}, err
	}
	entry := DetourLogEntry{Detour: *position.Detour, EndedAt: now.UTC().Format(time.RFC3339Nano), Outcome: outcome, Learned: learned}
	data, err := yaml.Marshal(append(log, entry))
	if err != nil {
		return DetourLogEntry{}, fmt.Errorf("encode detour log: %w", err)
	}
	if err := fsutil.WriteFileAtomic(detourLogPath(root, id), data, 0o644); err != nil {
		return DetourLogEntry{}, err
	}
	back := entry.ReturnTo
	position.Chapter, position.Section, position.CurrentConcept = back.Chapter, back.Section, back.Concept
	position.Detour = nil
	return entry, SavePosition(root, id, position)
}
