package monitor

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("all systems operational"))
		case "/slow":
			time.Sleep(50 * time.Millisecond)
			w.Write([]byte("ok"))
		case "/fail":
			http.Error(w, "boom", http.StatusBadGateway)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/echo":
			if r.Header.Get("X-Token") != "secret" || r.Method != http.MethodPost {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusCreated)
		}
	}))
	t.Cleanup(srv.Close)

	tests := []struct {
		name     string
		path     string
		settings Settings
		want     Status
		message  string
	}{
		{name: "ok", path: "/ok", want: StatusUp},
		{name: "server error", path: "/fail", want: StatusDown, message: "502"},
		{name: "keyword found", path: "/ok", settings: Settings{Keyword: "operational"}, want: StatusUp},
		{name: "keyword missing", path: "/ok", settings: Settings{Keyword: "outage"}, want: StatusDown, message: "not found"},
		{name: "keyword must be absent", path: "/ok", settings: Settings{Keyword: "operational", KeywordAbsent: true}, want: StatusDown, message: "present"},
		{name: "accepted list", path: "/fail", settings: Settings{AcceptedStatus: []int{502}}, want: StatusUp},
		{name: "redirect followed", path: "/redirect", settings: Settings{MaxRedirects: 3, Keyword: "operational"}, want: StatusUp},
		{name: "redirect not followed", path: "/redirect", want: StatusUp, message: "302"},
		{name: "slow is degraded", path: "/slow", settings: Settings{DegradedAfterMS: 10}, want: StatusDegraded, message: "slow"},
		{
			name:     "method headers body",
			path:     "/echo",
			settings: Settings{Method: "POST", Headers: map[string]string{"X-Token": "secret"}, Body: `{}`, AcceptedStatus: []int{201}},
			want:     StatusUp,
		},
	}

	registry := Registry{KindHTTP: NewHTTPChecker("heron-test")}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{Name: tt.name, Kind: KindHTTP, Target: srv.URL + tt.path, Settings: tt.settings, Interval: time.Minute}
			m.Normalize()
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
			res := registry.Check(t.Context(), m)
			if res.Status != tt.want {
				t.Fatalf("status = %s (%s), want %s", res.Status, res.Message, tt.want)
			}
			if !strings.Contains(res.Message, tt.message) {
				t.Errorf("message = %q, want it to contain %q", res.Message, tt.message)
			}
		})
	}
}

func TestHTTPCheckerTLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	checker := NewHTTPChecker("heron-test")

	m := &Monitor{Name: "tls", Kind: KindHTTP, Target: srv.URL}
	m.Normalize()
	if res := checker.Check(t.Context(), m); res.Status != StatusDown || !strings.Contains(res.Message, "tls") {
		t.Errorf("self-signed certificate accepted: %+v", res)
	}

	m.Settings.IgnoreTLS = true
	if res := checker.Check(t.Context(), m); res.Status != StatusUp {
		t.Errorf("ignore_tls not honoured: %+v", res)
	}
}

func TestTCPChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	m := &Monitor{Kind: KindTCP, Target: addr, Timeout: time.Second}
	if res := (TCPChecker{}).Check(t.Context(), m); res.Status != StatusUp {
		t.Fatalf("open port reported %s: %s", res.Status, res.Message)
	}

	ln.Close()
	if res := (TCPChecker{}).Check(t.Context(), m); res.Status != StatusDown {
		t.Fatalf("closed port reported %s", res.Status)
	}
}

func TestRegistryTimeout(t *testing.T) {
	registry := Registry{KindTCP: CheckerFunc(func(ctx context.Context, _ *Monitor) Result {
		<-ctx.Done()
		return Result{Status: StatusDown}
	})}
	m := &Monitor{Kind: KindTCP, Timeout: 20 * time.Millisecond}

	res := registry.Check(t.Context(), m)
	if res.Status != StatusDown || !strings.Contains(res.Message, "timed out") {
		t.Fatalf("got %+v", res)
	}
	if res.At.IsZero() {
		t.Error("result time not set")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mon    Monitor
		fields []string
	}{
		{name: "valid http", mon: Monitor{Name: "Site", Kind: KindHTTP, Target: "https://example.com"}},
		{name: "valid push", mon: Monitor{Name: "Backups", Kind: KindPush, Interval: time.Hour}},
		{name: "valid dns", mon: Monitor{Name: "DNS", Kind: KindDNS, Target: "example.com", Settings: Settings{RecordType: "mx", Resolver: "1.1.1.1"}}},
		{name: "valid icmp v6", mon: Monitor{Name: "Ping", Kind: KindICMP, Target: "2001:db8::1"}},
		{name: "missing name", mon: Monitor{Kind: KindTCP, Target: "db:5432", Slug: "db"}, fields: []string{"name"}},
		{name: "bad http target", mon: Monitor{Name: "x", Kind: KindHTTP, Target: "ftp://example.com"}, fields: []string{"target"}},
		{name: "tcp without port", mon: Monitor{Name: "x", Kind: KindTCP, Target: "db"}, fields: []string{"target"}},
		{name: "dns ip target", mon: Monitor{Name: "x", Kind: KindDNS, Target: "1.1.1.1"}, fields: []string{"target"}},
		{name: "interval too short", mon: Monitor{Name: "x", Kind: KindTCP, Target: "db:1", Interval: time.Second}, fields: []string{"interval"}},
		{name: "timeout above interval", mon: Monitor{Name: "x", Kind: KindTCP, Target: "db:1", Interval: time.Minute, Timeout: 2 * time.Minute}, fields: []string{"timeout"}},
		{name: "unknown kind", mon: Monitor{Name: "x", Kind: "smtp"}, fields: []string{"kind"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mon.Normalize()
			err := tt.mon.Validate()

			var got []string
			for _, e := range unwrap(err) {
				var fe *FieldError
				if errors.As(e, &fe) {
					got = append(got, fe.Field)
				}
			}
			if strings.Join(got, ",") != strings.Join(tt.fields, ",") {
				t.Errorf("fields = %v, want %v (%v)", got, tt.fields, err)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Main Website":     "main-website",
		"  API / v2  ":     "api-v2",
		"Postgres (prod)!": "postgres-prod",
		"Платежи":          "",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func unwrap(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
