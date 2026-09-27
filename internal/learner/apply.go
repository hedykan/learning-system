package learner

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/channelwill/learning-os/internal/conversation"
	"github.com/channelwill/learning-os/internal/record"
)

var (
	localIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	slugPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// TurnResolver returns a raw turn of any session.
type TurnResolver interface {
	Turn(session, turn string) (conversation.Turn, error)
}

type pos struct {
	session string
	ordinal int
}

func (a pos) less(b pos) bool {
	if a.session != b.session {
		return a.session < b.session
	}
	return a.ordinal < b.ordinal
}

type applier struct {
	m        *Model
	env      record.Envelope
	rec      *record.Record
	resolver TurnResolver
}

// Apply validates one envelope against the model and applies it. On error
// the model must be discarded; callers replay from records instead of undoing.
func (m *Model) Apply(env record.Envelope, resolver TurnResolver) error {
	a := &applier{m: m, env: env, rec: &env.Record, resolver: resolver}
	if env.Record.Schema != record.Schema {
		return fmt.Errorf("record schema must be %q", record.Schema)
	}
	if env.Record.Empty() {
		return fmt.Errorf("record contains no interpretation")
	}
	info := m.Sessions[env.Session]
	if info == nil {
		info = &SessionInfo{ID: env.Session, Curriculum: env.Record.Curriculum}
		m.Sessions[env.Session] = info
	} else if info.Curriculum != env.Record.Curriculum {
		return fmt.Errorf("record curriculum %q does not match session curriculum %q", env.Record.Curriculum, info.Curriculum)
	}
	info.Records++
	steps := []func() error{a.concepts, a.relations, a.noRelated, a.events, a.changes, a.attempts, a.stateUpdates, a.reviews, a.patterns, a.retractions, a.progress}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (a *applier) gid(local string) string { return a.env.Session + ":" + local }

// resolveRef turns a local or global reference into a global ID.
func (a *applier) resolveRef(ref string) string {
	if strings.Contains(ref, ":") {
		return ref
	}
	return a.gid(ref)
}

// claim registers a local ID. It returns skip=true when an identical item was
// already accepted, so cumulative resubmission is harmless.
func (a *applier) claim(kind, local string, item any) (gid string, skip bool, err error) {
	if !localIDPattern.MatchString(local) {
		return "", false, fmt.Errorf("%s id %q must match %s", kind, local, localIDPattern)
	}
	gid = a.gid(local)
	data, _ := json.Marshal(item)
	if prev, ok := a.m.itemJSON[gid]; ok {
		if prev == string(data) {
			return gid, true, nil
		}
		return "", false, fmt.Errorf("%s id %q was already used with different content", kind, local)
	}
	a.m.itemJSON[gid] = string(data)
	return gid, false, nil
}

func (a *applier) nextOrder() int { a.m.order++; return a.m.order }

func (a *applier) turnPos(ref string) (pos, conversation.Turn, error) {
	session, turn := a.env.Session, ref
	if s, t, ok := strings.Cut(ref, "#"); ok {
		session, turn = s, t
	}
	raw, err := a.resolver.Turn(session, turn)
	if err != nil {
		return pos{}, conversation.Turn{}, err
	}
	return pos{session, raw.Ordinal}, raw, nil
}

func (a *applier) evidence(what string, list []record.Evidence) ([]EvidenceRef, []pos, error) {
	if len(list) == 0 {
		return nil, nil, fmt.Errorf("%s requires learner evidence", what)
	}
	refs := make([]EvidenceRef, 0, len(list))
	positions := make([]pos, 0, len(list))
	for i, ev := range list {
		p, turn, err := a.turnPos(ev.Turn)
		if err != nil {
			return nil, nil, fmt.Errorf("%s evidence %d: %w", what, i, err)
		}
		if turn.Role != "user" {
			return nil, nil, fmt.Errorf("%s evidence %d cites a %s turn; only learner turns are evidence", what, i, turn.Role)
		}
		if err := conversation.QuoteMatches(turn.Text, ev.Quote); err != nil {
			return nil, nil, fmt.Errorf("%s evidence %d: %w", what, i, err)
		}
		refs = append(refs, EvidenceRef{Session: p.session, Turn: turn.ID, Quote: conversation.Normalize(ev.Quote)})
		positions = append(positions, p)
	}
	return refs, positions, nil
}

func (a *applier) requireConcept(what, id string) error {
	if _, ok := a.m.Concepts[id]; !ok {
		return fmt.Errorf("%s references unknown concept %q; declare it in concepts first", what, id)
	}
	return nil
}

func nonEmpty(what string, values map[string]string) error {
	for name, v := range values {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s requires %s", what, name)
		}
	}
	return nil
}

func (a *applier) concepts() error {
	for _, c := range a.rec.Concepts {
		if !slugPattern.MatchString(c.ID) {
			return fmt.Errorf("concept id %q must be a kebab-case slug", c.ID)
		}
		existing := a.m.Concepts[c.ID]
		if existing == nil {
			if strings.TrimSpace(c.Label) == "" {
				return fmt.Errorf("new concept %q requires a label", c.ID)
			}
			if err := a.checkNameFree(c.ID, c.Label); err != nil {
				return err
			}
			existing = &Concept{ID: c.ID, Label: strings.TrimSpace(c.Label), History: []*StateEntry{}}
			a.m.Concepts[c.ID] = existing
		} else if c.Label != "" && strings.TrimSpace(c.Label) != existing.Label {
			return fmt.Errorf("concept %q label is %q and cannot change", c.ID, existing.Label)
		}
		for _, alias := range c.Aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" || containsFold(existing.Aliases, alias) || fold(alias) == fold(existing.Label) {
				continue
			}
			if err := a.checkNameFree(c.ID, alias); err != nil {
				return err
			}
			existing.Aliases = append(existing.Aliases, alias)
		}
		if tp := c.TextbookPoints; tp != nil {
			if err := validatePoints(c.ID, tp); err != nil {
				return err
			}
			last := len(existing.Points) - 1
			if last < 0 || !samePoints(existing.Points[last], tp) {
				existing.Points = append(existing.Points, &PointsEntry{Session: a.env.Session, At: a.env.SubmittedAt,
					Pages: append([]int(nil), tp.Pages...), Points: append([]string(nil), tp.Points...)})
			}
		}
		if c.SourceRef != nil {
			ref := SourceRef{Curriculum: a.rec.Curriculum, Node: c.SourceRef.Node, Chapter: c.SourceRef.Chapter, Section: c.SourceRef.Section}
			dup := false
			for _, r := range existing.SourceRefs {
				dup = dup || r == ref
			}
			if !dup {
				existing.SourceRefs = append(existing.SourceRefs, ref)
			}
		}
	}
	return nil
}

