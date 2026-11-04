package store

import (
	"context"
	"time"
)

type Expiry struct {
	Host      string
	Kind      string
	ExpiresAt time.Time
	CheckedAt time.Time
	Issuer    string
	Error     string
	Warned    int
}

func (s *Store) Expiries(ctx context.Context) ([]Expiry, error) {
	rows, err := s.db.QueryContext(ctx, `select host, kind, expires_at, checked_at, issuer, error, warned from expiry order by host, kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Expiry
	for rows.Next() {
		var e Expiry
		var expires, checked int64
		if err := rows.Scan(&e.Host, &e.Kind, &expires, &checked, &e.Issuer, &e.Error, &e.Warned); err != nil {
			return nil, err
		}
		e.ExpiresAt = fromUnix(expires)
		e.CheckedAt = fromUnix(checked)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) SaveExpiry(ctx context.Context, e Expiry) error {
	_, err := s.db.ExecContext(ctx, `
		insert into expiry (host, kind, expires_at, checked_at, issuer, error, warned) values (?, ?, ?, ?, ?, ?, ?)
		on conflict (host, kind) do update set expires_at = excluded.expires_at, checked_at = excluded.checked_at,
			issuer = excluded.issuer, error = excluded.error, warned = excluded.warned`,
		e.Host, e.Kind, unix(e.ExpiresAt), unix(e.CheckedAt), e.Issuer, e.Error, e.Warned)
	return err
}

func (s *Store) DeleteExpiry(ctx context.Context, host, kind string) error {
	_, err := s.db.ExecContext(ctx, `delete from expiry where host = ? and kind = ?`, host, kind)
	return err
}
