package httpapi

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	token, err := s.store.MetricsToken(ctx)
	if err != nil {
		return err
	}
	if token == "" {
		return notFound("endpoint")
	}
	given, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="heron"`)
		return errorf(http.StatusUnauthorized, "unauthorized", "bearer token required")
	}

	list, err := s.store.Monitors(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	day, err := s.store.Uptimes(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return err
	}
	month, err := s.store.Uptimes(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		return err
	}
	incidents, err := s.store.Incidents(ctx, store.IncidentFilter{OpenOnly: true})
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	out := bufio.NewWriter(w)

	family(out, "heron_build_info", "gauge", "Build information.")
	fmt.Fprintf(out, "heron_build_info{version=%s} 1\n", quote(s.opts.Version))

	family(out, "heron_monitor_up", "gauge", "1 when the last check passed, 0 when the monitor is down.")
	for _, m := range list {
		switch m.State.Status {
		case monitor.StatusUp, monitor.StatusDegraded:
			fmt.Fprintf(out, "heron_monitor_up{%s} 1\n", labels(m))
		case monitor.StatusDown:
			fmt.Fprintf(out, "heron_monitor_up{%s} 0\n", labels(m))
		}
	}

	family(out, "heron_monitor_status", "gauge", "Current status of the monitor, one series per status.")
	for _, m := range list {
		for _, status := range []monitor.Status{monitor.StatusUp, monitor.StatusDegraded, monitor.StatusDown, monitor.StatusPending, monitor.StatusPaused, monitor.StatusMaintenance} {
			value := 0
			if m.State.Status == status {
				value = 1
			}
			fmt.Fprintf(out, "heron_monitor_status{%s,status=%s} %d\n", labels(m), quote(string(status)), value)
		}
	}

	family(out, "heron_monitor_latency_seconds", "gauge", "Response time of the last check.")
	for _, m := range list {
		if !m.State.CheckedAt.IsZero() {
			fmt.Fprintf(out, "heron_monitor_latency_seconds{%s} %s\n", labels(m), float(m.State.Latency.Seconds()))
		}
	}

	family(out, "heron_monitor_last_check_timestamp_seconds", "gauge", "Unix time of the last check.")
	for _, m := range list {
		if !m.State.CheckedAt.IsZero() {
			fmt.Fprintf(out, "heron_monitor_last_check_timestamp_seconds{%s} %d\n", labels(m), m.State.CheckedAt.Unix())
		}
	}

	family(out, "heron_monitor_uptime_ratio", "gauge", "Share of successful checks over the window.")
	for _, m := range list {
		for window, uptimes := range map[string]map[int64]store.Uptime{"24h": day, "30d": month} {
			if u, ok := uptimes[m.ID]; ok && u.Total > 0 {
				fmt.Fprintf(out, "heron_monitor_uptime_ratio{%s,window=%s} %s\n", labels(m), quote(window), float(u.Ratio()))
			}
		}
	}

	family(out, "heron_incidents_open", "gauge", "Incidents that are not resolved yet.")
	fmt.Fprintf(out, "heron_incidents_open %d\n", len(incidents))

	return out.Flush()
}

func (s *Server) getMetricsToken(w http.ResponseWriter, r *http.Request) error {
	token, err := s.store.MetricsToken(r.Context())
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"token": token})
}

func (s *Server) rotateMetricsToken(w http.ResponseWriter, r *http.Request) error {
	token := rand.Text()
	if err := s.store.SetMetricsToken(r.Context(), token); err != nil {
		return err
	}
	return ok(w, map[string]string{"token": token})
}

func (s *Server) disableMetrics(w http.ResponseWriter, r *http.Request) error {
	if err := s.store.SetMetricsToken(r.Context(), ""); err != nil {
		return err
	}
	return noContent(w)
}

func family(w *bufio.Writer, name, kind, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
}

func labels(m *monitor.Monitor) string {
	return fmt.Sprintf("monitor=%s,name=%s,kind=%s,group=%s", quote(m.Slug), quote(m.Name), quote(string(m.Kind)), quote(m.Group))
}

var labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func quote(v string) string {
	return `"` + labelEscaper.Replace(v) + `"`
}

func float(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
