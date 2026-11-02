package httpapi

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/store"
)

func (h *harness) guest() *harness {
	jar, _ := cookiejar.New(nil)
	other := *h
	other.client = &http.Client{Jar: jar}
	return &other
}

func (h *harness) invite(role store.Role, username string) *harness {
	h.t.Helper()
	var inv inviteDTO
	h.expect("POST", "/api/admin/invites", map[string]any{"role": role}, http.StatusCreated, &inv)
	if inv.Token == "" {
		h.t.Fatal("invite without token")
	}

	member := h.guest()
	var info inviteDTO
	member.expect("POST", "/api/auth/invite", map[string]string{"token": inv.Token}, http.StatusOK, &info)
	if info.Role != role || info.CreatedBy != "admin" {
		h.t.Fatalf("invite info = %+v", info)
	}
	member.expect("POST", "/api/auth/join", map[string]string{"token": inv.Token, "username": username, "password": "another long secret"}, http.StatusCreated, nil)
	h.guest().expect("POST", "/api/auth/join", map[string]string{"token": inv.Token, "username": "again", "password": "another long secret"}, http.StatusNotFound, nil)
	return member
}

func TestTeamRoles(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	h.expect("POST", "/api/admin/invites", map[string]any{"role": "owner"}, http.StatusUnprocessableEntity, nil)
	viewer := h.invite(store.RoleViewer, "watcher")
	editor := h.invite(store.RoleAdmin, "editor")

	monitor := map[string]any{"name": "API", "kind": "tcp", "target": "example.com:443", "interval_s": 60, "timeout_s": 10, "public": true}
	viewer.expect("POST", "/api/admin/monitors", monitor, http.StatusForbidden, nil)
	viewer.expect("GET", "/api/admin/monitors", nil, http.StatusOK, nil)
	viewer.expect("GET", "/api/admin/channels", nil, http.StatusForbidden, nil)
	editor.expect("POST", "/api/admin/monitors", monitor, http.StatusCreated, nil)
	editor.expect("POST", "/api/admin/invites", map[string]any{"role": "admin"}, http.StatusForbidden, nil)
	editor.expect("PUT", "/api/admin/site", map[string]any{"domain": ""}, http.StatusForbidden, nil)

	var team struct {
		Users   []memberDTO `json:"users"`
		Invites []inviteDTO `json:"invites"`
	}
	h.expect("GET", "/api/admin/users", nil, http.StatusOK, &team)
	if len(team.Users) != 3 || team.Users[0].Role != store.RoleOwner {
		t.Fatalf("users = %+v", team.Users)
	}
	owner, watcher := team.Users[0], team.Users[1]

	h.expect("DELETE", "/api/admin/users/"+itoa(owner.ID), nil, http.StatusForbidden, nil)
	h.expect("PUT", "/api/admin/users/"+itoa(watcher.ID), map[string]any{"role": "admin"}, http.StatusOK, nil)
	viewer.expect("POST", "/api/admin/monitors", map[string]any{"name": "Web", "kind": "tcp", "target": "example.com:80", "interval_s": 60, "timeout_s": 10}, http.StatusCreated, nil)

	h.expect("DELETE", "/api/admin/users/"+itoa(watcher.ID), nil, http.StatusNoContent, nil)
	viewer.expect("GET", "/api/admin/monitors", nil, http.StatusUnauthorized, nil)

	var log []auditDTO
	h.expect("GET", "/api/admin/audit?action=monitor", nil, http.StatusOK, &log)
	if len(log) != 2 || log[0].Username != "watcher" || log[0].Target != "Web" || log[1].Username != "editor" || log[1].Action != "monitor.create" {
		t.Fatalf("audit = %+v", log)
	}
	h.expect("GET", "/api/admin/audit?action=user.delete", nil, http.StatusOK, &log)
	if len(log) != 1 || log[0].Target != "watcher" {
		t.Fatalf("audit = %+v", log)
	}
}

func TestTwoFactorLogin(t *testing.T) {
	h := newHarness(t)
	h.signIn()

	var setup struct {
		Secret string `json:"secret"`
		URI    string `json:"uri"`
		QR     string `json:"qr"`
	}
	h.expect("POST", "/api/admin/account/totp", nil, http.StatusOK, &setup)
	if !strings.HasPrefix(setup.URI, "otpauth://totp/") || !strings.HasPrefix(setup.QR, "<svg") {
		t.Fatalf("setup = %+v", setup)
	}
	h.expect("PUT", "/api/admin/account/totp", map[string]string{"secret": setup.Secret, "code": "000000"}, http.StatusUnprocessableEntity, nil)
	h.expect("PUT", "/api/admin/account/totp", map[string]string{"secret": setup.Secret, "code": code(setup.Secret, time.Now())}, http.StatusNoContent, nil)

	guest := h.guest()
	var login loginDTO
	guest.expect("POST", "/api/auth/login", credentials{Username: "admin", Password: "correct horse battery"}, http.StatusOK, &login)
	if login.User != nil || login.Ticket == "" {
		t.Fatalf("login = %+v", login)
	}
	guest.expect("GET", "/api/admin/monitors", nil, http.StatusUnauthorized, nil)
	guest.expect("POST", "/api/auth/login/code", map[string]string{"ticket": login.Ticket, "code": code(setup.Secret, time.Now())}, http.StatusUnprocessableEntity, nil)
	guest.expect("POST", "/api/auth/login/code", map[string]string{"ticket": login.Ticket, "code": code(setup.Secret, time.Now().Add(30*time.Second))}, http.StatusOK, nil)
	guest.expect("GET", "/api/admin/monitors", nil, http.StatusOK, nil)
}

func TestMetrics(t *testing.T) {
	h := newHarness(t)
	h.signIn()
	h.expect("GET", "/metrics", nil, http.StatusNotFound, nil)

	var token struct {
		Token string `json:"token"`
	}
	h.expect("POST", "/api/admin/metrics", nil, http.StatusOK, &token)
	h.expect("POST", "/api/admin/monitors", map[string]any{"name": `My "API"`, "slug": "api", "kind": "tcp", "target": "example.com:443", "interval_s": 60, "timeout_s": 10}, http.StatusCreated, nil)
	h.guest().expect("GET", "/metrics", nil, http.StatusUnauthorized, nil)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, h.srv.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`heron_build_info{version="test"} 1`,
		`heron_monitor_status{monitor="api",name="My \"API\"",kind="tcp",group="",status="pending"} 1`,
		"heron_incidents_open 0",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics missing %q:\n%s", want, body)
		}
	}
}

func code(secret string, at time.Time) string {
	return auth.TOTP(secret, at)
}
