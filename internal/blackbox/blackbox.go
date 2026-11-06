package blackbox

import (
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
)

type Report struct {
	At       time.Time `json:"at"`
	Target   string    `json:"target"`
	Reason   string    `json:"reason"`
	DNS      *DNS      `json:"dns,omitzero"`
	Connect  *Connect  `json:"connect,omitzero"`
	TLS      *TLS      `json:"tls,omitzero"`
	Response *Response `json:"response,omitzero"`
	Route    *Route    `json:"route,omitzero"`
	Notes    []string  `json:"notes,omitzero"`
	Image    []byte    `json:"-"`
}

type DNS struct {
	Host      string   `json:"host"`
	Addresses []string `json:"addresses,omitzero"`
	CNAME     string   `json:"cname,omitzero"`
	TookMS    int64    `json:"took_ms"`
	Error     string   `json:"error,omitzero"`
}

type Connect struct {
	Address string `json:"address"`
	TookMS  int64  `json:"took_ms"`
	Error   string `json:"error,omitzero"`
}

type TLS struct {
	Version string `json:"version,omitzero"`
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitzero"`
	Chain   []Cert `json:"chain,omitzero"`
}

type Cert struct {
	Subject  string    `json:"subject"`
	Issuer   string    `json:"issuer"`
	NotAfter time.Time `json:"not_after"`
	Names    []string  `json:"names,omitzero"`
}

