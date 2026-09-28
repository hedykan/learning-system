package app

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/session"
	"github.com/hedykan/learning-system/internal/source"
)

// sourceCommand groups resource commands (CR-2026-026 to 028). A source is
// a curriculum's own material (its id is the curriculum id) or a resource.
func (a *App) sourceCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "source", Short: "Add, read and attach learning resources"}
	cmd.AddCommand(a.sourceAddCommand(explicitVault), a.sourceListCommand(explicitVault), a.sourceOutlineCommand(explicitVault),
		a.sourceReadCommand(explicitVault), a.sourceAttachCommand(explicitVault), a.sourceDetachCommand(explicitVault),
		a.sourceCheckCommand(explicitVault), a.sourceRefreshCommand(explicitVault))
	return cmd
}

func (a *App) sourceAddCommand(explicitVault *string) *cobra.Command {
	var id, title, url, note, kind, sitemap, prefix string
	var maxPages int
	var external, link, asJSON bool
	cmd := &cobra.Command{
		Use: "add <path> | --external --title <name>", Short: "Store a resource (file, folder, or material without a file)", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			opts := curriculum.ResourceOptions{ID: id, Title: title, External: external, URL: url, Note: note, Kind: kind, Sitemap: sitemap, Prefix: prefix, MaxPages: maxPages, Now: a.Now()}
			if len(args) == 1 {
				opts.Path = args[0]
			} else if !external {
				return fmt.Errorf("give the resource's path, or --external for a video course, paper book or class")
			}
			if link {
				opts.Mode = "link"
			}
			r, err := curriculum.AddResource(root, opts)
			if err != nil {
				return err
			}
			ref, err := curriculum.Resolve(root, r.ID)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"resource": r, "capabilities": ref.Caps})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added resource %s (%s): %s\n", r.ID, r.Kind, r.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "stable lowercase resource id (required)")
	cmd.Flags().StringVar(&title, "title", "", "human-readable title")
	cmd.Flags().BoolVar(&external, "external", false, "material without a file: a video course, paper book or class")
	cmd.Flags().StringVar(&url, "url", "", "where the material is (e.g. a playlist)")
	cmd.Flags().StringVar(&note, "note", "", "publication details")
	cmd.Flags().BoolVar(&link, "link", false, "link to the file outside the Vault instead of copying it")
	cmd.Flags().StringVar(&kind, "kind", "", "force a kind that is never detected: code (a Git project)")
	addFetchFlags(cmd, &sitemap, &prefix, &maxPages)
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func (a *App) sourceListCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "list", Short: "List curriculum materials and resources with their capabilities", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			var ids []string
			manifests, err := curriculum.List(root)
			if err != nil {
				return err
			}
			for _, m := range manifests {
				ids = append(ids, m.ID)
			}
			resources, err := curriculum.ListResources(root)
			if err != nil {
				return err
			}
			for _, r := range resources {
				ids = append(ids, r.ID)
			}
			refs := []curriculum.SourceRef{}
			for _, id := range ids {
				ref, err := curriculum.Resolve(root, id)
				if err != nil {
					return err
				}
				refs = append(refs, ref)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"sources": refs})
			}
			for _, r := range refs {
				role := "resource"
				if r.Primary {
					role = "curriculum"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", r.ID, role, r.Kind, r.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceOutlineCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "outline <source>", Short: "Print the headings a draft outline can be built from", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			ref, err := curriculum.Resolve(root, args[0])
			if err != nil {
				return err
			}
			sections, err := ref.Outline()
			status := "ok"
			if errors.Is(err, source.ErrNoStructure) {
				status, sections, err = "no_structure", []source.Section{}, nil
			}
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"source": ref.ID, "status": status, "sections": sections})
			}
			if status == "no_structure" {
				fmt.Fprintln(cmd.OutOrStdout(), "No structure: build the outline yourself.")
			}
			for _, s := range sections {
				fmt.Fprintf(cmd.OutOrStdout(), "%*s%s %s\n", 2*(s.Level-1), "", s.Title, s.Anchor)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceReadCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "read <source> <kind> <value>", Short: "Read a resource at a locator, as Markdown (e.g. read ddia anchor '#replication')", Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			ref, err := curriculum.Resolve(root, args[0])
			if err != nil {
				return err
			}
			loc := locator.Locator{Resource: ref.ID, Kind: args[1], Value: args[2]}
			content, err := ref.Read(loc)
			var unsupported *source.Unsupported
			if errors.As(err, &unsupported) {
				if asJSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "unsupported", "reason": unsupported.Reason, "capabilities": ref.Caps})
				}
				return err
			}
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "content": content})
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), content.Text)
			return err
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceAttachCommand(explicitVault *string) *cobra.Command {
	var curriculumID string
	var asJSON bool
	cmd := &cobra.Command{
		Use: "attach <node> <source> <kind> <value>", Short: "Attach a resource position to an outline entry (e.g. attach 1.2 lectures time 3/05:20-48:00)", Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, optional(curriculumID))
			if err != nil {
				return err
			}
			res, err := curriculum.Attach(root, id, args[0], locator.Locator{Resource: args[1], Kind: args[2], Value: args[3]})
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"curriculum": id, "node": args[0], "attached": res})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Attached %s %s to %s\n", res.Resource, res.Label, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&curriculumID, "curriculum", "", "curriculum id (default: the active one)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceDetachCommand(explicitVault *string) *cobra.Command {
	var curriculumID string
	cmd := &cobra.Command{
		Use: "detach <node> <source>", Short: "Remove a resource's positions from an outline entry", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, optional(curriculumID))
			if err != nil {
				return err
			}
			n, err := curriculum.Detach(root, id, args[0], args[1])
			if err != nil {
				return err
			}
			if err := session.Refresh(root); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d position(s) of %s from %s\n", n, args[1], args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&curriculumID, "curriculum", "", "curriculum id (default: the active one)")
	return cmd
}

