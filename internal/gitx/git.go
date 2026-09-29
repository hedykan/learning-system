// Package gitx keeps a Learning Vault under version control with go-git, a
// pure Go Git implementation: no git executable is needed, so the same code
// runs on desktops and inside mobile apps (CR-2026-048). The repository is a
// standard Git repository any client can use.
package gitx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/hedykan/learning-system/internal/fsutil"
)

// Init creates a repository whose default branch is main.
func Init(root string) error {
	_, err := git.PlainInitWithOptions(root, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	// Like `git init`, provide the local exclude file other tools append to.
	exclude := filepath.Join(root, ".git", "info", "exclude")
	if _, err := fsutil.WriteFileIfAbsent(exclude, []byte("# git ls-files --others --exclude-from=.git/info/exclude\n"), 0o644); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func open(root string) (*git.Repository, *git.Worktree, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, nil, fmt.Errorf("open git repository: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, fmt.Errorf("open git worktree: %w", err)
	}
	return repo, wt, nil
}

// changes returns the working tree status, ignored files excluded.
func changes(root string) (git.Status, error) {
	_, wt, err := open(root)
	if err != nil {
		return nil, err
	}
	st, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	return st, nil
}

// Status reports clean or dirty.
func Status(root string) (string, error) {
	st, err := changes(root)
	if err != nil {
		return "", err
	}
	if st.IsClean() {
		return "clean", nil
	}
	return "dirty", nil
}

// Uncommitted counts changed and untracked paths.
func Uncommitted(root string) (int, error) {
	st, err := changes(root)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, fs := range st {
		if fs.Worktree != git.Unmodified || fs.Staging != git.Unmodified {
			n++
		}
	}
	return n, nil
}

func CommitAll(root, message string) error {
	_, err := Commit(root, message)
	return err
}

// stageAll stages every change, deletions included, and reports whether
// anything differs from HEAD.
func stageAll(wt *git.Worktree) (bool, error) {
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	for _, fs := range st {
		if fs.Staging != git.Unmodified && fs.Staging != git.Untracked {
			return true, nil
		}
	}
	return false, nil
}

// Commit stages everything and commits, reporting whether a commit was made.
func Commit(root, message string) (bool, error) {
	repo, wt, err := open(root)
	if err != nil {
		return false, err
	}
	changed, err := stageAll(wt)
	if err != nil || !changed {
		return false, err
	}
	sig := signature(repo)
	if _, err := wt.Commit(message, &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		return false, fmt.Errorf("git commit: %w", err)
	}
	return true, nil
}

// AmendAll folds all current changes into the commit just made by Commit,
// keeping its message. Only use it right after this process committed.
func AmendAll(root string) error {
	repo, wt, err := open(root)
	if err != nil {
		return err
	}
	changed, err := stageAll(wt)
	if err != nil || !changed {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("git amend: %w", err)
	}
	last, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("git amend: %w", err)
	}
	sig := signature(repo)
	_, err = wt.Commit(last.Message, &git.CommitOptions{Amend: true, Author: &last.Author, Committer: sig})
	if err != nil {
		return fmt.Errorf("git amend: %w", err)
	}
	return nil
}

// LastCommit returns the ISO time of HEAD, or "" when there is no commit.
func LastCommit(root string) string {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		return ""
	}
	return c.Committer.When.Format(time.RFC3339)
}

// IsRepo reports whether root is itself a Git work tree.
func IsRepo(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

// Fallback identity when neither the environment nor any Git configuration
// names one, so a missing identity never blocks saving learning history.
const (
	FallbackName  = "Learning OS"
	FallbackEmail = "learn@localhost"
)

// signature picks the committer: GIT_AUTHOR_* / GIT_COMMITTER_* variables,
// the repository config, the user's global config, then the fallback.
func signature(repo *git.Repository) *object.Signature {
	name, email := os.Getenv("GIT_AUTHOR_NAME"), os.Getenv("GIT_AUTHOR_EMAIL")
	if name == "" {
		name = os.Getenv("GIT_COMMITTER_NAME")
	}
	if email == "" {
		email = os.Getenv("GIT_COMMITTER_EMAIL")
	}
	if name == "" || email == "" {
		for _, cfg := range configs(repo) {
			if name == "" {
				name = cfg.User.Name
			}
			if email == "" {
				email = cfg.User.Email
			}
		}
	}
	if name == "" {
		name = FallbackName
	}
	if email == "" {
		email = FallbackEmail
	}
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

func configs(repo *git.Repository) []*gitconfig.Config {
	var out []*gitconfig.Config
	if c, err := repo.Config(); err == nil {
		out = append(out, c)
	}
	if c, err := gitconfig.LoadConfig(gitconfig.GlobalScope); err == nil {
		out = append(out, c)
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = err // an unreadable global config only loses the identity
	}
	return out
}

// AutoResult is the outcome of the latest automatic commit (CR-2026-024).
type AutoResult struct {
	Result string `json:"result"` // committed, failed or none
	At     string `json:"at,omitempty"`
	Error  string `json:"error,omitempty"`
}

func autoPath(root string) string { return filepath.Join(root, ".learning", "runtime", "git.json") }

// LoadAuto reads the latest automatic commit outcome; "none" when unknown.
func LoadAuto(root string) AutoResult {
	var r AutoResult
	data, err := os.ReadFile(autoPath(root))
	if err != nil || json.Unmarshal(data, &r) != nil || r.Result == "" {
		return AutoResult{Result: "none"}
	}
	return r
}

// SaveAuto records an automatic commit outcome in a Git-ignored runtime file.
func SaveAuto(root string, r AutoResult) error {
	dir := filepath.Dir(autoPath(root))
	if _, err := fsutil.WriteFileIfAbsent(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0o644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(autoPath(root), append(data, '\n'), 0o644)
}