func validatePoints(id string, tp *record.TextbookPoints) error {
	if len(tp.Pages) != 2 || tp.Pages[0] < 1 || tp.Pages[0] > tp.Pages[1] {
		return fmt.Errorf("concept %s textbook_points.pages must be [start, end] with 1 <= start <= end", id)
	}
	if len(tp.Points) < 1 || len(tp.Points) > 5 {
		return fmt.Errorf("concept %s textbook_points needs 1-5 points", id)
	}
	for i, p := range tp.Points {
		if n := utf8.RuneCountInString(strings.TrimSpace(p)); n < 4 || n > 120 {
			return fmt.Errorf("concept %s textbook point %d must be 4-120 characters, got %d", id, i+1, n)
		}
	}
	return nil
}

func samePoints(e *PointsEntry, tp *record.TextbookPoints) bool {
	if len(e.Pages) != len(tp.Pages) || len(e.Points) != len(tp.Points) {
		return false
	}
	for i := range e.Pages {
		if e.Pages[i] != tp.Pages[i] {
			return false
		}
	}
	for i := range e.Points {
		if e.Points[i] != tp.Points[i] {
			return false
		}
	}
	return true
}

func (a *applier) checkNameFree(id, name string) error {
	for _, other := range a.m.Concepts {
		if other.ID == id {
			continue
		}
		if fold(other.Label) == fold(name) || containsFold(other.Aliases, name) {
			return fmt.Errorf("concept name %q already belongs to concept %q; reuse that id", name, other.ID)
		}
	}
	return nil
}

