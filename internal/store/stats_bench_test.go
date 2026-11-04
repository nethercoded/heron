package store

import (
	"os"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
)

func seedYear(b testing.TB, monitors int) (*Store, []int64) {
	b.Helper()
	path := os.Getenv("HERON_BIG_DB")
	if path == "" {
		path = b.TempDir() + "/heron.db"
	}
	s, err := Open(b.Context(), path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })

	ids := make([]int64, monitors)
	for i := range ids {
		m := &monitor.Monitor{Name: "m", Slug: "m-" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Kind: monitor.KindTCP, Target: "example.com:443", Public: true}
		m.Normalize()
		if err := s.CreateMonitor(b.Context(), m); err != nil {
			b.Fatal(err)
		}
		ids[i] = m.ID
	}

	tx, err := s.db.BeginTx(b.Context(), nil)
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.PrepareContext(b.Context(), `insert into rollups (monitor_id, bucket, total, down, degraded, latency_sum, latency_count, latency_max) values (?, ?, 60, ?, 0, 6000, 60, 300)`)
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()
	now := time.Now().Truncate(time.Hour)
	for _, id := range ids {
		for h := range 365 * 24 {
			if _, err := stmt.ExecContext(b.Context(), id, now.Add(-time.Duration(h)*time.Hour).Unix(), h%97/96); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return s, ids
}

func BenchmarkStatusQueries(b *testing.B) {
	s, ids := seedYear(b, 50)
	now := time.Now()
	from := now.AddDate(0, 0, -89)
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.UptimeWindows(b.Context(), now, 24*time.Hour, 7*24*time.Hour, 30*24*time.Hour, 90*24*time.Hour); err != nil {
			b.Fatal(err)
		}
		if _, err := s.Days(b.Context(), ids, from, time.UTC); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUptimeWindows(b *testing.B) {
	s, _ := seedYear(b, 50)
	now := time.Now()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.UptimeWindows(b.Context(), now, 24*time.Hour, 7*24*time.Hour, 30*24*time.Hour, 90*24*time.Hour); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDays(b *testing.B) {
	s, ids := seedYear(b, 50)
	from := time.Now().AddDate(0, 0, -89)
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.Days(b.Context(), ids, from, time.UTC); err != nil {
			b.Fatal(err)
		}
	}
}

func TestBigDatabase(t *testing.T) {
	if os.Getenv("HERON_BIG_DB") == "" {
		t.Skip("set HERON_BIG_DB to write a year of data for load tests")
	}
	seedYear(t, 50)
}
