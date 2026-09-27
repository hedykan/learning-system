package model_test

import (
	"context"
	"testing"

	"github.com/hedykan/learning-system/internal/model"
)

func TestConservativeProviderOnlyExtractsExplicitQuestions(t *testing.T) {
	provider := model.ConservativeProvider{}
	analysis, err := provider.AnalyzeSession(context.Background(), model.SessionAnalysisInput{
		Conversation: "## 2026-09-24T10:00:00Z — user\n\n> 什么是极限？\n> 我已经懂了。\n\n## 2026-09-24T10:01:00Z — assistant\n\n> 你能解释吗？\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Events) != 1 || analysis.Events[0].Type != "question" {
		t.Fatalf("events = %+v", analysis.Events)
	}
	if len(analysis.CognitiveChanges) != 0 {
		t.Fatalf("provider invented cognitive changes: %+v", analysis.CognitiveChanges)
	}
}