func (a *applier) events() error {
	for _, e := range a.rec.Events {
		gid, skip, err := a.claim("event", e.ID, e)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "event " + e.ID
		if !EventTypes[e.Type] {
			if e.Type == "strategy_switch" {
				return fmt.Errorf("%s: strategy_switch is derived from strategy_attempts.replaces", what)
			}
			return fmt.Errorf("%s has unsupported type %q", what, e.Type)
		}
		if e.Concept != "" {
			if err := a.requireConcept(what, e.Concept); err != nil {
				return err
			}
		}
		if err := nonEmpty(what, map[string]string{"summary": e.Summary}); err != nil {
			return err
		}
		refs, _, err := a.evidence(what, e.Evidence)
		if err != nil {
			return err
		}
		ev := &Event{GID: gid, Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: a.nextOrder(),
			Type: e.Type, Concept: e.Concept, Summary: e.Summary, Evidence: refs}
		a.m.Events = append(a.m.Events, ev)
		a.m.items[gid] = ev
	}
	return nil
}

func (a *applier) changes() error {
	for _, c := range a.rec.CognitiveChanges {
		gid, skip, err := a.claim("cognitive change", c.ID, c)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "cognitive change " + c.ID
		if err := a.requireConcept(what, c.Concept); err != nil {
			return err
		}
		if err := nonEmpty(what, map[string]string{"old_model": c.OldModel, "trigger": c.Trigger, "new_model": c.NewModel, "trigger_turn": c.TriggerTurn}); err != nil {
			return err
		}
		oldRefs, oldPos, err := a.evidence(what+" old_evidence", c.OldEvidence)
		if err != nil {
			return err
		}
		newRefs, newPos, err := a.evidence(what+" new_evidence", c.NewEvidence)
		if err != nil {
			return err
		}
		latestOld, earliestNew := oldPos[0], newPos[0]
		for _, p := range oldPos {
			if latestOld.less(p) {
				latestOld = p
			}
		}
		for _, p := range newPos {
			if p.less(earliestNew) {
				earliestNew = p
			}
		}
		if !latestOld.less(earliestNew) {
			return fmt.Errorf("%s: new_evidence must come after all old_evidence", what)
		}
		trigger, _, err := a.turnPos(c.TriggerTurn)
		if err != nil {
			return fmt.Errorf("%s trigger_turn: %w", what, err)
		}
		if !latestOld.less(trigger) || !trigger.less(earliestNew) {
			return fmt.Errorf("%s: trigger_turn must fall between old and new evidence", what)
		}
		ch := &Change{GID: gid, Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: a.nextOrder(),
			Concept: c.Concept, OldModel: c.OldModel, Trigger: c.Trigger, TriggerTurn: c.TriggerTurn, NewModel: c.NewModel,
			OldEvidence: oldRefs, NewEvidence: newRefs}
		a.m.Changes = append(a.m.Changes, ch)
		a.m.items[gid] = ch
		rev := &Event{GID: gid + "#revision", Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: ch.Order,
			Type: "understanding_revision", Concept: c.Concept, Summary: c.OldModel + " → " + c.NewModel, Evidence: newRefs, DerivedBy: gid}
		a.m.Events = append(a.m.Events, rev)
		a.m.items[rev.GID] = rev
	}
	return nil
}

