package forecast

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/pelican"
	"github.com/nethercoded/heron/internal/store"
)

type fakePanel struct {
	used, limit int64
	disk        pelican.Utilization
}

func (f *fakePanel) NodeUtilization(context.Context, string) (pelican.Utilization, error) {
	return f.disk, nil
}

func (f *fakePanel) ServerMemory(string) (int64, int64, bool) {
	return f.used, f.limit, f.limit > 0
}

type outbox struct{ sent []notify.Message }

func (o *outbox) Dispatch(_ []notify.Channel, msg notify.Message) { o.sent = append(o.sent, msg) }

func (o *outbox) titles() []string {
	var out []string
	for _, m := range o.sent {
		out = append(out, m.Title)
	}
	o.sent = nil
	slices.Sort(out)
	return out
}

func setup(t *testing.T) (*store.Store, *notify.Channel) {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ch := &notify.Channel{Name: "ops", Kind: notify.KindWebhook, Enabled: true, Config: notify.Config{URL: "https://example.com"}}
	if err := db.CreateChannel(t.Context(), ch); err != nil {
		t.Fatal(err)
	}
	return db, ch
}

func add(t *testing.T, db *store.Store, m *monitor.Monitor, ch *notify.Channel) *monitor.Monitor {
	t.Helper()
	m.ChannelIDs = []int64{ch.ID}
	m.Normalize()
	if err := db.CreateMonitor(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWarnings(t *testing.T) {
	db, ch := setup(t)
	now := time.Now()
	api := add(t, db, &monitor.Monitor{Name: "API", Slug: "api", Kind: monitor.KindTCP, Target: "api:443"}, ch)
	game := add(t, db, &monitor.Monitor{Name: "Survival", Slug: "survival", Kind: monitor.KindPelicanServer, Target: "aaa"}, ch)
	node := add(t, db, &monitor.Monitor{Name: "DE-1", Slug: "de-1", Kind: monitor.KindPelicanNode, Target: "1"}, ch)

	record := func(at time.Time, status monitor.Status, latency time.Duration) {
		t.Helper()
		if err := db.Record(t.Context(), api.ID, monitor.Result{Status: status, Latency: latency, At: at}, monitor.StatusUp); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 150 {
		record(now.Add(-48*time.Hour-time.Duration(i)*time.Hour), monitor.StatusUp, 200*time.Millisecond)
	}
	for i := range 30 {
		record(now.Add(-time.Duration(i)*5*time.Minute), monitor.StatusUp, 900*time.Millisecond)
	}
	for i := range 8 {
		record(now.Add(-time.Duration(i)*7*time.Minute-time.Minute), monitor.StatusPending, 0)
	}

	for h := range 48 {
		at := now.Add(-time.Duration(47-h) * time.Hour)
		if err := db.AddSample(t.Context(), node.ID, KindDisk, store.Sample{At: at.Truncate(time.Hour), Value: float64(400<<30 + h*(2<<30)), Total: 500 << 30}); err != nil {
			t.Fatal(err)
		}
	}

	panel := &fakePanel{used: 3900 << 20, limit: 4096 << 20}
	box := &outbox{}
	s := &Service{Store: db, Panel: panel, Sender: box, Every: 15 * time.Minute, Log: slog.New(slog.DiscardHandler), sampled: now}

	if err := s.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := box.titles()
	want := []string{"API answers 4.5x slower than usual", "API keeps failing checks without going down", "Survival is close to its memory limit", "The disk on DE-1 fills up in about 3 hours"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("alerts = %q", got)
	}

	if err := s.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(box.sent) != 0 {
		t.Fatal("the same warnings went out twice")
	}

	panel.used = 1 << 30
	if err := s.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	list, _ := db.Warnings(t.Context())
	for _, w := range list {
		if w.MonitorID == game.ID {
			t.Fatal("the memory warning stayed after memory went down")
		}
	}
	if len(list) != 3 {
		t.Fatalf("warnings left = %d", len(list))
	}
}

func TestUntil(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var flat, growing []Sample
	for h := range 30 {
		at := start.Add(time.Duration(h) * time.Hour)
		flat = append(flat, Sample{At: at, Value: 100, Total: 200})
		growing = append(growing, Sample{At: at, Value: 100 + float64(h), Total: 200})
	}
	if _, ok := Until(flat); ok {
		t.Error("a flat disk was predicted to fill up")
	}
	days, ok := Until(growing)
	if !ok || days < 2.9 || days > 3.0 {
		t.Errorf("days = %.2f, %v", days, ok)
	}
	if _, ok := Until(growing[:5]); ok {
		t.Error("five samples were enough for a forecast")
	}
}
