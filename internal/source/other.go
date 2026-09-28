package source

import (
	"io/fs"

	"github.com/hedykan/learning-system/internal/locator"
)

// pdfAdapter keeps v0.1 behavior: the Agent builds the outline and reads the
// pages itself (text extraction is deferred, IDEA-015).
type pdfAdapter struct{}

func (pdfAdapter) Kind() string { return "pdf" }
func (pdfAdapter) Detect(path string, info fs.FileInfo) bool {
	return !info.IsDir() && extOf(path) == ".pdf"
}
func (pdfAdapter) Capabilities() Caps                { return Caps{NeedsVision: true} }
func (pdfAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }
func (pdfAdapter) Read(string, locator.Locator) (Content, error) {
	return Content{}, &Unsupported{"PDF text extraction is not built in; read the pages yourself, and use vision for scanned pages"}
}
func (pdfAdapter) Validate(_ string, loc locator.Locator) error {
	return requireKinds("pdf", loc, "page")
}

// externalAdapter is a resource without a file: a video course, a paper
// book or a class (CR-2026-028). The learner brings its content.
type externalAdapter struct{}

func (externalAdapter) Kind() string                      { return "external" }
func (externalAdapter) Detect(string, fs.FileInfo) bool   { return false }
func (externalAdapter) Capabilities() Caps                { return Caps{External: true} }
func (externalAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }
func (externalAdapter) Read(string, locator.Locator) (Content, error) {
	return Content{}, &Unsupported{"external resource: ask the learner to bring the content (what they watched or read, a photo or a transcript)"}
}
func (externalAdapter) Validate(_ string, loc locator.Locator) error {
	return requireKinds("external", loc, "page", "time", "text")
}

// goalAdapter is the empty primary of a curriculum built from a learning
// goal (CR-2026-040): its entries are read through attached resources.
type goalAdapter struct{}

func (goalAdapter) Kind() string                      { return "goal" }
func (goalAdapter) Detect(string, fs.FileInfo) bool   { return false }
func (goalAdapter) Capabilities() Caps                { return Caps{} }
func (goalAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }
func (goalAdapter) Read(string, locator.Locator) (Content, error) {
	return Content{}, &Unsupported{"this curriculum was built from a learning goal and has no material of its own; read the resources attached to the entry (learn next lists them), or attach one with learn source add and learn source attach"}
}
func (goalAdapter) Validate(_ string, loc locator.Locator) error {
	return requireKinds("goal", loc, "text")
}
