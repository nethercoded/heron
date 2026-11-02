package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/maintenance"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newMonitor(t *testing.T, s *Store, slug string, kind monitor.Kind) *monitor.Monitor {
	t.Helper()
	m := &monitor.Monitor{Name: slug, Slug: slug, Kind: kind, Target: "example.com:443", Public: true}
	m.Normalize()
	if err := s.CreateMonitor(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heron.db")
	for range 2 {
		s, err := Open(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		s.Close()
	}
}

func TestMonitorLifecycle(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	ch := &notify.Channel{Name: "ops", Kind: notify.KindWebhook, Enabled: true, Config: notify.Config{URL: "https://example.com/hook"}}
	if err := s.CreateChannel(ctx, ch); err != nil {
		t.Fatal(err)
	}

	m := newMonitor(t, s, "api", monitor.KindTCP)
	other := newMonitor(t, s, "web", monitor.KindTCP)
	if m.Position != 0 || other.Position != 1 {
		t.Errorf("positions = %d, %d", m.Position, other.Position)
	}

	if err := s.CreateMonitor(ctx, &monitor.Monitor{Name: "dup", Slug: "api", Kind: monitor.KindTCP, Interval: time.Minute, Timeout: time.Second}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate slug: %v", err)
	}

	m.Name = "Public API"
	m.Settings.DegradedAfterMS = 300
	m.ChannelIDs = []int64{ch.ID}
	if err := s.UpdateMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}

	got, err := s.MonitorBySlug(ctx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Public API" || got.Settings.DegradedAfterMS != 300 || len(got.ChannelIDs) != 1 {
		t.Errorf("update not persisted: %+v", got)
	}
	if got.State.Status != monitor.StatusPending {
		t.Errorf("status = %s", got.State.Status)
	}

	channels, err := s.ChannelsForMonitor(ctx, m.ID)
	if err != nil || len(channels) != 1 || channels[0].Config.URL != "https://example.com/hook" {
		t.Errorf("channels = %v, %v", channels, err)
	}

	paused, err := s.SetPaused(ctx, m.ID, true)
	if err != nil || !paused.Paused || paused.State.Status != monitor.StatusPaused {
		t.Errorf("pause: %+v, %v", paused, err)
	}

	if err := s.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Monitor(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted monitor still found: %v", err)
	}
}

func TestPushToken(t *testing.T) {
	s := open(t)
	m := newMonitor(t, s, "backup", monitor.KindPush)
	if m.PushToken == "" {
		t.Fatal("push token not generated")
	}
	got, err := s.MonitorByPushToken(t.Context(), m.PushToken)
	if err != nil || got.ID != m.ID {
		t.Fatalf("lookup by token: %v", err)
	}
	rotated, err := s.RotatePushToken(t.Context(), m.ID)
	if err != nil || rotated == m.PushToken {
		t.Fatalf("rotate: %q %v", rotated, err)
	}
}

