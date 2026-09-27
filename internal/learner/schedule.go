package learner

import (
	"sort"
	"time"
)

// ReviewIntervals are the Ebbinghaus-style spacing steps in days.
var ReviewIntervals = []int{1, 2, 4, 7, 15, 30, 60}

// Schedule is a concept's spaced-review state derived from its history.
type Schedule struct {
	Concept    string `json:"concept"`
	Started    bool   `json:"started"`
	Level      int    `json:"level"`
	Interval   int    `json:"interval_days"`
	Due        string `json:"due,omitempty"` // local YYYY-MM-DD
	LastResult string `json:"last_result,omitempty"`
	LastReview string `json:"last_review,omitempty"`
	Reviews    int    `json:"reviews"`
	Evidence   string `json:"evidence,omitempty"` // gid of the entry that set Due
}

type scheduleEvent struct {
	at    time.Time
	order int
	state string
	rev   *Review
	gid   string
}

func localDay(ts string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, false
	}
	l := t.Local()
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.Local), true
}

// ScheduleFor replays state entries and review results in order.
func (m *Model) ScheduleFor(c *Concept) Schedule {
	s := Schedule{Concept: c.ID}
	var evs []scheduleEvent
	for _, h := range c.History {
		if h.Superseded != "" {
			continue
		}
		if day, ok := localDay(h.At); ok {
			evs = append(evs, scheduleEvent{at: day, order: h.Order, state: h.State, gid: h.GID})
		}
	}
	for _, r := range m.Reviews {
		if r.Concept != c.ID {
			continue
		}
		if day, ok := localDay(r.At); ok {
			evs = append(evs, scheduleEvent{at: day, order: r.Order, rev: r, gid: r.GID})
		}
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if !evs[i].at.Equal(evs[j].at) {
			return evs[i].at.Before(evs[j].at)
		}
		return evs[i].order < evs[j].order
	})
	due := time.Time{}
	set := func(day time.Time, level int, gid string) {
		if level >= len(ReviewIntervals) {
			level = len(ReviewIntervals) - 1
		}
		s.Level, s.Interval, s.Evidence = level, ReviewIntervals[level], gid
		due = day.AddDate(0, 0, s.Interval)
	}
	for _, e := range evs {
		switch {
		case e.rev != nil:
			if !s.Started {
				s.Started = true
				s.Level = 0
			}
			s.Reviews++
			s.LastResult, s.LastReview = e.rev.Outcome, e.at.Format("2006-01-02")
			switch e.rev.Outcome {
			case "recalled":
				set(e.at, s.Level+1, e.gid)
			case "partial":
				set(e.at, s.Level, e.gid)
			default:
				set(e.at, 0, e.gid)
			}
		case e.state == "fragile":
			s.Started = true
			set(e.at, 0, e.gid)
		case !s.Started:
			s.Started = true
			set(e.at, 0, e.gid)
		}
	}
	if s.Started {
		s.Due = due.Format("2006-01-02")
	}
	return s
}

// Schedules returns every started schedule ordered by due date, then id.
func (m *Model) Schedules() []Schedule {
	var out []Schedule
	for _, c := range m.ConceptList() {
		if s := m.ScheduleFor(c); s.Started {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Due != out[j].Due {
			return out[i].Due < out[j].Due
		}
		return out[i].Concept < out[j].Concept
	})
	return out
}

// ReviewedIn reports whether a concept already has a review in a session.
func (m *Model) ReviewedIn(concept, session string) bool {
	for _, r := range m.Reviews {
		if r.Concept == concept && r.Session == session {
			return true
		}
	}
	return false
}
