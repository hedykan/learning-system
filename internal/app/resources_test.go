package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourcesAttachAndShare(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	dir := filepath.Join(t.TempDir(), "book")
	for name, text := range map[string]string{"ch10.md": "# Ten\n\n## Late\n", "ch2.md": "# Two\n\n## Replication lag\n\nFollowers fall behind.\n", "ch1.md": "# One\n"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c.run("", false, "curriculum", "import", dir, "--id", "ddia", "--title", "DDIA", "--activate", "--yes")
	outline := c.run("", false, "curriculum", "outline", "show", "--json")
	if strings.Index(outline, `"Two"`) > strings.Index(outline, `"Ten"`) {
		t.Fatalf("draft outline not in natural order: %s", outline)
	}
	byAnchor := c.run("", false, "source", "read", "ddia", "anchor", "#replication-lag", "--json")
	if !strings.Contains(byAnchor, "Followers fall behind.") || strings.Contains(byAnchor, "Late") {
		t.Fatalf("read by anchor: %s", byAnchor)
	}
	if out := c.run("", true, "source", "read", "ddia", "page", "3", "--json"); !strings.Contains(out, "anchor or file") {
		t.Fatalf("page locator on markdown: %s", out)
	}

	c.run("", false, "source", "add", "--external", "--id", "kleppmann-talks", "--title", "Kleppmann lectures", "--url", "https://example.org/talks")
	if list := c.run("", false, "curriculum", "list", "--json"); strings.Contains(list, "kleppmann-talks") {
		t.Fatalf("a resource must not appear as a curriculum: %s", list)
	}
	if out := c.run("", true, "source", "attach", "2.1", "kleppmann-talks", "time", "4/10:00-20:00"); !strings.Contains(out, "confirm the outline") {
		t.Fatalf("attach before confirming: %s", out)
	}
	c.run("", false, "curriculum", "outline", "confirm")
	if out := c.run("", true, "source", "attach", "9.9", "kleppmann-talks", "time", "4/10:00"); !strings.Contains(out, "no entry") {
		t.Fatalf("attach to missing entry: %s", out)
	}
	if out := c.run("", true, "source", "attach", "2.1", "kleppmann-talks", "anchor", "#x"); !strings.Contains(out, "external resources use") {
		t.Fatalf("anchor on external resource: %s", out)
	}
	c.run("", false, "source", "attach", "2.1", "kleppmann-talks", "time", "4/10:00-20:00")
	c.run("", false, "source", "attach", "2.1", "ddia", "anchor", "#replication-lag")
	c.run("", false, "curriculum", "position", "set", "--node", "2.1")
	next := c.run("", false, "next", "--json")
	for _, want := range []string{`"resource": "kleppmann-talks"`, `"label": "第 4 讲 10:00–20:00"`, `"resource": "ddia"`} {
		if !strings.Contains(next, want) {
			t.Fatalf("next lacks %s: %s", want, next)
		}
	}
	if status := c.run("", false, "status", "--json"); !strings.Contains(status, `"node_resources": [`) || !strings.Contains(status, "kleppmann-talks") {
		t.Fatalf("status node resources: %s", status)
	}
	home := read(t, filepath.Join(c.root, "Curriculum", "ddia", "DDIA.md"))
	if !strings.Contains(home, "  - 资料：Kleppmann lectures 第 4 讲 10:00–20:00") || !strings.Contains(home, "## 资料") {
		t.Fatalf("curriculum page lacks resources:\n%s", home)
	}
	if check := c.run("", false, "source", "check", "--json"); !strings.Contains(check, `"invalid": []`) {
		t.Fatalf("check: %s", check)
	}

	// A second curriculum shares the lecture resource; removing the first keeps it.
	c.run("", false, "curriculum", "import", "--external", "--id", "talks-only", "--title", "Talks", "--yes")
	c.run(`{"nodes":[{"id":"1","title":"Replication","locator":{"kind":"text","value":"lecture 4"}}]}`, false, "curriculum", "outline", "set", "talks-only", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm", "talks-only")
	c.run("", false, "source", "attach", "1", "kleppmann-talks", "time", "4/00:00-50:00", "--curriculum", "talks-only")
	c.run("", false, "curriculum", "remove", "ddia", "--reason", "switching to talks", "--yes")
	if check := c.run("", false, "source", "check", "talks-only", "--json"); !strings.Contains(check, `"invalid": []`) {
		t.Fatalf("shared resource broken after removing ddia: %s", check)
	}
	if out := c.run("", false, "source", "read", "talks-only", "text", "lecture 4", "--json"); !strings.Contains(out, `"status": "unsupported"`) || !strings.Contains(out, "ask the learner") {
		t.Fatalf("external read: %s", out)
	}
}

func TestExternalCurriculumReportsUnsourcedEntries(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	if out := c.run("", true, "curriculum", "import", "--external", "--id", "paper"); !strings.Contains(out, "needs --title") {
		t.Fatalf("external without title: %s", out)
	}
	c.run("", false, "curriculum", "import", "--external", "--id", "paper", "--title", "纸质书", "--note", "高等教育出版社", "--activate", "--yes")
	c.run(`{"nodes":[{"id":"1","title":"实数","locator":{"kind":"page","value":"1-30"}},{"id":"2","title":"极限"}]}`, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	check := c.run("", false, "source", "check", "--json")
	if !strings.Contains(check, `"unsourced": [
    "2"
  ]`) {
		t.Fatalf("check: %s", check)
	}
	if out := c.run(`{"nodes":[{"id":"1","title":"实数","locator":{"kind":"anchor","value":"#x"}}]}`, true, "curriculum", "outline", "set", "--file", "-"); !strings.Contains(out, "external resources use") {
		t.Fatalf("anchor on external outline: %s", out)
	}
}
