package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
)

const monitorColumns = `id, slug, name, description, group_name, kind, target, interval_s, timeout_s, retries,
	public, paused, position, settings, coalesce(push_token, ''), status, status_since,
	last_checked_at, last_latency_ms, last_message, created_at, updated_at`

func scanMonitor(row scanner) (*monitor.Monitor, error) {
	var (
		m                          monitor.Monitor
		interval, timeout, latency int64
		since, checked             int64
		created, updated           int64
		settings                   []byte
	)
	err := row.Scan(&m.ID, &m.Slug, &m.Name, &m.Description, &m.Group, &m.Kind, &m.Target,
		&interval, &timeout, &m.Retries, &m.Public, &m.Paused, &m.Position, &settings, &m.PushToken,
		&m.State.Status, &since, &checked, &latency, &m.State.Message, &created, &updated)
	if err != nil {
		return nil, translate(err)
	}
	if err := json.Unmarshal(settings, &m.Settings); err != nil {
		return nil, fmt.Errorf("monitor %d settings: %w", m.ID, err)
	}
	m.Interval = time.Duration(interval) * time.Second
	m.Timeout = time.Duration(timeout) * time.Second
	m.State.Latency = time.Duration(latency) * time.Millisecond
	m.State.Since = fromUnix(since)
	m.State.CheckedAt = fromUnix(checked)
	m.CreatedAt = fromUnix(created)
	m.UpdatedAt = fromUnix(updated)
	return &m, nil
}

func (s *Store) Monitors(ctx context.Context) ([]*monitor.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, `select `+monitorColumns+` from monitors order by position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*monitor.Monitor
	byID := map[int64]*monitor.Monitor{}
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
		byID[m.ID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links, err := s.db.QueryContext(ctx, `select monitor_id, channel_id from monitor_channels order by channel_id`)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var monitorID, channelID int64
		if err := links.Scan(&monitorID, &channelID); err != nil {
			return nil, err
		}
		if m := byID[monitorID]; m != nil {
			m.ChannelIDs = append(m.ChannelIDs, channelID)
		}
	}
	return list, links.Err()
}

func (s *Store) Monitor(ctx context.Context, id int64) (*monitor.Monitor, error) {
	return s.monitorWhere(ctx, "id = ?", id)
}

func (s *Store) MonitorBySlug(ctx context.Context, slug string) (*monitor.Monitor, error) {
	return s.monitorWhere(ctx, "slug = ?", slug)
}

func (s *Store) MonitorByPushToken(ctx context.Context, token string) (*monitor.Monitor, error) {
	return s.monitorWhere(ctx, "push_token = ?", token)
}

func (s *Store) monitorWhere(ctx context.Context, cond string, arg any) (*monitor.Monitor, error) {
	m, err := scanMonitor(s.db.QueryRowContext(ctx, `select `+monitorColumns+` from monitors where `+cond, arg))
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `select channel_id from monitor_channels where monitor_id = ? order by channel_id`, m.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m.ChannelIDs = append(m.ChannelIDs, id)
	}
	return m, rows.Err()
}

func (s *Store) CreateMonitor(ctx context.Context, m *monitor.Monitor) error {
	settings, err := json.Marshal(m.Settings)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	m.CreatedAt, m.UpdatedAt = now, now
	m.State = monitor.State{Status: monitor.StatusPending, Since: now}
	if m.Paused {
		m.State.Status = monitor.StatusPaused
	}
	if m.Kind == monitor.KindPush && m.PushToken == "" {
		m.PushToken = rand.Text()
	}

	return s.tx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			insert into monitors (slug, name, description, group_name, kind, target, interval_s, timeout_s, retries,
				public, paused, position, settings, push_token, status, status_since, created_at, updated_at)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, coalesce((select max(position) + 1 from monitors), 0), ?, nullif(?, ''), ?, ?, ?, ?)
			returning id, position`,
			m.Slug, m.Name, m.Description, m.Group, m.Kind, m.Target, int64(m.Interval.Seconds()), int64(m.Timeout.Seconds()),
			m.Retries, m.Public, m.Paused, settings, m.PushToken, m.State.Status, now.Unix(), now.Unix(), now.Unix(),
		).Scan(&m.ID, &m.Position)
		if err != nil {
			return translate(err)
		}
		return linkChannels(ctx, tx, m.ID, m.ChannelIDs)
	})
}

