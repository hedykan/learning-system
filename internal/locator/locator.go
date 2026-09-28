// Package locator says where in a resource something is (CR-2026-025): a
// page range, an anchor, a chapter, a file and line range, a time range in a
// video or free text. It replaces PDF-only page numbers.
package locator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hedykan/learning-system/internal/i18n"
)

// Locator points into one resource of a curriculum; Resource is empty for
// the curriculum's primary resource.
type Locator struct {
	Resource string `json:"resource,omitempty" yaml:"resource,omitempty"`
	Kind     string `json:"kind" yaml:"kind"`
	Value    string `json:"value" yaml:"value"`
}

// Kinds lists the supported locator kinds.
var Kinds = []string{"page", "anchor", "chapter", "file", "time", "text"}

var (
	pagePattern = regexp.MustCompile(`^(\d+)(?:-(\d+))?$`)
	timePattern = regexp.MustCompile(`^(?:(\d+)/)?(\d{1,2}(?::\d{2}){1,2})(?:-(\d{1,2}(?::\d{2}){1,2}))?$`)
	filePattern = regexp.MustCompile(`^([^#@]+?)(?:#L(\d+)(?:-(\d+))?)?(?:@([0-9a-fA-F]{7,40}))?$`)
)

// Commit returns the commit a file locator is pinned to, if any
// (`src/raft.go#L120-180@a1b2c3d`, CR-2026-030).
func (l Locator) Commit() string {
	if l.Kind != "file" {
		return ""
	}
	if m := filePattern.FindStringSubmatch(l.Value); m != nil {
		return m[4]
	}
	return ""
}

// Pin returns the file locator pinned to commit unless it already is.
func (l Locator) Pin(commit string) Locator {
	if l.Kind == "file" && l.Commit() == "" && commit != "" {
		l.Value += "@" + commit
	}
	return l
}

// FromPages converts legacy [start, end] pages.
func FromPages(pages []int) Locator {
	return Locator{Kind: "page", Value: fmt.Sprintf("%d-%d", pages[0], pages[1])}
}

// Validate checks the value's format for its kind.
func (l Locator) Validate() error {
	v := strings.TrimSpace(l.Value)
	if v == "" {
		return fmt.Errorf("locator %s needs a value", l.Kind)
	}
	switch l.Kind {
	case "page":
		if _, _, ok := pageRange(v); !ok {
			return fmt.Errorf("page locator %q must be N or N-M with 1 <= N <= M", v)
		}
	case "time":
		if _, _, _, ok := timeRange(v); !ok {
			return fmt.Errorf("time locator %q must be [episode/]mm:ss[-mm:ss], e.g. 3/05:20-48:00", v)
		}
	case "file":
		path, _, _, ok := fileRange(v)
		if !ok || strings.HasPrefix(path, "/") || strings.Contains("/"+path+"/", "/../") {
			return fmt.Errorf("file locator %q must be a relative path with optional #Lstart-end", v)
		}
	case "anchor":
		if !strings.HasPrefix(v, "#") || len(v) < 2 {
			return fmt.Errorf("anchor locator %q must start with #", v)
		}
	case "chapter":
	case "text":
		if utf8.RuneCountInString(v) > 200 {
			return fmt.Errorf("text locator must be at most 200 characters")
		}
	default:
		return fmt.Errorf("unsupported locator kind %q (use %s)", l.Kind, strings.Join(Kinds, ", "))
	}
	return nil
}

// Within reports whether l lies inside outer. comparable is false when the
// two cannot be compared by position (different kinds or resources, or
// anchor, chapter and text), in which case only formats are checked.
func (l Locator) Within(outer Locator) (inside, comparable bool) {
	if l.Kind != outer.Kind || l.Resource != outer.Resource {
		return false, false
	}
	switch l.Kind {
	case "page":
		a, b, _ := pageRange(l.Value)
		c, d, _ := pageRange(outer.Value)
		return a >= c && b <= d, true
	case "time":
		ea, a, b, _ := timeRange(l.Value)
		eb, c, d, _ := timeRange(outer.Value)
		if ea != eb {
			return false, true
		}
		return a >= c && b <= d, true
	case "file":
		pa, a, b, _ := fileRange(l.Value)
		pb, c, d, _ := fileRange(outer.Value)
		if pa != pb {
			return false, true
		}
		return a >= c && b <= d, true
	}
	return false, false
}

// Label renders the locator for learners, e.g. 第 42–45 页.
func (l Locator) Label(lang string) string {
	switch l.Kind {
	case "page":
		a, b, ok := pageRange(l.Value)
		if !ok {
			break
		}
		return i18n.F(lang, "第 %s 页", fmt.Sprintf("%d–%d", a, b))
	case "time":
		ep, a, b, ok := timeRange(l.Value)
		if !ok {
			break
		}
		span := clock(a)
		if b != a {
			span += "–" + clock(b)
		}
		if ep > 0 {
			return i18n.F(lang, "第 %d 讲 %s", ep, span)
		}
		return span
	case "file":
		path, a, b, ok := fileRange(l.Value)
		if !ok {
			break
		}
		pin := ""
		if c := l.Commit(); c != "" {
			pin = " @" + c[:min(7, len(c))]
		}
		if a == 0 && b == maxLine {
			return "`" + path + "`" + pin
		}
		return i18n.F(lang, "`%s` 第 %s 行", path, fmt.Sprintf("%d–%d", a, b)) + pin
	}
	return l.Value
}

func pageRange(v string) (int, int, bool) {
	m := pagePattern.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, false
	}
	a, _ := strconv.Atoi(m[1])
	b := a
	if m[2] != "" {
		b, _ = strconv.Atoi(m[2])
	}
	return a, b, a >= 1 && a <= b
}

func seconds(v string) int {
	total := 0
	for _, part := range strings.Split(v, ":") {
		n, _ := strconv.Atoi(part)
		total = total*60 + n
	}
	return total
}

func clock(s int) string {
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func timeRange(v string) (episode, start, end int, ok bool) {
	m := timePattern.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, false
	}
	if m[1] != "" {
		episode, _ = strconv.Atoi(m[1])
	}
	start = seconds(m[2])
	end = start
	if m[3] != "" {
		end = seconds(m[3])
	}
	return episode, start, end, start <= end
}

const maxLine = 1 << 30

func fileRange(v string) (path string, start, end int, ok bool) {
	m := filePattern.FindStringSubmatch(v)
	if m == nil {
		return "", 0, 0, false
	}
	path, start, end = m[1], 0, maxLine
	if m[2] != "" {
		start, _ = strconv.Atoi(m[2])
		end = start
		if m[3] != "" {
			end, _ = strconv.Atoi(m[3])
		}
	}
	return path, start, end, start <= end
}
