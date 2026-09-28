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
	Version      int    `yaml:"version" json:"version"`
	ID           string `yaml:"id" json:"id"`
	Title        string `yaml:"title" json:"title"`
	Kind         string `yaml:"kind" json:"kind"`
	Mode         string `yaml:"mode" json:"mode"` // copy, link or none (external)
	OriginalName string `yaml:"original_name,omitempty" json:"original_name,omitempty"`
	OriginalPath string `yaml:"original_path,omitempty" json:"original_path,omitempty"`
	URL          string `yaml:"url,omitempty" json:"url,omitempty"`
	Note         string `yaml:"note,omitempty" json:"note,omitempty"`
	SHA256       string `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	ImportedAt   string `yaml:"imported_at" json:"imported_at"`
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
	Now      time.Time
}

// SourceRef is any readable source: a curriculum's primary material or a
// resource, with the adapter that reads it.
type SourceRef struct {
	ID      string         `json:"id"`
	Title   string         `json:"title"`
	Kind    string         `json:"kind"`
	Primary bool           `json:"primary"`
	URL     string         `json:"url,omitempty"`
	Note    string         `json:"note,omitempty"`
	Caps    source.Caps    `json:"capabilities"`
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
	path, err := filepath.Abs(opts.Path)
	if err != nil {
		return Resource{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Resource{}, fmt.Errorf("inspect source: %w", err)
	}
	a, err := source.Detect(path, info)
	if err != nil {
		return Resource{}, err
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
	if m, err := LoadManifest(root, id); err == nil {
		ref = SourceRef{ID: id, Title: m.Title, Kind: m.Kind, Primary: true, URL: m.URL, Note: m.Note}
		mode, name, linked = m.Mode, m.OriginalName, m.OriginalPath
	} else if r, rerr := LoadResource(root, id); rerr == nil {
		ref = SourceRef{ID: id, Title: r.Title, Kind: r.Kind, URL: r.URL, Note: r.Note}
		mode, name, linked = r.Mode, r.OriginalName, r.OriginalPath
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
	return ref, nil
}

// Validate checks a locator against the source it points into.
func (s SourceRef) Validate(loc locator.Locator) error { return s.Adapter.Validate(s.Path, loc) }

// Read extracts text at a locator.
func (s SourceRef) Read(loc locator.Locator) (source.Content, error) {
	return s.Adapter.Read(s.Path, loc)
}

// Outline derives the draft outline sections of a source.
func (s SourceRef) Outline() ([]source.Section, error) { return s.Adapter.Outline(s.Path) }
