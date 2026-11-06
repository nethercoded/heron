package store

import (
	"context"
	"time"
)

type Warning struct {
	MonitorID  int64
	Kind       string
	Message    string
	Since      time.Time
	NotifiedAt time.Time
}

func (s *Store) Warnings(ctx context.Context) ([]Warning, error) {
	rows, err := s.db.QueryContext(ctx, `select monitor_id, kind, message, since, notified_at from warnings order by since desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Warning
	for rows.Next() {
		var w Warning
		var since, notified int64
		if err := rows.Scan(&w.MonitorID, &w.Kind, &w.Message, &since, &notified); err != nil {
			return nil, err
		}
		w.Since, w.NotifiedAt = fromUnix(since), fromUnix(notified)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) SaveWarning(ctx context.Context, w Warning) error {
	_, err := s.db.ExecContext(ctx, `
		insert into warnings (monitor_id, kind, message, since, notified_at) values (?, ?, ?, ?, ?)
		on conflict (monitor_id, kind) do update set message = excluded.message, notified_at = excluded.notified_at`,
		w.MonitorID, w.Kind, w.Message, w.Since.Unix(), unix(w.NotifiedAt))
	return err
}

func (s *Store) ClearWarning(ctx context.Context, monitorID int64, kind string) error {
	_, err := s.db.ExecContext(ctx, `delete from warnings where monitor_id = ? and kind = ?`, monitorID, kind)
	return err
}

type Latency struct {
	Recent, Usual       time.Duration
	RecentN, UsualN     int64
	HiddenFails, Checks int64
}

func (s *Store) LatencyTrend(ctx context.Context, now time.Time, recent, usual time.Duration) (map[int64]Latency, error) {
	recentFrom := now.Add(-recent).Truncate(time.Hour).Unix()
	usualTo := now.Add(-24 * time.Hour).Truncate(time.Hour).Unix()
	usualFrom := now.Add(-24*time.Hour - usual).Truncate(time.Hour).Unix()
	rows, err := s.db.QueryContext(ctx, `
		select m.id,
			sum(case when r.bucket >= ?1 then r.latency_sum else 0 end), sum(case when r.bucket >= ?1 then r.latency_count else 0 end),
			sum(case when r.bucket >= ?2 and r.bucket < ?3 then r.latency_sum else 0 end), sum(case when r.bucket >= ?2 and r.bucket < ?3 then r.latency_count else 0 end)
		from monitors m join rollups r on r.monitor_id = m.id and r.bucket >= ?2
		group by m.id`, recentFrom, usualFrom, usualTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Latency{}
	for rows.Next() {
		var id, rs, rn, us, un int64
		if err := rows.Scan(&id, &rs, &rn, &us, &un); err != nil {
			return nil, err
		}
		l := Latency{RecentN: rn, UsualN: un}
		if rn > 0 {
			l.Recent = time.Duration(rs/rn) * time.Millisecond
		}
		if un > 0 {
			l.Usual = time.Duration(us/un) * time.Millisecond
		}
		out[id] = l
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	fails, err := s.db.QueryContext(ctx, `
		select monitor_id, sum(status = 'pending'), count(*) from results where at >= ? group by monitor_id`,
		now.Add(-24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	defer fails.Close()
	for fails.Next() {
		var id, hidden, total int64
		if err := fails.Scan(&id, &hidden, &total); err != nil {
			return nil, err
		}
		l := out[id]
		l.HiddenFails, l.Checks = hidden, total
		out[id] = l
	}
	return out, fails.Err()
}

type Sample struct {
	At    time.Time
	Value float64
	Total float64
}

func (s *Store) AddSample(ctx context.Context, monitorID int64, metric string, sm Sample) error {
	_, err := s.db.ExecContext(ctx, `insert or replace into samples (monitor_id, metric, at, value, total) values (?, ?, ?, ?, ?)`,
		monitorID, metric, sm.At.Unix(), sm.Value, sm.Total)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `delete from samples where monitor_id = ? and metric = ? and at < ?`, monitorID, metric, sm.At.AddDate(0, 0, -30).Unix())
	return err
}

func (s *Store) Samples(ctx context.Context, monitorID int64, metric string, since time.Time) ([]Sample, error) {
	rows, err := s.db.QueryContext(ctx, `select at, value, total from samples where monitor_id = ? and metric = ? and at >= ? order by at`,
		monitorID, metric, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sample
	for rows.Next() {
		var at int64
		var sm Sample
		if err := rows.Scan(&at, &sm.Value, &sm.Total); err != nil {
			return nil, err
		}
		sm.At = time.Unix(at, 0).UTC()
		out = append(out, sm)
	}
	return out, rows.Err()
}
