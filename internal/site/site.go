package site

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/nethercoded/heron/internal/monitor"
)

type Mode string

const (
	ModeNone   Mode = ""
	ModeDirect Mode = "direct"
	ModeProxy  Mode = "proxy"
)

type Settings struct {
	Domain string `json:"domain,omitzero"`
	Mode   Mode   `json:"mode,omitzero"`
	Email  string `json:"email,omitzero"`
}

func (s Settings) URL() string {
	if s.Domain == "" {
		return ""
	}
	return "https://" + s.Domain
}

var hostname = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

func (s *Settings) Normalize() {
	d := strings.ToLower(strings.TrimSpace(s.Domain))
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if host, _, err := net.SplitHostPort(d); err == nil {
		d = host
	}
	s.Domain = strings.TrimSuffix(d, ".")
	s.Email = strings.TrimSpace(s.Email)
	if s.Domain == "" {
		s.Mode = ModeNone
	}
}

func (s Settings) Validate() error {
	var errs []error
	if s.Mode != ModeNone && !hostname.MatchString(s.Domain) {
		errs = append(errs, &monitor.FieldError{Field: "domain", Message: "must be a domain name like status.example.com"})
	}
	if s.Mode != ModeNone && s.Mode != ModeDirect && s.Mode != ModeProxy {
		errs = append(errs, &monitor.FieldError{Field: "mode", Message: "must be direct or proxy"})
	}
	if s.Email != "" && !strings.Contains(s.Email, "@") {
		errs = append(errs, &monitor.FieldError{Field: "email", Message: "is not an email address"})
	}
	return errors.Join(errs...)
}

type Store interface {
	Site(ctx context.Context) (Settings, error)
	SaveSite(ctx context.Context, s Settings) error
}

type Result struct {
	URL       string
	Reachable bool
}

type Manager struct {
	store    Store
	certDir  string
	fallback string
	log      *slog.Logger

	Handler   http.Handler
	HTTPAddr  string
	HTTPSAddr string
	Client    *http.Client

	mu      sync.Mutex
	current Settings
	servers []*http.Server

	probeMu sync.Mutex
	probes  map[string]time.Time

	OnChange func(publicURL string)
}

func New(store Store, certDir, fallback string, log *slog.Logger) *Manager {
	return &Manager{
		store:     store,
		certDir:   certDir,
		fallback:  strings.TrimRight(fallback, "/"),
		log:       log,
		HTTPAddr:  ":80",
		HTTPSAddr: ":443",
		Client:    &http.Client{Timeout: 10 * time.Second},
		probes:    map[string]time.Time{},
	}
}

func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *Manager) PublicURL() string {
	if u := m.Settings().URL(); u != "" {
		return u
	}
	return m.fallback
}

func (m *Manager) Start(ctx context.Context) error {
	s, err := m.store.Site(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.current = s
	m.mu.Unlock()

	if s.Mode == ModeDirect {
		if err := m.serve(ctx, m.certManager(s)); err != nil {
			return fmt.Errorf("serve %s: %w", s.Domain, err)
		}
		m.log.Info("serving https", "domain", s.Domain)
	}
	m.changed()
	return nil
}

func (m *Manager) Apply(ctx context.Context, s Settings) (Result, error) {
	s.Normalize()
	if err := s.Validate(); err != nil {
		return Result{}, err
	}

	switch s.Mode {
	case ModeDirect:
		if err := m.applyDirect(ctx, s); err != nil {
			return Result{}, err
		}
	default:
		m.stop(ctx)
	}

	if err := m.store.SaveSite(ctx, s); err != nil {
		m.stop(ctx)
		return Result{}, err
	}
	m.mu.Lock()
	m.current = s
	m.mu.Unlock()
	m.changed()

	res := Result{URL: s.URL(), Reachable: s.Mode == ModeDirect}
	if s.Mode == ModeProxy {
		res.Reachable = m.reachable(ctx, s.URL()) == nil
	}
	return res, nil
}

func (m *Manager) applyDirect(ctx context.Context, s Settings) error {
	if cur := m.Settings(); cur.Mode == ModeDirect && cur.Domain == s.Domain && len(m.servers) > 0 {
		return nil
	}
	m.stop(ctx)

	manager := m.certManager(s)
	httpSrv, err := m.listenHTTP(ctx, manager)
	if err != nil {
		return err
	}

	if err := m.reachable(ctx, "http://"+s.Domain); err != nil {
		m.stop(ctx)
		return &monitor.FieldError{Field: "domain", Message: m.explain(ctx, s.Domain, err)}
	}

	if _, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: s.Domain}); err != nil {
		m.stop(ctx)
		return &monitor.FieldError{Field: "domain", Message: "could not get a certificate: " + err.Error()}
	}

	if err := m.listenHTTPS(ctx, manager); err != nil {
		m.stop(ctx)
		return err
	}
	m.log.Info("serving https", "domain", s.Domain, "http", httpSrv.Addr)
	return nil
}

