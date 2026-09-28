package source

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// epubBook is the parsed package of an EPUB: spine documents in reading
// order and the table of contents.
type epubBook struct {
	zip   *zip.ReadCloser
	base  string            // folder of the package document
	spine []string          // hrefs relative to base
	nav   string            // EPUB 3 navigation document href
	ncx   string            // EPUB 2 NCX href
	files map[string]string // href -> zip entry name
}

func openEPUB(p string) (*epubBook, error) {
	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("not a readable EPUB: %w", err)
	}
	b := &epubBook{zip: z, files: map[string]string{}}
	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := b.decode("META-INF/container.xml", &container); err != nil || len(container.Rootfiles) == 0 {
		z.Close()
		return nil, fmt.Errorf("EPUB has no package document")
	}
	opf := container.Rootfiles[0].FullPath
	b.base = path.Dir(opf)
	var pkg struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"manifest>item"`
		Spine struct {
			Toc   string `xml:"toc,attr"`
			Items []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}
	if err := b.decode(opf, &pkg); err != nil {
		z.Close()
		return nil, fmt.Errorf("EPUB package document: %w", err)
	}
	byID := map[string]string{}
	for _, it := range pkg.Items {
		byID[it.ID] = it.Href
		b.files[it.Href] = path.Join(b.base, it.Href)
		if strings.Contains(" "+it.Properties+" ", " nav ") {
			b.nav = it.Href
		}
		if it.ID == pkg.Spine.Toc || it.MediaType == "application/x-dtbncx+xml" {
			b.ncx = it.Href
		}
	}
	for _, ref := range pkg.Spine.Items {
		if href, ok := byID[ref.IDRef]; ok {
			b.spine = append(b.spine, href)
		}
	}
	return b, nil
}

func (b *epubBook) open(name string) (io.ReadCloser, error) {
	for _, f := range b.zip.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("EPUB entry %s is missing", name)
}

func (b *epubBook) read(href string) ([]byte, error) {
	name, ok := b.files[href]
	if !ok {
		name = path.Join(b.base, href)
	}
	r, err := b.open(name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 64<<20))
}

func (b *epubBook) decode(name string, v any) error {
	r, err := b.open(name)
	if err != nil {
		return err
	}
	defer r.Close()
	d := xml.NewDecoder(r)
	d.Strict = false
	return d.Decode(v)
}

// loadEPUB converts each spine document to Markdown; doc names are hrefs,
// which chapter locators use.
func loadEPUB(p string) ([]doc, error) {
	b, err := openEPUB(p)
	if err != nil {
		return nil, err
	}
	defer b.zip.Close()
	var docs []doc
	for _, href := range b.spine {
		data, err := b.read(href)
		if err != nil {
			return nil, err
		}
		text, err := htmlToMarkdown(data)
		if err != nil {
			return nil, fmt.Errorf("chapter %s: %w", href, err)
		}
		docs = append(docs, markdownDoc(href, text))
	}
	return docs, nil
}

// epubOutline reads the book's own table of contents: the EPUB 3 nav
// document, or else the EPUB 2 NCX.
func epubOutline(p string) ([]Section, error) {
	b, err := openEPUB(p)
	if err != nil {
		return nil, err
	}
	defer b.zip.Close()
	if b.nav != "" {
		if data, err := b.read(b.nav); err == nil {
			if s := navSections(data, path.Dir(b.nav)); len(s) > 0 {
				return s, nil
			}
		}
	}
	if b.ncx != "" {
		var ncx struct {
			Points []ncxPoint `xml:"navMap>navPoint"`
		}
		if err := b.decode(b.files[b.ncx], &ncx); err == nil {
			var out []Section
			walkNCX(ncx.Points, 1, path.Dir(b.ncx), &out)
			return out, nil
		}
	}
	return nil, ErrNoStructure
}

type ncxPoint struct {
	Label   string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

func walkNCX(points []ncxPoint, level int, dir string, out *[]Section) {
	for _, p := range points {
		*out = append(*out, tocSection(level, p.Label, p.Content.Src, dir))
		walkNCX(p.Points, level+1, dir, out)
	}
}

// navSections reads the toc <nav> of an EPUB 3 navigation document.
func navSections(data []byte, dir string) []Section {
	root, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil
	}
	var toc *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if toc != nil {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Nav {
			for _, a := range n.Attr {
				if strings.HasSuffix(a.Key, "type") && strings.Contains(a.Val, "toc") {
					toc = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	if toc == nil {
		return nil
	}
	var out []Section
	var walk func(list *html.Node, level int)
	walk = func(list *html.Node, level int) {
		for li := list.FirstChild; li != nil; li = li.NextSibling {
			if li.Type != html.ElementNode || li.DataAtom != atom.Li {
				continue
			}
			for c := li.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode {
					continue
				}
				switch c.DataAtom {
				case atom.A, atom.Span:
					out = append(out, tocSection(level, textOf(c), attr(c, "href"), dir))
				case atom.Ol, atom.Ul:
					walk(c, level+1)
				}
			}
		}
	}
	if list := findFirst(toc, atom.Ol); list != nil {
		walk(list, 1)
	}
	return out
}

func tocSection(level int, title, src, dir string) Section {
	title = strings.Join(strings.Fields(title), " ")
	href, frag, _ := strings.Cut(src, "#")
	if href != "" && dir != "." {
		href = path.Join(dir, href)
	}
	chapter := href
	anchor := "#" + Slug(title)
	if frag != "" {
		chapter += "#" + frag
		anchor = "#" + frag
	}
	return Section{Level: level, Title: title, Anchor: anchor, File: href, Chapter: chapter}
}

// fontObfuscation lists the only encryption an unprotected EPUB may use.
var fontObfuscation = map[string]bool{
	"http://www.idpf.org/2008/embedding": true,
	"http://ns.adobe.com/pdf/enc#RC":     true,
}

// checkEPUB refuses DRM-protected books: their text cannot be read.
func checkEPUB(p string) error {
	b, err := openEPUB(p)
	if err != nil {
		return err
	}
	defer b.zip.Close()
	for _, f := range b.zip.File {
		if f.Name == "META-INF/rights.xml" {
			return fmt.Errorf("this EPUB is DRM-protected; use a DRM-free copy of the same edition")
		}
	}
	var enc struct {
		Methods []struct {
			Algorithm string `xml:"Algorithm,attr"`
		} `xml:"EncryptedData>EncryptionMethod"`
	}
	if err := b.decode("META-INF/encryption.xml", &enc); err == nil {
		for _, m := range enc.Methods {
			if !fontObfuscation[m.Algorithm] {
				return fmt.Errorf("this EPUB is DRM-protected; use a DRM-free copy of the same edition")
			}
		}
	}
	return nil
}
