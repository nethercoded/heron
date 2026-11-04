package httpapi

import (
	"net/http"
	"slices"
	"time"

	"github.com/nethercoded/heron/internal/expiry"
)

type expiryDTO struct {
	Host      string       `json:"host"`
	Kind      string       `json:"kind"`
	ExpiresAt stamp        `json:"expires_at"`
	DaysLeft  *int         `json:"days_left"`
	CheckedAt stamp        `json:"checked_at"`
	Issuer    string       `json:"issuer,omitzero"`
	Error     string       `json:"error,omitzero"`
	Monitors  []monitorRef `json:"monitors"`
}

type monitorRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type expiryListDTO struct {
	Items    []expiryDTO `json:"items"`
	Checking bool        `json:"checking"`
}

func (s *Server) listExpiry(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	saved, err := s.store.Expiries(ctx)
	if err != nil {
		return err
	}
	monitors, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	byID := index(monitors)
	refs := map[string][]monitorRef{}
	for _, t := range expiry.Targets(monitors) {
		for _, id := range t.Monitors {
			if m := byID[id]; m != nil {
				refs[string(t.Kind)+" "+t.Host] = append(refs[string(t.Kind)+" "+t.Host], monitorRef{ID: m.ID, Name: m.Name})
			}
		}
	}

	now := time.Now()
	out := expiryListDTO{Items: make([]expiryDTO, 0, len(saved))}
	for _, e := range saved {
		dto := expiryDTO{
			Host:      e.Host,
			Kind:      e.Kind,
			ExpiresAt: stamp(e.ExpiresAt),
			CheckedAt: stamp(e.CheckedAt),
			Issuer:    e.Issuer,
			Error:     e.Error,
			Monitors:  refs[e.Kind+" "+e.Host],
		}
		if dto.Monitors == nil {
			dto.Monitors = []monitorRef{}
		}
		if !e.ExpiresAt.IsZero() {
			days := expiry.DaysLeft(e.ExpiresAt, now)
			dto.DaysLeft = &days
		}
		out.Items = append(out.Items, dto)
	}
	slices.SortStableFunc(out.Items, func(a, b expiryDTO) int {
		switch {
		case a.DaysLeft == nil && b.DaysLeft == nil:
			return 0
		case a.DaysLeft == nil:
			return 1
		case b.DaysLeft == nil:
			return -1
		}
		return *a.DaysLeft - *b.DaysLeft
	})
	if s.opts.Expiry != nil {
		out.Checking = s.opts.Expiry.Checking()
	}
	return ok(w, out)
}

func (s *Server) checkExpiry(w http.ResponseWriter, _ *http.Request) error {
	if s.opts.Expiry == nil {
		return errorf(http.StatusServiceUnavailable, "unavailable", "expiry checks are not running")
	}
	s.opts.Expiry.Begin()
	w.WriteHeader(http.StatusAccepted)
	return nil
}
