package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hedykan/learning-system/internal/config"
	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/locator"
	"github.com/hedykan/learning-system/internal/reading"
	runtimeState "github.com/hedykan/learning-system/internal/runtime"
	"github.com/hedykan/learning-system/internal/source"
)

func readingCourse(root, requested string) (string, error) {
	if requested != "" {
		if _, err := curriculum.LoadManifest(root, requested); err != nil {
			return "", fmt.Errorf("load curriculum %s: %w", requested, err)
		}
		return requested, nil
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	status := cfg.Curriculum.Active
	if status == "" {
		return "", fmt.Errorf("no active curriculum; use --curriculum")
	}
	return status, nil
}

func checkedReadingLocator(ref curriculum.SourceRef, kind, value string) (locator.Locator, error) {
	loc := locator.Locator{Resource: ref.ID, Kind: kind, Value: value}
	if err := ref.Validate(loc); err != nil {
		return loc, err
	}
	if kind == "page" {
		_, end, _ := locator.PageRange(value)
		if extent, err := source.ExtentOf(ref.Adapter, ref.Path); err == nil && extent.Total > 0 && end > extent.Total {
			return loc, fmt.Errorf("page %d exceeds %s's %d pages", end, ref.Title, extent.Total)
		}
	}
	return loc, nil
}

func (a *App) sourcePositionCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "position", Short: "Show or set the last reading position"}
	var showCourse string
	var showJSON bool
	show := &cobra.Command{Use: "show <source>", Short: "Show the last reading position", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, err := readingCourse(root, showCourse)
		if err != nil {
			return err
		}
		if _, err := curriculum.Resolve(root, args[0]); err != nil {
			return err
		}
		st, err := reading.Load(root, course, args[0])
		if err != nil {
			return err
		}
		if showJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"course": course, "source": args[0], "position": st.Position})
		}
		if st.Position == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "No saved reading position.")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), st.Position.Label("zh"))
		return nil
	}}
	show.Flags().StringVar(&showCourse, "curriculum", "", "curriculum id (default: active)")
	show.Flags().BoolVar(&showJSON, "json", false, "output JSON")

	var setCourse string
	var setJSON bool
	set := &cobra.Command{Use: "set <source> <kind> <value>", Short: "Save the last reading position", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, err := readingCourse(root, setCourse)
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
		st, err := reading.SetPosition(root, course, ref.ID, loc)
		if err != nil {
			return err
		}
		if setJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"course": course, "source": ref.ID, "position": st.Position})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Saved %s at %s\n", ref.Title, loc.Label("zh"))
		return nil
	}}
	set.Flags().StringVar(&setCourse, "curriculum", "", "curriculum id (default: active)")
	set.Flags().BoolVar(&setJSON, "json", false, "output JSON")
	cmd.AddCommand(show, set)
	return cmd
}

