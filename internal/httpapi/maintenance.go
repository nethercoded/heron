package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/nethercode/heron/internal/maintenance"
	"github.com/nethercode/heron/internal/monitor"
)

type maintenanceInput struct {
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	MonitorIDs []int64            `json:"monitor_ids"`
	StartsAt   time.Time          `json:"starts_at"`
	DurationS  int64              `json:"duration_s"`
	Repeat     maintenance.Repeat `json:"repeat"`
	Enabled    bool               `json:"enabled"`
}

type maintenanceDTO struct {
	ID         int64              `json:"id"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	StartsAt   stamp              `json:"starts_at"`
	EndsAt     stamp              `json:"ends_at"`
	DurationS  int64              `json:"duration_s"`
	Repeat     maintenance.Repeat `json:"repeat"`
	Active     bool               `json:"active"`
	Enabled    bool               `json:"enabled,omitzero"`
	Finished   bool               `json:"finished,omitzero"`
	Monitors   []affectedDTO      `json:"monitors"`
	MonitorIDs []int64            `json:"monitor_ids,omitzero"`
}

func (s *Server) toMaintenance(w maintenance.Window, names map[int64]*monitor.Monitor, now time.Time, admin bool) maintenanceDTO {
	dto := maintenanceDTO{
		ID:        w.ID,
		Title:     w.Title,
		Body:      w.Body,
		StartsAt:  stamp(w.StartsAt),
		EndsAt:    stamp(w.StartsAt.Add(w.Duration)),
		DurationS: int64(w.Duration.Seconds()),
		Repeat:    w.Repeat,
		Monitors:  []affectedDTO{},
	}
	if occ, ok := w.Next(now, s.engine.Calendar().Location()); ok {
		dto.StartsAt, dto.EndsAt = stamp(occ.Start), stamp(occ.End)
		dto.Active = w.Enabled && occ.Contains(now)
	} else {
		dto.Finished = true
	}
	if admin {
		dto.Enabled = w.Enabled
		dto.MonitorIDs = w.MonitorIDs
	}
	for _, id := range w.MonitorIDs {
		if m, ok := names[id]; ok && (admin || m.Public) {
			dto.Monitors = append(dto.Monitors, affectedDTO{Slug: m.Slug, Name: m.Name})
		}
	}
	return dto
}

func (s *Server) publicMaintenance(names map[int64]*monitor.Monitor, now time.Time) []maintenanceDTO {
	out := []maintenanceDTO{}
	for _, item := range s.engine.Calendar().Upcoming(now, 7*24*time.Hour) {
		dto := s.toMaintenance(item.Window, names, now, false)
		if len(dto.Monitors) > 0 {
			out = append(out, dto)
		}
	}
	return out
}

func (s *Server) listMaintenance(w http.ResponseWriter, r *http.Request) error {
	windows, err := s.store.MaintenanceWindows(r.Context())
	if err != nil {
		return err
	}
	names, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	now := time.Now()
	out := make([]maintenanceDTO, 0, len(windows))
	for _, mw := range windows {
		out = append(out, s.toMaintenance(mw, names, now, true))
	}
	return ok(w, out)
}

func (s *Server) getMaintenance(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	mw, err := s.store.MaintenanceWindow(r.Context(), id)
	if err != nil {
		return err
	}
	names, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	return ok(w, s.toMaintenance(mw, names, time.Now(), true))
}

func (s *Server) createMaintenance(w http.ResponseWriter, r *http.Request) error {
	return s.saveMaintenance(w, r, 0)
}

func (s *Server) updateMaintenance(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if _, err := s.store.MaintenanceWindow(r.Context(), id); err != nil {
		return err
	}
	return s.saveMaintenance(w, r, id)
}

func (s *Server) saveMaintenance(w http.ResponseWriter, r *http.Request, id int64) error {
	var in maintenanceInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	mw := maintenance.Window{
		ID:         id,
		Title:      strings.TrimSpace(in.Title),
		Body:       strings.TrimSpace(in.Body),
		MonitorIDs: in.MonitorIDs,
		StartsAt:   in.StartsAt,
		Duration:   time.Duration(in.DurationS) * time.Second,
		Repeat:     in.Repeat,
		Enabled:    in.Enabled,
	}
	if mw.Repeat == "" {
		mw.Repeat = maintenance.RepeatNone
	}
	if err := mw.Validate(); err != nil {
		return invalid(err)
	}
	if err := s.store.SaveMaintenanceWindow(r.Context(), &mw); err != nil {
		return err
	}
	if err := s.engine.ReloadMaintenance(r.Context()); err != nil {
		return err
	}

	names, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	dto := s.toMaintenance(mw, names, time.Now(), true)
	if id == 0 {
		return created(w, dto)
	}
	return ok(w, dto)
}

func (s *Server) deleteMaintenance(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteMaintenanceWindow(r.Context(), id); err != nil {
		return err
	}
	if err := s.engine.ReloadMaintenance(r.Context()); err != nil {
		return err
	}
	return noContent(w)
}
