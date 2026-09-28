// Package policy implements the deterministic v0.1.2 Learning Policy. Rules
// are fixed and ordered so a replay of the same history always yields the
// same Next Best Learning Action.
package policy

import (
	"strings"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/learner"
)

// Context is the curriculum situation the policy decides in.
type Context struct {
	Curriculum      string
	Assessed        bool
	BaselineSkipped bool
	Position        curriculum.Position
	ActiveSession   string
	// NextNode is the next unfinished outline entry, if an outline exists.
	NextNode *curriculum.Node
	// Today is the learner's local date (YYYY-MM-DD); empty disables R3b.
	Today string
	// Lang is the interface language of Reason texts (CR-2026-023).
	Lang string
}

// Action is one Next Best Learning Action.
type Action struct {
	Action             string   `json:"action"`
	Concept            string   `json:"concept,omitempty"`
	Node               string   `json:"node,omitempty"`
	NodeTitle          string   `json:"node_title,omitempty"`
	Strategy           string   `json:"strategy,omitempty"`
	Situation          string   `json:"situation,omitempty"`
	CurriculumRelation string   `json:"curriculum_relation"`
	Rule               string   `json:"rule"`
	Reason             string   `json:"reason"`
	Evidence           []string `json:"evidence"`
	HistoryUsed        bool     `json:"history_used"`
	// OpenQuestions lists up to three open learner questions (CR-2026-020).
	OpenQuestions []QuestionRef `json:"open_questions,omitempty"`
}

// QuestionRef is an open learner question shown with every recommendation.
type QuestionRef struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Concept  string `json:"concept,omitempty"`
	Node     string `json:"node,omitempty"`
}

// Defaults is the strategy used when no history applies.
var Defaults = map[string]string{
	"new_concept": "prediction_probe", "misconception": "counterexample", "prerequisite_gap": "direct_explanation",
	"retrieval": "retrieval_practice", "transfer": "concrete_example",
}

var fallbackOrder = []string{"prediction_probe", "concrete_example", "counterexample", "analogy", "diagram",
	"learner_action", "direct_explanation", "retrieval_practice", "prerequisite_repair"}

// Next returns the single recommended action.
func Next(m *learner.Model, ctx Context) Action {
	act := decide(m, ctx)
	if act.Situation != "" {
		var evidence []string
		act.Strategy, evidence = chooseStrategy(m, act.Situation, act.Concept)
		act.Evidence = append(act.Evidence, evidence...)
	}
	if act.Evidence == nil {
		act.Evidence = []string{}
	}
	for _, q := range m.OpenQuestions(ctx.Curriculum) {
		if len(act.OpenQuestions) == 3 {
			break
		}
		act.OpenQuestions = append(act.OpenQuestions, QuestionRef{ID: q.ID, Question: q.Question, Concept: q.Concept, Node: q.Node})
	}
	for _, gid := range act.Evidence {
		if session, _, _ := strings.Cut(gid, ":"); session != ctx.ActiveSession {
			act.HistoryUsed = true
		}
	}
	return act
}

