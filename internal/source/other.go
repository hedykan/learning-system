package source

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hedykan/learning-system/internal/locator"
)

// pdfAdapter reads text-layer PDFs page by page. Scanned pages return
// NeedsVision so the caller can render/OCR them without pretending they were
// empty reading material.
type pdfAdapter struct{}

func (pdfAdapter) Kind() string { return "pdf" }
func (pdfAdapter) Detect(path string, info fs.FileInfo) bool {
	return !info.IsDir() && extOf(path) == ".pdf"
}
func (pdfAdapter) Capabilities() Caps                { return Caps{Extractable: true, NeedsVision: true} }
func (pdfAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }
func (pdfAdapter) Read(path string, loc locator.Locator) (Content, error) {
	if err := requireKinds("pdf", loc, "page"); err != nil {
		return Content{}, err
	}
	from, to, _ := locator.PageRange(loc.Value)
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return Content{}, &Unsupported{"PDF reading needs the pdftotext command; use vision for scanned pages"}
	}
	cmd := exec.Command("pdftotext", "-f", strconv.Itoa(from), "-l", strconv.Itoa(to), "-layout", path, "-")
	b, err := cmd.Output()
	if err != nil {
		return Content{}, &Unsupported{fmt.Sprintf("PDF text extraction failed for pages %s: %v", loc.Value, err)}
	}
	text := strings.TrimSpace(string(bytes.ReplaceAll(b, []byte{'\f'}, []byte{'\n'})))
	needsVision := text == ""
	if text != "" {
		text += "\n"
	}
	return Content{Locator: loc, Format: "text", Text: text, Images: []string{}, NeedsVision: needsVision}, nil
}
func (pdfAdapter) Validate(_ string, loc locator.Locator) error {
	return requireKinds("pdf", loc, "page")
}

var pdfPages = regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`)

func (pdfAdapter) Extent(path string) (Extent, error) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		return Extent{}, &Unsupported{"PDF metadata needs the pdfinfo command"}
	}
	b, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		return Extent{}, &Unsupported{fmt.Sprintf("PDF metadata could not be read: %v", err)}
	}
	m := pdfPages.FindSubmatch(b)
	if m == nil {
		return Extent{}, fmt.Errorf("PDF page count is unavailable")
	}
	n, _ := strconv.Atoi(string(m[1]))
	return Extent{Unit: "page", Total: n}, nil
}

func (p pdfAdapter) Search(path, query string, limit int) ([]Match, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return []Match{}, nil
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return nil, &Unsupported{"PDF search needs the pdftotext command"}
	}
	b, err := exec.Command("pdftotext", "-layout", path, "-").Output()
	if err != nil {
		return nil, &Unsupported{fmt.Sprintf("PDF text extraction failed: %v", err)}
	}
	pages := bytes.Split(b, []byte{'\f'})
	var out []Match
	for i, raw := range pages {
		text := string(raw)
		at := strings.Index(strings.ToLower(text), needle)
		if at >= 0 {
			page := i + 1
			out = append(out, Match{Locator: locator.Locator{Kind: "page", Value: strconv.Itoa(page)}, Heading: "Page " + strconv.Itoa(page), Excerpt: excerptAt(text, at, len(query))})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (pdfAdapter) Render(path string, loc locator.Locator, output string) error {
	if err := requireKinds("pdf", loc, "page"); err != nil {
		return err
	}
	from, to, _ := locator.PageRange(loc.Value)
	if from != to {
		return fmt.Errorf("render one PDF page at a time, not %s", loc.Value)
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return &Unsupported{"PDF page rendering needs the pdftoppm command"}
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	prefix := strings.TrimSuffix(output, filepath.Ext(output))
	cmd := exec.Command("pdftoppm", "-f", strconv.Itoa(from), "-l", strconv.Itoa(from), "-singlefile", "-png", "-r", "144", path, prefix)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render PDF page %d: %v: %s", from, err, strings.TrimSpace(string(b)))
	}
	generated := prefix + ".png"
	if generated != output {
		return os.Rename(generated, output)
	}
	return nil
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
