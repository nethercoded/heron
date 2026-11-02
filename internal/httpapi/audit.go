package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/nethercoded/heron/internal/store"
)

type auditKey struct{}

type auditNote struct {
	target string
}

func (s *Server) audited(action string, h handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		note := &auditNote{}
		r = r.WithContext(context.WithValue(r.Context(), auditKey{}, note))
		if err := h(w, r); err != nil {
			return err
		}
		if note.target == "" && r.PathValue("id") != "" {
			note.target = "#" + r.PathValue("id")
		}
		s.record(r, userFrom(r.Context()), action, note.target)
		return nil
	}
}

func describe(r *http.Request, target string) {
	if note, ok := r.Context().Value(auditKey{}).(*auditNote); ok {
		note.target = target
	}
}

func (s *Server) record(r *http.Request, user *store.User, action, target string) {
	entry := store.AuditEntry{Action: action, Target: target, IP: s.clientIP(r)}
	if user != nil {
		entry.UserID = user.ID
		entry.Username = user.Username
	}
	if err := s.store.Audit(context.WithoutCancel(r.Context()), entry); err != nil {
		s.log.Warn("write audit log", "action", action, "err", err)
	}
}

type auditDTO struct {
	ID       int64  `json:"id"`
	At       stamp  `json:"at"`
	UserID   int64  `json:"user_id,omitzero"`
	Username string `json:"username"`
	Action   string `json:"action"`
	Target   string `json:"target"`
	IP       string `json:"ip"`
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	userID, _ := strconv.ParseInt(q.Get("user"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))

	list, err := s.store.AuditLog(r.Context(), store.AuditFilter{Before: before, UserID: userID, Action: q.Get("action"), Limit: limit})
	if err != nil {
		return err
	}
	out := make([]auditDTO, 0, len(list))
	for _, e := range list {
		out = append(out, auditDTO{ID: e.ID, At: stamp(e.At), UserID: e.UserID, Username: e.Username, Action: e.Action, Target: e.Target, IP: e.IP})
	}
	return ok(w, out)
}
