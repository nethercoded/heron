package store

import (
	"context"
	"encoding/json/v2"
	"errors"
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

func (s *Store) SavePage(ctx context.Context, page Page) error {
	raw, err := json.Marshal(page)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `insert into settings (key, value) values ('page', ?) on conflict (key) do update set value = excluded.value`, raw)
	return err
}
