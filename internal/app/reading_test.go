package app_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readerEPUB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "reader.epub")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package xmlns="http://www.idpf.org/2007/opf"><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml"><body><nav><ol><li><a href="ch1.xhtml#intro">Introduction</a></li><li><a href="ch2.xhtml#vectors">Vectors</a></li></ol></nav></body></html>`,
		"OEBPS/ch1.xhtml":        `<html><body><h1 id="intro">Introduction</h1><p>Limits describe approaching behavior.</p></body></html>`,
		"OEBPS/ch2.xhtml":        `<html><body><h1 id="vectors">Vectors</h1><p>A vector has magnitude and direction.</p></body></html>`,
	}
	for name, body := range files {
		entry, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = entry.Write([]byte(body)); e != nil {
			t.Fatal(e)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSourceReaderAndReadingProgressEndToEnd(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	book := filepath.Join(t.TempDir(), "reader.md")
	body := "# Start\n\nOpening.\n\n## Replication\n\nFollowers may lag behind leaders.\n\n## Conflicts\n\nConcurrent writes need reconciliation.\n"
	if err := os.WriteFile(book, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run("", false, "curriculum", "import", book, "--id", "reader", "--title", "Reader", "--activate", "--yes")

	if show := c.run("", false, "source", "show", "reader", "--json"); !strings.Contains(show, `"unit": "section"`) || !strings.Contains(show, `"total": 3`) {
		t.Fatalf("source show: %s", show)
	}
	read := c.run("", false, "source", "read", "reader", "anchor", "#replication", "--json")
	if !strings.Contains(read, "Followers may lag") || !strings.Contains(read, `"value": "#conflicts"`) {
		t.Fatalf("enhanced read: %s", read)
	}
	next := c.run("", false, "source", "navigate", "reader", "anchor", "#replication", "--next", "--json")
	if !strings.Contains(next, "Concurrent writes") || !strings.Contains(next, `"value": "#conflicts"`) {
		t.Fatalf("navigate: %s", next)
	}
	search := c.run("", false, "source", "search", "reader", "lag behind", "--json")
	if !strings.Contains(search, `"heading": "Replication"`) || !strings.Contains(search, `"value": "#replication"`) {
		t.Fatalf("search: %s", search)
	}

	c.run("", false, "source", "position", "set", "reader", "anchor", "#replication")
	if pos := c.run("", false, "source", "position", "show", "reader", "--json"); !strings.Contains(pos, `"value": "#replication"`) {
		t.Fatalf("position: %s", pos)
	}
	started := c.run("", false, "source", "reading", "start", "reader", "--mode", "self", "--json")
	start := strings.Index(started, `"id": "`)
	if start < 0 {
		t.Fatalf("reading start: %s", started)
	}
	start += len(`"id": "`)
	end := strings.Index(started[start:], `"`)
	sid := started[start : start+end]
	c.run("", false, "source", "reading", "progress", sid, "anchor", "#replication", "--active-seconds", "45", "--json")
	c.run("", false, "source", "reading", "end", sid, "--json")
	c.run("", false, "source", "reading", "cover", "reader", "anchor", "#conflicts", "--mode", "guided", "--active-seconds", "20", "--json")
	progress := c.run("", false, "source", "progress", "show", "reader", "--json")
	for _, want := range []string{`"unique_units": 2`, `"active_seconds": 65`, `"mode": "guided"`, `"mode": "self"`} {
		if !strings.Contains(progress, want) {
			t.Fatalf("progress lacks %s: %s", want, progress)
		}
	}
	context := c.run("", false, "source", "context", "reader", "anchor", "#replication", "--after", "1", "--curriculum", "reader", "--json")
	if !strings.Contains(context, "Followers may lag") || !strings.Contains(context, "Concurrent writes") || !strings.Contains(context, `"unique_units": 2`) {
		t.Fatalf("context: %s", context)
	}
	c.run("", false, "curriculum", "outline", "confirm")
	c.run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	c.run("Why can followers lag?", false, "session", "append", "--role", "user", "--source", "reader", "--locator-kind", "anchor", "--locator-value", "#replication", "--selection", "Followers may lag behind leaders.")
	c.run("Replication is asynchronous.", false, "session", "append", "--role", "assistant", "--citation", "reader:anchor:#replication")
	turns := c.run("", false, "session", "turns", "--json")
	for _, want := range []string{`"source_context":`, `"selection": "Followers may lag behind leaders."`, `"citations":`, `"value": "#replication"`} {
		if !strings.Contains(turns, want) {
			t.Fatalf("source-aware turns lack %s: %s", want, turns)
		}
	}
}

func TestEPUBReaderEndToEnd(t *testing.T) {
	c := newCLI(t)
	c.run("", false, "init", c.root)
	c.run("", false, "source", "add", readerEPUB(t), "--id", "epub-reader", "--title", "EPUB Reader", "--json")
	outline := c.run("", false, "source", "outline", "epub-reader", "--json")
	if !strings.Contains(outline, `"chapter": "ch1.xhtml#intro"`) || !strings.Contains(outline, `"chapter": "ch2.xhtml#vectors"`) {
		t.Fatalf("EPUB outline: %s", outline)
	}
	read := c.run("", false, "source", "read", "epub-reader", "chapter", "ch1.xhtml#intro", "--json")
	if !strings.Contains(read, "approaching behavior") || !strings.Contains(read, `"value": "ch2.xhtml#vectors"`) {
		t.Fatalf("EPUB read/navigation: %s", read)
	}
	search := c.run("", false, "source", "search", "epub-reader", "magnitude", "--json")
	if !strings.Contains(search, `"value": "ch2.xhtml#vectors"`) {
		t.Fatalf("EPUB search: %s", search)
	}
}
