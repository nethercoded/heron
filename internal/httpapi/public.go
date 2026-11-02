package httpapi

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/engine"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

type overall string

const (
	overallOperational overall = "operational"
	overallDegraded    overall = "degraded"
	overallPartial     overall = "partial_outage"
	overallMajor       overall = "major_outage"
	overallMaintenance overall = "maintenance"
)

type statusDTO struct {
	Page        store.Page       `json:"page"`
	Status      overall          `json:"status"`
	Groups      []groupDTO       `json:"groups"`
	Incidents   []incidentDTO    `json:"incidents"`
	Maintenance []maintenanceDTO `json:"maintenance"`
	UpdatedAt   stamp            `json:"updated_at"`
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	page, err := s.store.Page(ctx)
	if err != nil {
		return err
	}
	all, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	public := slices.DeleteFunc(slices.Clone(all), func(m *monitor.Monitor) bool { return !m.Public })

	dtos, err := s.publicMonitors(r, public, page.HistoryDays)
	if err != nil {
		return err
	}

	var groups []groupDTO
	for i, dto := range dtos {
		if i == 0 || groups[len(groups)-1].Name != dto.Group {
			groups = append(groups, groupDTO{Name: dto.Group})
		}
		g := &groups[len(groups)-1]
		g.Monitors = append(g.Monitors, dto)
	}

	active, err := s.store.Incidents(ctx, store.IncidentFilter{OpenOnly: true})
	if err != nil {
		return err
	}
	byID := index(all)
	incidents := make([]incidentDTO, 0, len(active))
	for _, inc := range active {
		incidents = append(incidents, toIncident(inc, byID, false))
	}

	w.Header().Set("Cache-Control", "no-cache")
	return ok(w, statusDTO{
		Page:        page,
		Status:      summarize(public, active),
		Groups:      groups,
		Incidents:   incidents,
		Maintenance: s.publicMaintenance(byID, time.Now()),
		UpdatedAt:   stamp(time.Now()),
	})
}

func (s *Server) publicMonitors(r *http.Request, list []*monitor.Monitor, historyDays int) ([]publicMonitorDTO, error) {
	ctx := r.Context()
	now := time.Now()
	historyDays = max(historyDays, 1)

	periods := []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour, time.Duration(historyDays) * 24 * time.Hour}
	uptimes := make([]map[int64]store.Uptime, len(periods))
	for i, p := range periods {
		u, err := s.store.Uptimes(ctx, now.Add(-p))
		if err != nil {
			return nil, err
		}
		uptimes[i] = u
	}

	ids := make([]int64, 0, len(list))
	for _, m := range list {
		ids = append(ids, m.ID)
	}
	from := now.In(s.opts.Location).AddDate(0, 0, -(historyDays - 1))
	days, err := s.store.Days(ctx, ids, from, s.opts.Location)
	if err != nil {
		return nil, err
	}

	firstSeen := map[string]int{}
	for i, m := range list {
		if _, seen := firstSeen[m.Group]; !seen {
			firstSeen[m.Group] = i
		}
	}
	slices.SortStableFunc(list, func(a, b *monitor.Monitor) int {
		return cmp.Compare(firstSeen[a.Group], firstSeen[b.Group])
	})

	out := make([]publicMonitorDTO, 0, len(list))
	for _, m := range list {
		out = append(out, publicMonitorDTO{
			Slug:        m.Slug,
			Name:        m.Name,
			Description: m.Description,
			Group:       m.Group,
			Kind:        m.Kind,
			Status:      m.State.Status,
			Since:       stamp(m.State.Since),
			CheckedAt:   stamp(m.State.CheckedAt),
			LatencyMS:   m.State.Latency.Milliseconds(),
			Uptime: uptimeDTO{
				Day:     uptimes[0][m.ID].Ratio(),
				Week:    uptimes[1][m.ID].Ratio(),
				Month:   uptimes[2][m.ID].Ratio(),
				History: uptimes[3][m.ID].Ratio(),
			},
			Days: fillDays(days[m.ID], from, historyDays, s.opts.Location),
		})
	}
	return out, nil
}

func fillDays(data []store.Day, from time.Time, n int, loc *time.Location) []dayDTO {
	byDate := make(map[string]store.Day, len(data))
	for _, d := range data {
		byDate[d.Date.In(loc).Format(time.DateOnly)] = d
	}

	out := make([]dayDTO, n)
	for i := range n {
		date := from.AddDate(0, 0, i).Format(time.DateOnly)
		dto := dayDTO{Date: date}
		if d, ok := byDate[date]; ok && d.Uptime.Total > 0 {
			ratio := d.Uptime.Ratio()
			dto.Uptime = &ratio
			dto.Checks = d.Uptime.Total
			dto.Down = d.Uptime.Down
			dto.Degraded = d.Degraded
		}
		out[i] = dto
	}
	return out
}

