package httpapi

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

func (h *harness) upload(path, name string, content []byte, out any) *http.Response {
	h.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", name)
	part.Write(content)
	form.Close()
	req, _ := http.NewRequestWithContext(h.t.Context(), http.MethodPost, h.srv.URL+path, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		json.UnmarshalRead(resp.Body, out)
	}
	return resp
}

func TestKumaImport(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	export := []byte(`{"notificationList":[],"monitorList":[
		{"id":1,"name":"Site","type":"http","url":"https://example.com","interval":60,"active":true},
		{"id":2,"name":"Queue","type":"mqtt","active":true}]}`)

	var plan importDTO
	if resp := h.upload("/api/admin/import/kuma/preview", "backup.json", export, &plan); resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status %d", resp.StatusCode)
	}
	if len(plan.Monitors) != 1 || len(plan.Skipped) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if list, _ := h.store.Monitors(t.Context()); len(list) != 0 {
		t.Fatal("preview created monitors")
	}

	if resp := h.upload("/api/admin/import/kuma", "backup.json", export, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("import status %d", resp.StatusCode)
	}
	list, _ := h.store.Monitors(t.Context())
	if len(list) != 1 || list[0].Slug != "site" || h.sched.synced[list[0].ID] == nil {
		t.Fatalf("monitors = %+v", list)
	}

	if resp := h.upload("/api/admin/import/kuma/preview", "notes.txt", []byte("hello"), nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("junk status %d", resp.StatusCode)
	}
}

func TestBackupDownload(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	resp := h.do("GET", "/api/admin/backup", nil, nil)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/vnd.sqlite3" {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	editor := h.invite(store.RoleAdmin, "editor")
	editor.expect("GET", "/api/admin/backup", nil, http.StatusForbidden, nil)
}

func TestBackupBody(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, h.srv.URL+"/api/admin/backup", nil)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	head := make([]byte, 15)
	io.ReadFull(resp.Body, head)
	if string(head) != "SQLite format 3" {
		t.Fatalf("body starts with %q", head)
	}
}

func TestPageBlocks(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	var page store.Page
	h.expect("GET", "/api/admin/page", nil, http.StatusOK, &page)
	if len(page.Blocks) != len(store.DefaultBlocks()) {
		t.Fatalf("blocks = %+v", page.Blocks)
	}
	page.Blocks = []store.Block{{ID: "note", Type: store.BlockNotice, Body: "Hi"}, {ID: "rest", Type: store.BlockMonitors}}
	h.expect("PUT", "/api/admin/page", page, http.StatusOK, &page)

	var status struct {
		Page store.Page `json:"page"`
	}
	h.expect("GET", "/api/status", nil, http.StatusOK, &status)
	if len(status.Page.Blocks) != 2 || status.Page.Blocks[0].Body != "Hi" {
		t.Fatalf("public blocks = %+v", status.Page.Blocks)
	}

	page.Blocks = append(page.Blocks, store.Block{ID: "note", Type: "chart"})
	var bad Error
	h.expect("PUT", "/api/admin/page", page, http.StatusUnprocessableEntity, &bad)
	if bad.Fields["blocks.2.id"] == "" || bad.Fields["blocks.2.type"] == "" {
		t.Fatalf("fields = %v", bad.Fields)
	}
}

func TestMonitorTest(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(site.Close)
	h := newHarness(t, func(o *Options) { o.Checker = monitor.NewRegistry("heron-test", false) })
	h.signIn()

	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	h.expect("POST", "/api/admin/monitors/test", map[string]any{"kind": "http", "target": site.URL, "interval_s": 60, "timeout_s": 5}, http.StatusOK, &res)
	if res.Status != "up" {
		t.Fatalf("result = %+v", res)
	}
	h.expect("POST", "/api/admin/monitors/test", map[string]any{"kind": "http", "target": "nope", "interval_s": 60, "timeout_s": 5}, http.StatusUnprocessableEntity, nil)
	h.expect("POST", "/api/admin/monitors/test", map[string]any{"kind": "push", "interval_s": 60}, http.StatusBadRequest, nil)
	if list, _ := h.store.Monitors(t.Context()); len(list) != 0 {
		t.Fatal("a test check saved a monitor")
	}
}

func TestDemoAccount(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.DemoUser, o.DemoPassword = "demo", "look around freely" })
	h.signIn()
	hash, _ := auth.Hash("look around freely")
	h.store.CreateUser(t.Context(), "demo", hash, store.RoleViewer)

	var state authStateDTO
	h.expect("GET", "/api/auth/state", nil, http.StatusOK, &state)
	if state.Demo == nil || state.Demo.Username != "demo" {
		t.Fatalf("state = %+v", state)
	}

	demo := h.guest()
	demo.expect("POST", "/api/auth/login", credentials{Username: "demo", Password: "look around freely"}, http.StatusOK, nil)
	demo.expect("GET", "/api/admin/monitors", nil, http.StatusOK, nil)
	demo.expect("PUT", "/api/admin/account/password", map[string]string{"current": "look around freely", "next": "a brand new secret"}, http.StatusForbidden, nil)
}
