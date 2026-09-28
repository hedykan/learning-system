// Package record defines the Interpretation Record schema that Agents submit
// and the append-only envelope the Runtime stores. Semantic validation lives
// in package learner, because it depends on the model replayed so far.
package record

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/fsutil"
)

const Schema = "learning-os/interpretation@1"

type Evidence struct {
	Turn  string `json:"turn"`
	Quote string `json:"quote"`
}

type SourceRef struct {
	Node    string `json:"node,omitempty"`
	Chapter string `json:"chapter,omitempty"`
	Section string `json:"section,omitempty"`
}

type Concept struct {
	ID             string          `json:"id"`
	Label          string          `json:"label,omitempty"`
	Aliases        []string        `json:"aliases,omitempty"`
	SourceRef      *SourceRef      `json:"source_ref,omitempty"`
	TextbookPoints *TextbookPoints `json:"textbook_points,omitempty"`
	Related        []Relation      `json:"related,omitempty"`
}

// Relation links two concepts; the Agent decides that they are related.
type Relation struct {
	Concept string `json:"concept"`
	Note    string `json:"note,omitempty"`
}

// TextbookPoints summarizes, in a few lines, what the book says about a
// concept on the pages the Agent actually read.
type TextbookPoints struct {
	Pages  []int    `json:"pages"`
	Points []string `json:"points"`
}

type Event struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Concept  string     `json:"concept,omitempty"`
	Summary  string     `json:"summary"`
	Evidence []Evidence `json:"evidence"`
}

type CognitiveChange struct {
	ID          string     `json:"id"`
	Concept     string     `json:"concept"`
	OldModel    string     `json:"old_model"`
	Trigger     string     `json:"trigger"`
	TriggerTurn string     `json:"trigger_turn"`
	NewModel    string     `json:"new_model"`
	OldEvidence []Evidence `json:"old_evidence"`
	NewEvidence []Evidence `json:"new_evidence"`
}

type StateUpdate struct {
	ID            string     `json:"id"`
	Concept       string     `json:"concept"`
	State         string     `json:"state"`
	Capabilities  []string   `json:"capabilities"`
	Summary       string     `json:"summary"`
	Evidence      []Evidence `json:"evidence"`
	OpenQuestions []string   `json:"open_questions,omitempty"`
}

type StrategyAttempt struct {
	ID             string     `json:"id"`
	Strategy       string     `json:"strategy"`
	Situation      string     `json:"situation"`
	Concept        string     `json:"concept"`
	ActionTurn     string     `json:"action_turn"`
	ExpectedChange string     `json:"expected_change"`
	Outcome        string     `json:"outcome"`
	Reason         string     `json:"reason,omitempty"`
	Linked         []string   `json:"linked,omitempty"`
	Evidence       []Evidence `json:"evidence"`
	Replaces       string     `json:"replaces,omitempty"`
	SwitchReason   string     `json:"switch_reason,omitempty"`
}

type PatternObservation struct {
	ID                string     `json:"id"`
	Pattern           string     `json:"pattern"`
	Description       string     `json:"description,omitempty"`
	PreferredStrategy string     `json:"preferred_strategy,omitempty"`
	Situation         string     `json:"situation,omitempty"`
	Stance            string     `json:"stance"`
	Summary           string     `json:"summary"`
	Linked            []string   `json:"linked"`
	Evidence          []Evidence `json:"evidence"`
}

// ReviewResult is the outcome of one spaced-review question.
type ReviewResult struct {
	ID         string     `json:"id"`
	Concept    string     `json:"concept"`
	Outcome    string     `json:"outcome"` // recalled, partial, forgotten
	ActionTurn string     `json:"action_turn"`
	Evidence   []Evidence `json:"evidence"`
}

// Question is a learner-generated key question, a first-class learning
// object (CR-2026-020). IDs are global kebab-case slugs, like concepts.
type Question struct {
	ID       string     `json:"id"`
	Question string     `json:"question"`
	Concept  string     `json:"concept,omitempty"`
	Node     string     `json:"node,omitempty"`
	Evidence []Evidence `json:"evidence"`
}

// QuestionResolution closes an open question with learner evidence.
type QuestionResolution struct {
	Question string     `json:"question"`
	Summary  string     `json:"summary"`
	Evidence []Evidence `json:"evidence"`
}

type Retraction struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

type ProgressDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type Record struct {
	Schema              string               `json:"schema"`
	Curriculum          string               `json:"curriculum"`
	Concepts            []Concept            `json:"concepts,omitempty"`
	Events              []Event              `json:"events,omitempty"`
	CognitiveChanges    []CognitiveChange    `json:"cognitive_changes,omitempty"`
	StateUpdates        []StateUpdate        `json:"state_updates,omitempty"`
	StrategyAttempts    []StrategyAttempt    `json:"strategy_attempts,omitempty"`
	PatternObservations []PatternObservation `json:"pattern_observations,omitempty"`
	ReviewResults       []ReviewResult       `json:"review_results,omitempty"`
	NoRelated           []string             `json:"no_related,omitempty"`
	Questions           []Question           `json:"questions,omitempty"`
	QuestionResolutions []QuestionResolution `json:"question_resolutions,omitempty"`
	Retractions         []Retraction         `json:"retractions,omitempty"`
	ProgressDecision    *ProgressDecision    `json:"progress_decision,omitempty"`
}

// Envelope is the stored, append-only form of an accepted record.
type Envelope struct {
	Session     string `json:"session"`
	Seq         int    `json:"seq"`
	Kind        string `json:"kind"`
	SubmittedAt string `json:"submitted_at"`
	Hash        string `json:"hash"`
	Record      Record `json:"record"`
}

// Parse decodes a submitted record, rejecting unknown fields.
func Parse(data []byte) (*Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var rec Record
	if err := decoder.Decode(&rec); err != nil {
		return nil, fmt.Errorf("parse interpretation record: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("parse interpretation record: trailing data")
	}
	if rec.Schema != Schema {
		return nil, fmt.Errorf("interpretation record schema must be %q", Schema)
	}
	return &rec, nil
}

// Empty reports whether a record carries no content. Concepts alone count
// only when they carry textbook points.
func (r *Record) Empty() bool {
	for _, c := range r.Concepts {
		if c.TextbookPoints != nil || len(c.Related) > 0 {
			return false
		}
	}
	return len(r.Events)+len(r.CognitiveChanges)+len(r.StateUpdates)+len(r.StrategyAttempts)+
		len(r.PatternObservations)+len(r.ReviewResults)+len(r.NoRelated)+len(r.Questions)+len(r.QuestionResolutions)+len(r.Retractions) == 0 && r.ProgressDecision == nil
}

// Hash returns the canonical content hash of a record.
func Hash(r *Record) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Dir is where a session's envelopes are stored.
func Dir(root, sessionID string) string {
	return filepath.Join(root, ".learning", "interpretations", sessionID)
}

// LoadAll reads every envelope in the Vault in replay order: submission time,
// then session, then sequence.
func LoadAll(root string) ([]Envelope, error) {
	base := filepath.Join(root, ".learning", "interpretations")
	sessions, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read interpretations: %w", err)
	}
	var all []Envelope
	for _, dir := range sessions {
		if !dir.IsDir() {
			continue
		}
		envs, err := LoadSession(root, dir.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, envs...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.SubmittedAt != b.SubmittedAt {
			return a.SubmittedAt < b.SubmittedAt
		}
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		return a.Seq < b.Seq
	})
	return all, nil
}

// LoadSession reads one session's envelopes in sequence order.
func LoadSession(root, sessionID string) ([]Envelope, error) {
	entries, err := os.ReadDir(Dir(root, sessionID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read interpretations for %s: %w", sessionID, err)
	}
	var envs []Envelope
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(Dir(root, sessionID), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read interpretation %s: %w", entry.Name(), err)
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("parse interpretation %s/%s: %w", sessionID, entry.Name(), err)
		}
		envs = append(envs, env)
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].Seq < envs[j].Seq })
	return envs, nil
}

// Write stores an accepted envelope. It never overwrites an existing file.
func Write(root string, env Envelope) (string, error) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode interpretation: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(Dir(root, env.Session), fmt.Sprintf("%04d.json", env.Seq))
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("interpretation %s already exists", path)
	}
	if err := fsutil.WriteFileAtomic(path, data, 0o644); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel), nil
}

// Generation summarizes every stored record hash; projections carry it so
// staleness can be detected without replaying.
func Generation(envs []Envelope) string {
	if len(envs) == 0 {
		return "empty"
	}
	h := sha256.New()
	for _, env := range envs {
		fmt.Fprintf(h, "%s/%d/%s\n", env.Session, env.Seq, env.Hash)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
