package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func init() { Delay = 0 }

func article(title string) string {
	return "<html><body><nav>menu</nav><main><h1>" + title + "</h1><p>" + strings.Repeat("Body text. ", 40) + "</p></main></body></html>"
}

func site(t *testing.T, robots string) *httptest.Server {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		if robots == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, robots)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<sitemapindex><sitemap><loc>%s/zh/sitemap.xml</loc></sitemap></sitemapindex>`, srv.URL)
	})
	mux.HandleFunc("/zh/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<urlset><url><loc>%[1]s/preface/</loc></url><url><loc>%[1]s/ch1/</loc></url><url><loc>%[1]s/ch2/</loc></url>
<url><loc>%[1]s/tw/ch1/</loc></url><url><loc>https://elsewhere.example/ch9/</loc></url><url><loc>%[1]s/private/x</loc></url></urlset>`, srv.URL)
	})
	mux.HandleFunc("/app/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><div id="root"></div><script src="/bundle.js"></script></body></html>`)
	})
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "learn") {
			t.Errorf("user agent %q", got)
		}
		fmt.Fprint(w, article("Page "+r.URL.Path))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSinglePage(t *testing.T) {
	srv := site(t, "")
	dir := filepath.Join(t.TempDir(), "snap")
	res, err := Snapshot(Options{URL: srv.URL + "/preface/"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pages) != 1 || res.Pages[0].File != "0001-preface.html" || res.SHA256 == "" {
		t.Fatalf("pages = %+v", res.Pages)
	}
	if _, err := os.Stat(filepath.Join(dir, "pages.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestSitemapIndexPrefixRobotsAndLimit(t *testing.T) {
	srv := site(t, "User-agent: *\nDisallow: /private/\nAllow: /private/ok\n\nUser-agent: badbot\nDisallow: /\n")
	res, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: "auto"}, filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, p := range res.Pages {
		files = append(files, p.File)
	}
	if strings.Join(files, ",") != "0001-preface.html,0002-ch1.html,0003-ch2.html,0004-tw-ch1.html" {
		t.Fatalf("files = %v", files)
	}
	if res.Skipped[srv.URL+"/private/x"] != "disallowed by robots.txt" {
		t.Fatalf("robots: %v", res.Skipped)
	}
	if _, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: srv.URL + "/zh/sitemap.xml", Prefix: "/zh/"}, filepath.Join(t.TempDir(), "e")); err == nil || !strings.Contains(err.Error(), "paths look like /preface/, /ch1/, /ch2/") {
		t.Fatalf("prefix hint: %v", err)
	}
	only, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: srv.URL + "/zh/sitemap.xml", Prefix: "/ch"}, filepath.Join(t.TempDir(), "b"))
	if err != nil || len(only.Pages) != 2 {
		t.Fatalf("prefix: %+v %v", only.Pages, err)
	}
	capped, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: "auto", MaxPages: 2}, filepath.Join(t.TempDir(), "c"))
	if err != nil || len(capped.Pages) != 2 || !strings.Contains(capped.Skipped[srv.URL+"/ch2/"], "limit") {
		t.Fatalf("cap: %+v %v", capped, err)
	}
	if _, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: "auto", MaxPages: 9999}, filepath.Join(t.TempDir(), "d")); err != nil {
		t.Fatal(err)
	}
}

func TestRefusals(t *testing.T) {
	srv := site(t, "User-agent: *\nDisallow: /\n")
	if _, err := Snapshot(Options{URL: srv.URL + "/ch1/"}, filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "robots") {
		t.Fatalf("robots-disallowed page stored: %v", err)
	}
	open := site(t, "")
	if _, err := Snapshot(Options{URL: open.URL + "/app/"}, filepath.Join(t.TempDir(), "y")); err == nil || !strings.Contains(err.Error(), "JavaScript") {
		t.Fatalf("script-only page: %v", err)
	}
	if _, err := Snapshot(Options{URL: open.URL + "/data.json"}, filepath.Join(t.TempDir(), "z")); err == nil || !strings.Contains(err.Error(), "not an HTML page") {
		t.Fatalf("json page: %v", err)
	}
	if _, err := Snapshot(Options{URL: "ftp://example.org/x"}, t.TempDir()); err == nil {
		t.Fatal("ftp accepted")
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	addr := closed.URL
	closed.Close()
	_, err := Snapshot(Options{URL: addr + "/"}, filepath.Join(t.TempDir(), "n"))
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("offline err = %v", err)
	}
}

func TestDelayBetweenPages(t *testing.T) {
	srv := site(t, "")
	Delay = 150 * time.Millisecond
	defer func() { Delay = 0 }()
	start := time.Now()
	res, err := Snapshot(Options{URL: srv.URL + "/", Sitemap: "auto", MaxPages: 3}, filepath.Join(t.TempDir(), "d"))
	if err != nil || len(res.Pages) != 3 {
		t.Fatal(err)
	}
	if time.Since(start) < 300*time.Millisecond {
		t.Fatal("pages were fetched without pausing")
	}
}
