package monitor

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Kind string

const (
	KindHTTP Kind = "http"
	KindTCP  Kind = "tcp"
	KindICMP Kind = "icmp"
	KindDNS  Kind = "dns"
	KindPush Kind = "push"
)

var Kinds = []Kind{KindHTTP, KindTCP, KindICMP, KindDNS, KindPush}

type Status string

const (
	StatusPending  Status = "pending"
	StatusUp       Status = "up"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusPaused   Status = "paused"
)

func (s Status) Healthy() bool {
	return s == StatusUp || s == StatusDegraded
}

type Monitor struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	Group       string
	Kind        Kind
	Target      string
	Interval    time.Duration
	Timeout     time.Duration
	Retries     int
	Public      bool
	Paused      bool
	Position    int
	Settings    Settings
	PushToken   string
	ChannelIDs  []int64
	State       State
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type State struct {
	Status    Status
	Since     time.Time
	CheckedAt time.Time
	Latency   time.Duration
	Message   string
}

type Settings struct {
	Method          string            `json:"method,omitzero"`
	Headers         map[string]string `json:"headers,omitzero"`
	Body            string            `json:"body,omitzero"`
	AcceptedStatus  []int             `json:"accepted_status,omitzero"`
	Keyword         string            `json:"keyword,omitzero"`
	KeywordAbsent   bool              `json:"keyword_absent,omitzero"`
	MaxRedirects    int               `json:"max_redirects,omitzero"`
	IgnoreTLS       bool              `json:"ignore_tls,omitzero"`
	CertExpiryDays  int               `json:"cert_expiry_days,omitzero"`
	DegradedAfterMS int               `json:"degraded_after_ms,omitzero"`
	RecordType      string            `json:"record_type,omitzero"`
	Resolver        string            `json:"resolver,omitzero"`
	Expect          string            `json:"expect,omitzero"`
	Packets         int               `json:"packets,omitzero"`
	GraceSeconds    int               `json:"grace_seconds,omitzero"`
}

type Result struct {
	Status  Status
	Latency time.Duration
	Message string
	At      time.Time
}

type Checker interface {
	Check(ctx context.Context, m *Monitor) Result
}

type CheckerFunc func(ctx context.Context, m *Monitor) Result

func (f CheckerFunc) Check(ctx context.Context, m *Monitor) Result {
	return f(ctx, m)
}

func Up(latency time.Duration, format string, args ...any) Result {
	return Result{Status: StatusUp, Latency: latency, Message: fmt.Sprintf(format, args...)}
}

func Degraded(latency time.Duration, format string, args ...any) Result {
	return Result{Status: StatusDegraded, Latency: latency, Message: fmt.Sprintf(format, args...)}
}

func Down(latency time.Duration, format string, args ...any) Result {
	return Result{Status: StatusDown, Latency: latency, Message: fmt.Sprintf(format, args...)}
}

func (r Result) withinBudget(m *Monitor) Result {
	if r.Status == StatusUp && m.Settings.DegradedAfterMS > 0 && r.Latency > time.Duration(m.Settings.DegradedAfterMS)*time.Millisecond {
		r.Status = StatusDegraded
		r.Message = fmt.Sprintf("slow response: %s", r.Latency.Round(time.Millisecond))
	}
	return r
}

const (
	MinInterval = 10 * time.Second
	MaxInterval = 24 * time.Hour
	MaxRetries  = 10
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	recordTypes = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT"}
	methods     = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
)

type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Message
}

