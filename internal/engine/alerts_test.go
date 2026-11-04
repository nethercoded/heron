package engine

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/maintenance"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/scheduler"
	"github.com/nethercoded/heron/internal/store"
)

type inbox struct {
	mu     sync.Mutex
	sent   []notify.Message
	reason string
}

func (b *inbox) Dispatch(_ []notify.Channel, msg notify.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent = append(b.sent, msg)
	b.reason = msg.Reason
}

func (b *inbox) take() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, m := range b.sent {
		out = append(out, m.Monitor.Slug+" "+string(m.Event))
	}
	b.sent = nil
	return out
}

func setup(t *testing.T) (*Engine, *inbox, *store.Store) {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	box := &inbox{}
	e := New(db, box, maintenance.NewCalendar(time.UTC), slog.New(slog.DiscardHandler), "")
	e.policy = Policy{FlapCount: 4, FlapWindow: time.Hour, StableAfter: time.Minute, MaxHold: 60 * time.Millisecond, Recheck: 60 * time.Millisecond}
	return e, box, db
}

func add(t *testing.T, db *store.Store, slug string, parent int64) *monitor.Monitor {
	t.Helper()
	ch := &notify.Channel{Name: slug, Kind: notify.KindWebhook, Enabled: true, Config: notify.Config{URL: "https://example.com"}}
	if err := db.CreateChannel(t.Context(), ch); err != nil {
		t.Fatal(err)
	}
	m := &monitor.Monitor{Name: slug, Slug: slug, Kind: monitor.KindTCP, Target: "db:5432", ChannelIDs: []int64{ch.ID}, Settings: monitor.Settings{DependsOn: parent}}
	m.Normalize()
	if err := db.CreateMonitor(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	return m
}

func beat(e *Engine, m *monitor.Monitor, from, to monitor.Status, at time.Time) {
	e.Record(context.Background(), scheduler.Beat{Monitor: m, Previous: from, Current: to, Result: monitor.Result{Status: to, At: at, Message: "checked"}})
}

func expect(t *testing.T, box *inbox, want ...string) {
	t.Helper()
	got := box.take()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("alerts = %v, want %v", got, want)
	}
}

func TestChildAlertsWaitForTheParent(t *testing.T) {
	e, box, db := setup(t)
	node := add(t, db, "node", 0)
	game := add(t, db, "game", node.ID)
	now := time.Now()

	beat(e, node, monitor.StatusUp, monitor.StatusDown, now)
	expect(t, box, "node monitor.down")
	if !strings.Contains(box.reason, "One monitor depends on it") {
		t.Fatal("the parent alert does not mention its dependents")
	}

	beat(e, game, monitor.StatusUp, monitor.StatusDown, now)
	beat(e, game, monitor.StatusDown, monitor.StatusUp, now)
	beat(e, game, monitor.StatusUp, monitor.StatusDown, now)
	expect(t, box)

	beat(e, node, monitor.StatusDown, monitor.StatusUp, now)
	expect(t, box, "node monitor.recovered")
	time.Sleep(150 * time.Millisecond)
	expect(t, box, "game monitor.down")
}

func TestHeldAlertGoesOutWhenTheParentStaysUp(t *testing.T) {
	e, box, db := setup(t)
	node := add(t, db, "node", 0)
	game := add(t, db, "game", node.ID)
	beat(e, node, monitor.StatusPending, monitor.StatusUp, time.Now())

	beat(e, game, monitor.StatusUp, monitor.StatusDown, time.Now())
	expect(t, box)
	time.Sleep(150 * time.Millisecond)
	expect(t, box, "game monitor.down")

	other := add(t, db, "other", node.ID)
	beat(e, other, monitor.StatusUp, monitor.StatusDown, time.Now())
	beat(e, node, monitor.StatusUp, monitor.StatusDown, time.Now())
	time.Sleep(150 * time.Millisecond)
	expect(t, box, "node monitor.down")
}

func TestFlappingSendsOneAlert(t *testing.T) {
	e, box, db := setup(t)
	m := add(t, db, "api", 0)
	at := time.Now()
	states := []monitor.Status{monitor.StatusUp, monitor.StatusDown, monitor.StatusUp, monitor.StatusDown, monitor.StatusUp, monitor.StatusDown, monitor.StatusUp}
	for i := 1; i < len(states); i++ {
		beat(e, m, states[i-1], states[i], at.Add(time.Duration(i)*time.Second))
	}
	expect(t, box, "api monitor.down", "api monitor.recovered", "api monitor.down", "api monitor.flapping")

	beat(e, m, monitor.StatusUp, monitor.StatusUp, at.Add(2*time.Minute))
	expect(t, box, "api monitor.stable")
}
