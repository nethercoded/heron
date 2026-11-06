package blackbox

import (
	"encoding/json/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nethercoded/heron/internal/monitor"
)

func quiet() *Recorder {
	return &Recorder{UserAgent: "heron-test", Timeout: 20 * time.Second}
}

func TestRecordWebsite(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "secret"})
		w.Header().Set("Server", "cloudflare")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<h1>502 Bad Gateway</h1>" + strings.Repeat("x", 5000)))
	}))
	t.Cleanup(srv.Close)

	m := &monitor.Monitor{Kind: monitor.KindHTTP, Target: srv.URL + "/health"}
	rep := quiet().Record(t.Context(), m, monitor.Down(0, "unexpected status 502"))

	r := rep.Response
	if r == nil || r.Code != http.StatusBadGateway || !strings.HasPrefix(r.Body, "<h1>502") || !r.Cut {
		t.Fatalf("response = %+v", r)
	}
	for _, h := range r.Headers {
		if h.Name == "Set-Cookie" && h.Value != "(hidden)" {
			t.Errorf("cookie leaked: %q", h.Value)
		}
	}
	if rep.TLS == nil || rep.TLS.Valid || len(rep.TLS.Chain) == 0 {
		t.Errorf("tls = %+v", rep.TLS)
	}
	if rep.DNS == nil || rep.DNS.Addresses[0] != "127.0.0.1" {
		t.Errorf("dns = %+v", rep.DNS)
	}
	if r.Timing.Total <= 0 && r.Timing.FirstByte < 0 {
		t.Errorf("timing = %+v", r.Timing)
	}
}

func TestRecordClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	rep := quiet().Record(t.Context(), &monitor.Monitor{Kind: monitor.KindTCP, Target: addr}, monitor.Down(0, "refused"))
	if rep.Connect == nil || rep.Connect.Error == "" {
		t.Fatalf("connect = %+v", rep.Connect)
	}
}

func TestRecordPush(t *testing.T) {
	rep := quiet().Record(t.Context(), &monitor.Monitor{Kind: monitor.KindPush}, monitor.Down(0, "no heartbeat"))
	if rep.DNS != nil || len(rep.Notes) == 0 {
		t.Fatalf("report = %+v", rep)
	}
}

func TestRouteOutputIsUTF8(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "tracert.bat")
	if runtime.GOOS != "windows" {
		script = filepath.Join(dir, "traceroute")
		os.WriteFile(script, []byte("#!/bin/sh\nprintf 'hop \x8e\xe2\xa2\xa5\xe2\n'\n"), 0o755)
	} else {
		os.WriteFile(script, []byte("@echo off\r\necho hop \x8e\xe2\xa2\xa5\xe2\r\n"), 0o755)
	}
	r := &Recorder{Tracer: script, Timeout: 10 * time.Second}
	route := r.trace(t.Context(), "127.0.0.1")
	if route == nil || !utf8.ValidString(route.Output) || !strings.Contains(route.Output, "hop") {
		t.Fatalf("route = %+v", route)
	}
	rep := &Report{Route: route}
	if _, err := json.Marshal(rep); err != nil {
		t.Fatal(err)
	}
}
