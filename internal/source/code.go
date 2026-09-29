package source

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/hedykan/learning-system/internal/locator"
)

// codeAdapter reads a Git project at a pinned commit (CR-2026-030). It is
// chosen explicitly (`--kind code`): a cloned book is also a Git folder.
// Every read goes through `git show <commit>:<path>`, so later edits in the
// working tree never change what a record points to.
type codeAdapter struct{}

func (codeAdapter) Kind() string                      { return "code" }
func (codeAdapter) Detect(string, fs.FileInfo) bool   { return false }
func (codeAdapter) Capabilities() Caps                { return Caps{Extractable: true} }
func (codeAdapter) Outline(string) ([]Section, error) { return nil, ErrNoStructure }

func (codeAdapter) Read(repo string, loc locator.Locator) (Content, error) {
	data, err := codeFile(repo, loc)
	if err != nil {
		return Content{}, err
	}
	_, from, to := splitFile(strings.SplitN(loc.Value, "@", 2)[0])
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if from < 1 {
		from, to = 1, len(lines)
	}
	if from > len(lines) {
		return Content{}, fmt.Errorf("the file has %d lines; line %d does not exist", len(lines), from)
	}
	to = min(to, len(lines))
	rel, _, _ := splitFile(strings.SplitN(loc.Value, "@", 2)[0])
	text := "```" + codeLanguage(rel) + "\n" + strings.Join(lines[from-1:to], "\n") + "\n```\n"
	return Content{Locator: loc, Format: "markdown", Text: text, Images: []string{}}, nil
}

func (codeAdapter) Validate(repo string, loc locator.Locator) error {
	_, err := codeFile(repo, loc)
	return err
}

// codeFile returns a file's content at the locator's commit, refusing
// excluded and binary files.
func codeFile(repo string, loc locator.Locator) ([]byte, error) {
	if err := requireKinds("code", loc, "file"); err != nil {
		return nil, err
	}
	commit := loc.Commit()
	if commit == "" {
		return nil, fmt.Errorf("code locators must be pinned to a commit, e.g. %s@<commit>", loc.Value)
	}
	rel, _, _ := splitFile(strings.SplitN(loc.Value, "@", 2)[0])
	rel = path.Clean(strings.TrimPrefix(rel, "./"))
	if why := ExcludedPath(rel); why != "" {
		return nil, fmt.Errorf("%s is not readable: %s", rel, why)
	}
	missing := fmt.Errorf("%s does not exist at commit %s", rel, commit[:min(7, len(commit))])
	r, err := git.PlainOpen(repo)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}
	hash, err := r.ResolveRevision(plumbing.Revision(commit))
	if err != nil {
		return nil, missing
	}
	c, err := r.CommitObject(*hash)
	if err != nil {
		return nil, missing
	}
	file, err := c.File(rel)
	if err != nil {
		return nil, missing
	}
	if file.Size > 4<<20 {
		return nil, fmt.Errorf("%s is not readable: larger than 4 MB", rel)
	}
	if bin, err := file.IsBinary(); err == nil && bin {
		return nil, fmt.Errorf("%s is not readable: binary file", rel)
	}
	text, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return []byte(text), nil
}

var excludedDirs = map[string]string{
	".git": "Git internals", "node_modules": "dependencies", "vendor": "dependencies", ".venv": "dependencies",
	"venv": "dependencies", "__pycache__": "build output", "dist": "build output", "build": "build output",
	"target": "build output", "out": "build output", ".next": "build output", ".gradle": "build output",
}

var secretSuffixes = []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".kdbx", ".asc", ".gpg"}

var secretNames = map[string]bool{".npmrc": true, ".pypirc": true, ".netrc": true, ".git-credentials": true,
	"credentials": true, "credentials.json": true, "secrets.yaml": true, "secrets.yml": true, "secrets.json": true}

// ExcludedPath reports why a project path must not be read: dependency and
// build folders, and files that usually hold credentials. The rule is
// enforced by the Runtime, not left to the Agent.
func ExcludedPath(rel string) string {
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if why, ok := excludedDirs[p]; ok {
			return why + " (" + p + "/)"
		}
	}
	name := strings.ToLower(parts[len(parts)-1])
	switch {
	case name == ".env" || strings.HasPrefix(name, ".env."):
		return "may contain credentials"
	case strings.HasPrefix(name, "id_rsa") || strings.HasPrefix(name, "id_ed25519") || strings.HasPrefix(name, "id_ecdsa") || strings.HasPrefix(name, "id_dsa"):
		return "private key"
	case secretNames[name]:
		return "may contain credentials"
	}
	for _, s := range secretSuffixes {
		if strings.HasSuffix(name, s) {
			return "key or credential file"
		}
	}
	return ""
}

func codeLanguage(rel string) string {
	switch extOf(rel) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".sh":
		return "sh"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".sql":
		return "sql"
	}
	return ""
}

// GitRevision describes a project's current commit.
type GitRevision struct {
	Commit string `json:"commit" yaml:"commit"`
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Dirty  bool   `json:"dirty" yaml:"dirty"`
}

// ProjectRevision reads a Git project's HEAD, branch and working-tree state.
// root must be the top of the work tree.
func ProjectRevision(root string) (GitRevision, error) {
	r, err := git.PlainOpenWithOptions(root, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return GitRevision{}, fmt.Errorf("%s is not a Git project", root)
	}
	wt, err := r.Worktree()
	if err != nil {
		return GitRevision{}, fmt.Errorf("%s is not a Git work tree", root)
	}
	if top := wt.Filesystem.Root(); !samePath(top, root) {
		return GitRevision{}, fmt.Errorf("give the project root %s, not a folder inside it", top)
	}
	head, err := r.Head()
	if err != nil {
		return GitRevision{}, fmt.Errorf("the project has no commit yet")
	}
	rev := GitRevision{Commit: head.Hash().String(), Branch: "HEAD"}
	if head.Name().IsBranch() {
		rev.Branch = head.Name().Short()
	}
	if st, err := wt.Status(); err == nil {
		rev.Dirty = !st.IsClean()
	}
	return rev, nil
}
