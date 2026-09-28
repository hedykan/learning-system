package source

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/hedykan/learning-system/internal/locator"
)

func extOf(path string) string { return strings.ToLower(filepath.Ext(path)) }

var headingLine = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)

// Slug turns a heading into its anchor the way Markdown renderers do:
// lower case, spaces to hyphens, punctuation dropped, letters of any
// script kept.
func Slug(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	return b.String()
}

type heading struct {
	level int
	title string
	line  int // 0-based
}

// readLines reads a file, reporting headings outside code fences.
func readLines(path string) ([]string, []heading, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer fh.Close()
	var lines []string
	var heads []heading
	inFence := false
	scanner := bufio.NewScanner(fh)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		} else if m := headingLine.FindStringSubmatch(line); m != nil && !inFence {
			heads = append(heads, heading{len(m[1]), m[2], len(lines)})
		}
		lines = append(lines, line)
	}
	return lines, heads, scanner.Err()
}

// naturalLess orders names so that numbers compare by value: ch2 < ch10.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digitsPrefix(a), digitsPrefix(b)
		if da != "" && db != "" {
			na, _ := strconv.Atoi(da)
			nb, _ := strconv.Atoi(db)
			if na != nb {
				return na < nb
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digitsPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// folderFiles lists files with the given extensions, in natural path order.
func folderFiles(dir string, exts ...string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != dir {
			return filepath.SkipDir
		}
		for _, e := range exts {
			if !d.IsDir() && extOf(p) == e {
				files = append(files, p)
			}
		}
		return nil
	})
	sort.SliceStable(files, func(i, j int) bool {
		ri, _ := filepath.Rel(dir, files[i])
		rj, _ := filepath.Rel(dir, files[j])
		return naturalLess(filepath.ToSlash(ri), filepath.ToSlash(rj))
	})
	return files, err
}

// readLineRange serves a file locator from a single file or a folder.
func readLineRange(path string, loc locator.Locator) (Content, error) {
	rel, from, to := splitFile(loc.Value)
	target, err := resolveFile(path, rel)
	if err != nil {
		return Content{}, err
	}
	lines, _, err := readLines(target)
	if err != nil {
		return Content{}, err
	}
	if from < 1 {
		from, to = 1, len(lines)
	}
	if from > len(lines) {
		return Content{}, fmt.Errorf("%s has %d lines; line %d does not exist", rel, len(lines), from)
	}
	if to > len(lines) {
		to = len(lines)
	}
	return Content{Locator: loc, Format: "markdown", Text: strings.Join(lines[from-1:to], "\n") + "\n", Images: []string{}}, nil
}

func splitFile(v string) (string, int, int) {
	rel, lines, _ := strings.Cut(v, "#L")
	if lines == "" {
		return rel, 0, 0
	}
	a, b, found := strings.Cut(lines, "-")
	from, _ := strconv.Atoi(a)
	to := from
	if found {
		to, _ = strconv.Atoi(b)
	}
	return rel, from, to
}

// resolveFile maps a relative file of a locator onto the stored resource:
// the file itself (by name) or a file inside the folder.
func resolveFile(path, rel string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		if filepath.Base(path) != filepath.Base(rel) {
			return "", fmt.Errorf("resource is the single file %s, not %s", filepath.Base(path), rel)
		}
		return path, nil
	}
	target := filepath.Join(path, filepath.FromSlash(rel))
	if r, err := filepath.Rel(path, target); err != nil || strings.HasPrefix(r, "..") {
		return "", fmt.Errorf("file %s is outside the resource", rel)
	}
	if _, err := os.Stat(target); err != nil {
		return "", fmt.Errorf("file %s does not exist in the resource", rel)
	}
	return target, nil
}

type textAdapter struct{}

func (textAdapter) Kind() string { return "text" }
func (textAdapter) Detect(path string, info fs.FileInfo) bool {
	return !info.IsDir() && extOf(path) == ".txt"
}
func (textAdapter) Capabilities() Caps                { return Caps{Extractable: true} }
func (textAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }
func (textAdapter) Read(path string, loc locator.Locator) (Content, error) {
	if err := requireKinds("text", loc, "file"); err != nil {
		return Content{}, err
	}
	return readLineRange(path, loc)
}
func (a textAdapter) Validate(path string, loc locator.Locator) error {
	if err := requireKinds("text", loc, "file"); err != nil {
		return err
	}
	rel, _, _ := splitFile(loc.Value)
	_, err := resolveFile(path, rel)
	return err
}