func (a *applier) attempts() error {
	for _, s := range a.rec.StrategyAttempts {
		gid, skip, err := a.claim("strategy attempt", s.ID, s)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "strategy attempt " + s.ID
		switch {
		case !Strategies[s.Strategy]:
			return fmt.Errorf("%s has unsupported strategy %q", what, s.Strategy)
		case !Situations[s.Situation]:
			return fmt.Errorf("%s has unsupported situation %q", what, s.Situation)
		case !Outcomes[s.Outcome]:
			return fmt.Errorf("%s has unsupported outcome %q", what, s.Outcome)
		}
		if err := a.requireConcept(what, s.Concept); err != nil {
			return err
		}
		if err := nonEmpty(what, map[string]string{"action_turn": s.ActionTurn, "expected_change": s.ExpectedChange}); err != nil {
			return err
		}
		if strings.Contains(s.ActionTurn, "#") {
			return fmt.Errorf("%s: action_turn must be in the current session", what)
		}
		action, actionTurn, err := a.turnPos(s.ActionTurn)
		if err != nil {
			return fmt.Errorf("%s action_turn: %w", what, err)
		}
		if actionTurn.Role != "assistant" {
			return fmt.Errorf("%s: action_turn must be an assistant turn", what)
		}
		refs, positions, err := a.evidence(what, s.Evidence)
		if err != nil {
			return err
		}
		for _, p := range positions {
			if p.session != action.session || p.ordinal <= action.ordinal {
				return fmt.Errorf("%s: evidence must come after action_turn in the same session", what)
			}
		}
		linked := make([]string, 0, len(s.Linked))
		positive := false
		for _, ref := range s.Linked {
			g := a.resolveRef(ref)
			switch item := a.m.items[g].(type) {
			case *Change:
				positive = true
			case *Event:
				positive = positive || positiveEvents[item.Type]
			default:
				return fmt.Errorf("%s links unknown event or change %q", what, ref)
			}
			linked = append(linked, g)
		}
		if s.Outcome == "effective" && !positive {
			return fmt.Errorf("%s: effective outcome must link a cognitive change or a positive learning event", what)
		}
		if s.Outcome != "effective" && strings.TrimSpace(s.Reason) == "" {
			return fmt.Errorf("%s: %s outcome requires a reason", what, s.Outcome)
		}
		at := &Attempt{GID: gid, Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: a.nextOrder(),
			Strategy: s.Strategy, Situation: s.Situation, Concept: s.Concept, ActionTurn: s.ActionTurn, ExpectedChange: s.ExpectedChange,
			Outcome: s.Outcome, Reason: s.Reason, Linked: linked, Evidence: refs, SwitchReason: s.SwitchReason}
		if s.Replaces != "" {
			prevGID := a.resolveRef(s.Replaces)
			prev, ok := a.m.items[prevGID].(*Attempt)
			if !ok || prevGID == gid {
				return fmt.Errorf("%s replaces unknown strategy attempt %q", what, s.Replaces)
			}
			if strings.TrimSpace(s.SwitchReason) == "" {
				return fmt.Errorf("%s: replaces requires switch_reason", what)
			}
			at.Replaces = prevGID
			sw := &Event{GID: gid + "#switch", Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: at.Order,
				Type: "strategy_switch", Concept: s.Concept, Summary: prev.Strategy + " → " + s.Strategy + "：" + s.SwitchReason,
				Evidence: refs, DerivedBy: gid}
			a.m.Events = append(a.m.Events, sw)
			a.m.items[sw.GID] = sw
		} else if s.SwitchReason != "" {
			return fmt.Errorf("%s: switch_reason requires replaces", what)
		}
		a.m.Attempts = append(a.m.Attempts, at)
		a.m.items[gid] = at
	}
	return nil
}

func (a *applier) stateUpdates() error {
	for _, u := range a.rec.StateUpdates {
		gid, skip, err := a.claim("state update", u.ID, u)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "state update " + u.ID
		if u.State == "unobserved" {
			return fmt.Errorf("%s: unobserved can only result from a retraction", what)
		}
		if u.State == "solid" {
			return fmt.Errorf("%s: state solid was replaced by stable in v0.1.2", what)
		}
		if !States[u.State] {
			return fmt.Errorf("%s has unsupported state %q", what, u.State)
		}
		if err := a.requireConcept(what, u.Concept); err != nil {
			return err
		}
		if len(u.Capabilities) == 0 {
			return fmt.Errorf("%s requires capabilities", what)
		}
		caps := map[string]bool{}
		for _, c := range u.Capabilities {
			if !Capabilities[c] {
				return fmt.Errorf("%s has unsupported capability %q", what, c)
			}
			caps[c] = true
		}
		if err := nonEmpty(what, map[string]string{"summary": u.Summary}); err != nil {
			return err
		}
		refs, positions, err := a.evidence(what, u.Evidence)
		if err != nil {
			return err
		}
		concept := a.m.Concepts[u.Concept]
		if err := a.gate(what, concept, u.State, caps, positions); err != nil {
			return err
		}
		entry := &StateEntry{GID: gid, Order: a.nextOrder(), Session: a.env.Session, At: a.env.SubmittedAt, State: u.State,
			Capabilities: sortedKeys(caps), Summary: u.Summary, Evidence: refs, OpenQuestions: u.OpenQuestions}
		concept.History = append(concept.History, entry)
		a.m.items[gid] = entry
	}
	return nil
}

