package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func codeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "raft")
	for name, body := range files {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, repo, "init", "-q")
	// No background gc or maintenance: they keep writing to .git after the
	// commit and race the temp-dir cleanup.
	gitRun(t, repo, "config", "gc.auto", "0")
	gitRun(t, repo, "config", "maintenance.auto", "false")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "init")
	return repo
}

var commitField = regexp.MustCompile(`"commit": "([0-9a-f]{40})"`)

func TestCodeProjectReadsPinnedCommits(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	repo := codeRepo(t, map[string]string{
		"src/raft.go":             "package raft\n\nfunc elect() {\n\t// v1\n}\n",
		".env":                    "TOKEN=secret\n",
		"deploy/id_rsa":           "KEY\n",
		"node_modules/x/index.js": "x\n",
		"assets/logo.bin":         "PNG\x00\x01binary",
	})
	os.WriteFile(filepath.Join(repo, "src", "raft.go"), []byte("package raft\n// edited, not committed\n"), 0o644)
	plan := c.run("", false, "curriculum", "import", repo, "--kind", "code", "--id", "raft", "--dry-run", "--json")
	if !strings.Contains(plan, "uncommitted changes") {
		t.Fatalf("dirty warning missing: %s", plan)
	}
	first := commitField.FindStringSubmatch(plan)[1]
	c.run("", false, "curriculum", "import", repo, "--kind", "code", "--id", "raft", "--activate", "--yes")
	if _, err := os.Stat(filepath.Join(c.root, "Sources", "raft", "original")); !os.IsNotExist(err) {
		t.Fatal("a code project must be linked, not copied")
	}
	read := c.run("", false, "source", "read", "raft", "file", "src/raft.go#L3-5", "--json")
	if !strings.Contains(read, "// v1") || strings.Contains(read, "edited") || !strings.Contains(read, "```go") {
		t.Fatalf("must read the imported commit: %s", read)
	}
	for file, why := range map[string]string{".env": "credentials", "deploy/id_rsa": "private key", "node_modules/x/index.js": "dependencies", "assets/logo.bin": "binary"} {
		if out := c.run("", true, "source", "read", "raft", "file", file); !strings.Contains(out, why) {
			t.Fatalf("%s: %s", file, out)
		}
	}

	// A new commit becomes a new revision; pinned positions keep the old text.
	os.WriteFile(filepath.Join(repo, "src", "raft.go"), []byte("package raft\n\nfunc elect() {\n\t// v2\n}\n"), 0o644)
	gitRun(t, repo, "commit", "-q", "-am", "v2")
	c.run(`{"nodes":[{"id":"1","title":"选举"}]}`, false, "curriculum", "outline", "set", "--file", "-")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "source", "attach", "1", "raft", "file", "src/raft.go#L3-5")
	refresh := c.run("", false, "source", "refresh", "raft", "--json")
	if !strings.Contains(refresh, `"changed": true`) {
		t.Fatalf("refresh: %s", refresh)
	}
	if now := c.run("", false, "source", "read", "raft", "file", "src/raft.go#L4"); !strings.Contains(now, "v2") {
		t.Fatalf("current revision: %s", now)
	}
	if old := c.run("", false, "source", "read", "raft", "file", "src/raft.go#L4@"+first); !strings.Contains(old, "v1") {
		t.Fatalf("pinned old revision: %s", old)
	}
	if res := read2(t, filepath.Join(c.root, "Curriculum", "raft", "resources.yaml")); !strings.Contains(res, "@"+first) {
		t.Fatalf("attachment not pinned to the commit it was read at:\n%s", res)
	}

	c.run("", false, "curriculum", "position", "set", "--node", "1")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("谁发起选举？", false, "session", "append", "--role", "assistant")
	c.run("超时的 follower 自己变成 candidate", false, "session", "append", "--role", "user")
	rec := func(loc string) string {
		return `{"schema":"learning-os/interpretation@1","curriculum":"raft","concepts":[{"id":"election","label":"领导者选举","source_ref":{"node":"1"},
		  "textbook_points":{"locator":{"kind":"file","value":"` + loc + `"},"points":["超时触发选举"]}}],
		  "state_updates":[{"id":"u1","concept":"election","state":"developing","capabilities":["explained"],"summary":"知道谁发起","evidence":[{"turn":"t0002","quote":"超时的 follower 自己变成 candidate"}]}]}`
	}
	if out := c.run(rec("src/raft.go#L3-5"), true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(out, "pin the code position") {
		t.Fatalf("unpinned code points: %s", out)
	}
	c.run(rec("src/raft.go#L3-5@"+first), false, "session", "checkpoint", "--analysis-file", "-")
}

func read2(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLargeCodeProjectIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 10,000-file repository")
	}
	files := map[string]string{}
	for i := 0; i < 10000; i++ {
		files[fmt.Sprintf("pkg%03d/file%05d.go", i%100, i)] = fmt.Sprintf("package pkg\n\nconst N%d = %d\n", i, i)
	}
	repo := codeRepo(t, files)
	c := newCLI(t)
	c.run("", false, "init", c.root)
	start := time.Now()
	c.run("", false, "source", "add", repo, "--kind", "code", "--id", "big")
	out := c.run("", false, "source", "read", "big", "file", "pkg042/file09942.go#L3")
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("import and read took %s", d)
	}
	if !strings.Contains(out, "N9942 = 9942") {
		t.Fatalf("read: %s", out)
	}
}
