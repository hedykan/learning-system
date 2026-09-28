package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/assessment"
	"github.com/hedykan/learning-system/internal/clock"
	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/gitx"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/learner"
	"github.com/hedykan/learning-system/internal/model"
	"github.com/hedykan/learning-system/internal/policy"
	"github.com/hedykan/learning-system/internal/projection"
	"github.com/hedykan/learning-system/internal/record"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/session"
	"github.com/hedykan/learning-system/internal/vault"
	"github.com/spf13/cobra"
)

const Version = "0.2.0"

type App struct {
	Out      io.Writer
	Err      io.Writer
	In       io.Reader
	Now      func() time.Time
	Provider model.Provider
}

func New() *App {
	return &App{
		Out: os.Stdout, Err: os.Stderr, In: os.Stdin,
		Now: clock.Now, Provider: model.ConservativeProvider{},
	}
}

func (a *App) RootCommand() *cobra.Command {
	var explicitVault string
	var verbose bool
	root := &cobra.Command{
		Use:           "learn",
		Short:         "Local-first Personal Learning OS",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	root.SetIn(a.In)
	root.PersistentFlags().StringVar(&explicitVault, "vault", "", "explicit Learning Vault path")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "print diagnostic details")
	root.AddCommand(a.versionCommand())
	root.AddCommand(a.initCommand())
	root.AddCommand(a.statusCommand(&explicitVault))
	root.AddCommand(a.curriculumCommand(&explicitVault))
	root.AddCommand(a.sessionCommand(&explicitVault))
	root.AddCommand(a.stateCommand(&explicitVault))
	root.AddCommand(a.agentCommand(&explicitVault))
	root.AddCommand(a.nextCommand(&explicitVault))
	root.AddCommand(a.modelCommand(&explicitVault))
	root.AddCommand(a.commitCommand(&explicitVault))
	root.AddCommand(a.reviewCommand(&explicitVault))
	root.AddCommand(a.configCommand(&explicitVault))
	root.AddCommand(a.sourceCommand(&explicitVault))
	_ = verbose
	return root
}

func (a *App) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print learn version",
		Run: func(cmd *cobra.Command, _ []string) { fmt.Fprintf(cmd.OutOrStdout(), "learn v%s\n", Version) },
	}
}

func (a *App) initCommand() *cobra.Command {
	var asJSON bool
	var language string
	cmd := &cobra.Command{
		Use: "init [path]", Short: "Initialize a Learning Vault", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			if language != "" && !i18n.Valid(language) {
				return fmt.Errorf("unsupported language %q; use one of %s", language, strings.Join(i18n.Languages, ", "))
			}
			result, err := vault.Init(path, a.Now())
			if err != nil {
				return err
			}
			if language != "" {
				if err := setLanguage(result.Root, language); err != nil {
					return err
				}
			}
			if err := session.Refresh(result.Root); err != nil {
				return fmt.Errorf("vault initialized, but writing the home page failed: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Learning Vault: %s\nCreated: %d\nPreserved: %d\nGit: %s\n", result.Root, len(result.Created), len(result.Preserved), result.Git)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&language, "language", "", "interface language of generated pages (zh or en)")
	return cmd
}

// vaultLanguage is the configured interface language of a Vault.
func vaultLanguage(root string) string {
	cfg, err := config.Load(root)
	if err != nil {
		return i18n.Default
	}
	return i18n.Normalize(cfg.Language)
}

func setLanguage(root, language string) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	cfg.Language = language
	return config.Save(root, cfg)
}

func (a *App) configCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Show or change Vault settings"}
	var asJSON bool
	show := &cobra.Command{
		Use: "show", Short: "Show Vault settings", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			out := map[string]any{"language": i18n.Normalize(cfg.Language), "git_enabled": cfg.Git.Enabled, "active_curriculum": cfg.Curriculum.Active}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "language: %s\ngit.enabled: %t\ncurriculum.active: %s\n", out["language"], cfg.Git.Enabled, cfg.Curriculum.Active)
			return nil
		},
	}
	show.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	set := &cobra.Command{
		Use: "set <key> <value>", Short: "Change a Vault setting (supported key: language)", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if args[0] != "language" {
				return fmt.Errorf("unknown setting %q; supported: language", args[0])
			}
			if !i18n.Valid(args[1]) {
				return fmt.Errorf("unsupported language %q; use one of %s", args[1], strings.Join(i18n.Languages, ", "))
			}
			if err := setLanguage(root, args[1]); err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return fmt.Errorf("language saved, but rebuilding pages failed (run 'learn model rebuild'): %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "language: %s (pages rebuilt)\n", args[1])
			return nil
		},
	}
	cmd.AddCommand(show, set)
	return cmd
}

type statusOutput struct {
	Vault           string                      `json:"vault"`
	ActiveSession   *runtimeState.ActiveSession `json:"active_session"`
	CurrentLearning string                      `json:"current_learning,omitempty"`
	Position        *curriculum.Position        `json:"position,omitempty"`
	Baseline        assessment.Status           `json:"baseline"`
	Detour          *curriculum.Detour          `json:"detour"`
	Outline         string                      `json:"outline"`
	PositionOK      bool                        `json:"position_verified"`
	Uncovered       []curriculum.Node           `json:"uncovered"`
	Partial         []curriculum.Node           `json:"partial"`
	GitLastCommit   string                      `json:"git_last_commit"`
	GitUncommitted  int                         `json:"git_uncommitted"`
	GitAutoCommit   string                      `json:"git_auto_commit"`
	Language        string                      `json:"language"`
	NodeResources   []curriculum.NodeResource   `json:"node_resources"`
	LastSession     string                      `json:"last_session,omitempty"`
	Projections     string                      `json:"projections"`
	Git             string                      `json:"git"`
}

