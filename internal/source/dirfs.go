package source

import (
	"io/fs"
	"os"
	"path/filepath"
)

// dirFS stats absolute paths through io/fs.
type dirFS struct{}

func (dirFS) Open(name string) (fs.File, error)     { return os.Open(name) }
func (dirFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// samePath compares two paths after resolving symbolic links.
func samePath(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
