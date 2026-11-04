package engine

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
)

type Policy struct {
	FlapCount   int
	FlapWindow  time.Duration
	StableAfter time.Duration
	MaxHold     time.Duration
	Recheck     time.Duration
}

func DefaultPolicy() Policy {
	return Policy{
		FlapCount:   6,
		FlapWindow:  time.Hour,
		StableAfter: 30 * time.Minute,
		MaxHold:     5 * time.Minute,
		Recheck:     2 * time.Minute,
	}
}

type guard struct {
	mu       sync.Mutex
	held     map[int64]*time.Timer
	covered  map[int64]int64
	flips    map[int64][]time.Time
	flapping map[int64]bool
}

func newGuard() *guard {
	return &guard{
		held:     map[int64]*time.Timer{},
		covered:  map[int64]int64{},
		flips:    map[int64][]time.Time{},
		flapping: map[int64]bool{},
	}
}

func (e *Engine) status(ctx context.Context, id int64) (*monitor.Monitor, monitor.Status) {
	m, err := e.store.Monitor(ctx, id)
	if err != nil {
		return nil, ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.last[id]; ok {
		return m, s
	}
	return m, m.State.Status
}

func (e *Engine) down(ctx context.Context, m *monitor.Monitor, res monitor.Result) {
	quiet := e.flip(ctx, m, res.At)
	if parent := m.Settings.DependsOn; parent != 0 {
		p, state := e.status(ctx, parent)
		switch {
		case p == nil:
		case state == monitor.StatusDown:
			e.cover(m.ID, parent)
			e.log.Info("parent is down, holding the alert back", "monitor", m.Slug, "parent", p.Slug)
			return
		default:
			wait := min(p.Interval*time.Duration(p.Retries+1)+15*time.Second, e.policy.MaxHold)
			e.hold(m, res, parent, wait)
			return
		}
	}
	e.wentDown(ctx, m, res, quiet)
}

func (e *Engine) up(ctx context.Context, m *monitor.Monitor, res monitor.Result) {
	quiet := e.flip(ctx, m, res.At)
	if e.release(m.ID) {
		return
	}
	e.recovered(ctx, m, res, quiet)
	e.recheckDependents(m.ID)
}

func (e *Engine) hold(m *monitor.Monitor, res monitor.Result, parent int64, wait time.Duration) {
	e.guard.mu.Lock()
	defer e.guard.mu.Unlock()
	if t := e.guard.held[m.ID]; t != nil {
		t.Stop()
	}
	e.guard.held[m.ID] = time.AfterFunc(wait, func() {
		e.guard.mu.Lock()
		delete(e.guard.held, m.ID)
		e.guard.mu.Unlock()

		ctx := context.Background()
		_, mine := e.status(ctx, m.ID)
		if mine != monitor.StatusDown {
			return
		}
		if _, state := e.status(ctx, parent); state == monitor.StatusDown {
			e.cover(m.ID, parent)
			return
		}
		e.wentDown(ctx, m, res, e.isFlapping(m.ID))
	})
}

func (e *Engine) cover(id, parent int64) {
	e.guard.mu.Lock()
	e.guard.covered[id] = parent
	e.guard.mu.Unlock()
}

func (e *Engine) release(id int64) bool {
	e.guard.mu.Lock()
	defer e.guard.mu.Unlock()
	if t := e.guard.held[id]; t != nil {
		t.Stop()
		delete(e.guard.held, id)
		return true
	}
	if _, ok := e.guard.covered[id]; ok {
		delete(e.guard.covered, id)
		return true
	}
	return false
}

func (e *Engine) recheckDependents(parent int64) {
	e.guard.mu.Lock()
	var children []int64
	for id, p := range e.guard.covered {
		if p == parent {
			children = append(children, id)
		}
	}
	e.guard.mu.Unlock()
	if len(children) == 0 {
		return
	}

	time.AfterFunc(e.policy.Recheck, func() {
		ctx := context.Background()
		for _, id := range children {
			m, state := e.status(ctx, id)
			if m == nil || state != monitor.StatusDown {
				continue
			}
			e.guard.mu.Lock()
			_, still := e.guard.covered[id]
			delete(e.guard.covered, id)
			e.guard.mu.Unlock()
			if still {
				e.log.Warn("still down after its parent came back", "monitor", m.Slug)
				e.wentDown(ctx, m, monitor.Result{Status: monitor.StatusDown, Message: m.State.Message, At: time.Now()}, e.isFlapping(id))
			}
		}
	})
}

func (e *Engine) dependents(ctx context.Context, id int64) int {
	list, err := e.store.Monitors(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, m := range list {
		if m.Settings.DependsOn == id && !m.Paused {
			n++
		}
	}
	return n
}

func (e *Engine) isFlapping(id int64) bool {
	e.guard.mu.Lock()
	defer e.guard.mu.Unlock()
	return e.guard.flapping[id]
}

func (e *Engine) flip(ctx context.Context, m *monitor.Monitor, at time.Time) bool {
	e.guard.mu.Lock()
	cutoff := at.Add(-e.policy.FlapWindow)
	flips := slices.DeleteFunc(e.guard.flips[m.ID], func(t time.Time) bool { return t.Before(cutoff) })
	flips = append(flips, at)
	e.guard.flips[m.ID] = flips
	if e.guard.flapping[m.ID] {
		e.guard.mu.Unlock()
		return true
	}
	if len(flips) < e.policy.FlapCount {
		e.guard.mu.Unlock()
		return false
	}
	e.guard.flapping[m.ID] = true
	e.guard.mu.Unlock()

	e.log.Warn("monitor is flapping, holding alerts until it settles", "monitor", m.Slug, "changes", len(flips))
	e.notify(ctx, m, notify.Message{
		Event:  notify.EventFlapping,
		Status: m.State.Status,
		Reason: fmt.Sprintf("It changed state %d times in the last %s. heron sends no more alerts for it until it stays put for %s.", len(flips), human(e.policy.FlapWindow), human(e.policy.StableAfter)),
		At:     at,
	})
	return true
}

func (e *Engine) settle(ctx context.Context, m *monitor.Monitor, current monitor.Status, at time.Time) {
	e.guard.mu.Lock()
	if !e.guard.flapping[m.ID] {
		e.guard.mu.Unlock()
		return
	}
	flips := e.guard.flips[m.ID]
	if len(flips) > 0 && at.Sub(flips[len(flips)-1]) < e.policy.StableAfter {
		e.guard.mu.Unlock()
		return
	}
	delete(e.guard.flapping, m.ID)
	delete(e.guard.flips, m.ID)
	e.guard.mu.Unlock()

	reason := "It has been up for " + human(e.policy.StableAfter) + "."
	if current == monitor.StatusDown {
		reason = "It has been down for " + human(e.policy.StableAfter) + "."
	}
	e.notify(ctx, m, notify.Message{Event: notify.EventStable, Status: current, Reason: reason, At: at})
}

func human(d time.Duration) string {
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		if d == time.Hour {
			return "hour"
		}
		return fmt.Sprintf("%d hours", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return d.String()
}
