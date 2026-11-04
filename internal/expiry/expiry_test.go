package expiry

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/store"
)

func TestTargets(t *testing.T) {
	list := []*monitor.Monitor{
		{ID: 1, Kind: monitor.KindHTTP, Target: "https://api.shop.example.co.uk/health"},
		{ID: 2, Kind: monitor.KindHTTP, Target: "https://www.example.co.uk"},
		{ID: 3, Kind: monitor.KindHTTP, Target: "http://intranet.example.com:8080"},
		{ID: 4, Kind: monitor.KindHTTP, Target: "https://10.0.0.5"},
		{ID: 5, Kind: monitor.KindDNS, Target: "mail.example.org"},
		{ID: 6, Kind: monitor.KindTCP, Target: "db:5432"},
		{ID: 7, Kind: monitor.KindHTTP, Target: "https://self.example.net", Settings: monitor.Settings{IgnoreTLS: true}},
	}
	var got []string
	for _, tg := range Targets(list) {
		got = append(got, string(tg.Kind)+" "+tg.Host)
		if tg.Host == "example.co.uk" && len(tg.Monitors) != 2 {
			t.Errorf("example.co.uk monitors = %v", tg.Monitors)
		}
	}
	want := "cert api.shop.example.co.uk:443,cert www.example.co.uk:443,domain example.co.uk,domain example.com,domain example.net,domain example.org"
	if strings.Join(got, ",") != want {
		t.Fatalf("targets = %v", got)
	}
}

func TestRDAP(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bootstrap":
			io.WriteString(w, `{"services":[[["com","net"],["`+srv.URL+`/rdap/"]]]}`)
		case "/rdap/domain/example.com":
			io.WriteString(w, `{"events":[{"eventAction":"registration","eventDate":"1995-08-14T04:00:00Z"},{"eventAction":"expiration","eventDate":"2027-08-13T04:00:00Z"}],
				"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Example Registrar, Inc."]]]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewChecker()
	c.Client, c.Bootstrap = srv.Client(), srv.URL+"/bootstrap"
	res, err := c.Check(t.Context(), Target{Host: "example.com", Kind: KindDomain})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ExpiresAt.Equal(time.Date(2027, 8, 13, 4, 0, 0, 0, time.UTC)) || res.Issuer != "Example Registrar, Inc." {
		t.Fatalf("result = %+v", res)
	}
}

func TestWhoisFallback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	answers := map[string]string{
		"iana":     "domain:       RU\nrefer:        " + ln.Addr().String() + "\n",
		"registry": "domain:        EXAMPLE.RU\nregistrar:     RU-CENTER-RU\npaid-till:     2027-01-30T21:00:00Z\n",
	}
	var mu sync.Mutex
	calls := 0
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 256)
			conn.Read(buf)
			mu.Lock()
			calls++
			reply := answers["registry"]
			if calls == 1 {
				reply = answers["iana"]
			}
			mu.Unlock()
			io.WriteString(conn, reply)
			conn.Close()
		}
	}()

	c := NewChecker()
	c.Bootstrap = "http://127.0.0.1:1/none"
	c.Whois = ln.Addr().String()
	c.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, ln.Addr().String())
	}
	res, err := c.Check(t.Context(), Target{Host: "example.ru", Kind: KindDomain})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ExpiresAt.Equal(time.Date(2027, 1, 30, 21, 0, 0, 0, time.UTC)) || res.Issuer != "RU-CENTER-RU" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	res, err := NewChecker().Check(t.Context(), Target{Host: strings.TrimPrefix(srv.URL, "https://"), Kind: KindCert})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ExpiresAt.Equal(srv.Certificate().NotAfter.UTC()) {
		t.Fatalf("expires = %s, want %s", res.ExpiresAt, srv.Certificate().NotAfter)
	}
}

func TestParseDate(t *testing.T) {
	for _, raw := range []string{"2027-01-30T21:00:00Z", "2027-01-30", "30-Jan-2027", "2027.01.30", "2027-01-30 21:00:00 (UTC)"} {
		if d, err := parseDate(raw); err != nil || d.Year() != 2027 || d.Month() != time.January || d.Day() != 30 {
			t.Errorf("%q = %s, %v", raw, d, err)
		}
	}
}

type fakeProber struct{ expires time.Time }

func (f *fakeProber) Check(context.Context, Target) (Result, error) {
	return Result{ExpiresAt: f.expires, Issuer: "Test CA"}, nil
}

type outbox struct{ sent []notify.Message }

func (o *outbox) Dispatch(_ []notify.Channel, msg notify.Message) { o.sent = append(o.sent, msg) }

func TestWarnings(t *testing.T) {
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "heron.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ch := &notify.Channel{Name: "ops", Kind: notify.KindWebhook, Enabled: true, Config: notify.Config{URL: "https://example.com"}}
	db.CreateChannel(t.Context(), ch)
	m := &monitor.Monitor{Name: "Site", Slug: "site", Kind: monitor.KindHTTP, Target: "https://example.com", ChannelIDs: []int64{ch.ID}}
	m.Normalize()
	db.CreateMonitor(t.Context(), m)

	probe := &fakeProber{expires: time.Now().Add(20 * 24 * time.Hour)}
	box := &outbox{}
	s := &Service{Store: db, Prober: probe, Sender: box, Every: 12 * time.Hour, Log: slog.New(slog.DiscardHandler)}

	refresh := func() {
		t.Helper()
		if err := s.Refresh(t.Context(), true); err != nil {
			t.Fatal(err)
		}
	}
	refresh()
	if len(box.sent) != 2 {
		t.Fatalf("first run sent %d alerts, want one for the domain and one for the certificate", len(box.sent))
	}
	refresh()
	if len(box.sent) != 2 {
		t.Fatal("the same warning went out twice")
	}

	probe.expires = time.Now().Add(5 * 24 * time.Hour)
	refresh()
	if len(box.sent) != 4 || !strings.Contains(box.sent[3].Title, "expires in 5 days") {
		t.Fatalf("alerts = %d, last %q", len(box.sent), box.sent[len(box.sent)-1].Title)
	}

	probe.expires = time.Now().Add(400 * 24 * time.Hour)
	refresh()
	probe.expires = time.Now().Add(25 * 24 * time.Hour)
	refresh()
	if len(box.sent) != 6 {
		t.Fatalf("a renewed certificate did not warn again: %d alerts", len(box.sent))
	}

	list, _ := db.Expiries(t.Context())
	if len(list) != 2 || list[0].Issuer != "Test CA" {
		t.Fatalf("saved = %+v", list)
	}
}