// gate enforces the minimum evidence for each qualitative state.
func (a *applier) gate(what string, c *Concept, state string, caps map[string]bool, positions []pos) error {
	switch state {
	case "developing":
		if !caps["explained"] && !caps["predicted"] && !caps["applied"] {
			return fmt.Errorf("%s: developing needs explained, predicted or applied evidence; recognition alone is not enough", what)
		}
	case "fragile":
		for _, ev := range a.m.Events {
			if ev.Superseded == "" && ev.Concept == c.ID && ev.Type == "misconception" {
				return nil
			}
		}
		for _, at := range a.m.Attempts {
			if at.Superseded == "" && at.Concept == c.ID && at.Outcome == "ineffective" {
				return nil
			}
		}
		return fmt.Errorf("%s: fragile needs a misconception or an ineffective attempt on this concept", what)
	case "stable":
		if !caps["retrieved"] && !caps["transferred"] {
			return fmt.Errorf("%s: stable needs retrieved or transferred evidence", what)
		}
		baseline := ""
		for _, e := range c.History {
			if e.Superseded == "" && containsFold(e.Capabilities, "explained") {
				baseline = e.Session
				break
			}
		}
		hadDeveloping := false
		for _, e := range c.History {
			if e.Superseded == "" && (e.State == "developing" || e.State == "stable") {
				hadDeveloping = true
				if baseline == "" {
					baseline = e.Session
				}
			}
		}
		if !hadDeveloping {
			return fmt.Errorf("%s: stable needs an earlier developing state", what)
		}
		for _, p := range positions {
			if p.session > baseline {
				return nil
			}
		}
		return fmt.Errorf("%s: stable needs retrieval or transfer evidence from a session after %s", what, baseline)
	}
	return nil
}

// relations runs after every concept of the record is declared.
func (a *applier) relations() error {
	for _, c := range a.rec.Concepts {
		for _, r := range c.Related {
			what := fmt.Sprintf("concept %s related %s", c.ID, r.Concept)
			if r.Concept == c.ID {
				return fmt.Errorf("%s: a concept cannot relate to itself", what)
			}
			if err := a.requireConcept(what, r.Concept); err != nil {
				return err
			}
			if n := utf8.RuneCountInString(r.Note); n > 40 {
				return fmt.Errorf("%s: note must be at most 40 characters", what)
			}
			from, to := a.m.Concepts[c.ID], a.m.Concepts[r.Concept]
			if hasRelation(from, to.ID) {
				continue
			}
			from.Related = append(from.Related, RelatedEntry{Concept: to.ID, Note: r.Note})
			to.Related = append(to.Related, RelatedEntry{Concept: from.ID, Note: r.Note})
		}
	}
	return nil
}

// noRelated records that the Agent checked a concept and found no related
// concept yet, so the end-of-session relation check stops asking.
func (a *applier) noRelated() error {
	for _, id := range a.rec.NoRelated {
		if err := a.requireConcept("no_related", id); err != nil {
			return err
		}
		a.m.Concepts[id].NoRelated = true
	}
	return nil
}

func hasRelation(c *Concept, other string) bool {
	for _, r := range c.Related {
		if r.Concept == other {
			return true
		}
	}
	return false
}

func (a *applier) reviews() error {
	for _, r := range a.rec.ReviewResults {
		gid, skip, err := a.claim("review result", r.ID, r)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "review result " + r.ID
		if !ReviewOutcomes[r.Outcome] {
			return fmt.Errorf("%s has unsupported outcome %q", what, r.Outcome)
		}
		if err := a.requireConcept(what, r.Concept); err != nil {
			return err
		}
		if r.ActionTurn == "" || strings.Contains(r.ActionTurn, "#") {
			return fmt.Errorf("%s: action_turn must be an assistant turn in the current session", what)
		}
		action, turn, err := a.turnPos(r.ActionTurn)
		if err != nil {
			return fmt.Errorf("%s action_turn: %w", what, err)
		}
		if turn.Role != "assistant" {
			return fmt.Errorf("%s: action_turn must be an assistant turn", what)
		}
		refs, positions, err := a.evidence(what, r.Evidence)
		if err != nil {
			return err
		}
		for _, p := range positions {
			if p.session != action.session || p.ordinal <= action.ordinal {
				return fmt.Errorf("%s: evidence must come after action_turn in the same session", what)
			}
		}
		rv := &Review{GID: gid, Session: a.env.Session, At: a.env.SubmittedAt, Order: a.nextOrder(), Concept: r.Concept, Outcome: r.Outcome, Evidence: refs}
		a.m.Reviews = append(a.m.Reviews, rv)
		a.m.items[gid] = rv
	}
	return nil
}

