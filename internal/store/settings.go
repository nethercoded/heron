package store

import (
	"context"
	"encoding/json/v2"
	"errors"

	"github.com/nethercoded/heron/internal/site"
)

type Page struct {
	Title       string `json:"title"`
	Description string `json:"description,omitzero"`
	LogoURL     string `json:"logo_url,omitzero"`
	HomeURL     string `json:"home_url,omitzero"`
	Footer      string `json:"footer,omitzero"`
	HistoryDays int    `json:"history_days,omitzero"`
}

func DefaultPage() Page {
	return Page{Title: "Status", HistoryDays: 90}
}

func (s *Store) Page(ctx context.Context) (Page, error) {
	page := DefaultPage()
	var raw []byte
	err := s.db.QueryRowContext(ctx, `select value from settings where key = 'page'`).Scan(&raw)
	if err != nil {
		if errors.Is(translate(err), ErrNotFound) {
			return page, nil
		}
		return page, err
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return page, err
	}
	return page, nil
}

func (s *Store) Site(ctx context.Context) (site.Settings, error) {
	var out site.Settings
	var raw []byte
	err := s.db.QueryRowContext(ctx, `select value from settings where key = 'site'`).Scan(&raw)
	if errors.Is(translate(err), ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out)
}

func (s *Store) SaveSite(ctx context.Context, settings site.Settings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `insert into settings (key, value) values ('site', ?) on conflict (key) do update set value = excluded.value`, raw)
	return err
}

func (s *Store) SavePage(ctx context.Context, page Page) error {
	raw, err := json.Marshal(page)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `insert into settings (key, value) values ('page', ?) on conflict (key) do update set value = excluded.value`, raw)
	return err
}

func (s *Store) MetricsToken(ctx context.Context) (string, error) {
	var token string
	err := s.db.QueryRowContext(ctx, `select value from settings where key = 'metrics_token'`).Scan(&token)
	if errors.Is(translate(err), ErrNotFound) {
		return "", nil
	}
	return token, err
}

func (s *Store) SetMetricsToken(ctx context.Context, token string) error {
	if token == "" {
		_, err := s.db.ExecContext(ctx, `delete from settings where key = 'metrics_token'`)
		return err
	}
	_, err := s.db.ExecContext(ctx, `insert into settings (key, value) values ('metrics_token', ?) on conflict (key) do update set value = excluded.value`, token)
	return err
}
