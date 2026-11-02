package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nethercode/heron/internal/broker"
	"github.com/nethercode/heron/internal/maintenance"
	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/notify"
	"github.com/nethercode/heron/internal/scheduler"
	"github.com/nethercode/heron/internal/store"
)

type EventType string

const (
	EventMonitor  EventType = "monitor"
	EventIncident EventType = "incident"
	EventLayout   EventType = "layout"
)

type Event struct {
	Type      EventType
	MonitorID int64
	Slug      string
	Public    bool
	Status    monitor.Status
	Latency   time.Duration
	Message   string
	At        time.Time
}

type Notifier interface {
	Dispatch(channels []notify.Channel, msg notify.Message)
}

type Engine struct {
	store     *store.Store
	notifier  Notifier
	events    *broker.Broker[Event]
	calendar  *maintenance.Calendar
	log       *slog.Logger
	publicURL atomic.Pointer[string]

	mu   sync.Mutex
	last map[int64]monitor.Status
}

func New(s *store.Store, n Notifier, calendar *maintenance.Calendar, log *slog.Logger, publicURL string) *Engine {
	e := &Engine{
		store:    s,
		notifier: n,
		events:   broker.New[Event](32),
		calendar: calendar,
		log:      log,
		last:     map[int64]monitor.Status{},
	}
	e.SetPublicURL(publicURL)
	return e
}

func (e *Engine) Events() *broker.Broker[Event] {
	return e.events
}

func (e *Engine) Calendar() *maintenance.Calendar {
	return e.calendar
}

func (e *Engine) ReloadMaintenance(ctx context.Context) error {
	windows, err := e.store.MaintenanceWindows(ctx)
	if err != nil {
		return err
	}
	e.calendar.Set(windows)
	e.events.Publish(Event{Type: EventLayout, Public: true})
	return nil
}

func (e *Engine) Record(ctx context.Context, beat scheduler.Beat) {
	m, res := beat.Monitor, beat.Result
	log := e.log.With("monitor", m.Slug)

	current := beat.Current
	if _, ok := e.calendar.Active(m.ID, res.At); ok {
		current = monitor.StatusMaintenance
		if !res.Status.Healthy() {
			res.Status = monitor.StatusMaintenance
		}
	}

	e.mu.Lock()
	previous, seen := e.last[m.ID]
	if !seen {
		previous = beat.Previous
	}
	e.last[m.ID] = current
	e.mu.Unlock()

	if err := e.store.Record(ctx, m.ID, res, current); err != nil {
		log.Error("record result", "err", err)
		return
	}

	switch {
	case current == monitor.StatusDown && previous != monitor.StatusDown:
		log.Warn("monitor is down", "reason", res.Message)
		e.wentDown(ctx, m, res)
	case previous == monitor.StatusDown && current.Healthy():
		log.Info("monitor recovered", "latency", res.Latency)
		e.recovered(ctx, m, res)
	case previous == monitor.StatusDown && current == monitor.StatusMaintenance:
		log.Info("maintenance started while down")
		e.resolve(ctx, m, res.At, func(time.Duration) string {
			return "Scheduled maintenance has started for this service."
		})
	case previous != current:
		log.Info("status changed", "from", previous, "to", current)
	}

	e.events.Publish(Event{
		Type:      EventMonitor,
		MonitorID: m.ID,
		Slug:      m.Slug,
		Public:    m.Public,
		Status:    current,
		Latency:   res.Latency,
		Message:   res.Message,
		At:        res.At,
	})
}

func (e *Engine) Forget(id int64) {
	e.mu.Lock()
	delete(e.last, id)
	e.mu.Unlock()
}

