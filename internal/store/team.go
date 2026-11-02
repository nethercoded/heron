package store

import (
	"context"
	"time"
)

type Passkey struct {
	ID         []byte
	UserID     int64
	Name       string
	Credential []byte
	CreatedAt  time.Time
	LastUsedAt time.Time
}

func (s *Store) Passkeys(ctx context.Context, userID int64) ([]Passkey, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, user_id, name, credential, created_at, last_used_at
		from passkeys where user_id = ? order by created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Passkey
	for rows.Next() {
		var (
			p             Passkey
			created, used int64
		)
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Credential, &created, &used); err != nil {
			return nil, err
		}
		p.CreatedAt = fromUnix(created)
		p.LastUsedAt = fromUnix(used)
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) AddPasskey(ctx context.Context, p *Passkey) error {
	p.CreatedAt = time.Now().UTC().Truncate(time.Second)
	_, err := s.db.ExecContext(ctx, `insert into passkeys (id, user_id, name, credential, created_at) values (?, ?, ?, ?, ?)`,
		p.ID, p.UserID, p.Name, p.Credential, p.CreatedAt.Unix())
	return translate(err)
}

func (s *Store) UsePasskey(ctx context.Context, id, credential []byte) error {
	_, err := s.db.ExecContext(ctx, `update passkeys set credential = ?, last_used_at = ? where id = ?`, credential, time.Now().Unix(), id)
	return err
}

func (s *Store) DeletePasskey(ctx context.Context, userID int64, id []byte) error {
	return s.updateUser(ctx, `delete from passkeys where user_id = ? and id = ?`, userID, id)
}

type AuditEntry struct {
	ID       int64
	At       time.Time
	UserID   int64
	Username string
	Action   string
	Target   string
	IP       string
}

func (s *Store) Audit(ctx context.Context, e AuditEntry) error {
	_, err := s.db.ExecContext(ctx, `insert into audit_log (at, user_id, username, action, target, ip) values (?, nullif(?, 0), ?, ?, ?, ?)`,
		time.Now().Unix(), e.UserID, e.Username, e.Action, e.Target, e.IP)
	return err
}

type AuditFilter struct {
	Before int64
	UserID int64
	Action string
	Limit  int
}

func (s *Store) AuditLog(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	query := `select id, at, coalesce(user_id, 0), username, action, target, ip from audit_log where 1 = 1`
	var params []any
	if f.Before > 0 {
		query += ` and id < ?`
		params = append(params, f.Before)
	}
	if f.UserID > 0 {
		query += ` and user_id = ?`
		params = append(params, f.UserID)
	}
	if f.Action != "" {
		query += ` and (action = ? or action like ? || '.%')`
		params = append(params, f.Action, f.Action)
	}
	query += ` order by id desc limit ?`
	params = append(params, f.Limit)

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AuditEntry
	for rows.Next() {
		var (
			e  AuditEntry
			at int64
		)
		if err := rows.Scan(&e.ID, &at, &e.UserID, &e.Username, &e.Action, &e.Target, &e.IP); err != nil {
			return nil, err
		}
		e.At = fromUnix(at)
		list = append(list, e)
	}
	return list, rows.Err()
}
