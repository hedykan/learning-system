package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hedykan/learning-system/internal/gitx"
)

// Everything must work where no git executable exists, as in a mobile app
// (CR-2026-048).
func TestWorksWithoutAGitExecutable(t *testing.T) {
	c := newCLI(t)
	t.Setenv("PATH", t.TempDir())
	c.run("", false, "init", c.root)
	c.learnEnglish() // import, outline, session, checkpoint, end: auto commits
	if st := c.run("", false, "status", "--json"); !strings.Contains(st, `"git_auto_commit": "committed"`) || !strings.Contains(st, `"git_uncommitted": 0`) {
		t.Fatalf("status without git: %s", st)
	}
	if gitx.LastCommit(c.root) == "" {
		t.Fatal("no commit was made")
	}

	repo := filepath.Join(t.TempDir(), "proj")
	os.MkdirAll(filepath.Join(repo, "src"), 0o755)
	os.WriteFile(filepath.Join(repo, "src", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	if err := gitx.Init(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Commit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "source", "add", repo, "--kind", "code", "--id", "proj")
	if out := c.run("", false, "source", "read", "proj", "file", "src/main.go#L3"); !strings.Contains(out, "func main() {}") {
		t.Fatalf("code read without git: %s", out)
	}
	list := c.run("", false, "source", "list", "--json")
	short := list[strings.Index(list, `"commit": "`)+11:][:7]
	if out := c.run("", false, "source", "read", "proj", "file", "src/main.go#L1@"+short); !strings.Contains(out, "package main") {
		t.Fatalf("short commit pin: %s", out)
	}
}
