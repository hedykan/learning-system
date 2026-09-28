package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/intake"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/session"
)

// goalCommand shows and sets a curriculum's goal card (CR-2026-043).
func (a *App) goalCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Show or set the learning goal card of a curriculum"}
	var asJSON bool
	var file string
	show := &cobra.Command{
		Use: "show [id]", Short: "Show the goal card", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			g, ok, err := curriculum.LoadGoal(root, id)
			if err != nil {
				return err
			}
			if asJSON {
				out := map[string]any{"curriculum": id, "goal": nil}
				if ok {
					out["goal"] = g
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "No goal card yet.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Outcome: %s\n", g.Outcome.Text)
			for _, f := range g.Focus {
				fmt.Fprintf(cmd.OutOrStdout(), "Focus %s: %s\n", f.ID, f.Text)
			}
			return nil
		},
	}
	show.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	set := &cobra.Command{
		Use: "set [id]", Short: "Set the goal card from the intake interview (JSON; evidence must be the learner's words in the active session)", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			data, err := readInput(cmd, file)
			if err != nil {
				return fmt.Errorf("read goal card: %w", err)
			}
			var card curriculum.GoalCard
			dec := json.NewDecoder(strings.NewReader(string(data)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&card); err != nil {
				return fmt.Errorf("parse goal card: %w", err)
			}
			st, err := runtimeState.Load(root)
			if err != nil {
				return err
			}
			if st.ActiveSession == nil || st.ActiveSession.Curriculum != id {
				return fmt.Errorf("start a session of %s first: the goal card's evidence must come from the learner's words in it", id)
			}
			conv, err := os.ReadFile(conversation.Path(root, st.ActiveSession.ID))
			if err != nil {
				return err
			}
			if err := assessment.CheckGoalCard(card, string(conv)); err != nil {
				return err
			}
			g, err := curriculum.SaveGoal(root, id, card, "set", st.ActiveSession.ID, a.Now())
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "goal": g})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Goal card v%d saved for %s\n", g.Version, id)
			return nil
		},
	}
	set.Flags().StringVar(&file, "file", "", "goal card file, or - for stdin")
	set.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(show, set)
	return cmd
}

// outlineReviewCommand prints a draft for the learner to review (CR-2026-045).
func (a *App) outlineReviewCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "review [id]", Short: "Review the outline entry by entry: why, prerequisites, evidence, sources, goal coverage", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			rev, err := intake.Build(root, id, vaultLanguage(root))
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), rev)
			}
			w := cmd.OutOrStdout()
			if rev.Goal != nil {
				fmt.Fprintf(w, "Goal: %s\n", rev.Goal.Outcome.Text)
			}
			for _, e := range rev.Entries {
				fmt.Fprintf(w, "%s %s [%s]\n", e.ID, e.Title, e.Status)
				if e.Why != "" {
					fmt.Fprintf(w, "  why: %s\n", e.Why)
				}
				if len(e.Prerequisites) > 0 {
					fmt.Fprintf(w, "  prerequisites: %s\n", strings.Join(e.Prerequisites, ", "))
				}
				if len(e.Serves) > 0 {
					fmt.Fprintf(w, "  serves: %s\n", strings.Join(e.Serves, ", "))
				}
				for _, h := range e.Hints {
					fmt.Fprintf(w, "  known: %s %s %s\n", h.Concept, h.Finding, h.State)
				}
				for _, r := range e.Resources {
					fmt.Fprintf(w, "  source: %s %s %s\n", r.Title, r.Label, r.Why)
				}
				if e.Unsourced {
					fmt.Fprintln(w, "  no source yet")
				}
			}
			for _, f := range rev.Uncovered {
				fmt.Fprintf(w, "Uncovered focus: %s (%s)\n", f.ID, f.Text)
			}
			if len(rev.LikelyKnown) > 0 {
				fmt.Fprintf(w, "Likely known: %s\n", strings.Join(rev.LikelyKnown, ", "))
			}
			if rev.Blocking != "" {
				fmt.Fprintf(w, "Not ready: %s\n", rev.Blocking)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
