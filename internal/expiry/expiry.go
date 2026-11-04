package expiry

import (
	"bufio"
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/nethercoded/heron/internal/monitor"
)

type Kind string

const (
	KindDomain Kind = "domain"
	KindCert   Kind = "cert"
)

type Target struct {
	Host     string
	Kind     Kind
	Monitors []int64
}

func Targets(list []*monitor.Monitor) []Target {
	byKey := map[string]*Target{}
	var keys []string
	add := func(host string, kind Kind, id int64) {
		name := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			name = h
		}
		if name == "" || net.ParseIP(strings.Trim(name, "[]")) != nil || strings.EqualFold(name, "localhost") {
			return
		}
		key := string(kind) + " " + host
		t, ok := byKey[key]
		if !ok {
			t = &Target{Host: host, Kind: kind}
			byKey[key] = t
			keys = append(keys, key)
		}
		if !slices.Contains(t.Monitors, id) {
			t.Monitors = append(t.Monitors, id)
		}
	}

	for _, m := range list {
		var host string
		switch m.Kind {
		case monitor.KindHTTP:
			u, err := url.Parse(m.Target)
			if err != nil {
				continue
			}
			host = u.Hostname()
			if u.Scheme == "https" && !m.Settings.IgnoreTLS {
				port := cmp.Or(u.Port(), "443")
				add(net.JoinHostPort(host, port), KindCert, m.ID)
			}
		case monitor.KindDNS:
			host = m.Target
		default:
			continue
		}
		if domain, err := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(strings.TrimSuffix(host, "."))); err == nil {
			add(domain, KindDomain, m.ID)
		}
	}

	out := make([]Target, 0, len(keys))
	slices.Sort(keys)
	for _, k := range keys {
		out = append(out, *byKey[k])
	}
	return out
}

type Result struct {
	ExpiresAt time.Time
	Issuer    string
}

type Checker struct {
	Client    *http.Client
	Bootstrap string
	Whois     string
	Dial      func(ctx context.Context, network, addr string) (net.Conn, error)

	mu      sync.Mutex
	servers map[string][]string
	loaded  time.Time
}

func NewChecker() *Checker {
	var d net.Dialer
	return &Checker{
		Client:    &http.Client{Timeout: 20 * time.Second},
		Bootstrap: "https://data.iana.org/rdap/dns.json",
		Whois:     "whois.iana.org:43",
		Dial:      d.DialContext,
	}
}

func (c *Checker) Check(ctx context.Context, t Target) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if t.Kind == KindCert {
		return c.cert(ctx, t.Host)
	}
	return c.domain(ctx, t.Host)
}

func (c *Checker) cert(ctx context.Context, addr string) (Result, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return Result{}, err
	}
	raw, err := c.Dial(ctx, "tcp", addr)
	if err != nil {
		return Result{}, err
	}
	defer raw.Close()
	conn := tls.Client(raw, &tls.Config{ServerName: host, InsecureSkipVerify: true}) //nolint:gosec
	if err := conn.HandshakeContext(ctx); err != nil {
		return Result{}, err
	}
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return Result{}, errors.New("the server sent no certificate")
	}
	leaf := certs[0]
	return Result{ExpiresAt: leaf.NotAfter.UTC(), Issuer: cmp.Or(leaf.Issuer.Organization...)}, nil
}

func (c *Checker) domain(ctx context.Context, name string) (Result, error) {
	bases, err := c.rdapServers(ctx, name)
	if err == nil && len(bases) > 0 {
		res, err := c.rdap(ctx, bases[0], name)
		if err == nil {
			return res, nil
		}
	}
	return c.whois(ctx, name)
}

func (c *Checker) rdapServers(ctx context.Context, name string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.servers == nil || time.Since(c.loaded) > 24*time.Hour {
		servers, err := c.loadBootstrap(ctx)
		if err != nil {
			return nil, err
		}
		c.servers, c.loaded = servers, time.Now()
	}
	tld := name[strings.LastIndex(name, ".")+1:]
	return c.servers[tld], nil
}

