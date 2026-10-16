// Package reading records how a learner moves through source material.
// Reading exposure is deliberately separate from understanding evidence:
// opening a page can update coverage, but only interpretation records can say
// that the learner understood it.
package reading

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/locator"
)

const Version = 1

var safeID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Event struct {
	At            string          `json:"at"`
	Type          string          `json:"type"`
	Mode          string          `json:"mode,omitempty"`
	Locator       locator.Locator `json:"locator"`
	ActiveSeconds int             `json:"active_seconds,omitempty"`
}

type Session struct {
	ID              string  `json:"id"`
	Mode            string  `json:"mode"`
	LearningSession string  `json:"learning_session,omitempty"`
	StartedAt       string  `json:"started_at"`
	EndedAt         string  `json:"ended_at,omitempty"`
	Events          []Event `json:"events"`
}

type State struct {
	Version  int              `json:"version"`
	Course   string           `json:"course"`
	Source   string           `json:"source"`
	Position *locator.Locator `json:"position,omitempty"`
	Sessions []Session        `json:"sessions"`
}

type ModeCount struct {
	Mode          string `json:"mode"`
	Units         int    `json:"units"`
	ActiveSeconds int    `json:"active_seconds"`
}

type Summary struct {
	Course        string            `json:"course"`
	Source        string            `json:"source"`
	LastLocator   *locator.Locator  `json:"last_locator,omitempty"`
	Covered       []locator.Locator `json:"covered"`
	UniqueUnits   int               `json:"unique_units"`
	ActiveSeconds int               `json:"active_seconds"`
	Sessions      int               `json:"sessions"`
	ByMode        []ModeCount       `json:"by_mode"`
}

func statePath(root, course, source string) (string, error) {
	if !safeID.MatchString(course) || !safeID.MatchString(source) {
		return "", fmt.Errorf("course and source ids must use lowercase letters, digits, and hyphens")
	}
	return filepath.Join(root, ".learning", "reading", course, source+".json"), nil
}

