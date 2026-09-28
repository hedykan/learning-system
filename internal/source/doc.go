package source

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hedykan/learning-system/internal/locator"
)

// doc is one readable unit of a resource as Markdown-like lines with its
// headings: a Markdown file, one chapter of an e-book, a converted Word file.
type doc struct {
	name  string // file name, or the chapter's href inside an e-book
	lines []string
	heads []heading
}

// heading is a section start. ids are explicit anchors (HTML id, LaTeX
// label) that a locator may use besides the heading's slug.
type heading struct {
	level int
	title string
	line  int // 0-based
	ids   []string
}

// markdownDoc parses Markdown text, skipping headings inside code fences.
func markdownDoc(name, text string) doc {
	d := doc{name: name}
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		} else if m := headingLine.FindStringSubmatch(line); m != nil && !inFence {
			title, ids := splitHeadingID(m[2])
			d.heads = append(d.heads, heading{level: len(m[1]), title: title, line: len(d.lines), ids: ids})
		}
		d.lines = append(d.lines, line)
	}
	return d
}

var headingID = regexp.MustCompile(`\s*\{#([^}\s]+)\}$`)

// splitHeadingID separates a trailing {#id} (written by converters for
// explicit anchors) from the heading title.
func splitHeadingID(title string) (string, []string) {
	if m := headingID.FindStringSubmatch(title); m != nil {
		return strings.TrimSpace(title[:len(title)-len(m[0])]), []string{m[1]}
	}
	return title, nil
}

func (h heading) matches(anchor string) bool {
	anchor = strings.TrimPrefix(anchor, "#")
	if Slug(h.title) == anchor {
		return true
	}
	for _, id := range h.ids {
		if id == anchor {
			return true
		}
	}
	return false
}

// section returns the lines of the heading at index i of d, up to the next
// heading of the same or a higher level.
func (d doc) section(i int) string {
	h := d.heads[i]
	end := len(d.lines)
	for _, next := range d.heads[i+1:] {
		if next.level <= h.level {
			end = next.line
			break
		}
	}
	return strings.TrimRight(strings.Join(d.lines[h.line:end], "\n"), "\n") + "\n"
}

func (d doc) text() string { return strings.TrimRight(strings.Join(d.lines, "\n"), "\n") + "\n" }

// docAdapter reads a format by loading it into docs; outline, anchor reads
// and validation are shared by every such format.
type docAdapter struct {
	kind string
	exts []string
	load func(path string) ([]doc, error)
	// lineFiles is true for plain-text formats whose file lines a `file`
	// locator can address.
	lineFiles bool
	// chapters is true for e-books, whose docs a `chapter` locator names.
	chapters bool
	// outline overrides heading-based outlines (e-book tables of contents).
	outline func(path string) ([]Section, error)
	// check rejects files that cannot be used at all (e.g. DRM).
	check func(path string) error
}

// Check reports whether a file can be imported; see Checker.
func (a docAdapter) Check(path string) error {
	if a.check == nil {
		return nil
	}
	return a.check(path)
}

func (a docAdapter) Kind() string { return a.kind }
func (a docAdapter) Detect(path string, info fs.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	for _, e := range a.exts {
		if extOf(path) == e {
			return true
		}
	}
	return false
}
func (a docAdapter) Capabilities() Caps { return Caps{Structured: true, Extractable: true} }

func (a docAdapter) locKinds() []string {
	kinds := []string{"anchor"}
	if a.lineFiles {
		kinds = append(kinds, "file")
	}
	if a.chapters {
		kinds = append(kinds, "chapter")
	}
	return kinds
}

func (a docAdapter) docs(path string) ([]doc, error) {
	docs, err := a.load(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return docs, nil
}

func (a docAdapter) Outline(path string) ([]Section, error) {
	if a.outline != nil {
		if s, err := a.outline(path); err == nil && len(s) > 0 {
			var top []Section // a draft outline uses the first two levels
			for _, x := range s {
				if x.Level <= 2 {
					top = append(top, x)
				}
			}
			return top, nil
		}
	}
	docs, err := a.docs(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoStructure, err)
	}
	return outlineOfDocs(docs, a.chapters)
}

func (a docAdapter) Read(path string, loc locator.Locator) (Content, error) {
	if err := requireKinds(a.kind, loc, a.locKinds()...); err != nil {
		return Content{}, err
	}
	if loc.Kind == "file" {
		return readLineRange(path, loc)
	}
	docs, err := a.docs(path)
	if err != nil {
		return Content{}, err
	}
	text, err := readDocs(docs, loc)
	if err != nil {
		return Content{}, err
	}
	return Content{Locator: loc, Format: "markdown", Text: text, Images: imagesIn(text)}, nil
}

func (a docAdapter) Validate(path string, loc locator.Locator) error {
	if err := requireKinds(a.kind, loc, a.locKinds()...); err != nil {
		return err
	}
	if loc.Kind == "file" {
		rel, _, _ := splitFile(loc.Value)
		_, err := resolveFile(path, rel)
		return err
	}
	docs, err := a.docs(path)
	if err != nil {
		return err
	}
	_, err = readDocs(docs, loc)
	return err
}

// outlineOfDocs lists level 1 and 2 headings in document order.
func outlineOfDocs(docs []doc, chapters bool) ([]Section, error) {
	var out []Section
	for _, d := range docs {
		for _, h := range d.heads {
			if h.level > 2 {
				continue
			}
			s := Section{Level: h.level, Title: h.title, Anchor: "#" + Slug(h.title), File: d.name}
			if len(h.ids) > 0 {
				s.Anchor = "#" + h.ids[0]
			}
			if chapters {
				s.Chapter = d.name + s.Anchor
			}
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoStructure
	}
	return out, nil
}

// readDocs serves anchor and chapter locators.
func readDocs(docs []doc, loc locator.Locator) (string, error) {
	switch loc.Kind {
	case "anchor":
		var found []string
		text := ""
		for _, d := range docs {
			for i, h := range d.heads {
				if h.matches(loc.Value) {
					if len(found) == 0 {
						text = d.section(i)
					}
					found = append(found, d.name)
					break
				}
			}
		}
		switch len(found) {
		case 0:
			return "", fmt.Errorf("no heading with anchor %s", loc.Value)
		case 1:
			return text, nil
		}
		return "", fmt.Errorf("anchor %s appears in %s; use a chapter locator such as %s%s", loc.Value, strings.Join(found, ", "), found[len(found)-1], loc.Value)
	case "chapter":
		name, frag, _ := strings.Cut(loc.Value, "#")
		for _, d := range docs {
			if d.name != name && filepath.Base(d.name) != name {
				continue
			}
			if frag == "" {
				return d.text(), nil
			}
			for i, h := range d.heads {
				if h.matches(frag) {
					return d.section(i), nil
				}
			}
			return "", fmt.Errorf("chapter %s has no section #%s", name, frag)
		}
		return "", fmt.Errorf("no chapter %s", name)
	}
	return "", fmt.Errorf("unsupported locator kind %s", loc.Kind)
}

// fileDocs loads one file with a per-format parser.
func fileDocs(parse func(name string, data []byte) ([]doc, error)) func(string) ([]doc, error) {
	return func(path string) ([]doc, error) {
		data, err := readFile(path)
		if err != nil {
			return nil, err
		}
		return parse(filepath.Base(path), data)
	}
}
