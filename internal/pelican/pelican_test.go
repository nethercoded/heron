package pelican

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

type fakePelican struct {
	mu      sync.Mutex
	servers string
	states  string
	node    string
	wings   *httptest.Server
	panel   *httptest.Server
}

func newFake(t *testing.T) *fakePelican {
	f := &fakePelican{
		node:    "DE-1",
		servers: `{"uuid":"aaa","name":"Survival","node":1,"suspended":false},{"uuid":"bbb","name":"Creative","node":1,"suspended":false},{"uuid":"ccc","name":"Old map","node":1,"suspended":true}`,
		states:  `[{"state":"running","is_suspended":false,"configuration":{"uuid":"aaa"},"utilization":{"memory_bytes":1073741824,"memory_limit_bytes":4294967296,"cpu_absolute":37}},{"state":"offline","is_suspended":false,"configuration":{"uuid":"bbb"}}]`,
	}
	f.wings = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node-secret" {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api/system":
			io.WriteString(w, `{"version":"1.0.0"}`)
		case "/api/servers":
			io.WriteString(w, f.states)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.wings.Close)
	wingsURL, _ := url.Parse(f.wings.URL)

	f.panel = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer papp_key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api/application/nodes":
			io.WriteString(w, `{"object":"list","data":[{"object":"node","attributes":{"id":1,"name":"`+f.node+`","fqdn":"`+wingsURL.Hostname()+`","scheme":"http","daemon_listen":8080,"daemon_connect":`+wingsURL.Port()+`,"maintenance_mode":false}}],"meta":{"pagination":{"current_page":1,"total_pages":1}}}`)
		case "/api/application/servers":
			var items []string
			for s := range strings.SplitSeq(f.servers, "},") {
				if s = strings.TrimSuffix(s, "}"); s != "" {
					items = append(items, `{"object":"server","attributes":`+s+`}}`)
				}
			}
			io.WriteString(w, `{"object":"list","data":[`+strings.Join(items, ",")+`],"meta":{"pagination":{"current_page":1,"total_pages":1}}}`)
		case "/api/application/nodes/1/configuration":
			io.WriteString(w, `{"uuid":"n1","token_id":"abc","token":"node-secret","api":{"host":"0.0.0.0","port":8080}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.panel.Close)
	return f
}

func TestSyncAndChecks(t *testing.T) {
	f := newFake(t)
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.SavePelican(t.Context(), store.Pelican{Enabled: true, URL: f.panel.URL, Key: "papp_key", Public: true}); err != nil {
		t.Fatal(err)
	}

	var changes []string
	s := New(db, slog.New(slog.DiscardHandler))
	s.OnChange = func(m *monitor.Monitor, removed bool) {
		changes = append(changes, m.Slug)
	}
	if err := s.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Nodes != 1 || st.Servers != 3 || st.Err != nil {
		t.Fatalf("status = %+v", st)
	}

	list, _ := db.Monitors(t.Context())
	bySlug := map[string]*monitor.Monitor{}
	for _, m := range list {
		bySlug[m.Slug] = m
	}
	node, survival, creative, old := bySlug["node-de-1"], bySlug["survival"], bySlug["creative"], bySlug["old-map"]
	if node == nil || survival == nil || creative == nil || old == nil {
		t.Fatalf("monitors = %v", changes)
	}
	if survival.Settings.DependsOn != node.ID || survival.Group != "DE-1" || !survival.Public || !old.Paused {
		t.Fatalf("survival = %+v, old paused = %v", survival, old.Paused)
	}

	if res := s.Check(t.Context(), node); res.Status != monitor.StatusUp {
		t.Errorf("node = %+v", res)
	}
	if res := s.Check(t.Context(), survival); res.Status != monitor.StatusUp || !strings.Contains(res.Message, "1024 of 4096 MB") {
		t.Errorf("survival = %+v", res)
	}
	if res := s.Check(t.Context(), creative); res.Status != monitor.StatusDown {
		t.Errorf("creative = %+v", res)
	}

	f.mu.Lock()
	f.servers = `{"uuid":"aaa","name":"Survival","node":1,"suspended":false}`
	f.mu.Unlock()
	changes = nil
	if err := s.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("removed = %v, want creative and old-map", changes)
	}
	if again, _ := db.Monitors(t.Context()); len(again) != 2 {
		t.Fatalf("monitors left = %d", len(again))
	}

	db.SavePelican(t.Context(), store.Pelican{Enabled: true, URL: f.panel.URL, Key: "wrong"})
	if err := s.Sync(t.Context()); err == nil || !strings.Contains(err.Error(), "refused the API key") {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRespectsLocalChanges(t *testing.T) {
	f := newFake(t)
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SavePelican(t.Context(), store.Pelican{Enabled: true, URL: f.panel.URL, Key: "papp_key"})

	changes := 0
	s := New(db, slog.New(slog.DiscardHandler))
	s.OnChange = func(*monitor.Monitor, bool) { changes++ }
	sync := func() {
		t.Helper()
		if err := s.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	get := func(slug string) *monitor.Monitor {
		t.Helper()
		m, err := db.MonitorBySlug(t.Context(), slug)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	sync()

	survival := get("survival")
	if _, err := db.SetPaused(t.Context(), survival.ID, true); err != nil {
		t.Fatal(err)
	}
	creative := get("creative")
	creative.Group = "Lobby"
	if err := db.UpdateMonitor(t.Context(), creative); err != nil {
		t.Fatal(err)
	}

	changes = 0
	sync()
	sync()
	if changes != 0 {
		t.Fatalf("syncs with nothing new in the panel touched %d monitors", changes)
	}
	if !get("survival").Paused || get("creative").Group != "Lobby" {
		t.Fatal("a sync undid a change made in heron")
	}

	f.mu.Lock()
	f.node = "DE-2"
	f.servers = `{"uuid":"aaa","name":"Survival","node":1,"suspended":false},{"uuid":"bbb","name":"Creative","node":1,"suspended":true},{"uuid":"ccc","name":"Old map","node":1,"suspended":false}`
	f.mu.Unlock()
	sync()
	if !get("creative").Paused || get("old-map").Paused {
		t.Fatalf("suspension: creative paused %v, old map paused %v", get("creative").Paused, get("old-map").Paused)
	}
	if get("old-map").Group != "DE-2" || get("creative").Group != "Lobby" || !get("survival").Paused {
		t.Fatalf("groups: old map %q, creative %q", get("old-map").Group, get("creative").Group)
	}
}
