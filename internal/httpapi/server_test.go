package httpapi

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nethercode/heron/internal/auth"
	"github.com/nethercode/heron/internal/engine"
	"github.com/nethercode/heron/internal/maintenance"
	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/notify"
	"github.com/nethercode/heron/internal/scheduler"
	"github.com/nethercode/heron/internal/store"
)

type fakeScheduler struct {
	mu     sync.Mutex
	synced map[int64]*monitor.Monitor
	pushed []monitor.Result
}

func (f *fakeScheduler) Sync(m *monitor.Monitor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m.Paused {
		delete(f.synced, m.ID)
		return
	}
	f.synced[m.ID] = m
}

func (f *fakeScheduler) Stop(id int64) {
	f.mu.Lock()
	delete(f.synced, id)
	f.mu.Unlock()
}

func (f *fakeScheduler) Trigger(id int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.synced[id]
	return ok
}

func (f *fakeScheduler) Push(id int64, res monitor.Result) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.synced[id]; !ok {
		return false
	}
	f.pushed = append(f.pushed, res)
	return true
}

type nopSender struct{}

func (nopSender) Send(context.Context, notify.Channel, notify.Message) error { return nil }
func (nopSender) Dispatch([]notify.Channel, notify.Message)                  {}

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	store  *store.Store
	engine *engine.Engine
	sched  *fakeScheduler
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	log := slog.New(slog.DiscardHandler)
	eng := engine.New(db, nopSender{}, maintenance.NewCalendar(time.UTC), log, "https://status.example.com")
	sched := &fakeScheduler{synced: map[int64]*monitor.Monitor{}}
	assets := fstest.MapFS{
		"index.html":               {Data: []byte("<!doctype html><title>heron</title>")},
		"_app/immutable/app.js":    {Data: []byte("console.log(1)")},
		"_app/immutable/app.js.br": {Data: []byte("brotli")},
	}
	api := New(db, eng, sched, nopSender{}, auth.NewSessions(db, false), log, Options{Version: "test", Assets: assets})

	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	return &harness{t: t, srv: srv, client: &http.Client{Jar: jar}, store: db, engine: eng, sched: sched}
}

