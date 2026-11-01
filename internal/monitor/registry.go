package monitor

import (
	"context"
	"fmt"
	"time"
)

type Registry map[Kind]Checker

func NewRegistry(userAgent string, privilegedICMP bool) Registry {
	return Registry{
		KindHTTP:      NewHTTPChecker(userAgent),
		KindTCP:       TCPChecker{},
		KindICMP:      ICMPChecker{Privileged: privilegedICMP},
		KindDNS:       DNSChecker{},
		KindRedis:     RedisChecker{},
		KindPostgres:  PostgresChecker{},
		KindSMTP:      SMTPChecker{},
		KindIMAP:      IMAPChecker{},
		KindDocker:    DockerChecker{},
		KindGRPC:      GRPCChecker{},
		KindMinecraft: MinecraftChecker{},
		KindSteam:     SteamChecker{},
	}
}

func (r Registry) Check(ctx context.Context, m *Monitor) Result {
	checker, ok := r[m.Kind]
	if !ok {
		return Down(0, "no checker for kind %q", m.Kind)
	}

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	started := time.Now()
	res := checker.Check(ctx, m)
	if res.At.IsZero() {
		res.At = started
	}
	if res.Status == StatusDown && ctx.Err() != nil && res.Message == "" {
		res.Message = fmt.Sprintf("timed out after %s", m.Timeout)
	}
	return res.withinBudget(m)
}
