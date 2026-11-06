package httpapi

import (
	"encoding/json/jsontext"
	"net/http"
	"strconv"
)

type evidenceDTO struct {
	ID          int64          `json:"id"`
	MonitorID   int64          `json:"monitor_id"`
	MonitorName string         `json:"monitor_name"`
	At          stamp          `json:"at"`
	Report      jsontext.Value `json:"report"`
	Image       bool           `json:"image"`
}

func (s *Server) incidentEvidence(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	list, err := s.store.IncidentEvidence(r.Context(), id)
	if err != nil {
		return err
	}
	monitors, err := s.store.Monitors(r.Context())
	if err != nil {
		return err
	}
	byID := index(monitors)
	out := make([]evidenceDTO, 0, len(list))
	for _, e := range list {
		dto := evidenceDTO{ID: e.ID, MonitorID: e.MonitorID, At: stamp(e.At), Report: jsontext.Value(e.Report), Image: e.HasImage}
		if m := byID[e.MonitorID]; m != nil {
			dto.MonitorName = m.Name
		}
		out = append(out, dto)
	}
	return ok(w, out)
}

func (s *Server) evidenceImage(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	image, err := s.store.EvidenceImage(r.Context(), id)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(image)))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, err = w.Write(image)
	return err
}