func (h *harness) do(method, path string, body any, out any) *http.Response {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(h.t.Context(), method, h.srv.URL+path, reader)
	if err != nil {
		h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.UnmarshalRead(resp.Body, out); err != nil {
			h.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return resp
}

func (h *harness) expect(method, path string, body any, status int, out any) {
	h.t.Helper()
	if resp := h.do(method, path, body, out); resp.StatusCode != status {
		h.t.Fatalf("%s %s: status %d, want %d", method, path, resp.StatusCode, status)
	}
}

func (h *harness) signIn() {
	h.t.Helper()
	h.expect("POST", "/api/auth/setup", credentials{Username: "admin", Password: "correct horse battery"}, http.StatusCreated, nil)
}

func TestSetupAndLogin(t *testing.T) {
	h := newHarness(t)

	var state authStateDTO
	h.expect("GET", "/api/auth/state", nil, http.StatusOK, &state)
	if !state.SetupRequired || state.User != nil {
		t.Fatalf("state = %+v", state)
	}

	var apiErr Error
	h.expect("POST", "/api/auth/setup", credentials{Username: "a", Password: "short"}, http.StatusUnprocessableEntity, &apiErr)
	if apiErr.Fields["username"] == "" || apiErr.Fields["password"] == "" {
		t.Errorf("fields = %v", apiErr.Fields)
	}

	h.signIn()
	h.expect("POST", "/api/auth/setup", credentials{Username: "other", Password: "correct horse battery"}, http.StatusConflict, nil)

	h.expect("GET", "/api/auth/state", nil, http.StatusOK, &state)
	if state.SetupRequired || state.User == nil || state.User.Username != "admin" {
		t.Fatalf("state after setup = %+v", state)
	}

	h.expect("POST", "/api/auth/logout", nil, http.StatusNoContent, nil)
	h.expect("GET", "/api/admin/monitors", nil, http.StatusUnauthorized, nil)

	h.expect("POST", "/api/auth/login", credentials{Username: "admin", Password: "wrong password!"}, http.StatusUnauthorized, nil)
	h.expect("POST", "/api/auth/login", credentials{Username: "ADMIN", Password: "correct horse battery"}, http.StatusOK, nil)
	h.expect("GET", "/api/admin/monitors", nil, http.StatusOK, nil)
}

func TestLoginRateLimit(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	for range 10 {
		h.do("POST", "/api/auth/login", credentials{Username: "admin", Password: "nope"}, nil)
	}
	resp := h.do("POST", "/api/auth/login", credentials{Username: "admin", Password: "correct horse battery"}, nil)
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestMonitorFlow(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	var apiErr Error
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Broken", Kind: monitor.KindHTTP, Target: "nope"}, http.StatusUnprocessableEntity, &apiErr)
	if apiErr.Fields["target"] == "" {
		t.Errorf("fields = %v", apiErr.Fields)
	}

	var site adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{
		Name: "Website", Kind: monitor.KindHTTP, Target: "https://example.com", IntervalS: 60, Public: true, Group: "Frontend",
	}, http.StatusCreated, &site)
	if site.Slug != "website" || site.Settings.Method != "GET" || site.Status != monitor.StatusPending {
		t.Fatalf("created = %+v", site)
	}
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Website", Kind: monitor.KindTCP, Target: "db:5432"}, http.StatusUnprocessableEntity, &apiErr)
	if apiErr.Fields["slug"] == "" {
		t.Errorf("duplicate slug fields = %v", apiErr.Fields)
	}

	var hidden adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Internal DB", Kind: monitor.KindTCP, Target: "db:5432", Public: false}, http.StatusCreated, &hidden)

	m, _ := h.store.Monitor(t.Context(), site.ID)
	h.engine.Record(t.Context(), scheduler.Beat{
		Monitor:  m,
		Result:   monitor.Result{Status: monitor.StatusDown, Message: "502 Bad Gateway", At: time.Now()},
		Previous: monitor.StatusUp,
		Current:  monitor.StatusDown,
	})

	var status statusDTO
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if len(status.Groups) != 1 || len(status.Groups[0].Monitors) != 1 {
		t.Fatalf("groups = %+v", status.Groups)
	}
	pub := status.Groups[0].Monitors[0]
	if pub.Slug != "website" || pub.Status != monitor.StatusDown || len(pub.Days) != 90 {
		t.Errorf("public monitor = %+v", pub)
	}
	if status.Status != overallMajor {
		t.Errorf("overall = %s", status.Status)
	}
	if len(status.Incidents) != 1 || !status.Incidents[0].Automatic || status.Incidents[0].Monitors[0].Slug != "website" {
		t.Fatalf("incidents = %+v", status.Incidents)
	}

	h.expect("GET", "/api/monitors/internal-db", nil, http.StatusNotFound, nil)

	var detail detailDTO
	h.expect("GET", "/api/monitors/website", nil, http.StatusOK, &detail)
	if len(detail.Beats) != 1 || detail.Beats[0].Message != "" {
		t.Errorf("public beats leak messages: %+v", detail.Beats)
	}
	h.expect("GET", "/api/monitors/website/series?range=7d", nil, http.StatusOK, nil)
	h.expect("GET", "/api/monitors/website/series?range=1y", nil, http.StatusBadRequest, nil)

	h.expect("POST", "/api/admin/monitors/"+itoa(site.ID)+"/check", nil, http.StatusAccepted, nil)
	h.expect("POST", "/api/admin/monitors/"+itoa(site.ID)+"/pause", nil, http.StatusOK, nil)
	h.expect("POST", "/api/admin/monitors/"+itoa(site.ID)+"/check", nil, http.StatusConflict, nil)
	h.expect("DELETE", "/api/admin/monitors/"+itoa(hidden.ID), nil, http.StatusNoContent, nil)
	h.expect("GET", "/api/admin/monitors/"+itoa(hidden.ID), nil, http.StatusNotFound, nil)
}

func TestMaintenanceSuppressesIncidents(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	var db adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Database", Kind: monitor.KindTCP, Target: "db:5432", IntervalS: 60, Public: true}, http.StatusCreated, &db)

	h.expect("POST", "/api/admin/maintenance", maintenanceInput{Title: "Upgrade"}, http.StatusUnprocessableEntity, nil)

	var window maintenanceDTO
	h.expect("POST", "/api/admin/maintenance", maintenanceInput{
		Title:      "Postgres upgrade",
		MonitorIDs: []int64{db.ID},
		StartsAt:   time.Now().Add(-time.Minute),
		DurationS:  3600,
		Repeat:     "weekly",
		Enabled:    true,
	}, http.StatusCreated, &window)
	if !window.Active {
		t.Fatalf("window should be active: %+v", window)
	}

	m, _ := h.store.Monitor(t.Context(), db.ID)
	beat := func(status monitor.Status) {
		h.engine.Record(t.Context(), scheduler.Beat{
			Monitor:  m,
			Result:   monitor.Result{Status: status, Message: "connection refused", At: time.Now()},
			Previous: monitor.StatusUp,
			Current:  status,
		})
	}
	beat(monitor.StatusDown)

	var status statusDTO
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if len(status.Incidents) != 0 {
		t.Errorf("incident opened during maintenance: %+v", status.Incidents)
	}
	if got := status.Groups[0].Monitors[0].Status; got != monitor.StatusMaintenance {
		t.Errorf("monitor status = %s", got)
	}
	if status.Status != overallMaintenance || len(status.Maintenance) != 1 || !status.Maintenance[0].Active {
		t.Errorf("status = %s, maintenance = %+v", status.Status, status.Maintenance)
	}

	h.expect("DELETE", "/api/admin/maintenance/"+itoa(window.ID), nil, http.StatusNoContent, nil)
	beat(monitor.StatusDown)

	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if len(status.Incidents) != 1 {
		t.Errorf("an outage after maintenance must open an incident, got %d", len(status.Incidents))
	}
}

func TestPush(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	var backup adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Nightly backup", Kind: monitor.KindPush, IntervalS: 86400}, http.StatusCreated, &backup)
	if backup.PushToken == "" {
		t.Fatal("no push token")
	}

	anon := &http.Client{}
	resp, err := anon.Post(h.srv.URL+"/api/push/"+backup.PushToken+"?status=down&msg=disk%20full&ping=12.5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push status = %d", resp.StatusCode)
	}
	if len(h.sched.pushed) != 1 || h.sched.pushed[0].Status != monitor.StatusDown || h.sched.pushed[0].Message != "disk full" {
		t.Errorf("pushed = %+v", h.sched.pushed)
	}

	h.expect("GET", "/api/push/wrong-token", nil, http.StatusNotFound, nil)
	h.expect("GET", "/api/push/"+backup.PushToken+"?status=weird", nil, http.StatusBadRequest, nil)
}

func TestIncidentsAndChannels(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	var inc incidentDTO
	h.expect("POST", "/api/admin/incidents", incidentInput{Title: "Database maintenance", Impact: store.ImpactMaintenance, Body: "Upgrading to Postgres 19"}, http.StatusCreated, &inc)
	h.expect("POST", "/api/admin/incidents/"+itoa(inc.ID)+"/updates", updateInput{Status: store.IncidentResolved, Body: "Done"}, http.StatusCreated, &inc)
	if inc.ResolvedAt.IsZero() || len(inc.Updates) != 2 {
		t.Errorf("incident = %+v", inc)
	}

	var list []incidentDTO
	h.expect("GET", "/api/incidents", nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Errorf("public incidents = %d", len(list))
	}

	var ch channelDTO
	h.expect("POST", "/api/admin/channels", channelInput{Name: "Ops", Kind: notify.KindTelegram, Enabled: true, Config: notify.Config{BotToken: "t", ChatID: "1"}}, http.StatusCreated, &ch)
	h.expect("POST", "/api/admin/channels/"+itoa(ch.ID)+"/test", nil, http.StatusNoContent, nil)
	h.expect("POST", "/api/admin/channels", channelInput{Name: "Bad", Kind: notify.KindWebhook}, http.StatusUnprocessableEntity, nil)
}

func TestCrossOriginRejected(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	req, _ := http.NewRequestWithContext(t.Context(), "POST", h.srv.URL+"/api/admin/monitors", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestSPA(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{"/", "/admin/monitors", "/monitor/website"} {
		resp := h.do("GET", path, nil, nil)
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			t.Errorf("%s: %d %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}

	req, _ := http.NewRequestWithContext(t.Context(), "GET", h.srv.URL+"/_app/immutable/app.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "br" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("asset headers = %v", resp.Header)
	}

	if resp := h.do("GET", "/_app/missing.js", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset: %d", resp.StatusCode)
	}
	if resp := h.do("GET", "/api/nothing", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown api: %d", resp.StatusCode)
	}
}

func TestSummarize(t *testing.T) {
	up := &monitor.Monitor{State: monitor.State{Status: monitor.StatusUp}}
	down := &monitor.Monitor{State: monitor.State{Status: monitor.StatusDown}}
	slow := &monitor.Monitor{State: monitor.State{Status: monitor.StatusDegraded}}
	maintenance := &store.Incident{Impact: store.ImpactMaintenance}

	tests := []struct {
		monitors  []*monitor.Monitor
		incidents []*store.Incident
		want      overall
	}{
		{[]*monitor.Monitor{up, up}, nil, overallOperational},
		{[]*monitor.Monitor{up, slow}, nil, overallDegraded},
		{[]*monitor.Monitor{up, up, down}, nil, overallPartial},
		{[]*monitor.Monitor{up, down}, nil, overallMajor},
		{[]*monitor.Monitor{up}, []*store.Incident{maintenance}, overallMaintenance},
		{[]*monitor.Monitor{up}, []*store.Incident{{Impact: store.ImpactCritical}}, overallMajor},
		{nil, nil, overallOperational},
	}
	for _, tt := range tests {
		if got := summarize(tt.monitors, tt.incidents); got != tt.want {
			t.Errorf("summarize = %s, want %s", got, tt.want)
		}
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
