package projection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
)

// renderHome renders the Vault's README.md home page.
func renderHome(in Inputs) string {
	m := in.Model
	var b strings.Builder
	b.WriteString(frontmatter("home", "", m.Generation))
	b.WriteString("# 我的学习\n\n> 本页由 Learning OS 生成，是这个学习库的首页；只有「手写笔记」区域会原样保留。\n\n")

	b.WriteString("## 正在学习\n\n")
	switch {
	case len(in.Library) == 0:
		b.WriteString("还没有导入教材。在本目录启动 Codex 或 Claude，告诉它教材文件的位置即可导入。\n")
	case in.Active == "":
		b.WriteString("当前没有激活的教材。告诉 Agent 你想学哪一本。\n")
	default:
		pos := in.Positions[in.Active]
		fmt.Fprintf(&b, "- 教材：[[Curriculum/%s/index|%s]]\n", in.Active, in.Titles[in.Active])
		where := strings.Trim(strings.Join([]string{pos.Chapter, pos.Section}, " / "), " /")
		if where == "" {
			where = "尚未设定"
		}
		fmt.Fprintf(&b, "- 学到：%s\n", where)
		if pos.CurrentConcept != "" {
			fmt.Fprintf(&b, "- 当前概念：%s\n", pos.CurrentConcept)
		}
		switch in.Outlines[in.Active].Status {
		case "confirmed":
		case "draft":
			b.WriteString("- 目录：草稿，等待你确认\n")
		default:
			b.WriteString("- 目录：尚未建立，学习位置还不能和原书核对\n")
		}
		if n := in.Next; n != nil {
			fmt.Fprintf(&b, "- 下一步：%s\n", nextSentence(m, n.Action, n.Concept, n.Node, n.NodeTitle))
		}
	}

	fmt.Fprintf(&b, "\n## 今天该复习（%s）\n\n", in.Today)
	var due []learner.Schedule
	var next *learner.Schedule
	for _, s := range m.Schedules() {
		s := s
		if s.Due <= in.Today {
			due = append(due, s)
		} else if next == nil {
			next = &s
		}
	}
	switch {
	case len(due) > 0:
		for _, s := range due {
			fmt.Fprintf(&b, "- %s（%s 到期，第 %d 档）\n", conceptLink(m, s.Concept), s.Due, s.Level+1)
		}
		b.WriteString("\n说“继续学习”，会先安排这些复习。\n")
	case next != nil:
		fmt.Fprintf(&b, "今天没有到期的复习。下一次：%s，%s。\n", next.Due, conceptLink(m, next.Concept))
	default:
		b.WriteString("还没有需要复习的概念。\n")
	}

	b.WriteString("\n## 我的教材\n\n")
	if len(in.Library) == 0 {
		b.WriteString("暂无。\n")
	}
	for _, id := range in.Library {
		done, skipped, total := 0, 0, 0
		for _, s := range in.Statuses[id] {
			if hasChildren(in.Statuses[id], s.ID) {
				continue
			}
			total++
			switch s.Status {
			case "completed":
				done++
			case "skipped":
				skipped++
			}
		}
		progress := "尚未建立目录"
		if total > 0 {
			progress = fmt.Sprintf("完成 %d / %d 节，跳过 %d 节", done, total, skipped)
		}
		active := ""
		if id == in.Active {
			active = "（正在学习）"
		}
		fmt.Fprintf(&b, "- [[Curriculum/%s/index|%s]]%s：%s · [[Curriculum/%s/progress|进度]]\n", id, in.Titles[id], active, progress, id)
	}

	b.WriteString("\n## 学习者总览\n\n")
	if m.Empty() {
		b.WriteString("还没有学习记录。学完第一个概念后，这里会显示各概念的理解状态。\n")
	} else {
		counts := map[string]int{}
		misconceptions := 0
		for _, c := range m.ConceptList() {
			counts[c.State()]++
			misconceptions += len(m.UnresolvedMisconceptions(c.ID))
		}
		fmt.Fprintf(&b, "- [[Profile/learner-state|查看学习者总览]]\n- 概念：形成中 %d 个，脆弱 %d 个，稳定 %d 个\n- 待修正的误解：%d 个\n",
			counts["developing"], counts["fragile"], counts["stable"], misconceptions)
	}

	b.WriteString("\n## 最近学习\n\n")
	if len(in.Recent) == 0 {
		b.WriteString("暂无。\n")
	}
	kinds := map[string]string{"baseline": "摸底", "lesson": "学习", "review": "复习", "practice": "练习"}
	for _, r := range in.Recent {
		target := "Conversations/" + r.ID
		if r.HasSession {
			target = "Sessions/" + r.ID
		}
		kind := kinds[r.Kind]
		if kind == "" {
			kind = "学习"
		}
		title := in.Titles[r.Curriculum]
		if title == "" {
			title = r.Curriculum
		}
		fmt.Fprintf(&b, "- [[%s|%s]] %s · %s\n", target, sessionLabel(r.ID), kind, title)
	}

	b.WriteString("\n## 最近变化的概念\n\n")
	type change struct {
		concept *learner.Concept
		entry   *learner.StateEntry
	}
	var changes []change
	for _, c := range m.ConceptList() {
		for _, h := range c.History {
			if h.Superseded == "" {
				changes = append(changes, change{c, h})
			}
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].entry.At > changes[j].entry.At })
	if len(changes) > 5 {
		changes = changes[:5]
	}
	if len(changes) == 0 {
		b.WriteString("暂无。\n")
	}
	for _, ch := range changes {
		fmt.Fprintf(&b, "- %s → %s（%s）\n", conceptLink(m, ch.concept.ID), state(ch.entry.State), sessionLabel(ch.entry.Session))
	}

	b.WriteString("\n## 怎么用\n\n")
	b.WriteString("- 在这个目录启动 Codex 或 Claude，说“继续学习”。\n")
	b.WriteString("- 想学新教材时，告诉它文件路径；它会先和你核对目录。\n")
	b.WriteString("- 在任何笔记的「手写笔记」区写下自己的想法，重建时不会被覆盖。\n")
	return b.String()
}

func hasChildren(statuses []curriculum.NodeStatus, id string) bool {
	for _, s := range statuses {
		if strings.HasPrefix(s.ID, id+".") {
			return true
		}
	}
	return false
}

func nextSentence(m *learner.Model, action, concept, node, nodeTitle string) string {
	label := concept
	if c, ok := m.Concepts[concept]; ok {
		label = c.Label
	}
	switch action {
	case "baseline":
		return "先做一次摸底，找到合适的起点"
	case "return_to_mainline":
		return "补完先修内容，回到教材主线"
	case "repair_misconception":
		return fmt.Sprintf("先修正「%s」上的误解", label)
	case "retrieval_probe":
		return fmt.Sprintf("先独立回忆一次「%s」", label)
	case "review_due":
		return fmt.Sprintf("复习到期的「%s」", label)
	case "transfer_probe", "application_probe":
		return fmt.Sprintf("把「%s」用到新的场景里", label)
	case "explain_probe":
		return fmt.Sprintf("开始学习「%s」", label)
	default:
		if node != "" {
			return fmt.Sprintf("进入 %s %s", node, nodeTitle)
		}
		return "沿教材继续"
	}
}
