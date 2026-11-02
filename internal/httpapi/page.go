package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

func (s *Server) getPage(w http.ResponseWriter, r *http.Request) error {
	page, err := s.store.Page(r.Context())
	if err != nil {
		return err
	}
	return ok(w, page)
}

func (s *Server) savePage(w http.ResponseWriter, r *http.Request) error {
	var page store.Page
	if err := decode(w, r, &page); err != nil {
		return err
	}
	page.Title = strings.TrimSpace(page.Title)
	page.Description = strings.TrimSpace(page.Description)
	page.Footer = strings.TrimSpace(page.Footer)

	fields := map[string]string{}
	if page.Title == "" || len(page.Title) > 80 {
		fields["title"] = "must be between 1 and 80 characters"
	}
	if page.HistoryDays < 30 || page.HistoryDays > 365 {
		fields["history_days"] = "must be between 30 and 365"
	}
	for field, value := range map[string]string{"logo_url": page.LogoURL, "home_url": page.HomeURL} {
		if value != "" && !webURL(value) {
			fields[field] = "must be an http or https url"
		}
	}
	if len(fields) > 0 {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: fields}
	}

	if err := s.store.SavePage(r.Context(), page); err != nil {
		return err
	}
	s.engine.Changed(&monitor.Monitor{Public: true})
	return ok(w, page)
}

func webURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