func fieldErr(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func (m *Monitor) Normalize() {
	m.Name = strings.TrimSpace(m.Name)
	m.Group = strings.TrimSpace(m.Group)
	m.Description = strings.TrimSpace(m.Description)
	m.Target = strings.TrimSpace(m.Target)
	m.Slug = strings.TrimSpace(m.Slug)
	if m.Slug == "" {
		m.Slug = Slugify(m.Name)
	}
	if m.Interval == 0 {
		m.Interval = time.Minute
	}
	if m.Timeout == 0 {
		m.Timeout = min(10*time.Second, m.Interval)
	}

	s := &m.Settings
	switch m.Kind {
	case KindHTTP:
		s.Method = strings.ToUpper(cmp.Or(s.Method, "GET"))
		if s.CertExpiryDays == 0 {
			s.CertExpiryDays = 14
		}
	case KindDNS:
		s.RecordType = strings.ToUpper(cmp.Or(s.RecordType, "A"))
	case KindICMP:
		if s.Packets == 0 {
			s.Packets = 3
		}
	case KindPush:
		m.Target = ""
		if s.GraceSeconds == 0 {
			s.GraceSeconds = int(m.Interval.Seconds() / 2)
		}
	}
}

func (m *Monitor) Validate() error {
	var errs []error

	if m.Name == "" {
		errs = append(errs, fieldErr("name", "is required"))
	}
	if !slugPattern.MatchString(m.Slug) {
		errs = append(errs, fieldErr("slug", "must contain lowercase letters, digits and dashes"))
	}
	if !slices.Contains(Kinds, m.Kind) {
		errs = append(errs, fieldErr("kind", "unknown kind %q", m.Kind))
	}
	if m.Interval < MinInterval || m.Interval > MaxInterval {
		errs = append(errs, fieldErr("interval", "must be between %s and %s", MinInterval, MaxInterval))
	}
	if m.Timeout <= 0 || m.Timeout > m.Interval {
		errs = append(errs, fieldErr("timeout", "must be positive and not exceed the interval"))
	}
	if m.Retries < 0 || m.Retries > MaxRetries {
		errs = append(errs, fieldErr("retries", "must be between 0 and %d", MaxRetries))
	}
	if m.Settings.DegradedAfterMS < 0 {
		errs = append(errs, fieldErr("degraded_after_ms", "must not be negative"))
	}

	switch m.Kind {
	case KindHTTP:
		errs = append(errs, m.validateHTTP()...)
	case KindTCP:
		if _, _, err := net.SplitHostPort(m.Target); err != nil {
			errs = append(errs, fieldErr("target", "must be host:port"))
		}
	case KindICMP:
		if !validHost(m.Target) {
			errs = append(errs, fieldErr("target", "must be a hostname or ip address"))
		}
		if m.Settings.Packets < 1 || m.Settings.Packets > 10 {
			errs = append(errs, fieldErr("packets", "must be between 1 and 10"))
		}
	case KindDNS:
		if !validHost(m.Target) || net.ParseIP(m.Target) != nil {
			errs = append(errs, fieldErr("target", "must be a domain name"))
		}
		if !slices.Contains(recordTypes, m.Settings.RecordType) {
			errs = append(errs, fieldErr("record_type", "must be one of %s", strings.Join(recordTypes, ", ")))
		}
		if r := m.Settings.Resolver; r != "" {
			if _, _, err := net.SplitHostPort(r); err != nil && net.ParseIP(r) == nil {
				errs = append(errs, fieldErr("resolver", "must be an ip address or host:port"))
			}
		}
	case KindPush:
		if m.Settings.GraceSeconds < 0 {
			errs = append(errs, fieldErr("grace_seconds", "must not be negative"))
		}
	}

	return errors.Join(errs...)
}

func (m *Monitor) validateHTTP() []error {
	var errs []error
	u, err := url.Parse(m.Target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fieldErr("target", "must be an http or https url"))
	}
	if !slices.Contains(methods, m.Settings.Method) {
		errs = append(errs, fieldErr("method", "unsupported method %q", m.Settings.Method))
	}
	for _, code := range m.Settings.AcceptedStatus {
		if code < 100 || code > 599 {
			errs = append(errs, fieldErr("accepted_status", "%d is not a valid status code", code))
			break
		}
	}
	if m.Settings.MaxRedirects < 0 || m.Settings.MaxRedirects > 20 {
		errs = append(errs, fieldErr("max_redirects", "must be between 0 and 20"))
	}
	return errs
}

func (m *Monitor) Deadline() time.Duration {
	return m.Interval + time.Duration(m.Settings.GraceSeconds)*time.Second
}

func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func validHost(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	return s != "" && len(s) <= 253 && !strings.ContainsAny(s, "/:@ ")
}
