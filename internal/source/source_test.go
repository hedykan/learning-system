package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hedykan/learning-system/internal/locator"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func detect(t *testing.T, path string) Adapter {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Detect(path, info)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFolderOutlineUsesNaturalOrder(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"ch10", "ch2", "ch1"} {
		write(t, filepath.Join(dir, n+".md"), "# Chapter "+strings.TrimPrefix(n, "ch")+"\n\n## Part\n")
	}
	a := detect(t, dir)
	if a.Kind() != "directory" {
		t.Fatalf("kind = %s", a.Kind())
	}
	sections, err := a.Outline(dir)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, s := range sections {
		if s.Level == 1 {
			titles = append(titles, s.Title)
		}
	}
	if strings.Join(titles, ",") != "Chapter 1,Chapter 2,Chapter 10" {
		t.Fatalf("order = %v", titles)
	}
}

func TestMarkdownReadByAnchorAndLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.md")
	write(t, path, "# HTTP 缓存\n\nintro\n\n## 强缓存 Freshness\n\nmax-age ![图](fig.png)\n\n```\n# not a heading\n```\n\n## 协商缓存\n\nETag\n")
	a := detect(t, path)
	c, err := a.Read(path, locator.Locator{Kind: "anchor", Value: "#强缓存-freshness"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Text, "## 强缓存 Freshness\n") || !strings.Contains(c.Text, "# not a heading") || strings.Contains(c.Text, "ETag") {
		t.Fatalf("section text:\n%s", c.Text)
	}
	if len(c.Images) != 1 || c.Images[0] != "fig.png" {
		t.Fatalf("images = %v", c.Images)
	}
	whole, err := a.Read(path, locator.Locator{Kind: "anchor", Value: "#http-缓存"})
	if err != nil || !strings.Contains(whole.Text, "ETag") {
		t.Fatalf("chapter should include its sections: %v\n%s", err, whole.Text)
	}
	lines, err := a.Read(path, locator.Locator{Kind: "file", Value: "cache.md#L3-3"})
	if err != nil || lines.Text != "intro\n" {
		t.Fatalf("lines = %q, %v", lines.Text, err)
	}
	if a.Validate(path, locator.Locator{Kind: "anchor", Value: "#missing"}) == nil {
		t.Fatal("missing anchor accepted")
	}
	if a.Validate(path, locator.Locator{Kind: "page", Value: "3"}) == nil {
		t.Fatal("page locator accepted for markdown")
	}
}

func TestTextPdfAndExternal(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	write(t, txt, "a\nb\nc\n")
	a := detect(t, txt)
	if _, err := a.Outline(txt); !errors.Is(err, ErrNoStructure) {
		t.Fatalf("text outline err = %v", err)
	}
	if c, err := a.Read(txt, locator.Locator{Kind: "file", Value: "notes.txt#L2-3"}); err != nil || c.Text != "b\nc\n" {
		t.Fatalf("text read = %q, %v", c.Text, err)
	}
	pdf := filepath.Join(dir, "book.pdf")
	write(t, pdf, "%PDF-1.4")
	p := detect(t, pdf)
	var u *Unsupported
	if _, err := p.Read(pdf, locator.Locator{Kind: "page", Value: "1"}); !errors.As(err, &u) {
		t.Fatalf("pdf read err = %v", err)
	}
	ext, _ := ForKind("external")
	if !ext.Capabilities().External || ext.Validate("", locator.Locator{Kind: "time", Value: "3/05:20"}) != nil {
		t.Fatal("external should accept time locators")
	}
	if ext.Validate("", locator.Locator{Kind: "anchor", Value: "#x"}) == nil {
		t.Fatal("external accepted an anchor")
	}
	mobi := filepath.Join(dir, "book.mobi")
	write(t, mobi, "x")
	info, _ := os.Stat(mobi)
	if _, err := Detect(mobi, info); err == nil || !strings.Contains(err.Error(), "unsupported source format") {
		t.Fatalf("mobi err = %v", err)
	}
}

// fakeAdapter proves a new format needs only a new adapter.
type fakeAdapter struct{}

func (fakeAdapter) Kind() string { return "fake" }
func (fakeAdapter) Detect(path string, info fs.FileInfo) bool {
	return !info.IsDir() && extOf(path) == ".fake"
}
func (fakeAdapter) Capabilities() Caps { return Caps{Structured: true, Extractable: true} }
func (fakeAdapter) Outline(string) ([]Section, error) {
	return []Section{{Level: 1, Title: "Only", Anchor: "#only"}}, nil
}
func (fakeAdapter) Read(_ string, loc locator.Locator) (Content, error) {
	return Content{Locator: loc, Format: "markdown", Text: "fake text\n"}, nil
}
func (fakeAdapter) Validate(string, locator.Locator) error { return nil }

func TestRegisteredAdapterIsUsed(t *testing.T) {
	Register(fakeAdapter{})
	path := filepath.Join(t.TempDir(), "x.fake")
	write(t, path, "anything")
	a := detect(t, path)
	if a.Kind() != "fake" {
		t.Fatalf("kind = %s", a.Kind())
	}
	if byKind, ok := ForKind("fake"); !ok || byKind.Kind() != "fake" {
		t.Fatal("ForKind cannot find a registered adapter")
	}
	if c, err := a.Read(path, locator.Locator{Kind: "anchor", Value: "#only"}); err != nil || c.Text != "fake text\n" {
		t.Fatalf("read = %q, %v", c.Text, err)
	}
}
