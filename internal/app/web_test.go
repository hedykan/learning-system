package app_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSnapshotCurriculumReadsOffline(t *testing.T) {
	body := "<html><body><main><h1>复制</h1><p>" + strings.Repeat("领导者把变更发给追随者。", 30) + "</p><h2 id=\"lag\">复制延迟</h2><p>追随者可能落后。</p></main></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		fmt.Fprint(w, body)
	}))
	c := newCLI(t)
	c.run("", false, "init", c.root)
	if plan := c.run("", false, "curriculum", "import", srv.URL+"/ch5/", "--id", "ddia-web", "--dry-run", "--json"); !strings.Contains(plan, `"kind": "web"`) {
		t.Fatalf("dry run: %s", plan)
	}
	c.run("", false, "curriculum", "import", srv.URL+"/ch5/", "--id", "ddia-web", "--title", "DDIA 在线版", "--activate", "--yes")
	if outline := c.run("", false, "curriculum", "outline", "show", "--json"); !strings.Contains(outline, "复制延迟") {
		t.Fatalf("draft outline: %s", outline)
	}
	if again := c.run("", false, "source", "refresh", "ddia-web", "--json"); !strings.Contains(again, `"changed": false`) {
		t.Fatalf("unchanged refresh: %s", again)
	}
	srv.Close()
	if out := c.run("", false, "source", "read", "ddia-web", "anchor", "#lag", "--json"); !strings.Contains(out, "追随者可能落后。") {
		t.Fatalf("offline read: %s", out)
	}
	if out := c.run("", true, "source", "refresh", "ddia-web"); !strings.Contains(out, "network_access = true") {
		t.Fatalf("offline refresh should explain the sandbox: %s", out)
	}
	if out := c.run("", true, "source", "add", srv.URL+"/x/", "--id", "gone"); !strings.Contains(out, "normal terminal") {
		t.Fatalf("offline add: %s", out)
	}
	if list := c.run("", false, "source", "list", "--json"); strings.Contains(list, `"gone"`) {
		t.Fatal("failed fetch left a resource behind")
	}
}
