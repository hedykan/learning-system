package fsutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hedykan/learning-system/internal/fsutil"
)

func TestWriteFileIfAbsentPreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	if err := os.WriteFile(path, []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := fsutil.WriteFileIfAbsent(path, []byte("replacement"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("existing file reported as created")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "user content"; got != want {
		t.Fatalf("content = %q; want %q", got, want)
	}
}

func TestHashPathIsStableForDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := fsutil.HashPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fsutil.HashPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hash changed: %s != %s", first, second)
	}
}
