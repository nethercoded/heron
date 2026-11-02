package monitor

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"
)

type SMTPChecker struct{}

func (SMTPChecker) Check(ctx context.Context, m *Monitor) Result {
	s, started, err := open(ctx, m.Target, m.Settings.TLS)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer s.Close()

	banner, err := s.smtpReply()
	if err != nil {
		return Down(time.Since(started), "greeting: %v", err)
	}
	if !strings.HasPrefix(banner, "220") {
		return Down(time.Since(started), "unexpected greeting %q", banner)
	}

	if err := s.send("EHLO heron.local"); err != nil {
		return Down(time.Since(started), "ehlo: %v", err)
	}
	ehlo, err := s.smtpReply()
	if err != nil || !strings.HasPrefix(ehlo, "250") {
		return Down(time.Since(started), "ehlo rejected: %s", firstLine(ehlo, err))
	}

	if m.Settings.StartTLS && !m.Settings.TLS {
		if !strings.Contains(strings.ToUpper(ehlo), "STARTTLS") {
			return Down(time.Since(started), "server does not offer STARTTLS")
		}
		if err := s.send("STARTTLS"); err != nil {
			return Down(time.Since(started), "starttls: %v", err)
		}
		if reply, err := s.smtpReply(); err != nil || !strings.HasPrefix(reply, "220") {
			return Down(time.Since(started), "starttls rejected: %s", firstLine(reply, err))
		}
		if err := s.upgrade(ctx, m.Target); err != nil {
			return Down(time.Since(started), "tls handshake: %v", err)
		}
	}

	latency := time.Since(started)
	_ = s.send("QUIT")
	return Up(latency, "%s", strings.TrimSpace(strings.TrimPrefix(firstLine(banner, nil), "220")))
}

func (s *session) smtpReply() (string, error) {
	var lines []string
	for {
		line, err := s.line()
		if err != nil {
			return strings.Join(lines, "\n"), err
		}
		lines = append(lines, line)
		if len(line) < 4 || line[3] != '-' {
			return strings.Join(lines, "\n"), nil
		}
	}
}

func (s *session) upgrade(ctx context.Context, address string) error {
	host, _, _ := net.SplitHostPort(address)
	tc := tls.Client(s.conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err := tc.HandshakeContext(ctx); err != nil {
		return err
	}
	s.conn = tc
	s.r.Reset(tc)
	return nil
}

type IMAPChecker struct{}

func (IMAPChecker) Check(ctx context.Context, m *Monitor) Result {
	s, started, err := open(ctx, m.Target, m.Settings.TLS)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer s.Close()

	greeting, err := s.line()
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "greeting: %v", err)
	}
	switch {
	case strings.HasPrefix(greeting, "* OK"), strings.HasPrefix(greeting, "* PREAUTH"):
	case strings.HasPrefix(greeting, "* BYE"):
		return Down(latency, "server refused: %s", greeting)
	default:
		return Down(latency, "unexpected greeting %q", greeting)
	}

	_ = s.send("h1 LOGOUT")
	return Up(latency, "%s", strings.TrimSpace(strings.TrimPrefix(greeting, "* OK")))
}

func firstLine(s string, err error) string {
	if err != nil {
		return err.Error()
	}
	line, _, _ := strings.Cut(s, "\n")
	return line
}
