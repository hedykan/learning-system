package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalCardSetHistoryAndEvidence(t *testing.T) {
	c := goalCurriculum(t)
	if out := c.run(`{"outcome":{"text":"x","evidence":"y"}}`, true, "curriculum", "goal", "set", "--file", "-"); !strings.Contains(out, "start a session") {
		t.Fatalf("goal set without a session: %s", out)
	}
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("除了选型还想做什么？", false, "session", "append", "--role", "assistant")
	c.run("还要支持多租户隔离，每周大概三小时", false, "session", "append", "--role", "user")
	for card, want := range map[string]string{
		`{"focus":[{"id":"tenancy","text":"多租户","evidence":"多租户隔离"}]}`:                                            "needs an outcome",
		`{"outcome":{"text":"能选型","evidence":"我编的一句话"}}`:                                                          "not found in a raw learner turn",
		`{"outcome":{"text":"能选型","evidence":"多租户隔离"},"focus":[{"id":"Bad Id","text":"多租户","evidence":"多租户隔离"}]}`: "kebab-case",
	} {
		if out := c.run(card, true, "curriculum", "goal", "set", "--file", "-"); !strings.Contains(out, want) {
			t.Fatalf("%s: %s", card, out)
		}
	}
	card := `{"outcome":{"text":"能选型并调优，支持多租户","evidence":"还要支持多租户隔离"},
	  "constraints":{"text":"每周三小时","evidence":"每周大概三小时"},
	  "focus":[{"id":"ann-index","text":"近似最近邻索引","evidence":"多租户隔离"},{"id":"multi-tenancy","text":"多租户隔离","evidence":"多租户隔离"}]}`
	c.run(card, false, "curriculum", "goal", "set", "--file", "-")
	if _, err := os.Stat(filepath.Join(c.root, "Curriculum", "vdb", "goal-history", "0001.yaml")); err != nil {
		t.Fatal("the intake goal card was not kept in history")
	}
	page := read(t, filepath.Join(c.root, "Curriculum", "vdb", "向量数据库入门.md"))
	for _, want := range []string{"## 学习目标", "- 要达成：能选型并调优，支持多租户（「还要支持多租户隔离」）", "- 时间与深度：每周三小时", "- 关注点 `multi-tenancy`：多租户隔离"} {
		if !strings.Contains(page, want) {
			t.Fatalf("goal section lacks %q:\n%s", want, page)
		}
	}
	if !strings.Contains(read(t, filepath.Join(c.root, "README.md")), "- 目标：能选型并调优，支持多租户") {
		t.Fatal("home lacks the goal outcome")
	}
}

