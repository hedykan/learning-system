package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A review may interrupt a lesson; the lesson resumes when it ends and the
// two conversations stay apart (CR-2026-049).
func TestReviewInterruptsLesson(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(src, []byte("# 定义非功能性需求\n\n## 描述性能\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", src, "--id", "ddia", "--title", "DDIA", "--activate", "--yes")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("怎么描述性能？", false, "session", "append", "--role", "assistant")
	c.run("不能只看平均值，要看最慢的请求", false, "session", "append", "--role", "user")
	c.run(`{"schema":"learning-os/interpretation@1","curriculum":"ddia","concepts":[{"id":"tail","label":"尾延迟"}],
	  "state_updates":[{"id":"u1","concept":"tail","state":"developing","capabilities":["explained"],"summary":"能解释","evidence":[{"turn":"t0002","quote":"要看最慢的请求"}]}]}`, false, "session", "end", "--analysis-file", "-")

	c.clock = c.clock.Add(48 * time.Hour)
	t.Setenv("LEARN_NOW", c.clock.Format(time.RFC3339))
	lesson := jsonField(c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline", "--json"), "id")
	c.run("接着讲吞吐量。", false, "session", "append", "--role", "assistant")
	if next := c.run("", false, "next", "--json"); !strings.Contains(next, `"rule": "R3b-review-due"`) {
		t.Fatalf("review should be due: %s", next)
	}
	if out := c.run("", true, "session", "start", "--kind", "lesson", "--skip-baseline"); !strings.Contains(out, "only a review (--kind review) can interrupt it") {
		t.Fatalf("second lesson: %s", out)
	}

	started := c.run("", false, "session", "start", "--kind", "review", "--json")
	review := jsonField(started, "id")
	if jsonField(started, "suspended") != lesson {
		t.Fatalf("review start: %s", started)
	}
	if out := c.run("", true, "session", "start", "--kind", "review"); !strings.Contains(out, "it interrupted "+lesson) {
		t.Fatalf("nested review: %s", out)
	}
	status := c.run("", false, "status", "--json")
	if !strings.Contains(status, `"suspended_session": {`) || !strings.Contains(status, `"id": "`+lesson+`"`) || !strings.Contains(status, `"kind": "review"`) {
		t.Fatalf("status during review: %s", status)
	}
	if human := c.run("", false, "status"); !strings.Contains(human, "Suspended Session: "+lesson+" (lesson)") {
		t.Fatalf("human status: %s", human)
	}
	if vaults := c.run("", false, "vaults", filepath.Dir(c.root), "--json"); !strings.Contains(vaults, `"suspended_session": {`) {
		t.Fatalf("vaults: %s", vaults)
	}
	record := `{"schema":"learning-os/interpretation@1","curriculum":"ddia"}`
	if out := c.run(record, true, "session", "annotate", lesson, "--analysis-file", "-"); !strings.Contains(out, "is suspended by review "+review) {
		t.Fatalf("annotate suspended lesson: %s", out)
	}
	c.run("还记得尾延迟吗？", false, "session", "append", "--role", "assistant")
	c.run("要看 p99，最慢那批请求", false, "session", "append", "--role", "user")
	ended := c.run(`{"schema":"learning-os/interpretation@1","curriculum":"ddia","review_results":[{"id":"r1","concept":"tail","outcome":"recalled","action_turn":"t0001","evidence":[{"turn":"t0002","quote":"最慢那批请求"}]}]}`, false, "session", "end", "--analysis-file", "-", "--json")
	if jsonField(ended, "resumed") != lesson {
		t.Fatalf("review end: %s", ended)
	}

	status = c.run("", false, "status", "--json")
	if !strings.Contains(status, `"id": "`+lesson+`"`) || strings.Contains(status, "suspended_session") {
		t.Fatalf("lesson not resumed: %s", status)
	}
	if next := c.run("", false, "next", "--json"); strings.Contains(next, "R3b-review-due") {
		t.Fatalf("review still due after resuming: %s", next)
	}
	if turn := c.run("你说吞吐量是什么？", false, "session", "append", "--role", "user", "--json"); !strings.Contains(turn, `"turn": "t0002"`) {
		t.Fatalf("resumed lesson turn: %s", turn)
	}
	lessonConv := read(t, filepath.Join(c.root, "Conversations", lesson+".md"))
	reviewConv := read(t, filepath.Join(c.root, "Conversations", review+".md"))
	if strings.Contains(lessonConv, "尾延迟") || !strings.Contains(reviewConv, "p99") || strings.Contains(reviewConv, "吞吐量") {
		t.Fatalf("conversations mixed:\nlesson:\n%s\nreview:\n%s", lessonConv, reviewConv)
	}

	// Aborting a review resumes the lesson too.
	c.run("", false, "session", "start", "--kind", "review")
	if aborted := c.run("", false, "session", "abort", "--reason", "left the review page"); !strings.Contains(aborted, "Resumed: "+lesson) {
		t.Fatalf("abort: %s", aborted)
	}
	c.run("", false, "session", "end", "--no-analysis", "--reason", "short lesson")
	if status := c.run("", false, "status", "--json"); !strings.Contains(status, `"active_session": null`) {
		t.Fatalf("after lesson end: %s", status)
	}
}

func TestBaselineCannotBeInterrupted(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "b.md")
	if err := os.WriteFile(src, []byte("# 一\n\n## 甲\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", src, "--id", "b", "--activate", "--yes")
	c.run("", false, "session", "start", "--kind", "baseline")
	if out := c.run("", true, "session", "start", "--kind", "review"); !strings.Contains(out, "a baseline session cannot be interrupted") {
		t.Fatalf("review during baseline: %s", out)
	}
}

func jsonField(out, key string) string {
	_, rest, ok := strings.Cut(out, `"`+key+`": "`)
	if !ok {
		return ""
	}
	value, _, _ := strings.Cut(rest, `"`)
	return value
}