func (a *App) statusCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "status", Short: "Show Learning Vault status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			state, err := runtimeState.Load(root)
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			out := statusOutput{Vault: root, ActiveSession: state.ActiveSession, CurrentLearning: cfg.Curriculum.Active, LastSession: state.LastSession}
			if cfg.Curriculum.Active != "" {
				position, err := curriculum.LoadPosition(root, cfg.Curriculum.Active)
				if err != nil {
					return err
				}
				out.Position = &position
				out.Detour = position.Detour
				outline, err := curriculum.LoadOutline(root, cfg.Curriculum.Active)
				if err != nil {
					return err
				}
				entries, err := curriculum.LoadProgress(root, cfg.Curriculum.Active)
				if err != nil {
					return err
				}
				out.Outline = outline.Status
				out.PositionOK = curriculum.PositionVerified(outline, position)
				m, _, err := learner.Load(root)
				if err != nil {
					return err
				}
				statuses := curriculum.Statuses(outline, entries, position, m.NodesWithEvidence(cfg.Curriculum.Active))
				for _, n := range curriculum.Uncovered(statuses) {
					out.Uncovered = append(out.Uncovered, n.Node)
				}
				for _, n := range curriculum.Partial(statuses) {
					out.Partial = append(out.Partial, n.Node)
				}
			}
			if out.Uncovered == nil {
				out.Uncovered = []curriculum.Node{}
			}
			if out.Partial == nil {
				out.Partial = []curriculum.Node{}
			}
			out.GitLastCommit = gitx.LastCommit(root)
			out.GitAutoCommit = gitx.LoadAuto(root).Result
			if n, err := gitx.Uncommitted(root); err == nil {
				out.GitUncommitted = n
			}
			out.Language = i18n.Normalize(cfg.Language)
			out.NodeResources = []curriculum.NodeResource{}
			if out.Position != nil && cfg.Curriculum.Active != "" {
				if res, err := curriculum.NodeResources(root, cfg.Curriculum.Active, out.Position.Node, out.Language); err == nil {
					out.NodeResources = res
				}
			}
			envs, err := record.LoadAll(root)
			if err != nil {
				return err
			}
			out.Projections = projection.Staleness(root, record.Generation(envs))
			out.Baseline, err = assessment.CurrentStatus(root, cfg.Curriculum.Active)
			if err != nil {
				return err
			}
			out.Git, err = gitx.Status(root)
			if err != nil {
				out.Git = "unavailable: " + err.Error()
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			writeHumanStatus(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func writeHumanStatus(w io.Writer, out statusOutput) {
	fmt.Fprintf(w, "Vault: %s\n\n", out.Vault)
	if out.ActiveSession == nil {
		fmt.Fprintln(w, "Active Session: none")
	} else {
		fmt.Fprintf(w, "Active Session: %s (%s)\n", out.ActiveSession.ID, valueOrNone(out.ActiveSession.Kind))
	}
	fmt.Fprintf(w, "Current Learning: %s\n", valueOrNone(out.CurrentLearning))
	if out.CurrentLearning != "" {
		fmt.Fprintf(w, "Outline: %s (position verified: %t)\n", valueOrNone(out.Outline), out.PositionOK)
		for _, n := range out.Uncovered {
			fmt.Fprintf(w, "Uncovered: %s\n", n.Label())
		}
		for _, n := range out.Partial {
			fmt.Fprintf(w, "Partly studied: %s\n", n.Label())
		}
	}
	if out.Position != nil {
		fmt.Fprintf(w, "Current Position: %s / %s\nCurrent Concept: %s\n", valueOrNone(out.Position.Chapter), valueOrNone(out.Position.Section), valueOrNone(out.Position.CurrentConcept))
	}
	if out.Detour != nil {
		fmt.Fprintf(w, "Detour: %s (return to %s / %s)\n", out.Detour.Topic, valueOrNone(out.Detour.ReturnTo.Chapter), valueOrNone(out.Detour.ReturnTo.Concept))
	}
	fmt.Fprintf(w, "Baseline: %s\nProjections: %s\n", out.Baseline.State, out.Projections)
	fmt.Fprintf(w, "Last Session: %s\nGit: %s (uncommitted: %d, last auto commit: %s)\nLanguage: %s\n", valueOrNone(out.LastSession), out.Git, out.GitUncommitted, out.GitAutoCommit, out.Language)
}

func (a *App) curriculumCommand(explicitVault *string) *cobra.Command {
	var parentJSON bool
	cmd := &cobra.Command{
		Use: "curriculum", Short: "Manage curriculum and learning position", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.showCurrentCurriculum(cmd, *explicitVault, parentJSON)
		},
	}
	cmd.Flags().BoolVar(&parentJSON, "json", false, "output JSON")
	cmd.AddCommand(a.curriculumListCommand(explicitVault))
	cmd.AddCommand(a.curriculumImportCommand(explicitVault))
	cmd.AddCommand(a.curriculumShowCommand(explicitVault))
	cmd.AddCommand(a.curriculumActivateCommand(explicitVault))
	cmd.AddCommand(a.curriculumPositionCommand(explicitVault))
	cmd.AddCommand(a.curriculumDetourCommand(explicitVault))
	cmd.AddCommand(a.curriculumOutlineCommand(explicitVault))
	cmd.AddCommand(a.curriculumLifecycleCommands(explicitVault)...)
	cmd.AddCommand(a.curriculumMarkCommand(explicitVault, "complete", "completed", "Mark an outline entry as completed"))
	cmd.AddCommand(a.curriculumMarkCommand(explicitVault, "skip", "skipped", "Mark an outline entry as deliberately skipped"))
	cmd.AddCommand(a.proposalCommands(explicitVault)...)
	return cmd
}

func (a *App) showCurrentCurriculum(cmd *cobra.Command, explicit string, asJSON bool) error {
	root, err := resolveVault(explicit)
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	if cfg.Curriculum.Active == "" {
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"active": "", "position": nil})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No active curriculum.")
		return nil
	}
	position, err := curriculum.LoadPosition(root, cfg.Curriculum.Active)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{"active": cfg.Curriculum.Active, "position": position})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Active: %s\nChapter: %s\nSection: %s\nConcept: %s\n", cfg.Curriculum.Active, valueOrNone(position.Chapter), valueOrNone(position.Section), valueOrNone(position.CurrentConcept))
	return nil
}

