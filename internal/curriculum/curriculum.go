package curriculum

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var ErrAlreadyImported = errors.New("curriculum already imported")

type Manifest struct {
	Version      int    `yaml:"version" json:"version"`
	ID           string `yaml:"id" json:"id"`
	Title        string `yaml:"title" json:"title"`
	Kind         string `yaml:"kind" json:"kind"`
	Mode         string `yaml:"mode" json:"mode"`
	OriginalName string `yaml:"original_name" json:"original_name"`
	OriginalPath string `yaml:"original_path,omitempty" json:"original_path,omitempty"`
	SHA256       string `yaml:"sha256" json:"sha256"`
	ImportedAt   string `yaml:"imported_at" json:"imported_at"`
}

type Position struct {
	Book             string  `yaml:"book" json:"book"`
	Node             string  `yaml:"node,omitempty" json:"node,omitempty"`
	Chapter          string  `yaml:"chapter" json:"chapter"`
	Section          string  `yaml:"section" json:"section"`
	CurrentConcept   string  `yaml:"current_concept" json:"current_concept"`
	LastCompleted    string  `yaml:"last_completed" json:"last_completed"`
	NextTextbookStep string  `yaml:"next_textbook_step" json:"next_textbook_step"`
	Detour           *Detour `yaml:"detour" json:"detour"`
}

type Detour struct {
	ID              string      `yaml:"id,omitempty" json:"id,omitempty"`
	Type            string      `yaml:"type" json:"type"`
	Topic           string      `yaml:"topic" json:"topic"`
	Concept         string      `yaml:"concept,omitempty" json:"concept,omitempty"`
	Reason          string      `yaml:"reason" json:"reason"`
	ReturnCondition string      `yaml:"return_condition,omitempty" json:"return_condition,omitempty"`
	ReturnTo        ReturnPoint `yaml:"return_to" json:"return_to"`
	StartedSession  string      `yaml:"started_session,omitempty" json:"started_session,omitempty"`
	StartedAt       string      `yaml:"started_at,omitempty" json:"started_at,omitempty"`
}

type ReturnPoint struct {
	Chapter string `yaml:"chapter" json:"chapter"`
	Section string `yaml:"section" json:"section"`
	Concept string `yaml:"concept,omitempty" json:"concept,omitempty"`
}

type ImportOptions struct {
	SourcePath string
	ID         string
	Title      string
	Mode       string
	Activate   bool
	DryRun     bool
	Confirmed  bool
	Now        time.Time
}

