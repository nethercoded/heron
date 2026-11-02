package store

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/nethercoded/heron/internal/maintenance"
)

func (s *Store) MaintenanceWindows(ctx context.Context) ([]maintenance.Window, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, title, body, starts_at, duration_s, repeat, enabled, created_at, updated_at
		from maintenance_windows order by starts_at desc, id desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []maintenance.Window
	byID := map[int64]int{}
	for rows.Next() {
		var (
			w                                  maintenance.Window
			starts, duration, created, updated int64
		)
		if err := rows.Scan(&w.ID, &w.Title, &w.Body, &starts, &duration, &w.Repeat, &w.Enabled, &created, &updated); err != nil {
			return nil, err
		}
		w.StartsAt = fromUnix(starts)
		w.Duration = time.Duration(duration) * time.Second
		w.CreatedAt = fromUnix(created)
		w.UpdatedAt = fromUnix(updated)
		byID[w.ID] = len(list)
		list = append(list, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links, err := s.db.QueryContext(ctx, `select window_id, monitor_id from maintenance_monitors order by monitor_id`)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var windowID, monitorID int64
		if err := links.Scan(&windowID, &monitorID); err != nil {
			return nil, err
		}
		if i, ok := byID[windowID]; ok {
			list[i].MonitorIDs = append(list[i].MonitorIDs, monitorID)
		}
	}
	return list, links.Err()
}

func (s *Store) MaintenanceWindow(ctx context.Context, id int64) (maintenance.Window, error) {
	list, err := s.MaintenanceWindows(ctx)
	if err != nil {
		return maintenance.Window{}, err
	}
	i := slices.IndexFunc(list, func(w maintenance.Window) bool { return w.ID == id })
	if i < 0 {
		return maintenance.Window{}, ErrNotFound
	}
	return list[i], nil
}

func (s *Store) SaveMaintenanceWindow(ctx context.Context, w *maintenance.Window) error {
	now := time.Now().UTC().Truncate(time.Second)
	w.UpdatedAt = now
	w.StartsAt = w.StartsAt.UTC().Truncate(time.Second)

	return s.tx(ctx, func(tx *sql.Tx) error {
		if w.ID == 0 {
			w.CreatedAt = now
			err := tx.QueryRowContext(ctx, `
				insert into maintenance_windows (title, body, starts_at, duration_s, repeat, enabled, created_at, updated_at)
				values (?, ?, ?, ?, ?, ?, ?, ?) returning id`,
				w.Title, w.Body, w.StartsAt.Unix(), int64(w.Duration.Seconds()), w.Repeat, w.Enabled, now.Unix(), now.Unix(),
			).Scan(&w.ID)
			if err != nil {
				return err
			}
		} else {
			res, err := tx.ExecContext(ctx, `
				update maintenance_windows set title = ?, body = ?, starts_at = ?, duration_s = ?, repeat = ?, enabled = ?, updated_at = ?
				where id = ?`,
				w.Title, w.Body, w.StartsAt.Unix(), int64(w.Duration.Seconds()), w.Repeat, w.Enabled, now.Unix(), w.ID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return ErrNotFound
			}
			if _, err := tx.ExecContext(ctx, `delete from maintenance_monitors where window_id = ?`, w.ID); err != nil {
				return err
			}
		}

		for _, id := range slices.Compact(slices.Sorted(slices.Values(w.MonitorIDs))) {
			if _, err := tx.ExecContext(ctx, `insert into maintenance_monitors (window_id, monitor_id) values (?, ?)`, w.ID, id); err != nil {
				return translate(err)
			}
		}
		return nil
	})
}

func (s *Store) DeleteMaintenanceWindow(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `delete from maintenance_windows where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