func (a *App) curriculumListCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "list", Short: "List imported curricula", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			items, err := curriculum.List(root)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), items)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No curricula imported.")
			}
			for _, item := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.ID, item.Title, item.Kind)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) curriculumImportCommand(explicitVault *string) *cobra.Command {
	var id, title, url, note, kind, sitemap, prefix, goal string
	var maxPages int
	var copyMode, linkMode, activate, dryRun, confirmed, asJSON, external bool
	cmd := &cobra.Command{
		Use: "import <path> | --external --title <name>", Short: "Import learning material, or register material without a file", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if copyMode && linkMode {
				return fmt.Errorf("--copy and --link are mutually exclusive")
			}
			if len(args) == 0 && !external && goal == "" {
				return fmt.Errorf("give the material's path, or --external for a video course, paper book or class")
			}
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			mode := "copy"
			if linkMode {
				mode = "link"
			}
			plan, err := curriculum.Import(root, curriculum.ImportOptions{
				SourcePath: path, External: external, URL: url, Note: note, Kind: kind, Goal: goal, Sitemap: sitemap, Prefix: prefix, MaxPages: maxPages, ID: id, Title: title, Mode: mode, Activate: activate,
				DryRun: dryRun, Confirmed: confirmed, Now: a.Now(),
			})
			if err != nil {
				if asJSON {
					_ = writeJSON(cmd.OutOrStdout(), map[string]any{"plan": plan, "error": err.Error()})
				}
				return err
			}
			if !dryRun {
				if err := session.Refresh(root); err != nil {
					return fmt.Errorf("imported, but refreshing the home page failed: %w", err)
				}
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), plan)
			}
			verb := "Imported"
			if dryRun {
				verb = "Import plan"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\nSHA-256: %s\nDestination: %s\n", verb, plan.Title, plan.Kind, plan.SHA256, plan.Destination)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "stable lowercase curriculum id (required)")
	cmd.Flags().StringVar(&title, "title", "", "human-readable title")
	cmd.Flags().BoolVar(&copyMode, "copy", false, "copy source into the Vault (default)")
	cmd.Flags().BoolVar(&linkMode, "link", false, "link to source outside the Vault")
	cmd.Flags().BoolVar(&activate, "activate", false, "make this the active curriculum")
	cmd.Flags().BoolVar(&external, "external", false, "material without a file: a video course, paper book or class")
	cmd.Flags().StringVar(&url, "url", "", "where the external material is (e.g. a playlist)")
	cmd.Flags().StringVar(&note, "note", "", "publication details of the external material")
	cmd.Flags().StringVar(&kind, "kind", "", "force a kind that is never detected: code (a Git project, linked and read at a commit)")
	cmd.Flags().StringVar(&goal, "goal", "", "build a curriculum from a learning goal instead of a material (needs --title)")
	addFetchFlags(cmd, &sitemap, &prefix, &maxPages)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and print the import plan without writing")
	cmd.Flags().BoolVar(&confirmed, "yes", false, "confirm the import plan")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func (a *App) curriculumShowCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "show <id>", Short: "Show an imported curriculum", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			manifest, err := curriculum.LoadManifest(root, args[0])
			if err != nil {
				return err
			}
			position, err := curriculum.LoadPosition(root, args[0])
			if err != nil {
				return err
			}
			out := map[string]any{"source": manifest, "position": position}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\nKind: %s\nMode: %s\nChapter: %s\nSection: %s\nConcept: %s\n", manifest.Title, manifest.ID, manifest.Kind, manifest.Mode, valueOrNone(position.Chapter), valueOrNone(position.Section), valueOrNone(position.CurrentConcept))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) curriculumActivateCommand(explicitVault *string) *cobra.Command {
	return &cobra.Command{
		Use: "activate <id>", Short: "Activate a curriculum", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if err := curriculum.Activate(root, args[0]); err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active curriculum: %s\n", args[0])
			return nil
		},
	}
}

func (a *App) curriculumPositionCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "position", Short: "Show or set the active curriculum position", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			if cfg.Curriculum.Active == "" {
				return fmt.Errorf("no active curriculum")
			}
			position, err := curriculum.LoadPosition(root, cfg.Curriculum.Active)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), position)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Chapter: %s\nSection: %s\nConcept: %s\n", valueOrNone(position.Chapter), valueOrNone(position.Section), valueOrNone(position.CurrentConcept))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(a.curriculumPositionSetCommand(explicitVault))
	cmd.AddCommand(a.curriculumPositionResetCommand(explicitVault))
	return cmd
}

