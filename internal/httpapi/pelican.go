package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/store"
)

type pelicanDTO struct {
	Enabled    bool          `json:"enabled"`
	URL        string        `json:"url"`
	HasKey     bool          `json:"has_key"`
	Public     bool          `json:"public"`
	ChannelIDs []int64       `json:"channel_ids"`
	Status     pelicanStatus `json:"status"`
}

type pelicanStatus struct {
	SyncedAt stamp  `json:"synced_at"`
	Nodes    int    `json:"nodes"`
	Servers  int    `json:"servers"`
	Error    string `json:"error,omitzero"`
	Syncing  bool   `json:"syncing"`
}

type pelicanInput struct {
	Enabled    bool    `json:"enabled"`
	URL        string  `json:"url"`
	Key        string  `json:"key"`
	Public     bool    `json:"public"`
	ChannelIDs []int64 `json:"channel_ids"`
}

func (s *Server) pelicanDTO(r *http.Request) (pelicanDTO, error) {
	cfg, err := s.store.Pelican(r.Context())
	if err != nil {
		return pelicanDTO{}, err
	}
	out := pelicanDTO{Enabled: cfg.Enabled, URL: cfg.URL, HasKey: cfg.Key != "", Public: cfg.Public, ChannelIDs: cfg.ChannelIDs}
	if out.ChannelIDs == nil {
		out.ChannelIDs = []int64{}
	}
	if s.opts.Pelican != nil {
		st := s.opts.Pelican.Status()
		out.Status = pelicanStatus{SyncedAt: stamp(st.SyncedAt), Nodes: st.Nodes, Servers: st.Servers, Syncing: st.Syncing}
		if st.Err != nil {
			out.Status.Error = st.Err.Error()
		}
	}
	return out, nil
}

func (s *Server) getPelican(w http.ResponseWriter, r *http.Request) error {
	out, err := s.pelicanDTO(r)
	if err != nil {
		return err
	}
	return ok(w, out)
}

func (s *Server) savePelican(w http.ResponseWriter, r *http.Request) error {
	var in pelicanInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	old, err := s.store.Pelican(r.Context())
	if err != nil {
		return err
	}
	cfg := store.Pelican{
		Enabled:    in.Enabled,
		URL:        strings.TrimSuffix(strings.TrimSpace(in.URL), "/"),
		Key:        strings.TrimSpace(in.Key),
		Public:     in.Public,
		ChannelIDs: in.ChannelIDs,
	}
	if cfg.Key == "" {
		cfg.Key = old.Key
	}
	fields := map[string]string{}
	if cfg.Enabled && !webURL(cfg.URL) {
		fields["url"] = "must be the address of your panel, like https://panel.example.com"
	}
	if cfg.Enabled && cfg.Key == "" {
		fields["key"] = "paste an application API key from the panel"
	}
	if len(fields) > 0 {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: fields}
	}
	if err := s.store.SavePelican(r.Context(), cfg); err != nil {
		return err
	}
	describe(r, map[bool]string{true: "on", false: "off"}[cfg.Enabled])

	if s.opts.Pelican != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 25*time.Second)
		defer cancel()
		_ = s.opts.Pelican.Sync(ctx)
	}
	out, err := s.pelicanDTO(r)
	if err != nil {
		return err
	}
	return ok(w, out)
}

func (s *Server) syncPelican(w http.ResponseWriter, _ *http.Request) error {
	if s.opts.Pelican == nil {
		return errorf(http.StatusServiceUnavailable, "unavailable", "the pelican integration is not running")
	}
	s.opts.Pelican.Begin()
	w.WriteHeader(http.StatusAccepted)
	return nil
}
