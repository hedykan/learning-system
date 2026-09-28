package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goalOutline = `{"nodes":[
 {"id":"1","title":"向量与相似度","why":"检索的基础是把内容变成向量","concepts":["embedding"]},
  {"id":"2","title":"近似最近邻索引","why":"精确搜索太慢","prerequisites":["1"],"serves":["ann-index"]},
 {"id":"2.1","title":"HNSW","why":"最常用的图索引","prerequisites":["1"],"serves":["ann-index"]},
 {"id":"3","title":"混合检索","why":"结合关键词与向量","prerequisites":["2"]}]}`

// intakeAssessment ends an intake baseline with a goal card (CR-2026-043/044).
const intakeAssessment = `{"curriculum":"vdb","depth":"quick",
 "learning_goals":[{"claim":"能自己选型并调优","evidence":"想能自己选型并调优向量检索"}],
 "existing_knowledge":[{"claim":"熟悉余弦相似度","evidence":"余弦相似度我在推荐系统里用过","concepts":["embedding"]}],
 "prerequisite_gaps":[],"possible_misconceptions":[],"familiar_vocabulary":[],"unknown_vocabulary":[],
 "recommended_entry":{"book":"","chapter":"","section":"","current_concept":"","last_completed":"","next_textbook_step":"","detour":null},
 "recommendation_reason":"从索引开始","next_probe":"问 HNSW 参数",
 "goal_card":{"outcome":{"text":"能自己选型并调优向量检索服务","evidence":"想能自己选型并调优向量检索"},
   "focus":[{"id":"ann-index","text":"近似最近邻索引","evidence":"最想搞懂索引"}]}}`

func goalCurriculum(t *testing.T) *cli {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	if out := c.run("", true, "curriculum", "import", "--goal", "搞懂向量数据库", "--id", "vdb"); !strings.Contains(out, "needs --title") {
		t.Fatalf("goal without title: %s", out)
	}
	c.run("", false, "curriculum", "import", "--goal", "搞懂向量数据库", "--id", "vdb", "--title", "向量数据库入门", "--activate", "--yes")
	if out := c.run(goalOutline, true, "curriculum", "outline", "set", "--file", "-"); !strings.Contains(out, "no goal card yet") {
		t.Fatalf("outline before intake: %s", out)
	}
	// Intake before the outline: interview and diagnosis in one baseline.
	c.run("", false, "session", "start", "--kind", "baseline", "--depth", "quick")
	c.run("你想达到什么程度？", false, "session", "append", "--role", "assistant")
	c.run("想能自己选型并调优向量检索，最想搞懂索引。余弦相似度我在推荐系统里用过", false, "session", "append", "--role", "user")
	c.run(intakeAssessment, false, "session", "end", "--assessment-file", "-")
	if goal := c.run("", false, "curriculum", "goal", "show", "--json"); !strings.Contains(goal, `"source": "assessment"`) || !strings.Contains(goal, "ann-index") {
		t.Fatalf("goal card from the intake: %s", goal)
	}
	return c
}