func (s *Store) UpdateMonitor(ctx context.Context, m *monitor.Monitor) error {
	settings, err := json.Marshal(m.Settings)
	if err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	if m.Kind == monitor.KindPush && m.PushToken == "" {
		m.PushToken = rand.Text()
	}
	if m.Kind != monitor.KindPush {
		m.PushToken = ""
	}

	return s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			update monitors set slug = ?, name = ?, description = ?, group_name = ?, kind = ?, target = ?,
				interval_s = ?, timeout_s = ?, retries = ?, public = ?, settings = ?, push_token = nullif(?, ''), updated_at = ?
			where id = ?`,
			m.Slug, m.Name, m.Description, m.Group, m.Kind, m.Target, int64(m.Interval.Seconds()), int64(m.Timeout.Seconds()),
			m.Retries, m.Public, settings, m.PushToken, m.UpdatedAt.Unix(), m.ID)
		if err != nil {
			return translate(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `delete from monitor_channels where monitor_id = ?`, m.ID); err != nil {
			return err
		}
		return linkChannels(ctx, tx, m.ID, m.ChannelIDs)
	})
}

func linkChannels(ctx context.Context, tx *sql.Tx, monitorID int64, channelIDs []int64) error {
	for _, id := range channelIDs {
		if _, err := tx.ExecContext(ctx, `insert or ignore into monitor_channels (monitor_id, channel_id) values (?, ?)`, monitorID, id); err != nil {
			return fmt.Errorf("link channel %d: %w", id, translate(err))
		}
	}
	return nil
}

func (s *Store) DeleteMonitor(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `delete from monitors where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ReorderMonitors(ctx context.Context, ids []int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `update monitors set position = ? where id = ?`, i, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SetPaused(ctx context.Context, id int64, paused bool) (*monitor.Monitor, error) {
	status := monitor.StatusPending
	if paused {
		status = monitor.StatusPaused
	}
	res, err := s.db.ExecContext(ctx, `
		update monitors set paused = ?, status = ?, status_since = ?, updated_at = ?
		where id = ? and paused != ?`,
		paused, status, time.Now().Unix(), time.Now().Unix(), id, paused)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.Monitor(ctx, id); err != nil {
			return nil, err
		}
	}
	return s.Monitor(ctx, id)
}

func (s *Store) RotatePushToken(ctx context.Context, id int64) (string, error) {
	token := rand.Text()
	res, err := s.db.ExecContext(ctx, `update monitors set push_token = ? where id = ? and kind = 'push'`, token, id)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotFound
	}
	return token, nil
}

func (s *Store) Record(ctx context.Context, id int64, r monitor.Result, current monitor.Status) error {
	at := r.At.Unix()
	latency := r.Latency.Milliseconds()
	bucket := r.At.Truncate(time.Hour).Unix()

	var down, degraded, measured int
	switch r.Status {
	case monitor.StatusDown:
		down = 1
	case monitor.StatusDegraded:
		degraded = 1
	}
	if r.Status.Healthy() && latency > 0 {
		measured = 1
	}

	return s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `insert into results (monitor_id, at, status, latency_ms, message) values (?, ?, ?, ?, ?)`,
			id, at, r.Status, latency, r.Message); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			insert into rollups (monitor_id, bucket, total, down, degraded, latency_sum, latency_count, latency_max)
			values (?, ?, 1, ?, ?, ?, ?, ?)
			on conflict (monitor_id, bucket) do update set
				total = total + 1,
				down = down + excluded.down,
				degraded = degraded + excluded.degraded,
				latency_sum = latency_sum + excluded.latency_sum,
				latency_count = latency_count + excluded.latency_count,
				latency_max = max(latency_max, excluded.latency_max)`,
			id, bucket, down, degraded, latency*int64(measured), measured, latency*int64(measured)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			update monitors set
				status_since = case when status = ?1 then status_since else ?2 end,
				status = ?1, last_checked_at = ?2, last_latency_ms = ?3, last_message = ?4
			where id = ?5 and paused = 0`,
			current, at, latency, r.Message, id)
		return err
	})
}

func (s *Store) SlugTaken(ctx context.Context, slug string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from monitors where slug = ?`, slug).Scan(&n)
	return n > 0, err
}

func (s *Store) PushTokenTaken(ctx context.Context, token string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from monitors where push_token = ?`, token).Scan(&n)
	return n > 0, err
}
