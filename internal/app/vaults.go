package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/clock"
	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/learner"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
)

type vaultCurriculum struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Kind      string `json:"kind"`
	Outline   string `json:"outline"`
	Completed int    `json:"completed"`
	Entries   int    `json:"entries"`
}

type vaultSession struct {
	Kind       string `json:"kind"`
	Curriculum string `json:"curriculum"`
}

type vaultSummary struct {
	Path          string            `json:"path"`
	Name          string            `json:"name"`
	Language      string            `json:"language,omitempty"`
	Active        string            `json:"active_curriculum,omitempty"`
	Curricula     []vaultCurriculum `json:"curricula"`
	ActiveSession *vaultSession     `json:"active_session,omitempty"`
	Suspended     *vaultSession     `json:"suspended_session,omitempty"`
	DueReviews    int               `json:"due_reviews"`
	Error         string            `json:"error,omitempty"`
}

func isVault(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".learning", "schema-version"))
	return err == nil
}

// findVaults lists vaults at dir and up to two levels below, without
// looking inside a vault for more vaults (CR-2026-047).
func findVaults(dir string, depth int) []string {
	if isVault(dir) {
		return []string{dir}
	}
	if depth == 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, findVaults(filepath.Join(dir, e.Name()), depth-1)...)
		}
	}
	return out
}

func summarizeVault(root string) vaultSummary {
	s := vaultSummary{Path: root, Name: filepath.Base(root), Curricula: []vaultCurriculum{}}
	cfg, err := config.Load(root)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.Language, s.Active = i18n.Normalize(cfg.Language), cfg.Curriculum.Active
	manifests, err := curriculum.List(root)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	m, _, err := learner.Load(root)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	for _, man := range manifests {
		c := vaultCurriculum{ID: man.ID, Title: man.Title, Kind: man.Kind}
		o, err := curriculum.LoadOutline(root, man.ID)
		if err == nil {
			c.Type, c.Outline = o.TypeOf(), o.Status
			entries, _ := curriculum.LoadProgress(root, man.ID)
			pos, _ := curriculum.LoadPosition(root, man.ID)
			statuses := curriculum.Statuses(o, entries, pos, m.NodesWithEvidence(man.ID))
			for _, st := range statuses {
				if hasChildStatus(statuses, st.ID) {
					continue
				}
				c.Entries++
				if st.Status == "completed" || st.Status == "skipped" {
					c.Completed++
				}
			}
		}
		s.Curricula = append(s.Curricula, c)
	}
	if st, err := runtimeState.Load(root); err == nil && st.ActiveSession != nil {
		s.ActiveSession = &vaultSession{Kind: st.ActiveSession.Kind, Curriculum: st.ActiveSession.Curriculum}
		if p := st.Suspended; p != nil {
			s.Suspended = &vaultSession{Kind: p.Kind, Curriculum: p.Curriculum}
		}
	}
	today := clock.Date(clock.Now())
	for _, sch := range m.Schedules() {
		if sch.Due <= today {
			s.DueReviews++
		}
	}
	return s
}

func hasChildStatus(statuses []curriculum.NodeStatus, id string) bool {
	for _, s := range statuses {
		if strings.HasPrefix(s.ID, id+".") {
			return true
		}
	}
	return false
}

func (a *App) vaultsCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "vaults [dir]", Short: "List the Learning Vaults in a folder (and two levels below); use --vault to work in one", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if info, err := os.Stat(abs); err != nil || !info.IsDir() {
				return fmt.Errorf("%s is not a folder", dir)
			}
			paths := findVaults(abs, 2)
			sort.Strings(paths)
			vaults := []vaultSummary{}
			for _, p := range paths {
				vaults = append(vaults, summarizeVault(p))
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"folder": abs, "vaults": vaults})
			}
			w := cmd.OutOrStdout()
			if len(vaults) == 0 {
				fmt.Fprintln(w, "No Learning Vault here. Create one with `learn init <path>`.")
			}
			for _, v := range vaults {
				fmt.Fprintf(w, "%s  %s\n", v.Name, v.Path)
				if v.Error != "" {
					fmt.Fprintf(w, "  error: %s\n", v.Error)
					continue
				}
				for _, c := range v.Curricula {
					mark := " "
					if c.ID == v.Active {
						mark = "*"
					}
					fmt.Fprintf(w, "  %s %s  %s  %d/%d\n", mark, c.ID, c.Title, c.Completed, c.Entries)
				}
				if v.ActiveSession != nil {
					fmt.Fprintf(w, "  active session: %s (%s)\n", v.ActiveSession.Kind, v.ActiveSession.Curriculum)
				}
				if v.Suspended != nil {
					fmt.Fprintf(w, "  suspended session: %s (%s)\n", v.Suspended.Kind, v.Suspended.Curriculum)
				}
				if v.DueReviews > 0 {
					fmt.Fprintf(w, "  due reviews: %d\n", v.DueReviews)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
