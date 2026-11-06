package store

import (
	"context"
	"database/sql"
	"time"
)

type Evidence struct {
	ID         int64
	MonitorID  int64
	IncidentID int64
	At         time.Time
	Report     []byte
	HasImage   bool
}

func (s *Store) SaveEvidence(ctx context.Context, e *Evidence, image []byte) error {
	incident := sql.NullInt64{Int64: e.IncidentID, Valid: e.IncidentID != 0}
	return s.db.QueryRowContext(ctx, `insert into evidence (monitor_id, incident_id, at, report, image) values (?, ?, ?, ?, ?) returning id`,
		e.MonitorID, incident, e.At.Unix(), e.Report, image).Scan(&e.ID)
}

func (s *Store) IncidentEvidence(ctx context.Context, incidentID int64) ([]Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, monitor_id, coalesce(incident_id, 0), at, report, image is not null
		from evidence where incident_id = ? order by at, id`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		var e Evidence
		var at int64
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.IncidentID, &at, &e.Report, &e.HasImage); err != nil {
			return nil, err
		}
		e.At = fromUnix(at)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) EvidenceImage(ctx context.Context, id int64) ([]byte, error) {
	var image []byte
	err := s.db.QueryRowContext(ctx, `select image from evidence where id = ? and image is not null`, id).Scan(&image)
	return image, translate(err)
}

func (s *Store) CountEvidence(ctx context.Context, incidentID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from evidence where incident_id = ?`, incidentID).Scan(&n)
	return n, err
}
