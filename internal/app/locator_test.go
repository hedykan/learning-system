package app_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeLocatorsOnOutlineAndTextbookPoints(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.run("", false, "curriculum", "import", "--external", "--id", "analysis", "--title", "数学分析（陈纪修）", "--url", "https://example.org/playlist", "--activate", "--yes")
	bad := `{"nodes":[{"id":"1","title":"数列极限","locator":{"kind":"time","value":"3/05:20-48:00"}},{"id":"1.1","title":"ε-N 定义","locator":{"kind":"time","value":"3/47:00-52:00"}}]}`
	if out := c.run(bad, true, "curriculum", "outline", "set", "--file", "-"); !strings.Contains(out, "falls outside parent 1") {
		t.Fatalf("child outside parent accepted: %s", out)
	}
	outline := `{"nodes":[{"id":"1","title":"数列极限","locator":{"kind":"time","value":"3/05:20-48:00"}},{"id":"1.1","title":"ε-N 定义","locator":{"kind":"time","value":"3/05:20-20:00"}}]}`
	c.run(outline, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("什么叫数列收敛？", false, "session", "append", "--role", "assistant")
	c.run("任给 ε，总能找到 N，n 大于 N 以后都落在 ε 邻域里", false, "session", "append", "--role", "user")
	rec := func(value string) string {
		return `{"schema":"learning-os/interpretation@1","curriculum":"analysis",
		  "concepts":[{"id":"limit","label":"数列极限","source_ref":{"node":"1.1"},"textbook_points":{"locator":{"kind":"time","value":"` + value + `"},"points":["N 依赖于 ε 的选取"]}}],
		  "state_updates":[{"id":"u1","concept":"limit","state":"developing","capabilities":["explained"],"summary":"能说出 ε-N","evidence":[{"turn":"t0002","quote":"任给 ε，总能找到 N"}]}]}`
	}
	if out := c.run(rec("3/30:00-35:00"), true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, "falls outside outline entry 1.1") {
		t.Fatalf("points outside the entry accepted: %s", out)
	}
	c.run(rec("3/10:00-12:00"), false, "session", "checkpoint", "--analysis-file", "-")
	note := read(t, filepath.Join(c.root, "Concepts", "数列极限.md"))
	if !strings.Contains(note, "> AI 根据第 3 讲 10:00–12:00 概括") {
		t.Fatalf("time locator not rendered:\n%s", note)
	}
}
