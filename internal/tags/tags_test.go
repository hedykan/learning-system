package tags

import "testing"

func TestEnsure(t *testing.T) {
	cases := map[string]string{
		"---\nid: a\n---\n\nbody\n":             "---\nid: a\ntags:\n  - x/y\n---\n\nbody\n",
		"---\ntags:\n  - old\nid: a\n---\nbody": "---\ntags:\n  - x/y\n  - old\nid: a\n---\nbody",
		"---\nid: a\ntags:\n  - x/y\n---\nbody": "---\nid: a\ntags:\n  - x/y\n---\nbody",
		"no frontmatter\n":                      "no frontmatter\n",
		"---\nmytags: 1\n---\nbody":             "---\nmytags: 1\ntags:\n  - x/y\n---\nbody",
	}
	for in, want := range cases {
		if got, _ := Ensure(in, "x/y"); got != want {
			t.Errorf("Ensure(%q) = %q, want %q", in, got, want)
		}
	}
}
