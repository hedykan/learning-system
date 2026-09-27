package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/hedykan/learning-system/internal/conversation"
)

var AllowedEventTypes = map[string]bool{
	"question": true, "prediction": true, "misconception": true,
	"correction": true, "insight": true, "understanding_revision": true,
	"retrieval": true, "connection": true, "learner_proposed_method": true,
	"prerequisite_gap": true, "transfer": true, "policy_failure": true,
	"strategy_switch": true,
}

type SessionAnalysisInput struct {
	SessionID    string
	Conversation string
	Curriculum   string
	Chapter      string
	Section      string
	Concept      string
}

type LearningEvent struct {
	Type     string `json:"type"`
	Summary  string `json:"summary"`
	Evidence string `json:"evidence"`
}

type CognitiveChange struct {
	OldUnderstanding string `json:"old_understanding"`
	Trigger          string `json:"trigger"`
	NewUnderstanding string `json:"new_understanding"`
}

type SessionAnalysis struct {
	Events           []LearningEvent   `json:"events"`
	CognitiveChanges []CognitiveChange `json:"cognitive_changes"`
	OpenLoops        []string          `json:"open_loops"`
	NextProbe        string            `json:"next_probe"`
}

type Provider interface {
	AnalyzeSession(context.Context, SessionAnalysisInput) (*SessionAnalysis, error)
}

func Validate(analysis *SessionAnalysis) error {
	if analysis == nil {
		return fmt.Errorf("analysis is nil")
	}
	for i, event := range analysis.Events {
		if !AllowedEventTypes[event.Type] {
			return fmt.Errorf("event %d has unsupported type %q", i, event.Type)
		}
		if strings.TrimSpace(event.Summary) == "" {
			return fmt.Errorf("event %d has no summary", i)
		}
	}
	return nil
}

// ConservativeProvider deliberately makes no mastery claims. It gives v0.1 a
// deterministic end-to-end pipeline while real cognitive inference remains a
// replaceable Provider concern.
type ConservativeProvider struct{}

func (ConservativeProvider) AnalyzeSession(_ context.Context, input SessionAnalysisInput) (*SessionAnalysis, error) {
	analysis := &SessionAnalysis{
		NextProbe: "Ask the learner to retrieve and apply the current concept in a variation.",
	}
	conv, err := conversation.Parse(input.Conversation)
	if err != nil {
		return nil, err
	}
	for _, turn := range conv.Turns {
		if turn.Role != "user" {
			continue
		}
		for _, line := range strings.Split(turn.Text, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "?") || strings.Contains(line, "？") {
				analysis.Events = append(analysis.Events, LearningEvent{
					Type: "question", Summary: "Learner asked a question.", Evidence: line,
				})
				analysis.OpenLoops = append(analysis.OpenLoops, line)
			}
		}
	}
	return analysis, nil
}

type FakeProvider struct {
	Analysis *SessionAnalysis
	Err      error
}

func (f FakeProvider) AnalyzeSession(context.Context, SessionAnalysisInput) (*SessionAnalysis, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Analysis, nil
}