type Response struct {
	Status   string   `json:"status,omitzero"`
	Code     int      `json:"code,omitzero"`
	Remote   string   `json:"remote,omitzero"`
	Headers  []Header `json:"headers,omitzero"`
	Body     string   `json:"body,omitzero"`
	Cut      bool     `json:"cut,omitzero"`
	Timing   Timing   `json:"timing"`
	Error    string   `json:"error,omitzero"`
	Redirect string   `json:"redirect,omitzero"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Timing struct {
	DNS       int64 `json:"dns_ms"`
	Connect   int64 `json:"connect_ms"`
	TLS       int64 `json:"tls_ms"`
	FirstByte int64 `json:"first_byte_ms"`
	Total     int64 `json:"total_ms"`
}

type Route struct {
	Tool   string `json:"tool"`
	Output string `json:"output,omitzero"`
	Error  string `json:"error,omitzero"`
}

type Recorder struct {
	UserAgent string
	Browser   string
	Tracer    string
	Timeout   time.Duration
}

func New(userAgent string) *Recorder {
	return &Recorder{UserAgent: userAgent, Browser: findBrowser(), Tracer: findTracer(), Timeout: 45 * time.Second}
}

const bodyLimit = 4 << 10

var hidden = []string{"set-cookie", "authorization", "cookie", "proxy-authorization", "www-authenticate"}

func (r *Recorder) Record(ctx context.Context, m *monitor.Monitor, res monitor.Result) *Report {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	rep := &Report{At: time.Now().UTC(), Target: m.DisplayTarget(), Reason: res.Message}
	host, port, web := endpoint(m)
	if host == "" {
		rep.Notes = append(rep.Notes, "This kind of check has no network address to inspect.")
		return rep
	}

	rep.DNS = lookup(ctx, host)
	switch {
	case web != nil:
		rep.Response = r.fetch(ctx, m, web)
		if web.Scheme == "https" {
			rep.TLS = inspectTLS(ctx, net.JoinHostPort(host, cmp.Or(web.Port(), "443")), host)
		}
	case port != "":
		rep.Connect = dial(ctx, net.JoinHostPort(host, port))
		if m.Settings.TLS {
			rep.TLS = inspectTLS(ctx, net.JoinHostPort(host, port), host)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if web != nil {
			rep.Image = r.screenshot(ctx, web.String())
		}
	}()
	rep.Route = r.trace(ctx, host)
	<-done

	if r.Tracer == "" {
		rep.Notes = append(rep.Notes, "Install traceroute or mtr on the server to see the network route here.")
	}
	if web != nil && r.Browser == "" {
		rep.Notes = append(rep.Notes, "Install Chromium on the server to get a screenshot of the page.")
	}
	return rep
}

func endpoint(m *monitor.Monitor) (string, string, *url.URL) {
	switch m.Kind {
	case monitor.KindHTTP:
		u, err := url.Parse(m.Target)
		if err != nil {
			return "", "", nil
		}
		return u.Hostname(), u.Port(), u
	case monitor.KindPostgres:
		u, err := url.Parse(m.Target)
		if err != nil {
			return "", "", nil
		}
		return u.Hostname(), cmp.Or(u.Port(), "5432"), nil
	case monitor.KindICMP, monitor.KindDNS:
		return m.Target, "", nil
	case monitor.KindPush, monitor.KindDocker, monitor.KindPelicanNode, monitor.KindPelicanServer:
		return "", "", nil
	}
	host, port, err := net.SplitHostPort(m.Target)
	if err != nil {
		return m.Target, "", nil
	}
	return host, port, nil
}

func ms(d time.Duration) int64 { return d.Milliseconds() }

func lookup(ctx context.Context, host string) *DNS {
	out := &DNS{Host: host}
	if net.ParseIP(host) != nil {
		out.Addresses = []string{host}
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	out.TookMS = ms(time.Since(started))
	if err != nil {
		out.Error = err.Error()
		return out
	}
	slices.Sort(addrs)
	out.Addresses = addrs
	if cname, err := net.DefaultResolver.LookupCNAME(ctx, host); err == nil && strings.TrimSuffix(cname, ".") != host {
		out.CNAME = strings.TrimSuffix(cname, ".")
	}
	return out
}

func dial(ctx context.Context, addr string) *Connect {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out := &Connect{Address: addr}
	started := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	out.TookMS = ms(time.Since(started))
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Address = conn.RemoteAddr().String()
	conn.Close()
	return out
}

func inspectTLS(ctx context.Context, addr, host string) *TLS {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out := &TLS{Valid: true}
	conn, err := (&tls.Dialer{Config: &tls.Config{ServerName: host}}).DialContext(ctx, "tcp", addr)
	if err != nil {
		out.Valid = false
		out.Error = err.Error()
		var certErr *tls.CertificateVerificationError
		if !errors.As(err, &certErr) && !strings.Contains(err.Error(), "x509") {
			return out
		}
		conn, err = (&tls.Dialer{Config: &tls.Config{ServerName: host, InsecureSkipVerify: true}}).DialContext(ctx, "tcp", addr) //nolint:gosec
		if err != nil {
			return out
		}
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	out.Version = tls.VersionName(state.Version)
	for _, c := range state.PeerCertificates {
		out.Chain = append(out.Chain, cert(c))
	}
	return out
}

func cert(c *x509.Certificate) Cert {
	return Cert{Subject: c.Subject.CommonName, Issuer: cmp.Or(c.Issuer.CommonName, strings.Join(c.Issuer.Organization, ", ")), NotAfter: c.NotAfter.UTC(), Names: c.DNSNames}
}

func (r *Recorder) fetch(ctx context.Context, m *monitor.Monitor, u *url.URL) *Response {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out := &Response{}

	var start, dnsStart, connStart, tlsStart time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:          func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:           func(httptrace.DNSDoneInfo) { out.Timing.DNS = ms(time.Since(dnsStart)) },
		ConnectStart:      func(string, string) { connStart = time.Now() },
		ConnectDone:       func(string, string, error) { out.Timing.Connect = ms(time.Since(connStart)) },
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { out.Timing.TLS = ms(time.Since(tlsStart)) },
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn != nil {
				out.Remote = info.Conn.RemoteAddr().String()
			}
		},
		GotFirstResponseByte: func() { out.Timing.FirstByte = ms(time.Since(start)) },
	}

	method := cmp.Or(m.Settings.Method, http.MethodGet)
	var body io.Reader
	if m.Settings.Body != "" {
		body = strings.NewReader(m.Settings.Body)
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, u.String(), body)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	for k, v := range m.Settings.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", r.UserAgent)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start = time.Now()
	resp, err := client.Do(req)
	out.Timing.Total = ms(time.Since(start))
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()

	out.Status, out.Code = resp.Status, resp.StatusCode
	out.Redirect = resp.Header.Get("Location")
	names := make([]string, 0, len(resp.Header))
	for name := range resp.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		value := strings.Join(resp.Header.Values(name), ", ")
		if slices.Contains(hidden, strings.ToLower(name)) {
			value = "(hidden)"
		}
		out.Headers = append(out.Headers, Header{Name: name, Value: value})
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	out.Timing.Total = ms(time.Since(start))
	if len(raw) > bodyLimit {
		raw, out.Cut = raw[:bodyLimit], true
	}
	if textual(resp.Header.Get("Content-Type"), raw) {
		out.Body = strings.ToValidUTF8(string(raw), "?")
	} else if len(raw) > 0 {
		out.Body = "(" + strconv.Itoa(len(raw)) + " bytes of " + cmp.Or(resp.Header.Get("Content-Type"), "binary data") + ")"
	}
	return out
}

func textual(kind string, raw []byte) bool {
	kind = strings.ToLower(kind)
	for _, t := range []string{"text/", "json", "xml", "javascript", "html"} {
		if strings.Contains(kind, t) {
			return true
		}
	}
	return kind == "" && !bytes.ContainsRune(raw, 0)
}

func findTracer() string {
	for _, name := range []string{"mtr", "traceroute", "tracepath", "tracert"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func (r *Recorder) trace(ctx context.Context, host string) *Route {
	if r.Tracer == "" {
		return nil
	}
	tool := strings.TrimSuffix(filepath.Base(r.Tracer), ".exe")
	var args []string
	switch tool {
	case "mtr":
		args = []string{"--report", "--report-cycles", "2", "--no-dns", "--max-ttl", "20", host}
	case "traceroute":
		args = []string{"-n", "-q", "1", "-w", "1", "-m", "20", host}
	case "tracepath":
		args = []string{"-n", "-m", "20", host}
	case "tracert":
		args = []string{"-d", "-h", "20", "-w", "800", host}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.Tracer, args...).CombinedOutput()
	route := &Route{Tool: tool, Output: strings.TrimSpace(strings.ToValidUTF8(string(out), ""))}
	if len(route.Output) > 6<<10 {
		route.Output = route.Output[:6<<10]
	}
	if err != nil && route.Output == "" {
		route.Error = err.Error()
	}
	return route
}

func findBrowser() string {
	names := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "microsoft-edge"}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "windows" {
		for _, p := range []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func (r *Recorder) screenshot(ctx context.Context, page string) []byte {
	if r.Browser == "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "heron-shot-")
	if err != nil {
		return nil
	}
	defer func() { _ = os.RemoveAll(dir) }()
	file := filepath.Join(dir, "page.png")
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Browser, "--headless=new", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
		"--no-first-run", "--user-data-dir="+filepath.Join(dir, "profile"), "--window-size=1280,800",
		"--virtual-time-budget=8000", "--screenshot="+file, page)
	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return nil
	}
	raw, err := os.ReadFile(file)
	if err != nil || len(raw) > 4<<20 {
		return nil
	}
	return raw
}
