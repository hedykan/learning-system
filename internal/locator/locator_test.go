package locator

import "testing"

func TestValidateAndLabel(t *testing.T) {
	good := map[Locator]string{
		{Kind: "page", Value: "42-45"}:                "第 42–45 页",
		{Kind: "page", Value: "7"}:                    "第 7–7 页",
		{Kind: "time", Value: "3/05:20-48:00"}:        "第 3 讲 05:20–48:00",
		{Kind: "time", Value: "1:02:03"}:              "1:02:03",
		{Kind: "file", Value: "src/raft.go#L120-180"}: "`src/raft.go` 第 120–180 行",
		{Kind: "file", Value: "src/raft.go"}:          "`src/raft.go`",
		{Kind: "anchor", Value: "#replication-lag"}:   "#replication-lag",
		{Kind: "chapter", Value: "ch05.xhtml#sec2"}:   "ch05.xhtml#sec2",
		{Kind: "text", Value: "讲义第二部分开头"}:             "讲义第二部分开头",
	}
	for l, label := range good {
		if err := l.Validate(); err != nil {
			t.Errorf("%+v: %v", l, err)
		}
		if got := l.Label("zh"); got != label {
			t.Errorf("%+v label = %q, want %q", l, got, label)
		}
	}
	if got := (Locator{Kind: "page", Value: "42-45"}).Label("en"); got != "pages 42–45" {
		t.Errorf("english label = %q", got)
	}
	bad := []Locator{{Kind: "page", Value: "5-3"}, {Kind: "page", Value: "0"}, {Kind: "time", Value: "5 min"},
		{Kind: "file", Value: "/etc/passwd"}, {Kind: "file", Value: "a/../../x"}, {Kind: "anchor", Value: "intro"},
		{Kind: "video", Value: "x"}, {Kind: "text", Value: " "}}
	for _, l := range bad {
		if l.Validate() == nil {
			t.Errorf("%+v accepted", l)
		}
	}
}

func TestWithin(t *testing.T) {
	cases := []struct {
		inner, outer   Locator
		inside, compar bool
	}{
		{Locator{Kind: "page", Value: "43-44"}, Locator{Kind: "page", Value: "40-50"}, true, true},
		{Locator{Kind: "page", Value: "49-52"}, Locator{Kind: "page", Value: "40-50"}, false, true},
		{Locator{Kind: "time", Value: "3/10:00-12:00"}, Locator{Kind: "time", Value: "3/05:20-48:00"}, true, true},
		{Locator{Kind: "time", Value: "4/10:00"}, Locator{Kind: "time", Value: "3/05:20-48:00"}, false, true},
		{Locator{Kind: "file", Value: "a.go#L5-9"}, Locator{Kind: "file", Value: "a.go"}, true, true},
		{Locator{Kind: "file", Value: "b.go#L5"}, Locator{Kind: "file", Value: "a.go"}, false, true},
		{Locator{Kind: "anchor", Value: "#a"}, Locator{Kind: "anchor", Value: "#b"}, false, false},
		{Locator{Kind: "page", Value: "3"}, Locator{Kind: "time", Value: "1/00:10"}, false, false},
		{Locator{Resource: "r2", Kind: "page", Value: "3"}, Locator{Kind: "page", Value: "1-9"}, false, false},
	}
	for _, c := range cases {
		inside, comparable := c.inner.Within(c.outer)
		if inside != c.inside || comparable != c.compar {
			t.Errorf("%+v within %+v = %t,%t", c.inner, c.outer, inside, comparable)
		}
	}
}