func (a *App) curriculumPositionSetCommand(explicitVault *string) *cobra.Command {
	var chapter, section, concept, node string
	cmd := &cobra.Command{
		Use: "set", Short: "Set fields on the active curriculum position", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("chapter") && !cmd.Flags().Changed("section") && !cmd.Flags().Changed("concept") && !cmd.Flags().Changed("node") {
				return fmt.Errorf("set at least one of --node, --chapter, --section, or --concept")
			}
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			state, err := runtimeState.Load(root)
			if err != nil {
				return err
			}
			if state.ActiveSession != nil && state.ActiveSession.Kind == "baseline" {
				return fmt.Errorf("curriculum position is locked during a baseline assessment")
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			if cfg.Curriculum.Active == "" {
				return fmt.Errorf("no active curriculum")
			}
			position, err := curriculum.LoadPosition(root, cfg.Curriculum.Active)
			if err != nil {
				return err
			}
			outline, err := curriculum.LoadOutline(root, cfg.Curriculum.Active)
			if err != nil {
				return err
			}
			labels := cmd.Flags().Changed("chapter") || cmd.Flags().Changed("section")
			if cmd.Flags().Changed("node") {
				if labels {
					return fmt.Errorf("--node fills chapter and section from the outline; do not combine them")
				}
				if position, err = curriculum.PositionAtNode(outline, position, node); err != nil {
					return err
				}
			} else if labels && outline.Status == "confirmed" {
				return fmt.Errorf("the outline is confirmed; set the position with --node <id> (see 'learn curriculum outline show')")
			}
			if cmd.Flags().Changed("chapter") {
				position.Chapter, position.Node = chapter, ""
			}
			if cmd.Flags().Changed("section") {
				position.Section, position.Node = section, ""
			}
			if cmd.Flags().Changed("concept") {
				position.CurrentConcept = concept
			}
			if err := curriculum.SavePosition(root, cfg.Curriculum.Active, position); err != nil {
				return err
			}
			if position.Node != "" {
				if entries, err := curriculum.LoadProgress(root, cfg.Curriculum.Active); err == nil {
					for _, s := range curriculum.Statuses(outline, entries, curriculum.Position{}, nil) {
						if s.ID == position.Node && (s.Status == "completed" || s.Status == "skipped") {
							fmt.Fprintf(cmd.ErrOrStderr(), "Note: %s is already %s; this is a review. Run 'learn next' for the next unfinished entry.\n", s.Label(), s.Status)
						}
					}
				}
			}
			if err := curriculum.RenderProgress(root, cfg.Curriculum.Active); err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return fmt.Errorf("position updated, but refreshing projections failed: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Curriculum position updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&node, "node", "", "outline entry id, e.g. 2.3")
	cmd.Flags().StringVar(&chapter, "chapter", "", "chapter label")
	cmd.Flags().StringVar(&section, "section", "", "section label")
	cmd.Flags().StringVar(&concept, "concept", "", "current concept")
	return cmd
}

func (a *App) curriculumPositionResetCommand(explicitVault *string) *cobra.Command {
	return &cobra.Command{
		Use: "reset", Short: "Reset the active curriculum position", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			state, err := runtimeState.Load(root)
			if err != nil {
				return err
			}
			if state.ActiveSession != nil {
				return fmt.Errorf("cannot reset curriculum position while session %s is active", state.ActiveSession.ID)
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			if cfg.Curriculum.Active == "" {
				return fmt.Errorf("no active curriculum")
			}
			if err := curriculum.SavePosition(root, cfg.Curriculum.Active, curriculum.Position{Book: cfg.Curriculum.Active}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Curriculum position reset.")
			return nil
		},
	}
}

func (a *App) sessionCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Manage a learning session"}
	cmd.AddCommand(a.sessionStartCommand(explicitVault))
	cmd.AddCommand(a.sessionAppendCommand(explicitVault))
	cmd.AddCommand(a.sessionTurnsCommand(explicitVault))
	cmd.AddCommand(a.sessionCheckpointCommand(explicitVault))
	cmd.AddCommand(a.sessionEndCommand(explicitVault))
	cmd.AddCommand(a.sessionAnnotateCommand(explicitVault))
	cmd.AddCommand(a.sessionAbortCommand(explicitVault))
	return cmd
}

func (a *App) sessionStartCommand(explicitVault *string) *cobra.Command {
	var domain, kind, depth string
	var skipBaseline bool
	var asJSON bool
	cmd := &cobra.Command{
		Use: "start", Short: "Start a learning session", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			result, err := session.Start(root, session.StartOptions{Domain: domain, Kind: kind, Depth: depth, SkipBaseline: skipBaseline}, a.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Started %s (%s)\nConversation: %s\n", result.ID, result.Kind, result.Conversation)
			return nil
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "learning domain")
	cmd.Flags().StringVar(&kind, "kind", "", "session kind: baseline, lesson, review, or practice")
	cmd.Flags().StringVar(&depth, "depth", "", "baseline depth: quick, standard, or deep")
	cmd.Flags().BoolVar(&skipBaseline, "skip-baseline", false, "explicitly start a lesson without a baseline")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sessionAppendCommand(explicitVault *string) *cobra.Command {
	var role, text, file string
	var asJSON bool
	cmd := &cobra.Command{
		Use: "append", Short: "Append one raw conversation turn", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			content := text
			switch {
			case cmd.Flags().Changed("text") && file != "":
				return fmt.Errorf("use either --text or --file")
			case file != "":
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read conversation content: %w", err)
				}
				content = string(data)
			case !cmd.Flags().Changed("text"):
				data, err := readInput(cmd, "-")
				if err != nil {
					return fmt.Errorf("read conversation content: %w", err)
				}
				content = string(data)
			}
			result, err := session.Append(root, role, content, a.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Conversation turn %s appended.\n", result.Turn)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&role, "role", "", "turn role: user, assistant, or system")
	cmd.Flags().StringVar(&text, "text", "", "turn content")
	cmd.Flags().StringVar(&file, "file", "", "read turn content from a file")
	_ = cmd.MarkFlagRequired("role")
	return cmd
}

func (a *App) sessionEndCommand(explicitVault *string) *cobra.Command {
	var asJSON, noAnalysis bool
	var assessmentFile, analysisFile, reason string
	cmd := &cobra.Command{
		Use: "end", Short: "Validate interpretation and finish the active session", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if assessmentFile == "-" && analysisFile == "-" {
				return fmt.Errorf("only one of --assessment-file and --analysis-file can read stdin")
			}
			var report *assessment.Report
			if assessmentFile != "" {
				data, err := readInput(cmd, assessmentFile)
				if err != nil {
					return fmt.Errorf("read assessment: %w", err)
				}
				if report, err = assessment.Parse(data); err != nil {
					return err
				}
			}
			var analysis *record.Record
			if analysisFile != "" {
				if analysis, err = readRecord(cmd, analysisFile); err != nil {
					return err
				}
			}
			result, err := session.End(cmd.Context(), root, a.Provider, session.EndOptions{
				Assessment: report, Analysis: analysis, NoAnalysis: noAnalysis, NoAnalysisReason: reason,
			}, a.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Completed %s (%s)\nSession: %s\nProjections: %s\nGit: %s\n", result.ID, result.Kind, result.Session, valueOrNone(result.Projections), result.Git)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&assessmentFile, "assessment-file", "", "baseline assessment JSON file, or - for stdin")
	cmd.Flags().StringVar(&analysisFile, "analysis-file", "", "interpretation record JSON file, or - for stdin")
	cmd.Flags().BoolVar(&noAnalysis, "no-analysis", false, "end without an interpretation record (requires --reason)")
	cmd.Flags().StringVar(&reason, "reason", "", "why the session ends without analysis")
	return cmd
}

func (a *App) sessionCheckpointCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	var analysisFile string
	cmd := &cobra.Command{
		Use: "checkpoint", Short: "Submit an interpretation record for the active session", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			rec, err := readRecord(cmd, analysisFile)
			if err != nil {
				return err
			}
			result, err := session.Checkpoint(root, rec, a.Now())
			if err != nil {
				return err
			}
			return writeCheckpoint(cmd, result, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&analysisFile, "analysis-file", "", "interpretation record JSON file, or - for stdin")
	_ = cmd.MarkFlagRequired("analysis-file")
	return cmd
}

func (a *App) sessionAnnotateCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	var analysisFile string
	cmd := &cobra.Command{
		Use: "annotate <session-id>", Short: "Add an interpretation record to a finished session", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			rec, err := readRecord(cmd, analysisFile)
			if err != nil {
				return err
			}
			result, err := session.Annotate(root, args[0], rec, a.Now())
			if err != nil {
				return err
			}
			return writeCheckpoint(cmd, result, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&analysisFile, "analysis-file", "", "interpretation record JSON file, or - for stdin")
	_ = cmd.MarkFlagRequired("analysis-file")
	return cmd
}

func writeCheckpoint(cmd *cobra.Command, result session.CheckpointResult, asJSON bool) error {
	if asJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Interpretation %s for %s\nProjections: %s\nGit: %s\n", result.Status, result.Session, result.Projections, result.Git)
	return nil
}

func (a *App) sessionTurnsCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	var sessionID, role string
	cmd := &cobra.Command{
		Use: "turns", Short: "List raw turns with stable turn IDs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			id, turns, err := session.Turns(root, sessionID, role)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"session": id, "turns": turns})
			}
			for _, t := range turns {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.ID, t.Role, strings.ReplaceAll(t.Text, "\n", " "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&sessionID, "session", "", "session id (default: active session)")
	cmd.Flags().StringVar(&role, "role", "", "only list turns of this role")
	return cmd
}

func (a *App) curriculumDetourCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "detour", Short: "Record a prerequisite detour away from the mainline"}
	var topic, concept, reason, condition string
	var startJSON bool
	start := &cobra.Command{
		Use: "start", Short: "Leave the mainline for a prerequisite, remembering the return point", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, id, active, err := detourContext(*explicitVault)
			if err != nil {
				return err
			}
			d, err := curriculum.StartDetour(root, id, curriculum.DetourStart{Topic: topic, Concept: concept, Reason: reason, ReturnCondition: condition, Session: active}, a.Now())
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return fmt.Errorf("detour started, but refreshing projections failed: %w", err)
			}
			if startJSON {
				return writeJSON(cmd.OutOrStdout(), d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Detour %s started: %s\n", d.ID, d.Topic)
			return nil
		},
	}
	start.Flags().StringVar(&topic, "topic", "", "prerequisite topic")
	start.Flags().StringVar(&concept, "concept", "", "concept id of the prerequisite, if known")
	start.Flags().StringVar(&reason, "reason", "", "why the mainline is paused")
	start.Flags().StringVar(&condition, "return-condition", "", "observable condition for returning")
	start.Flags().BoolVar(&startJSON, "json", false, "output JSON")
	var outcome, learned string
	var endJSON bool
	end := &cobra.Command{
		Use: "end", Short: "Close the detour and return to the recorded point", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, id, _, err := detourContext(*explicitVault)
			if err != nil {
				return err
			}
			entry, err := curriculum.EndDetour(root, id, outcome, learned, a.Now())
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return fmt.Errorf("detour ended, but refreshing projections failed: %w", err)
			}
			if endJSON {
				return writeJSON(cmd.OutOrStdout(), entry)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Detour %s ended (%s); back to %s / %s\n", entry.ID, entry.Outcome, valueOrNone(entry.ReturnTo.Chapter), valueOrNone(entry.ReturnTo.Concept))
			return nil
		},
	}
	end.Flags().StringVar(&outcome, "outcome", "", "completed or abandoned")
	end.Flags().StringVar(&learned, "learned", "", "what was learned during the detour")
	end.Flags().BoolVar(&endJSON, "json", false, "output JSON")
	cmd.AddCommand(start, end)
	return cmd
}

func detourContext(explicit string) (root, curriculumID, activeSession string, err error) {
	root, err = resolveVault(explicit)
	if err != nil {
		return
	}
	state, err := runtimeState.Load(root)
	if err != nil {
		return
	}
	if state.ActiveSession != nil {
		if state.ActiveSession.Kind == "baseline" {
			err = fmt.Errorf("detours are locked during a baseline assessment")
			return
		}
		activeSession = state.ActiveSession.ID
	}
	cfg, err := config.Load(root)
	if err != nil {
		return
	}
	if cfg.Curriculum.Active == "" {
		err = fmt.Errorf("no active curriculum")
		return
	}
	return root, cfg.Curriculum.Active, activeSession, nil
}

func (a *App) sessionAbortCommand(explicitVault *string) *cobra.Command {
	var reason string
	var asJSON bool
	cmd := &cobra.Command{
		Use: "abort", Short: "Abort the active session without advancing learning state", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			result, err := session.Abort(root, reason, a.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Aborted %s\nSession: %s\nGit: %s\n", result.ID, result.Session, result.Git)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for aborting the session")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

type conceptSummary struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	State        string   `json:"state"`
	Capabilities []string `json:"capabilities"`
	Curricula    []string `json:"curricula"`
}

type patternSummary struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	Description       string `json:"description"`
	PreferredStrategy string `json:"preferred_strategy,omitempty"`
	Supports          int    `json:"supports"`
	Contradicts       int    `json:"contradicts"`
}

type strategySummary struct {
	Strategy     string `json:"strategy"`
	Situation    string `json:"situation"`
	Effective    int    `json:"effective"`
	Inconclusive int    `json:"inconclusive"`
	Ineffective  int    `json:"ineffective"`
}

func (a *App) stateCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "state", Short: "Show the learner model", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			m, _, err := learner.Load(root)
			if err != nil {
				return err
			}
			if m.Empty() {
				if asJSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"available": false, "concepts": []any{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No validated learner state yet.")
				return nil
			}
			if !asJSON {
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(projection.OverviewFile(vaultLanguage(root)))))
				if err == nil {
					_, err = cmd.OutOrStdout().Write(data)
					return err
				}
			}
			concepts := []conceptSummary{}
			for _, c := range m.ConceptList() {
				curricula := []string{}
				for _, r := range c.SourceRefs {
					curricula = append(curricula, r.Curriculum)
				}
				concepts = append(concepts, conceptSummary{ID: c.ID, Label: c.Label, State: c.State(), Capabilities: c.Capabilities(), Curricula: curricula})
			}
			patterns := []patternSummary{}
			for _, p := range m.PatternList() {
				ps := patternSummary{ID: p.ID, Status: p.Status(), Description: p.Description, PreferredStrategy: p.PreferredStrategy}
				for _, o := range p.Observations {
					if o.Stance == "supports" {
						ps.Supports++
					} else {
						ps.Contradicts++
					}
				}
				patterns = append(patterns, ps)
			}
			index := map[string]*strategySummary{}
			strategies := []*strategySummary{}
			for _, at := range m.LiveAttempts() {
				key := at.Strategy + "/" + at.Situation
				s := index[key]
				if s == nil {
					s = &strategySummary{Strategy: at.Strategy, Situation: at.Situation}
					index[key] = s
					strategies = append(strategies, s)
				}
				switch at.Outcome {
				case "effective":
					s.Effective++
				case "inconclusive":
					s.Inconclusive++
				default:
					s.Ineffective++
				}
			}
			out := map[string]any{"available": true, "generation": m.Generation, "concepts": concepts, "patterns": patterns, "strategies": strategies}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			for _, c := range concepts {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", c.ID, c.State, strings.Join(c.Capabilities, ","))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(a.stateConceptCommand(explicitVault))
	return cmd
}

func (a *App) stateConceptCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "concept <concept-id>", Short: "Show one concept with its cognitive history", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			m, _, err := learner.Load(root)
			if err != nil {
				return err
			}
			c := m.FindConcept(args[0])
			if c == nil {
				return fmt.Errorf("unknown concept %q", args[0])
			}
			if !asJSON {
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(projection.ConceptFile(m, c.ID))))
				if err == nil {
					_, err = cmd.OutOrStdout().Write(data)
					return err
				}
			}
			attempts := []*learner.Attempt{}
			for _, at := range m.Attempts {
				if at.Concept == c.ID {
					attempts = append(attempts, at)
				}
			}
			out := map[string]any{"concept": c, "state": c.State(), "capabilities": c.Capabilities(),
				"events": m.EventsFor(c.ID), "cognitive_changes": m.ChangesFor(c.ID), "strategy_attempts": attempts,
				"open_misconceptions": m.UnresolvedMisconceptions(c.ID)}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", c.Label, c.ID, c.State())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) curriculumOutlineCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "outline [id]", Short: "Show the outline with completion status", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			o, err := curriculum.LoadOutline(root, id)
			if err != nil {
				return err
			}
			entries, err := curriculum.LoadProgress(root, id)
			if err != nil {
				return err
			}
			pos, err := curriculum.LoadPosition(root, id)
			if err != nil {
				return err
			}
			m, _, err := learner.Load(root)
			if err != nil {
				return err
			}
			statuses := curriculum.Statuses(o, entries, pos, m.NodesWithEvidence(id))
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "status": o.Status, "type": o.TypeOf(), "position_verified": curriculum.PositionVerified(o, pos), "nodes": statuses})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Outline: %s\n", o.Status)
			for _, s := range statuses {
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s  [%s]\n", strings.Repeat("  ", s.Depth-1), s.Label(), s.Status)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	var file string
	var dryRun, setJSON bool
	set := &cobra.Command{
		Use: "set [id]", Short: "Submit a draft outline (YAML or JSON with a nodes list)", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			data, err := readInput(cmd, file)
			if err != nil {
				return fmt.Errorf("read outline: %w", err)
			}
			o, err := curriculum.SetOutline(root, id, data, dryRun)
			if err != nil {
				return err
			}
			if !dryRun {
				if err := session.Refresh(root); err != nil {
					return err
				}
			}
			if setJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "dry_run": dryRun, "outline": o})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Outline %s: %d entries (%s)\n", map[bool]string{true: "plan", false: "saved"}[dryRun], len(o.Nodes), o.Status)
			return nil
		},
	}
	set.Flags().StringVar(&file, "file", "", "outline file, or - for stdin")
	set.Flags().BoolVar(&dryRun, "dry-run", false, "validate without writing")
	set.Flags().BoolVar(&setJSON, "json", false, "output JSON")
	_ = set.MarkFlagRequired("file")
	var confirmJSON bool
	confirm := &cobra.Command{
		Use: "confirm [id]", Short: "Confirm the outline after the learner reviewed it", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			o, err := curriculum.ConfirmOutline(root, id)
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			if confirmJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "status": o.Status, "entries": len(o.Nodes)})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Outline confirmed: %d entries\n", len(o.Nodes))
			return nil
		},
	}
	confirm.Flags().BoolVar(&confirmJSON, "json", false, "output JSON")
	show := &cobra.Command{Use: "show [id]", Short: "Show the outline with completion status", Args: cobra.MaximumNArgs(1), RunE: cmd.RunE}
	show.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	history := &cobra.Command{
		Use: "history [id]", Short: "List earlier outlines replaced by accepted proposals", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			h, err := curriculum.OutlineHistory(root, id)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "history": h})
			}
			for _, e := range h {
				fmt.Fprintf(cmd.OutOrStdout(), "v%d %s %s: %s (%d entries)\n", e.Version, e.At, e.Proposal, e.Reason, len(e.Outline.Nodes))
			}
			return nil
		},
	}
	history.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(show, set, confirm, history)
	return cmd
}

