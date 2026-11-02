package httpapi

import (
	"cmp"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/store"
)

const (
	badgeGreen  = "#2ea44f"
	badgeLime   = "#7cb342"
	badgeYellow = "#dfb317"
	badgeOrange = "#fe7d37"
	badgeRed    = "#e05d44"
	badgeBlue   = "#3b82f6"
	badgeGrey   = "#9f9f9f"
)

var badgePeriods = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
	"90d": 90 * 24 * time.Hour,
}

func (s *Server) overallBadge(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	all, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	public := slices.DeleteFunc(all, func(m *monitor.Monitor) bool { return !m.Public })
	active, err := s.store.Incidents(ctx, store.IncidentFilter{OpenOnly: true})
	if err != nil {
		return err
	}

	value, color := "operational", badgeGreen
	switch summarize(public, active) {
	case overallDegraded:
		value, color = "degraded", badgeYellow
	case overallPartial:
		value, color = "partial outage", badgeOrange
	case overallMajor:
		value, color = "major outage", badgeRed
	case overallMaintenance:
		value, color = "maintenance", badgeBlue
	}
	return writeBadge(w, r, "status", value, color)
}

func (s *Server) statusBadge(w http.ResponseWriter, r *http.Request) error {
	m, err := s.publicMonitor(r)
	if err != nil {
		return writeBadge(w, r, "status", "unknown", badgeGrey)
	}
	color := map[monitor.Status]string{
		monitor.StatusUp:          badgeGreen,
		monitor.StatusDegraded:    badgeYellow,
		monitor.StatusDown:        badgeRed,
		monitor.StatusMaintenance: badgeBlue,
	}[m.State.Status]
	value := string(m.State.Status)
	if m.State.Status == monitor.StatusUp {
		value = "up"
	}
	return writeBadge(w, r, m.Name, value, cmp.Or(color, badgeGrey))
}

func (s *Server) uptimeBadge(w http.ResponseWriter, r *http.Request) error {
	m, err := s.publicMonitor(r)
	if err != nil {
		return writeBadge(w, r, "uptime", "unknown", badgeGrey)
	}
	period := cmp.Or(r.URL.Query().Get("period"), "30d")
	span, found := badgePeriods[period]
	if !found {
		return badRequest("period must be one of 24h, 7d, 30d, 90d")
	}
	uptimes, err := s.store.Uptimes(r.Context(), time.Now().Add(-span))
	if err != nil {
		return err
	}
	u, measured := uptimes[m.ID]
	if !measured {
		return writeBadge(w, r, "uptime "+period, "no data", badgeGrey)
	}
	ratio := u.Ratio()
	color := badgeRed
	switch {
	case ratio >= 0.999:
		color = badgeGreen
	case ratio >= 0.99:
		color = badgeLime
	case ratio >= 0.97:
		color = badgeYellow
	case ratio >= 0.95:
		color = badgeOrange
	}
	return writeBadge(w, r, "uptime "+period, formatRatio(ratio), color)
}

func (s *Server) responseBadge(w http.ResponseWriter, r *http.Request) error {
	m, err := s.publicMonitor(r)
	if err != nil {
		return writeBadge(w, r, "response", "unknown", badgeGrey)
	}
	if m.State.Latency <= 0 || !m.State.Status.Healthy() {
		return writeBadge(w, r, "response", "n/a", badgeGrey)
	}
	ms := m.State.Latency.Milliseconds()
	color := badgeRed
	switch {
	case ms < 300:
		color = badgeGreen
	case ms < 800:
		color = badgeYellow
	case ms < 2000:
		color = badgeOrange
	}
	return writeBadge(w, r, "response", fmt.Sprintf("%d ms", ms), color)
}

func formatRatio(ratio float64) string {
	pct := ratio * 100
	if pct >= 100 {
		return "100%"
	}
	return fmt.Sprintf("%.2f%%", math.Floor(pct*100)/100)
}

var badgeTemplate = template.Must(template.New("badge").Parse(
	`<svg xmlns="http://www.w3.org/2000/svg" width="{{.Width}}" height="20" role="img" aria-label="{{.Label}}: {{.Value}}">` +
		`<title>{{.Label}}: {{.Value}}</title>` +
		`<linearGradient id="s" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>` +
		`<clipPath id="r"><rect width="{{.Width}}" height="20" rx="3" fill="#fff"/></clipPath>` +
		`<g clip-path="url(#r)"><rect width="{{.LabelWidth}}" height="20" fill="#555"/><rect x="{{.LabelWidth}}" width="{{.ValueWidth}}" height="20" fill="{{.Color}}"/><rect width="{{.Width}}" height="20" fill="url(#s)"/></g>` +
		`<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">` +
		`<text x="{{.LabelX}}" y="15" fill="#010101" fill-opacity=".3">{{.Label}}</text><text x="{{.LabelX}}" y="14">{{.Label}}</text>` +
		`<text x="{{.ValueX}}" y="15" fill="#010101" fill-opacity=".3">{{.Value}}</text><text x="{{.ValueX}}" y="14">{{.Value}}</text>` +
		`</g></svg>`))

type badge struct {
	Label, Value, Color           string
	Width, LabelWidth, ValueWidth int
	LabelX, ValueX                float64
}

func writeBadge(w http.ResponseWriter, r *http.Request, label, value, color string) error {
	label = cmp.Or(strings.TrimSpace(r.URL.Query().Get("label")), label)
	if runes := []rune(label); len(runes) > 40 {
		label = string(runes[:40])
	}

	b := badge{Label: label, Value: value, Color: color, LabelWidth: textWidth(label) + 10, ValueWidth: textWidth(value) + 10}
	b.Width = b.LabelWidth + b.ValueWidth
	b.LabelX = float64(b.LabelWidth) / 2
	b.ValueX = float64(b.LabelWidth) + float64(b.ValueWidth)/2

	h := w.Header()
	h.Set("Content-Type", "image/svg+xml; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=60, s-maxage=60")
	h.Set("Access-Control-Allow-Origin", "*")
	return badgeTemplate.Execute(w, b)
}

func textWidth(s string) int {
	var width float64
	for _, r := range s {
		switch {
		case strings.ContainsRune("ijlI.,:;|!'1 ", r):
			width += 3.6
		case strings.ContainsRune("frt()[]-", r):
			width += 4.6
		case strings.ContainsRune("mwMW%@", r):
			width += 10.2
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			width += 7.4
		default:
			width += 6.6
		}
	}
	return int(math.Ceil(width))
}
