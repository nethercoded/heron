package store

import (
	"context"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, hash string) (*User, error) {
	u := &User{Username: username, PasswordHash: hash, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	err := s.db.QueryRowContext(ctx, `insert into users (username, password_hash, created_at) values (?, ?, ?) returning id`,
		username, hash, u.CreatedAt.Unix()).Scan(&u.ID)
	if err != nil {
		return nil, translate(err)
	}
	return u, nil
}

func (s *Store) CreateFirstUser(ctx context.Context, username, hash string) (*User, error) {
	u := &User{Username: username, PasswordHash: hash, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	err := s.db.QueryRowContext(ctx, `
		insert into users (username, password_hash, created_at)
		select ?, ?, ? where not exists (select 1 from users) returning id`,
		username, hash, u.CreatedAt.Unix()).Scan(&u.ID)
	if err != nil {
		if translate(err) == ErrNotFound {
			return nil, ErrConflict
		}
		return nil, translate(err)
	}
	return u, nil
}

func (s *Store) UserByName(ctx context.Context, username string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `select id, username, password_hash, created_at from users where username = ?`, username))
}

func (s *Store) User(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `select id, username, password_hash, created_at from users where id = ?`, id))
}

func scanUser(row scanner) (*User, error) {
	var u User
	var created int64
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &created); err != nil {
		return nil, translate(err)
	}
	u.CreatedAt = fromUnix(created)
	return &u, nil
}

func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	res, err := s.db.ExecContext(ctx, `update users set password_hash = ? where id = ?`, hash, id)
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
	var (
		u                User
		created, expires int64
	)
	err := s.db.QueryRowContext(ctx, `
		select u.id, u.username, u.password_hash, u.created_at, s.expires_at
		from sessions s join users u on u.id = s.user_id
		where s.token_hash = ? and s.expires_at > ?`, tokenHash, time.Now().Unix()).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &created, &expires)
	if err != nil {
		return nil, time.Time{}, translate(err)
	}
	u.CreatedAt = fromUnix(created)
	return &u, fromUnix(expires), nil
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