func (a *App) curriculumMarkCommand(explicitVault *string, use, kind, short string) *cobra.Command {
	var reason string
	var asJSON bool
	cmd := &cobra.Command{
		Use: use + " <node-id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, nil)
			if err != nil {
				return err
			}
			state, err := runtimeState.Load(root)
			if err != nil {
				return err
			}
			activeSession := ""
			if state.ActiveSession != nil {
				if state.ActiveSession.Kind == "baseline" {
					return fmt.Errorf("curriculum progress is locked during a baseline assessment")
				}
				activeSession = state.ActiveSession.ID
			}
			entry, err := curriculum.Mark(root, id, args[0], kind, reason, activeSession, a.Now())
			if err != nil {
				return err
			}
			moved, err := curriculum.AdvanceIfCurrent(root, id, entry.Node)
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			next, err := nextOutlineNode(root, id)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"entry": entry, "moved_to": moved, "next_node": next})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Marked %s %s: %s\n", kind, entry.Node, entry.Title)
			switch {
			case moved != nil:
				fmt.Fprintf(cmd.OutOrStdout(), "Position moved to %s\n", moved.Label())
			case next != nil:
				fmt.Fprintf(cmd.OutOrStdout(), "Next in the outline: %s (set it with 'learn curriculum position set --node %s')\n", next.Label(), next.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why the entry is complete or skipped")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func (a *App) curriculumLifecycleCommands(explicitVault *string) []*cobra.Command {
	deactivate := &cobra.Command{
		Use: "deactivate", Short: "Clear the active curriculum", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			id, err := curriculum.Deactivate(root)
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deactivated %s; no curriculum is active.\n", id)
			return nil
		},
	}
	var reason string
	var dryRun, confirmed, removeJSON bool
	remove := &cobra.Command{
		Use: "remove <id>", Short: "Archive a curriculum (recoverable); history is kept", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if dryRun {
				plan, err := curriculum.PlanRemoval(root, args[0], a.Now())
				if err != nil {
					return err
				}
				if removeJSON {
					return writeJSON(cmd.OutOrStdout(), plan)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Archive plan for %s (%s)\nMoves: %s, %s\nActive: %t\nSessions kept in place: %d\n", plan.ID, plan.Title, plan.Source, plan.Curriculum, plan.Active, len(plan.Sessions))
				if plan.Blocked != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Blocked: %s\n", plan.Blocked)
				}
				return nil
			}
			if !confirmed {
				return fmt.Errorf("curriculum remove requires --yes after reviewing --dry-run")
			}
			archive, err := curriculum.Remove(root, args[0], reason, a.Now())
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			if removeJSON {
				return writeJSON(cmd.OutOrStdout(), archive)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived %s as %s\n", archive.ID, archive.ArchiveID)
			return nil
		},
	}
	remove.Flags().StringVar(&reason, "reason", "", "why the curriculum is removed")
	remove.Flags().BoolVar(&dryRun, "dry-run", false, "show what would move without writing")
	remove.Flags().BoolVar(&confirmed, "yes", false, "confirm the archive")
	remove.Flags().BoolVar(&removeJSON, "json", false, "output JSON")
	restore := &cobra.Command{
		Use: "restore <archive-id>", Short: "Restore an archived curriculum", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			archive, err := curriculum.Restore(root, args[0])
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %s; activate it with 'learn curriculum activate %s'.\n", archive.ID, archive.ID)
			return nil
		},
	}
	var confirmation string
	purge := &cobra.Command{
		Use: "purge <archive-id>", Short: "Permanently delete an archive (cannot be undone)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if !confirmed {
				return fmt.Errorf("purge requires --yes and --confirm <archive-id>")
			}
			archive, err := curriculum.Purge(root, args[0], confirmation)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Permanently deleted archive %s (%s). Conversations and Sessions were kept.\n", archive.ArchiveID, archive.ID)
			return nil
		},
	}
	purge.Flags().BoolVar(&confirmed, "yes", false, "first confirmation")
	purge.Flags().StringVar(&confirmation, "confirm", "", "second confirmation: repeat the archive id")
	var archivesJSON bool
	archives := &cobra.Command{
		Use: "archives", Short: "List archived curricula", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			list, err := curriculum.ListArchives(root)
			if err != nil {
				return err
			}
			if archivesJSON {
				return writeJSON(cmd.OutOrStdout(), list)
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No archived curricula.")
			}
			for _, a := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", a.ArchiveID, a.ID, a.Title, a.Reason)
			}
			return nil
		},
	}
	archives.Flags().BoolVar(&archivesJSON, "json", false, "output JSON")
	return []*cobra.Command{deactivate, remove, restore, purge, archives}
}

