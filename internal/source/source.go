// Package source reads learning resources through one adapter per format
// (CR-2026-026). Adapters only do deterministic work: recognize a format,
// derive a draft outline from its structure, extract text at a locator and
// validate locators. Understanding the content is the Agent's job.
package source

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/hedykan/learning-system/internal/locator"
)

// Caps declares what the Runtime can do with a resource.
type Caps struct {
	Structured  bool `json:"structured"`   // a draft outline can be derived
	Extractable bool `json:"extractable"`  // text can be read at a locator
	NeedsVision bool `json:"needs_vision"` // the Agent may have to look at page images
	External    bool `json:"external"`     // no file: the learner brings the content
}

// Section is one heading of a structured resource, in document order.
type Section struct {
	Level  int    `json:"level"`
	Title  string `json:"title"`
	Anchor string `json:"anchor"`
	File   string `json:"file,omitempty"` // file within a folder resource
	// Chapter is the e-book chapter locator value of the section.
	Chapter string `json:"chapter,omitempty"`
}

// Content is text extracted at a locator, always as Markdown.
type Content struct {
	Locator     locator.Locator `json:"locator"`
	Format      string          `json:"format"`
	Text        string          `json:"text"`
	Images      []string        `json:"images"`
	NeedsVision bool            `json:"needs_vision"`
}

// ErrNoStructure means no draft outline can be derived; the Agent builds one.
var ErrNoStructure = errors.New("resource has no structure to derive an outline from")

// Unsupported reports an operation an adapter cannot do, with the reason the
// Agent should act on.
type Unsupported struct{ Reason string }

func (u *Unsupported) Error() string { return "unsupported: " + u.Reason }

// Adapter handles one resource format. path is the stored content (a file or
// a folder); external resources have no path.
type Adapter interface {
	Kind() string
	Detect(path string, info fs.FileInfo) bool
	Capabilities() Caps
	Outline(path string) ([]Section, error)
	Read(path string, loc locator.Locator) (Content, error)
	Validate(path string, loc locator.Locator) error
}

// Checker is implemented by adapters that can refuse a file outright, such
// as a DRM-protected e-book; importers call it before storing the file.
type Checker interface {
	Check(path string) error
}

var registry []Adapter

// Register adds an adapter; earlier registrations win on Detect.
func Register(a Adapter) { registry = append(registry, a) }

func init() {
	Register(docAdapter{kind: "markdown", exts: []string{".md", ".markdown"}, load: fileDocs(parseMarkdown), lineFiles: true})
	Register(folderAdapter{})
	Register(newText())
	Register(docAdapter{kind: "html", exts: []string{".html", ".htm", ".xhtml"}, load: fileDocs(parseHTML)})
	Register(docAdapter{kind: "epub", exts: []string{".epub"}, load: loadEPUB, chapters: true, outline: epubOutline, check: checkEPUB})
	Register(docAdapter{kind: "docx", exts: []string{".docx"}, load: fileDocs(parseDOCX)})
	Register(docAdapter{kind: "ipynb", exts: []string{".ipynb"}, load: fileDocs(parseNotebook)})
	Register(docAdapter{kind: "latex", exts: []string{".tex"}, load: fileDocs(parseLaTeX), lineFiles: true})
	Register(docAdapter{kind: "rst", exts: []string{".rst"}, load: fileDocs(parseRST), lineFiles: true})
	Register(docAdapter{kind: "asciidoc", exts: []string{".adoc", ".asciidoc"}, load: fileDocs(parseAsciiDoc), lineFiles: true})
	Register(docAdapter{kind: "org", exts: []string{".org"}, load: fileDocs(parseOrg), lineFiles: true})
	Register(pdfAdapter{})
	Register(codeAdapter{})
	Register(webAdapter{})
	Register(externalAdapter{})
}

// Detect picks the adapter for a file or folder.
func Detect(path string, info fs.FileInfo) (Adapter, error) {
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source must be a regular file or directory")
	}
	for _, a := range registry {
		if a.Detect(path, info) {
			return a, nil
		}
	}
	var kinds []string
	for _, a := range registry {
		if !a.Capabilities().External {
			kinds = append(kinds, a.Kind())
		}
	}
	return nil, fmt.Errorf("unsupported source format %q; supported: %s", extOf(path), strings.Join(kinds, ", "))
}

// ForKind returns the adapter registered for a kind.
func ForKind(kind string) (Adapter, bool) {
	for _, a := range registry {
		if a.Kind() == kind {
			return a, true
		}
	}
	return nil, false
}

// requireKinds rejects locator kinds an adapter does not understand.
func requireKinds(adapter string, loc locator.Locator, kinds ...string) error {
	if err := loc.Validate(); err != nil {
		return err
	}
	for _, k := range kinds {
		if loc.Kind == k {
			return nil
		}
	}
	return fmt.Errorf("%s resources use %s locators, not %s", adapter, strings.Join(kinds, " or "), loc.Kind)
}
