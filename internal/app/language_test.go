package app_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/app"
)

type cli struct {
	t     *testing.T
	root  string
	clock time.Time
}

func newCLI(t *testing.T) *cli {
	c := &cli{t: t, root: filepath.Join(t.TempDir(), "vault"), clock: time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)}
	t.Setenv("LEARN_NOW", c.clock.Format(time.RFC3339))
	// Vault auto commits need a Git identity; CI machines have none.
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "Learning OS test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.invalid")
	}
	return c
}

func (c *cli) run(stdin string, wantError bool, args ...string) string {
	c.t.Helper()
	c.clock = c.clock.Add(time.Second)
	var out bytes.Buffer
	a := app.New()
	now := c.clock
	a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return now }
	cmd := a.RootCommand()
	cmd.SetArgs(append([]string{"--vault", c.root}, args...))
	err := cmd.ExecuteContext(context.Background())
	if wantError != (err != nil) {
		c.t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
	}
	if err != nil {
		return err.Error()
	}
	return out.String()
}

// learnEnglish records one English lesson with every kind of generated page.
func (c *cli) learnEnglish() {
	c.t.Helper()
	src := filepath.Join(c.t.TempDir(), "caching.md")
	if err := os.WriteFile(src, []byte("# HTTP caching\n\n## Freshness\n\n## Validation\n"), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", src, "--id", "caching", "--title", "HTTP caching", "--activate", "--yes")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "position", "set", "--node", "1.1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("What does max-age do?", false, "session", "append", "--role", "assistant")
	c.run("It deletes the file after an hour", false, "session", "append", "--role", "user")
	c.run("Where is the copy kept, and who decides it is still fresh?", false, "session", "append", "--role", "assistant")
	c.run("Oh, the browser keeps it and just stops trusting it after an hour. Why not ask the server every time?", false, "session", "append", "--role", "user")
	rec := `{"schema":"learning-os/interpretation@1","curriculum":"caching",
	  "concepts":[{"id":"max-age","label":"Freshness lifetime"},{"id":"etag","label":"Validation with ETag","related":[{"concept":"max-age","note":"used after expiry"}]}],
	  "events":[{"id":"e1","type":"misconception","concept":"max-age","summary":"thought max-age deletes the file","evidence":[{"turn":"t0002","quote":"It deletes the file after an hour"}]}],
	  "cognitive_changes":[{"id":"c1","concept":"max-age","old_model":"expiry deletes","trigger":"where is the copy kept","trigger_turn":"t0003","new_model":"expiry stops trust","old_evidence":[{"turn":"t0002","quote":"It deletes the file"}],"new_evidence":[{"turn":"t0004","quote":"just stops trusting it after an hour"}]}],
	  "state_updates":[{"id":"u1","concept":"max-age","state":"developing","capabilities":["explained"],"summary":"explains freshness as trust","evidence":[{"turn":"t0004","quote":"just stops trusting it after an hour"}],"open_questions":["what happens after expiry"]}],
	  "strategy_attempts":[{"id":"s1","strategy":"counterexample","situation":"misconception","concept":"max-age","action_turn":"t0003","expected_change":"locate the copy","outcome":"effective","linked":["c1"],"evidence":[{"turn":"t0004","quote":"the browser keeps it"}]}],
	  "questions":[{"id":"q1","question":"Why not ask the server every time?","concept":"max-age","node":"1.2","evidence":[{"turn":"t0004","quote":"Why not ask the server every time?"}]}]}`
	c.run(rec, false, "session", "checkpoint", "--analysis-file", "-")
	c.run("", false, "curriculum", "complete", "1.1", "--reason", "freshness understood")
	c.run("", false, "session", "end")
}

var cjk = regexp.MustCompile(`[\p{Han}（）「」：；、]`)

// generatedPages lists every Markdown page the Runtime wrote.
func generatedPages(t *testing.T, root string) map[string]string {
	t.Helper()
	pages := map[string]string{}
	for _, dir := range []string{".", "Concepts", "Questions", "Profile", "Sessions", "Curriculum/caching"} {
		entries, _ := os.ReadDir(filepath.Join(root, dir))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "AGENTS.md" || e.Name() == "CLAUDE.md" || e.Name() == "book.md" {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(dir, e.Name()))
			pages[rel] = read(t, filepath.Join(root, rel))
		}
	}
	return pages
}

