package monitor

import (
	"context"
	"net"
	"time"
)

type TCPChecker struct{}

func (TCPChecker) Check(ctx context.Context, m *Monitor) Result {
	var d net.Dialer
	started := time.Now()
	conn, err := d.DialContext(ctx, "tcp", m.Target)
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "%s", describeNetError(err))
	}
	conn.Close()
	return Up(latency, "port open")
}
