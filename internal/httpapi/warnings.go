package httpapi

import "net/http"

type warningDTO struct {
	MonitorID   int64  `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
	Since       stamp  `json:"since"`
}

func (s *Server) listWarnings(w http.ResponseWriter, r *http.Request) error {
	list, err := s.store.Warnings(r.Context())
	if err != nil {
		return err
	}
	monitors, err := s.store.Monitors(r.Context())
	if err != nil {
		return err
	}
	byID := index(monitors)
	out := make([]warningDTO, 0, len(list))
	for _, wr := range list {
		m := byID[wr.MonitorID]
		if m == nil {
			continue
		}
		out = append(out, warningDTO{MonitorID: wr.MonitorID, MonitorName: m.Name, Kind: wr.Kind, Message: wr.Message, Since: stamp(wr.Since)})
	}
	return ok(w, out)
}