func Load(root, course, source string) (State, error) {
	path, err := statePath(root, course, source)
	if err != nil {
		return State{}, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{Version: Version, Course: course, Source: source, Sessions: []Session{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, fmt.Errorf("parse reading state: %w", err)
	}
	if st.Version != Version || st.Course != course || st.Source != source {
		return State{}, fmt.Errorf("invalid reading state for %s/%s", course, source)
	}
	return st, nil
}

func save(root string, st State) error {
	path, err := statePath(root, st.Course, st.Source)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(b, '\n'), 0o644)
}

func SetPosition(root, course, source string, loc locator.Locator) (State, error) {
	st, err := Load(root, course, source)
	if err != nil {
		return st, err
	}
	loc.Resource = source
	st.Position = &loc
	return st, save(root, st)
}

func Start(root, course, source, mode, learningSession string, now time.Time) (Session, error) {
	if mode == "" {
		mode = "self"
	}
	if mode != "self" && mode != "guided" && mode != "skim" && mode != "reference" {
		return Session{}, fmt.Errorf("invalid reading mode %q (use self, guided, skim, or reference)", mode)
	}
	st, err := Load(root, course, source)
	if err != nil {
		return Session{}, err
	}
	s := Session{ID: "reading-" + now.UTC().Format("20060102-150405.000000000"), Mode: mode,
		LearningSession: learningSession, StartedAt: now.UTC().Format(time.RFC3339), Events: []Event{}}
	st.Sessions = append(st.Sessions, s)
	return s, save(root, st)
}

func Progress(root, course, source, sessionID string, loc locator.Locator, activeSeconds int, now time.Time) (State, error) {
	if activeSeconds < 0 {
		return State{}, fmt.Errorf("active seconds cannot be negative")
	}
	st, err := Load(root, course, source)
	if err != nil {
		return st, err
	}
	i := sessionIndex(st, sessionID)
	if i < 0 {
		return st, fmt.Errorf("reading session %s not found", sessionID)
	}
	if st.Sessions[i].EndedAt != "" {
		return st, fmt.Errorf("reading session %s already ended", sessionID)
	}
	loc.Resource = source
	st.Sessions[i].Events = append(st.Sessions[i].Events, Event{At: now.UTC().Format(time.RFC3339), Type: "view", Mode: st.Sessions[i].Mode, Locator: loc, ActiveSeconds: activeSeconds})
	st.Position = &loc
	return st, save(root, st)
}

func Cover(root, course, source, mode string, loc locator.Locator, learningSession string, activeSeconds int, now time.Time) (State, error) {
	if mode == "" {
		mode = "guided"
	}
	s, err := Start(root, course, source, mode, learningSession, now)
	if err != nil {
		return State{}, err
	}
	st, err := Progress(root, course, source, s.ID, loc, activeSeconds, now)
	if err != nil {
		return st, err
	}
	_, err = End(root, course, source, s.ID, now)
	if err != nil {
		return State{}, err
	}
	return Load(root, course, source)
}

func End(root, course, source, sessionID string, now time.Time) (Session, error) {
	st, err := Load(root, course, source)
	if err != nil {
		return Session{}, err
	}
	i := sessionIndex(st, sessionID)
	if i < 0 {
		return Session{}, fmt.Errorf("reading session %s not found", sessionID)
	}
	if st.Sessions[i].EndedAt == "" {
		st.Sessions[i].EndedAt = now.UTC().Format(time.RFC3339)
	}
	return st.Sessions[i], save(root, st)
}

func sessionIndex(st State, id string) int {
	for i := range st.Sessions {
		if st.Sessions[i].ID == id {
			return i
		}
	}
	return -1
}

func Summarize(st State) Summary {
	out := Summary{Course: st.Course, Source: st.Source, LastLocator: st.Position, Covered: []locator.Locator{}, Sessions: len(st.Sessions), ByMode: []ModeCount{}}
	type counts struct{ units, seconds int }
	byMode := map[string]*counts{}
	pageByMode := map[string]map[int]bool{}
	otherByMode := map[string]map[string]locator.Locator{}
	allPages := map[int]bool{}
	allOther := map[string]locator.Locator{}
	for _, s := range st.Sessions {
		for _, e := range s.Events {
			mode := e.Mode
			if mode == "" {
				mode = s.Mode
			}
			if byMode[mode] == nil {
				byMode[mode] = &counts{}
			}
			byMode[mode].seconds += e.ActiveSeconds
			out.ActiveSeconds += e.ActiveSeconds
			if a, b, ok := locator.PageRange(e.Locator.Value); e.Locator.Kind == "page" && ok {
				if pageByMode[mode] == nil {
					pageByMode[mode] = map[int]bool{}
				}
				for p := a; p <= b; p++ {
					pageByMode[mode][p] = true
					allPages[p] = true
				}
			} else {
				if otherByMode[mode] == nil {
					otherByMode[mode] = map[string]locator.Locator{}
				}
				key := e.Locator.Kind + ":" + e.Locator.Value
				otherByMode[mode][key], allOther[key] = e.Locator, e.Locator
			}
		}
	}
	out.Covered = append(out.Covered, mergePages(st.Source, allPages)...)
	keys := make([]string, 0, len(allOther))
	for k := range allOther {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.Covered = append(out.Covered, allOther[k])
	}
	out.UniqueUnits = len(allPages) + len(allOther)
	modes := make([]string, 0, len(byMode))
	for mode := range byMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	for _, mode := range modes {
		byMode[mode].units = len(pageByMode[mode]) + len(otherByMode[mode])
		out.ByMode = append(out.ByMode, ModeCount{Mode: mode, Units: byMode[mode].units, ActiveSeconds: byMode[mode].seconds})
	}
	return out
}

func mergePages(source string, pages map[int]bool) []locator.Locator {
	if len(pages) == 0 {
		return nil
	}
	ns := make([]int, 0, len(pages))
	for p := range pages {
		ns = append(ns, p)
	}
	sort.Ints(ns)
	var out []locator.Locator
	start, end := ns[0], ns[0]
	flush := func() {
		v := strconv.Itoa(start)
		if end != start {
			v += "-" + strconv.Itoa(end)
		}
		out = append(out, locator.Locator{Resource: source, Kind: "page", Value: v})
	}
	for _, p := range ns[1:] {
		if p == end+1 {
			end = p
			continue
		}
		flush()
		start, end = p, p
	}
	flush()
	return out
}

// FindSession locates a reading session when only its stable id is known.
func FindSession(root, id string) (course, source string, ok bool) {
	base := filepath.Join(root, ".learning", "reading")
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") || ok {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var st State
		if json.Unmarshal(b, &st) != nil {
			return nil
		}
		if sessionIndex(st, id) >= 0 {
			course, source, ok = st.Course, st.Source, true
		}
		return nil
	})
	return
}