func decide(m *learner.Model, ctx Context) Action {
	pos := ctx.Position
	relation := "mainline"
	if pos.Detour != nil {
		relation = "detour"
	}
	if ctx.Curriculum != "" && !ctx.Assessed && !ctx.BaselineSkipped {
		return Action{Action: "baseline", CurriculumRelation: relation, Rule: "R0-no-baseline", Reason: i18n.T(ctx.Lang, "当前教材还没有摸底评估")}
	}
	if d := pos.Detour; d != nil {
		key := d.Concept
		if key == "" {
			key = d.Topic
		}
		if c := m.FindConcept(key); c != nil && (c.State() == "developing" || c.State() == "stable") {
			target := d.ReturnTo.Concept
			if rc := m.FindConcept(target); rc != nil {
				target = rc.ID
			}
			return Action{Action: "return_to_mainline", Concept: target, Situation: "new_concept", CurriculumRelation: "return",
				Rule: "R1-detour-resolved", Reason: i18n.F(ctx.Lang, "先修主题「%s」已达 %s，应回到返回点", c.Label, c.State()),
				Evidence: []string{c.Current().GID}}
		}
	}
	scope := scopeConcepts(m, ctx)
	for _, c := range scope {
		if open := m.UnresolvedMisconceptions(c.ID); len(open) > 0 {
			latest := open[len(open)-1]
			return Action{Action: "repair_misconception", Concept: c.ID, Situation: "misconception", CurriculumRelation: relation,
				Rule: "R2-open-misconception", Reason: i18n.F(ctx.Lang, "「%s」有尚未修正的误解：%s", c.Label, latest.Summary),
				Evidence: []string{latest.GID}}
		}
	}
	if ctx.Position.Detour == nil {
		current := currentConcept(m, ctx)
		for _, q := range m.OpenQuestions(ctx.Curriculum) {
			onNode := q.Node != "" && q.Node == ctx.Position.Node
			onConcept := q.Concept != "" && current != nil && current.ID == q.Concept
			if !onNode && !onConcept {
				continue
			}
			return Action{Action: "address_question", Concept: q.Concept, Situation: "new_concept", CurriculumRelation: relation,
				Rule: "R2b-open-question", Reason: i18n.F(ctx.Lang, "学习者之前提出的问题在这里可以回答：%s", q.Question),
				Evidence: []string{q.Session + ":" + q.ID}}
		}
	}
	for _, c := range scope {
		if c.State() == "fragile" {
			return Action{Action: "retrieval_probe", Concept: c.ID, Situation: "retrieval", CurriculumRelation: relation,
				Rule: "R3-fragile", Reason: i18n.F(ctx.Lang, "「%s」当前理解脆弱，需要重新检索验证", c.Label),
				Evidence: []string{c.Current().GID}}
		}
	}
	if ctx.Today != "" && ctx.Curriculum != "" && ctx.Position.Detour == nil {
		for _, s := range m.Schedules() {
			c := m.Concepts[s.Concept]
			if s.Due > ctx.Today || !c.InChapter(ctx.Curriculum, "", "") || (ctx.ActiveSession != "" && m.ReviewedIn(c.ID, ctx.ActiveSession)) {
				continue
			}
			return Action{Action: "review_due", Concept: c.ID, Situation: "retrieval", CurriculumRelation: relation,
				Rule: "R3b-review-due", Reason: i18n.F(ctx.Lang, "「%s」按间隔复习计划在 %s 到期（第 %d 档，间隔 %d 天）", c.Label, s.Due, s.Level+1, s.Interval),
				Evidence: []string{s.Evidence}}
		}
	}
	current := currentConcept(m, ctx)
	if current != nil && current.State() == "developing" {
		caps := strings.Join(current.Capabilities(), ",")
		if strings.Contains(caps, "explained") && !strings.Contains(caps, "transferred") {
			action, missing := "transfer_probe", i18n.T(ctx.Lang, "迁移")
			if !strings.Contains(caps, "applied") {
				action, missing = "application_probe", i18n.T(ctx.Lang, "应用")
			}
			return Action{Action: action, Concept: current.ID, Situation: "transfer", CurriculumRelation: relation,
				Rule: "R4-missing-transfer", Reason: i18n.F(ctx.Lang, "「%s」已能解释，但还没有%s证据", current.Label, missing),
				Evidence: []string{current.Current().GID}}
		}
	}
	if ctx.Curriculum != "" && ctx.Position.Detour == nil {
		for _, c := range scope {
			if gid := needsConsolidation(m, c, ctx.ActiveSession); gid != "" {
				return Action{Action: "retrieval_probe", Concept: c.ID, Situation: "retrieval", CurriculumRelation: relation,
					Rule: "R4b-consolidate", Reason: i18n.F(ctx.Lang, "「%s」上次已能应用或迁移，推进前先在新的 Session 里独立回忆一次", c.Label),
					Evidence: []string{gid}}
			}
		}
	}
	target := pos.CurrentConcept
	if pos.Detour != nil && pos.Detour.Concept != "" {
		target = pos.Detour.Concept
	}
	if target != "" && (current == nil || current.State() == "unobserved") {
		id := target
		if current != nil {
			id = current.ID
		}
		return Action{Action: "explain_probe", Concept: id, Situation: "new_concept", CurriculumRelation: relation,
			Rule: "R5-unobserved", Reason: i18n.F(ctx.Lang, "「%s」还没有学习证据，先引出学习者的理解", target)}
	}
	act := Action{Action: "continue_curriculum", Situation: "new_concept", CurriculumRelation: relation,
		Rule: "R6-continue", Reason: i18n.T(ctx.Lang, "当前范围没有待修复或待验证的理解，沿教材继续")}
	if n := ctx.NextNode; n != nil {
		act.Node, act.NodeTitle = n.ID, n.Title
		act.Reason = i18n.F(ctx.Lang, "当前范围没有待修复或待验证的理解，按目录进入 %s %s", n.ID, n.Title)
	}
	return act
}

