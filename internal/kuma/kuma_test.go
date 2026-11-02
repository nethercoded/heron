package kuma

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
)

const export = `{
	"version": "1.23.16",
	"notificationList": [
		{"id": 1, "name": "Ops chat", "active": true, "config": "{\"name\":\"Ops chat\",\"type\":\"telegram\",\"telegramBotToken\":\"123:abc\",\"telegramChatID\":\"-100\"}"},
		{"id": 2, "name": "Pager", "active": true, "config": "{\"type\":\"pagerduty\"}"},
		{"id": 3, "name": "Discord", "active": true, "config": "{\"type\":\"discord\",\"discordWebhookUrl\":\"https://discord.com/api/webhooks/1/x\"}"}
	],
	"monitorList": [
		{"id": 10, "name": "Backend", "type": "group", "active": true},
		{"id": 11, "name": "API", "type": "keyword", "url": "https://api.example.com/health", "method": "GET", "interval": 30, "maxretries": 2,
		 "keyword": "ok", "invertKeyword": false, "accepted_statuscodes": ["200-299", "301"], "parent": 10, "active": true,
		 "headers": "{\"X-Token\":\"t\"}", "notificationIDList": {"1": true, "2": true, "3": false}},
		{"id": 12, "name": "Nightly backup", "type": "push", "pushToken": "kumaToken123", "interval": 86400, "active": false},
		{"id": 13, "name": "Postgres", "type": "port", "hostname": "db.internal", "port": 5432, "interval": 5, "active": true},
		{"id": 14, "name": "Broker", "type": "mqtt", "hostname": "mq", "active": true},
		{"id": 15, "name": "Cache", "type": "redis", "databaseConnectionString": "redis://:secret@cache:6380", "active": true}
	]
}`

func TestPlanFromJSON(t *testing.T) {
	src, err := ParseJSON([]byte(export))
	if err != nil {
		t.Fatal(err)
	}
	p := src.Plan()

	if len(p.Channels) != 2 || p.Channels[0].Kind != notify.KindTelegram || p.Channels[1].Kind != notify.KindDiscord {
		t.Fatalf("channels = %+v", p.Channels)
	}
	names := []string{}
	for _, s := range p.Skipped {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"Pager", "Broker"}) {
		t.Errorf("skipped = %v", names)
	}

	byName := map[string]*monitor.Monitor{}
	for _, m := range p.Monitors {
		byName[m.Name] = m
	}
	api := byName["API"]
	if api == nil || api.Kind != monitor.KindHTTP || api.Group != "Backend" || api.Settings.Keyword != "ok" || api.Retries != 2 {
		t.Fatalf("api = %+v", api)
	}
	if len(api.Settings.AcceptedStatus) != 101 || api.Settings.Headers["X-Token"] != "t" {
		t.Errorf("api settings = %+v", api.Settings)
	}
	if push := byName["Nightly backup"]; push.PushToken != "kumaToken123" || !push.Paused {
		t.Errorf("push = %+v", push)
	}
	if pg := byName["Postgres"]; pg.Target != "db.internal:5432" || pg.Interval != monitor.MinInterval || len(p.Notes["Postgres"]) != 1 {
		t.Errorf("tcp = %+v notes %v", pg, p.Notes["Postgres"])
	}
	if cache := byName["Cache"]; cache.Target != "cache:6380" || cache.Settings.Password != "secret" {
		t.Errorf("redis = %+v", cache)
	}
}

type memStore struct {
	channels []*notify.Channel
	monitors []*monitor.Monitor
	slugs    map[string]bool
}

func (s *memStore) CreateChannel(_ context.Context, c *notify.Channel) error {
	c.ID = int64(len(s.channels) + 100)
	s.channels = append(s.channels, c)
	return nil
}

func (s *memStore) CreateMonitor(_ context.Context, m *monitor.Monitor) error {
	s.slugs[m.Slug] = true
	s.monitors = append(s.monitors, m)
	return nil
}

func (s *memStore) SlugTaken(_ context.Context, slug string) (bool, error) { return s.slugs[slug], nil }
func (s *memStore) PushTokenTaken(context.Context, string) (bool, error)   { return false, nil }

func TestApply(t *testing.T) {
	src, _ := ParseJSON([]byte(export))
	p := src.Plan()
	s := &memStore{slugs: map[string]bool{"api": true}}
	if err := p.Apply(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	api := s.monitors[0]
	if api.Slug != "api-2" {
		t.Errorf("slug = %s", api.Slug)
	}
	if !slices.Equal(api.ChannelIDs, []int64{100}) {
		t.Errorf("channels = %v", api.ChannelIDs)
	}
}

func TestReadDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kuma.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`create table monitor (id integer primary key, name text, type text, url text, hostname text, port integer, interval integer, maxretries integer, active boolean, parent integer, accepted_statuscodes_json text, invert_keyword boolean, keyword text, push_token text, dns_resolve_type text, dns_resolve_server text)`,
		`create table notification (id integer primary key, name text, config text, active boolean)`,
		`create table monitor_notification (id integer primary key, monitor_id integer, notification_id integer)`,
		`create table status_page (id integer primary key)`,
		`create table "group" (id integer primary key, status_page_id integer)`,
		`create table monitor_group (id integer primary key, monitor_id integer, group_id integer)`,
		`insert into monitor values (1, 'Site', 'http', 'https://example.com', null, null, 60, 1, 1, null, '["200-299"]', 0, null, null, null, null)`,
		`insert into monitor values (2, 'Resolver', 'dns', null, 'example.com', 53, 120, 0, 1, null, null, 0, null, null, 'AAAA', '1.1.1.1')`,
		`insert into notification values (1, 'Hook', '{"type":"webhook","webhookURL":"https://hooks.example.com/x"}', 1)`,
		`insert into monitor_notification values (1, 2, 1)`,
		`insert into status_page values (1)`,
		`insert into "group" values (1, 1)`,
		`insert into monitor_group values (1, 1, 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	src, err := Read(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	p := src.Plan()
	if len(p.Monitors) != 2 || len(p.Channels) != 1 {
		t.Fatalf("plan = %d monitors, %d channels, skipped %v", len(p.Monitors), len(p.Channels), p.Skipped)
	}
	site, dns := p.Monitors[0], p.Monitors[1]
	if !site.Public || dns.Public {
		t.Errorf("public = %v %v, want only monitors on a status page", site.Public, dns.Public)
	}
	if dns.Settings.RecordType != "AAAA" || dns.Settings.Resolver != "1.1.1.1" || site.Settings.AcceptedStatus != nil {
		t.Errorf("settings = %+v / %+v", dns.Settings, site.Settings)
	}
	if links := src.Links[2]; !slices.Equal(links, []int64{1}) {
		t.Errorf("links = %v", links)
	}
}

func TestReadRejectsOtherFiles(t *testing.T) {
	if _, err := ParseJSON([]byte(`{"monitors": []}`)); err == nil {
		t.Fatal("parsed a file that is not a kuma backup")
	}
}
