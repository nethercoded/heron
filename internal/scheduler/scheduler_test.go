package scheduler

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nethercode/heron/internal/monitor"
)

type recorder struct {
	mu    sync.Mutex
	beats []Beat
}

func (r *recorder) Record(_ context.Context, b Beat) {
	r.mu.Lock()
	r.beats = append(r.beats, b)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []Beat {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.beats)
}

func scripted(statuses ...monitor.Status) monitor.Checker {
	var mu sync.Mutex
	i := 0
	return monitor.CheckerFunc(func(context.Context, *monitor.Monitor) monitor.Result {
		mu.Lock()
		defer mu.Unlock()
		s := statuses[min(i, len(statuses)-1)]
		i++
		return monitor.Result{Status: s}
	})
}

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPollingWithRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			up   = monitor.StatusUp
			down = monitor.StatusDown
		)
		rec := &recorder{}
		s := New(scripted(up, down, down, up, down, down, down), rec, discard())

		ctx, cancel := context.WithCancel(t.Context())
		m := &monitor.Monitor{ID: 1, Slug: "api", Kind: monitor.KindHTTP, Interval: time.Minute, Retries: 1}
		s.Start(ctx, []*monitor.Monitor{m})

		time.Sleep(6*time.Minute + 30*time.Second)
		synctest.Wait()
		cancel()
		s.Wait()

		beats := rec.snapshot()
		if len(beats) != 7 {
			t.Fatalf("got %d beats, want 7", len(beats))
		}

		var got []monitor.Status
		var events []string
		for _, b := range beats {
			got = append(got, b.Result.Status)
			switch {
			case b.WentDown():
				events = append(events, "down")
			case b.Recovered():
				events = append(events, "up")
			}
		}

		want := []monitor.Status{up, monitor.StatusPending, down, up, monitor.StatusPending, down, down}
		if !slices.Equal(got, want) {
			t.Errorf("beats = %v, want %v", got, want)
		}
		if !slices.Equal(events, []string{"down", "up", "down"}) {
			t.Errorf("transitions = %v", events)
		}
	})
}

func TestPushHeartbeats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rec := &recorder{}
		s := New(nil, rec, discard())

		ctx, cancel := context.WithCancel(t.Context())
		m := &monitor.Monitor{ID: 7, Slug: "backup", Kind: monitor.KindPush, Interval: time.Hour, Settings: monitor.Settings{GraceSeconds: 600}}
		s.Start(ctx, []*monitor.Monitor{m})

		time.Sleep(30 * time.Minute)
		if !s.Push(7, monitor.Up(0, "ok")) {
			t.Fatal("push rejected")
		}
		synctest.Wait()

		time.Sleep(60 * time.Minute)
		synctest.Wait()
		if n := len(rec.snapshot()); n != 1 {
			t.Fatalf("expected only the pushed beat before the deadline, got %d", n)
		}

		time.Sleep(15 * time.Minute)
		synctest.Wait()
		cancel()
		s.Wait()

		beats := rec.snapshot()
		if len(beats) != 2 {
			t.Fatalf("got %d beats, want 2", len(beats))
		}
		if !beats[1].WentDown() {
			t.Errorf("missing heartbeat should take the monitor down: %+v", beats[1])
		}
		if s.Trigger(7) {
			t.Error("push monitors cannot be triggered")
		}
	})
}

func TestSyncReplacesAndPauses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rec := &recorder{}
		s := New(scripted(monitor.StatusUp), rec, discard())

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		m := &monitor.Monitor{ID: 3, Slug: "db", Kind: monitor.KindTCP, Interval: time.Minute}
		s.Start(ctx, []*monitor.Monitor{m})

		time.Sleep(90 * time.Second)
		synctest.Wait()

		s.Sync(&monitor.Monitor{ID: 3, Slug: "db", Kind: monitor.KindTCP, Interval: time.Minute, Paused: true})
		before := len(rec.snapshot())

		time.Sleep(10 * time.Minute)
		synctest.Wait()

		if after := len(rec.snapshot()); after != before {
			t.Errorf("paused monitor kept running: %d -> %d", before, after)
		}
		if s.Trigger(3) {
			t.Error("paused monitor accepted a trigger")
		}

		cancel()
		s.Wait()
	})
}

func TestTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rec := &recorder{}
		s := New(scripted(monitor.StatusUp), rec, discard())
		ctx, cancel := context.WithCancel(t.Context())

		s.Start(ctx, []*monitor.Monitor{{ID: 1, Slug: "a", Kind: monitor.KindTCP, Interval: time.Hour}})
		time.Sleep(20 * time.Second)
		synctest.Wait()

		s.Trigger(1)
		synctest.Wait()
		cancel()
		s.Wait()

		if n := len(rec.snapshot()); n != 2 {
			t.Fatalf("got %d beats, want 2", n)
		}
	})
}
