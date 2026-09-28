package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/tags"
)

// GraphConfigPath is Obsidian's graph view settings file.
const GraphConfigPath = ".obsidian/graph.json"

// DefaultGraphFilter hides evidence, process and navigation pages and the
// imported material under Sources/, keeping knowledge notes and the
// learner's own notes (CR-2026-035).
const DefaultGraphFilter = "-tag:#learning/evidence -tag:#learning/process -tag:#learning/nav -path:Sources"

type graphColor struct {
	A   int `json:"a"`
	RGB int `json:"rgb"`
}

type colorGroup struct {
	Query string     `json:"query"`
	Color graphColor `json:"color"`
}

func defaultGraphConfig() []byte {
	cfg := map[string]any{
		"search":    DefaultGraphFilter,
		"showTags":  false,
		"showArrow": true,
		"colorGroups": []colorGroup{
			{"tag:#learning/state/stable", graphColor{1, 0x3fa34d}},
			{"tag:#learning/state/fragile", graphColor{1, 0xe8871e}},
			{"tag:#learning/state/developing", graphColor{1, 0x3b82c4}},
			{"tag:#" + tags.Question, graphColor{1, 0x8e5bd6}},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return append(data, '\n')
}

// ensureGraphConfig writes the default graph settings only when Obsidian has
// none yet; an existing file is never touched.
func ensureGraphConfig(root string, dryRun bool) (bool, error) {
	path := filepath.Join(root, filepath.FromSlash(GraphConfigPath))
	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		return false, err
	}
	if dryRun {
		return true, nil
	}
	return fsutil.WriteFileIfAbsent(path, defaultGraphConfig(), 0o644)
}

// untaggedConversations lists conversation files without the evidence tag.
func untaggedConversations(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Conversations"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := "Conversations/" + e.Name()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if _, changed := tags.Ensure(string(data), tags.Conversation); changed {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out, nil
}

// tagConversation adds the evidence tag to a conversation's frontmatter;
// the turns are left byte for byte.
func tagConversation(root, rel string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, changed := tags.Ensure(string(data), tags.Conversation)
	if !changed {
		return nil
	}
	return fsutil.WriteFileAtomic(path, []byte(text), 0o644)
}
