package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListVaultsInAFolder(t *testing.T) {
	c := newCLI(t)
	folder := filepath.Dir(c.root) // c.root is <tmp>/vault
	c.run("", false, "init", c.root)
	src := filepath.Join(t.TempDir(), "book.md")
	os.WriteFile(src, []byte("# 一\n\n## 甲\n\n## 乙\n"), 0o644)
	c.run("", false, "curriculum", "import", src, "--id", "book", "--title", "书", "--activate", "--yes")
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "curriculum", "complete", "1.1", "--reason", "学完")

	math := filepath.Join(folder, "personal", "math")
	c.run("", false, "init", math, "--language", "en")
	broken := filepath.Join(folder, "broken")
	c.run("", false, "init", broken)
	os.WriteFile(filepath.Join(broken, ".learning", "config.yaml"), []byte("version: 99\n"), 0o644)
	os.MkdirAll(filepath.Join(folder, "too", "deep", "down"), 0o755)
	c.run("", false, "init", filepath.Join(folder, "too", "deep", "down"))

	out := c.run("", false, "vaults", folder, "--json")
	for _, want := range []string{`"name": "vault"`, `"name": "math"`, `"language": "en"`, `"active_curriculum": "book"`,
		`"completed": 1`, `"entries": 2`, `"name": "broken"`, `"error": "unsupported config version 99"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("vaults lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"name": "down"`) {
		t.Fatal("vaults deeper than two levels must not be listed")
	}
	if strings.Count(out, `"path":`) != 3 {
		t.Fatalf("a vault's own folders were scanned as vaults:\n%s", out)
	}
	if self := c.run("", false, "vaults", c.root, "--json"); strings.Count(self, `"path":`) != 1 || !strings.Contains(self, `"name": "vault"`) {
		t.Fatalf("a vault folder itself: %s", self)
	}
	if human := c.run("", false, "vaults", folder); !strings.Contains(human, "* book  书  1/2") {
		t.Fatalf("human output:\n%s", human)
	}
	if status := c.run("", false, "--vault", math, "status", "--json"); !strings.Contains(status, `"language": "en"`) {
		t.Fatalf("--vault into a listed vault: %s", status)
	}
}
