package httpapi

import (
	"net/http"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

func (s *Server) listMonitors(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	list, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	uptimes, err := s.store.Uptimes(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return err
	}

	viewer := !userFrom(ctx).Role.Allows(store.RoleAdmin)
	out := make([]adminMonitorDTO, 0, len(list))
	for _, m := range list {
		dto := toAdminMonitor(m)
		if viewer {
			dto.redact(m)
		}
		if u, ok := uptimes[m.ID]; ok {
			ratio := u.Ratio()
			dto.Uptime = &ratio
		}
		out = append(out, dto)
	}
	return ok(w, out)
}

func (s *Server) getMonitor(w http.ResponseWriter, r *http.Request) error {
	m, err := s.loadMonitor(r)
	if err != nil {
		return err
	}
	beats, err := s.store.Beats(r.Context(), m.ID, 100)
	if err != nil {
		return err
	}
	dto := toAdminMonitor(m)
	if !userFrom(r.Context()).Role.Allows(store.RoleAdmin) {
		dto.redact(m)
	}
	return ok(w, map[string]any{"monitor": dto, "beats": toBeats(beats, true)})
}

func (s *Server) createMonitor(w http.ResponseWriter, r *http.Request) error {
	var in monitorInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	m := &monitor.Monitor{Paused: in.Paused}
	in.apply(m)
	m.Normalize()
	if err := m.Validate(); err != nil {
		return invalid(err)
	}

	if err := s.store.CreateMonitor(r.Context(), m); err != nil {
		return slugConflict(err)
	}
	s.engine.Forget(m.ID)
	s.scheduler.Sync(m)
	s.engine.Changed(m)
	describe(r, m.Name)
	s.log.Info("monitor created", "monitor", m.Slug, "kind", m.Kind)
	return created(w, toAdminMonitor(m))
}

func (s *Server) updateMonitor(w http.ResponseWriter, r *http.Request) error {
	m, err := s.loadMonitor(r)
	if err != nil {
		return err
	}
	var in monitorInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	kindChanged := in.Kind != m.Kind
	in.apply(m)
	m.Normalize()
	if err := m.Validate(); err != nil {
		return invalid(err)
	}
	if kindChanged {
		m.State.CheckedAt = time.Time{}
	}

	if err := s.store.UpdateMonitor(r.Context(), m); err != nil {
		return slugConflict(err)
	}
	describe(r, m.Name)
	s.engine.Forget(m.ID)
	s.scheduler.Sync(m)
	s.engine.Changed(m)
	return ok(w, toAdminMonitor(m))
}

func (s *Server) deleteMonitor(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if m, err := s.store.Monitor(r.Context(), id); err == nil {
		describe(r, m.Name)
	}
	s.engine.Forget(id)
	s.scheduler.Stop(id)
	if err := s.store.DeleteMonitor(r.Context(), id); err != nil {
		return err
	}
	s.engine.Changed(&monitor.Monitor{ID: id, Public: true})
	return noContent(w)
}

func (s *Server) pauseMonitor(paused bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := pathID(r)
		if err != nil {
			return err
		}
		m, err := s.store.SetPaused(r.Context(), id, paused)
		if err != nil {
			return err
		}
		describe(r, m.Name)
		s.engine.Forget(m.ID)
		s.scheduler.Sync(m)
		s.engine.Changed(m)
		return ok(w, toAdminMonitor(m))
	}
}

func (s *Server) checkMonitor(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if !s.scheduler.Trigger(id) {
		return errorf(http.StatusConflict, "not_running", "monitor is paused or does not support manual checks")
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (s *Server) rotateToken(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	token, err := s.store.RotatePushToken(r.Context(), id)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"push_token": token})
}

func (s *Server) reorderMonitors(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := s.store.ReorderMonitors(r.Context(), in.IDs); err != nil {
		return err
	}
	s.engine.Changed(&monitor.Monitor{Public: true})
	return noContent(w)
}

func (s *Server) loadMonitor(r *http.Request) (*monitor.Monitor, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	m, err := s.store.Monitor(r.Context(), id)
	if err != nil {
		return nil, notFound("monitor")
	}
	return m, nil
}

func slugConflict(err error) error {
	if isConflict(err) {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"slug": "is already taken"}}
	}
	return err
}
