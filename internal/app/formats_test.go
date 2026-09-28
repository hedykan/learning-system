package app_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, _ := w.Create(name)
		fw.Write([]byte(body))
	}
	w.Close()
	f.Close()
}

func epubFiles(extra map[string]string) map[string]string {
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`,
		"content.opf": `<package><manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="a" href="a.xhtml"/></manifest>
<spine><itemref idref="a"/></spine></package>`,
		"nav.xhtml": `<html><body><nav epub:type="toc"><ol><li><a href="a.xhtml">一致性</a><ol><li><a href="a.xhtml#lin">线性一致性</a></li></ol></li></ol></nav></body></html>`,
		"a.xhtml":   `<html><body><h1>一致性</h1><h2 id="lin">线性一致性</h2><p>像只有一个副本。</p></body></html>`,
	}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

func TestImportEPUBBuildsDraftOutlineAndRefusesDRM(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	book := filepath.Join(t.TempDir(), "ddia.epub")
	writeZip(t, book, epubFiles(nil))
	c.run("", false, "curriculum", "import", book, "--id", "ddia", "--title", "DDIA", "--activate", "--yes")
	outline := c.run("", false, "curriculum", "outline", "show", "--json")
	if !strings.Contains(outline, `"status": "draft"`) || !strings.Contains(outline, "线性一致性") {
		t.Fatalf("epub draft outline: %s", outline)
	}
	if out := c.run("", false, "source", "read", "ddia", "chapter", "a.xhtml#lin", "--json"); !strings.Contains(out, "像只有一个副本。") {
		t.Fatalf("epub read: %s", out)
	}
	locked := filepath.Join(t.TempDir(), "locked.epub")
	writeZip(t, locked, epubFiles(map[string]string{"META-INF/rights.xml": "<rights/>"}))
	if out := c.run("", true, "curriculum", "import", locked, "--id", "locked", "--yes"); !strings.Contains(out, "DRM") {
		t.Fatalf("DRM import: %s", out)
	}
	if out := c.run("", true, "source", "add", locked, "--id", "locked"); !strings.Contains(out, "DRM") {
		t.Fatalf("DRM resource: %s", out)
	}
	broken := filepath.Join(t.TempDir(), "broken.docx")
	os.WriteFile(broken, []byte("not a zip"), 0o644)
	c.run("", false, "curriculum", "import", broken, "--id", "broken", "--yes")
	if out := c.run("", false, "source", "outline", "broken", "--json"); !strings.Contains(out, `"status": "no_structure"`) {
		t.Fatalf("broken docx outline: %s", out)
	}
}
