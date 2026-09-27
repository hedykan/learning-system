package vault

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/assets"
	"github.com/hedykan/learning-system/internal/fsutil"
)

type AgentAssetChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

type AgentAssetUpdate struct {
	Root      string             `json:"root"`
	DryRun    bool               `json:"dry_run"`
	Changes   []AgentAssetChange `json:"changes"`
	BackupDir string             `json:"backup_dir,omitempty"`
}

func UpdateAgentAssets(root string, dryRun, confirmed bool, now time.Time) (AgentAssetUpdate, error) {
	result := AgentAssetUpdate{Root: root, DryRun: dryRun}
	if !dryRun && !confirmed {
		return result, fmt.Errorf("agent asset update requires --yes after reviewing a dry run")
	}
	if !dryRun {
		if err := EnsureTmp(root); err != nil {
			return result, err
		}
	}
	assetPaths, err := embeddedAgentAssetPaths()
	if err != nil {
		return result, err
	}
	var changed []string
	for _, rel := range assetPaths {
		embedded, err := assets.Vault.ReadFile("vault/" + filepath.ToSlash(rel))
		if err != nil {
			return result, err
		}
		existing, err := os.ReadFile(filepath.Join(root, rel))
		action := "update"
		if os.IsNotExist(err) {
			action = "create"
		} else if err != nil {
			return result, fmt.Errorf("read agent asset %s: %w", rel, err)
		} else if bytes.Equal(existing, embedded) {
			continue
		}
		result.Changes = append(result.Changes, AgentAssetChange{Path: filepath.ToSlash(rel), Action: action})
		changed = append(changed, rel)
	}
	emptyLegacy := legacyDirectoryChanges(root, &result)
	if dryRun {
		return result, nil
	}
	for _, dir := range emptyLegacy {
		if err := removeIfEmpty(filepath.Join(root, dir)); err != nil {
			return result, err
		}
	}
	if len(changed) == 0 {
		return result, nil
	}
	backupRel := filepath.ToSlash(filepath.Join(".learning", "backups", "agent-assets-"+now.UTC().Format("20060102T150405.000000000Z")))
	for _, rel := range changed {
		destination := filepath.Join(root, rel)
		if existing, err := os.ReadFile(destination); err == nil {
			backup := filepath.Join(root, filepath.FromSlash(backupRel), rel)
			if err := fsutil.WriteFileAtomic(backup, existing, 0o644); err != nil {
				return result, fmt.Errorf("backup agent asset %s: %w", rel, err)
			}
		} else if !os.IsNotExist(err) {
			return result, err
		}
		embedded, err := assets.Vault.ReadFile("vault/" + filepath.ToSlash(rel))
		if err != nil {
			return result, err
		}
		if err := fsutil.WriteFileAtomic(destination, embedded, 0o644); err != nil {
			return result, fmt.Errorf("update agent asset %s: %w", rel, err)
		}
	}
	if err := excludeAgentBackupsFromGit(root); err != nil {
		return result, err
	}
	result.BackupDir = backupRel
	return result, nil
}

func excludeAgentBackupsFromGit(root string) error {
	path := filepath.Join(root, ".git", "info", "exclude")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Git exclude file: %w", err)
	}
	const pattern = ".learning/backups/"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, []byte(pattern+"\n")...)
	if err := fsutil.WriteFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("update Git exclude file: %w", err)
	}
	return nil
}

// legacyDirectoryChanges reports v0.1 directories: empty ones will be
// removed, ones holding user files are kept.
func legacyDirectoryChanges(root string, result *AgentAssetUpdate) []string {
	var empty []string
	for _, dir := range LegacyDirectories {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		userFiles := false
		for _, e := range entries {
			if e.Name() != ".DS_Store" {
				userFiles = true
			}
		}
		if userFiles {
			result.Changes = append(result.Changes, AgentAssetChange{Path: dir + "/", Action: "keep-dir-with-user-files"})
			continue
		}
		result.Changes = append(result.Changes, AgentAssetChange{Path: dir + "/", Action: "remove-empty-dir"})
		empty = append(empty, dir)
	}
	return empty
}

// removeIfEmpty re-checks emptiness right before deleting.
func removeIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.Name() != ".DS_Store" {
			return nil
		}
	}
	os.Remove(filepath.Join(dir, ".DS_Store"))
	return os.Remove(dir)
}

func embeddedAgentAssetPaths() ([]string, error) {
	var paths []string
	err := fs.WalkDir(assets.Vault, "vault", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), "vault/")
		if rel == "AGENTS.md" || rel == "CLAUDE.md" || strings.HasPrefix(rel, ".agents/") || strings.HasPrefix(rel, ".claude/") {
			paths = append(paths, filepath.FromSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}
