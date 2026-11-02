package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

func (r Role) rank() int {
	switch r {
	case RoleOwner:
		return 3
	case RoleAdmin:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

func (r Role) Allows(min Role) bool {
	return r.rank() >= min.rank()
}

func (r Role) Valid() bool {
	return r.rank() > 0
}

type User struct {
	ID           int64
	Username     string
	Role         Role
	PasswordHash string
	TOTPSecret   string
	TOTPStep     int64
	Handle       []byte
	CreatedAt    time.Time
	LastSeenAt   time.Time
}

func (u *User) TwoFactor() bool {
	return u.TOTPSecret != ""
}

const userColumns = `u.id, u.username, u.role, u.password_hash, u.totp_secret, u.totp_step, u.handle, u.created_at, u.last_seen_at`

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, hash string, role Role) (*User, error) {
	var u *User
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var err error
		u, err = insertUser(ctx, tx, username, hash, role)
		return err
	})
	return u, err
}

func (s *Store) CreateFirstUser(ctx context.Context, username, hash string) (*User, error) {
	var u *User
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `select count(*) from users`).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
		var err error
		u, err = insertUser(ctx, tx, username, hash, RoleOwner)
		return err
	})
	return u, err
}

func insertUser(ctx context.Context, tx *sql.Tx, username, hash string, role Role) (*User, error) {
	u := &User{Username: username, Role: role, PasswordHash: hash, Handle: make([]byte, 32), CreatedAt: time.Now().UTC().Truncate(time.Second)}
	_, _ = rand.Read(u.Handle)
	err := tx.QueryRowContext(ctx, `insert into users (username, role, password_hash, handle, created_at) values (?, ?, ?, ?, ?) returning id`,
		username, role, hash, u.Handle, u.CreatedAt.Unix()).Scan(&u.ID)
	if err != nil {
		return nil, translate(err)
	}
	return u, nil
}

func (s *Store) UserByName(ctx context.Context, username string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `select `+userColumns+` from users u where u.username = ?`, username))
}

func (s *Store) User(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `select `+userColumns+` from users u where u.id = ?`, id))
}

func (s *Store) UserByHandle(ctx context.Context, handle []byte) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `select `+userColumns+` from users u where u.handle = ?`, handle))
}

func (s *Store) Users(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, `select `+userColumns+` from users u order by u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

func scanUser(row scanner, extra ...any) (*User, error) {
	var (
		u             User
		created, seen int64
	)
	dest := append([]any{&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.TOTPSecret, &u.TOTPStep, &u.Handle, &created, &seen}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, translate(err)
	}
	u.CreatedAt = fromUnix(created)
	u.LastSeenAt = fromUnix(seen)
	return &u, nil
}

func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	return s.updateUser(ctx, `update users set password_hash = ? where id = ?`, hash, id)
}

func (s *Store) SetRole(ctx context.Context, id int64, role Role) error {
	return s.updateUser(ctx, `update users set role = ? where id = ? and role != 'owner'`, role, id)
}

func (s *Store) SetTOTP(ctx context.Context, id int64, secret string) error {
	return s.updateUser(ctx, `update users set totp_secret = ?, totp_step = 0 where id = ?`, secret, id)
}

func (s *Store) UseTOTPStep(ctx context.Context, id, step int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `update users set totp_step = ? where id = ? and totp_step < ?`, step, id, step)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) TouchUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `update users set last_seen_at = ? where id = ?`, time.Now().Unix(), id)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return s.updateUser(ctx, `delete from users where id = ? and role != 'owner'`, id)
}

func (s *Store) updateUser(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `insert into sessions (token_hash, user_id, created_at, expires_at) values (?, ?, ?, ?)`,
		tokenHash, userID, time.Now().Unix(), expires.Unix())
	return translate(err)
}

func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (*User, time.Time, error) {
	var expires int64
	u, err := scanUser(s.db.QueryRowContext(ctx, `
		select `+userColumns+`, s.expires_at
		from sessions s join users u on u.id = s.user_id
		where s.token_hash = ? and s.expires_at > ?`, tokenHash, time.Now().Unix()), &expires)
	if err != nil {
		return nil, time.Time{}, err
	}
	return u, fromUnix(expires), nil
}

func (s *Store) CountSessions(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `select user_id, count(*) from sessions where expires_at > ? group by user_id`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (s *Store) ExtendSession(ctx context.Context, tokenHash []byte, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `update sessions set expires_at = ? where token_hash = ?`, expires.Unix(), tokenHash)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, `delete from sessions where token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, keep []byte) error {
	_, err := s.db.ExecContext(ctx, `delete from sessions where user_id = ? and token_hash != ?`, userID, keep)
	return err
}

type Invite struct {
	ID        int64
	Role      Role
	CreatedBy string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *Store) CreateInvite(ctx context.Context, tokenHash []byte, role Role, by int64, expires time.Time) (*Invite, error) {
	inv := &Invite{Role: role, CreatedAt: time.Now().UTC().Truncate(time.Second), ExpiresAt: expires.UTC().Truncate(time.Second)}
	err := s.db.QueryRowContext(ctx, `
		insert into invites (token_hash, role, created_by, created_at, expires_at) values (?, ?, ?, ?, ?)
		returning id, coalesce((select username from users where id = ?), '')`,
		tokenHash, role, by, inv.CreatedAt.Unix(), inv.ExpiresAt.Unix(), by).Scan(&inv.ID, &inv.CreatedBy)
	if err != nil {
		return nil, translate(err)
	}
	return inv, nil
}

func (s *Store) Invites(ctx context.Context) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx, `
		select i.id, i.role, coalesce(u.username, ''), i.created_at, i.expires_at
		from invites i left join users u on u.id = i.created_by
		where i.expires_at > ? order by i.id desc`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *inv)
	}
	return list, rows.Err()
}

func (s *Store) Invite(ctx context.Context, tokenHash []byte) (*Invite, error) {
	return scanInvite(s.db.QueryRowContext(ctx, `
		select i.id, i.role, coalesce(u.username, ''), i.created_at, i.expires_at
		from invites i left join users u on u.id = i.created_by
		where i.token_hash = ? and i.expires_at > ?`, tokenHash, time.Now().Unix()))
}

func scanInvite(row scanner) (*Invite, error) {
	var (
		inv              Invite
		created, expires int64
	)
	if err := row.Scan(&inv.ID, &inv.Role, &inv.CreatedBy, &created, &expires); err != nil {
		return nil, translate(err)
	}
	inv.CreatedAt = fromUnix(created)
	inv.ExpiresAt = fromUnix(expires)
	return &inv, nil
}

func (s *Store) DeleteInvite(ctx context.Context, id int64) error {
	return s.updateUser(ctx, `delete from invites where id = ?`, id)
}

func (s *Store) AcceptInvite(ctx context.Context, tokenHash []byte, username, hash string) (*User, error) {
	var u *User
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var role Role
		err := tx.QueryRowContext(ctx, `delete from invites where token_hash = ? and expires_at > ? returning role`, tokenHash, time.Now().Unix()).Scan(&role)
		if err != nil {
			return translate(err)
		}
		u, err = insertUser(ctx, tx, username, hash, role)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	return u, err
}
