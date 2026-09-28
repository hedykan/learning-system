package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/session"
)

// proposalView is a proposal with the learner's decision, if any.
type proposalView struct {
	*learner.Proposal
	Decision *curriculum.Decision `json:"decision,omitempty"`
}

func loadProposals(root, id string) ([]proposalView, error) {
	m, _, err := learner.Load(root)
	if err != nil {
		return nil, err
	}
	decisions, err := curriculum.LoadDecisions(root, id)
	if err != nil {
		return nil, err
	}
	out := []proposalView{}
	for _, p := range m.ProposalList(id) {
		v := proposalView{Proposal: p}
		if d, ok := decisions[p.ID]; ok {
			d := d
			v.Decision = &d
		}
		out = append(out, v)
	}
	return out, nil
}

func specOf(p *learner.Proposal) curriculum.ProposalSpec {
	return curriculum.ProposalSpec{ID: p.ID, Action: p.Action, Node: p.Node, Title: p.Title, Why: p.Why,
		Prerequisites: p.Prerequisites, Reason: p.Reason}
}

// proposalCommands list, accept and reject curriculum proposals (CR-2026-041).
func (a *App) proposalCommands(explicitVault *string) []*cobra.Command {
	var asJSON, all bool
	var curriculumID, reason string
	list := &cobra.Command{
		Use: "proposals [id]", Short: "List curriculum change proposals waiting for the learner", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			views, err := loadProposals(root, id)
			if err != nil {
				return err
			}
			shown := []proposalView{}
			for _, v := range views {
				if all || v.Decision == nil {
					shown = append(shown, v)
				}
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "proposals": shown})
			}
			if len(shown) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pending proposals.")
			}
			for _, v := range shown {
				state := "pending"
				if v.Decision != nil {
					state = v.Decision.Decision
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s %s\t%s\n", v.ID, state, v.Action, v.Node, v.Reason)
			}
			return nil
		},
	}
	list.Flags().BoolVar(&all, "all", false, "include decided proposals")
	list.Flags().BoolVar(&asJSON, "json", false, "output JSON")

	find := func(root, id, proposal string) (*learner.Proposal, error) {
		views, err := loadProposals(root, id)
		if err != nil {
			return nil, err
		}
		for _, v := range views {
			if v.ID == proposal {
				return v.Proposal, nil
			}
		}
		return nil, fmt.Errorf("curriculum %s has no proposal %q", id, proposal)
	}
	accept := &cobra.Command{
		Use: "accept <proposal>", Short: "Apply a proposal the learner agreed to", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, optional(curriculumID))
			if err != nil {
				return err
			}
			p, err := find(root, id, args[0])
			if err != nil {
				return err
			}
			active := ""
			if st, err := runtime.Load(root); err == nil && st.ActiveSession != nil {
				active = st.ActiveSession.ID
			}
			d, err := curriculum.Accept(root, id, specOf(p), active, a.Now())
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"proposal": p.ID, "decision": d})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Accepted %s: %s %s\n", p.ID, p.Action, p.Node)
			return nil
		},
	}
	accept.Flags().StringVar(&curriculumID, "curriculum", "", "curriculum id (default: the active one)")
	accept.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	reject := &cobra.Command{
		Use: "reject <proposal> --reason <why>", Short: "Record that the learner declined a proposal", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, optional(curriculumID))
			if err != nil {
				return err
			}
			if _, err := find(root, id, args[0]); err != nil {
				return err
			}
			if _, err := curriculum.Reject(root, id, args[0], reason, a.Now()); err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rejected %s\n", args[0])
			return nil
		},
	}
	reject.Flags().StringVar(&curriculumID, "curriculum", "", "curriculum id (default: the active one)")
	reject.Flags().StringVar(&reason, "reason", "", "why the learner declined (required)")
	return []*cobra.Command{list, accept, reject}
}