type ImportPlan struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Mode        string `json:"mode"`
	SourcePath  string `json:"source_path"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	Activate    bool   `json:"activate"`
	Duplicate   string `json:"duplicate,omitempty"`
	ArchivedAs  string `json:"archived_match,omitempty"`
	DryRun      bool   `json:"dry_run"`
}

func Import(root string, opts ImportOptions) (ImportPlan, error) {
	if !validID.MatchString(opts.ID) {
		return ImportPlan{}, fmt.Errorf("invalid curriculum id %q: use lowercase letters, digits, and hyphens", opts.ID)
	}
	if opts.Mode == "" {
		opts.Mode = "copy"
	}
	if opts.Mode != "copy" && opts.Mode != "link" {
		return ImportPlan{}, fmt.Errorf("invalid import mode %q", opts.Mode)
	}
	source, err := filepath.Abs(opts.SourcePath)
	if err != nil {
		return ImportPlan{}, fmt.Errorf("resolve source path: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil {
		return ImportPlan{}, fmt.Errorf("inspect source: %w", err)
	}
	kind, err := sourceKind(source, info)
	if err != nil {
		return ImportPlan{}, err
	}
	hash, err := fsutil.HashPath(source)
	if err != nil {
		return ImportPlan{}, fmt.Errorf("hash source: %w", err)
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
	}
	plan := ImportPlan{
		ID: opts.ID, Title: title, Kind: kind, Mode: opts.Mode, SourcePath: source,
		Destination: filepath.Join(root, "Sources", opts.ID), SHA256: hash,
		Activate: opts.Activate, DryRun: opts.DryRun,
	}
	manifests, err := List(root)
	if err != nil {
		return plan, err
	}
	for _, manifest := range manifests {
		if manifest.SHA256 == hash {
			plan.Duplicate = manifest.ID
			break
		}
	}
	plan.ArchivedAs = archivedMatch(root, hash)
	if _, err := os.Stat(filepath.Join(root, "Sources", opts.ID)); err == nil {
		return plan, fmt.Errorf("%w: id %s", ErrAlreadyImported, opts.ID)
	} else if !os.IsNotExist(err) {
		return plan, fmt.Errorf("inspect import destination: %w", err)
	}
	if plan.Duplicate != "" {
		return plan, fmt.Errorf("%w: identical content is registered as %s", ErrAlreadyImported, plan.Duplicate)
	}
	if opts.DryRun {
		return plan, nil
	}
	if !opts.Confirmed {
		return plan, fmt.Errorf("import requires --yes after reviewing a dry run")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	manifest := Manifest{
		Version: 1, ID: opts.ID, Title: title, Kind: kind, Mode: opts.Mode,
		OriginalName: info.Name(), SHA256: hash, ImportedAt: opts.Now.UTC().Format(time.RFC3339),
	}
	if opts.Mode == "link" {
		manifest.OriginalPath = source
	}
	if err := writeSource(root, source, info, manifest); err != nil {
		return plan, err
	}
	if err := writeCurriculum(root, manifest, source); err != nil {
		return plan, err
	}
	if opts.Activate {
		if err := Activate(root, opts.ID); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func sourceKind(path string, info fs.FileInfo) (string, error) {
	if info.IsDir() {
		return "directory", nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("source must be a regular file or directory")
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "markdown", nil
	case ".txt":
		return "text", nil
	case ".pdf":
		return "pdf", nil
	default:
		return "", fmt.Errorf("unsupported source format %q; v0.1 supports Markdown, text, and PDF", filepath.Ext(path))
	}
}

func writeSource(root, source string, info fs.FileInfo, manifest Manifest) error {
	parent := filepath.Join(root, "Sources")
	tmp, err := os.MkdirTemp(parent, ".import-*")
	if err != nil {
		return fmt.Errorf("create import staging directory: %w", err)
	}
	defer os.RemoveAll(tmp)
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode source manifest: %w", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(tmp, "manifest.yaml"), data, 0o644); err != nil {
		return err
	}
	if manifest.Mode == "copy" {
		original := filepath.Join(tmp, "original")
		if info.IsDir() {
			if err := copyDirectory(source, filepath.Join(original, info.Name())); err != nil {
				return err
			}
		} else if err := fsutil.CopyFile(source, filepath.Join(original, info.Name())); err != nil {
			return err
		}
	}
	destination := filepath.Join(parent, manifest.ID)
	if err := os.Rename(tmp, destination); err != nil {
		return fmt.Errorf("commit source import: %w", err)
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported: %s", path)
		}
		return fsutil.CopyFile(path, target)
	})
}

func writeCurriculum(root string, manifest Manifest, source string) error {
	dir := filepath.Join(root, "Curriculum", manifest.ID)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return fmt.Errorf("create curriculum: %w", err)
	}
	book := fmt.Sprintf("---\nid: %s\ntitle: %q\nsource: %s\n---\n\n# %s\n\nImported learning curriculum.\n", manifest.ID, manifest.Title, manifest.ID, manifest.Title)
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "book.md"), []byte(book), 0o644); err != nil {
		return err
	}
	outline := Outline{Version: 2, Status: "missing"}
	if nodes, err := MarkdownOutline(source); err == nil && len(nodes) > 0 {
		outline = Outline{Version: 2, Status: "draft", Nodes: nodes}
		if outline.Validate() != nil {
			outline = Outline{Version: 2, Status: "missing"}
		}
	}
	if err := saveOutline(root, manifest.ID, outline); err != nil {
		return err
	}
	if err := SavePosition(root, manifest.ID, Position{Book: manifest.ID}); err != nil {
		return err
	}
	return RenderProgress(root, manifest.ID)
}

func List(root string) ([]Manifest, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Sources"))
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	var manifests []Manifest
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifest, err := LoadManifest(root, entry.Name())
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	return manifests, nil
}

func LoadManifest(root, id string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "Sources", id, "manifest.yaml"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read source manifest %s: %w", id, err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse source manifest %s: %w", id, err)
	}
	return manifest, nil
}

func Activate(root, id string) error {
	if _, err := LoadManifest(root, id); err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	cfg.Curriculum.Active = id
	return config.Save(root, cfg)
}

func LoadPosition(root, id string) (Position, error) {
	data, err := os.ReadFile(filepath.Join(root, "Curriculum", id, "current-position.md"))
	if err != nil {
		return Position{}, fmt.Errorf("read curriculum position: %w", err)
	}
	frontmatter, err := parseFrontmatter(data)
	if err != nil {
		return Position{}, err
	}
	var position Position
	if err := yaml.Unmarshal(frontmatter, &position); err != nil {
		return Position{}, fmt.Errorf("parse curriculum position: %w", err)
	}
	return position, nil
}

func SavePosition(root, id string, position Position) error {
	if position.Book == "" {
		position.Book = id
	}
	data, err := yaml.Marshal(position)
	if err != nil {
		return fmt.Errorf("encode curriculum position: %w", err)
	}
	content := append([]byte("---\n"), data...)
	content = append(content, []byte("---\n\n# Current Position\n\nManaged by `learn curriculum position`.\n")...)
	return fsutil.WriteFileAtomic(filepath.Join(root, "Curriculum", id, "current-position.md"), content, 0o644)
}

func parseFrontmatter(data []byte) ([]byte, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("unterminated YAML frontmatter")
	}
	return []byte(rest[:end]), nil
}
