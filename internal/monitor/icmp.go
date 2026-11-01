package monitor

import (
	"context"
	"runtime"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type ICMPChecker struct {
	Privileged bool
}

func (c ICMPChecker) Check(ctx context.Context, m *Monitor) Result {
	pinger, err := probing.NewPinger(m.Target)
	if err != nil {
		return Down(0, "resolve %s: %v", m.Target, err)
	}
	pinger.SetPrivileged(c.Privileged || runtime.GOOS == "windows")
	pinger.Count = m.Settings.Packets
	pinger.Interval = 200 * time.Millisecond
	pinger.Timeout = m.Timeout

	if err := pinger.RunWithContext(ctx); err != nil {
		return Down(0, "ping: %v", err)
	}

	stats := pinger.Statistics()
	switch {
	case stats.PacketsRecv == 0:
		return Down(0, "%d of %d packets lost", stats.PacketsSent, stats.PacketsSent)
	case stats.PacketLoss > 0:
		return Degraded(stats.AvgRtt, "%.0f%% packet loss", stats.PacketLoss)
	}
	return Up(stats.AvgRtt, "%d packets, avg %s", stats.PacketsRecv, stats.AvgRtt.Round(time.Microsecond*100))
}
