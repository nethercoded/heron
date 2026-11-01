<p align="center">
  <img src="web/static/favicon.svg" width="72" alt="heron">
</p>

<h1 align="center">heron</h1>

<p align="center">
  A small, fast, self-hosted status page and uptime monitor.<br>
  One binary. One SQLite file. Nothing else to run.
</p>

<p align="center">
  <a href="https://github.com/nethercode/heron/actions/workflows/ci.yml"><img src="https://github.com/nethercode/heron/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="https://github.com/nethercode/heron/releases"><img src="https://img.shields.io/github/v/release/nethercode/heron" alt="release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/nethercode/heron" alt="license"></a>
</p>

---

heron watches your websites, APIs, servers and cron jobs, keeps the history, and shows it on a clean public status page. When something breaks it opens an incident and pings you in Telegram or any webhook. When it recovers, it closes the incident and tells you how long it was down.

## Features

- **Checks**: HTTP(S) with status codes, keywords, headers, body and TLS certificate expiry; TCP ports; ICMP ping; DNS records; push heartbeats for cron jobs and backups
- **Public page** with 90-day uptime bars, response time charts, incident history and live updates over Server-Sent Events
- **Incidents** opened and resolved automatically, plus manual incidents and maintenance notes with a timeline of updates
- **Alerts** to Telegram (including forum topics) and signed webhooks, with retries and backoff
- **Admin panel** for monitors, incidents, notification channels and page settings
- Retries before alerting, degraded thresholds for slow responses, private monitors, groups
- Dark mode, mobile-friendly, no tracking, no external requests from the browser
- Single static binary with the UI embedded, pure Go SQLite (no CGO), graceful shutdown, hardened systemd unit

## Install

### One-liner (Linux with systemd)

```sh
curl -fsSL https://raw.githubusercontent.com/nethercode/heron/main/deploy/install.sh | sudo sh
```

The script downloads the latest release, verifies its checksum, creates a `heron` system user, writes `/etc/heron/heron.toml` and starts the service. Open `http://your-server:8080/admin` and create the administrator account.

### Docker

```sh
docker run -d --name heron \
  -p 8080:8080 \
  -v heron-data:/data \
  --cap-add NET_RAW \
  -e HERON_PUBLIC_URL=https://status.example.com \
  ghcr.io/nethercode/heron:latest
```

Or use the bundled [`compose.yaml`](compose.yaml).

### From source

Requires Go 1.27+, Node 24+ and pnpm.

```sh
git clone https://github.com/nethercode/heron && cd heron
task build        # or: (cd web && pnpm install && pnpm build) && go build ./cmd/heron
./heron serve
```

## Configuration

heron reads `heron.toml` from the working directory, from `$HERON_CONFIG`, or from `-config`. Every key can also be set through an environment variable.

| Key               | Env                     | Default   | Description                                                    |
| ----------------- | ----------------------- | --------- | -------------------------------------------------------------- |
| `listen`          | `HERON_LISTEN`          | `:8080`   | Address to listen on                                           |
| `data_dir`        | `HERON_DATA_DIR`        | `./data`  | Where the SQLite database lives                                |
| `public_url`      | `HERON_PUBLIC_URL`      |           | Public address, used in alerts and to enable secure cookies    |
| `timezone`        | `HERON_TIMEZONE`        | `UTC`     | Timezone for daily uptime bars                                 |
| `retention_days`  | `HERON_RETENTION_DAYS`  | `7`       | How long raw check results are kept; hourly rollups stay a year |
| `trusted_proxies` | `HERON_TRUSTED_PROXIES` |           | Proxies allowed to set `X-Forwarded-For`                       |
| `icmp_privileged` | `HERON_ICMP_PRIVILEGED` | `false`   | Use raw sockets for ping (needs `CAP_NET_RAW`)                 |
| `log_level`       | `HERON_LOG_LEVEL`       | `info`    | `debug`, `info`, `warn` or `error`                             |
| `log_format`      | `HERON_LOG_FORMAT`      | `text`    | `text` or `json`                                               |

See [`heron.example.toml`](heron.example.toml).

### Behind a reverse proxy

heron streams live updates, so turn off response buffering for `/api/stream`:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_buffering off;
}
```

With Caddy it is just `reverse_proxy 127.0.0.1:8080`.

## Push monitors

Create a monitor of type **Push** and heron gives you a URL. Call it from your job:

```sh
0 3 * * * /usr/local/bin/backup.sh && curl -fsS -m 10 --retry 3 https://status.example.com/api/push/<token>
```

If no heartbeat arrives within the interval plus the grace period, the monitor goes down. You can also report failures and timings explicitly:

```
/api/push/<token>?status=down&msg=disk%20full
/api/push/<token>?status=up&ping=532
```

## Webhooks

Webhook channels receive a JSON `POST` on every state change:

```json
{
  "event": "monitor.down",
  "at": "2026-11-02T03:14:00Z",
  "status": "down",
  "reason": "unexpected status 502 Bad Gateway",
  "link": "https://status.example.com/monitor/api",
  "monitor": { "id": 3, "slug": "api", "name": "API", "kind": "http", "target": "https://api.example.com/health" }
}
```

When a signing secret is set, every request carries `X-Heron-Timestamp` and `X-Heron-Signature: sha256=<hex>`, where the signature is an HMAC-SHA256 of `timestamp + "." + body`.

## CLI

```
heron serve                 start the server
heron user create -u name   create an administrator (password from stdin or prompt)
heron user passwd -u name   reset a password and sign out all sessions
heron version
```

## Development

```sh
task dev     # Go backend on :8080 and Vite on :5173 with a proxy to it
task test    # go test -race ./...
task lint    # golangci-lint, svelte-check, biome
```

The backend lives in `internal/`: `monitor` (checkers), `scheduler` (one goroutine per monitor, retries, push deadlines), `engine` (results, incidents, alerts, live events), `store` (SQLite), `notify`, `auth` and `httpapi`. The frontend is a SvelteKit app in `web/`, built to static files and embedded into the binary.

## License

[MIT](LICENSE)
