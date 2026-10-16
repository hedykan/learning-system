package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/reading"
	"github.com/hedykan/learning-system/internal/session"
	"github.com/hedykan/learning-system/internal/source"
)

// sourceCommand groups resource commands (CR-2026-026 to 028). A source is
// a curriculum's own material (its id is the curriculum id) or a resource.
func (a *App) sourceCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "source", Short: "Add, read and attach learning resources"}
	cmd.AddCommand(a.sourceAddCommand(explicitVault), a.sourceListCommand(explicitVault), a.sourceOutlineCommand(explicitVault),
		a.sourceReadCommand(explicitVault), a.sourceAttachCommand(explicitVault), a.sourceDetachCommand(explicitVault),
		a.sourceCheckCommand(explicitVault), a.sourceRefreshCommand(explicitVault), a.sourceShowCommand(explicitVault),
		a.sourceNavigateCommand(explicitVault), a.sourceSearchCommand(explicitVault), a.sourceContextCommand(explicitVault),
		a.sourcePositionCommand(explicitVault), a.sourceReadingCommand(explicitVault), a.sourceProgressCommand(explicitVault),
		a.sourceRenderCommand(explicitVault))
	return cmd
}

var renderName = regexp.MustCompile(`[^a-zA-Z0-9.-]+`)

func (a *App) sourceRenderCommand(explicitVault *string) *cobra.Command {
	var output string
	var asJSON bool
	cmd := &cobra.Command{Use: "render <source> <kind> <value>", Short: "Render one visual source unit, such as a scanned PDF page", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		loc, err := checkedReadingLocator(ref, args[1], args[2])
		if err != nil {
			return err
		}
		if output == "" {
			name := renderName.ReplaceAllString(loc.Kind+"-"+loc.Value, "-") + ".png"
			output = filepath.Join(root, ".learning", "tmp", "rendered", ref.ID, name)
		} else if output, err = filepath.Abs(output); err != nil {
			return err
		}
		if err := source.Render(ref.Adapter, ref.Path, loc, output); err != nil {
			return err
		}
		info, err := os.Stat(output)
		if err != nil {
			return err
		}
		data := map[string]any{"source": ref.ID, "locator": loc, "path": output, "media_type": "image/png", "bytes": info.Size()}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}}
	cmd.Flags().StringVar(&output, "output", "", "PNG output path (default: Vault temporary cache)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
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
				previous, next, _ := sourceNeighbours(ref, loc)
				extent, _ := source.ExtentOf(ref.Adapter, ref.Path)
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "source": ref.ID, "title": ref.Title,
					"kind": ref.Kind, "content": content, "previous": previous, "next": next, "extent": extent})
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), content.Text)
			return err
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceShowCommand(explicitVault *string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{Use: "show <source>", Short: "Show one source and its reading capabilities", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		extent, extentErr := source.ExtentOf(ref.Adapter, ref.Path)
		data := map[string]any{"source": ref, "extent": extent}
		if extentErr != nil {
			data["extent_status"] = "unavailable"
			data["extent_reason"] = extentErr.Error()
		}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\nReadable: %v\nLength: %d %s\n", ref.Title, ref.Kind, ref.Caps.Extractable, extent.Total, extent.Unit)
		return nil
	}}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func sourceNeighbours(ref curriculum.SourceRef, loc locator.Locator) (*locator.Locator, *locator.Locator, error) {
	withResource := func(l locator.Locator) *locator.Locator { l.Resource = ref.ID; return &l }
	if loc.Kind == "page" {
		a, b, ok := locator.PageRange(loc.Value)
		if !ok {
			return nil, nil, loc.Validate()
		}
		extent, err := source.ExtentOf(ref.Adapter, ref.Path)
		if err != nil {
			return nil, nil, err
		}
		var previous, next *locator.Locator
		if a > 1 {
			previous = withResource(locator.Locator{Kind: "page", Value: fmt.Sprint(a - 1)})
		}
		if b < extent.Total {
			next = withResource(locator.Locator{Kind: "page", Value: fmt.Sprint(b + 1)})
		}
		return previous, next, nil
	}
	sections, err := ref.Outline()
	if err != nil {
		return nil, nil, err
	}
	ordered := []locator.Locator{}
	for _, section := range sections {
		candidate := locator.Locator{Resource: ref.ID, Kind: "anchor", Value: section.Anchor}
		if loc.Kind == "chapter" && section.Chapter != "" {
			candidate.Kind, candidate.Value = "chapter", section.Chapter
		}
		if candidate.Kind == loc.Kind {
			ordered = append(ordered, candidate)
		}
	}
	for i, candidate := range ordered {
		if candidate.Value != loc.Value {
			continue
		}
		var previous, next *locator.Locator
		if i > 0 {
			p := ordered[i-1]
			previous = &p
		}
		if i+1 < len(ordered) {
			n := ordered[i+1]
			next = &n
		}
		return previous, next, nil
	}
	return nil, nil, fmt.Errorf("%s %s is not in the source's navigable outline", loc.Kind, loc.Value)
}

