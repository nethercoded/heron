package blackbox

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"log/slog"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

const perIncident = 3

func (r *Recorder) Keeper(ctx context.Context, db *store.Store, log *slog.Logger) func(*monitor.Monitor, int64, monitor.Result) {
	return func(m *monitor.Monitor, incidentID int64, res monitor.Result) {
		if incidentID != 0 {
			if n, err := db.CountEvidence(ctx, incidentID); err != nil || n >= perIncident {
				return
			}
		}
		rep := r.Record(ctx, m, res)
		raw, err := json.Marshal(rep, jsontext.AllowInvalidUTF8(true))
		if err != nil {
			log.Error("encode black box report", "monitor", m.Slug, "err", err)
			return
		}
		e := &store.Evidence{MonitorID: m.ID, IncidentID: incidentID, At: rep.At, Report: raw}
		if err := db.SaveEvidence(ctx, e, rep.Image); err != nil {
			log.Error("save black box report", "monitor", m.Slug, "err", err)
			return
		}
		log.Info("saved what the monitor looked like when it went down", "monitor", m.Slug, "screenshot", rep.Image != nil)
	}
}
