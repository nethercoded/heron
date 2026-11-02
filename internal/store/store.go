package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, file string) (*Store, error) {
	q := url.Values{}
	for _, p := range []string{"foreign_keys(1)", "journal_mode(WAL)", "synchronous(NORMAL)", "busy_timeout(10000)", "temp_store(MEMORY)"} {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")

	db, err := sql.Open("sqlite", "file:"+file+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("migrate: %w", err), db.Close())
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Chmod(file+suffix, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(fmt.Errorf("restrict database permissions: %w", err), db.Close())
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	if _, err := s.db.ExecContext(context.Background(), "pragma optimize"); err != nil {
		return errors.Join(err, s.db.Close())
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `create table if not exists schema_migrations (version integer primary key, applied_at integer not null)`); err != nil {
		return err
	}

	var current int
	if err := s.db.QueryRowContext(ctx, `select coalesce(max(version), 0) from schema_migrations`).Scan(&current); err != nil {
		return err
	}

	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(files)

	for _, file := range files {
		version, err := strconv.Atoi(strings.SplitN(path.Base(file), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: bad version", file)
		}
		if version <= current {
			continue
		}
		body, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}
		err = s.tx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
			_, err := tx.ExecContext(ctx, `insert into schema_migrations (version, applied_at) values (?, ?)`, version, time.Now().Unix())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

func (s *Store) Prune(ctx context.Context, resultsBefore, rollupsBefore time.Time) (int64, error) {
	var removed int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `delete from results where at < ?`, resultsBefore.Unix())
		if err != nil {
			return err
		}
		removed, _ = res.RowsAffected()
		if _, err := tx.ExecContext(ctx, `delete from rollups where bucket < ?`, rollupsBefore.Unix()); err != nil {
			return err
		}
		now := time.Now()
		if _, err := tx.ExecContext(ctx, `delete from invites where expires_at < ?`, now.Unix()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `delete from audit_log where at < ?`, now.AddDate(-1, 0, 0).Unix()); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `delete from sessions where expires_at < ?`, now.Unix())
		return err
	})
	return removed, err
}

func translate(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var se *sqlite.Error
	if errors.As(err, &se) && (se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
		return ErrConflict
	}
	return err
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0).UTC()
}

func nullUnix(t time.Time) sql.NullInt64 {
	return sql.NullInt64{Int64: t.Unix(), Valid: !t.IsZero()}
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func args[T any](values []T) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

type scanner interface {
	Scan(dest ...any) error
}
