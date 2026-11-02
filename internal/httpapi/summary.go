package httpapi

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

type summaryItem struct {
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	StartsAt stamp  `json:"starts_at"`
	Active   bool   `json:"active"`
	URL      string `json:"url"`
}

type summaryDTO struct {
	Title  string        `json:"title"`
	Status overall       `json:"status"`
	URL    string        `json:"url"`
	Items  []summaryItem `json:"items"`
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	page, err := s.store.Page(ctx)
	if err != nil {
		return err
	}
	all, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	byID := index(all)
	public := slices.DeleteFunc(slices.Clone(all), func(m *monitor.Monitor) bool { return !m.Public })
	active, err := s.store.Incidents(ctx, store.IncidentFilter{OpenOnly: true})
	if err != nil {
		return err
	}

	out := summaryDTO{Title: page.Title, Status: summarize(public, active), URL: "/", Items: []summaryItem{}}
	for _, inc := range active {
		if !visible(inc, byID) {
			continue
		}
		out.Items = append(out.Items, summaryItem{
			Title:    inc.Title,
			Kind:     "incident",
			Severity: string(inc.Impact),
			StartsAt: stamp(inc.StartedAt),
			Active:   true,
			URL:      "/incidents/" + strconv.FormatInt(inc.ID, 10),
		})
	}
	now := time.Now()
	for _, mw := range s.publicMaintenance(byID, now) {
		out.Items = append(out.Items, summaryItem{
			Title:    mw.Title,
			Kind:     "maintenance",
			Severity: "maintenance",
			StartsAt: mw.StartsAt,
			Active:   mw.Active,
			URL:      "/",
		})
	}

	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cache-Control", "public, max-age=30")
	return ok(w, out)
}
