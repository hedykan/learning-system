package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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

// LastCommit returns the ISO time of HEAD, or "" when there is no commit.
func LastCommit(root string) string {
	out, err := run(root, "log", "-1", "--format=%cI")
	if err != nil {
		return ""
	}
	return out
}
