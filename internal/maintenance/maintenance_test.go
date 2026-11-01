package maintenance

import (
	"testing"
	"time"
)

func TestNextOneOff(t *testing.T) {
	start := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	w := Window{StartsAt: start, Duration: time.Hour, Repeat: RepeatNone, Enabled: true}

	if occ, ok := w.Next(start.Add(-time.Hour), time.UTC); !ok || !occ.Start.Equal(start) {
		t.Fatalf("before: %v %v", occ, ok)
	}
	if !w.ActiveAt(start.Add(30*time.Minute), time.UTC) {
		t.Error("should be active inside the window")
	}
	if w.ActiveAt(start.Add(time.Hour), time.UTC) {
		t.Error("end is exclusive")
	}
	if _, ok := w.Next(start.Add(2*time.Hour), time.UTC); ok {
		t.Error("finished one-off window has no next occurrence")
	}
}

func TestNextRepeating(t *testing.T) {
	start := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)

	daily := Window{StartsAt: start, Duration: 30 * time.Minute, Repeat: RepeatDaily, Enabled: true}
	at := time.Date(2026, 11, 7, 3, 10, 0, 0, time.UTC)
	occ, ok := daily.Next(at, time.UTC)
	if !ok || !occ.Start.Equal(time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)) || !occ.Contains(at) {
		t.Fatalf("daily current = %v", occ)
	}
	occ, _ = daily.Next(at.Add(time.Hour), time.UTC)
	if !occ.Start.Equal(time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("daily next = %v", occ.Start)
	}

	weekly := Window{StartsAt: start, Duration: 2 * time.Hour, Repeat: RepeatWeekly, Enabled: true}
	occ, _ = weekly.Next(time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.UTC)
	if !occ.Start.Equal(time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("weekly next = %v", occ.Start)
	}
	if weekly.ActiveAt(time.Date(2026, 11, 9, 3, 30, 0, 0, time.UTC), time.UTC) {
		t.Error("weekly window must not fire on other weekdays")
	}
}

func TestNextKeepsLocalTimeAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip(err)
	}
	start := time.Date(2026, 10, 20, 2, 30, 0, 0, berlin)
	w := Window{StartsAt: start, Duration: time.Hour, Repeat: RepeatDaily, Enabled: true}

	occ, _ := w.Next(time.Date(2026, 11, 2, 0, 0, 0, 0, berlin), berlin)
	if h, m, _ := occ.Start.In(berlin).Clock(); h != 2 || m != 30 {
		t.Errorf("occurrence drifted to %s", occ.Start.In(berlin))
	}
}

func TestCalendar(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	c := NewCalendar(time.UTC)
	c.Set([]Window{
		{ID: 1, Title: "db", MonitorIDs: []int64{7}, StartsAt: now.Add(-time.Minute), Duration: time.Hour, Repeat: RepeatNone, Enabled: true},
		{ID: 2, Title: "later", MonitorIDs: []int64{8}, StartsAt: now.Add(48 * time.Hour), Duration: time.Hour, Repeat: RepeatNone, Enabled: true},
		{ID: 3, Title: "off", MonitorIDs: []int64{9}, StartsAt: now.Add(-time.Minute), Duration: time.Hour, Repeat: RepeatNone},
	})

	if w, ok := c.Active(7, now); !ok || w.ID != 1 {
		t.Errorf("monitor 7 should be in maintenance")
	}
	if _, ok := c.Active(8, now); ok {
		t.Error("monitor 8 window has not started")
	}
	if _, ok := c.Active(9, now); ok {
		t.Error("disabled windows never apply")
	}

	upcoming := c.Upcoming(now, 7*24*time.Hour)
	if len(upcoming) != 2 || !upcoming[0].Active || upcoming[1].ID != 2 {
		t.Errorf("upcoming = %+v", upcoming)
	}
}

func TestValidate(t *testing.T) {
	ok := Window{Title: "x", StartsAt: time.Now(), Duration: time.Hour, Repeat: RepeatDaily, MonitorIDs: []int64{1}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Window{Duration: 25 * time.Hour, Repeat: RepeatDaily}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected errors")
	}
}
