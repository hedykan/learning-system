package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// learnSecondBook studies a CDN book whose concept needs max-age first.
func (c *cli) learnSecondBook(rel string, wantError bool) string {
	c.t.Helper()
	c.startSecondBook()
	return c.submitEdge(rel, wantError)
}

func (c *cli) startSecondBook() {
	c.t.Helper()
	src := filepath.Join(c.t.TempDir(), "cdn.md")
	if err := os.WriteFile(src, []byte("# CDN basics\n\n## Edge caches\n"), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", src, "--id", "cdn", "--title", "CDN basics", "--activate", "--yes")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("Where does an edge node get its copy?", false, "session", "append", "--role", "assistant")
	c.run("From the origin, and it keeps it fresh for max-age like a browser", false, "session", "append", "--role", "user")
}

func (c *cli) submitEdge(rel string, wantError bool) string {
	c.t.Helper()
	rec := `{"schema":"learning-os/interpretation@1","curriculum":"cdn",
	  "concepts":[{"id":"edge","label":"Edge cache","related":[` + rel + `]}],
	  "state_updates":[{"id":"u1","concept":"edge","state":"developing","capabilities":["explained"],"summary":"links edge caching to freshness","evidence":[{"turn":"t0002","quote":"keeps it fresh for max-age like a browser"}]}]}`
	return c.run(rec, wantError, "session", "checkpoint", "--analysis-file", "-")
}

func TestRelationTypesAndCrossCurriculumLinks(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.learnEnglish()
	c.startSecondBook()
	if out := c.submitEdge(`{"concept":"max-age","type":"depends_on"}`, true); !strings.Contains(out, "unsupported type") {
		t.Fatalf("invalid relation type: %s", out)
	}
	c.submitEdge(`{"concept":"max-age","type":"prerequisite","note":"reuses freshness"}`, false)

	edge := read(t, filepath.Join(c.root, "Concepts", "Edge cache.md"))
	for _, want := range []string{"  - learning/cross-curriculum\n", "## 与其他教材的联系", "- 先修：[[Concepts/Freshness lifetime|Freshness lifetime]]（HTTP caching）：reuses freshness"} {
		if !strings.Contains(edge, want) {
			t.Fatalf("edge note missing %q:\n%s", want, edge)
		}
	}
	fresh := read(t, filepath.Join(c.root, "Concepts", "Freshness lifetime.md"))
	if strings.Contains(fresh, "[[Concepts/Edge cache") {
		t.Fatal("incoming prerequisite must be plain text, not a link")
	}
	if !strings.Contains(fresh, "被这些概念引用：Edge cache（CDN basics）（以它为先修）") {
		t.Fatalf("incoming prerequisite not listed:\n%s", fresh)
	}
	if !strings.Contains(fresh, "- 相关：[[Concepts/Validation with ETag|Validation with ETag]]") {
		t.Fatalf("same-book relation missing:\n%s", fresh)
	}

	// edge needs max-age; max-age → origin → edge closes a cycle.
	cycle := `{"schema":"learning-os/interpretation@1","curriculum":"cdn","concepts":[
	  {"id":"origin","label":"Origin fetch","related":[{"concept":"edge","type":"prerequisite"}]},
	  {"id":"max-age","related":[{"concept":"origin","type":"prerequisite"}]}]}`
	if out := c.run(cycle, true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, "cycle") {
		t.Fatalf("prerequisite cycle accepted: %s", out)
	}
}

func TestKnowledgeNotesLinkEvidenceOncePerSession(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.learnEnglish()
	c.learnSecondBook(`{"concept":"max-age","type":"prerequisite"}`, false)
	for _, rel := range []string{"Concepts/Freshness lifetime.md", "Concepts/Edge cache.md", "Questions/Why not ask the server every time.md"} {
		body := read(t, filepath.Join(c.root, rel))
		for _, banned := range []string{"[[Sessions/", "[[Curriculum/"} {
			if strings.Contains(body, banned) {
				t.Fatalf("%s links to %s:\n%s", rel, banned, body)
			}
		}
		if n := strings.Count(body, "[[Conversations/"); n == 0 || n > 2 {
			t.Fatalf("%s has %d conversation links, want one per session", rel, n)
		}
	}
	fresh := read(t, filepath.Join(c.root, "Concepts", "Freshness lifetime.md"))
	if !strings.Contains(fresh, "  - 「It deletes the file after an hour」（t0002）") {
		t.Fatalf("quotes not grouped under their session:\n%s", fresh)
	}
}

func TestLayerTagsGraphConfigAndConversationMigration(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	graph := read(t, filepath.Join(c.root, ".obsidian", "graph.json"))
	for _, want := range []string{`"showTags": false`, `"search": "-tag:#learning/evidence -tag:#learning/process -tag:#learning/nav -path:Sources"`, `tag:#learning/state/fragile`} {
		if !strings.Contains(graph, want) {
			t.Fatalf("graph config missing %s:\n%s", want, graph)
		}
	}
	c.learnEnglish()
	want := map[string]string{
		"README.md":                                      "learning/nav/home",
		"Profile/学习者总览.md":                               "learning/nav/overview",
		"Curriculum/caching/HTTP caching.md":             "learning/nav/curriculum",
		"Curriculum/caching/学习进度.md":                     "learning/process/progress",
		"Concepts/Freshness lifetime.md":                 "learning/knowledge/concept",
		"Questions/Why not ask the server every time.md": "learning/knowledge/question",
	}
	for rel, tag := range want {
		if !strings.Contains(read(t, filepath.Join(c.root, rel)), "  - "+tag+"\n") {
			t.Fatalf("%s lacks tag %s", rel, tag)
		}
	}
	sessions, _ := filepath.Glob(filepath.Join(c.root, "Sessions", "*.md"))
	convs, _ := filepath.Glob(filepath.Join(c.root, "Conversations", "*.md"))
	if len(sessions) != 1 || len(convs) != 1 {
		t.Fatalf("sessions=%v conversations=%v", sessions, convs)
	}
	if !strings.Contains(read(t, sessions[0]), "  - learning/process/session\n") {
		t.Fatal("session file lacks tag")
	}

	// A v0.1.7 conversation has no tag: the upgrade adds it and nothing else.
	conv := read(t, convs[0])
	old := strings.Replace(conv, "tags:\n  - learning/evidence/conversation\n", "", 1)
	if old == conv {
		t.Fatal("new conversation should be tagged at creation")
	}
	if err := os.WriteFile(convs[0], []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := `{"search":"my own filter"}`
	if err := os.WriteFile(filepath.Join(c.root, ".obsidian", "graph.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	dry := c.run("", false, "agent", "update", "--dry-run", "--json")
	if !strings.Contains(dry, `"action": "add-tag"`) || strings.Contains(dry, "graph.json") {
		t.Fatalf("dry run: %s", dry)
	}
	if read(t, convs[0]) != old {
		t.Fatal("dry run changed the conversation")
	}
	c.run("", false, "agent", "update", "--yes", "--json")
	if read(t, convs[0]) != conv {
		t.Fatalf("migration must only add the tag line:\n%s", read(t, convs[0]))
	}
	if read(t, filepath.Join(c.root, ".obsidian", "graph.json")) != custom {
		t.Fatal("existing graph settings were overwritten")
	}
	c.run("", false, "config", "set", "language", "en")
	if !strings.Contains(read(t, filepath.Join(c.root, "Profile", "Learner overview.md")), "  - learning/nav/overview\n") {
		t.Fatal("tags must not change with the interface language")
	}
}

func TestNextReportsLearningStage(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "caching.md")
	if err := os.WriteFile(src, []byte("# HTTP caching\n\n## Freshness\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", src, "--id", "caching", "--title", "HTTP caching", "--activate", "--yes")
	if next := c.run("", false, "next", "--json"); !strings.Contains(next, `"stage": "collect"`) {
		t.Fatalf("draft outline should be the collect stage: %s", next)
	}
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	if next := c.run("", false, "next", "--json"); !strings.Contains(next, `"stage": "learn"`) {
		t.Fatalf("confirmed outline should be the learn stage: %s", next)
	}
	home := read(t, filepath.Join(c.root, "README.md"))
	for _, h := range []string{"## ① 资料", "## ② 学习", "## ③ 巩固", "### 今天该复习（"} {
		if !strings.Contains(home, h) {
			t.Fatalf("home lacks %q:\n%s", h, home)
		}
	}
}
