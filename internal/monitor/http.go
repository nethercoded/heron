package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

const maxBodyBytes = 1 << 20

type HTTPChecker struct {
	userAgent string
	strict    *http.Transport
	insecure  *http.Transport
}

func NewHTTPChecker(userAgent string) *HTTPChecker {
	return &HTTPChecker{
		userAgent: userAgent,
		strict:    newTransport(false),
		insecure:  newTransport(true),
	}
}

func newTransport(insecure bool) *http.Transport {
	return &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{KeepAlive: -1}).DialContext,
		TLSClientConfig:        &tls.Config{InsecureSkipVerify: insecure, MinVersion: tls.VersionTLS12},
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
}

func (c *HTTPChecker) Check(ctx context.Context, m *Monitor) Result {
	s := m.Settings
	req, err := http.NewRequestWithContext(ctx, s.Method, m.Target, bodyReader(s.Body))
	if err != nil {
		return Down(0, "build request: %v", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	if s.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Transport:     c.strict,
		CheckRedirect: redirectPolicy(s.MaxRedirects),
	}
	if s.IgnoreTLS {
		client.Transport = c.insecure
	}

	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "read body: %v", err)
	}

	if !accepted(resp.StatusCode, s.AcceptedStatus) {
		return Down(latency, "unexpected status %s", resp.Status)
	}

	if s.Keyword != "" {
		found := bytes.Contains(body, []byte(s.Keyword))
		switch {
		case !found && !s.KeywordAbsent:
			return Down(latency, "keyword %q not found", s.Keyword)
		case found && s.KeywordAbsent:
			return Down(latency, "keyword %q is present", s.Keyword)
		}
	}

	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 && !s.IgnoreTLS {
		expires := resp.TLS.PeerCertificates[0].NotAfter
		left := time.Until(expires)
		if left < time.Duration(s.CertExpiryDays)*24*time.Hour {
			return Degraded(latency, "certificate expires in %d days", int(left.Hours()/24))
		}
	}

	return Up(latency, "%s", resp.Status)
}

func bodyReader(body string) io.Reader {
	if body == "" {
		return http.NoBody
	}
	return strings.NewReader(body)
}

func redirectPolicy(limit int) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if len(via) > limit {
			return http.ErrUseLastResponse
		}
		return nil
	}
}

func accepted(code int, list []int) bool {
	if len(list) == 0 {
		return code >= 200 && code < 400
	}
	return slices.Contains(list, code)
}

func describeNetError(err error) string {
	var (
		dnsErr  *net.DNSError
		certErr *tls.CertificateVerificationError
		opErr   *net.OpError
	)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case errors.As(err, &dnsErr):
		return "dns lookup failed for " + dnsErr.Name
	case errors.As(err, &certErr):
		return fmt.Sprintf("tls verification failed: %v", certErr.Err)
	case errors.As(err, &opErr) && opErr.Op == "dial":
		return fmt.Sprintf("connection failed: %v", opErr.Err)
	}
	return err.Error()
}