func summarize(monitors []*monitor.Monitor, active []*store.Incident) overall {
	var down, degraded int
	var critical, maintenance, other bool
	for _, m := range monitors {
		switch m.State.Status {
		case monitor.StatusDown:
			down++
		case monitor.StatusDegraded:
			degraded++
		case monitor.StatusMaintenance:
			maintenance = true
		}
	}

	for _, inc := range active {
		switch inc.Impact {
		case store.ImpactCritical:
			critical = true
		case store.ImpactMaintenance:
			maintenance = true
		default:
			other = true
		}
	}

	switch {
	case critical || (down > 0 && down*2 >= len(monitors)):
		return overallMajor
	case down > 0:
		return overallPartial
	case maintenance && !other:
		return overallMaintenance
	case degraded > 0 || other:
		return overallDegraded
	}
	return overallOperational
}

type detailDTO struct {
	Monitor   publicMonitorDTO `json:"monitor"`
	Beats     []beatDTO        `json:"beats"`
	Incidents []incidentDTO    `json:"incidents"`
}

func (s *Server) monitorDetail(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	m, err := s.publicMonitor(r)
	if err != nil {
		return err
	}
	page, err := s.store.Page(ctx)
	if err != nil {
		return err
	}
	dtos, err := s.publicMonitors(r, []*monitor.Monitor{m}, page.HistoryDays)
	if err != nil {
		return err
	}
	beats, err := s.store.Beats(ctx, m.ID, 60)
	if err != nil {
		return err
	}
	slices.Reverse(beats)

	history, err := s.store.Incidents(ctx, store.IncidentFilter{Since: time.Now().AddDate(0, 0, -page.HistoryDays)})
	if err != nil {
		return err
	}
	names := map[int64]*monitor.Monitor{m.ID: m}
	var incidents []incidentDTO
	for _, inc := range history {
		if slices.Contains(inc.MonitorIDs, m.ID) {
			incidents = append(incidents, toIncident(inc, names, false))
		}
	}

	return ok(w, detailDTO{Monitor: dtos[0], Beats: toBeats(beats, false), Incidents: incidents})
}

var seriesRanges = map[string]struct {
	span, step time.Duration
}{
	"24h": {24 * time.Hour, 10 * time.Minute},
	"7d":  {7 * 24 * time.Hour, time.Hour},
	"30d": {30 * 24 * time.Hour, 6 * time.Hour},
	"90d": {90 * 24 * time.Hour, 24 * time.Hour},
}

func (s *Server) monitorSeries(w http.ResponseWriter, r *http.Request) error {
	m, err := s.publicMonitor(r)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	return s.series(w, r, m)
}

func (s *Server) adminSeries(w http.ResponseWriter, r *http.Request) error {
	m, err := s.loadMonitor(r)
	if err != nil {
		return err
	}
	return s.series(w, r, m)
}

func (s *Server) monitorCalendar(w http.ResponseWriter, r *http.Request) error {
	m, err := s.publicMonitor(r)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	return s.calendar(w, r, m)
}

func (s *Server) adminCalendar(w http.ResponseWriter, r *http.Request) error {
	m, err := s.loadMonitor(r)
	if err != nil {
		return err
	}
	return s.calendar(w, r, m)
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request, m *monitor.Monitor) error {
	const span = 371
	from := time.Now().In(s.opts.Location).AddDate(0, 0, -(span - 1))
	days, err := s.store.Days(r.Context(), []int64{m.ID}, from, s.opts.Location)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{
		"since": m.CreatedAt.In(s.opts.Location).Format(time.DateOnly),
		"days":  fillDays(days[m.ID], from, span, s.opts.Location),
	})
}

func (s *Server) series(w http.ResponseWriter, r *http.Request, m *monitor.Monitor) error {
	rng, found := seriesRanges[cmp.Or(r.URL.Query().Get("range"), "24h")]
	if !found {
		return badRequest("range must be one of 24h, 7d, 30d, 90d")
	}

	points, err := s.store.Series(r.Context(), m.ID, time.Now().Add(-rng.span), rng.step)
	if err != nil {
		return err
	}
	out := make([]pointDTO, 0, len(points))
	for _, p := range points {
		out = append(out, pointDTO{
			At:     stamp(p.At),
			AvgMS:  float64(p.Avg.Microseconds()) / 1000,
			MaxMS:  p.Max.Milliseconds(),
			Checks: p.Checks,
			Down:   p.Down,
		})
	}
	return ok(w, map[string]any{"step_s": int64(rng.step.Seconds()), "points": out})
}

