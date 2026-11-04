package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
)

type Uptime struct {
	Total int64
	Down  int64
}

func (u Uptime) Ratio() float64 {
	if u.Total == 0 {
		return 1
	}
	return float64(u.Total-u.Down) / float64(u.Total)
}

type Day struct {
	Date     time.Time
	Uptime   Uptime
	Degraded int64
}

type Point struct {
	At     time.Time
	Avg    time.Duration
	Max    time.Duration
	Checks int64
	Down   int64
}

type Beat struct {
	At      time.Time
	Status  monitor.Status
	Latency time.Duration
	Message string
}

func (s *Store) Uptimes(ctx context.Context, since time.Time) (map[int64]Uptime, error) {
	windows, err := s.UptimeWindows(ctx, time.Now(), time.Since(since))
	if err != nil {
		return nil, err
	}
	out := make(map[int64]Uptime, len(windows))
	for id, w := range windows {
		out[id] = w[0]
	}
	return out, nil
}

func (s *Store) UptimeWindows(ctx context.Context, now time.Time, periods ...time.Duration) (map[int64][]Uptime, error) {
	out := map[int64][]Uptime{}
	if len(periods) == 0 {
		return out, nil
	}
	starts := make([]any, len(periods))
	var sums strings.Builder
	oldest := now
	for i, p := range periods {
		start := now.Add(-p).Truncate(time.Hour)
		if start.Before(oldest) {
			oldest = start
		}
		starts[i] = start.Unix()
		fmt.Fprintf(&sums, ", sum(case when r.bucket >= ?%[1]d then r.total else 0 end), sum(case when r.bucket >= ?%[1]d then r.down else 0 end)", i+1)
	}

	rows, err := s.db.QueryContext(ctx, `
		select m.id`+sums.String()+`
		from monitors m join rollups r on r.monitor_id = m.id and r.bucket >= ?`+strconv.Itoa(len(periods)+1)+`
		group by m.id`, append(starts, oldest.Unix())...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		w := make([]Uptime, len(periods))
		dest := []any{&id}
		for i := range w {
			dest = append(dest, &w[i].Total, &w[i].Down)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out[id] = w
	}
	return out, rows.Err()
}

func (s *Store) Days(ctx context.Context, ids []int64, from time.Time, loc *time.Location) (map[int64][]Day, error) {
	out := make(map[int64][]Day, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	_, offset := from.In(loc).Zone()
	start := midnight(from, loc)

	rows, err := s.db.QueryContext(ctx, `
		select monitor_id, (bucket + ?1) / 86400 as day, sum(total), sum(down), sum(degraded)
		from rollups
		where bucket >= ?2 and monitor_id in (`+placeholders(len(ids))+`)
		group by monitor_id, day order by monitor_id, day`,
		append([]any{offset, start.Unix()}, args(ids)...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, day int64
		var d Day
		if err := rows.Scan(&id, &day, &d.Uptime.Total, &d.Uptime.Down, &d.Degraded); err != nil {
			return nil, err
		}
		d.Date = time.Unix(day*86400-int64(offset), 0).In(loc)
		out[id] = append(out[id], d)
	}
	return out, rows.Err()
}

func (s *Store) Series(ctx context.Context, id int64, since time.Time, step time.Duration) ([]Point, error) {
	seconds := int64(step.Seconds())
	query := `
		select (at / ?1) * ?1 as slot, coalesce(avg(case when status != 'down' then latency_ms end), 0),
			max(latency_ms), count(*), sum(status = 'down')
		from results where monitor_id = ?2 and at >= ?3
		group by slot order by slot`
	if step >= time.Hour {
		query = `
		select (bucket / ?1) * ?1 as slot, coalesce(sum(latency_sum) * 1.0 / nullif(sum(latency_count), 0), 0),
			max(latency_max), sum(total), sum(down)
		from rollups where monitor_id = ?2 and bucket >= ?3
		group by slot order by slot`
	}

	rows, err := s.db.QueryContext(ctx, query, seconds, id, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var slot, maxMS int64
		var avgMS float64
		var p Point
		if err := rows.Scan(&slot, &avgMS, &maxMS, &p.Checks, &p.Down); err != nil {
			return nil, err
		}
		p.At = time.Unix(slot, 0).UTC()
		p.Avg = time.Duration(avgMS * float64(time.Millisecond))
		p.Max = time.Duration(maxMS) * time.Millisecond
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Beats(ctx context.Context, id int64, limit int) ([]Beat, error) {
	rows, err := s.db.QueryContext(ctx, `
		select at, status, latency_ms, message from results
		where monitor_id = ? order by at desc, id desc limit ?`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Beat
	for rows.Next() {
		var at, latency int64
		var b Beat
		if err := rows.Scan(&at, &b.Status, &latency, &b.Message); err != nil {
			return nil, err
		}
		b.At = fromUnix(at)
		b.Latency = time.Duration(latency) * time.Millisecond
		out = append(out, b)
	}
	return out, rows.Err()
}

func midnight(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
