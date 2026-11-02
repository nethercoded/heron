package httpapi

import (
	"crypto/subtle"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/nethercode/heron/internal/auth"
	"github.com/nethercode/heron/internal/store"
)

type authStateDTO struct {
	SetupRequired bool     `json:"setup_required"`
	User          *userDTO `json:"user"`
	Version       string   `json:"version"`
}

func (s *Server) authState(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	n, err := s.store.CountUsers(ctx)
	if err != nil {
		return err
	}
	state := authStateDTO{SetupRequired: n == 0, Version: s.opts.Version}
	if u, err := s.sessions.User(ctx, w, r); err == nil {
		state.User = toUser(u)
	}
	w.Header().Set("Cache-Control", "no-store")
	return ok(w, state)
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) error {
	if allowed, _ := s.limiter.Allow("setup:" + s.clientIP(r)); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}

	var in setupInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if code := s.opts.SetupCode; code != "" && subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(in.Code))), []byte(code)) != 1 {
		s.log.Warn("setup attempt with a wrong code", "ip", s.clientIP(r))
		return &Error{Status: http.StatusForbidden, Code: "invalid", Message: "validation failed", Fields: map[string]string{"code": "does not match the code in the server log"}}
	}
	in.Username = strings.TrimSpace(in.Username)
	if err := validateCredentials(in.credentials); err != nil {
		return err
	}

	hash, err := auth.Hash(in.Password)
	if err != nil {
		return err
	}
	user, err := s.store.CreateFirstUser(r.Context(), in.Username, hash)
	if errors.Is(err, store.ErrConflict) {
		return errorf(http.StatusConflict, "already_setup", "an administrator already exists")
	}
	if err != nil {
		return err
	}
	if err := s.sessions.Start(r.Context(), w, user); err != nil {
		return err
	}
	s.log.Info("administrator created", "username", user.Username)
	return created(w, toUser(user))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	ip := s.clientIP(r)
	if allowed, retry := s.limiter.Allow(ip); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in %d minutes", int(math.Ceil(retry.Minutes())))
	}

	var in credentials
	if err := decode(w, r, &in); err != nil {
		return err
	}
	user, err := s.sessions.Login(r.Context(), w, strings.TrimSpace(in.Username), in.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.log.Warn("failed login", "username", in.Username, "ip", ip)
		return errorf(http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	}
	if err != nil {
		return err
	}
	s.limiter.Reset(ip)
	return ok(w, toUser(user))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if err := s.sessions.Logout(r.Context(), w, r); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	user := userFrom(r.Context())

	valid, err := auth.Verify(user.PasswordHash, in.Current)
	if err != nil {
		return err
	}
	if !valid {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"current": "is incorrect"}}
	}
	if err := auth.CheckStrength(in.Next); err != nil {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"next": err.Error()}}
	}

	hash, err := auth.Hash(in.Next)
	if err != nil {
		return err
	}
	if err := s.store.SetPassword(r.Context(), user.ID, hash); err != nil {
		return err
	}
	if err := s.sessions.Revoke(r.Context(), r, user.ID); err != nil {
		return err
	}
	return noContent(w)
}

func validateCredentials(in credentials) error {
	fields := map[string]string{}
	if len(in.Username) < 3 || len(in.Username) > 32 || strings.ContainsFunc(in.Username, func(r rune) bool { return r <= ' ' }) {
		fields["username"] = "must be 3 to 32 characters without spaces"
	}
	if err := auth.CheckStrength(in.Password); err != nil {
		fields["password"] = err.Error()
	}
	if len(fields) == 0 {
		return nil
	}
	return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: fields}
}