// nextOutlineNode is the next unfinished outline entry after the position.
func nextOutlineNode(root, id string) (*curriculum.Node, error) {
	m, _, err := learner.Load(root)
	if err != nil {
		return nil, err
	}
	in, err := projection.Gather(root, m)
	if err != nil {
		return nil, err
	}
	n, ok := curriculum.NextNode(in.Outlines[id], in.Statuses[id], in.Positions[id])
	if !ok {
		return nil, nil
	}
	return &n, nil
}

func curriculumArg(explicit string, args []string) (string, string, error) {
	root, err := resolveVault(explicit)
	if err != nil {
		return "", "", err
	}
	if len(args) == 1 {
		return root, args[0], nil
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", "", err
	}
	if cfg.Curriculum.Active == "" {
		return "", "", fmt.Errorf("no active curriculum; pass a curriculum id")
	}
	return root, cfg.Curriculum.Active, nil
}

func (a *App) reviewCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "review", Short: "List concepts due for spaced review today and in the next 7 days", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			m, _, err := learner.Load(root)
			if err != nil {
				return err
			}
			now := a.Now()
			today, horizon := clock.Date(now), clock.Date(now.AddDate(0, 0, 7))
			type item struct {
				learner.Schedule
				Label string `json:"label"`
			}
			due, upcoming := []item{}, []item{}
			for _, s := range m.Schedules() {
				it := item{Schedule: s, Label: m.Concepts[s.Concept].Label}
				switch {
				case s.Due <= today:
					due = append(due, it)
				case s.Due <= horizon:
					upcoming = append(upcoming, it)
				}
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"today": today, "due": due, "upcoming": upcoming})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Today: %s\n", today)
			if len(due) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing due.")
			}
			for _, it := range due {
				fmt.Fprintf(cmd.OutOrStdout(), "DUE  %s  %s (%s, step %d)\n", it.Due, it.Label, it.Concept, it.Level+1)
			}
			for _, it := range upcoming {
				fmt.Fprintf(cmd.OutOrStdout(), "     %s  %s (%s, step %d)\n", it.Due, it.Label, it.Concept, it.Level+1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) commitCommand(explicitVault *string) *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use: "commit", Short: "Commit Vault changes to Git (e.g. after a sandbox blocked automatic commits)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			if n, err := gitx.Uncommitted(root); err == nil && n == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to commit.")
				return nil
			}
			if result := session.AutoCommit(root, message, a.Now()); result != "committed" {
				return fmt.Errorf("commit %s", result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Committed: %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "learning: manual commit", "commit message")
	return cmd
}

