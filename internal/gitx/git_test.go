package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestCommitWithoutAnyIdentityUsesFallback(t *testing.T) {
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir()) // no global Git config
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "a.md"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "ignored"), 0o755)
	os.WriteFile(filepath.Join(root, "ignored", "b.md"), []byte("y\n"), 0o644)
	if n, _ := Uncommitted(root); n != 2 {
		t.Fatalf("uncommitted = %d, want 2 (ignored files excluded)", n)
	}
	if ok, err := Commit(root, "first"); err != nil || !ok {
		t.Fatalf("commit: %v %v", ok, err)
	}
	repo, _ := git.PlainOpen(root)
	head, _ := repo.Head()
	if head.Name().Short() != "main" {
		t.Fatalf("default branch = %s", head.Name().Short())
	}
	c, _ := repo.CommitObject(head.Hash())
	if c.Author.Name != FallbackName || c.Author.Email != FallbackEmail {
		t.Fatalf("author = %s <%s>", c.Author.Name, c.Author.Email)
	}
	if st, _ := Status(root); st != "clean" || LastCommit(root) == "" {
		t.Fatalf("status %s, last commit %q", st, LastCommit(root))
	}
	if ok, _ := Commit(root, "nothing"); ok {
		t.Fatal("empty commit made")
	}
	os.Remove(filepath.Join(root, "a.md"))
	os.WriteFile(filepath.Join(root, "c.md"), []byte("z\n"), 0o644)
	if ok, err := Commit(root, "second"); err != nil || !ok {
		t.Fatalf("commit with a deletion: %v %v", ok, err)
	}
	os.WriteFile(filepath.Join(root, "d.md"), []byte("w\n"), 0o644)
	if err := AmendAll(root); err != nil {
		t.Fatal(err)
	}
	head, _ = repo.Head()
	c, _ = repo.CommitObject(head.Hash())
	if c.Message != "second" || c.NumParents() != 1 {
		t.Fatalf("amend kept message %q, parents %d", c.Message, c.NumParents())
	}
	tree, _ := c.Tree()
	for name, want := range map[string]bool{"a.md": false, "c.md": true, "d.md": true, "ignored/b.md": false} {
		if _, err := tree.File(name); (err == nil) != want {
			t.Fatalf("%s in tree = %v, want %v", name, err == nil, want)
		}
	}
	if st, _ := Status(root); st != "clean" {
		t.Fatalf("status after amend: %s", st)
	}
}
