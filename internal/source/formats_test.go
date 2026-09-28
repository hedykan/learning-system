package source

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hedykan/learning-system/internal/locator"
)

func zipFile(t *testing.T, path string, files map[string]string, order ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	names := append([]string{}, order...)
	for n := range files {
		found := false
		for _, o := range order {
			found = found || o == n
		}
		if !found {
			names = append(names, n)
		}
	}
	for _, n := range names {
		fw, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(files[n]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func titles(t *testing.T, a Adapter, path string) string {
	t.Helper()
	s, err := a.Outline(path)
	if err != nil {
		t.Fatalf("outline: %v", err)
	}
	var parts []string
	for _, x := range s {
		parts = append(parts, strings.Repeat(">", x.Level)+x.Title)
	}
	return strings.Join(parts, " ")
}

func read(t *testing.T, a Adapter, path string, loc locator.Locator) string {
	t.Helper()
	c, err := a.Read(path, loc)
	if err != nil {
		t.Fatalf("read %+v: %v", loc, err)
	}
	return c.Text
}

const page = `<!doctype html><html><head><title>t</title><script>var x=1</script></head><body>
<nav><a href="/">Home</a> menu</nav>
<main>
<h1 id="replication">复制 Replication</h1>
<p>Leaders send <em>changes</em> to <code>followers</code>. See <a href="ch6.html">chapter 6</a>.</p>
<h2 id="lag">Replication lag</h2>
<ul><li>async<ul><li>nested</li></ul></li><li>sync</li></ul>
<pre>SELECT 1;
SELECT 2;</pre>
<table><tr><th>mode</th><th>risk</th></tr><tr><td>async</td><td>stale reads</td></tr></table>
<img src="fig1.png" alt="lag">
<h2>Consistency</h2><p>Read your writes.</p>
</main>
<footer>© site</footer></body></html>`

func TestHTML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ch5.html")
	write(t, p, page)
	a := detect(t, p)
	if got := titles(t, a, p); got != ">复制 Replication >>Replication lag >>Consistency" {
		t.Fatalf("outline = %s", got)
	}
	lag := read(t, a, p, locator.Locator{Kind: "anchor", Value: "#lag"})
	for _, want := range []string{"## Replication lag {#lag}", "- async\n  - nested\n- sync", "```\nSELECT 1;\nSELECT 2;\n```", "| mode | risk |", "![lag](fig1.png)"} {
		if !strings.Contains(lag, want) {
			t.Fatalf("section lacks %q:\n%s", want, lag)
		}
	}
	if strings.Contains(lag, "Read your writes") {
		t.Fatal("section ran into the next heading")
	}
	whole := read(t, a, p, locator.Locator{Kind: "anchor", Value: "#replication"})
	if strings.Contains(whole, "menu") || strings.Contains(whole, "© site") || strings.Contains(whole, "var x") {
		t.Fatalf("site chrome kept:\n%s", whole)
	}
	if !strings.Contains(whole, "Leaders send *changes* to `followers`. See [chapter 6](ch6.html).") {
		t.Fatalf("inline formatting:\n%s", whole)
	}
	if read(t, a, p, locator.Locator{Kind: "anchor", Value: "#consistency"}) == "" {
		t.Fatal("slug anchor failed")
	}
}

func epub(t *testing.T, dir string, extra map[string]string, epub2 bool) string {
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/text/ch1.xhtml":   `<html><body><h1 id="c1">Chapter One</h1><p>Intro text.</p><h2 id="s1">First part</h2><p>Part text.</p></body></html>`,
		"OEBPS/text/ch2.xhtml":   `<html><body><h1>Chapter Two</h1><h2 id="s2">Second part</h2><p>More text.</p><h2 id="s3">Third part</h2><p>End.</p></body></html>`,
	}
	nav := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	if epub2 {
		nav = `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
		files["OEBPS/toc.ncx"] = `<ncx><navMap>
<navPoint><navLabel><text>One (NCX)</text></navLabel><content src="text/ch1.xhtml"/>
  <navPoint><navLabel><text>First part</text></navLabel><content src="text/ch1.xhtml#s1"/></navPoint></navPoint>
<navPoint><navLabel><text>Two (NCX)</text></navLabel><content src="text/ch2.xhtml"/></navPoint></navMap></ncx>`
	} else {
		files["OEBPS/nav.xhtml"] = `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol>
<li><a href="text/ch1.xhtml">第一章</a><ol><li><a href="text/ch1.xhtml#s1">第一节</a></li></ol></li>
<li><a href="text/ch2.xhtml">第二章</a><ol><li><a href="text/ch2.xhtml#s2">第二节</a></li><li><a href="text/ch2.xhtml#s3">第三节</a></li></ol></li>
</ol></nav></body></html>`
	}
	files["OEBPS/content.opf"] = `<package><manifest>` + nav + `
<item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="c2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/></manifest>
<spine toc="ncx"><itemref idref="c1"/><itemref idref="c2"/></spine></package>`
	for k, v := range extra {
		files[k] = v
	}
	p := filepath.Join(dir, "book.epub")
	zipFile(t, p, files, "mimetype")
	return p
}

func TestEPUB(t *testing.T) {
	p := epub(t, t.TempDir(), nil, false)
	a := detect(t, p)
	if got := titles(t, a, p); got != ">第一章 >>第一节 >第二章 >>第二节 >>第三节" {
		t.Fatalf("nav outline = %s", got)
	}
	s, _ := a.Outline(p)
	if s[3].Chapter != "text/ch2.xhtml#s2" {
		t.Fatalf("chapter locator = %q", s[3].Chapter)
	}
	sec := read(t, a, p, locator.Locator{Kind: "chapter", Value: "text/ch2.xhtml#s2"})
	if !strings.Contains(sec, "More text.") || strings.Contains(sec, "End.") {
		t.Fatalf("chapter section:\n%s", sec)
	}
	if whole := read(t, a, p, locator.Locator{Kind: "chapter", Value: "text/ch1.xhtml"}); !strings.Contains(whole, "Part text.") {
		t.Fatalf("whole chapter:\n%s", whole)
	}
	if a.Validate(p, locator.Locator{Kind: "chapter", Value: "text/ch9.xhtml"}) == nil {
		t.Fatal("missing chapter accepted")
	}
	if err := a.(Checker).Check(p); err != nil {
		t.Fatalf("clean epub refused: %v", err)
	}

	p2 := epub(t, t.TempDir(), nil, true)
	if got := titles(t, a, p2); got != ">One (NCX) >>First part >Two (NCX)" {
		t.Fatalf("ncx outline = %s", got)
	}

	fonts := epub(t, t.TempDir(), map[string]string{"META-INF/encryption.xml": `<encryption><EncryptedData><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/></EncryptedData></encryption>`}, false)
	if err := a.(Checker).Check(fonts); err != nil {
		t.Fatalf("font obfuscation is not DRM: %v", err)
	}
	drm := epub(t, t.TempDir(), map[string]string{"META-INF/encryption.xml": `<encryption><EncryptedData><EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/></EncryptedData></encryption>`}, false)
	if err := a.(Checker).Check(drm); err == nil || !strings.Contains(err.Error(), "DRM") {
		t.Fatalf("DRM not detected: %v", err)
	}
}

func TestDOCXWithLocalizedStyleIDs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notes.docx")
	zipFile(t, p, map[string]string{
		"word/styles.xml": `<w:styles xmlns:w="w"><w:style w:styleId="1"><w:name w:val="heading 1"/></w:style><w:style w:styleId="2"><w:name w:val="heading 2"/></w:style></w:styles>`,
		"word/document.xml": `<w:document xmlns:w="w"><w:body>
<w:p><w:pPr><w:pStyle w:val="1"/></w:pPr><w:r><w:t>第一章 实数</w:t></w:r></w:p>
<w:p><w:r><w:t>实数的</w:t></w:r><w:r><w:t>完备性。</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="2"/></w:pPr><w:r><w:t>确界</w:t></w:r></w:p>
<w:tbl><w:tr><w:tc><w:p><w:r><w:t>上确界</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>sup</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
<w:p><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:r><w:t>极限</w:t></w:r></w:p></w:body></w:document>`,
	})
	a := detect(t, p)
	if got := titles(t, a, p); got != ">第一章 实数 >>确界 >>极限" {
		t.Fatalf("docx outline = %s", got)
	}
	sec := read(t, a, p, locator.Locator{Kind: "anchor", Value: "#确界"})
	if !strings.Contains(sec, "| 上确界 | sup |") {
		t.Fatalf("table lost:\n%s", sec)
	}
	if !strings.Contains(read(t, a, p, locator.Locator{Kind: "anchor", Value: "#第一章-实数"}), "实数的完备性。") {
		t.Fatal("split runs not joined")
	}
	bad := filepath.Join(t.TempDir(), "broken.docx")
	write(t, bad, "not a zip")
	if _, err := detect(t, bad).Outline(bad); !errors.Is(err, ErrNoStructure) {
		t.Fatalf("broken docx outline err = %v", err)
	}
}

func TestNotebookAndMarkupFormats(t *testing.T) {
	dir := t.TempDir()
	nb := filepath.Join(dir, "intro.ipynb")
	write(t, nb, `{"metadata":{"language_info":{"name":"python"}},"cells":[
{"cell_type":"markdown","source":["# Linear regression\n","\n","Fit a line."]},
{"cell_type":"code","source":"x = 1\nprint(x)","outputs":[{"text":["1\n"]}]},
{"cell_type":"markdown","source":"## Loss"}]}`)
	a := detect(t, nb)
	if got := titles(t, a, nb); got != ">Linear regression >>Loss" {
		t.Fatalf("notebook outline = %s", got)
	}
	if text := read(t, a, nb, locator.Locator{Kind: "anchor", Value: "#linear-regression"}); !strings.Contains(text, "```python\nx = 1\nprint(x)\n```\n\n```\n1\n```") {
		t.Fatalf("notebook code:\n%s", text)
	}

	cases := map[string]struct{ text, want string }{
		"book.tex": {"\\documentclass{book}\n\\chapter{Limits}\\label{ch:lim}\nText.\n% \\section{Commented}\n\\section{Epsilon $\\{N\\}$}\nMore.\n\\subsection{Deep}\n", ">Limits >>Epsilon $\\{N\\}$"},
		"doc.rst":  {"=====\nTitle\n=====\n\nIntro\n\nSection A\n---------\n\nText\n\nSection B\n---------\n", ">Title >>Section A >>Section B"},
		"doc.adoc": {"= Guide\n\n== Install\n\n----\n== not a heading\n----\n\n== Use\n", ">Guide >>Install >>Use"},
		"doc.org":  {"* Tasks\n** Read\n#+begin_src sh\n* not a heading\n#+end_src\n** Write\n", ">Tasks >>Read >>Write"},
	}
	for name, c := range cases {
		p := filepath.Join(dir, name)
		write(t, p, c.text)
		a := detect(t, p)
		if got := titles(t, a, p); got != c.want {
			t.Errorf("%s outline = %q, want %q", name, got, c.want)
		}
	}
	tex := filepath.Join(dir, "book.tex")
	if text := read(t, detect(t, tex), tex, locator.Locator{Kind: "anchor", Value: "#ch:lim"}); !strings.Contains(text, "More.") {
		t.Fatalf("latex label anchor:\n%s", text)
	}
	if text := read(t, detect(t, tex), tex, locator.Locator{Kind: "file", Value: "book.tex#L3-3"}); text != "Text.\n" {
		t.Fatalf("latex lines = %q", text)
	}
}

func TestMixedFolder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "ch10.md"), "# Ten\n")
	write(t, filepath.Join(dir, "ch2.html"), "<h1>Two</h1><h2 id='x'>Two A</h2><p>html body</p>")
	write(t, filepath.Join(dir, "ch1.rst"), "One\n===\n")
	write(t, filepath.Join(dir, "notes.txt"), "plain\n")
	a := detect(t, dir)
	if got := titles(t, a, dir); got != ">One >Two >>Two A >Ten" {
		t.Fatalf("mixed folder outline = %s", got)
	}
	if !strings.Contains(read(t, a, dir, locator.Locator{Kind: "anchor", Value: "#x"}), "html body") {
		t.Fatal("anchor in html file of a folder")
	}
	if read(t, a, dir, locator.Locator{Kind: "file", Value: "notes.txt#L1"}) != "plain\n" {
		t.Fatal("text file lines in a folder")
	}
}

func TestEbookHeaderKeepsTitleAndBannerIsDropped(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.html")
	write(t, p, `<body><header><a href="/">Site logo</a></header><section><header><h2 id="t">Getting Started</h2></header><p>Body.</p></section></body>`)
	text := read(t, detect(t, p), p, locator.Locator{Kind: "anchor", Value: "#t"})
	if !strings.Contains(text, "## Getting Started {#t}") || strings.Contains(text, "Site logo") {
		t.Fatalf("header handling:\n%s", text)
	}
}

func TestAmbiguousAnchorInFolderAsksForChapter(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "01-toc.md"), "# Contents\n\n## 6. Replication\n")
	write(t, filepath.Join(dir, "10-ch6.md"), "# 6 Replication\n\nLeaders and followers.\n")
	a := detect(t, dir)
	_, err := a.Read(dir, locator.Locator{Kind: "anchor", Value: "#6-replication"})
	if err == nil || !strings.Contains(err.Error(), "10-ch6.md#6-replication") {
		t.Fatalf("ambiguous anchor err = %v", err)
	}
	if text := read(t, a, dir, locator.Locator{Kind: "chapter", Value: "10-ch6.md#6-replication"}); !strings.Contains(text, "Leaders and followers.") {
		t.Fatalf("chapter in folder:\n%s", text)
	}
	s, _ := a.Outline(dir)
	if s[2].Chapter != "10-ch6.md#6-replication" {
		t.Fatalf("outline chapter = %q", s[2].Chapter)
	}
}