func (a *App) nextCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "next", Short: "Recommend the next best learning action", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			m, _, err := learner.Load(root)
			if err != nil {
				return err
			}
			in, err := projection.Gather(root, m)
			if err != nil {
				return err
			}
			next := policy.Next(m, policy.Context{})
			if in.Next != nil {
				next = *in.Next
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), next)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stage: %s\nAction: %s\nConcept: %s\nStrategy: %s\nRelation: %s\nReason: %s (%s)\nHistory used: %t\n",
				next.Stage, next.Action, valueOrNone(next.Concept), valueOrNone(next.Strategy), next.CurriculumRelation, next.Reason, next.Rule, next.HistoryUsed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) modelCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Maintain the derived learner model"}
	var dryRun, asJSON bool
	rebuild := &cobra.Command{
		Use: "rebuild", Short: "Replay all interpretation records and rewrite projections", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			m, envs, err := learner.Load(root)
			if err != nil {
				return err
			}
			in, err := projection.Gather(root, m)
			if err != nil {
				return err
			}
			files, err := projection.Plan(root, in)
			if err != nil {
				return err
			}
			if !dryRun && !m.Empty() {
				if err := learner.SaveCache(root, m); err != nil {
					return err
				}
				if err := projection.Write(root, files); err != nil {
					return err
				}
			}
			out := map[string]any{"records": len(envs), "generation": m.Generation, "dry_run": dryRun, "files": files}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Records: %d\nGeneration: %s\n", len(envs), m.Generation)
			for _, f := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", f.Action, f.Path)
			}
			return nil
		},
	}
	rebuild.Flags().BoolVar(&dryRun, "dry-run", false, "report changes without writing")
	rebuild.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(rebuild)
	return cmd
}

