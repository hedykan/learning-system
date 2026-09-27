package assessment_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/curriculum"
)

func TestValidateRequiresEvidenceFromUserTurns(t *testing.T) {
	conversation := `# Raw Conversation

## 2026-09-24T10:00:00Z — assistant

> 缓存就是临时副本。

## 2026-09-24T10:01:00Z — user

> 我用过数据库，但不知道缓存是什么。
`
	report := validReport("我用过数据库，但不知道缓存是什么。")
	if err := assessment.Validate(&report, conversation, "ddia", "standard"); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}

	report.PrerequisiteGaps[0].Evidence = "缓存就是临时副本。"
	err := assessment.Validate(&report, conversation, "ddia", "standard")
	if err == nil || !strings.Contains(err.Error(), "raw user turn") {
		t.Fatalf("assistant-only evidence error = %v", err)
	}
}

func TestSaveAndCurrentStatus(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 24, 10, 2, 0, 0, time.UTC)
	report := validReport("我用过数据库，但不知道缓存是什么。")
	path, err := assessment.Save(root, "session-1", report, now)
	if err != nil {
		t.Fatal(err)
	}
	if path != "Profile/Assessments/ddia/session-1.md" {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Prerequisite Gaps") || !strings.Contains(string(data), "不知道缓存") {
		t.Fatalf("unexpected assessment markdown:\n%s", data)
	}
	status, err := assessment.CurrentStatus(root, "ddia")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "assessed" || status.SessionID != "session-1" || status.Path != path {
		t.Fatalf("status = %+v", status)
	}
}

func validReport(evidence string) assessment.Report {
	return assessment.Report{
		Curriculum: "ddia",
		Depth:      "standard",
		PrerequisiteGaps: []assessment.Finding{{
			Claim:    "尚未建立缓存概念",
			Evidence: evidence,
		}},
		UnknownVocabulary: []string{"缓存"},
		RecommendedEntry: curriculum.Position{
			Book:           "ddia",
			Chapter:        "1",
			Section:        "引言",
			CurrentConcept: "数据系统的基本职责",
		},
		RecommendationReason: "先从熟悉的数据存储场景建立基础词汇。",
		NextProbe:            "订单数据为什么需要长期保存？",
	}
}
