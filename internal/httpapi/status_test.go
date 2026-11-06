package httpapi

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/scheduler"
	"github.com/nethercoded/heron/internal/store"
)

func TestStatusCacheFollowsChanges(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	var m adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "API", Kind: monitor.KindTCP, Target: "db:5432", IntervalS: 60, TimeoutS: 5, Public: true}, http.StatusCreated, &m)

	var status statusDTO
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if status.Status != overallOperational {
		t.Fatalf("status = %s", status.Status)
	}

	mon, err := h.store.Monitor(t.Context(), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	h.engine.Record(t.Context(), scheduler.Beat{
		Monitor:  mon,
		Result:   monitor.Result{Status: monitor.StatusDown, Message: "refused", At: time.Now()},
		Previous: monitor.StatusPending,
		Current:  monitor.StatusDown,
	})
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if status.Status == overallOperational || status.Groups[0].Monitors[0].Status != monitor.StatusDown {
		t.Fatalf("status after outage = %s, monitor %s", status.Status, status.Groups[0].Monitors[0].Status)
	}

	page := status.Page
	page.Title = "Acme"
	h.expect("PUT", "/api/admin/page", page, http.StatusOK, nil)
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if status.Page.Title != "Acme" {
		t.Fatalf("title = %q", status.Page.Title)
	}
}

func TestStatusIsCompressed(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, h.srv.URL+"/api/status", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("encoding = %q", resp.Header.Get("Content-Encoding"))
	}
}

func TestCompressedAPI(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	cookies := h.client.Jar.Cookies(mustURL(t, h.srv.URL))
	get := func(path string) *http.Response {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, h.srv.URL+path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	if resp := get("/api/admin/monitors"); resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("monitors encoding = %q", resp.Header.Get("Content-Encoding"))
	}
	if resp := get("/api/admin/backup"); resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("backup was compressed on the fly")
	}
	if resp := get("/api/stream"); resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("the live stream was compressed")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestMonitorParents(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	in := monitorInput{Name: "Node", Kind: monitor.KindTCP, Target: "node:8080", IntervalS: 60, TimeoutS: 5}
	var node adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", in, http.StatusCreated, &node)

	in.Name, in.Settings.DependsOn = "Game", node.ID
	var game adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", in, http.StatusCreated, &game)

	in.Name, in.Settings.DependsOn = "Ghost", 9999
	h.expect("POST", "/api/admin/monitors", in, http.StatusUnprocessableEntity, nil)

	loop := monitorInput{Name: "Node", Slug: node.Slug, Kind: monitor.KindTCP, Target: "node:8080", IntervalS: 60, TimeoutS: 5}
	loop.Settings.DependsOn = game.ID
	var apiErr Error
	h.expect("PUT", "/api/admin/monitors/"+itoa(node.ID), loop, http.StatusUnprocessableEntity, &apiErr)
	if apiErr.Fields["depends_on"] == "" {
		t.Fatalf("fields = %v", apiErr.Fields)
	}
}

func TestEvidenceAndWarnings(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	var m adminMonitorDTO
	h.expect("POST", "/api/admin/monitors", monitorInput{Name: "Shop", Kind: monitor.KindTCP, Target: "shop:443", IntervalS: 60, TimeoutS: 5}, http.StatusCreated, &m)
	inc := &store.Incident{Title: "Shop is down", Impact: store.ImpactMajor, Status: store.IncidentInvestigating, MonitorIDs: []int64{m.ID}, StartedAt: time.Now()}
	if err := h.store.CreateIncident(t.Context(), inc, "down"); err != nil {
		t.Fatal(err)
	}
	e := &store.Evidence{MonitorID: m.ID, IncidentID: inc.ID, At: time.Now(), Report: []byte(`{"target":"shop:443","reason":"refused"}`)}
	if err := h.store.SaveEvidence(t.Context(), e, []byte("\x89PNG")); err != nil {
		t.Fatal(err)
	}
	h.store.SaveWarning(t.Context(), store.Warning{MonitorID: m.ID, Kind: "slow", Message: "slower", Since: time.Now()})

	var list []struct {
		ID     int64          `json:"id"`
		Report map[string]any `json:"report"`
		Image  bool           `json:"image"`
	}
	h.expect("GET", "/api/admin/incidents/"+itoa(inc.ID)+"/evidence", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Report["reason"] != "refused" || !list[0].Image {
		t.Fatalf("evidence = %+v", list)
	}
	h.expect("GET", "/api/admin/evidence/"+itoa(list[0].ID)+"/image", nil, http.StatusOK, nil)

	var warnings []warningDTO
	h.expect("GET", "/api/admin/warnings", nil, http.StatusOK, &warnings)
	if len(warnings) != 1 || warnings[0].MonitorName != "Shop" {
		t.Fatalf("warnings = %+v", warnings)
	}

	viewer := h.invite(store.RoleViewer, "watcher")
	viewer.expect("GET", "/api/admin/incidents/"+itoa(inc.ID)+"/evidence", nil, http.StatusForbidden, nil)
}