func TestRecordAndRollups(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	m := newMonitor(t, s, "api", monitor.KindTCP)

	base := time.Now().Add(-3 * time.Hour).Truncate(time.Hour)
	beats := []monitor.Result{
		{Status: monitor.StatusUp, Latency: 100 * time.Millisecond, At: base.Add(1 * time.Minute)},
		{Status: monitor.StatusUp, Latency: 300 * time.Millisecond, At: base.Add(2 * time.Minute)},
		{Status: monitor.StatusDown, Latency: 0, At: base.Add(3 * time.Minute), Message: "refused"},
		{Status: monitor.StatusDegraded, Latency: 900 * time.Millisecond, At: base.Add(61 * time.Minute)},
	}
	for _, b := range beats {
		current := b.Status
		if err := s.Record(ctx, m.ID, b, current); err != nil {
			t.Fatal(err)
		}
	}

	uptimes, err := s.Uptimes(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	u := uptimes[m.ID]
	if u.Total != 4 || u.Down != 1 || u.Ratio() != 0.75 {
		t.Errorf("uptime = %+v (%.2f)", u, u.Ratio())
	}

	series, err := s.Series(ctx, m.ID, base, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 {
		t.Fatalf("series = %+v", series)
	}
	if series[0].Avg != 200*time.Millisecond || series[0].Max != 300*time.Millisecond || series[0].Down != 1 {
		t.Errorf("first hour = %+v", series[0])
	}

	days, err := s.Days(ctx, []int64{m.ID}, base.AddDate(0, 0, -1), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, d := range days[m.ID] {
		total += d.Uptime.Total
	}
	if total != 4 {
		t.Errorf("days = %+v", days)
	}

	got, err := s.Monitor(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Status != monitor.StatusDegraded || !got.State.Since.Equal(beats[3].At.Truncate(time.Second).UTC()) {
		t.Errorf("state = %+v", got.State)
	}

	recent, err := s.Beats(ctx, m.ID, 2)
	if err != nil || len(recent) != 2 || recent[0].Status != monitor.StatusDegraded {
		t.Errorf("beats = %+v, %v", recent, err)
	}

	removed, err := s.Prune(ctx, base.Add(30*time.Minute), base.Add(-time.Hour))
	if err != nil || removed != 3 {
		t.Errorf("prune removed %d: %v", removed, err)
	}
}

func TestIncidents(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	m := newMonitor(t, s, "api", monitor.KindTCP)

	inc := &Incident{Title: "API down", Impact: ImpactMajor, Status: IncidentInvestigating, Automatic: true, MonitorIDs: []int64{m.ID, m.ID}}
	if err := s.CreateIncident(ctx, inc, "looking into it"); err != nil {
		t.Fatal(err)
	}

	open, err := s.OpenAutomaticIncident(ctx, m.ID)
	if err != nil || open.ID != inc.ID || len(open.MonitorIDs) != 1 || len(open.Updates) != 1 {
		t.Fatalf("open incident = %+v, %v", open, err)
	}

	resolved, err := s.AddIncidentUpdate(ctx, inc.ID, IncidentResolved, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Open() || resolved.Status != IncidentResolved || len(resolved.Updates) != 2 || resolved.Updates[0].Body != "fixed" {
		t.Errorf("resolved = %+v", resolved)
	}

	if _, err := s.OpenAutomaticIncident(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolved incident still open: %v", err)
	}

	active, err := s.Incidents(ctx, IncidentFilter{OpenOnly: true})
	if err != nil || len(active) != 0 {
		t.Errorf("active = %v, %v", active, err)
	}
	all, err := s.Incidents(ctx, IncidentFilter{Limit: 10})
	if err != nil || len(all) != 1 {
		t.Errorf("all = %v, %v", all, err)
	}
}

func TestSessions(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	u, err := s.CreateFirstUser(ctx, "admin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFirstUser(ctx, "intruder", "hash"); !errors.Is(err, ErrConflict) {
		t.Errorf("second first user: %v", err)
	}

	token := []byte("token-hash")
	if err := s.CreateSession(ctx, token, u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.SessionUser(ctx, token)
	if err != nil || got.Username != "admin" {
		t.Fatalf("session user = %v, %v", got, err)
	}

	expired := []byte("expired")
	s.CreateSession(ctx, expired, u.ID, time.Now().Add(-time.Minute))
	if _, _, err := s.SessionUser(ctx, expired); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session accepted: %v", err)
	}

	if err := s.DeleteUserSessions(ctx, u.ID, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SessionUser(ctx, token); err != nil {
		t.Errorf("kept session was removed: %v", err)
	}
}

func TestMaintenanceWindows(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	db := newMonitor(t, s, "db", monitor.KindTCP)
	api := newMonitor(t, s, "api", monitor.KindTCP)

	w := &maintenance.Window{
		Title:      "Upgrade",
		MonitorIDs: []int64{db.ID, api.ID, db.ID},
		StartsAt:   time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC),
		Duration:   time.Hour,
		Repeat:     maintenance.RepeatWeekly,
		Enabled:    true,
	}
	if err := s.SaveMaintenanceWindow(ctx, w); err != nil {
		t.Fatal(err)
	}

	w.Title = "Weekly upgrade"
	w.MonitorIDs = []int64{api.ID}
	if err := s.SaveMaintenanceWindow(ctx, w); err != nil {
		t.Fatal(err)
	}

	got, err := s.MaintenanceWindow(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Weekly upgrade" || len(got.MonitorIDs) != 1 || got.Duration != time.Hour || got.Repeat != maintenance.RepeatWeekly {
		t.Errorf("window = %+v", got)
	}

	if err := s.DeleteMonitor(ctx, api.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MaintenanceWindow(ctx, w.ID); len(got.MonitorIDs) != 0 {
		t.Errorf("deleted monitor still linked: %v", got.MonitorIDs)
	}

	if err := s.DeleteMaintenanceWindow(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MaintenanceWindow(ctx, w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted window found: %v", err)
	}
}

func TestPageDefaults(t *testing.T) {
	s := open(t)
	page, err := s.Page(t.Context())
	if err != nil || page.Title != "Status" || page.HistoryDays != 90 {
		t.Fatalf("defaults = %+v, %v", page, err)
	}
	page.Title = "Heron"
	if err := s.SavePage(t.Context(), page); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Page(t.Context()); got.Title != "Heron" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestBlocks(t *testing.T) {
	s := open(t)
	page, _ := s.Page(t.Context())
	if len(page.Blocks) != len(DefaultBlocks()) {
		t.Fatalf("blocks = %+v", page.Blocks)
	}
	page.Blocks = NormalizeBlocks([]Block{
		{ID: "top", Type: BlockNotice, Title: "Moving", Body: "We move on Friday", Tone: "info", Group: "dropped"},
		{ID: "api", Type: BlockMonitors, Group: "API", Style: "list", Days: 7, Width: 6},
		{ID: "hero", Type: BlockHero, Width: 6},
	})
	if page.Blocks[2].Width != 0 || page.Blocks[1].Width != 6 {
		t.Errorf("widths = %+v", page.Blocks)
	}
	if page.Blocks[0].Group != "" || page.Blocks[1].Days != 0 {
		t.Errorf("normalize kept foreign settings: %+v", page.Blocks)
	}
	if fields := ValidateBlocks(page.Blocks); len(fields) > 0 {
		t.Fatalf("fields = %v", fields)
	}
	if err := s.SavePage(t.Context(), page); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Page(t.Context())
	if len(got.Blocks) != 3 || got.Blocks[1].Group != "API" {
		t.Errorf("blocks = %+v", got.Blocks)
	}

	bad := []Block{{ID: "a", Type: BlockHero}, {ID: "a", Type: BlockHero}, {ID: "Bad id", Type: "chart"}, {ID: "n", Type: BlockNotice}, {ID: "w", Type: BlockHistory, Width: 5}}
	fields := ValidateBlocks(bad)
	for _, key := range []string{"blocks.1.id", "blocks.1.type", "blocks.2.id", "blocks.2.type", "blocks.3.body", "blocks.4.width"} {
		if fields[key] == "" {
			t.Errorf("no error for %s in %v", key, fields)
		}
	}
}
