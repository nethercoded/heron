package monitor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type RedisChecker struct{}

func (RedisChecker) Check(ctx context.Context, m *Monitor) Result {
	s, started, err := open(ctx, m.Target, m.Settings.TLS)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer s.Close()

	if m.Settings.Password != "" {
		reply, err := s.command("AUTH", m.Settings.Password)
		if err != nil {
			return Down(time.Since(started), "auth: %v", err)
		}
		if reply != "OK" {
			return Down(time.Since(started), "auth rejected: %s", reply)
		}
	}

	reply, err := s.command("PING")
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "ping: %v", err)
	}
	if reply != "PONG" {
		return Down(latency, "unexpected reply %q", reply)
	}

	role, err := s.command("ROLE")
	if err == nil && role != "" {
		return Up(latency, "PONG, role %s", role)
	}
	return Up(latency, "PONG")
}

func (s *session) command(args ...string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := s.conn.Write([]byte(b.String())); err != nil {
		return "", err
	}
	return s.reply()
}

func (s *session) reply() (string, error) {
	line, err := s.line()
	if err != nil {
		return "", err
	}
	if line == "" {
		return "", fmt.Errorf("empty reply")
	}
	switch line[0] {
	case '+', ':':
		return line[1:], nil
	case '-':
		return "", fmt.Errorf("%s", line[1:])
	case '$':
		var n int
		if _, err := fmt.Sscanf(line[1:], "%d", &n); err != nil || n < 0 {
			return "", nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(s.r, buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	case '*':
		var n int
		fmt.Sscanf(line[1:], "%d", &n)
		var first string
		for i := range n {
			v, err := s.reply()
			if err != nil {
				return "", err
			}
			if i == 0 {
				first = v
			}
		}
		return first, nil
	}
	return "", fmt.Errorf("unexpected reply %q", line)
}
