package httpapi

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nethercoded/heron/internal/store"
	"github.com/nethercoded/heron/internal/update"
)

func TestUpdates(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.MarshalWrite(w, map[string]any{"tag_name": "v0.3.0", "html_url": "https://example.com/r", "body": "- faster", "published_at": "2026-11-01T10:00:00Z"})
	}))
	t.Cleanup(github.Close)

	u := update.New("0.2.0", "heron-test")
	u.API = github.URL
	u.OS = "linux"
	h := newHarness(t, func(o *Options) {
		o.Updates = &update.Service{Updater: u, DataDir: t.TempDir(), Log: slog.New(slog.DiscardHandler)}
	})
	h.signIn()

	var st updatesDTO
	h.expect("GET", "/api/admin/updates", nil, http.StatusOK, &st)
	if st.Latest != nil || st.Available || st.Current != "0.2.0" {
		t.Fatalf("before check = %+v", st)
	}
	h.expect("POST", "/api/admin/updates/check", nil, http.StatusOK, &st)
	if !st.Available || st.Latest == nil || st.Latest.Version != "0.3.0" || st.Latest.Notes != "- faster" || st.Blocked != "" {
		t.Fatalf("after check = %+v", st)
	}
	h.expect("PUT", "/api/admin/updates", map[string]bool{"auto": true}, http.StatusOK, &st)
	if !st.Auto {
		t.Fatal("auto update was not saved")
	}

	editor := h.invite(store.RoleAdmin, "editor")
	editor.expect("GET", "/api/admin/updates", nil, http.StatusForbidden, nil)
	editor.expect("POST", "/api/admin/updates/install", nil, http.StatusForbidden, nil)

	var log []auditDTO
	h.expect("GET", "/api/admin/audit?action=settings.auto_update", nil, http.StatusOK, &log)
	if len(log) != 1 || log[0].Target != "on" {
		t.Fatalf("audit = %+v", log)
	}
}