func (a *App) sourceReadingCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "reading", Short: "Record self-directed and tutor-guided reading"}

	var startCourse, mode string
	var startJSON bool
	start := &cobra.Command{Use: "start <source>", Short: "Start a reading session", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, err := readingCourse(root, startCourse)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		learningSession := ""
		if rt, e := runtimeState.Load(root); e == nil && rt.ActiveSession != nil && rt.ActiveSession.Curriculum == course {
			learningSession = rt.ActiveSession.ID
		}
		s, err := reading.Start(root, course, ref.ID, mode, learningSession, a.Now())
		if err != nil {
			return err
		}
		st, _ := reading.Load(root, course, ref.ID)
		data := map[string]any{"reading_session": s, "resume": st.Position}
		if startJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		fmt.Fprintln(cmd.OutOrStdout(), s.ID)
		return nil
	}}
	start.Flags().StringVar(&startCourse, "curriculum", "", "curriculum id (default: active)")
	start.Flags().StringVar(&mode, "mode", "self", "self, guided, skim, or reference")
	start.Flags().BoolVar(&startJSON, "json", false, "output JSON")

	var activeSeconds int
	var progressJSON bool
	progress := &cobra.Command{Use: "progress <session> <kind> <value>", Short: "Record a visible reading range", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, sourceID, ok := reading.FindSession(root, args[0])
		if !ok {
			return fmt.Errorf("reading session %s not found", args[0])
		}
		ref, err := curriculum.Resolve(root, sourceID)
		if err != nil {
			return err
		}
		loc, err := checkedReadingLocator(ref, args[1], args[2])
		if err != nil {
			return err
		}
		st, err := reading.Progress(root, course, sourceID, args[0], loc, activeSeconds, a.Now())
		if err != nil {
			return err
		}
		result := reading.Summarize(st)
		if progressJSON {
			return writeJSON(cmd.OutOrStdout(), result)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s; %d unique unit(s) covered\n", loc.Label("zh"), result.UniqueUnits)
		return nil
	}}
	progress.Flags().IntVar(&activeSeconds, "active-seconds", 0, "foreground reading time for this update")
	progress.Flags().BoolVar(&progressJSON, "json", false, "output JSON")

	var coverCourse, coverMode, learningSession string
	var coverSeconds int
	var coverJSON bool
	cover := &cobra.Command{Use: "cover <source> <kind> <value>", Short: "Record a bounded guided or reference reading range", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, err := readingCourse(root, coverCourse)
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
		st, err := reading.Cover(root, course, ref.ID, coverMode, loc, learningSession, coverSeconds, a.Now())
		if err != nil {
			return err
		}
		result := reading.Summarize(st)
		if coverJSON {
			return writeJSON(cmd.OutOrStdout(), result)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s coverage of %s\n", coverMode, loc.Label("zh"))
		return nil
	}}
	cover.Flags().StringVar(&coverCourse, "curriculum", "", "curriculum id (default: active)")
	cover.Flags().StringVar(&coverMode, "mode", "guided", "guided, self, skim, or reference")
	cover.Flags().StringVar(&learningSession, "learning-session", "", "learning session that delivered this material")
	cover.Flags().IntVar(&coverSeconds, "active-seconds", 0, "foreground reading or teaching time")
	cover.Flags().BoolVar(&coverJSON, "json", false, "output JSON")

	var endJSON bool
	end := &cobra.Command{Use: "end <session>", Short: "End a reading session", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, sourceID, ok := reading.FindSession(root, args[0])
		if !ok {
			return fmt.Errorf("reading session %s not found", args[0])
		}
		s, err := reading.End(root, course, sourceID, args[0], a.Now())
		if err != nil {
			return err
		}
		if endJSON {
			return writeJSON(cmd.OutOrStdout(), s)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Ended %s\n", s.ID)
		return nil
	}}
	end.Flags().BoolVar(&endJSON, "json", false, "output JSON")
	cmd.AddCommand(start, progress, cover, end)
	return cmd
}

func (a *App) sourceProgressCommand(explicitVault *string) *cobra.Command {
	cmd := &cobra.Command{Use: "progress", Short: "Show derived reading coverage"}
	var courseID string
	var asJSON bool
	show := &cobra.Command{Use: "show <source>", Short: "Show reading coverage for a source", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveVault(*explicitVault)
		if err != nil {
			return err
		}
		course, err := readingCourse(root, courseID)
		if err != nil {
			return err
		}
		ref, err := curriculum.Resolve(root, args[0])
		if err != nil {
			return err
		}
		st, err := reading.Load(root, course, ref.ID)
		if err != nil {
			return err
		}
		summary := reading.Summarize(st)
		extent, _ := source.ExtentOf(ref.Adapter, ref.Path)
		data := map[string]any{"progress": summary, "extent": extent, "completion": 0.0}
		if extent.Total > 0 {
			data["completion"] = float64(summary.UniqueUnits) / float64(extent.Total)
		}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), data)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%d/%d %s covered; %d seconds active\n", summary.UniqueUnits, extent.Total, extent.Unit, summary.ActiveSeconds)
		return nil
	}}
	show.Flags().StringVar(&courseID, "curriculum", "", "curriculum id (default: active)")
	show.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.AddCommand(show)
	return cmd
}
