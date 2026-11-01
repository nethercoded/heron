package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"github.com/nethercode/heron/internal/store"
)

const (
	CookieName    = "heron_session"
	sessionTTL    = 30 * 24 * time.Hour
	refreshWindow = 7 * 24 * time.Hour
)

var ErrInvalidCredentials = errors.New("invalid username or password")

type Sessions struct {
	store  *store.Store
	secure bool
	dummy  string
}

func NewSessions(s *store.Store, secure bool) *Sessions {
	dummy, _ := Hash(rand.Text())
	return &Sessions{store: s, secure: secure, dummy: dummy}
}

func (s *Sessions) Login(ctx context.Context, w http.ResponseWriter, username, password string) (*store.User, error) {
	user, err := s.store.UserByName(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		Verify(s.dummy, password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	ok, err := Verify(user.PasswordHash, password)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	return user, s.Start(ctx, w, user)
}

func (s *Sessions) Start(ctx context.Context, w http.ResponseWriter, user *store.User) error {
	token := rand.Text()
	expires := time.Now().Add(sessionTTL)
	if err := s.store.CreateSession(ctx, digest(token), user.ID, expires); err != nil {
		return err
	}
	http.SetCookie(w, s.cookie(token, expires))
	return nil
}

func (s *Sessions) User(ctx context.Context, w http.ResponseWriter, r *http.Request) (*store.User, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil, store.ErrNotFound
	}
	hash := digest(c.Value)
	user, expires, err := s.store.SessionUser(ctx, hash)
	if err != nil {
		return nil, err
	}
	if time.Until(expires) < sessionTTL-refreshWindow {
		next := time.Now().Add(sessionTTL)
		if err := s.store.ExtendSession(ctx, hash, next); err == nil {
			http.SetCookie(w, s.cookie(c.Value, next))
		}
	}
	return user, nil
}

func (s *Sessions) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, s.cookie("", time.Unix(1, 0)))
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	return s.store.DeleteSession(ctx, digest(c.Value))
}

func (s *Sessions) Revoke(ctx context.Context, r *http.Request, userID int64) error {
	var keep []byte
	if c, err := r.Cookie(CookieName); err == nil {
		keep = digest(c.Value)
	}
	return s.store.DeleteUserSessions(ctx, userID, keep)
}

func (s *Sessions) cookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func digest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