func (s *Server) publicMonitor(r *http.Request) (*monitor.Monitor, error) {
	m, err := s.store.MonitorBySlug(r.Context(), r.PathValue("slug"))
	if err != nil || !m.Public {
		return nil, notFound("monitor")
	}
	return m, nil
}

func (s *Server) incidents(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 366 {
			return badRequest("days must be between 1 and 366")
		}
		days = n
	}

	list, err := s.store.Incidents(ctx, store.IncidentFilter{Since: time.Now().AddDate(0, 0, -days), Limit: 200})
	if err != nil {
		return err
	}
	monitors, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	byID := index(monitors)

	out := make([]incidentDTO, 0, len(list))
	for _, inc := range list {
		if visible(inc, byID) {
			out = append(out, toIncident(inc, byID, false))
		}
	}
	return ok(w, out)
}

func (s *Server) incident(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	inc, err := s.store.Incident(r.Context(), id)
	if err != nil {
		return notFound("incident")
	}
	monitors, err := s.store.Monitors(r.Context())
	if err != nil {
		return err
	}
	byID := index(monitors)
	if !visible(inc, byID) {
		return notFound("incident")
	}
	return ok(w, toIncident(inc, byID, false))
}

func visible(inc *store.Incident, monitors map[int64]*monitor.Monitor) bool {
	if !inc.Automatic {
		return true
	}
	return slices.ContainsFunc(inc.MonitorIDs, func(id int64) bool {
		m, ok := monitors[id]
		return ok && m.Public
	})
}

func (s *Server) push(w http.ResponseWriter, r *http.Request) error {
	m, err := s.store.MonitorByPushToken(r.Context(), r.PathValue("token"))
	if err != nil {
		return notFound("monitor")
	}
	if m.Paused {
		return errorf(http.StatusConflict, "paused", "monitor is paused")
	}

	q := r.URL.Query()
	msg := strings.TrimSpace(q.Get("msg"))
	if len(msg) > 200 {
		msg = msg[:200]
	}
	var latency time.Duration
	if v, err := strconv.ParseFloat(q.Get("ping"), 64); err == nil && v >= 0 {
		latency = time.Duration(v * float64(time.Millisecond))
	}

	res := monitor.Up(latency, "%s", cmp.Or(msg, "heartbeat received"))
	switch q.Get("status") {
	case "", "up":
	case "down":
		res = monitor.Down(latency, "%s", cmp.Or(msg, "reported down"))
	case "degraded":
		res = monitor.Degraded(latency, "%s", cmp.Or(msg, "reported degraded"))
	default:
		return badRequest("status must be up, down or degraded")
	}
	res.At = time.Now()

	if !s.scheduler.Push(m.ID, res) {
		return errorf(http.StatusServiceUnavailable, "busy", "heartbeat could not be accepted, try again")
	}
	return ok(w, map[string]bool{"ok": true})
}

type streamEvent struct {
	Slug      string         `json:"slug,omitzero"`
	Status    monitor.Status `json:"status,omitzero"`
	LatencyMS int64          `json:"latency_ms,omitzero"`
	At        stamp          `json:"at,omitzero"`
	ID        int64          `json:"id,omitzero"`
	Message   string         `json:"message,omitzero"`
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		return errorf(http.StatusNotImplemented, "unsupported", "streaming is not supported")
	}

	admin := false
	if r.URL.Query().Has("admin") {
		if u, err := s.sessions.User(r.Context(), w, r); err == nil && u != nil {
			admin = true
		}
	}

	events, cancel := s.engine.Events().Subscribe()
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 5000\n\n")
	if err := rc.Flush(); err != nil {
		return nil
	}

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n")
		case ev, open := <-events:
			if !open {
				return nil
			}
			if ev.Type != engine.EventIncident && !ev.Public && !admin {
				continue
			}
			payload := streamEvent{Slug: ev.Slug, Status: ev.Status, LatencyMS: ev.Latency.Milliseconds(), At: stamp(ev.At)}
			if admin {
				payload.ID = ev.MonitorID
				payload.Message = ev.Message
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return nil
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}

func index(list []*monitor.Monitor) map[int64]*monitor.Monitor {
	out := make(map[int64]*monitor.Monitor, len(list))
	for _, m := range list {
		out[m.ID] = m
	}
	return out
}
