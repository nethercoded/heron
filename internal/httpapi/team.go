package httpapi

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/store"
)

const inviteTTL = 72 * time.Hour

type memberDTO struct {
	userDTO
	Sessions int `json:"sessions"`
	Passkeys int `json:"passkeys"`
}

type inviteDTO struct {
	ID        int64      `json:"id"`
	Role      store.Role `json:"role"`
	CreatedBy string     `json:"created_by"`
	CreatedAt stamp      `json:"created_at"`
	ExpiresAt stamp      `json:"expires_at"`
	Token     string     `json:"token,omitzero"`
}

func toInvite(inv *store.Invite) inviteDTO {
	return inviteDTO{ID: inv.ID, Role: inv.Role, CreatedBy: inv.CreatedBy, CreatedAt: stamp(inv.CreatedAt), ExpiresAt: stamp(inv.ExpiresAt)}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	users, err := s.store.Users(ctx)
	if err != nil {
		return err
	}
	sessions, err := s.store.CountSessions(ctx)
	if err != nil {
		return err
	}

	members := make([]memberDTO, 0, len(users))
	for _, u := range users {
		keys, err := s.store.Passkeys(ctx, u.ID)
		if err != nil {
			return err
		}
		members = append(members, memberDTO{userDTO: *toUser(u), Sessions: sessions[u.ID], Passkeys: len(keys)})
	}

	invites := []inviteDTO{}
	if userFrom(ctx).Role.Allows(store.RoleOwner) {
		list, err := s.store.Invites(ctx)
		if err != nil {
			return err
		}
		for _, inv := range list {
			invites = append(invites, toInvite(&inv))
		}
	}
	return ok(w, map[string]any{"users": members, "invites": invites})
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Role store.Role `json:"role"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Role != store.RoleAdmin && in.Role != store.RoleViewer {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"role": "must be admin or viewer"}}
	}

	token := rand.Text()
	inv, err := s.store.CreateInvite(r.Context(), auth.Digest(token), in.Role, userFrom(r.Context()).ID, time.Now().Add(inviteTTL))
	if err != nil {
		return err
	}
	describe(r, string(in.Role))
	dto := toInvite(inv)
	dto.Token = token
	return created(w, dto)
}

func (s *Server) deleteInvite(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteInvite(r.Context(), id); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) loadMember(r *http.Request) (*store.User, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	u, err := s.store.User(r.Context(), id)
	if err != nil {
		return nil, notFound("user")
	}
	if u.Role == store.RoleOwner {
		return nil, errorf(http.StatusForbidden, "forbidden", "the owner account cannot be changed here")
	}
	describe(r, u.Username)
	return u, nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) error {
	u, err := s.loadMember(r)
	if err != nil {
		return err
	}
	var in struct {
		Role store.Role `json:"role"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Role != store.RoleAdmin && in.Role != store.RoleViewer {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"role": "must be admin or viewer"}}
	}
	if err := s.store.SetRole(r.Context(), u.ID, in.Role); err != nil {
		return err
	}
	describe(r, u.Username+" → "+string(in.Role))
	u.Role = in.Role
	return ok(w, toUser(u))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) error {
	u, err := s.loadMember(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteUser(r.Context(), u.ID); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) revokeUserSessions(w http.ResponseWriter, r *http.Request) error {
	u, err := s.loadMember(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteUserSessions(r.Context(), u.ID, nil); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) resetTwoFactor(w http.ResponseWriter, r *http.Request) error {
	u, err := s.loadMember(r)
	if err != nil {
		return err
	}
	if err := s.store.SetTOTP(r.Context(), u.ID, ""); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) inviteInfo(w http.ResponseWriter, r *http.Request) error {
	if allowed, _ := s.limiter.Allow("invite:" + s.clientIP(r)); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	inv, err := s.store.Invite(r.Context(), auth.Digest(strings.TrimSpace(in.Token)))
	if err != nil {
		return errorf(http.StatusNotFound, "invite_invalid", "this invite link is invalid or has expired")
	}
	return ok(w, toInvite(inv))
}

func (s *Server) join(w http.ResponseWriter, r *http.Request) error {
	if allowed, _ := s.limiter.Allow("invite:" + s.clientIP(r)); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}
	var in struct {
		credentials
		Token string `json:"token"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Username = strings.TrimSpace(in.Username)
	if err := validateCredentials(in.credentials); err != nil {
		return err
	}

	hash, err := auth.Hash(in.Password)
	if err != nil {
		return err
	}
	user, err := s.store.AcceptInvite(r.Context(), auth.Digest(strings.TrimSpace(in.Token)), in.Username, hash)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errorf(http.StatusNotFound, "invite_invalid", "this invite link is invalid or has expired")
	case errors.Is(err, store.ErrConflict):
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"username": "is already taken"}}
	case err != nil:
		return err
	}
	if err := s.sessions.Start(r.Context(), w, r, user); err != nil {
		return err
	}
	s.record(r, user, "user.join", string(user.Role))
	return created(w, toUser(user))
}
