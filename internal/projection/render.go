package projection

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/i18n"

	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/policy"
	"github.com/hedykan/learning-system/internal/tags"
)

var reviewLabel = map[string]string{"recalled": "想起", "partial": "部分想起", "forgotten": "忘记"}

var stateLabel = map[string]string{"unobserved": "未观察", "developing": "形成中", "fragile": "脆弱", "stable": "稳定"}

func (in Inputs) state(s string) string { return in.t(stateLabel[s]) + in.t("（") + s + in.t("）") }

// t translates fixed page text into the interface language.
func (in Inputs) t(s string) string { return i18n.T(in.Lang, s) }

// sessionLabel turns session-20260924-090233.584 into 2026-09-24 09:02.
func sessionLabel(id string) string {
	raw := strings.TrimPrefix(id, "session-")
	if len(raw) < 13 {
		return id
	}
	return fmt.Sprintf("%s-%s-%s %s:%s", raw[0:4], raw[4:6], raw[6:8], raw[9:11], raw[11:13])
}

func sessionLink(id string) string { return fmt.Sprintf("[[Sessions/%s|%s]]", id, sessionLabel(id)) }

func (in Inputs) evidenceLink(e learner.EvidenceRef) string {
	target, when := "Conversations/"+e.Session, sessionLabel(e.Session)
	if t, err := in.Resolver.Turn(e.Session, e.Turn); err == nil {
		if t.Anchored {
			target += "#^" + e.Turn
		}
		if ts, err := time.Parse(time.RFC3339Nano, t.Timestamp); err == nil {
			when = ts.Local().Format("2006-01-02 15:04")
		}
	}
	return fmt.Sprintf("[[%s|%s · %s]]", target, when, e.Turn)
}

// pointsSource says where textbook points were read: "原书第 42–45 页" for
// pages, the locator label otherwise.
func (in Inputs) pointsSource(l locator.Locator) string {
	if l.Kind == "page" {
		return fmt.Sprintf(in.t("原书%s"), l.Label(in.Lang))
	}
	return fmt.Sprintf(in.t("%s "), l.Label(in.Lang))
}

// plainQuotes lists quotes with their turn but no link; knowledge notes
// link to evidence only once per session (CR-2026-036).
func (in Inputs) plainQuotes(list []learner.EvidenceRef) string {
	parts := make([]string, 0, len(list))
	for _, e := range list {
		parts = append(parts, fmt.Sprintf(in.t("「%s」（%s · %s）"), e.Quote, in.when(e), e.Turn))
	}
	return strings.Join(parts, in.t("；"))
}

// when is the local time of an evidence turn, or its session start.
func (in Inputs) when(e learner.EvidenceRef) string {
	if t, err := in.Resolver.Turn(e.Session, e.Turn); err == nil {
		if ts, err := time.Parse(time.RFC3339Nano, t.Timestamp); err == nil {
			return ts.Local().Format("2006-01-02 15:04")
		}
	}
	return sessionLabel(e.Session)
}

// evidenceGroups renders quotes grouped by session, with one link per
// session to the conversation at its first quoted turn.
func (in Inputs) evidenceGroups(quotes []learner.EvidenceRef) string {
	if len(quotes) == 0 {
		return in.t("暂无。\n")
	}
	var b strings.Builder
	for i, q := range quotes {
		if i == 0 || quotes[i-1].Session != q.Session {
			fmt.Fprintf(&b, "- %s\n", in.evidenceLink(q))
		}
		fmt.Fprintf(&b, in.t("  - 「%s」（%s）\n"), q.Quote, q.Turn)
	}
	return b.String()
}

func (in Inputs) quotes(list []learner.EvidenceRef) string {
	parts := make([]string, 0, len(list))
	for _, e := range list {
		parts = append(parts, fmt.Sprintf(in.t("「%s」%s"), e.Quote, in.evidenceLink(e)))
	}
	return strings.Join(parts, in.t("；"))
}

func conceptLink(m *learner.Model, id string) string {
	if c, ok := m.Concepts[id]; ok {
		return link(ConceptFile(m, id), c.Label)
	}
	return id
}