func TestEnglishInterfaceHasNoChineseFixedText(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root, "--language", "en")
	c.learnEnglish()
	pages := generatedPages(t, c.root)
	for _, want := range []string{"README.md", "Profile/Learner overview.md", "Curriculum/caching/Progress.md", "Curriculum/caching/HTTP caching.md", "Concepts/Freshness lifetime.md", "Questions/Why not ask the server every time.md"} {
		if _, ok := pages[want]; !ok {
			t.Fatalf("missing page %s; have %v", want, keys(pages))
		}
	}
	for rel, body := range pages {
		if loc := cjk.FindStringIndex(body); loc != nil {
			start := max(0, loc[0]-80)
			t.Fatalf("%s contains Chinese fixed text near %q", rel, body[start:min(len(body), loc[1]+80)])
		}
	}
	if !strings.Contains(pages["Concepts/Freshness lifetime.md"], "## My notes") {
		t.Fatal("user section heading not translated")
	}
	if status := c.run("", false, "status", "--json"); !strings.Contains(status, `"language": "en"`) {
		t.Fatalf("status language: %s", status)
	}
}

func TestLanguageSwitchKeepsNotesAndRemovesOldPages(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.learnEnglish()
	zhOverview := filepath.Join(c.root, "Profile", "学习者总览.md")
	body := read(t, zhOverview)
	body = strings.Replace(body, "<!-- learn:user:begin -->\n", "<!-- learn:user:begin -->\nmy overview note\n", 1)
	if err := os.WriteFile(zhOverview, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "config", "set", "language", "en")
	enOverview := filepath.Join(c.root, "Profile", "Learner overview.md")
	if !strings.Contains(read(t, enOverview), "my overview note") {
		t.Fatal("user note lost when switching to English")
	}
	for _, gone := range []string{zhOverview, filepath.Join(c.root, "Curriculum", "caching", "学习进度.md")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("old page %s still present", gone)
		}
	}
	c.run("", false, "config", "set", "language", "zh")
	if !strings.Contains(read(t, zhOverview), "my overview note") {
		t.Fatal("user note lost when switching back")
	}
	if _, err := os.Stat(enOverview); !os.IsNotExist(err) {
		t.Fatal("English overview left behind")
	}
	if !strings.Contains(read(t, filepath.Join(c.root, "Curriculum", "caching", "学习进度.md")), "# 学习进度 — HTTP caching") {
		t.Fatal("progress page not restored in Chinese")
	}
	c.run("", true, "config", "set", "language", "fr")
}

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestGitReminderWhenAutoCommitFails(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	// Make .git unwritable, as an Agent sandbox does.
	objects := filepath.Join(c.root, ".git", "objects")
	if err := os.Chmod(objects, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(objects, 0o755) })
	c.learnEnglish()
	home := read(t, filepath.Join(c.root, "README.md"))
	if !strings.Contains(home, "## ⚠ 学习记录还没有保存到 Git") || !strings.Contains(home, "最近一次提交：从未提交") {
		t.Fatalf("home lacks git reminder:\n%s", home)
	}
	status := c.run("", false, "status", "--json")
	if !strings.Contains(status, `"git_auto_commit": "failed"`) || strings.Contains(status, `"git_uncommitted": 0`) {
		t.Fatalf("status: %s", status)
	}
	if err := os.Chmod(objects, 0o755); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "commit")
	if strings.Contains(read(t, filepath.Join(c.root, "README.md")), "## ⚠") {
		t.Fatal("reminder still shown after learn commit")
	}
	if dirty := gitRun(t, c.root, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
		t.Fatalf("tree not clean after commit:\n%s", dirty)
	}
	if !strings.Contains(c.run("", false, "status", "--json"), `"git_auto_commit": "committed"`) {
		t.Fatal("status did not record the commit")
	}
}

func TestNoGitReminderWhenGitDisabled(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	cfgPath := filepath.Join(c.root, ".learning", "config.yaml")
	cfg := strings.Replace(read(t, cfgPath), "enabled: true", "enabled: false", 1)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	c.learnEnglish()
	if strings.Contains(read(t, filepath.Join(c.root, "README.md")), "## ⚠") {
		t.Fatal("reminder shown with git disabled")
	}
}

func TestGitReminderAbsentWhenCommitsWork(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.learnEnglish()
	if strings.Contains(read(t, filepath.Join(c.root, "README.md")), "## ⚠") {
		t.Fatal("reminder shown although auto commits succeed")
	}
	if dirty := gitRun(t, c.root, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
		t.Fatalf("tree not clean after auto commits:\n%s", dirty)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
