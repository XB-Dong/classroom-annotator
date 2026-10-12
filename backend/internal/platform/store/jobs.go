package store

import (
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

func (s *Store) AddJob(j domain.Job) error {
	_, e := s.DB.Exec(`INSERT INTO jobs(id,kind,video_id,batch_id,status,progress,error,created_at) VALUES(?,?,nullif(?,''),nullif(?,''),?,?,?,?)`, j.ID, j.Kind, j.VideoID, j.BatchID, j.Status, j.Progress, j.Error, j.CreatedAt)
	return e
}

const jobSelect = `SELECT id,kind,coalesce(video_id,''),coalesce(batch_id,''),status,progress,error,created_at FROM jobs`

func scanJob(r interface{ Scan(...any) error }) (j domain.Job, e error) {
	e = r.Scan(&j.ID, &j.Kind, &j.VideoID, &j.BatchID, &j.Status, &j.Progress, &j.Error, &j.CreatedAt)
	return
}
func (s *Store) Job(id string) (domain.Job, error) {
	return scanJob(s.DB.QueryRow(jobSelect+` WHERE id=?`, id))
}
func (s *Store) Jobs() ([]domain.Job, error) {
	out := []domain.Job{}
	r, e := s.DB.Query(jobSelect + ` ORDER BY created_at DESC,id DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	for r.Next() {
		j, e := scanJob(r)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, r.Err()
}
func (s *Store) StartJob(id string) (bool, error) {
	r, e := s.DB.Exec(`UPDATE jobs SET status='running' WHERE id=? AND status='queued'`, id)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *Store) CancelJob(id string) (domain.Job, error) {
	_, e := s.DB.Exec(`UPDATE jobs SET status=CASE status WHEN 'queued' THEN 'cancelled' WHEN 'running' THEN 'cancelling' ELSE status END WHERE id=?`, id)
	if e != nil {
		return domain.Job{}, e
	}
	return s.Job(id)
}
func (s *Store) FinishJob(id, status, code string) error {
	_, e := s.DB.Exec(`UPDATE jobs SET status=CASE WHEN status='cancelling' THEN 'cancelled' ELSE ? END,error=CASE WHEN status='cancelling' THEN '' ELSE ? END WHERE id=? AND status IN ('running','cancelling')`, status, code, id)
	return e
}
func (s *Store) SetProgress(id string, p float64) error {
	_, e := s.DB.Exec(`UPDATE jobs SET progress=max(progress,?) WHERE id=? AND status='running'`, p, id)
	return e
}
func requireRunning(tx *sql.Tx, id string) error {
	var state string
	if e := tx.QueryRow(`SELECT status FROM jobs WHERE id=?`, id).Scan(&state); e != nil {
		return e
	}
	if state != "running" {
		return ErrState
	}
	return nil
}
func (s *Store) PublishBatch(j domain.Job, frames []domain.Frame) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = requireRunning(tx, j.ID); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO batches VALUES(?,?,?,?)`, j.BatchID, j.VideoID, j.ID, domain.Now()); e != nil {
		return e
	}
	for _, f := range frames {
		if _, e = tx.Exec(`INSERT INTO frames(id,video_id,batch_id,timestamp_ms,width,height,storage_key) VALUES(?,?,?,?,?,?,?)`, f.ID, f.VideoID, f.BatchID, f.TimestampMs, f.Width, f.Height, f.StorageKey); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`UPDATE jobs SET status='completed',progress=1 WHERE id=? AND status='running'`, j.ID); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) PublishImport(j domain.Job, v domain.Video) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = requireRunning(tx, j.ID); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO videos VALUES(?,?,?,?,?,?,?,?)`, v.ID, v.Name, v.Width, v.Height, v.DurationSeconds, v.Status, v.CreatedAt, v.StorageKey); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE jobs SET status='completed',progress=1,video_id=? WHERE id=? AND status='running'`, v.ID, j.ID); e != nil {
		return e
	}
	return tx.Commit()
}