func (e *Engine) WatchMaintenance(ctx context.Context) {
	var active []int64
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		var now []int64
		for _, s := range e.calendar.Upcoming(time.Now(), 0) {
			if s.Active {
				now = append(now, s.ID)
			}
		}
		if !slices.Equal(now, active) {
			e.events.Publish(Event{Type: EventLayout, Public: true})
		}
		active = now

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *Engine) wentDown(ctx context.Context, m *monitor.Monitor, res monitor.Result) {
	_, err := e.store.OpenAutomaticIncident(ctx, m.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		inc := &store.Incident{
			Title:      m.Name + " is unavailable",
			Impact:     store.ImpactMajor,
			Status:     store.IncidentInvestigating,
			Automatic:  true,
			MonitorIDs: []int64{m.ID},
			StartedAt:  res.At.UTC().Truncate(time.Second),
		}
		body := m.Name + " is not responding. We are looking into it."
		if err := e.store.CreateIncident(ctx, inc, body); err != nil {
			e.log.Error("open incident", "monitor", m.Slug, "err", err)
		} else {
			e.events.Publish(Event{Type: EventIncident})
		}
	case err != nil:
		e.log.Error("find incident", "monitor", m.Slug, "err", err)
	}

	e.notify(ctx, m, notify.Message{
		Event:  notify.EventDown,
		Status: monitor.StatusDown,
		Reason: res.Message,
		At:     res.At,
	})
}

func (e *Engine) recovered(ctx context.Context, m *monitor.Monitor, res monitor.Result) {
	downtime := e.resolve(ctx, m, res.At, func(d time.Duration) string {
		return fmt.Sprintf("Service recovered after %s.", d.Round(time.Second))
	})

	e.notify(ctx, m, notify.Message{
		Event:    notify.EventRecovered,
		Status:   res.Status,
		Reason:   res.Message,
		At:       res.At,
		Downtime: downtime,
	})
}

func (e *Engine) resolve(ctx context.Context, m *monitor.Monitor, at time.Time, body func(time.Duration) string) time.Duration {
	inc, err := e.store.OpenAutomaticIncident(ctx, m.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			e.log.Error("find incident", "monitor", m.Slug, "err", err)
		}
		return 0
	}
	downtime := at.Sub(inc.StartedAt)
	if _, err := e.store.AddIncidentUpdate(ctx, inc.ID, store.IncidentResolved, body(downtime)); err != nil {
		e.log.Error("resolve incident", "monitor", m.Slug, "err", err)
		return downtime
	}
	e.events.Publish(Event{Type: EventIncident})
	return downtime
}

func (e *Engine) notify(ctx context.Context, m *monitor.Monitor, msg notify.Message) {
	channels, err := e.store.ChannelsForMonitor(ctx, m.ID)
	if err != nil {
		e.log.Error("load channels", "monitor", m.Slug, "err", err)
		return
	}
	if len(channels) == 0 {
		return
	}

	msg.Monitor = m
	msg.Link = e.Link(m)
	list := make([]notify.Channel, 0, len(channels))
	for _, c := range channels {
		list = append(list, *c)
	}
	e.notifier.Dispatch(list, msg)
}

func (e *Engine) SetPublicURL(url string) {
	e.publicURL.Store(&url)
}

func (e *Engine) Link(m *monitor.Monitor) string {
	base := ""
	if p := e.publicURL.Load(); p != nil {
		base = *p
	}
	if base == "" {
		return ""
	}
	if m == nil || !m.Public {
		return base
	}
	return base + "/monitor/" + m.Slug
}

func (e *Engine) Changed(m *monitor.Monitor) {
	e.events.Publish(Event{
		Type:      EventLayout,
		MonitorID: m.ID,
		Slug:      m.Slug,
		Public:    m.Public,
		Status:    m.State.Status,
	})
}

func (e *Engine) IncidentsChanged() {
	e.events.Publish(Event{Type: EventIncident})
}

func (e *Engine) Housekeep(ctx context.Context, retention time.Duration) {
	run := func() {
		now := time.Now()
		removed, err := e.store.Prune(ctx, now.Add(-retention), now.AddDate(-1, -1, 0))
		if err != nil {
			e.log.Error("prune", "err", err)
			return
		}
		if removed > 0 {
			e.log.Debug("pruned old results", "count", removed)
		}
	}

	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
