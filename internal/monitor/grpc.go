package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type GRPCChecker struct{}

var servingStatus = map[uint64]string{
	0: "UNKNOWN",
	1: "SERVING",
	2: "NOT_SERVING",
	3: "SERVICE_UNKNOWN",
}

func (GRPCChecker) Check(ctx context.Context, m *Monitor) Result {
	var protocols http.Protocols
	scheme := "https"
	if m.Settings.TLS {
		protocols.SetHTTP2(true)
	} else {
		scheme = "http"
		protocols.SetUnencryptedHTTP2(true)
	}
	transport := &http.Transport{
		Protocols:         &protocols,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: m.Settings.IgnoreTLS},
		DisableKeepAlives: true,
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

var errMalformedHealth = errors.New("malformed health response")

func healthStatus(body []byte) (uint64, error) {
	if len(body) < 5 {
		return 0, errors.New("empty health response")
	}
	if body[0] != 0 {
		return 0, errors.New("compressed responses are not supported")
	}
	size := binary.BigEndian.Uint32(body[1:5])
	msg := body[5:]
	if uint32(len(msg)) < size {
		return 0, errors.New("truncated health response")
	}
	msg = msg[:size]
	if len(msg) == 0 {
		return 0, nil
	}

	key, n := binary.Uvarint(msg)
	if n <= 0 || key != 0x08 {
		return 0, errMalformedHealth
	}
	value, n := binary.Uvarint(msg[n:])
	if n <= 0 {
		return 0, errMalformedHealth
	}
	return value, nil
}

func grpcStatus(resp *http.Response) string {
	if v := resp.Trailer.Get("Grpc-Status"); v != "" {
		return v
	}
	if v := resp.Header.Get("Grpc-Status"); v != "" {
		return v
	}
	if resp.StatusCode != http.StatusOK {
		return "http " + strconv.Itoa(resp.StatusCode)
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
