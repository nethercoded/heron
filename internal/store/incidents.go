package store

import (
	"context"
	"database/sql"
	"slices"
	"time"
)

type Impact string

const (
	ImpactMinor       Impact = "minor"
	ImpactMajor       Impact = "major"
	ImpactCritical    Impact = "critical"
	ImpactMaintenance Impact = "maintenance"
)

var Impacts = []Impact{ImpactMinor, ImpactMajor, ImpactCritical, ImpactMaintenance}

type IncidentStatus string

const (
	IncidentInvestigating IncidentStatus = "investigating"
	IncidentIdentified    IncidentStatus = "identified"
	IncidentMonitoring    IncidentStatus = "monitoring"
	IncidentResolved      IncidentStatus = "resolved"
)

var IncidentStatuses = []IncidentStatus{IncidentInvestigating, IncidentIdentified, IncidentMonitoring, IncidentResolved}

type Incident struct {
	ID         int64
	Title      string
	Impact     Impact
	Status     IncidentStatus
	Automatic  bool
	MonitorIDs []int64
	Updates    []IncidentUpdate
	StartedAt  time.Time
	ResolvedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (i *Incident) Open() bool {
	return i.ResolvedAt.IsZero()
}

type IncidentUpdate struct {
	ID        int64
	Status    IncidentStatus
	Body      string
	CreatedAt time.Time
}

type IncidentFilter struct {
	OpenOnly bool
	Since    time.Time
	Limit    int
}

const incidentColumns = `id, title, impact, status, automatic, started_at, coalesce(resolved_at, 0), created_at, updated_at`

func scanIncident(row scanner) (*Incident, error) {
	var (
		inc                                Incident
		started, resolved, created, update int64
	)
	if err := row.Scan(&inc.ID, &inc.Title, &inc.Impact, &inc.Status, &inc.Automatic, &started, &resolved, &created, &update); err != nil {
		return nil, translate(err)
	}
	inc.StartedAt = fromUnix(started)
	inc.ResolvedAt = fromUnix(resolved)
	inc.CreatedAt = fromUnix(created)
	inc.UpdatedAt = fromUnix(update)
	return &inc, nil
}

func (s *Store) Incidents(ctx context.Context, f IncidentFilter) ([]*Incident, error) {
	query := `select ` + incidentColumns + ` from incidents where started_at >= ?`
	if f.OpenOnly {
		query += ` and resolved_at is null`
	}
	query += ` order by started_at desc, id desc`
	params := []any{unix(f.Since)}
	if f.Limit > 0 {
		query += ` limit ?`
		params = append(params, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, inc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, s.attach(ctx, list)
}

func (s *Store) Incident(ctx context.Context, id int64) (*Incident, error) {
	inc, err := scanIncident(s.db.QueryRowContext(ctx, `select `+incidentColumns+` from incidents where id = ?`, id))
	if err != nil {
		return nil, err
	}
	return inc, s.attach(ctx, []*Incident{inc})
}

func (s *Store) attach(ctx context.Context, list []*Incident) error {
	if len(list) == 0 {
		return nil
	}
	byID := make(map[int64]*Incident, len(list))
	ids := make([]int64, 0, len(list))
	for _, inc := range list {
		byID[inc.ID] = inc
		ids = append(ids, inc.ID)
	}
	in := placeholders(len(ids))

	rows, err := s.db.QueryContext(ctx, `select incident_id, monitor_id from incident_monitors where incident_id in (`+in+`) order by monitor_id`, args(ids)...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var incidentID, monitorID int64
		if err := rows.Scan(&incidentID, &monitorID); err != nil {
			rows.Close()
			return err
		}
		byID[incidentID].MonitorIDs = append(byID[incidentID].MonitorIDs, monitorID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = s.db.QueryContext(ctx, `
		select incident_id, id, status, body, created_at from incident_updates
		where incident_id in (`+in+`) order by created_at desc, id desc`, args(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var incidentID, created int64
		var u IncidentUpdate
		if err := rows.Scan(&incidentID, &u.ID, &u.Status, &u.Body, &created); err != nil {
			return err
		}
		u.CreatedAt = fromUnix(created)
		byID[incidentID].Updates = append(byID[incidentID].Updates, u)
	}
	return rows.Err()
}

func (s *Store) CreateIncident(ctx context.Context, inc *Incident, body string) error {
	now := time.Now().UTC().Truncate(time.Second)
	if inc.StartedAt.IsZero() {
		inc.StartedAt = now
	}
	if inc.Status == IncidentResolved && inc.ResolvedAt.IsZero() {
		inc.ResolvedAt = now
	}
	inc.CreatedAt, inc.UpdatedAt = now, now

	return s.tx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			insert into incidents (title, impact, status, automatic, started_at, resolved_at, created_at, updated_at)
			values (?, ?, ?, ?, ?, ?, ?, ?) returning id`,
			inc.Title, inc.Impact, inc.Status, inc.Automatic, inc.StartedAt.Unix(), nullUnix(inc.ResolvedAt), now.Unix(), now.Unix(),
		).Scan(&inc.ID)
		if err != nil {
			return err
		}
		if err := linkMonitors(ctx, tx, inc.ID, inc.MonitorIDs); err != nil {
			return err
		}
		if body == "" {
			return nil
		}
		u, err := insertUpdate(ctx, tx, inc.ID, inc.Status, body, now)
		inc.Updates = []IncidentUpdate{u}
		return err
	})
}

func (s *Store) UpdateIncident(ctx context.Context, inc *Incident) error {
	inc.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	return s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `update incidents set title = ?, impact = ?, started_at = ?, updated_at = ? where id = ?`,
			inc.Title, inc.Impact, inc.StartedAt.Unix(), inc.UpdatedAt.Unix(), inc.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `delete from incident_monitors where incident_id = ?`, inc.ID); err != nil {
			return err
		}
		return linkMonitors(ctx, tx, inc.ID, inc.MonitorIDs)
	})
}

func (s *Store) AddIncidentUpdate(ctx context.Context, id int64, status IncidentStatus, body string) (*Incident, error) {
	now := time.Now().UTC().Truncate(time.Second)
	err := s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			update incidents set status = ?1, updated_at = ?2,
				resolved_at = case when ?1 = 'resolved' then coalesce(resolved_at, ?2) else null end
			where id = ?3`, status, now.Unix(), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = insertUpdate(ctx, tx, id, status, body, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.Incident(ctx, id)
}

func (s *Store) DeleteIncident(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `delete from incidents where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) OpenAutomaticIncident(ctx context.Context, monitorID int64) (*Incident, error) {
	inc, err := scanIncident(s.db.QueryRowContext(ctx, `
		select `+incidentColumns+` from incidents
		where automatic = 1 and resolved_at is null
			and id in (select incident_id from incident_monitors where monitor_id = ?)
		order by started_at desc limit 1`, monitorID))
	if err != nil {
		return nil, err
	}
	return inc, s.attach(ctx, []*Incident{inc})
}

func linkMonitors(ctx context.Context, tx *sql.Tx, incidentID int64, monitorIDs []int64) error {
	for _, id := range slices.Compact(slices.Sorted(slices.Values(monitorIDs))) {
		if _, err := tx.ExecContext(ctx, `insert into incident_monitors (incident_id, monitor_id) values (?, ?)`, incidentID, id); err != nil {
			return translate(err)
		}
	}
	return nil
}

func insertUpdate(ctx context.Context, tx *sql.Tx, incidentID int64, status IncidentStatus, body string, at time.Time) (IncidentUpdate, error) {
	u := IncidentUpdate{Status: status, Body: body, CreatedAt: at}
	err := tx.QueryRowContext(ctx, `insert into incident_updates (incident_id, status, body, created_at) values (?, ?, ?, ?) returning id`,
		incidentID, status, body, at.Unix()).Scan(&u.ID)
	return u, err
}
