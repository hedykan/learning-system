// Package web snapshots web pages for learning (CR-2026-031). It fetches
// only when a command asks for it; afterwards everything reads the stored
// snapshot, fully offline. Politeness rules are enforced here, not left to
// the Agent: robots.txt, at least one second between pages, at most 500
// pages, and an honest User-Agent.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

// MaxPages is the hard limit of pages in one snapshot.
const MaxPages = 500

// Delay is the minimum pause between page requests. Tests shorten it.
var Delay = time.Second

// UserAgent identifies the Runtime to sites.
var UserAgent = "learn (Personal Learning OS; +https://github.com/hedykan/learning-system)"

// Options describes what to snapshot.
type Options struct {
	URL string
	// Sitemap is "" for a single page, "auto" to use the site's sitemap
	// (from robots.txt or /sitemap.xml), or a sitemap URL.
	Sitemap  string
	Prefix   string // only pages whose path starts with it (default "/")
	MaxPages int
	Client   *http.Client
}

// Page is one stored page.
type Page struct {
	URL       string `yaml:"url" json:"url"`
	Final     string `yaml:"final_url" json:"final_url"`
	Status    int    `yaml:"status" json:"status"`
	File      string `yaml:"file" json:"file"`
	SHA256    string `yaml:"sha256" json:"sha256"`
	FetchedAt string `yaml:"fetched_at" json:"fetched_at"`
}

// Result lists the pages stored and those skipped with the reason.
type Result struct {
	Pages   []Page            `yaml:"pages" json:"pages"`
	Skipped map[string]string `yaml:"skipped,omitempty" json:"skipped,omitempty"`
	SHA256  string            `yaml:"sha256" json:"sha256"` // over all pages, in order
}

// ErrNetwork marks a connection failure, typically an Agent sandbox
// without network access.
var ErrNetwork = errors.New("cannot reach the site")

// Snapshot fetches the pages into dir (created) and writes pages.yaml.
func Snapshot(opts Options, dir string) (Result, error) {
	res := Result{Skipped: map[string]string{}}
	start, err := url.Parse(opts.URL)
	if err != nil || (start.Scheme != "http" && start.Scheme != "https") || start.Host == "" {
		return res, fmt.Errorf("%q is not an http(s) URL", opts.URL)
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	limit := opts.MaxPages
	if limit <= 0 || limit > MaxPages {
		limit = MaxPages
	}
	prefix := opts.Prefix
	if prefix == "" {
		prefix = "/"
	}
	robots, err := loadRobots(client, start)
	if err != nil {
		return res, err
	}
	targets := []string{start.String()}
	if opts.Sitemap != "" {
		sitemap := opts.Sitemap
		if sitemap == "auto" {
			sitemap = start.Scheme + "://" + start.Host + "/sitemap.xml"
			if len(robots.sitemaps) > 0 {
				sitemap = robots.sitemaps[0]
			}
		}
		locs, err := sitemapURLs(client, sitemap, 0)
		if err != nil {
			return res, err
		}
		targets = nil
		seen := map[string]bool{}
		for _, loc := range locs {
			u, err := url.Parse(loc)
			if err != nil || u.Host != start.Host || !strings.HasPrefix(u.Path, prefix) || seen[u.String()] {
				continue
			}
			seen[u.String()] = true
			targets = append(targets, u.String())
		}
		if len(targets) == 0 {
			var examples []string
			for _, loc := range locs {
				if u, err := url.Parse(loc); err == nil && u.Host == start.Host && len(examples) < 3 {
					examples = append(examples, u.Path)
				}
			}
			if len(examples) == 0 {
				return res, fmt.Errorf("the sitemap lists no pages on %s", start.Host)
			}
			return res, fmt.Errorf("the sitemap lists no pages on %s under %s; its paths look like %s, so adjust or drop --prefix", start.Host, prefix, strings.Join(examples, ", "))
		}
	}
	if len(targets) > limit {
		for _, t := range targets[limit:] {
			res.Skipped[t] = fmt.Sprintf("over the limit of %d pages", limit)
		}
		targets = targets[:limit]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	all := sha256.New()
	for i, target := range targets {
		u, _ := url.Parse(target)
		if !robots.allowed(u.Path) {
			res.Skipped[target] = "disallowed by robots.txt"
			continue
		}
		if i > 0 {
			time.Sleep(Delay)
		}
		page, body, err := fetch(client, target)
		if err != nil {
			if errors.Is(err, ErrNetwork) {
				return res, err
			}
			res.Skipped[target] = err.Error()
			continue
		}
		page.File = fmt.Sprintf("%04d-%s.html", len(res.Pages)+1, pageName(u))
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, page.File), body, 0o644); err != nil {
			return res, err
		}
		all.Write([]byte(page.SHA256))
		res.Pages = append(res.Pages, page)
	}
	if len(res.Pages) == 0 {
		reasons := []string{}
		for u, why := range res.Skipped {
			reasons = append(reasons, u+": "+why)
		}
		return res, fmt.Errorf("no page could be stored (%s)", strings.Join(reasons, "; "))
	}
	res.SHA256 = hex.EncodeToString(all.Sum(nil))
	data, err := yaml.Marshal(res)
	if err != nil {
		return res, err
	}
	return res, fsutil.WriteFileAtomic(filepath.Join(dir, "pages.yaml"), data, 0o644)
}

