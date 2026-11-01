package monitor

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"time"
)

func dial(ctx context.Context, network, address string, secure bool) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	if !secure {
		return conn, nil
	}
	host, _, _ := net.SplitHostPort(address)
	tc := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err := tc.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return tc, nil
}

type session struct {
	conn net.Conn
	r    *bufio.Reader
}

func open(ctx context.Context, address string, secure bool) (*session, time.Time, error) {
	started := time.Now()
	conn, err := dial(ctx, "tcp", address, secure)
	if err != nil {
		return nil, started, err
	}
	return &session{conn: conn, r: bufio.NewReader(conn)}, started, nil
}

func (s *session) line() (string, error) {
	line, err := s.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return trimEOL(line), nil
}

func (s *session) send(text string) error {
	_, err := s.conn.Write([]byte(text + "\r\n"))
	return err
}

func (s *session) Close() error {
	return s.conn.Close()
}

func trimEOL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