func frontmatter(kind, key string, gen string) string {
	return fmt.Sprintf("---\ngenerated_by: learn\nprojection: %s\n%smodel_generation: %s\n---\n\n", kind, key, gen)
}

const notice = "> 本页由 Learning OS 根据已校验的学习证据生成，重建时会被覆盖；只有「手写笔记」区域会原样保留。AI 解释以第三人称书写，学习者原话链接到原始对话。\n\n"

func renderConcept(in Inputs, c *learner.Concept) string {
	m := in.Model
	var b strings.Builder
	var extra strings.Builder
	fmt.Fprintf(&extra, "concept: %s\nstate: %s\n", c.ID, c.State())
	extra.WriteString("aliases:\n")
	for _, a := range append(append([]string{}, c.Aliases...), c.ID) {
		fmt.Fprintf(&extra, "  - %q\n", a)
	}
	extra.WriteString("tags:\n  - " + tags.Concept + "\n")
	fmt.Fprintf(&extra, "  - learning/state/%s\n", c.State())
	seenCurr := map[string]bool{}
	for _, r := range c.SourceRefs {
		if !seenCurr[r.Curriculum] {
			seenCurr[r.Curriculum] = true
			fmt.Fprintf(&extra, "  - learning/curriculum/%s\n", r.Curriculum)
		}
	}
	local, cross := splitRelations(m, c)
	if len(cross) > 0 {
		extra.WriteString("  - learning/cross-curriculum\n")
	}
	b.WriteString(frontmatter("concept", extra.String(), m.Generation))
	fmt.Fprintf(&b, "# %s\n\n%s", c.Label, in.t(notice))
	caps := c.Capabilities()
	capText := in.t("无")
	if len(caps) > 0 {
		capText = strings.Join(caps, in.t("、"))
	}
	fmt.Fprintf(&b, in.t("- 当前状态：%s\n- 能力证据：%s\n"), in.state(c.State()), capText)
	if len(c.Aliases) > 0 {
		fmt.Fprintf(&b, in.t("- 别名：%s\n"), strings.Join(c.Aliases, "、"))
	}
	for _, r := range c.SourceRefs {
		loc := strings.Trim(strings.TrimSpace(strings.Join([]string{r.Chapter, r.Section}, " · ")), " ·")
		fmt.Fprintf(&b, in.t("- 教材来源：%s %s\n"), in.Titles[r.Curriculum], loc)
	}

	if n := len(c.Points); n > 0 {
		cur := c.Points[n-1]
		fmt.Fprintf(&b, in.t("\n## 教材要点\n\n> AI 根据%s概括，未经学习者核对。\n\n"), in.pointsSource(cur.Locator))
		for _, p := range cur.Points {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		if n > 1 {
			b.WriteString(in.t("\n此前版本：\n\n"))
			for i := n - 2; i >= 0; i-- {
				old := c.Points[i]
				fmt.Fprintf(&b, in.t("- %s（%s）：%s\n"), sessionLabel(old.Session), old.Locator.Label(in.Lang), strings.Join(old.Points, in.t("；")))
			}
		}
	}

	in.renderRelations(&b, in.t("\n## 相关概念\n\n"), local, false)
	in.renderRelations(&b, in.t("\n## 与其他教材的联系\n\n"), cross, true)

	b.WriteString(in.t("\n## 学习者原话\n\n"))
	seen := map[string]bool{}
	var quotes []learner.EvidenceRef
	collect := func(list []learner.EvidenceRef) {
		for _, e := range list {
			key := e.Session + e.Turn + e.Quote
			if !seen[key] {
				seen[key] = true
				quotes = append(quotes, e)
			}
		}
	}
	for _, ev := range m.EventsFor(c.ID) {
		if ev.Superseded == "" && ev.DerivedBy == "" {
			collect(ev.Evidence)
		}
	}
	for _, ch := range m.ChangesFor(c.ID) {
		if ch.Superseded == "" {
			collect(ch.OldEvidence)
			collect(ch.NewEvidence)
		}
	}
	for _, h := range c.History {
		if h.Superseded == "" {
			collect(h.Evidence)
		}
	}
	quotes = mergeOverlapping(quotes)
	sort.SliceStable(quotes, func(i, j int) bool {
		if quotes[i].Session != quotes[j].Session {
			return quotes[i].Session < quotes[j].Session
		}
		return quotes[i].Turn < quotes[j].Turn
	})
	b.WriteString(in.evidenceGroups(quotes))

	b.WriteString(in.t("\n## 当前理解（AI 解释）\n\n"))
	if cur := c.Current(); cur != nil {
		fmt.Fprintf(&b, "%s\n", cur.Summary)
	} else {
		b.WriteString(in.t("尚无已校验的理解判断。\n"))
	}

	b.WriteString(in.t("\n## 误解与认知变化\n\n"))
	wrote := false
	open := map[string]bool{}
	for _, ev := range m.UnresolvedMisconceptions(c.ID) {
		open[ev.GID] = true
	}
	for _, ev := range m.EventsFor(c.ID) {
		if ev.Type != "misconception" {
			continue
		}
		status := in.t("已修正")
		if open[ev.GID] {
			status = in.t("待修正")
		}
		if ev.Superseded != "" {
			status = in.t("已撤回")
		}
		fmt.Fprintf(&b, in.t("- 误解（%s）：%s；证据 %s\n"), status, ev.Summary, in.plainQuotes(ev.Evidence))
		wrote = true
	}
	for _, ch := range m.ChangesFor(c.ID) {
		mark := ""
		if ch.Superseded != "" {
			mark = in.t("（已撤回）")
		}
		fmt.Fprintf(&b, in.t("- 认知变化%s：%s → 触发：%s → %s\n  - 之前：%s\n  - 之后：%s\n"), mark, ch.OldModel, ch.Trigger, ch.NewModel, in.plainQuotes(ch.OldEvidence), in.plainQuotes(ch.NewEvidence))
		wrote = true
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n## 教学策略记录\n\n"))
	wrote = false
	for _, a := range m.Attempts {
		if a.Concept != c.ID {
			continue
		}
		mark := ""
		if a.Superseded != "" {
			mark = in.t("（已撤回）")
		}
		line := fmt.Sprintf(in.t("- %s · %s · %s%s：预期 %s"), sessionLabel(a.Session), a.Strategy, a.Outcome, mark, a.ExpectedChange)
		if a.Reason != "" {
			line += in.t("；原因：") + a.Reason
		}
		b.WriteString(line + "\n")
		wrote = true
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	var asked []*learner.QuestionItem
	for _, q := range m.QuestionList() {
		if q.Concept == c.ID {
			asked = append(asked, q)
		}
	}
	if len(asked) > 0 {
		b.WriteString(in.t("\n## 学习者提出的问题\n\n"))
		for _, q := range asked {
			fmt.Fprintf(&b, in.t("- %s（%s）\n"), questionLink(m, q), in.t(questionStatus[q.Status]))
		}
	}

	b.WriteString(in.t("\n## 复习计划\n\n"))
	if s := m.ScheduleFor(c); s.Started {
		fmt.Fprintf(&b, in.t("- 下次复习：%s（第 %d 档，间隔 %d 天）\n- 已复习：%d 次\n"), s.Due, s.Level+1, s.Interval, s.Reviews)
		for _, r := range m.Reviews {
			if r.Concept == c.ID {
				fmt.Fprintf(&b, in.t("  - %s：%s；证据 %s\n"), sessionLabel(r.Session), in.t(reviewLabel[r.Outcome]), in.plainQuotes(r.Evidence))
			}
		}
	} else {
		b.WriteString(in.t("尚未开始。概念形成理解后自动排期。\n"))
	}

	b.WriteString(in.t("\n## 开放问题\n\n"))
	if cur := c.Current(); cur != nil && len(cur.OpenQuestions) > 0 {
		for _, q := range cur.OpenQuestions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
	} else {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n## 修订历史\n\n"))
	if len(c.History) == 0 {
		b.WriteString(in.t("暂无。\n"))
	} else {
		b.WriteString(in.t("| Session | 状态 | 能力 | 判断 | 备注 |\n| --- | --- | --- | --- | --- |\n"))
		for _, h := range c.History {
			note := ""
			if h.Superseded != "" {
				note = h.Superseded
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", sessionLabel(h.Session), in.state(h.State), strings.Join(h.Capabilities, in.t("、")), cell(h.Summary), cell(note))
		}
	}
	return b.String()
}

// mergeOverlapping keeps, per turn, only quotes not contained in a longer one.
func mergeOverlapping(quotes []learner.EvidenceRef) []learner.EvidenceRef {
	var out []learner.EvidenceRef
	for i, q := range quotes {
		covered := false
		for j, other := range quotes {
			if i == j || other.Session != q.Session || other.Turn != q.Turn {
				continue
			}
			if len(other.Quote) > len(q.Quote) && strings.Contains(other.Quote, q.Quote) || (other.Quote == q.Quote && j < i) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, q)
		}
	}
	return out
}

var questionStatus = map[string]string{"open": "开放", "resolved": "已解决"}

func questionLink(m *learner.Model, q *learner.QuestionItem) string {
	return link(QuestionFile(m, q.ID), q.Question)
}

// renderQuestion renders one learner-generated key question as its own note.
func renderQuestion(in Inputs, q *learner.QuestionItem) string {
	m := in.Model
	var b strings.Builder
	extra := fmt.Sprintf("question: %s\nstatus: %s\naliases:\n  - %q\ntags:\n  - %s\n  - learning/question/%s\n  - learning/curriculum/%s\n", q.ID, q.Status, q.ID, tags.Question, q.Status, q.Curriculum)
	b.WriteString(frontmatter("question", extra, m.Generation))
	fmt.Fprintf(&b, "# %s\n\n%s", q.Question, in.t(notice))
	fmt.Fprintf(&b, in.t("- 状态：%s\n- 提出于：%s\n"), in.t(questionStatus[q.Status]), sessionLabel(q.Session))
	if q.Concept != "" {
		fmt.Fprintf(&b, in.t("- 相关概念：%s\n"), conceptLink(m, q.Concept))
	}
	if q.Node != "" {
		fmt.Fprintf(&b, in.t("- 预计在目录条目 %s 回答\n"), q.Node)
	}
	b.WriteString(in.t("\n## 学习者原话\n\n") + in.evidenceGroups(q.Evidence))
	b.WriteString(in.t("\n## 解答\n\n"))
	if r := q.Resolution; r != nil {
		fmt.Fprintf(&b, in.t("%s\n\n- 解决于：%s\n- 证据：%s\n"), r.Summary, sessionLabel(r.Session), in.plainQuotes(r.Evidence))
	} else {
		b.WriteString(in.t("尚未解决。学习位置到达相关小节或概念时，Agent 会先回来回答它。\n"))
	}
	return b.String()
}

func cell(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }

func (in Inputs) renderNext(b *strings.Builder, m *learner.Model, next *policy.Action) {
	b.WriteString(in.t("\n## 推荐下一步\n\n"))
	if next == nil {
		b.WriteString(in.t("当前没有激活的教材。\n"))
		return
	}
	fmt.Fprintf(b, in.t("- 动作：%s"), next.Action)
	if next.Concept != "" {
		fmt.Fprintf(b, in.t("（%s）"), conceptLink(m, next.Concept))
	}
	b.WriteString("\n")
	if next.Strategy != "" {
		fmt.Fprintf(b, in.t("- 策略：%s\n"), next.Strategy)
	}
	if next.Node != "" {
		fmt.Fprintf(b, in.t("- 下一节：%s %s\n"), next.Node, next.NodeTitle)
	}
	fmt.Fprintf(b, in.t("- 与教材主线的关系：%s\n- 理由：%s（%s）\n"), next.CurriculumRelation, next.Reason, next.Rule)
	if next.HistoryUsed {
		b.WriteString(in.t("- 依据了此前 Session 的学习证据\n"))
	}
}

func renderOverview(in Inputs) string {
	m := in.Model
	var b strings.Builder
	b.WriteString(frontmatter("learner-overview", tags.Lines(tags.Overview), m.Generation))
	b.WriteString(in.t("# 学习者总览\n\n") + in.t(notice))

	b.WriteString(in.t("## 概念状态\n\n| 概念 | 状态 | 能力证据 | 教材 |\n| --- | --- | --- | --- |\n"))
	for _, c := range m.ConceptList() {
		var curricula []string
		for _, r := range c.SourceRefs {
			curricula = append(curricula, in.Titles[r.Curriculum])
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", conceptLink(m, c.ID), in.state(c.State()), strings.Join(c.Capabilities(), in.t("、")), cell(strings.Join(curricula, in.t("、"))))
	}

	b.WriteString(in.t("\n## 待修正的误解\n\n"))
	wrote := false
	for _, c := range m.ConceptList() {
		for _, ev := range m.UnresolvedMisconceptions(c.ID) {
			fmt.Fprintf(&b, in.t("- %s：%s；证据 %s\n"), conceptLink(m, c.ID), ev.Summary, in.quotes(ev.Evidence))
			wrote = true
		}
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n## 开放问题\n\n"))
	wrote = false
	for _, c := range m.ConceptList() {
		if cur := c.Current(); cur != nil {
			for _, q := range cur.OpenQuestions {
				fmt.Fprintf(&b, in.t("- %s：%s\n"), conceptLink(m, c.ID), q)
				wrote = true
			}
		}
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n## 教学策略证据\n\n"))
	type key struct{ strategy, situation string }
	counts := map[key][3]int{}
	for _, a := range m.LiveAttempts() {
		k := key{a.Strategy, a.Situation}
		c := counts[k]
		c[map[string]int{"effective": 0, "inconclusive": 1, "ineffective": 2}[a.Outcome]]++
		counts[k] = c
	}
	if len(counts) == 0 {
		b.WriteString(in.t("暂无。\n"))
	} else {
		keys := make([]key, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].strategy != keys[j].strategy {
				return keys[i].strategy < keys[j].strategy
			}
			return keys[i].situation < keys[j].situation
		})
		b.WriteString(in.t("结果是可修正的观察，不代表因果证明。\n\n| 策略 | 情境 | 有效 | 不确定 | 无效 |\n| --- | --- | --- | --- | --- |\n"))
		for _, k := range keys {
			c := counts[k]
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d |\n", k.strategy, k.situation, c[0], c[1], c[2])
		}
	}

	b.WriteString(in.t("\n## 学习者提出的问题\n\n"))
	if qs := m.QuestionList(); len(qs) == 0 {
		b.WriteString(in.t("暂无。\n"))
	} else {
		for _, q := range qs {
			fmt.Fprintf(&b, in.t("- %s（%s）\n"), questionLink(m, q), in.t(questionStatus[q.Status]))
		}
	}

	b.WriteString(in.t("\n## 复习日程\n\n"))
	if schedules := m.Schedules(); len(schedules) == 0 {
		b.WriteString(in.t("暂无。\n"))
	} else {
		b.WriteString(in.t("| 概念 | 下次复习 | 档位 | 最近结果 |\n| --- | --- | --- | --- |\n"))
		for _, s := range schedules {
			last := "—"
			if s.LastResult != "" {
				last = in.t(reviewLabel[s.LastResult]) + in.t("（") + s.LastReview + in.t("）")
			}
			fmt.Fprintf(&b, in.t("| %s | %s | 第 %d 档（%d 天） | %s |\n"), conceptLink(m, s.Concept), s.Due, s.Level+1, s.Interval, last)
		}
	}

	b.WriteString(in.t("\n## 学习模式\n\n"))
	if len(m.Patterns) == 0 {
		b.WriteString(in.t("暂无。模式需要来自至少两门教材、两个 Session 的独立证据才会被采信。\n"))
	}
	for _, p := range m.PatternList() {
		supports, contradicts := 0, 0
		for _, o := range p.Observations {
			if o.Stance == "supports" {
				supports++
			} else {
				contradicts++
			}
		}
		fmt.Fprintf(&b, in.t("### %s（%s）\n\n%s\n\n- 支持 %d 次，反例 %d 次\n"), p.ID, p.Status(), p.Description, supports, contradicts)
		if p.PreferredStrategy != "" {
			fmt.Fprintf(&b, in.t("- 倾向策略：%s\n"), p.PreferredStrategy)
		}
		for _, o := range p.Observations {
			if o.Stance == "contradicts" {
				fmt.Fprintf(&b, in.t("- 反例（%s）：%s；证据 %s\n"), in.Titles[o.Curriculum], o.Summary, in.quotes(o.Evidence))
			}
		}
		b.WriteString("\n")
	}
	in.renderNext(&b, m, in.Next)
	return b.String()
}

func renderCurriculum(in Inputs, id string) string {
	m := in.Model
	var b strings.Builder
	b.WriteString(frontmatter("curriculum-index", "curriculum: "+id+"\n"+tags.Lines(tags.Curriculum), m.Generation))
	fmt.Fprintf(&b, "# %s\n\n%s", in.Titles[id], in.t(notice))
	pos := in.Positions[id]
	if o := in.Outlines[id]; o.TypeOf() == curriculum.Synthesized {
		b.WriteString(in.t("> 这门课程由 Agent 根据学习目标组织，条目的顺序与来源都记录在下方；没有原始资料的条目是 AI 综合的内容。\n\n"))
	}
	b.WriteString(in.t("## 教材位置\n\n"))
	fmt.Fprintf(&b, in.t("- 章节：%s\n- 小节：%s\n- 当前概念：%s\n"), in.orNone(pos.Chapter), in.orNone(pos.Section), in.orNone(pos.CurrentConcept))
	if pos.Node != "" {
		fmt.Fprintf(&b, in.t("- 目录条目：%s\n"), pos.Node)
	}
	if d := pos.Detour; d != nil {
		fmt.Fprintf(&b, in.t("- 正在绕行补先修：%s（原因：%s；返回条件：%s；返回点：%s / %s）\n"), d.Topic, d.Reason, d.ReturnCondition, in.orNone(d.ReturnTo.Chapter), in.orNone(d.ReturnTo.Concept))
	}
	b.WriteString(in.t("\n教材位置与理解状态相互独立：读到某一章不代表已经理解。\n"))

	b.WriteString(in.t("\n## 目录\n\n"))
	switch o := in.Outlines[id]; {
	case o.Status == "" || o.Status == "missing":
		b.WriteString(in.t("尚未建立目录，学习位置无法与原书核对。\n"))
	default:
		if o.Status == "draft" {
			b.WriteString(in.t("目录为草稿，尚未经学习者确认。\n\n"))
		}
		for _, s := range in.Statuses[id] {
			mark := ""
			if in.Unsourced[id][s.ID] {
				mark = in.t("（AI 综合，无原始资料）")
			}
			if known, _ := policy.KnownElsewhere(m, s.Concepts); known && s.Status != "completed" && s.Status != "skipped" {
				mark += in.t("（已在其他课程掌握）")
			}
			fmt.Fprintf(&b, "%s- %s %s%s\n", strings.Repeat("  ", s.Depth-1), in.t(curriculum.StatusLabel[s.Status]), s.Label(), mark)
			indent := strings.Repeat("  ", s.Depth-1)
			if s.Why != "" {
				fmt.Fprintf(&b, in.t("%s  - 为什么学：%s\n"), indent, s.Why)
			}
			if len(s.Prerequisites) > 0 {
				fmt.Fprintf(&b, in.t("%s  - 先修：%s\n"), indent, strings.Join(s.Prerequisites, in.t("、")))
			}
			for _, a := range in.Resources[id].Attachments {
				if a.Node == s.ID {
					fmt.Fprintf(&b, in.t("%s  - 资料：%s %s\n"), strings.Repeat("  ", s.Depth-1), in.resourceTitle(a.Locator.Resource), a.Locator.Label(in.Lang))
				}
			}
		}
	}

	if set := in.Resources[id]; len(set.Resources) > 0 {
		b.WriteString(in.t("\n## 资料\n\n"))
		fmt.Fprintf(&b, in.t("- %s（主资料）\n"), in.Titles[id])
		for _, r := range set.Resources {
			line := "- " + in.resourceTitle(r.ID)
			if res, err := curriculum.LoadResource(in.Root, r.ID); err == nil && res.URL != "" {
				line += " · " + res.URL
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString(in.t("\n## 本教材的概念\n\n"))
	var rows []string
	for _, c := range m.ConceptList() {
		for _, r := range c.SourceRefs {
			if r.Curriculum == id {
				rows = append(rows, fmt.Sprintf("| %s | %s | %s |", in.orNone(r.Chapter), conceptLink(m, c.ID), in.state(c.State())))
				break
			}
		}
	}
	if len(rows) == 0 {
		b.WriteString(in.t("暂无。\n"))
	} else {
		b.WriteString(in.t("| 章节 | 概念 | 理解状态 |\n| --- | --- | --- |\n") + strings.Join(rows, "\n") + "\n")
	}

	b.WriteString(in.t("\n## 先修绕行记录\n\n"))
	if len(in.DetourLogs[id]) == 0 {
		b.WriteString(in.t("暂无。\n"))
	}
	for _, d := range in.DetourLogs[id] {
		fmt.Fprintf(&b, in.t("- %s（%s）：原因 %s；学到 %s；返回 %s / %s\n"), d.Topic, d.Outcome, d.Reason, d.Learned, in.orNone(d.ReturnTo.Chapter), in.orNone(d.ReturnTo.Concept))
	}

	b.WriteString(in.t("\n## 学习记录\n\n"))
	var sessions []string
	for sid, s := range m.Sessions {
		if s.Curriculum == id {
			sessions = append(sessions, sid)
		}
	}
	sort.Strings(sessions)
	if len(sessions) == 0 {
		b.WriteString(in.t("暂无。\n"))
	}
	for _, sid := range sessions {
		fmt.Fprintf(&b, "- %s\n", sessionLink(sid))
	}
	if id == in.Active {
		in.renderNext(&b, m, in.Next)
	}
	return b.String()
}

func renderSessionAnalysis(in Inputs, id string) string {
	m := in.Model
	var b strings.Builder
	fmt.Fprintf(&b, in.t("## 学习解读\n\n由已校验的 Interpretation Record 生成（记录 %d 条）。\n"), m.Sessions[id].Records)

	b.WriteString(in.t("\n### 学习事件\n\n"))
	wrote := false
	for _, ev := range m.Events {
		if ev.Session != id {
			continue
		}
		mark := ""
		if ev.Superseded != "" {
			mark = in.t("（已撤回）")
		}
		concept := ""
		if ev.Concept != "" {
			concept = " · " + conceptLink(m, ev.Concept)
		}
		fmt.Fprintf(&b, in.t("- **%s**%s%s：%s；证据 %s\n"), ev.Type, concept, mark, ev.Summary, in.quotes(ev.Evidence))
		wrote = true
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n### 认知变化\n\n"))
	wrote = false
	for _, ch := range m.Changes {
		if ch.Session == id {
			fmt.Fprintf(&b, in.t("- %s：%s → 触发：%s → %s\n"), conceptLink(m, ch.Concept), ch.OldModel, ch.Trigger, ch.NewModel)
			wrote = true
		}
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n### 概念状态更新\n\n"))
	wrote = false
	for _, c := range m.ConceptList() {
		for _, h := range c.History {
			if h.Session == id {
				fmt.Fprintf(&b, in.t("- %s → %s：%s\n"), conceptLink(m, c.ID), in.state(h.State), h.Summary)
				wrote = true
			}
		}
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	b.WriteString(in.t("\n### 教学策略\n\n"))
	wrote = false
	for _, a := range m.Attempts {
		if a.Session == id {
			fmt.Fprintf(&b, in.t("- %s · %s（%s）· %s：%s\n"), conceptLink(m, a.Concept), a.Strategy, a.Situation, a.Outcome, a.ExpectedChange)
			wrote = true
		}
	}
	if !wrote {
		b.WriteString(in.t("暂无。\n"))
	}

	for _, p := range m.Progress {
		if p.Session == id {
			fmt.Fprintf(&b, in.t("\n### 进度决策\n\n%s：%s\n"), p.Decision, p.Reason)
		}
	}
	return b.String()
}

// resourceTitle names a resource for learners, falling back to its id.
func (in Inputs) resourceTitle(id string) string {
	if t, ok := in.Titles[id]; ok {
		return t
	}
	if r, err := curriculum.LoadResource(in.Root, id); err == nil {
		return r.Title
	}
	return id
}

func (in Inputs) orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return in.t("未设置")
	}
	return s
}