func TestOutlineReviewCoverageAndReasons(t *testing.T) {
	c := goalCurriculum(t)
	if out := c.run(`{"nodes":[{"id":"1","title":"a","why":"b","serves":["nope"]}]}`, true, "curriculum", "outline", "set", "--file", "-"); !strings.Contains(out, "not a focus of the goal card") {
		t.Fatalf("unknown serves: %s", out)
	}
	draft := `{"nodes":[{"id":"1","title":"向量与相似度","concepts":["embedding"]},{"id":"2","title":"HNSW","why":"主流索引","concepts":["hnsw"]}]}`
	c.run(draft, false, "curriculum", "outline", "set", "--file", "-")
	if out := c.run("", true, "curriculum", "outline", "confirm"); !strings.Contains(out, "entry 1 has no why") || !strings.Contains(out, "no entry serves focus ann-index") {
		t.Fatalf("confirm gate: %s", out)
	}
	review := c.run("", false, "curriculum", "outline", "review", "--json")
	for _, want := range []string{`"ready_to_confirm": false`, `"id": "ann-index"`, `"finding": "existing_knowledge"`, `"likely_known": [
    "1"
  ]`} {
		if !strings.Contains(review, want) {
			t.Fatalf("review lacks %s:\n%s", want, review)
		}
	}
	fixed := strings.Replace(strings.Replace(draft, `"title":"向量与相似度",`, `"title":"向量与相似度","why":"一切的基础",`, 1), `"why":"主流索引",`, `"why":"主流索引","serves":["ann-index"],`, 1)
	c.run(fixed, false, "curriculum", "outline", "set", "--file", "-")
	if review := c.run("", false, "curriculum", "outline", "review", "--json"); !strings.Contains(review, `"ready_to_confirm": true`) {
		t.Fatalf("fixed draft not ready: %s", review)
	}
	c.run("", false, "curriculum", "outline", "confirm")

	c.run("", false, "source", "add", "--external", "--id", "hnsw-paper", "--title", "HNSW 论文")
	long := strings.Repeat("理", 121)
	if out := c.run("", true, "source", "attach", "2", "hnsw-paper", "text", "第 4 节", "--why", long); !strings.Contains(out, "at most 120") {
		t.Fatalf("long why: %s", out)
	}
	if out := c.run("", true, "source", "attach", "2", "hnsw-paper", "text", "第 4 节", "--serves", "nope"); !strings.Contains(out, "not a focus") {
		t.Fatalf("unknown serves on attach: %s", out)
	}
	c.run("", false, "source", "attach", "2", "hnsw-paper", "text", "第 4 节", "--why", "原始论文，参数含义讲得最清楚", "--serves", "ann-index")
	if review := c.run("", false, "curriculum", "outline", "review", "--json"); !strings.Contains(review, `"why": "原始论文，参数含义讲得最清楚"`) {
		t.Fatalf("review lacks the source reason: %s", review)
	}
	page := read(t, filepath.Join(c.root, "Curriculum", "vdb", "向量数据库入门.md"))
	if !strings.Contains(page, "资料：HNSW 论文 第 4 节——原始论文，参数含义讲得最清楚") || !strings.Contains(page, "  - 服务于：近似最近邻索引") {
		t.Fatalf("curriculum page:\n%s", page)
	}
}

func TestIntakeHintsForATextbook(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "book.md")
	os.WriteFile(src, []byte("# 复制\n\n## 单主\n\n## 多主\n"), 0o644)
	c.run("", false, "curriculum", "import", src, "--id", "book", "--title", "书", "--activate", "--yes")
	c.run(`{"nodes":[{"id":"1","title":"复制"},{"id":"1.1","title":"单主","concepts":["leader-follower"]},{"id":"1.2","title":"多主","concepts":["multi-leader"]}]}`, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "session", "start", "--kind", "baseline", "--depth", "quick")
	c.run("主从复制熟悉吗？", false, "session", "append", "--role", "assistant")
	c.run("主从复制我们线上一直在用，多主没碰过", false, "session", "append", "--role", "user")
	c.run(`{"curriculum":"book","depth":"quick","learning_goals":[],
	  "existing_knowledge":[{"claim":"熟悉单主复制","evidence":"主从复制我们线上一直在用","concepts":["leader-follower"]}],
	  "prerequisite_gaps":[{"claim":"没接触多主","evidence":"多主没碰过","concepts":["multi-leader"]}],"possible_misconceptions":[],
	  "familiar_vocabulary":[],"unknown_vocabulary":[],
	  "recommended_entry":{"book":"book","node":"1.2","chapter":"","section":"","current_concept":"","last_completed":"","next_textbook_step":"","detour":null},
	  "recommendation_reason":"单主已熟悉","next_probe":"多主冲突"}`, false, "session", "end", "--assessment-file", "-")
	review := c.run("", false, "curriculum", "outline", "review", "--json")
	if !strings.Contains(review, `"likely_known": [
    "1.1"
  ]`) || !strings.Contains(review, `"finding": "prerequisite_gap"`) {
		t.Fatalf("textbook hints: %s", review)
	}
	if outline := c.run("", false, "curriculum", "outline", "show", "--json"); strings.Contains(outline, `"status": "skipped"`) || strings.Contains(outline, `"status": "completed"`) {
		t.Fatalf("hints must not change the outline: %s", outline)
	}
}
