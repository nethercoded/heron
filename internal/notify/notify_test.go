package notify

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nethercode/heron/internal/monitor"
)

var sample = Message{
	Event:    EventRecovered,
	Monitor:  &monitor.Monitor{ID: 4, Slug: "api", Name: "API <prod>", Kind: monitor.KindHTTP, Target: "https://api.example.com"},
	Status:   monitor.StatusUp,
	Reason:   "200 OK",
	At:       time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
	Downtime: 3*time.Minute + 20*time.Second,
	Link:     "https://status.example.com/monitor/api",
}

func TestWebhookSignsPayload(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		want := "sha256=" + Sign("s3cret", r.Header.Get(HeaderTimestamp), body)
		if r.Header.Get(HeaderSignature) != want {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Extra") != "1" || r.Header.Get(HeaderEvent) != string(EventRecovered) {
			http.Error(w, "missing headers", http.StatusBadRequest)
			return
		}
		json.Unmarshal(body, &got)
	}))
	t.Cleanup(srv.Close)

	f := Factory{Client: srv.Client(), UserAgent: "heron-test"}
	n, _ := f.New(Channel{Kind: KindWebhook, Config: Config{URL: srv.URL, Secret: "s3cret", Headers: map[string]string{"X-Extra": "1"}}})
	if err := n.Notify(t.Context(), sample); err != nil {
		t.Fatal(err)
	}
	if got.Monitor == nil || got.Monitor.Slug != "api" || got.Downtime != 200 || got.Status != "up" {
		t.Errorf("payload = %+v", got)
	}
}

func TestTelegram(t *testing.T) {
	var got telegramRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"ok":false,"description":"Not Found"}`))
			return
		}
		json.UnmarshalRead(r.Body, &got)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	f := Factory{Client: srv.Client(), TelegramAPI: srv.URL}
	n, _ := f.New(Channel{Kind: KindTelegram, Config: Config{BotToken: "TOKEN", ChatID: "-100"}})
	if err := n.Notify(t.Context(), sample); err != nil {
		t.Fatal(err)
	}
	if got.ChatID != "-100" || got.ParseMode != "HTML" {
		t.Errorf("request = %+v", got)
	}
	for _, part := range []string{"API &lt;prod&gt; is back up", "Downtime: 3m 20s", `href="https://status.example.com/monitor/api"`} {
		if !strings.Contains(got.Text, part) {
			t.Errorf("text %q is missing %q", got.Text, part)
		}
	}

	bad, _ := f.New(Channel{Kind: KindTelegram, Config: Config{BotToken: "WRONG", ChatID: "-100"}})
	err := bad.Notify(t.Context(), sample)
	if se, ok := errors.AsType[*StatusError](err); !ok || se.Code != 404 || strings.Contains(err.Error(), "WRONG") {
		t.Errorf("error = %v", err)
	}
}

func TestDispatcherRetriesTemporaryFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(srv.Close)

	d := NewDispatcher(Factory{Client: srv.Client()}, slog.New(slog.DiscardHandler))
	d.backoff = time.Millisecond
	d.Dispatch([]Channel{
		{Name: "hook", Kind: KindWebhook, Enabled: true, Config: Config{URL: srv.URL}},
		{Name: "off", Kind: KindWebhook, Enabled: false, Config: Config{URL: srv.URL}},
	}, sample)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := d.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("calls = %d, want 3", n)
	}
}

func TestDispatcherStopsOnPermanentFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	t.Cleanup(srv.Close)

	d := NewDispatcher(Factory{Client: srv.Client()}, slog.New(slog.DiscardHandler))
	d.backoff = time.Millisecond
	d.Dispatch([]Channel{{Name: "hook", Kind: KindWebhook, Enabled: true, Config: Config{URL: srv.URL}}}, sample)
	d.Close(t.Context())

	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want 1", n)
	}
}

func TestChannelValidate(t *testing.T) {
	valid := []Channel{
		{Name: "tg", Kind: KindTelegram, Config: Config{BotToken: "t", ChatID: "1"}},
		{Name: "hook", Kind: KindWebhook, Config: Config{URL: "https://example.com"}},
	}
	for _, c := range valid {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
	}
	invalid := []Channel{
		{Kind: KindWebhook, Config: Config{URL: "https://example.com"}},
		{Name: "tg", Kind: KindTelegram},
		{Name: "hook", Kind: KindWebhook, Config: Config{URL: "mailto:x@example.com"}},
		{Name: "sms", Kind: "sms"},
	}
	for _, c := range invalid {
		if err := c.Validate(); err == nil {
			t.Errorf("%+v passed validation", c)
		}
	}
}
