package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

type GRPCChecker struct{}

var servingStatus = map[uint64]string{
	0: "UNKNOWN",
	1: "SERVING",
	2: "NOT_SERVING",
	3: "SERVICE_UNKNOWN",
}

func (GRPCChecker) Check(ctx context.Context, m *Monitor) Result {
	transport := &http2.Transport{}
	scheme := "https"
	if m.Settings.TLS {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: m.Settings.IgnoreTLS}
	} else {
		scheme = "http"
		transport.AllowHTTP = true
		transport.DialTLSContext = func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}
	}
	defer transport.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scheme+"://"+m.Target+"/grpc.health.v1.Health/Check", bytes.NewReader(healthRequest(m.Settings.Service)))
	if err != nil {
		return Down(0, "%v", err)
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")

	started := time.Now()
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "read response: %v", err)
	}
	if code := grpcStatus(resp); code != "0" {
		return Down(latency, "grpc status %s: %s", code, grpcMessage(resp))
	}

	status, err := healthStatus(body)
	if err != nil {
		return Down(latency, "%v", err)
	}
	if status != 1 {
		return Down(latency, "health %s", servingStatus[status])
	}
	return Up(latency, "SERVING")
}

func healthRequest(service string) []byte {
	var msg []byte
	if service != "" {
		msg = append(msg, 0x0a)
		msg = binary.AppendUvarint(msg, uint64(len(service)))
		msg = append(msg, service...)
	}
	frame := make([]byte, 5, 5+len(msg))
	binary.BigEndian.PutUint32(frame[1:], uint32(len(msg)))
	return append(frame, msg...)
}

func healthStatus(body []byte) (uint64, error) {
	if len(body) < 5 {
		return 0, fmt.Errorf("empty health response")
	}
	if body[0] != 0 {
		return 0, fmt.Errorf("compressed responses are not supported")
	}
	size := binary.BigEndian.Uint32(body[1:5])
	msg := body[5:]
	if uint32(len(msg)) < size {
		return 0, fmt.Errorf("truncated health response")
	}
	msg = msg[:size]
	for len(msg) > 0 {
		key, n := binary.Uvarint(msg)
		if n <= 0 {
			return 0, fmt.Errorf("malformed health response")
		}
		msg = msg[n:]
		if key == 0x08 {
			value, n := binary.Uvarint(msg)
			if n <= 0 {
				return 0, fmt.Errorf("malformed health response")
			}
			return value, nil
		}
		return 0, fmt.Errorf("unexpected field in health response")
	}
	return 0, nil
}

func grpcStatus(resp *http.Response) string {
	if v := resp.Trailer.Get("Grpc-Status"); v != "" {
		return v
	}
	if v := resp.Header.Get("Grpc-Status"); v != "" {
		return v
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("http %d", resp.StatusCode)
	}
	return "0"
}

func grpcMessage(resp *http.Response) string {
	msg := resp.Trailer.Get("Grpc-Message")
	if msg == "" {
		msg = resp.Header.Get("Grpc-Message")
	}
	return strings.TrimSpace(msg)
}
