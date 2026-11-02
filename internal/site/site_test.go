package site

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nethercode/heron/internal/monitor"
)

type memStore struct{ s Settings }

func (m *memStore) Site(context.Context) (Settings, error)       { return m.s, nil }
func (m *memStore) SaveSite(_ context.Context, s Settings) error { m.s = s; return nil }

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Status.Example.com":               "status.example.com",
		"https://status.example.com/admin": "status.example.com",
		"http://example.com:8080":          "example.com",
		"status.example.com.":              "status.example.com",
	} {
		s := Settings{Domain: in, Mode: ModeDirect}
		s.Normalize()
		if s.Domain != want {
			t.Errorf("%q -> %q, want %q", in, s.Domain, want)
		}
		if err := s.Validate(); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}

	for _, bad := range []string{"localhost", "203.0.113.5", "exa mple.com", "-bad.example.com"} {
		s := Settings{Domain: bad, Mode: ModeDirect}
		s.Normalize()
		if err := s.Validate(); err == nil {
			t.Errorf("%q passed validation", bad)
		}
	}
}

func TestProxyModeChecksReachability(t *testing.T) {
	store := &memStore{}
	m := New(store, t.TempDir(), "http://fallback.test", slog.New(slog.DiscardHandler))

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ProbePath+"{nonce}", func(w http.ResponseWriter, r *http.Request) {
		if m.Probe(r.PathValue("nonce")) {
			io.WriteString(w, r.PathValue("nonce"))
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	m.Client = srv.Client()
	m.Client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, srv.Listener.Addr().String())
	}

	var changed string
	m.OnChange = func(u string) { changed = u }

	res, err := m.Apply(t.Context(), Settings{Domain: "status.example.com", Mode: ModeProxy})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reachable || res.URL != "https://status.example.com" {
		t.Errorf("result = %+v", res)
	}
	if store.s.Domain != "status.example.com" || changed != "https://status.example.com" || m.PublicURL() != "https://status.example.com" {
		t.Errorf("stored %+v, changed %q", store.s, changed)
	}

	if _, err := m.Apply(t.Context(), Settings{}); err != nil {
		t.Fatal(err)
	}
	if m.PublicURL() != "http://fallback.test" {
		t.Errorf("fallback = %q", m.PublicURL())
	}
}

func TestDirectModeReportsBusyPort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { busy.Close() })

	m := New(&memStore{}, t.TempDir(), "", slog.New(slog.DiscardHandler))
	m.Handler = http.NotFoundHandler()
	m.HTTPAddr = busy.Addr().String()

	_, err = m.Apply(t.Context(), Settings{Domain: "status.example.com", Mode: ModeDirect})
	fe, ok := errors.AsType[*monitor.FieldError](err)
	if !ok || !strings.Contains(fe.Message, "reverse proxy") {
		t.Fatalf("err = %v", err)
	}
}