func (a *applier) patterns() error {
	for _, o := range a.rec.PatternObservations {
		gid, skip, err := a.claim("pattern observation", o.ID, o)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		what := "pattern observation " + o.ID
		if !slugPattern.MatchString(o.Pattern) {
			return fmt.Errorf("%s: pattern %q must be a kebab-case slug", what, o.Pattern)
		}
		if o.Stance != "supports" && o.Stance != "contradicts" {
			return fmt.Errorf("%s has unsupported stance %q", what, o.Stance)
		}
		if o.PreferredStrategy != "" && !Strategies[o.PreferredStrategy] {
			return fmt.Errorf("%s has unsupported preferred_strategy %q", what, o.PreferredStrategy)
		}
		if o.Situation != "" && !Situations[o.Situation] {
			return fmt.Errorf("%s has unsupported situation %q", what, o.Situation)
		}
		if err := nonEmpty(what, map[string]string{"summary": o.Summary}); err != nil {
			return err
		}
		p := a.m.Patterns[o.Pattern]
		if p == nil {
			if strings.TrimSpace(o.Description) == "" {
				return fmt.Errorf("%s: first observation of pattern %q requires a description", what, o.Pattern)
			}
			p = &Pattern{ID: o.Pattern, Description: o.Description, PreferredStrategy: o.PreferredStrategy, Situation: o.Situation, Observations: []*Observation{}}
			a.m.Patterns[o.Pattern] = p
		} else if (o.Description != "" && o.Description != p.Description) ||
			(o.PreferredStrategy != "" && o.PreferredStrategy != p.PreferredStrategy) ||
			(o.Situation != "" && o.Situation != p.Situation) {
			return fmt.Errorf("%s: pattern %q definition cannot change", what, o.Pattern)
		}
		if len(o.Linked) == 0 {
			return fmt.Errorf("%s must link at least one event or change", what)
		}
		linked := make([]string, 0, len(o.Linked))
		for _, ref := range o.Linked {
			g := a.resolveRef(ref)
			switch a.m.items[g].(type) {
			case *Event, *Change:
			default:
				return fmt.Errorf("%s links unknown event or change %q", what, ref)
			}
			linked = append(linked, g)
		}
		refs, _, err := a.evidence(what, o.Evidence)
		if err != nil {
			return err
		}
		obs := &Observation{GID: gid, Session: a.env.Session, Curriculum: a.rec.Curriculum, At: a.env.SubmittedAt, Order: a.nextOrder(),
			Stance: o.Stance, Summary: o.Summary, Linked: linked, Evidence: refs}
		p.Observations = append(p.Observations, obs)
		a.m.items[gid] = obs
	}
	return nil
}

func (a *applier) retractions() error {
	for _, r := range a.rec.Retractions {
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("retraction of %q requires a reason", r.Ref)
		}
		reason := "retracted: " + r.Reason
		switch item := a.m.items[r.Ref].(type) {
		case *Event:
			if item.DerivedBy != "" {
				return fmt.Errorf("retraction target %q is derived; retract %q instead", r.Ref, item.DerivedBy)
			}
			item.Superseded = reason
		case *Change:
			item.Superseded = reason
			a.supersedeDerived(r.Ref, reason)
		case *Attempt:
			item.Superseded = reason
			a.supersedeDerived(r.Ref, reason)
		case *StateEntry:
			item.Superseded = reason
		default:
			return fmt.Errorf("retraction target %q is not an accepted event, change, state update or attempt", r.Ref)
		}
		a.m.Retractions = append(a.m.Retractions, &Retraction{Ref: r.Ref, Reason: r.Reason, Session: a.env.Session, At: a.env.SubmittedAt})
	}
	return nil
}

func (a *applier) supersedeDerived(gid, reason string) {
	for _, ev := range a.m.Events {
		if ev.DerivedBy == gid {
			ev.Superseded = reason
		}
	}
}

func (a *applier) progress() error {
	d := a.rec.ProgressDecision
	if d == nil {
		return nil
	}
	if a.env.Kind != "end" {
		return fmt.Errorf("progress_decision is only allowed when ending a session")
	}
	if !Decisions[d.Decision] {
		return fmt.Errorf("progress_decision has unsupported decision %q", d.Decision)
	}
	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("progress_decision requires a reason")
	}
	a.m.Progress = append(a.m.Progress, &Progress{Session: a.env.Session, At: a.env.SubmittedAt, Decision: d.Decision, Reason: d.Reason})
	return nil
}

func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if fold(item) == fold(v) {
			return true
		}
	}
	return false
}
