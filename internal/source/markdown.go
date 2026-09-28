package source

import (
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/hedykan/learning-system/internal/locator"
)

func parseMarkdown(name string, data []byte) ([]doc, error) {
	return []doc{markdownDoc(name, string(data))}, nil
}

// folderAdapter is a folder of documents in any supported text format,
// read in natural path order (ch2 before ch10).
type folderAdapter struct{}

func (folderAdapter) Kind() string                           { return "directory" }
func (folderAdapter) Detect(_ string, info fs.FileInfo) bool { return info.IsDir() }
func (folderAdapter) Capabilities() Caps                     { return Caps{Structured: true, Extractable: true} }

// folderDocs loads every file a document adapter understands; plain text
// files take part as documents without headings.
func folderDocs(dir string) ([]doc, error) {
	files, err := folderFiles(dir)
	if err != nil {
		return nil, err
	}
	var out []doc
	for _, f := range files {
		info, err := fs.Stat(dirFS{}, f)
		if err != nil {
			return nil, err
		}
		a, err := Detect(f, info)
		if err != nil {
			continue
		}
		da, ok := a.(docAdapter)
		if !ok {
			continue
		}
		docs, err := da.load(f)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(dir, f)
		for i := range docs {
			if len(docs) == 1 {
				docs[i].name = filepath.ToSlash(rel)
			} else {
				docs[i].name = filepath.ToSlash(filepath.Join(rel, docs[i].name))
			}
		}
		out = append(out, docs...)
	}
	return out, nil
}

func (folderAdapter) Outline(path string) ([]Section, error) {
	docs, err := folderDocs(path)
	if err != nil {
		return nil, err
	}
	return outlineOfDocs(docs, true)
}

func (folderAdapter) Read(path string, loc locator.Locator) (Content, error) {
	if err := requireKinds("directory", loc, "anchor", "file", "chapter"); err != nil {
		return Content{}, err
	}
	if loc.Kind == "file" {
		return readLineRange(path, loc)
	}
	docs, err := folderDocs(path)
	if err != nil {
		return Content{}, err
	}
	text, err := readDocs(docs, loc)
	if err != nil {
		return Content{}, err
	}
	return Content{Locator: loc, Format: "markdown", Text: text, Images: imagesIn(text)}, nil
}

func (folderAdapter) Validate(path string, loc locator.Locator) error {
	if err := requireKinds("directory", loc, "anchor", "file", "chapter"); err != nil {
		return err
	}
	if loc.Kind == "file" {
		rel, _, _ := splitFile(loc.Value)
		_, err := resolveFile(path, rel)
		return err
	}
	docs, err := folderDocs(path)
	if err != nil {
		return err
	}
	_, err = readDocs(docs, loc)
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

// webAdapter is a stored web snapshot: numbered HTML pages in fetch order
// (CR-2026-031). It is never detected; snapshots are made by `learn source
// add <url>`.
type webAdapter struct{ folderAdapter }

func (webAdapter) Kind() string                    { return "web" }
func (webAdapter) Detect(string, fs.FileInfo) bool { return false }
