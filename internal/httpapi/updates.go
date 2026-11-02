package httpapi

import (
	"errors"
	"net/http"

	"github.com/nethercoded/heron/internal/update"
)

type releaseDTO struct {
	Version     string `json:"version"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
	PublishedAt stamp  `json:"published_at"`
}

type updatesDTO struct {
	Current   string      `json:"current"`
	Latest    *releaseDTO `json:"latest"`
	Available bool        `json:"available"`
	CheckedAt stamp       `json:"checked_at"`
	Error     string      `json:"error,omitzero"`
	Blocked   string      `json:"blocked,omitzero"`
	Busy      bool        `json:"busy"`
	Failed    string      `json:"failed,omitzero"`
	Auto      bool        `json:"auto"`
}

func (s *Server) updatesDTO(r *http.Request, st update.Status) (updatesDTO, error) {
	auto, err := s.store.AutoUpdate(r.Context())
	if err != nil {
		return updatesDTO{}, err
	}
	out := updatesDTO{
		Current:   st.Current,
		Available: st.Available,
		CheckedAt: stamp(st.CheckedAt),
		Busy:      st.Busy,
		Auto:      auto,
	}
	if st.Latest != nil {
		out.Latest = &releaseDTO{
			Version:     st.Latest.Version(),
			URL:         st.Latest.URL,
			Notes:       st.Latest.Notes,
			PublishedAt: stamp(st.Latest.Published),
		}
	}
	if st.Err != nil {
		out.Error = "could not reach GitHub"
	}
	if st.Ready != nil {
		out.Blocked = st.Ready.Error()
	}
	if st.Failed != nil {
		out.Failed = st.Failed.Error()
	}
	return out, nil
}

func (s *Server) getUpdates(w http.ResponseWriter, r *http.Request) error {
	if s.opts.Updates == nil {
		return notFound("updates")
	}
	out, err := s.updatesDTO(r, s.opts.Updates.Status())
	if err != nil {
		return err
	}
	return ok(w, out)
}

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) error {
	if s.opts.Updates == nil {
		return notFound("updates")
	}
	st := s.opts.Updates.Check(r.Context())
	if st.Err != nil {
		s.log.Warn("could not check for updates", "err", st.Err)
	}
	out, err := s.updatesDTO(r, st)
	if err != nil {
		return err
	}
	return ok(w, out)
}

func (s *Server) saveUpdates(w http.ResponseWriter, r *http.Request) error {
	if s.opts.Updates == nil {
		return notFound("updates")
	}
	var in struct {
		Auto bool `json:"auto"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := s.store.SetAutoUpdate(r.Context(), in.Auto); err != nil {
		return err
	}
	if in.Auto {
		describe(r, "on")
	} else {
		describe(r, "off")
	}
	out, err := s.updatesDTO(r, s.opts.Updates.Status())
	if err != nil {
		return err
	}
	return ok(w, out)
}

func (s *Server) installUpdate(w http.ResponseWriter, r *http.Request) error {
	if s.opts.Updates == nil {
		return notFound("updates")
	}
	if err := s.opts.Updates.Updater.Ready(); err != nil {
		return errorf(http.StatusConflict, "unsupported", "%s", err)
	}
	rel, err := s.opts.Updates.Begin(r.Context())
	switch {
	case errors.Is(err, update.ErrUpToDate):
		return errorf(http.StatusConflict, "up_to_date", "heron %s is the latest version", s.opts.Version)
	case errors.Is(err, update.ErrBusy):
		return errorf(http.StatusConflict, "busy", "an update is already being installed")
	case err != nil:
		s.log.Error("update failed", "err", err)
		return errorf(http.StatusBadGateway, "update_failed", "%s", err)
	}
	describe(r, rel.Version())
	return writeJSON(w, http.StatusAccepted, map[string]string{"version": rel.Version()})
}
