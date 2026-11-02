package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nethercoded/heron/internal/monitor"
)

func TestBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "heron.db")
	s, err := Open(t.Context(), file)
	if err != nil {
		t.Fatal(err)
	}
	newMonitor(t, s, "api", monitor.KindHTTP)

	backup := filepath.Join(dir, "backups", "one.db")
	if err := s.Backup(t.Context(), backup); err != nil {
		t.Fatal(err)
	}
	if v, err := Inspect(t.Context(), backup); err != nil || v != LatestSchema() {
		t.Fatalf("inspect = %d, %v", v, err)
	}
	newMonitor(t, s, "web", monitor.KindHTTP)
	s.Close()

	saved, err := Restore(t.Context(), file, backup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("the database before the restore was not kept: %v", err)
	}

	s, err = Open(t.Context(), file)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	list, err := s.Monitors(t.Context())
	if err != nil || len(list) != 1 || list[0].Slug != "api" {
		t.Fatalf("monitors after restore = %d, %v", len(list), err)
	}
}

func TestRestoreRejectsStrangers(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.db")
	os.WriteFile(junk, []byte("definitely not sqlite, just some text that is long enough"), 0o600)
	if _, err := Restore(t.Context(), filepath.Join(dir, "heron.db"), junk); err == nil {
		t.Fatal("restored a file that is not a database")
	}
	if _, err := Restore(t.Context(), filepath.Join(dir, "heron.db"), filepath.Join(dir, "missing.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestRestoreIfNewer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "heron.db")
	s, err := Open(t.Context(), file)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "before.db")
	s.Backup(t.Context(), backup)
	newMonitor(t, s, "api", monitor.KindHTTP)

	if restored, err := RestoreIfNewer(t.Context(), file, backup); err != nil || restored {
		t.Fatalf("restored = %v, %v with the same schema", restored, err)
	}
	if _, err := s.db.ExecContext(t.Context(), `insert into schema_migrations (version, applied_at) values (999, 0)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if restored, err := RestoreIfNewer(t.Context(), file, backup); err != nil || !restored {
		t.Fatalf("restored = %v, %v after a schema change", restored, err)
	}
}

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"before-update-1.db", "before-update-2.db", "before-update-3.db", "mine.db"} {
		os.WriteFile(filepath.Join(dir, name), nil, 0o600)
	}
	if err := Prune(dir, "before-update-", 2); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 || entries[0].Name() != "before-update-2.db" {
		t.Fatalf("left %v", entries)
	}
}
