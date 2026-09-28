package source

import (
	"regexp"
	"strings"
)

func splitLines(data []byte) []string {
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
}

var (
	adocHeading = regexp.MustCompile(`^(=+)\s+(.+?)\s*$`)
	orgHeading  = regexp.MustCompile(`^(\*+)\s+(.+?)\s*$`)
	texHeading  = regexp.MustCompile(`^\s*\\(part|chapter|section|subsection|subsubsection)\*?(?:\[[^\]]*\])?\{`)
	texLabel    = regexp.MustCompile(`\\label\{([^}]+)\}`)
)

// parseAsciiDoc: "= Title" is level 1, "== Section" level 2 and so on.
func parseAsciiDoc(name string, data []byte) ([]doc, error) {
	return []doc{prefixHeadings(name, splitLines(data), adocHeading, "----")}, nil
}

// parseOrg: "* Heading" is level 1, "** Sub" level 2.
func parseOrg(name string, data []byte) ([]doc, error) {
	return []doc{prefixHeadings(name, splitLines(data), orgHeading, "#+begin_src")}, nil
}

// prefixHeadings reads headings marked by repeated leading characters,
// skipping blocks delimited by fence (literal blocks, source blocks).
func prefixHeadings(name string, lines []string, pattern *regexp.Regexp, fence string) doc {
	d := doc{name: name, lines: lines}
	inBlock := false
	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, fence) || (fence == "#+begin_src" && strings.HasPrefix(lower, "#+end_src")) {
			inBlock = !inBlock
			continue
		}
		if m := pattern.FindStringSubmatch(line); m != nil && !inBlock {
			d.heads = append(d.heads, heading{level: len(m[1]), title: m[2], line: i})
		}
	}
	return d
}

// parseRST: a title is a line underlined (and optionally overlined) with a
// punctuation character; levels follow the order styles first appear in.
func parseRST(name string, data []byte) ([]doc, error) {
	lines := splitLines(data)
	d := doc{name: name, lines: lines}
	var styles []string
	level := func(style string) int {
		for i, s := range styles {
			if s == style {
				return i + 1
			}
		}
		styles = append(styles, style)
		return len(styles)
	}
	for i := 1; i < len(lines); i++ {
		title := strings.TrimSpace(lines[i-1])
		under := strings.TrimSpace(lines[i])
		ch, ok := adornment(under)
		if _, isAdorn := adornment(title); !ok || title == "" || isAdorn || len([]rune(under)) < len([]rune(title)) {
			continue
		}
		start, style := i-1, "u"+ch
		if i >= 2 && strings.TrimSpace(lines[i-2]) == under {
			start, style = i-2, "o"+ch
		}
		d.heads = append(d.heads, heading{level: level(style), title: title, line: start})
	}
	return []doc{d}, nil
}

// adornment reports whether s is a reStructuredText section line: at least
// two copies of one punctuation character.
func adornment(s string) (string, bool) {
	if len(s) < 2 || !strings.ContainsRune("=-~^\"'`#*+:.", rune(s[0])) {
		return "", false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return "", false
		}
	}
	return s[:1], true
}

var texRank = map[string]int{"part": 0, "chapter": 1, "section": 2, "subsection": 3, "subsubsection": 4}

// parseLaTeX reads sectioning commands; the highest level present becomes
// level 1. A \label right after the heading becomes an explicit anchor.
func parseLaTeX(name string, data []byte) ([]doc, error) {
	lines := splitLines(data)
	d := doc{name: name, lines: lines}
	top := 5
	type raw struct {
		rank int
		h    heading
	}
	var found []raw
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "%") {
			continue
		}
		m := texHeading.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		cmd := line[m[2]:m[3]]
		title, rest := balanced(line[m[1]:])
		h := heading{title: strings.TrimSpace(title), line: i}
		label := texLabel.FindStringSubmatch(rest)
		if label == nil && i+1 < len(lines) {
			label = texLabel.FindStringSubmatch(lines[i+1])
		}
		if label != nil {
			h.ids = []string{label[1]}
		}
		found = append(found, raw{texRank[cmd], h})
		if texRank[cmd] < top {
			top = texRank[cmd]
		}
	}
	for _, f := range found {
		f.h.level = f.rank - top + 1
		d.heads = append(d.heads, f.h)
	}
	return []doc{d}, nil
}

// balanced returns the text up to the brace closing an already opened one.
func balanced(s string) (inside, rest string) {
	depth := 1
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i], s[i+1:]
			}
		}
	}
	return s, ""
}