// scopeConcepts is the detour concept, or concepts in the current chapter.
func scopeConcepts(m *learner.Model, ctx Context) []*learner.Concept {
	if d := ctx.Position.Detour; d != nil {
		key := d.Concept
		if key == "" {
			key = d.Topic
		}
		if c := m.FindConcept(key); c != nil {
			return []*learner.Concept{c}
		}
		return nil
	}
	chapterNode := ""
	if ctx.Position.Node != "" {
		chapterNode = strings.Split(ctx.Position.Node, ".")[0]
	}
	if ctx.Curriculum == "" || (ctx.Position.Chapter == "" && chapterNode == "") {
		return nil
	}
	var out []*learner.Concept
	for _, c := range m.ConceptList() {
		if c.InChapter(ctx.Curriculum, chapterNode, ctx.Position.Chapter) {
			out = append(out, c)
		}
	}
	return out
}

// needsConsolidation implements R4b: a developing concept that was applied or
// transferred in an earlier session and not yet retrieved in this one. It
// returns the state entry that motivates the probe.
func needsConsolidation(m *learner.Model, c *learner.Concept, active string) string {
	cur := c.Current()
	if cur == nil || cur.State != "developing" || cur.Session == active {
		return ""
	}
	caps := strings.Join(c.Capabilities(), ",")
	if !strings.Contains(caps, "applied") && !strings.Contains(caps, "transferred") {
		return ""
	}
	if active != "" {
		for _, a := range m.LiveAttempts() {
			if a.Session == active && a.Concept == c.ID && a.Situation == "retrieval" {
				return ""
			}
		}
		for _, e := range m.EventsFor(c.ID) {
			if e.Session == active && e.Superseded == "" && e.Type == "retrieval" {
				return ""
			}
		}
	}
	return cur.GID
}

func currentConcept(m *learner.Model, ctx Context) *learner.Concept {
	if d := ctx.Position.Detour; d != nil {
		if c := m.FindConcept(d.Concept); c != nil {
			return c
		}
		return m.FindConcept(d.Topic)
	}
	return m.FindConcept(ctx.Position.CurrentConcept)
}

// chooseStrategy applies 7.3: supported pattern, then evidence score, then
// defaults, never picking a strategy that failed twice in a row on the concept.
func chooseStrategy(m *learner.Model, situation, conceptID string) (string, []string) {
	attempts := m.LiveAttempts()
	excluded := map[string][]string{}
	last := map[string][]*learner.Attempt{}
	for _, a := range attempts {
		if a.Concept == conceptID {
			last[a.Strategy] = append(last[a.Strategy], a)
		}
	}
	for strategy, list := range last {
		if n := len(list); n >= 2 && list[n-1].Outcome == "ineffective" && list[n-2].Outcome == "ineffective" {
			excluded[strategy] = []string{list[n-2].GID, list[n-1].GID}
		}
	}
	var exclusionEvidence []string
	for _, s := range fallbackOrder {
		exclusionEvidence = append(exclusionEvidence, excluded[s]...)
	}
	for _, p := range m.PatternList() {
		if p.Status() != "supported" || p.PreferredStrategy == "" || excluded[p.PreferredStrategy] != nil {
			continue
		}
		if p.Situation != "" && p.Situation != situation {
			continue
		}
		var evidence []string
		for _, o := range p.Observations {
			if o.Stance == "supports" {
				evidence = append(evidence, o.GID)
			}
		}
		return p.PreferredStrategy, append(evidence, exclusionEvidence...)
	}
	score := map[string]int{}
	lastEffective := map[string]int{}
	support := map[string][]string{}
	for _, a := range attempts {
		if a.Situation != situation {
			continue
		}
		switch a.Outcome {
		case "effective":
			score[a.Strategy]++
			lastEffective[a.Strategy] = a.Order
			support[a.Strategy] = append(support[a.Strategy], a.GID)
		case "ineffective":
			score[a.Strategy]--
		}
	}
	best, bestScore := "", 0
	for _, s := range fallbackOrder {
		if excluded[s] != nil || score[s] <= 0 {
			continue
		}
		if score[s] > bestScore || (score[s] == bestScore && lastEffective[s] > lastEffective[best]) {
			best, bestScore = s, score[s]
		}
	}
	if best != "" {
		return best, append(support[best], exclusionEvidence...)
	}
	if d := Defaults[situation]; excluded[d] == nil {
		return d, exclusionEvidence
	}
	for _, s := range fallbackOrder {
		if excluded[s] == nil {
			return s, exclusionEvidence
		}
	}
	return Defaults[situation], exclusionEvidence
}