func (a *App) sourceNavigateCommand(explicitVault *string) *cobra.Command {
	var next, previous, asJSON bool
	cmd := &cobra.Command{Use: "navigate <source> <kind> <value>", Short: "Read the next or previous page or section", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		if next == previous {
			return fmt.Errorf("choose exactly one of --next or --previous")
		}
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		loc := locator.Locator{Resource: ref.ID, Kind: args[1], Value: args[2]}
		prevLoc, nextLoc, err := sourceNeighbours(ref, loc)
		if err != nil {
			return err
		}
		target := nextLoc
		if previous {
			target = prevLoc
		}
		if target == nil {
			return fmt.Errorf("already at the %s of %s", map[bool]string{true: "end", false: "beginning"}[next], ref.Title)
		}
		content, err := ref.Read(*target)
		if err != nil {
			return err
		}
		p, n, _ := sourceNeighbours(ref, *target)
		data := map[string]any{"source": ref.ID, "locator": target, "content": content, "previous": p, "next": n}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), content.Text)
		return err
	}}
	cmd.Flags().BoolVar(&next, "next", false, "move to the next page or section")
	cmd.Flags().BoolVar(&previous, "previous", false, "move to the previous page or section")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceSearchCommand(explicitVault *string) *cobra.Command {
	var limit int
	var asJSON bool
	cmd := &cobra.Command{Use: "search <source> <query>", Short: "Search a source and return jumpable locations", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		matches, err := source.Search(ref.Adapter, ref.Path, args[1], limit)
		if err != nil {
			return err
		}
		for i := range matches {
			matches[i].Locator.Resource = ref.ID
		}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"source": ref.ID, "query": args[1], "matches": matches})
		}
		for _, m := range matches {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", m.Locator.Label("zh"), m.Heading, m.Excerpt)
		}
		return nil
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum matches")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceContextCommand(explicitVault *string) *cobra.Command {
	var before, after int
	var curriculumID string
	var asJSON bool
	cmd := &cobra.Command{Use: "context <source> <kind> <value>", Short: "Read a bounded context window for the tutor", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		loc := locator.Locator{Resource: ref.ID, Kind: args[1], Value: args[2]}
		current, err := ref.Read(loc)
		if err != nil {
			return err
		}
		previous, next := []source.Content{}, []source.Content{}
		cursor := loc
		for range before {
			p, _, e := sourceNeighbours(ref, cursor)
			if e != nil || p == nil {
				break
			}
			c, e := ref.Read(*p)
			if e != nil {
				break
			}
			previous = append([]source.Content{c}, previous...)
			cursor = *p
		}
		cursor = loc
		for range after {
			_, n, e := sourceNeighbours(ref, cursor)
			if e != nil || n == nil {
				break
			}
			c, e := ref.Read(*n)
			if e != nil {
				break
			}
			next = append(next, c)
			cursor = *n
		}
		var progress any
		if curriculumID == "" {
			if _, active, e := curriculumArg(*explicitVault, nil); e == nil {
				curriculumID = active
			}
		}
		if curriculumID != "" {
			st, e := reading.Load(root, curriculumID, ref.ID)
			if e == nil {
				progress = reading.Summarize(st)
			}
		}
		data := map[string]any{"source": ref.ID, "title": ref.Title, "locator": loc, "current": current, "before": previous, "after": next, "reading_progress": progress}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		for _, c := range previous {
			fmt.Fprint(cmd.OutOrStdout(), c.Text)
		}
		fmt.Fprint(cmd.OutOrStdout(), current.Text)
		for _, c := range next {
			fmt.Fprint(cmd.OutOrStdout(), c.Text)
		}
		return nil
	}}
	cmd.Flags().IntVar(&before, "before", 0, "preceding reading units")
	cmd.Flags().IntVar(&after, "after", 0, "following reading units")
	cmd.Flags().StringVar(&curriculumID, "curriculum", "", "include reading progress for this curriculum")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (a *App) sourceAttachCommand(explicitVault *string) *cobra.Command {
	var curriculumID, why, serves string
	var asJSON bool
	cmd := &cobra.Command{
		Use: "attach <node> <source> <kind> <value>", Short: "Attach a resource position to an outline entry (e.g. attach 1.2 lectures time 3/05:20-48:00)", Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, id, err := curriculumArg(*explicitVault, optional(curriculumID))
			if err != nil {
				return err
			}
			res, err := curriculum.AttachWith(root, id, args[0], locator.Locator{Resource: args[1], Kind: args[2], Value: args[3]}, why, serves)
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
	cmd.Flags().StringVar(&why, "why", "", "why this resource was chosen for the entry (at most 120 characters)")
	cmd.Flags().StringVar(&serves, "serves", "", "the goal card focus id this resource serves")
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
