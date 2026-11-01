package maintenance

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nethercode/heron/internal/monitor"
)

type Repeat string

const (
	RepeatNone   Repeat = "none"
	RepeatDaily  Repeat = "daily"
	RepeatWeekly Repeat = "weekly"
)

var Repeats = []Repeat{RepeatNone, RepeatDaily, RepeatWeekly}

const (
	MinDuration = time.Minute
	MaxDuration = 30 * 24 * time.Hour
)

type Window struct {
	ID         int64
	Title      string
	Body       string
	MonitorIDs []int64
	StartsAt   time.Time
	Duration   time.Duration
	Repeat     Repeat
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Occurrence struct {
	Start time.Time
	End   time.Time
}

func (o Occurrence) Contains(t time.Time) bool {
	return !t.Before(o.Start) && t.Before(o.End)
}

func (w *Window) Validate() error {
	var errs []error
	if strings.TrimSpace(w.Title) == "" || len(w.Title) > 200 {
		errs = append(errs, &monitor.FieldError{Field: "title", Message: "must be between 1 and 200 characters"})
	}
	if w.StartsAt.IsZero() {
		errs = append(errs, &monitor.FieldError{Field: "starts_at", Message: "is required"})
	}
	if w.Duration < MinDuration || w.Duration > MaxDuration {
		errs = append(errs, &monitor.FieldError{Field: "duration", Message: "must be between 1 minute and 30 days"})
	}
	if !slices.Contains(Repeats, w.Repeat) {
		errs = append(errs, &monitor.FieldError{Field: "repeat", Message: "must be none, daily or weekly"})
	}
	if step := w.step(); step > 0 && w.Duration >= time.Duration(step)*24*time.Hour {
		errs = append(errs, &monitor.FieldError{Field: "duration", Message: "must be shorter than the repeat interval"})
	}
	if len(w.MonitorIDs) == 0 {
		errs = append(errs, &monitor.FieldError{Field: "monitor_ids", Message: "select at least one monitor"})
	}
	return errors.Join(errs...)
}

func (w *Window) step() int {
	switch w.Repeat {
	case RepeatDaily:
		return 1
	case RepeatWeekly:
		return 7
	}
	return 0
}

func (w *Window) Next(at time.Time, loc *time.Location) (Occurrence, bool) {
	base := w.StartsAt.In(loc)
	first := Occurrence{Start: base, End: base.Add(w.Duration)}
	step := w.step()
	if step == 0 || at.Before(first.End) {
		return first, at.Before(first.End)
	}

	k := int(at.Sub(base).Hours()/24) / step
	for i := max(k-1, 0); ; i++ {
		start := base.AddDate(0, 0, i*step)
		occ := Occurrence{Start: start, End: start.Add(w.Duration)}
		if at.Before(occ.End) {
			return occ, true
		}
	}
}

func (w *Window) ActiveAt(t time.Time, loc *time.Location) bool {
	if !w.Enabled {
		return false
	}
	occ, ok := w.Next(t, loc)
	return ok && occ.Contains(t)
}

func (w *Window) Covers(monitorID int64) bool {
	return slices.Contains(w.MonitorIDs, monitorID)
}

type Calendar struct {
	mu      sync.RWMutex
	loc     *time.Location
	windows []Window
}

func NewCalendar(loc *time.Location) *Calendar {
	if loc == nil {
		loc = time.UTC
	}
	return &Calendar{loc: loc}
}

func (c *Calendar) Location() *time.Location {
	return c.loc
}

func (c *Calendar) Set(windows []Window) {
	c.mu.Lock()
	c.windows = slices.Clone(windows)
	c.mu.Unlock()
}

func (c *Calendar) Active(monitorID int64, t time.Time) (Window, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, w := range c.windows {
		if w.Covers(monitorID) && w.ActiveAt(t, c.loc) {
			return w, true
		}
	}
	return Window{}, false
}

type Scheduled struct {
	Window
	Occurrence
	Active bool
}

func (c *Calendar) Upcoming(now time.Time, horizon time.Duration) []Scheduled {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var out []Scheduled
	for _, w := range c.windows {
		if !w.Enabled {
			continue
		}
		occ, ok := w.Next(now, c.loc)
		if !ok || occ.Start.After(now.Add(horizon)) {
			continue
		}
		out = append(out, Scheduled{Window: w, Occurrence: occ, Active: occ.Contains(now)})
	}
	slices.SortFunc(out, func(a, b Scheduled) int { return a.Start.Compare(b.Start) })
	return out
}
