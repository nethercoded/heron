package kuma

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
)

var ErrFormat = errors.New("this is neither an Uptime Kuma backup (.json) nor its database (kuma.db)")

type row map[string]any

func (r row) get(names ...string) any {
	for _, name := range names {
		if v, ok := r[key(name)]; ok && v != nil {
			return v
		}
	}
	return nil
}

func (r row) str(names ...string) string {
	switch v := r.get(names...).(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	}
	return ""
}

func (r row) num(names ...string) int64 {
	switch v := r.get(names...).(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case bool:
		if v {
			return 1
		}
	case string, []byte:
		n, _ := strconv.ParseFloat(r.str(names...), 64)
		return int64(n)
	}
	return 0
}

func (r row) flag(names ...string) bool {
	if v, ok := r.get(names...).(bool); ok {
		return v
	}
	return r.num(names...) != 0
}

func key(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

func normalize(in map[string]any) row {
	out := make(row, len(in))
	for k, v := range in {
		out[key(k)] = v
	}
	return out
}

type Source struct {
	Monitors      []row
	Notifications []row
	Links         map[int64][]int64
	Published     map[int64]bool
}

func Read(ctx context.Context, path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	f.Close()
	if bytes.HasPrefix(head[:n], []byte("SQLite format 3")) {
		return readDatabase(ctx, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJSON(raw)
}

func ParseJSON(raw []byte) (*Source, error) {
	var export struct {
		Monitors      []map[string]any `json:"monitorList"`
		Notifications []map[string]any `json:"notificationList"`
	}
	if err := json.Unmarshal(raw, &export); err != nil || (export.Monitors == nil && export.Notifications == nil) {
		return nil, ErrFormat
	}
	src := &Source{Links: map[int64][]int64{}}
	for _, m := range export.Monitors {
		r := normalize(m)
		src.Monitors = append(src.Monitors, r)
		if ids, ok := r.get("notificationIDList").(map[string]any); ok {
			for id, on := range ids {
				n, err := strconv.ParseInt(id, 10, 64)
				if on == true && err == nil {
					src.Links[r.num("id")] = append(src.Links[r.num("id")], n)
				}
			}
		}
	}
	for _, n := range export.Notifications {
		src.Notifications = append(src.Notifications, normalize(n))
	}
	return src, nil
}

func readDatabase(ctx context.Context, path string) (*Source, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	src := &Source{Links: map[int64][]int64{}}
	if src.Monitors, err = query(ctx, db, `select * from monitor order by id`); err != nil {
		return nil, ErrFormat
	}
	src.Notifications, _ = query(ctx, db, `select * from notification order by id`)
	links, _ := query(ctx, db, `select monitor_id, notification_id from monitor_notification`)
	for _, l := range links {
		src.Links[l.num("monitor_id")] = append(src.Links[l.num("monitor_id")], l.num("notification_id"))
	}
	if published, err := query(ctx, db, `select distinct mg.monitor_id from monitor_group mg join "group" g on g.id = mg.group_id where g.status_page_id is not null`); err == nil && len(published) > 0 {
		src.Published = map[int64]bool{}
		for _, p := range published {
			src.Published[p.num("monitor_id")] = true
		}
	}
	return src, nil
}

func query(ctx context.Context, db *sql.DB, q string) ([]row, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []row
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := make(row, len(cols))
		for i, c := range cols {
			r[key(c)] = values[i]
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Skipped struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type Plan struct {
	Monitors []*monitor.Monitor
	Channels []*notify.Channel
	Skipped  []Skipped
	Notes    map[string][]string

	channelOf map[int64]int
	linksOf   map[*monitor.Monitor][]int64
}

func (s *Source) Plan() *Plan {
	p := &Plan{Notes: map[string][]string{}, channelOf: map[int64]int{}, linksOf: map[*monitor.Monitor][]int64{}}

	for _, n := range s.Notifications {
		ch, err := channel(n)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{Name: n.str("name"), Kind: "notification", Reason: err.Error()})
			continue
		}
		p.channelOf[n.num("id")] = len(p.Channels)
		p.Channels = append(p.Channels, ch)
	}

	groups := map[int64]string{}
	for _, m := range s.Monitors {
		if m.str("type") == "group" {
			groups[m.num("id")] = m.str("name")
		}
	}

	for _, r := range s.Monitors {
		if r.str("type") == "group" {
			continue
		}
		m, notes, err := convert(r)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{Name: r.str("name"), Kind: r.str("type"), Reason: err.Error()})
			continue
		}
		m.Group = groups[r.num("parent")]
		m.Public = s.Published == nil || s.Published[r.num("id")]
		m.Normalize()
		if err := m.Validate(); err != nil {
			p.Skipped = append(p.Skipped, Skipped{Name: r.str("name"), Kind: r.str("type"), Reason: err.Error()})
			continue
		}
		if len(notes) > 0 {
			p.Notes[m.Name] = notes
		}
		p.linksOf[m] = s.Links[r.num("id")]
		p.Monitors = append(p.Monitors, m)
	}
	return p
}

func channel(n row) (*notify.Channel, error) {
	var cfg row
	switch v := n.get("config").(type) {
	case string:
		var m map[string]any
		if json.Unmarshal([]byte(v), &m) != nil {
			return nil, errors.New("unreadable settings")
		}
		cfg = normalize(m)
	case []byte:
		var m map[string]any
		if json.Unmarshal(v, &m) != nil {
			return nil, errors.New("unreadable settings")
		}
		cfg = normalize(m)
	case map[string]any:
		cfg = normalize(v)
	default:
		return nil, errors.New("no settings")
	}

	ch := &notify.Channel{Name: cmp.Or(n.str("name"), cfg.str("name")), Enabled: n.get("active") == nil || n.flag("active")}
	kind := cfg.str("type")
	switch kind {
	case "telegram":
		ch.Kind = notify.KindTelegram
		ch.Config.BotToken = cfg.str("telegramBotToken")
		ch.Config.ChatID = cfg.str("telegramChatID")
		ch.Config.ThreadID = cfg.num("telegramMessageThreadID")
	case "discord":
		ch.Kind = notify.KindDiscord
		ch.Config.URL = cfg.str("discordWebhookUrl")
	case "webhook":
		ch.Kind = notify.KindWebhook
		ch.Config.URL = cfg.str("webhookURL")
		if raw := cfg.str("webhookAdditionalHeaders"); raw != "" {
			var headers map[string]any
			if json.Unmarshal([]byte(raw), &headers) == nil {
				ch.Config.Headers = map[string]string{}
				for k, v := range headers {
					ch.Config.Headers[k] = fmt.Sprint(v)
				}
			}
		}
	default:
		return nil, fmt.Errorf("heron does not send to %s, only Telegram, Discord and webhooks", cmp.Or(kind, "this service"))
	}
	if err := ch.Validate(); err != nil {
		return nil, err
	}
	return ch, nil
}

func convert(r row) (*monitor.Monitor, []string, error) {
	m := &monitor.Monitor{
		Name:        r.str("name"),
		Description: r.str("description"),
		Paused:      r.get("active") != nil && !r.flag("active"),
		Retries:     int(min(max(r.num("maxretries"), 0), monitor.MaxRetries)),
	}
	interval := time.Duration(r.num("interval")) * time.Second
	m.Interval = min(max(interval, monitor.MinInterval), monitor.MaxInterval)
	if t := time.Duration(r.num("timeout")) * time.Second; t > 0 {
		m.Timeout = min(t, m.Interval)
	}

	var notes []string
	if interval > 0 && interval != m.Interval {
		notes = append(notes, fmt.Sprintf("interval changed from %s to %s", interval, m.Interval))
	}
	if r.flag("upsideDown") {
		notes = append(notes, "upside down mode is not supported, the monitor is up when the service is up")
	}

	host := r.str("hostname")
	port := r.str("port")
	hostPort := func() string {
		if port == "" || port == "0" {
			return host
		}
		return net.JoinHostPort(host, port)
	}

	s := &m.Settings
	switch kind := r.str("type"); kind {
	case "http", "keyword", "json-query":
		m.Kind = monitor.KindHTTP
		m.Target = r.str("url")
		s.Method = r.str("method")
		s.Body = r.str("body")
		s.IgnoreTLS = r.flag("ignoreTls")
		s.MaxRedirects = int(min(max(r.num("maxredirects"), 0), 20))
		s.Headers = headers(r.str("headers"))
		s.AcceptedStatus = statuses(r.get("accepted_statuscodes", "accepted_statuscodes_json"))
		if kind == "keyword" {
			s.Keyword = r.str("keyword")
			s.KeywordAbsent = r.flag("invertKeyword")
		}
		if kind == "json-query" {
			notes = append(notes, "the JSON query is not supported, only the status code is checked")
		}
		if r.str("authMethod") != "" && r.str("authMethod") != "null" {
			notes = append(notes, "authentication was not imported, add an Authorization header by hand")
		}
	case "port":
		m.Kind = monitor.KindTCP
		m.Target = hostPort()
	case "ping":
		m.Kind = monitor.KindICMP
		m.Target = host
	case "dns":
		m.Kind = monitor.KindDNS
		m.Target = host
		s.RecordType = r.str("dns_resolve_type")
		if resolver := r.str("dns_resolve_server"); resolver != "" {
			s.Resolver = resolver
			if port != "" && port != "53" && port != "0" {
				s.Resolver = net.JoinHostPort(resolver, port)
			}
		}
	case "push":
		m.Kind = monitor.KindPush
		m.PushToken = r.str("pushToken")
	case "postgres":
		m.Kind = monitor.KindPostgres
		m.Target = r.str("databaseConnectionString")
	case "redis":
		m.Kind = monitor.KindRedis
		u, err := url.Parse(r.str("databaseConnectionString"))
		if err != nil || u.Host == "" {
			return nil, nil, errors.New("the redis connection string could not be read")
		}
		m.Target = u.Host
		if pass, ok := u.User.Password(); ok {
			s.Password = pass
		}
		s.TLS = u.Scheme == "rediss"
	case "docker":
		m.Kind = monitor.KindDocker
		m.Target = r.str("docker_container")
		notes = append(notes, "check the Docker host, heron uses the local socket")
	case "grpc-keyword":
		m.Kind = monitor.KindGRPC
		m.Target = r.str("grpcUrl")
		s.Service = r.str("grpcServiceName")
		if r.str("keyword") != "" {
			notes = append(notes, "the gRPC keyword is not supported, only the health status is checked")
		}
	case "gamedig":
		m.Kind = monitor.KindSteam
		if r.str("game") == "minecraft" {
			m.Kind = monitor.KindMinecraft
		}
		m.Target = hostPort()
	case "steam":
		m.Kind = monitor.KindSteam
		m.Target = hostPort()
	case "smtp":
		m.Kind = monitor.KindSMTP
		m.Target = hostPort()
	default:
		return nil, nil, fmt.Errorf("heron has no %s monitors", cmp.Or(kind, "untyped"))
	}
	return m, notes, nil
}

func headers(raw string) map[string]string {
	if raw == "" || raw == "null" {
		return nil
	}
	var in map[string]any
	if json.Unmarshal([]byte(raw), &in) != nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprint(v)
	}
	return out
}

