package assessment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/fsutil"
)

type Finding struct {
	Claim    string `json:"claim"`
	Evidence string `json:"evidence"`
	// Concepts names the concept ids the finding is about, the same ids
	// outline entries declare, so the finding can shape the curriculum
	// (CR-2026-044).
	Concepts []string `json:"concepts,omitempty"`
}

type Report struct {
	Curriculum             string              `json:"curriculum"`
	Depth                  string              `json:"depth"`
	LearningGoals          []Finding           `json:"learning_goals"`
	ExistingKnowledge      []Finding           `json:"existing_knowledge"`
	PrerequisiteGaps       []Finding           `json:"prerequisite_gaps"`
	PossibleMisconceptions []Finding           `json:"possible_misconceptions"`
	FamiliarVocabulary     []string            `json:"familiar_vocabulary"`
	UnknownVocabulary      []string            `json:"unknown_vocabulary"`
	RecommendedEntry       curriculum.Position `json:"recommended_entry"`
	RecommendationReason   string              `json:"recommendation_reason"`
	NextProbe              string              `json:"next_probe"`
	// GoalCard is the structured goal from the intake interview (CR-2026-043).
	GoalCard *curriculum.GoalCard `json:"goal_card,omitempty"`
}

type Status struct {
	State       string `json:"state"`
	Path        string `json:"path,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

type savedReport struct {
	Version     int    `json:"version"`
	SessionID   string `json:"session_id"`
	CompletedAt string `json:"completed_at"`
	Report
}

func Parse(data []byte) (*Report, error) {
	var report Report
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("parse baseline assessment: %w", err)
	}
	return &report, nil
}

func Validate(report *Report, conversationText, curriculumID, depth string) error {
	if report == nil {
		return fmt.Errorf("baseline session requires a structured assessment")
	}
	if report.Curriculum != curriculumID {
		return fmt.Errorf("assessment curriculum %q does not match active curriculum %q", report.Curriculum, curriculumID)
	}
	if report.Depth != depth {
		return fmt.Errorf("assessment depth %q does not match active depth %q", report.Depth, depth)
	}
	if len(report.ExistingKnowledge)+len(report.PrerequisiteGaps)+len(report.PossibleMisconceptions) == 0 {
		return fmt.Errorf("assessment must contain at least one evidence-backed finding")
	}
	userEvidence, err := roleText(conversationText, "user")
	if err != nil {
		return err
	}
	for category, findings := range map[string][]Finding{
		"learning_goals":          report.LearningGoals,
		"existing_knowledge":      report.ExistingKnowledge,
		"prerequisite_gaps":       report.PrerequisiteGaps,
		"possible_misconceptions": report.PossibleMisconceptions,
	} {
		for i, finding := range findings {
			if strings.TrimSpace(finding.Claim) == "" {
				return fmt.Errorf("%s finding %d has no claim", category, i)
			}
			evidence := strings.TrimSpace(finding.Evidence)
			if evidence == "" {
				return fmt.Errorf("%s finding %d has no evidence", category, i)
			}
			if !strings.Contains(conversation.Normalize(userEvidence), conversation.Normalize(evidence)) {
				return fmt.Errorf("%s finding %d cites evidence not found in a raw user turn", category, i)
			}
			for _, c := range finding.Concepts {
				if !conceptID.MatchString(c) {
					return fmt.Errorf("%s finding %d: concept id %q must be kebab-case", category, i, c)
				}
			}
		}
	}
	if report.GoalCard != nil {
		if err := CheckGoalCard(*report.GoalCard, conversationText); err != nil {
			return err
		}
	}
	if strings.TrimSpace(report.RecommendationReason) == "" {
		return fmt.Errorf("assessment has no recommendation reason")
	}
	if strings.TrimSpace(report.NextProbe) == "" {
		return fmt.Errorf("assessment has no next probe")
	}
	if report.RecommendedEntry.Book != "" && report.RecommendedEntry.Book != curriculumID {
		return fmt.Errorf("recommended entry book %q does not match active curriculum %q", report.RecommendedEntry.Book, curriculumID)
	}
	return nil
}

var conceptID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// CheckGoalCard validates a goal card and that each field's evidence is the
// learner's own words in the conversation.
func CheckGoalCard(card curriculum.GoalCard, conversationText string) error {
	if err := card.Check(); err != nil {
		return err
	}
	userEvidence, err := roleText(conversationText, "user")
	if err != nil {
		return err
	}
	for name, it := range card.Items() {
		if !strings.Contains(conversation.Normalize(userEvidence), conversation.Normalize(strings.TrimSpace(it.Evidence))) {
			return fmt.Errorf("goal card %s cites evidence not found in a raw learner turn", name)
		}
	}
	return nil
}

func roleText(text, wantedRole string) (string, error) {
	conv, err := conversation.Parse(text)
	if err != nil {
		return "", err
	}
	return conv.RoleText(wantedRole), nil
}

func Save(root, sessionID string, report Report, now time.Time) (string, error) {
	dir := filepath.Join(root, "Profile", "Assessments", report.Curriculum)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create assessment directory: %w", err)
	}
	rel := filepath.ToSlash(filepath.Join("Profile", "Assessments", report.Curriculum, sessionID+".json"))
	saved := savedReport{
		Version: 1, SessionID: sessionID, CompletedAt: now.UTC().Format(time.RFC3339Nano), Report: report,
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode assessment: %w", err)
	}
	data = append(data, '\n')
	if err := fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(rel)), data, 0o644); err != nil {
		return "", err
	}
	markdownRel := strings.TrimSuffix(rel, ".json") + ".md"
	if err := fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(markdownRel)), []byte(render(saved)), 0o644); err != nil {
		return "", err
	}
	return markdownRel, nil
}

func CurrentStatus(root, curriculumID string) (Status, error) {
	if curriculumID == "" {
		return Status{State: "not_applicable"}, nil
	}
	dir := filepath.Join(root, "Profile", "Assessments", curriculumID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return Status{State: "not_assessed"}, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("read assessments: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return Status{State: "not_assessed"}, nil
	}
	sort.Strings(names)
	latest := names[len(names)-1]
	data, err := os.ReadFile(filepath.Join(dir, latest))
	if err != nil {
		return Status{}, fmt.Errorf("read latest assessment: %w", err)
	}
	var saved savedReport
	if err := json.Unmarshal(data, &saved); err != nil {
		return Status{}, fmt.Errorf("parse latest assessment: %w", err)
	}
	return Status{
		State: "assessed", Path: filepath.ToSlash(filepath.Join("Profile", "Assessments", curriculumID, strings.TrimSuffix(latest, ".json")+".md")),
		CompletedAt: saved.CompletedAt, SessionID: saved.SessionID,
	}, nil
}

func render(saved savedReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nsession: %s\ncurriculum: %s\ndepth: %s\ncompleted_at: %s\n---\n\n# Baseline Assessment\n", saved.SessionID, saved.Curriculum, saved.Depth, saved.CompletedAt)
	renderFindings(&b, "Learning Goals", saved.LearningGoals)
	renderFindings(&b, "Existing Knowledge", saved.ExistingKnowledge)
	renderFindings(&b, "Prerequisite Gaps", saved.PrerequisiteGaps)
	renderFindings(&b, "Possible Misconceptions", saved.PossibleMisconceptions)
	renderStrings(&b, "Familiar Vocabulary", saved.FamiliarVocabulary)
	renderStrings(&b, "Unknown Vocabulary", saved.UnknownVocabulary)
	b.WriteString("\n## Recommended Entry\n\n")
	fmt.Fprintf(&b, "- Chapter: %s\n- Section: %s\n- Concept: %s\n- Reason: %s\n", valueOrNone(saved.RecommendedEntry.Chapter), valueOrNone(saved.RecommendedEntry.Section), valueOrNone(saved.RecommendedEntry.CurrentConcept), saved.RecommendationReason)
	fmt.Fprintf(&b, "\n## Next Probe\n\n%s\n", saved.NextProbe)
	return b.String()
}

func renderFindings(b *strings.Builder, title string, findings []Finding) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	if len(findings) == 0 {
		b.WriteString("None recorded.\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- %s\n  - Evidence: %s\n", finding.Claim, finding.Evidence)
	}
}

func renderStrings(b *strings.Builder, title string, values []string) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	if len(values) == 0 {
		b.WriteString("None recorded.\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
}

func valueOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Not set"
	}
	return value
}

// Latest returns the most recent assessment of a curriculum, or nil.
func Latest(root, curriculumID string) (*Report, error) {
	status, err := CurrentStatus(root, curriculumID)
	if err != nil || status.State != "assessed" {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(status.Path, ".md")+".json")))
	if err != nil {
		return nil, fmt.Errorf("read latest assessment: %w", err)
	}
	var saved savedReport
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("parse latest assessment: %w", err)
	}
	return &saved.Report, nil
}