func (a *App) agentCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Manage embedded AI Agent rules and skills"}
	cmd.AddCommand(a.agentUpdateCommand(explicitVault))
	return cmd
}

func (a *App) agentUpdateCommand(explicitVault *string) *cobra.Command {
	var dryRun, confirmed, asJSON bool
	cmd := &cobra.Command{
		Use: "update", Short: "Update Vault Agent rules and skills with backups", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			result, err := vault.UpdateAgentAssets(root, dryRun, confirmed, a.Now())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent asset update plan: %d change(s)\n", len(result.Changes))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent assets updated: %d change(s)\n", len(result.Changes))
				if result.BackupDir != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Backup: %s\n", result.BackupDir)
				}
			}
			for _, change := range result.Changes {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", change.Action, change.Path)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without writing")
	cmd.Flags().BoolVar(&confirmed, "yes", false, "confirm the update")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func resolveVault(explicit string) (string, error) {
	if explicit != "" {
		return vault.Discover(explicit)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	return vault.Discover(wd)
}

func readInput(cmd *cobra.Command, path string) ([]byte, error) {
	if path != "-" {
		return os.ReadFile(path)
	}
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return nil, fmt.Errorf("no input on stdin; pipe content, use a quoted heredoc, or pass a file")
		}
	}
	return io.ReadAll(in)
}

func readRecord(cmd *cobra.Command, path string) (*record.Record, error) {
	data, err := readInput(cmd, path)
	if err != nil {
		return nil, fmt.Errorf("read interpretation record: %w", err)
	}
	return record.Parse(data)
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func valueOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func Execute(ctx context.Context) error {
	return New().RootCommand().ExecuteContext(ctx)
}