func statuses(v any) []int {
	var list []string
	switch v := v.(type) {
	case []any:
		for _, s := range v {
			list = append(list, fmt.Sprint(s))
		}
	case string, []byte:
		raw, _ := v.(string)
		if b, ok := v.([]byte); ok {
			raw = string(b)
		}
		if json.Unmarshal([]byte(raw), &list) != nil {
			return nil
		}
	}
	if len(list) == 0 || (len(list) == 1 && list[0] == "200-299") {
		return nil
	}
	seen := map[int]bool{}
	for _, item := range list {
		lo, hi, isRange := strings.Cut(item, "-")
		from, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			continue
		}
		to := from
		if isRange {
			if to, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil {
				continue
			}
		}
		for code := max(from, 100); code <= min(to, 599); code++ {
			seen[code] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

type Store interface {
	CreateChannel(ctx context.Context, c *notify.Channel) error
	CreateMonitor(ctx context.Context, m *monitor.Monitor) error
	SlugTaken(ctx context.Context, slug string) (bool, error)
	PushTokenTaken(ctx context.Context, token string) (bool, error)
}

func (p *Plan) Apply(ctx context.Context, s Store) error {
	ids := make([]int64, len(p.Channels))
	for i, ch := range p.Channels {
		if err := s.CreateChannel(ctx, ch); err != nil {
			return fmt.Errorf("channel %s: %w", ch.Name, err)
		}
		ids[i] = ch.ID
	}
	for _, m := range p.Monitors {
		for _, kumaID := range p.linksOf[m] {
			if i, ok := p.channelOf[kumaID]; ok {
				m.ChannelIDs = append(m.ChannelIDs, ids[i])
			}
		}
		slug, err := freeSlug(ctx, s, m.Slug)
		if err != nil {
			return err
		}
		m.Slug = slug
		if m.PushToken != "" {
			if taken, err := s.PushTokenTaken(ctx, m.PushToken); err != nil {
				return err
			} else if taken {
				m.PushToken = ""
				p.Notes[m.Name] = append(p.Notes[m.Name], "the push token was already in use, a new one was made")
			}
		}
		if err := s.CreateMonitor(ctx, m); err != nil {
			return fmt.Errorf("monitor %s: %w", m.Name, err)
		}
	}
	return nil
}

func freeSlug(ctx context.Context, s Store, slug string) (string, error) {
	base := cmp.Or(slug, "monitor")
	for i := 1; ; i++ {
		candidate := base
		if i > 1 {
			candidate = base + "-" + strconv.Itoa(i)
		}
		taken, err := s.SlugTaken(ctx, candidate)
		if err != nil || !taken {
			return candidate, err
		}
	}
}