func (m *Manager) serve(ctx context.Context, manager *autocert.Manager) error {
	if _, err := m.listenHTTP(ctx, manager); err != nil {
		return err
	}
	if err := m.listenHTTPS(ctx, manager); err != nil {
		m.stop(ctx)
		return err
	}
	return nil
}

func (m *Manager) certManager(s Settings) *autocert.Manager {
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.Domain),
		Cache:      autocert.DirCache(m.certDir),
		Email:      s.Email,
	}
}

func (m *Manager) listenHTTP(ctx context.Context, manager *autocert.Manager) (*http.Server, error) {
	redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, ProbePath) {
			m.Handler.ServeHTTP(w, r)
			return
		}
		target := "https://" + stripPort(r.Host) + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	srv := &http.Server{Addr: m.HTTPAddr, Handler: manager.HTTPHandler(redirect), ReadHeaderTimeout: 10 * time.Second}
	return srv, m.start(ctx, srv, nil)
}

func (m *Manager) listenHTTPS(ctx context.Context, manager *autocert.Manager) error {
	cfg := manager.TLSConfig()
	cfg.MinVersion = tls.VersionTLS12
	srv := &http.Server{
		Addr:              m.HTTPSAddr,
		Handler:           m.Handler,
		TLSConfig:         cfg,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(m.log.Handler(), slog.LevelDebug),
	}
	return m.start(ctx, srv, cfg)
}

func (m *Manager) start(ctx context.Context, srv *http.Server, cfg *tls.Config) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", srv.Addr)
	if err != nil {
		return &monitor.FieldError{Field: "domain", Message: fmt.Sprintf("cannot listen on %s, is another web server running there? Use the reverse proxy option instead", srv.Addr)}
	}
	if cfg != nil {
		ln = tls.NewListener(ln, cfg)
	}
	m.mu.Lock()
	m.servers = append(m.servers, srv)
	m.mu.Unlock()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.log.Error("listener stopped", "addr", srv.Addr, "err", err)
		}
	}()
	return nil
}

func (m *Manager) stop(ctx context.Context) {
	m.mu.Lock()
	servers := m.servers
	m.servers = nil
	m.mu.Unlock()
	for _, srv := range servers {
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := srv.Shutdown(shutdown); err != nil {
			m.log.Debug("stop listener", "addr", srv.Addr, "err", err)
		}
		cancel()
	}
}

func (m *Manager) Close(ctx context.Context) {
	m.stop(ctx)
}

func (m *Manager) changed() {
	if m.OnChange != nil {
		m.OnChange(m.PublicURL())
	}
}

const ProbePath = "/.well-known/heron/probe/"

func (m *Manager) Probe(nonce string) bool {
	m.probeMu.Lock()
	defer m.probeMu.Unlock()
	for k, at := range m.probes {
		if time.Since(at) > time.Minute {
			delete(m.probes, k)
		}
	}
	_, ok := m.probes[nonce]
	return ok
}

func (m *Manager) reachable(ctx context.Context, base string) error {
	nonce := rand.Text()
	m.probeMu.Lock()
	m.probes[nonce] = time.Now()
	m.probeMu.Unlock()
	defer func() {
		m.probeMu.Lock()
		delete(m.probes, nonce)
		m.probeMu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+ProbePath+nonce, nil)
	if err != nil {
		return err
	}
	resp, err := m.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != nonce {
		return fmt.Errorf("the domain answered, but not from this heron (status %d)", resp.StatusCode)
	}
	return nil
}

func (m *Manager) explain(ctx context.Context, domain string, cause error) string {
	addrs, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil || len(addrs) == 0 {
		return domain + " does not resolve yet. Add an A record pointing to this server and try again"
	}
	return fmt.Sprintf("%s points to %s, but heron could not reach itself through it (%v). Check the A record and that ports 80 and 443 are open", domain, strings.Join(addrs, ", "), cause)
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
