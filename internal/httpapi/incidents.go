package httpapi

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/store"
)

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	list, err := s.store.Incidents(ctx, store.IncidentFilter{Limit: 500})
	if err != nil {
		return err
	}
	byID, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	out := make([]incidentDTO, 0, len(list))
	for _, inc := range list {
		out = append(out, toIncident(inc, byID, true))
	}
	return ok(w, out)
}

func (s *Server) getIncident(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	inc, err := s.store.Incident(r.Context(), id)
	if err != nil {
		return err
	}
	byID, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	return ok(w, toIncident(inc, byID, true))
}

func (s *Server) createIncident(w http.ResponseWriter, r *http.Request) error {
	var in incidentInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Status == "" {
		in.Status = store.IncidentInvestigating
	}
	inc := &store.Incident{
		Title:      strings.TrimSpace(in.Title),
		Impact:     in.Impact,
		Status:     in.Status,
		MonitorIDs: in.MonitorIDs,
		StartedAt:  in.StartedAt.UTC().Truncate(time.Second),
	}
	body := strings.TrimSpace(in.Body)
	if err := validateIncident(inc, body, true); err != nil {
		return err
	}
	if err := s.store.CreateIncident(r.Context(), inc, body); err != nil {
		return err
	}
	s.engine.IncidentsChanged()

	byID, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	return created(w, toIncident(inc, byID, true))
}

func (s *Server) updateIncident(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	inc, err := s.store.Incident(r.Context(), id)
	if err != nil {
		return err
	}
	var in incidentInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	inc.Title = strings.TrimSpace(in.Title)
	inc.Impact = in.Impact
	inc.MonitorIDs = in.MonitorIDs
	if !in.StartedAt.IsZero() {
		inc.StartedAt = in.StartedAt.UTC().Truncate(time.Second)
	}
	if err := validateIncident(inc, "", false); err != nil {
		return err
	}
	if err := s.store.UpdateIncident(r.Context(), inc); err != nil {
		return err
	}
	s.engine.IncidentsChanged()
	return s.getIncident(w, r)
}

func (s *Server) addIncidentUpdate(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in updateInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Body = strings.TrimSpace(in.Body)

	fields := map[string]string{}
	if !slices.Contains(store.IncidentStatuses, in.Status) {
		fields["status"] = "is not a valid status"
	}
	if in.Body == "" {
		fields["body"] = "is required"
	}
	if len(fields) > 0 {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: fields}
	}

	inc, err := s.store.AddIncidentUpdate(r.Context(), id, in.Status, in.Body)
	if err != nil {
		return err
	}
	s.engine.IncidentsChanged()

	byID, err := s.monitorIndex(r)
	if err != nil {
		return err
	}
	return created(w, toIncident(inc, byID, true))
}

func (s *Server) deleteIncident(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteIncident(r.Context(), id); err != nil {
		return err
	}
	s.engine.IncidentsChanged()
	return noContent(w)
}

func (s *Server) monitorIndex(r *http.Request) (map[int64]*monitor.Monitor, error) {
	list, err := s.store.Monitors(r.Context())
	if err != nil {
		return nil, err
	}
	return index(list), nil
}

func validateIncident(inc *store.Incident, body string, creating bool) error {
	fields := map[string]string{}
	if inc.Title == "" || len(inc.Title) > 200 {
		fields["title"] = "must be between 1 and 200 characters"
	}
	if !slices.Contains(store.Impacts, inc.Impact) {
		fields["impact"] = "is not a valid impact"
	}
	if creating && !slices.Contains(store.IncidentStatuses, inc.Status) {
		fields["status"] = "is not a valid status"
	}
	if creating && body == "" {
		fields["body"] = "is required"
	}
	if len(fields) == 0 {
		return nil
	}
	return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: fields}
}
