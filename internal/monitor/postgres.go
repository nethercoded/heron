package monitor

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PostgresChecker struct{}

func (PostgresChecker) Check(ctx context.Context, m *Monitor) Result {
	cfg, err := pgx.ParseConfig(m.Target)
	if err != nil {
		return Down(0, "invalid connection url")
	}
	cfg.RuntimeParams["application_name"] = "heron"

	started := time.Now()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return Down(time.Since(started), "%s", redactPassword(err.Error(), cfg.Password))
	}
	defer conn.Close(context.WithoutCancel(ctx))

	var version string
	var recovery bool
	err = conn.QueryRow(ctx, "select current_setting('server_version'), pg_is_in_recovery()").Scan(&version, &recovery)
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "query: %s", redactPassword(err.Error(), cfg.Password))
	}

	role := "primary"
	if recovery {
		role = "replica"
	}
	return Up(latency, "postgres %s, %s", strings.Fields(version)[0], role)
}

func redactPassword(msg, password string) string {
	if password == "" {
		return msg
	}
	return strings.ReplaceAll(msg, password, "***")
}
