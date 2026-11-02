package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var ErrNotHeron = errors.New("the file is not a heron database")

func (s *Store) Backup(ctx context.Context, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	_ = os.Remove(tmp)
	if _, err := s.db.ExecContext(ctx, `vacuum into ?`, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("backup: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `select coalesce(max(version), 0) from schema_migrations`).Scan(&v)
	return v, err
}

func LatestSchema() int {
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	latest := 0
	for _, f := range files {
		if v, err := strconv.Atoi(strings.SplitN(path.Base(f), "_", 2)[0]); err == nil {
			latest = max(latest, v)
		}
	}
	return latest
}

func Inspect(ctx context.Context, file string) (int, error) {
	if _, err := os.Stat(file); err != nil {
		return 0, err
	}
	db, err := sql.Open("sqlite", "file:"+file+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var check string
	if err := db.QueryRowContext(ctx, `pragma quick_check`).Scan(&check); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrNotHeron, err)
	}
	if check != "ok" {
		return 0, fmt.Errorf("the database is damaged: %s", check)
	}
	var version int
	if err := db.QueryRowContext(ctx, `select coalesce(max(version), 0) from schema_migrations`).Scan(&version); err != nil {
		return 0, ErrNotHeron
	}
	var tables int
	if err := db.QueryRowContext(ctx, `select count(*) from sqlite_master where type = 'table' and name in ('monitors', 'settings')`).Scan(&tables); err != nil || tables != 2 {
		return 0, ErrNotHeron
	}
	return version, nil
}

func Restore(ctx context.Context, file, src string) (string, error) {
	version, err := Inspect(ctx, src)
	if err != nil {
		return "", err
	}
	if version > LatestSchema() {
		return "", fmt.Errorf("the backup comes from a newer heron (schema %d, this build knows %d), update heron first", version, LatestSchema())
	}

	var saved string
	if _, err := os.Stat(file); err == nil {
		current, err := Open(ctx, file)
		if err != nil {
			return "", fmt.Errorf("open the current database: %w", err)
		}
		saved = file + ".before-restore"
		_ = os.Remove(saved)
		err = current.Backup(ctx, saved)
		if err := errors.Join(err, current.db.Close()); err != nil {
			return "", fmt.Errorf("save the current database: %w", err)
		}
	}

	for _, suffix := range []string{"-wal", "-shm", ""} {
		if err := os.Remove(file + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return saved, err
		}
	}
	if err := copyFile(src, file); err != nil {
		return saved, err
	}
	return saved, nil
}

func RestoreIfNewer(ctx context.Context, file, backup string) (bool, error) {
	want, err := Inspect(ctx, backup)
	if err != nil {
		return false, err
	}
	have, err := Inspect(ctx, file)
	if err != nil || have <= want {
		return false, err
	}
	_, err = Restore(ctx, file, backup)
	return err == nil, err
}

func Prune(dir, prefix string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	var errs []error
	for _, name := range names[:max(0, len(names)-keep)] {
		errs = append(errs, os.Remove(filepath.Join(dir, name)))
	}
	return errors.Join(errs...)
}

func BackupName(prefix string, at time.Time, note string) string {
	name := prefix + at.UTC().Format("20060102-150405")
	if note != "" {
		name += "-" + note
	}
	return name + ".db"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
