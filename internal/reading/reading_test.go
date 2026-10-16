package reading

import (
	"testing"

	"github.com/hedykan/learning-system/internal/locator"
)

func TestSummaryMergesPageRangesWithoutCountingJumps(t *testing.T) {
	st := State{Course: "ddia", Source: "book", Position: &locator.Locator{Resource: "book", Kind: "page", Value: "100"}, Sessions: []Session{
		{ID: "r1", Mode: "self", Events: []Event{{Type: "view", Locator: locator.Locator{Resource: "book", Kind: "page", Value: "1-3"}, ActiveSeconds: 30}, {Type: "view", Locator: locator.Locator{Resource: "book", Kind: "page", Value: "3-4"}, ActiveSeconds: 20}}},
		{ID: "r2", Mode: "guided", Events: []Event{{Type: "view", Locator: locator.Locator{Resource: "book", Kind: "page", Value: "100"}, ActiveSeconds: 10}}},
	}}
	got := Summarize(st)
	if got.UniqueUnits != 5 || got.ActiveSeconds != 60 || len(got.Covered) != 2 || got.Covered[0].Value != "1-4" || got.Covered[1].Value != "100" {
		t.Fatalf("summary = %#v", got)
	}
	if len(got.ByMode) != 2 || got.ByMode[0].Mode != "guided" || got.ByMode[0].Units != 1 || got.ByMode[1].Mode != "self" || got.ByMode[1].Units != 4 {
		t.Fatalf("by mode = %#v", got.ByMode)
	}
}
