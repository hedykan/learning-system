package gitx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hedykan/learning-system/internal/fsutil"
)

func run(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func Init(root string) error {
	_, err := run(root, "init", "--quiet")
	return err
}

func Status(root string) (string, error) {
	out, err := run(root, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "clean", nil
	}
	return "dirty", nil
}

func CommitAll(root, message string) error {
	_, err := Commit(root, message)
	return err
}

// Commit stages everything and commits, reporting whether a commit was made.
func Commit(root, message string) (bool, error) {
	if _, err := run(root, "add", "--all"); err != nil {
		return false, err
	}
	if out, err := run(root, "diff", "--cached", "--quiet"); err == nil && out == "" {
		return false, nil
	}
	if _, err := run(root, "commit", "--quiet", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

// AmendAll folds all current changes into the commit just made by Commit,
// keeping its message. Only use it right after this process committed.
func AmendAll(root string) error {
	if _, err := run(root, "add", "--all"); err != nil {
		return err
	}
	if out, err := run(root, "diff", "--cached", "--quiet"); err == nil && out == "" {
		return nil
	}
	_, err := run(root, "commit", "--quiet", "--amend", "--no-edit")
	return err
}

// LastCommit returns the ISO time of HEAD, or "" when there is no commit.
func LastCommit(root string) string {
	out, err := run(root, "log", "-1", "--format=%cI")
	if err != nil {
		return ""
	}
	return out
}

// IsRepo reports whether root is itself a Git work tree.
func IsRepo(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

// Uncommitted counts changed and untracked paths. It never takes the index
// lock, so it also works where a sandbox makes .git read-only.
func Uncommitted(root string) (int, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-C", root, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git status: %w", err)
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n, nil
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
