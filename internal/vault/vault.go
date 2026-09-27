package vault

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/channelwill/learning-os/internal/assets"
	"github.com/channelwill/learning-os/internal/config"
	"github.com/channelwill/learning-os/internal/fsutil"
	"github.com/channelwill/learning-os/internal/gitx"
	runtimeState "github.com/channelwill/learning-os/internal/runtime"
	"gopkg.in/yaml.v3"
)

var RequiredDirectories = []string{
	"Conversations", "Sessions", "Sources", "Concepts", "Curriculum", "Profile", ".learning",
}

// LegacyDirectories were created by v0.1 but are never written since v0.1.2.
var LegacyDirectories = []string{"Questions", "Ideas", "Hypotheses", "Misconceptions", "Insights"}

type InitResult struct {
	Root      string   `json:"root"`
	Created   []string `json:"created"`
	Preserved []string `json:"preserved"`
	Git       string   `json:"git"`
}

func Init(path string, now time.Time) (InitResult, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return InitResult{}, fmt.Errorf("resolve vault path: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create vault root: %w", err)
	}
	result := InitResult{Root: root}
	for _, dir := range RequiredDirectories {
		full := filepath.Join(root, dir)
		if info, err := os.Stat(full); err == nil {
			if !info.IsDir() {
				return result, fmt.Errorf("required directory is a file: %s", dir)
			}
			result.Preserved = append(result.Preserved, dir+"/")
			continue
		} else if !os.IsNotExist(err) {
			return result, fmt.Errorf("inspect directory %s: %w", dir, err)
		}
		if err := os.MkdirAll(full, 0o755); err != nil {
			return result, fmt.Errorf("create directory %s: %w", dir, err)
		}
		result.Created = append(result.Created, dir+"/")
	}

	configData, err := yaml.Marshal(config.Default())
	if err != nil {
		return result, err
	}
	stateData, err := json.MarshalIndent(runtimeState.NewState(now), "", "  ")
	if err != nil {
		return result, err
	}
	baseFiles := map[string][]byte{
		".learning/config.yaml":    configData,
		".learning/state.json":     append(stateData, '\n'),
		".learning/schema-version": []byte("1\n"),
	}
	for rel, data := range baseFiles {
		if err := createAsset(root, rel, data, &result); err != nil {
			return result, err
		}
	}
	if err := EnsureTmp(root); err != nil {
		return result, err
	}
	if err := fs.WalkDir(assets.Vault, "vault", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := assets.Vault.ReadFile(path)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), "vault/")
		return createAsset(root, filepath.FromSlash(rel), data, &result)
	}); err != nil {
		return result, fmt.Errorf("write embedded assets: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		if err := gitx.Init(root); err != nil {
			return result, fmt.Errorf("initialize git: %w", err)
		}
		result.Git = "initialized"
	} else if err != nil {
		return result, fmt.Errorf("inspect git directory: %w", err)
	} else {
		result.Git = "preserved"
	}
	return result, nil
}

// EnsureTmp creates .learning/tmp, a Git-ignored scratch area for Agents.
func EnsureTmp(root string) error {
	_, err := fsutil.WriteFileIfAbsent(filepath.Join(root, ".learning", "tmp", ".gitignore"), []byte("*\n"), 0o644)
	return err
}

func createAsset(root, rel string, data []byte, result *InitResult) error {
	created, err := fsutil.WriteFileIfAbsent(filepath.Join(root, rel), data, 0o644)
	if err != nil {
		return fmt.Errorf("initialize %s: %w", rel, err)
	}
	if created {
		result.Created = append(result.Created, filepath.ToSlash(rel))
	} else {
		result.Preserved = append(result.Preserved, filepath.ToSlash(rel))
	}
	return nil
}

func Discover(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		marker := filepath.Join(current, ".learning", "schema-version")
		if info, err := os.Stat(marker); err == nil && info.Mode().IsRegular() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("no Learning Vault found from %s; run 'learn init' first", start)
}
