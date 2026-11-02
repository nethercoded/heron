package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nethercoded/heron/internal/kuma"
	"github.com/nethercoded/heron/internal/store"
)

const maxImport = 256 << 20

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) error {
	dir, err := os.MkdirTemp("", "heron-backup-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	name := store.BackupName("heron-", time.Now(), "")
	path := filepath.Join(dir, name)
	if err := s.store.Backup(r.Context(), path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	describe(r, name)
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	_, err = io.Copy(w, f)
	return err
}

type importChannelDTO struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type importMonitorDTO struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Target string   `json:"target"`
	Group  string   `json:"group,omitzero"`
	Paused bool     `json:"paused"`
	Notes  []string `json:"notes,omitzero"`
}

type importDTO struct {
	Channels []importChannelDTO `json:"channels"`
	Monitors []importMonitorDTO `json:"monitors"`
	Skipped  []kuma.Skipped     `json:"skipped"`
}

func (s *Server) readKuma(w http.ResponseWriter, r *http.Request) (*kuma.Plan, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImport)
	file, _, err := r.FormFile("file")
	if err != nil {
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			return nil, errorf(http.StatusRequestEntityTooLarge, "too_large", "the file is larger than 256 MB, use heron import kuma on the server")
		}
		return nil, badRequest("attach the Uptime Kuma backup as file")
	}
	defer file.Close()
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	tmp, err := os.CreateTemp("", "heron-kuma-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	src, err := kuma.Read(r.Context(), tmp.Name())
	if err != nil {
		return nil, badRequest("%s", err.Error())
	}
	return src.Plan(), nil
}

func toImport(p *kuma.Plan) importDTO {
	out := importDTO{Channels: []importChannelDTO{}, Monitors: []importMonitorDTO{}, Skipped: p.Skipped}
	if out.Skipped == nil {
		out.Skipped = []kuma.Skipped{}
	}
	for _, ch := range p.Channels {
		out.Channels = append(out.Channels, importChannelDTO{Name: ch.Name, Kind: string(ch.Kind)})
	}
	for _, m := range p.Monitors {
		out.Monitors = append(out.Monitors, importMonitorDTO{
			Name:   m.Name,
			Kind:   string(m.Kind),
			Target: m.DisplayTarget(),
			Group:  m.Group,
			Paused: m.Paused,
			Notes:  p.Notes[m.Name],
		})
	}
	return out
}

func (s *Server) previewKuma(w http.ResponseWriter, r *http.Request) error {
	plan, err := s.readKuma(w, r)
	if err != nil {
		return err
	}
	return ok(w, toImport(plan))
}

func (s *Server) importKuma(w http.ResponseWriter, r *http.Request) error {
	plan, err := s.readKuma(w, r)
	if err != nil {
		return err
	}
	if len(plan.Monitors) == 0 && len(plan.Channels) == 0 {
		return badRequest("there is nothing heron can import from this file")
	}
	if err := plan.Apply(r.Context(), s.store); err != nil {
		return err
	}
	for _, m := range plan.Monitors {
		s.engine.Forget(m.ID)
		s.scheduler.Sync(m)
		s.engine.Changed(m)
	}
	describe(r, fmt.Sprintf("%d monitors, %d channels", len(plan.Monitors), len(plan.Channels)))
	s.log.Info("imported from uptime kuma", "monitors", len(plan.Monitors), "channels", len(plan.Channels), "skipped", len(plan.Skipped))
	return ok(w, toImport(plan))
}
