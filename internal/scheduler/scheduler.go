package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
)

type Beat struct {
	Monitor  *monitor.Monitor
	Result   monitor.Result
	Previous monitor.Status
	Current  monitor.Status
}

func (b Beat) WentDown() bool {
	return b.Current == monitor.StatusDown && b.Previous != monitor.StatusDown
}

func (b Beat) Recovered() bool {
	return b.Previous == monitor.StatusDown && b.Current.Healthy()
}

type Sink interface {
	Record(ctx context.Context, beat Beat)
}

type Scheduler struct {
	checker monitor.Checker
	sink    Sink
	log     *slog.Logger

	mu   sync.Mutex
	ctx  context.Context
	runs map[int64]*run
	wg   sync.WaitGroup
}

type run struct {
	monitor *monitor.Monitor
	cancel  context.CancelFunc
	trigger chan struct{}
	push    chan monitor.Result
}

func New(checker monitor.Checker, sink Sink, log *slog.Logger) *Scheduler {
	return &Scheduler{checker: checker, sink: sink, log: log, runs: map[int64]*run{}}
}

func (s *Scheduler) Start(ctx context.Context, monitors []*monitor.Monitor) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()

	for _, m := range monitors {
		s.Sync(m)
	}
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) Sync(m *monitor.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopLocked(m.ID)
	if m.Paused || s.ctx == nil || s.ctx.Err() != nil {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	r := &run{
		monitor: m,
		cancel:  cancel,
		trigger: make(chan struct{}, 1),
		push:    make(chan monitor.Result, 1),
	}
	s.runs[m.ID] = r

	s.wg.Go(func() {
		log := s.log.With("monitor", m.Slug, "kind", m.Kind)
		log.Debug("monitor started", "interval", m.Interval)
		if m.Kind == monitor.KindPush {
			s.watch(ctx, r)
		} else {
			s.poll(ctx, r)
		}
		log.Debug("monitor stopped")
	})
}

func (s *Scheduler) Stop(id int64) {
	s.mu.Lock()
	s.stopLocked(id)
	s.mu.Unlock()
}

func (s *Scheduler) stopLocked(id int64) {
	if r, ok := s.runs[id]; ok {
		r.cancel()
		delete(s.runs, id)
	}
}

func (s *Scheduler) Trigger(id int64) bool {
	s.mu.Lock()
	r, ok := s.runs[id]
	s.mu.Unlock()
	if !ok || r.monitor.Kind == monitor.KindPush {
		return false
	}
	select {
	case r.trigger <- struct{}{}:
	default:
	}
	return true
}

func (s *Scheduler) Push(id int64, res monitor.Result) bool {
	s.mu.Lock()
	r, ok := s.runs[id]
	s.mu.Unlock()
	if !ok || r.monitor.Kind != monitor.KindPush {
		return false
	}
	select {
	case r.push <- res:
		return true
	default:
		return false
	}
}

func (s *Scheduler) poll(ctx context.Context, r *run) {
	m := r.monitor
	t := newTracker(m)

	timer := time.NewTimer(jitter(m.Interval))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-r.trigger:
		}

		started := time.Now()
		res := s.checker.Check(ctx, m)
		if ctx.Err() != nil {
			return
		}
		s.emit(ctx, m, t, res)
		timer.Reset(time.Until(started.Add(m.Interval)))
	}
}

func (s *Scheduler) watch(ctx context.Context, r *run) {
	m := r.monitor
	t := newTracker(m)

	first := m.Deadline()
	if !m.State.CheckedAt.IsZero() {
		first = max(time.Until(m.State.CheckedAt.Add(m.Deadline())), time.Second)
	}
	deadline := time.NewTimer(first)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case res := <-r.push:
			s.emit(ctx, m, t, res)
			deadline.Reset(m.Deadline())
		case <-deadline.C:
			s.emit(ctx, m, t, monitor.Down(0, "no heartbeat received within %s", m.Deadline()))
			deadline.Reset(m.Interval)
		}
	}
}

func (s *Scheduler) emit(ctx context.Context, m *monitor.Monitor, t *tracker, res monitor.Result) {
	if res.At.IsZero() {
		res.At = time.Now()
	}
	previous := t.status
	res.Status = t.observe(res.Status)
	s.sink.Record(ctx, Beat{Monitor: m, Result: res, Previous: previous, Current: t.status})
}

func jitter(interval time.Duration) time.Duration {
	return rand.N(min(interval, 10*time.Second)) + 500*time.Millisecond
}

type tracker struct {
	retries      int
	recoverAfter int
	failures     int
	successes    int
	status       monitor.Status
}

func newTracker(m *monitor.Monitor) *tracker {
	status := m.State.Status
	if status == "" || status == monitor.StatusPaused {
		status = monitor.StatusPending
	}
	return &tracker{retries: m.Retries, recoverAfter: m.Settings.RecoverAfter, status: status}
}

func (t *tracker) observe(status monitor.Status) monitor.Status {
	if status != monitor.StatusDown {
		t.failures = 0
		if t.status == monitor.StatusDown && status.Healthy() {
			if t.successes++; t.successes < t.recoverAfter {
				return status
			}
		}
		t.successes = 0
		t.status = status
		return status
	}

	t.successes = 0
	t.failures++
	if t.status != monitor.StatusDown && t.failures <= t.retries {
		return monitor.StatusPending
	}
	t.status = monitor.StatusDown
	return status
}