func (a *App) sourceCheckCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "check [curriculum]", Short: "Report invalid resource positions and outline entries without any resource", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, args)
			if err != nil {
				return err
			}
			rep, err := curriculum.Check(root, id)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), rep)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Resources: %v\nInvalid positions: %d\nEntries without a resource: %v\n", rep.Resources, len(rep.Invalid), rep.Unsourced)
			for _, i := range rep.Invalid {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s: %s\n", i.Node, i.Locator.Kind, i.Locator.Value, i.Error)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func optional(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

func (a *App) sourceRefreshCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "refresh <source>", Short: "Record a new revision of a code project or web snapshot", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveVault(*explicitVault)
			if err != nil {
				return err
			}
			res, err := curriculum.Refresh(root, args[0], a.Now())
			if err != nil {
				return err
			}
			if res.Changed {
				if err := session.Refresh(root); err != nil {
					return err
				}
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), res)
			}
			if !res.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is unchanged\n", args[0])
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: new revision recorded; earlier records keep their pinned positions\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// addFetchFlags adds the web snapshot options of a URL source. --sitemap
// and --sitemap-url both fill sitemap ("auto" or a URL).
func addFetchFlags(cmd *cobra.Command, sitemap, prefix *string, maxPages *int) {
	auto := new(bool)
	cmd.Flags().BoolVar(auto, "sitemap", false, "for a URL: snapshot every page listed in the site's sitemap")
	cmd.Flags().StringVar(sitemap, "sitemap-url", "", "for a URL: snapshot every page listed in this sitemap")
	cmd.PreRunE = func(*cobra.Command, []string) error {
		if *auto && *sitemap == "" {
			*sitemap = "auto"
		}
		return nil
	}
	cmd.Flags().StringVar(prefix, "prefix", "", "for a URL with --sitemap: only pages whose path starts with this")
	cmd.Flags().IntVar(maxPages, "max-pages", 0, "for a URL with --sitemap: at most this many pages (never more than 500)")
}
