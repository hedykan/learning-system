package source

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hedykan/learning-system/internal/locator"
)

type markdownAdapter struct{}

func (markdownAdapter) Kind() string { return "markdown" }
func (markdownAdapter) Detect(path string, info fs.FileInfo) bool {
	return !info.IsDir() && extOf(path) == ".md"
}
func (markdownAdapter) Capabilities() Caps                     { return Caps{Structured: true, Extractable: true} }
func (markdownAdapter) Outline(path string) ([]Section, error) { return outlineOf([]string{path}, "") }
func (markdownAdapter) Read(path string, loc locator.Locator) (Content, error) {
	return readMarkdown([]string{path}, path, loc)
}
func (markdownAdapter) Validate(path string, loc locator.Locator) error {
	return validateMarkdown([]string{path}, path, loc)
}

// folderAdapter is a folder of Markdown and text files, read in natural order.
type folderAdapter struct{}

func (folderAdapter) Kind() string                           { return "directory" }
func (folderAdapter) Detect(_ string, info fs.FileInfo) bool { return info.IsDir() }
func (folderAdapter) Capabilities() Caps                     { return Caps{Structured: true, Extractable: true} }
func (folderAdapter) Outline(path string) ([]Section, error) {
	files, err := folderFiles(path, ".md")
	if err != nil {
		return nil, err
	}
	return outlineOf(files, path)
}
func (folderAdapter) Read(path string, loc locator.Locator) (Content, error) {
	files, err := folderFiles(path, ".md")
	if err != nil {
		return Content{}, err
	}
	return readMarkdown(files, path, loc)
}
func (folderAdapter) Validate(path string, loc locator.Locator) error {
	files, err := folderFiles(path, ".md")
	if err != nil {
		return err
	}
	return validateMarkdown(files, path, loc)
}

// outlineOf lists level 1 and 2 headings, the levels a draft outline uses.
func outlineOf(files []string, root string) ([]Section, error) {
	var out []Section
	for _, f := range files {
		_, heads, err := readLines(f)
		if err != nil {
			return nil, err
		}
		rel := ""
		if root != "" {
			r, _ := filepath.Rel(root, f)
			rel = filepath.ToSlash(r)
		}
		for _, h := range heads {
			if h.level <= 2 {
				out = append(out, Section{Level: h.level, Title: h.title, Anchor: "#" + Slug(h.title), File: rel})
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrNoStructure
	}
	return out, nil
}

// findAnchor locates the heading of an anchor in the first file having it.
func findAnchor(files []string, anchor string) (string, []string, int, int, error) {
	want := strings.TrimPrefix(anchor, "#")
	for _, f := range files {
		lines, heads, err := readLines(f)
		if err != nil {
			return "", nil, 0, 0, err
		}
		for i, h := range heads {
			if Slug(h.title) != want {
				continue
			}
			end := len(lines)
			for _, next := range heads[i+1:] {
				if next.level <= h.level {
					end = next.line
					break
				}
			}
			return f, lines, h.line, end, nil
		}
	}
	return "", nil, 0, 0, fmt.Errorf("no heading with anchor %s", anchor)
}

func readMarkdown(files []string, path string, loc locator.Locator) (Content, error) {
	if err := requireKinds("markdown", loc, "anchor", "file"); err != nil {
		return Content{}, err
	}
	if loc.Kind == "file" {
		return readLineRange(path, loc)
	}
	_, lines, start, end, err := findAnchor(files, loc.Value)
	if err != nil {
		return Content{}, err
	}
	text := strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n") + "\n"
	return Content{Locator: loc, Format: "markdown", Text: text, Images: imagesIn(text)}, nil
}

func validateMarkdown(files []string, path string, loc locator.Locator) error {
	if err := requireKinds("markdown", loc, "anchor", "file"); err != nil {
		return err
	}
	if loc.Kind == "file" {
		rel, _, _ := splitFile(loc.Value)
		_, err := resolveFile(path, rel)
		return err
	}
	_, _, _, _, err := findAnchor(files, loc.Value)
	return err
}

var imageRef = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)

// imagesIn lists Markdown image targets so the Agent can look at them.
func imagesIn(text string) []string {
	out := []string{}
	for _, m := range imageRef.FindAllStringSubmatch(text, -1) {
		if isImage(m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

func isImage(target string) bool {
	switch extOf(target) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return true
	}
	return false
}