func get(client *http.Client, target string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		var opErr *net.OpError
		var dnsErr *net.DNSError
		if errors.As(err, &opErr) || errors.As(err, &dnsErr) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return nil, fmt.Errorf("%w %s: %v", ErrNetwork, target, err)
		}
		return nil, err
	}
	return resp, nil
}

var scriptTag = regexp.MustCompile(`(?i)<script`)

func fetch(client *http.Client, target string) (Page, []byte, error) {
	resp, err := get(client, target)
	if err != nil {
		return Page{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Page{}, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") {
		return Page{}, nil, fmt.Errorf("not an HTML page (%s)", ct)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return Page{}, nil, err
	}
	if visibleText(body) < 200 && scriptTag.Match(body) {
		return Page{}, nil, fmt.Errorf("the page shows almost no text without JavaScript; save it from a browser or fetch it with your own tools")
	}
	sum := sha256.Sum256(body)
	return Page{URL: target, Final: resp.Request.URL.String(), Status: resp.StatusCode, SHA256: hex.EncodeToString(sum[:]),
		FetchedAt: time.Now().UTC().Format(time.RFC3339)}, body, nil
}

var tags = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)

func visibleText(body []byte) int {
	return len(strings.Join(strings.Fields(tags.ReplaceAllString(string(body), " ")), " "))
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func pageName(u *url.URL) string {
	name := strings.Trim(unsafeChars.ReplaceAllString(strings.Trim(u.Path, "/"), "-"), "-.")
	if name == "" {
		name = "index"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

// sitemapURLs reads a sitemap or sitemap index, following indexes once.
func sitemapURLs(client *http.Client, target string, depth int) ([]string, error) {
	resp, err := get(client, target)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap %s: HTTP %d", target, resp.StatusCode)
	}
	var doc struct {
		XMLName  xml.Name
		Sitemaps []string `xml:"sitemap>loc"`
		URLs     []string `xml:"url>loc"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("sitemap %s: %w", target, err)
	}
	out := doc.URLs
	if depth < 2 {
		for _, s := range doc.Sitemaps {
			child, err := sitemapURLs(client, strings.TrimSpace(s), depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, child...)
		}
	}
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out, nil
}

type robotsRules struct {
	rules    []rule
	sitemaps []string
}

type rule struct {
	allow bool
	path  string
}

// allowed applies the longest matching rule; Allow wins a tie.
func (r robotsRules) allowed(path string) bool {
	best, ok := -1, true
	for _, x := range r.rules {
		if x.path != "" && strings.HasPrefix(path, x.path) && (len(x.path) > best || (len(x.path) == best && x.allow)) {
			best, ok = len(x.path), x.allow
		}
	}
	return ok
}

// loadRobots reads the rules for all agents ("*") and for "learn"; a
// missing robots.txt allows everything.
func loadRobots(client *http.Client, start *url.URL) (robotsRules, error) {
	var r robotsRules
	resp, err := get(client, start.Scheme+"://"+start.Host+"/robots.txt")
	if err != nil {
		if errors.Is(err, ErrNetwork) {
			return r, err
		}
		return r, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return r, nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	applies, inGroup := false, false
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		switch key {
		case "user-agent":
			if !inGroup {
				applies = false
			}
			inGroup = true
			agent := strings.ToLower(value)
			applies = applies || agent == "*" || strings.HasPrefix(agent, "learn")
		case "allow", "disallow":
			inGroup = false
			if applies {
				r.rules = append(r.rules, rule{allow: key == "allow", path: value})
			}
		case "sitemap":
			r.sitemaps = append(r.sitemaps, value)
		}
	}
	return r, nil
}
