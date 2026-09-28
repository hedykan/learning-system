package projection

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
)

// renderHome renders the Vault's README.md home page.
func renderHome(in Inputs) string {
	m := in.Model
	var b strings.Builder
	b.WriteString(frontmatter("home", "", m.Generation))
	b.WriteString(in.t("# 我的学习\n\n> 本页由 Learning OS 生成，是这个学习库的首页；只有「手写笔记」区域会原样保留。\n\n"))

	if g := in.Git; g != nil {
		last := in.t("从未提交")
		if ts, err := time.Parse(time.RFC3339, g.LastCommit); err == nil {
			last = ts.Local().Format("2006-01-02 15:04")
		}
		b.WriteString(in.t("## ⚠ 学习记录还没有保存到 Git\n\n"))
		fmt.Fprintf(&b, in.t("有 %d 处改动还没有提交（最近一次提交：%s）。学习记录仍在本地，但没有版本保护。\n\n"), g.Uncommitted, last)
		b.WriteString(in.t("常见原因是 Agent 的沙箱禁止写入 `.git`。在沙箱之外的终端里进入本目录，运行 `learn commit` 即可保存。\n\n"))
	}

	b.WriteString(in.t("## 正在学习\n\n"))
	switch {
	case len(in.Library) == 0:
		b.WriteString(in.t("还没有导入教材。在本目录启动 Codex 或 Claude，告诉它教材文件的位置即可导入。\n"))
	case in.Active == "":
		b.WriteString(in.t("当前没有激活的教材。告诉 Agent 你想学哪一本。\n"))
	default:
		pos := in.Positions[in.Active]
		fmt.Fprintf(&b, in.t("- 教材：%s\n"), link(CurriculumIndexFile(in.Active, in.Titles[in.Active]), in.Titles[in.Active]))
		where := strings.Trim(strings.Join([]string{pos.Chapter, pos.Section}, " / "), " /")
		if where == "" {
			where = in.t("尚未设定")
		}
		fmt.Fprintf(&b, in.t("- 学到：%s\n"), where)
		if pos.CurrentConcept != "" {
			fmt.Fprintf(&b, in.t("- 当前概念：%s\n"), pos.CurrentConcept)
		}
		switch in.Outlines[in.Active].Status {
		case "confirmed":
		case "draft":
			b.WriteString(in.t("- 目录：草稿，等待你确认\n"))
		default:
			b.WriteString(in.t("- 目录：尚未建立，学习位置还不能和原书核对\n"))
		}
		if n := in.Next; n != nil {
			fmt.Fprintf(&b, in.t("- 下一步：%s\n"), in.nextSentence(m, n.Action, n.Concept, n.Node, n.NodeTitle))
		}
	}

	fmt.Fprintf(&b, in.t("\n## 今天该复习（%s）\n\n"), in.Today)
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
			fmt.Fprintf(&b, in.t("- %s（%s 到期，第 %d 档）\n"), conceptLink(m, s.Concept), s.Due, s.Level+1)
		}
		b.WriteString(in.t("\n说“继续学习”，会先安排这些复习。\n"))
	case next != nil:
		fmt.Fprintf(&b, in.t("今天没有到期的复习。下一次：%s，%s。\n"), next.Due, conceptLink(m, next.Concept))
	default:
		b.WriteString(in.t("还没有需要复习的概念。\n"))
	}

	if open := m.OpenQuestions(""); len(open) > 0 {
		b.WriteString(in.t("\n## 我提出的问题\n\n"))
		for _, q := range open {
			fmt.Fprintf(&b, "- %s\n", questionLink(m, q))
		}
	}

	b.WriteString(in.t("\n## 我的教材\n\n"))
	if len(in.Library) == 0 {
		b.WriteString(in.t("暂无。\n"))
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
		progress := in.t("尚未建立目录")
		if total > 0 {
			progress = fmt.Sprintf(in.t("完成 %d / %d 节，跳过 %d 节"), done, total, skipped)
		}
		active := ""
		if id == in.Active {
			active = in.t("（正在学习）")
		}
		fmt.Fprintf(&b, in.t("- %s%s：%s · %s\n"), link(CurriculumIndexFile(id, in.Titles[id]), in.Titles[id]), active, progress, link(CurriculumProgressFile(id, in.Lang), in.t("进度")))
	}

	b.WriteString(in.t("\n## 学习者总览\n\n"))
	if m.Empty() {
		b.WriteString(in.t("还没有学习记录。学完第一个概念后，这里会显示各概念的理解状态。\n"))
	} else {
		counts := map[string]int{}
		misconceptions := 0
		for _, c := range m.ConceptList() {
			counts[c.State()]++
			misconceptions += len(m.UnresolvedMisconceptions(c.ID))
		}
		fmt.Fprintf(&b, in.t("- %s\n- 概念：形成中 %d 个，脆弱 %d 个，稳定 %d 个\n- 待修正的误解：%d 个\n"), link(OverviewFile(in.Lang), in.t("查看学习者总览")),
			counts["developing"], counts["fragile"], counts["stable"], misconceptions)
	}

	b.WriteString(in.t("\n## 最近学习\n\n"))
	if len(in.Recent) == 0 {
		b.WriteString(in.t("暂无。\n"))
	}
	kinds := map[string]string{"baseline": in.t("摸底"), "lesson": in.t("学习"), "review": in.t("复习"), "practice": in.t("练习")}
	for _, r := range in.Recent {
		target := "Conversations/" + r.ID
		if r.HasSession {
			target = "Sessions/" + r.ID
		}
		kind := in.t(kinds[r.Kind])
		if kind == "" {
			kind = in.t("学习")
		}
		title := in.Titles[r.Curriculum]
		if title == "" {
			title = r.Curriculum
		}
		fmt.Fprintf(&b, "- [[%s|%s]] %s · %s\n", target, sessionLabel(r.ID), kind, title)
	}

	b.WriteString(in.t("\n## 最近变化的概念\n\n"))
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
		b.WriteString(in.t("暂无。\n"))
	}
	for _, ch := range changes {
		fmt.Fprintf(&b, in.t("- %s → %s（%s）\n"), conceptLink(m, ch.concept.ID), in.state(ch.entry.State), sessionLabel(ch.entry.Session))
	}

	b.WriteString(in.t("\n## 怎么用\n\n"))
	b.WriteString(in.t("- 在这个目录启动 Codex 或 Claude，说“继续学习”。\n"))
	b.WriteString(in.t("- 想学新教材时，告诉它文件路径；它会先和你核对目录。\n"))
	b.WriteString(in.t("- 在任何笔记的「手写笔记」区写下自己的想法，重建时不会被覆盖。\n"))
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

func (in Inputs) nextSentence(m *learner.Model, action, concept, node, nodeTitle string) string {
	label := concept
	if c, ok := m.Concepts[concept]; ok {
		label = c.Label
	}
	switch action {
	case "baseline":
		return in.t("先做一次摸底，找到合适的起点")
	case "return_to_mainline":
		return in.t("补完先修内容，回到教材主线")
	case "repair_misconception":
		return fmt.Sprintf(in.t("先修正「%s」上的误解"), label)
	case "retrieval_probe":
		return fmt.Sprintf(in.t("先独立回忆一次「%s」"), label)
	case "address_question":
		return in.t("先回答你之前提出的问题")
	case "review_due":
		return fmt.Sprintf(in.t("复习到期的「%s」"), label)
	case "transfer_probe", "application_probe":
		return fmt.Sprintf(in.t("把「%s」用到新的场景里"), label)
	case "explain_probe":
		return fmt.Sprintf(in.t("开始学习「%s」"), label)
	default:
		if node != "" {
			return fmt.Sprintf(in.t("进入 %s %s"), node, nodeTitle)
		}
		return in.t("沿教材继续")
	}
}
