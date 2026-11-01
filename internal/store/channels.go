package store

import (
	"context"
	"encoding/json/v2"
	"time"

	"github.com/nethercode/heron/internal/notify"
)

const channelColumns = `id, name, kind, config, enabled, created_at, updated_at`

func scanChannel(row scanner) (*notify.Channel, error) {
	var (
		c                notify.Channel
		config           []byte
		created, updated int64
	)
	if err := row.Scan(&c.ID, &c.Name, &c.Kind, &config, &c.Enabled, &created, &updated); err != nil {
		return nil, translate(err)
	}
	if err := json.Unmarshal(config, &c.Config); err != nil {
		return nil, err
	}
	c.CreatedAt = fromUnix(created)
	c.UpdatedAt = fromUnix(updated)
	return &c, nil
}

func (s *Store) Channels(ctx context.Context) ([]*notify.Channel, error) {
	return s.channelsWhere(ctx, `select `+channelColumns+` from channels order by name, id`)
}

func (s *Store) ChannelsForMonitor(ctx context.Context, monitorID int64) ([]*notify.Channel, error) {
	return s.channelsWhere(ctx, `
		select `+channelColumns+` from channels
		where enabled = 1 and id in (select channel_id from monitor_channels where monitor_id = ?)
		order by id`, monitorID)
}

func (s *Store) channelsWhere(ctx context.Context, query string, params ...any) ([]*notify.Channel, error) {
	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*notify.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) Channel(ctx context.Context, id int64) (*notify.Channel, error) {
	return scanChannel(s.db.QueryRowContext(ctx, `select `+channelColumns+` from channels where id = ?`, id))
}

func (s *Store) CreateChannel(ctx context.Context, c *notify.Channel) error {
	config, err := json.Marshal(c.Config)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	c.CreatedAt, c.UpdatedAt = now, now
	return s.db.QueryRowContext(ctx, `
		insert into channels (name, kind, config, enabled, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?) returning id`,
		c.Name, c.Kind, config, c.Enabled, now.Unix(), now.Unix()).Scan(&c.ID)
}

func (s *Store) UpdateChannel(ctx context.Context, c *notify.Channel) error {
	config, err := json.Marshal(c.Config)
	if err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `update channels set name = ?, kind = ?, config = ?, enabled = ?, updated_at = ? where id = ?`,
		c.Name, c.Kind, config, c.Enabled, c.UpdatedAt.Unix(), c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `delete from channels where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