func (c *Checker) loadBootstrap(ctx context.Context) (map[string][]string, error) {
	var doc struct {
		Services [][][]string `json:"services"`
	}
	if err := c.getJSON(ctx, c.Bootstrap, &doc); err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, svc := range doc.Services {
		if len(svc) < 2 {
			continue
		}
		for _, tld := range svc[0] {
			out[strings.ToLower(tld)] = svc[1]
		}
	}
	return out, nil
}

func (c *Checker) rdap(ctx context.Context, base, name string) (Result, error) {
	var doc struct {
		Events []struct {
			Action string `json:"eventAction"`
			Date   string `json:"eventDate"`
		} `json:"events"`
		Entities []struct {
			Roles []string `json:"roles"`
			Card  []any    `json:"vcardArray"`
		} `json:"entities"`
	}
	if err := c.getJSON(ctx, strings.TrimSuffix(base, "/")+"/domain/"+name, &doc); err != nil {
		return Result{}, err
	}
	var res Result
	for _, e := range doc.Events {
		if e.Action == "expiration" {
			t, err := time.Parse(time.RFC3339, e.Date)
			if err != nil {
				return Result{}, fmt.Errorf("rdap date %q: %w", e.Date, err)
			}
			res.ExpiresAt = t.UTC()
		}
	}
	if res.ExpiresAt.IsZero() {
		return Result{}, errors.New("the registry does not publish an expiry date")
	}
	for _, ent := range doc.Entities {
		if slices.Contains(ent.Roles, "registrar") {
			res.Issuer = vcardName(ent.Card)
		}
	}
	return res, nil
}

func vcardName(card []any) string {
	if len(card) < 2 {
		return ""
	}
	fields, _ := card[1].([]any)
	for _, f := range fields {
		parts, _ := f.([]any)
		if len(parts) >= 4 && parts[0] == "fn" {
			s, _ := parts[3].(string)
			return s
		}
	}
	return ""
}

func (c *Checker) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.UnmarshalRead(io.LimitReader(resp.Body, 2<<20), v)
}

var (
	referPattern  = regexp.MustCompile(`(?im)^\s*(?:refer|whois):\s*(\S+)`)
	expiryPattern = regexp.MustCompile(`(?im)^\s*(?:registry expiry date|registrar registration expiration date|expiration date|expiry date|expires on|expires|paid-till|free-date|renewal date)\s*:\s*(.+)$`)
	registrarLine = regexp.MustCompile(`(?im)^\s*(?:registrar|sponsoring registrar)\s*:\s*(.+)$`)
	dateLayouts   = []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02", "2006.01.02", "02-Jan-2006", "2006/01/02", "02.01.2006", "January 2 2006"}
)

func (c *Checker) whois(ctx context.Context, name string) (Result, error) {
	answer, err := c.ask(ctx, c.Whois, name)
	if err != nil {
		return Result{}, err
	}
	if m := referPattern.FindStringSubmatch(answer); m != nil {
		if answer, err = c.ask(ctx, net.JoinHostPort(strings.TrimSpace(m[1]), "43"), name); err != nil {
			return Result{}, err
		}
	}
	m := expiryPattern.FindStringSubmatch(answer)
	if m == nil {
		return Result{}, errors.New("neither RDAP nor WHOIS gave an expiry date")
	}
	t, err := parseDate(m[1])
	if err != nil {
		return Result{}, err
	}
	res := Result{ExpiresAt: t}
	if r := registrarLine.FindStringSubmatch(answer); r != nil {
		res.Issuer = strings.TrimSpace(r[1])
	}
	return res, nil
}

func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	candidates := []string{raw}
	if f := strings.Fields(raw); len(f) > 1 {
		candidates = append(candidates, f[0])
	}
	for _, layout := range dateLayouts {
		for _, candidate := range candidates {
			if t, err := time.Parse(layout, candidate); err == nil {
				return t.UTC(), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format %q", raw)
}

func (c *Checker) ask(ctx context.Context, server, query string) (string, error) {
	conn, err := c.Dial(ctx, "tcp", server)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if _, err := io.WriteString(conn, query+"\r\n"); err != nil {
		return "", err
	}
	var b strings.Builder
	scan := bufio.NewScanner(io.LimitReader(conn, 256<<10))
	for scan.Scan() {
		b.WriteString(scan.Text())
		b.WriteByte('\n')
	}
	return b.String(), scan.Err()
}
