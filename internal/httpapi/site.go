package httpapi

import (
	"context"
	"crypto/subtle"
	"io"
	"net/http"
	"strings"

	"github.com/nethercoded/heron/internal/site"
)

type Site interface {
	Settings() site.Settings
	Apply(ctx context.Context, s site.Settings) (site.Result, error)
	Probe(nonce string) bool
}

type siteInput struct {
	Code   string    `json:"code"`
	Domain string    `json:"domain"`
	Mode   site.Mode `json:"mode"`
	Email  string    `json:"email"`
}

type siteDTO struct {
	Domain    string    `json:"domain"`
	Mode      site.Mode `json:"mode"`
	Email     string    `json:"email"`
	URL       string    `json:"url"`
	Reachable bool      `json:"reachable"`
}

func (s *Server) probe(w http.ResponseWriter, r *http.Request) {
	nonce := r.PathValue("nonce")
	if s.opts.Site == nil || !s.opts.Site.Probe(nonce) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.WriteString(w, nonce); err != nil {
		s.log.Debug("write probe", "err", err)
	}
}

func (s *Server) getSite(w http.ResponseWriter, _ *http.Request) error {
	if s.opts.Site == nil {
		return ok(w, siteDTO{})
	}
	cur := s.opts.Site.Settings()
	return ok(w, siteDTO{Domain: cur.Domain, Mode: cur.Mode, Email: cur.Email, URL: cur.URL(), Reachable: true})
}

func (s *Server) saveSite(w http.ResponseWriter, r *http.Request) error {
	var in siteInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	return s.applySite(w, r, in)
}

func (s *Server) installDomain(w http.ResponseWriter, r *http.Request) error {
	if allowed, _ := s.limiter.Allow("setup:" + s.clientIP(r)); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}
	var in siteInput
	if err := decode(w, r, &in); err != nil {
		return err
	}

	users, err := s.store.CountUsers(r.Context())
	if err != nil {
		return err
	}
	if users > 0 {
		if _, err := s.sessions.User(r.Context(), w, r); err != nil {
			return errorf(http.StatusUnauthorized, "unauthorized", "sign in required")
		}
	} else if code := s.opts.SetupCode; code != "" && subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(in.Code))), []byte(code)) != 1 {
		return &Error{Status: http.StatusForbidden, Code: "invalid", Message: "validation failed", Fields: map[string]string{"code": "does not match the code in the server log"}}
	}
	return s.applySite(w, r, in)
}

func (s *Server) applySite(w http.ResponseWriter, r *http.Request, in siteInput) error {
	if s.opts.Site == nil {
		return errorf(http.StatusNotImplemented, "unsupported", "domain management is not available")
	}
	res, err := s.opts.Site.Apply(r.Context(), site.Settings{Domain: in.Domain, Mode: in.Mode, Email: in.Email})
	if err != nil {
		return err
	}
	cur := s.opts.Site.Settings()
	s.log.Info("site updated", "domain", cur.Domain, "mode", cur.Mode)
	return ok(w, siteDTO{Domain: cur.Domain, Mode: cur.Mode, Email: cur.Email, URL: res.URL, Reachable: res.Reachable})
}
