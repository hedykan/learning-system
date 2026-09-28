package source

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseHTML(name string, data []byte) ([]doc, error) {
	text, err := htmlToMarkdown(data)
	if err != nil {
		return nil, err
	}
	return []doc{markdownDoc(name, text)}, nil
}

// htmlToMarkdown converts the main content of a page to Markdown: headings
// keep their id as {#id}, and site chrome (nav, header, footer, aside,
// scripts) is dropped. Only formatting changes; nothing is summarized.
func htmlToMarkdown(data []byte) (string, error) {
	root, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	body := findFirst(root, atom.Main)
	if body == nil {
		body = findFirst(root, atom.Article)
	}
	if body == nil {
		body = findFirst(root, atom.Body)
	}
	if body == nil {
		body = root
	}
	w := &mdWriter{}
	w.block(body)
	return strings.TrimSpace(w.out.String()) + "\n", nil
}

func findFirst(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findFirst(c, a); f != nil {
			return f
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

type mdWriter struct {
	out  strings.Builder
	list int // nesting depth of lists
}

func (w *mdWriter) para(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	w.out.WriteString(s + "\n\n")
}

var skipped = map[atom.Atom]bool{atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Nav: true,
	atom.Footer: true, atom.Aside: true, atom.Form: true, atom.Button: true, atom.Template: true}

// isChrome reports site chrome: skipped elements, and headers without a
// heading (a page banner). E-books put chapter titles in <header>.
func isChrome(n *html.Node) bool {
	if skipped[n.DataAtom] {
		return true
	}
	if n.DataAtom != atom.Header {
		return false
	}
	for _, h := range []atom.Atom{atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6} {
		if findFirst(n, h) != nil {
			return false
		}
	}
	return true
}

// block writes block-level content.
func (w *mdWriter) block(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode:
			w.para(collapse(c.Data))
		case c.Type != html.ElementNode || isChrome(c):
		case headingLevel(c) > 0:
			title := strings.TrimSpace(collapse(textOf(c))) // headings are plain text
			if title == "" {
				continue
			}
			if id := attr(c, "id"); id != "" {
				title += " {#" + id + "}"
			}
			w.out.WriteString(strings.Repeat("#", headingLevel(c)) + " " + title + "\n\n")
		case c.DataAtom == atom.P:
			w.para(w.inline(c))
		case c.DataAtom == atom.Pre:
			w.out.WriteString("```\n" + strings.TrimRight(textOf(c), "\n") + "\n```\n\n")
		case c.DataAtom == atom.Ul || c.DataAtom == atom.Ol:
			w.listItems(c, c.DataAtom == atom.Ol)
		case c.DataAtom == atom.Blockquote:
			inner := &mdWriter{}
			inner.block(c)
			for _, line := range strings.Split(strings.TrimSpace(inner.out.String()), "\n") {
				w.out.WriteString("> " + line + "\n")
			}
			w.out.WriteString("\n")
		case c.DataAtom == atom.Table:
			w.table(c)
		case c.DataAtom == atom.Img:
			if src := attr(c, "src"); src != "" {
				w.para("![" + attr(c, "alt") + "](" + src + ")")
			}
		case c.DataAtom == atom.Hr:
			w.out.WriteString("---\n\n")
		case c.DataAtom == atom.Br:
			w.out.WriteString("\n")
		default:
			if isInlineOnly(c) {
				w.para(w.inline(c))
			} else {
				w.block(c)
			}
		}
	}
}

func headingLevel(n *html.Node) int {
	switch n.DataAtom {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

var blockAtoms = map[atom.Atom]bool{atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true, atom.Ul: true,
	atom.Ol: true, atom.Li: true, atom.Table: true, atom.Pre: true, atom.Blockquote: true, atom.H1: true, atom.H2: true,
	atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true, atom.Figure: true, atom.Main: true, atom.Dl: true, atom.Hr: true}

func isInlineOnly(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (blockAtoms[c.DataAtom] || !isInlineOnly(c)) {
			return false
		}
	}
	return true
}

func (w *mdWriter) listItems(list *html.Node, ordered bool) {
	i := 0
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.DataAtom != atom.Li {
			continue
		}
		i++
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", i)
		}
		indent := strings.Repeat("  ", w.list)
		var own, nested []*html.Node
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.DataAtom == atom.Ul || c.DataAtom == atom.Ol) {
				nested = append(nested, c)
			} else {
				own = append(own, c)
			}
		}
		holder := &html.Node{Type: html.ElementNode}
		for _, c := range own {
			holder.AppendChild(cloneNode(c))
		}
		w.out.WriteString(indent + marker + strings.TrimSpace(collapse(w.inline(holder))) + "\n")
		for _, n := range nested {
			w.list++
			w.listItems(n, n.DataAtom == atom.Ol)
			w.list--
		}
	}
	if w.list == 0 {
		w.out.WriteString("\n")
	}
}

func cloneNode(n *html.Node) *html.Node {
	c := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Attr: n.Attr}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		c.AppendChild(cloneNode(k))
	}
	return c
}

func (w *mdWriter) table(t *html.Node) {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.DataAtom == atom.Tr {
				var cells []string
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == html.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) {
						cells = append(cells, strings.ReplaceAll(strings.TrimSpace(collapse(w.inline(td))), "|", "\\|"))
					}
				}
				rows = append(rows, cells)
			} else {
				walk(c)
			}
		}
	}
	walk(t)
	if len(rows) == 0 {
		return
	}
	for i, r := range rows {
		w.out.WriteString("| " + strings.Join(r, " | ") + " |\n")
		if i == 0 {
			w.out.WriteString("|" + strings.Repeat(" --- |", len(r)) + "\n")
		}
	}
	w.out.WriteString("\n")
}

// inline renders inline content: text, emphasis, code, links and images.
func (w *mdWriter) inline(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode:
			b.WriteString(collapse(c.Data))
		case c.Type != html.ElementNode || isChrome(c):
		case c.DataAtom == atom.Br:
			b.WriteString("\n")
		case c.DataAtom == atom.Code:
			b.WriteString("`" + textOf(c) + "`")
		case c.DataAtom == atom.Strong || c.DataAtom == atom.B:
			b.WriteString("**" + strings.TrimSpace(w.inline(c)) + "**")
		case c.DataAtom == atom.Em || c.DataAtom == atom.I:
			b.WriteString("*" + strings.TrimSpace(w.inline(c)) + "*")
		case c.DataAtom == atom.A:
			text := strings.TrimSpace(w.inline(c))
			if href := attr(c, "href"); href != "" && text != "" && !strings.HasPrefix(href, "javascript:") {
				b.WriteString("[" + text + "](" + href + ")")
			} else {
				b.WriteString(text)
			}
		case c.DataAtom == atom.Img:
			if src := attr(c, "src"); src != "" {
				b.WriteString("![" + attr(c, "alt") + "](" + src + ")")
			}
		default:
			b.WriteString(w.inline(c))
		}
	}
	return b.String()
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// collapse folds whitespace runs to single spaces, as browsers render text.
func collapse(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		if s != "" {
			return " "
		}
		return ""
	}
	out := strings.Join(fields, " ")
	if strings.TrimLeft(s, " \t\n\r") != s {
		out = " " + out
	}
	if strings.TrimRight(s, " \t\n\r") != s {
		out += " "
	}
	return out
}
