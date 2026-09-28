package curriculum

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/source"
	"gopkg.in/yaml.v3"
)

// Resource is learning material that is not itself a curriculum: a
// supplementary book, a video course or a paper (CR-2026-026, CR-2026-027).
// It lives in Sources/<id>/resource.yaml; a curriculum's own material keeps
// its manifest.yaml.
type Resource struct {
	Version      int        `yaml:"version" json:"version"`
	ID           string     `yaml:"id" json:"id"`
	Title        string     `yaml:"title" json:"title"`
	Kind         string     `yaml:"kind" json:"kind"`
	Mode         string     `yaml:"mode" json:"mode"` // copy, link or none (external)
	OriginalName string     `yaml:"original_name,omitempty" json:"original_name,omitempty"`
	OriginalPath string     `yaml:"original_path,omitempty" json:"original_path,omitempty"`
	URL          string     `yaml:"url,omitempty" json:"url,omitempty"`
	Note         string     `yaml:"note,omitempty" json:"note,omitempty"`
	SHA256       string     `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	ImportedAt   string     `yaml:"imported_at" json:"imported_at"`
	Revisions    []Revision `yaml:"revisions,omitempty" json:"revisions,omitempty"`
	Fetch        *FetchSpec `yaml:"fetch,omitempty" json:"fetch,omitempty"`
}

// ResourceOptions describes a resource to add.
type ResourceOptions struct {
	Path     string // file or folder; empty for an external resource
	ID       string
	Title    string
	External bool
	URL      string
	Note     string
	Mode     string // copy (default) or link
	Kind     string // "code" for a Git project; other formats are detected
	// Fetch options when Path is an http(s) URL.
	Sitemap  string
	Prefix   string
	MaxPages int
	Now      time.Time
}

// SourceRef is any readable source: a curriculum's primary material or a
// resource, with the adapter that reads it.
type SourceRef struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Kind    string      `json:"kind"`
	Primary bool        `json:"primary"`
	URL     string      `json:"url,omitempty"`
	Note    string      `json:"note,omitempty"`
	Caps    source.Caps `json:"capabilities"`
	// Commit is the current commit of a code project.
	Commit  string         `json:"commit,omitempty"`
	Path    string         `json:"-"`
	Adapter source.Adapter `json:"-"`
}

func resourcePath(root, id string) string { return filepath.Join(root, "Sources", id, "resource.yaml") }

// AddResource stores a resource: a copy (or link) of the file with its hash,
// or, for an external resource, only its metadata.
func AddResource(root string, opts ResourceOptions) (Resource, error) {
	if !validID.MatchString(opts.ID) {
		return Resource{}, fmt.Errorf("invalid resource id %q: use lowercase letters, digits, and hyphens", opts.ID)
	}
	if _, err := os.Stat(filepath.Join(root, "Sources", opts.ID)); err == nil {
		return Resource{}, fmt.Errorf("%w: id %s", ErrAlreadyImported, opts.ID)
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	r := Resource{Version: 1, ID: opts.ID, Title: strings.TrimSpace(opts.Title), URL: opts.URL, Note: opts.Note,
		ImportedAt: opts.Now.UTC().Format(time.RFC3339)}
	if opts.External {
		if opts.Path != "" {
			return Resource{}, fmt.Errorf("an external resource has no file; drop the path or --external")
		}
		if r.Title == "" {
			return Resource{}, fmt.Errorf("an external resource needs --title")
		}
		r.Kind, r.Mode = "external", "none"
		return r, saveResource(root, r)
	}
	if IsURL(opts.Path) {
		spec := FetchSpec{URL: opts.Path, Sitemap: opts.Sitemap, Prefix: opts.Prefix, MaxPages: opts.MaxPages}
		rev, _, err := snapshotInto(root, opts.ID, spec, 1, opts.Now)
		if err != nil {
			os.RemoveAll(filepath.Join(root, "Sources", opts.ID))
			return Resource{}, err
		}
		if r.Title == "" {
			r.Title = opts.Path
		}
		r.Kind, r.Mode, r.URL, r.SHA256, r.Fetch, r.Revisions = "web", "copy", opts.Path, rev.SHA256, &spec, []Revision{rev}
		return r, saveResource(root, r)
	}
	path, err := filepath.Abs(opts.Path)
	if err != nil {
		return Resource{}, err
	}
	if opts.Kind == "code" {
		rev, err := source.ProjectRevision(path)
		if err != nil {
			return Resource{}, err
		}
		if r.Title == "" {
			r.Title = filepath.Base(path)
		}
		r.Kind, r.Mode, r.OriginalName, r.OriginalPath, r.SHA256 = "code", "link", filepath.Base(path), path, "git:"+rev.Commit
		r.Revisions = []Revision{{Commit: rev.Commit, Branch: rev.Branch, Dirty: rev.Dirty, CreatedAt: r.ImportedAt}}
		return r, saveResource(root, r)
	}
	if opts.Kind != "" {
		return Resource{}, fmt.Errorf("unknown --kind %q; only code needs to be given, other formats are detected", opts.Kind)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Resource{}, fmt.Errorf("inspect source: %w", err)
	}
	a, err := source.Detect(path, info)
	if err != nil {
		return Resource{}, err
	}
	if c, ok := a.(source.Checker); ok {
		if err := c.Check(path); err != nil {
			return Resource{}, err
		}
	}
	hash, err := fsutil.HashPath(path)
	if err != nil {
		return Resource{}, fmt.Errorf("hash source: %w", err)
	}
	r.Kind, r.SHA256, r.OriginalName = a.Kind(), hash, info.Name()
	if r.Title == "" {
		r.Title = strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
	}
	r.Mode = opts.Mode
	if r.Mode == "" {
		r.Mode = "copy"
	}
	switch r.Mode {
	case "link":
		r.OriginalPath = path
	case "copy":
		original := filepath.Join(root, "Sources", opts.ID, "original", info.Name())
		if info.IsDir() {
			err = copyDirectory(path, original)
		} else {
			err = fsutil.CopyFile(path, original)
		}
		if err != nil {
			os.RemoveAll(filepath.Join(root, "Sources", opts.ID))
			return Resource{}, err
		}
	default:
		return Resource{}, fmt.Errorf("invalid import mode %q", r.Mode)
	}
	return r, saveResource(root, r)
}

func saveResource(root string, r Resource) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(resourcePath(root, r.ID), data, 0o644)
}

// LoadResource reads a resource's metadata.
func LoadResource(root, id string) (Resource, error) {
	data, err := os.ReadFile(resourcePath(root, id))
	if err != nil {
		return Resource{}, fmt.Errorf("read resource %s: %w", id, err)
	}
	var r Resource
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Resource{}, fmt.Errorf("parse resource %s: %w", id, err)
	}
	return r, nil
}

// ListResources lists every resource that is not a curriculum.
func ListResources(root string) ([]Resource, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Sources"))
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	var out []Resource
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(resourcePath(root, e.Name())); err != nil {
			continue
		}
		r, err := LoadResource(root, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Resolve finds a source by ID: a curriculum's primary material or a resource.
func Resolve(root, id string) (SourceRef, error) {
	var ref SourceRef
	var mode, name, linked string
	var revs []Revision
	if m, err := LoadManifest(root, id); err == nil {
		ref = SourceRef{ID: id, Title: m.Title, Kind: m.Kind, Primary: true, URL: m.URL, Note: m.Note}
		mode, name, linked, revs = m.Mode, m.OriginalName, m.OriginalPath, m.Revisions
	} else if r, rerr := LoadResource(root, id); rerr == nil {
		ref = SourceRef{ID: id, Title: r.Title, Kind: r.Kind, URL: r.URL, Note: r.Note}
		mode, name, linked, revs = r.Mode, r.OriginalName, r.OriginalPath, r.Revisions
	} else {
		return ref, fmt.Errorf("no source or resource with id %q", id)
	}
	a, ok := source.ForKind(ref.Kind)
	if !ok {
		return ref, fmt.Errorf("source %s has unknown kind %q", id, ref.Kind)
	}
	ref.Adapter, ref.Caps = a, a.Capabilities()
	switch mode {
	case "copy":
		ref.Path = filepath.Join(root, "Sources", id, "original", name)
	case "link":
		ref.Path = linked
	}
	if n := len(revs); n > 0 {
		ref.Commit = revs[n-1].Commit
		if revs[n-1].Snapshot != "" {
			ref.Path = filepath.Join(root, "Sources", id, "original", revs[n-1].Snapshot)
		}
	}
	return ref, nil
}

// pin fixes unpinned code locators to the current commit.
func (s SourceRef) pin(loc locator.Locator) locator.Locator {
	if s.Kind == "code" {
		return loc.Pin(s.Commit)
	}
	return loc
}

// Validate checks a locator against the source it points into.
func (s SourceRef) Validate(loc locator.Locator) error { return s.Adapter.Validate(s.Path, s.pin(loc)) }

// Read extracts text at a locator.
// Code is read at the current commit unless the locator names one.
func (s SourceRef) Read(loc locator.Locator) (source.Content, error) {
	return s.Adapter.Read(s.Path, s.pin(loc))
}

// Outline derives the draft outline sections of a source.
func (s SourceRef) Outline() ([]source.Section, error) { return s.Adapter.Outline(s.Path) }

// RefreshResult reports a new revision of a source.
type RefreshResult struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Changed  bool     `json:"changed"`
	Previous Revision `json:"previous"`
	Current  Revision `json:"current"`
}

// Refresh records a new revision of a code project. Earlier records keep
// pointing at the commits they were pinned to.
func Refresh(root, id string, now time.Time) (RefreshResult, error) {
	ref, err := Resolve(root, id)
	if err != nil {
		return RefreshResult{}, err
	}
	if ref.Kind == "web" {
		return refreshWeb(root, id, ref, now)
	}
	if ref.Kind != "code" {
		return RefreshResult{}, fmt.Errorf("%s is %s material; refresh applies to code projects and web snapshots", id, ref.Kind)
	}
	rev, err := source.ProjectRevision(ref.Path)
	if err != nil {
		return RefreshResult{}, err
	}
	next := Revision{Commit: rev.Commit, Branch: rev.Branch, Dirty: rev.Dirty, CreatedAt: now.UTC().Format(time.RFC3339)}
	return updateRevisions(root, id, ref.Primary, next, func(a, b Revision) bool { return a.Commit == b.Commit })
}

// updateRevisions appends a revision to a manifest or resource unless it
// equals the current one.
func updateRevisions(root, id string, primary bool, next Revision, same func(a, b Revision) bool) (RefreshResult, error) {
	res := RefreshResult{ID: id, Current: next}
	if primary {
		m, err := LoadManifest(root, id)
		if err != nil {
			return res, err
		}
		res.Kind = m.Kind
		if n := len(m.Revisions); n > 0 {
			res.Previous = m.Revisions[n-1]
			if same(res.Previous, next) {
				res.Current = res.Previous
				return res, nil
			}
		}
		m.Revisions = append(m.Revisions, next)
		if next.Commit != "" {
			m.SHA256 = "git:" + next.Commit
		} else if next.SHA256 != "" {
			m.SHA256 = next.SHA256
		}
		data, err := yaml.Marshal(m)
		if err != nil {
			return res, err
		}
		res.Changed = true
		return res, fsutil.WriteFileAtomic(filepath.Join(root, "Sources", id, "manifest.yaml"), data, 0o644)
	}
	r, err := LoadResource(root, id)
	if err != nil {
		return res, err
	}
	res.Kind = r.Kind
	if n := len(r.Revisions); n > 0 {
		res.Previous = r.Revisions[n-1]
		if same(res.Previous, next) {
			res.Current = res.Previous
			return res, nil
		}
	}
	r.Revisions = append(r.Revisions, next)
	if next.Commit != "" {
		r.SHA256 = "git:" + next.Commit
	} else if next.SHA256 != "" {
		r.SHA256 = next.SHA256
	}
	res.Changed = true
	return res, saveResource(root, r)
}