func TestGoalCurriculumTopicsAndPrerequisites(t *testing.T) {
	c := goalCurriculum(t)
	if out := c.run("", false, "curriculum", "outline", "show", "--json"); !strings.Contains(out, `"type": "synthesized"`) {
		t.Fatalf("goal curriculum type: %s", out)
	}
	bad := map[string]string{
		`{"nodes":[{"id":"1","title":"a","prerequisites":["9"]}]}`:                                              "not in the outline",
		`{"nodes":[{"id":"1","title":"a"},{"id":"1.1","title":"b","prerequisites":["1"]}]}`:                     "parent or its child",
		`{"nodes":[{"id":"1","title":"a","prerequisites":["2"]},{"id":"2","title":"b","prerequisites":["1"]}]}`: "cycle",
		`{"nodes":[{"id":"1","title":"a","concepts":["Not Kebab"]}]}`:                                           "kebab-case",
		`{"nodes":[{"id":"1","title":"a","locator":{"kind":"page","value":"3"}}]}`:                              "goal resources use text",
	}
	for outline, want := range bad {
		if out := c.run(outline, true, "curriculum", "outline", "set", "--file", "-"); !strings.Contains(out, want) {
			t.Fatalf("%s: %s", outline, out)
		}
	}
	c.run(goalOutline, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	check := c.run("", false, "source", "check", "--json")
	for _, n := range []string{`"1"`, `"2.1"`, `"3"`} {
		if !strings.Contains(check, n) {
			t.Fatalf("unsourced entries: %s", check)
		}
	}
	c.run("", false, "curriculum", "position", "set", "--node", "1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	next := c.run("", false, "next", "--json")
	if !strings.Contains(next, `"node": "1"`) || !strings.Contains(next, `"unsourced": true`) {
		t.Fatalf("next on an unsourced entry: %s", next)
	}
	home := read(t, filepath.Join(c.root, "README.md"))
	if !strings.Contains(home, "3 个小节还没有原始资料") {
		t.Fatalf("home unsourced count:\n%s", home)
	}
	page := read(t, filepath.Join(c.root, "Curriculum", "vdb", "向量数据库入门.md"))
	for _, want := range []string{"由 Agent 根据学习目标组织", "向量与相似度（AI 综合，无原始资料）", "  - 为什么学：精确搜索太慢", "  - 先修：1"} {
		if !strings.Contains(page, want) {
			t.Fatalf("curriculum page lacks %q:\n%s", want, page)
		}
	}

	// 2.1 waits for 1; skipping 1 unblocks it. With 1 skipped, next is 2.1.
	c.run("", false, "curriculum", "position", "set", "--node", "2.1")
	if next := c.run("", false, "next", "--json"); !strings.Contains(next, `"blocked_by": [`) {
		t.Fatalf("blocked entry: %s", next)
	}
	c.run("", false, "curriculum", "skip", "1", "--reason", "已经会了")
	if next := c.run("", false, "next", "--json"); strings.Contains(next, "blocked_by") || !strings.Contains(next, `"node": "2.1"`) {
		t.Fatalf("unblocked: %s", next)
	}

	c.run("", false, "source", "add", "--external", "--id", "hnsw-paper", "--title", "HNSW 论文")
	c.run("", false, "source", "attach", "2.1", "hnsw-paper", "text", "Malkov & Yashunin 2016, 第 4 节")
	if check := c.run("", false, "source", "check", "--json"); strings.Contains(check, `"2.1"`) {
		t.Fatalf("attached entry still unsourced: %s", check)
	}
	c.run("为什么要分层？", false, "session", "append", "--role", "assistant")
	c.run("上层稀疏先快速跳到附近，下层再细找", false, "session", "append", "--role", "user")
	points := func(node string) string {
		return `{"schema":"learning-os/interpretation@1","curriculum":"vdb","concepts":[{"id":"hnsw","label":"HNSW 分层图","source_ref":{"node":"` + node + `"},
		  "textbook_points":{"locator":{"resource":"hnsw-paper","kind":"text","value":"第 4 节"},"points":["上层稀疏用于快速定位"]}}],
		  "state_updates":[{"id":"u1","concept":"hnsw","state":"developing","capabilities":["explained"],"summary":"能说出分层作用","evidence":[{"turn":"t0002","quote":"上层稀疏先快速跳到附近"}]}]}`
	}
	if out := c.run(points("3"), true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, "no source material") {
		t.Fatalf("points on an unsourced entry: %s", out)
	}
	c.run(points("2.1"), false, "session", "checkpoint", "--analysis-file", "-")
}

func TestCurriculumProposals(t *testing.T) {
	c := goalCurriculum(t)
	c.run(goalOutline, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("你用过向量检索吗？", false, "session", "append", "--role", "assistant")
	c.run("做过语义搜索，余弦相似度很熟了，不过我想先弄懂量化压缩", false, "session", "append", "--role", "user")
	rec := func(proposals string) string {
		return `{"schema":"learning-os/interpretation@1","curriculum":"vdb","curriculum_proposals":[` + proposals + `]}`
	}
	ev := `"evidence":[{"turn":"t0002","quote":"余弦相似度很熟了"}]`
	for p, want := range map[string]string{
		`{"id":"p0","action":"mark_known","node":"1","reason":"学习者说很熟了","evidence":[]}`:       "learner's words",
		`{"id":"p0","action":"rename","node":"1","reason":"改名字",` + ev + `}`:                  "action must be",
		`{"id":"p0","action":"remove","node":"9","reason":"没有这一节",` + ev + `}`:                "no entry 9",
		`{"id":"p0","action":"insert","node":"2","title":"量化","reason":"学习者想学量化",` + ev + `}`: "already exists",
		`{"id":"p0","action":"remove","node":"2","reason":"想删掉有子条目的",` + ev + `}`:             "sub-entries",
	} {
		if out := c.run(rec(p), true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, want) {
			t.Fatalf("%s: %s", p, out)
		}
	}
	c.run(rec(`{"id":"p1","action":"mark_known","node":"1","reason":"学习者已熟悉余弦相似度",`+ev+`},
	  {"id":"p2","action":"insert","node":"2.2","title":"量化压缩","why":"学习者想先弄懂内存占用","prerequisites":["1"],"reason":"学习者主动提出",`+ev+`},
	  {"id":"p3","action":"retitle","node":"3","title":"混合检索与重排","reason":"与学习者目标一致",`+ev+`}`), false, "session", "checkpoint", "--analysis-file", "-")
	if next := c.run("", false, "next", "--json"); !strings.Contains(next, `"pending_proposals": [`) || !strings.Contains(next, `"p2"`) {
		t.Fatalf("next pending proposals: %s", next)
	}
	if !strings.Contains(read(t, filepath.Join(c.root, "README.md")), "3 条课程调整建议等你决定") {
		t.Fatal("home does not mention pending proposals")
	}
	if outline := c.run("", false, "curriculum", "outline", "show", "--json"); strings.Contains(outline, "量化压缩") {
		t.Fatal("a proposal changed the outline before the learner agreed")
	}
	c.run("", false, "curriculum", "accept", "p2")
	c.run("", false, "curriculum", "accept", "p1")
	c.run("", false, "curriculum", "reject", "p3", "--reason", "学习者想保留原名")
	if out := c.run("", true, "curriculum", "accept", "p3"); !strings.Contains(out, "already rejected") {
		t.Fatalf("decided twice: %s", out)
	}
	outline := c.run("", false, "curriculum", "outline", "show", "--json")
	if !strings.Contains(outline, "量化压缩") || strings.Contains(outline, "混合检索与重排") || !strings.Contains(outline, `"status": "confirmed"`) {
		t.Fatalf("outline after decisions: %s", outline)
	}
	if !strings.Contains(outline, `"status": "completed"`) {
		t.Fatalf("mark_known should complete entry 1: %s", outline)
	}
	history := c.run("", false, "curriculum", "outline", "history", "--json")
	if !strings.Contains(history, `"proposal": "p2"`) || strings.Contains(history, "量化压缩") {
		t.Fatalf("history must keep the outline before p2: %s", history)
	}
	if list := c.run("", false, "curriculum", "proposals", "--json"); strings.Contains(list, `"p1"`) || strings.Contains(list, `"p3"`) {
		t.Fatalf("decided proposals listed as pending: %s", list)
	}
	if list := c.run("", false, "curriculum", "proposals", "--all", "--json"); !strings.Contains(list, `"decision": "rejected"`) {
		t.Fatalf("all proposals: %s", list)
	}
	// Resubmitting the same record after decisions is fine.
	c.run(rec(`{"id":"p1","action":"mark_known","node":"1","reason":"学习者已熟悉余弦相似度",`+ev+`},
	  {"id":"p2","action":"insert","node":"2.2","title":"量化压缩","why":"学习者想先弄懂内存占用","prerequisites":["1"],"reason":"学习者主动提出",`+ev+`},
	  {"id":"p3","action":"retitle","node":"3","title":"混合检索与重排","reason":"与学习者目标一致",`+ev+`}`), false, "session", "checkpoint", "--analysis-file", "-")
}

func TestSourceAlignedAllowsOnlySkipAndKnown(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "book.md")
	os.WriteFile(src, []byte("# 复制\n\n## 单主\n\n## 多主\n"), 0o644)
	c.run("", false, "curriculum", "import", src, "--id", "book", "--title", "书", "--activate", "--yes")
	c.run("", false, "curriculum", "outline", "confirm")
	if out := c.run("", false, "curriculum", "outline", "show", "--json"); strings.Contains(out, "synthesized") {
		t.Fatalf("a book must stay source_aligned: %s", out)
	}
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("熟悉单主复制吗？", false, "session", "append", "--role", "assistant")
	c.run("单主复制我在工作里天天用", false, "session", "append", "--role", "user")
	insert := `{"schema":"learning-os/interpretation@1","curriculum":"book","curriculum_proposals":[{"id":"q1","action":"insert","node":"1.3","title":"CRDT","reason":"学习者想加一节","evidence":[{"turn":"t0002","quote":"单主复制我在工作里天天用"}]}]}`
	if out := c.run(insert, true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, "only skip and mark_known") {
		t.Fatalf("insert on a book: %s", out)
	}
	known := strings.Replace(strings.Replace(insert, `"insert","node":"1.3","title":"CRDT"`, `"mark_known","node":"1.1"`, 1), "学习者想加一节", "学习者工作中常用", 1)
	c.run(known, false, "session", "checkpoint", "--analysis-file", "-")
	c.run("", false, "curriculum", "accept", "q1")
	if st := c.run("", false, "status", "--json"); !strings.Contains(st, `"node": "1.2"`) {
		t.Fatalf("accepting mark_known on the current entry should advance: %s", st)
	}
}
