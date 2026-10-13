package store

import (
	"context"
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

func (s *Store) AddJob(j domain.Job) error {
	return s.AddJobWithContext(context.Background(), j)
}
func (s *Store) AddJobWithContext(parent context.Context, j domain.Job) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	_, e := s.DB.ExecContext(ctx, `INSERT INTO jobs(id,kind,video_id,batch_id,status,progress,error,created_at) VALUES($1,$2,nullif($3,''),nullif($4,''),$5,$6,$7,$8)`, j.ID, j.Kind, j.VideoID, j.BatchID, j.Status, j.Progress, j.Error, j.CreatedAt)
	return e
}

const jobSelect = `SELECT id,kind,coalesce(video_id,''),coalesce(batch_id,''),status,progress,error,created_at FROM jobs`

func scanJob(r interface{ Scan(...any) error }) (j domain.Job, e error) {
	e = r.Scan(&j.ID, &j.Kind, &j.VideoID, &j.BatchID, &j.Status, &j.Progress, &j.Error, &j.CreatedAt)
	return
}
func (s *Store) Job(id string) (domain.Job, error) {
	return s.JobWithContext(context.Background(), id)
}
func (s *Store) JobWithContext(parent context.Context, id string) (domain.Job, error) {
	ctx, cancel := s.Context(parent)
	defer cancel()
	return scanJob(s.DB.QueryRowContext(ctx, jobSelect+` WHERE id=$1`, id))
}
func (s *Store) Jobs() ([]domain.Job, error) {
	return s.jobsWhere("", nil)
}
func (s *Store) JobsInWorkspace(workspace string) ([]domain.Job, error) {
	return s.jobsWhere(` WHERE video_id IN (SELECT id FROM videos WHERE workspace_id=$1) OR (video_id IS NULL AND $1='00000000-0000-4000-8000-000000000001')`, []any{workspace})
}
func (s *Store) jobsWhere(where string, args []any) ([]domain.Job, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	out := []domain.Job{}
	r, e := s.DB.QueryContext(ctx, jobSelect+where+` ORDER BY created_at DESC,id DESC LIMIT 100`, args...)
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
	return s.StartJobWithContext(context.Background(), id)
}
func (s *Store) StartJobWithContext(parent context.Context, id string) (bool, error) {
	ctx, cancel := s.Context(parent)
	defer cancel()
	r, e := s.DB.ExecContext(ctx, `UPDATE jobs SET status='running' WHERE id=$1 AND status='queued'`, id)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *Store) CancelJob(id string) (domain.Job, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return domain.Job{}, e
	}
	defer tx.Rollback()
	j, e := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return domain.Job{}, e
	}
	switch j.Status {
	case "queued":
		j.Status = "cancelled"
	case "running":
		j.Status = "cancelling"
	}
	if _, e = tx.ExecContext(ctx, `UPDATE jobs SET status=$1 WHERE id=$2`, j.Status, id); e != nil {
		return domain.Job{}, e
	}
	if e = tx.Commit(); e != nil {
		return domain.Job{}, e
	}
	return j, nil
}
func (s *Store) FinishJob(id, status, code string) error {
	return s.FinishJobWithContext(context.Background(), id, status, code)
}
func (s *Store) FinishJobWithContext(parent context.Context, id, status, code string) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	_, e := s.DB.ExecContext(ctx, `UPDATE jobs SET status=CASE WHEN status='cancelling' AND $1 NOT IN ('CLEANUP_FAILED','REMOTE_CLEANUP_PENDING') THEN 'cancelled' ELSE $2 END,error=CASE WHEN status='cancelling' AND $3 NOT IN ('CLEANUP_FAILED','REMOTE_CLEANUP_PENDING') THEN '' ELSE $4 END WHERE id=$5 AND status IN ('running','cancelling')`, code, status, code, code, id)
	return e
}
func (s *Store) SetProgress(id string, p float64) error {
	return s.SetProgressWithContext(context.Background(), id, p)
}
func (s *Store) SetProgressWithContext(parent context.Context, id string, p float64) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	_, e := s.DB.ExecContext(ctx, `UPDATE jobs SET progress=GREATEST(progress,$1) WHERE id=$2 AND status='running'`, p, id)
	return e
}
func requireRunning(ctx context.Context, tx *sql.Tx, id string) error {
	var state string
	if e := tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, id).Scan(&state); e != nil {
		return e
	}
	if state != "running" {
		return ErrState
	}
	return nil
}
func (s *Store) PublishBatch(j domain.Job, frames []domain.Frame) error {
	return s.PublishBatchWithContext(context.Background(), j, frames)
}
func (s *Store) PublishBatchWithContext(parent context.Context, j domain.Job, frames []domain.Frame) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = publishBatchTx(ctx, tx, j, frames); e != nil {
		return e
	}
	return tx.Commit()
}
func publishBatchTx(ctx context.Context, tx *sql.Tx, j domain.Job, frames []domain.Frame) error {
	var e error
	if e = requireRunning(ctx, tx, j.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO batches(id,video_id,job_id,created_at) VALUES($1,$2,$3,$4)`, j.BatchID, j.VideoID, j.ID, domain.Now()); e != nil {
		return e
	}
	for _, f := range frames {
		if _, e = tx.ExecContext(ctx, `INSERT INTO frames(id,video_id,batch_id,timestamp_ms,width,height,storage_key,storage_provider,media_sha256,media_size_bytes) SELECT $1,$2,$3,$4,$5,$6,$7,storage_provider,$8,$9 FROM videos WHERE id=$2`, f.ID, f.VideoID, f.BatchID, f.TimestampMs, f.Width, f.Height, f.StorageKey, f.MediaSHA256, f.MediaSizeBytes); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE jobs SET status='completed',progress=1 WHERE id=$1 AND status='running'`, j.ID); e != nil {
		return e
	}
	return nil
}
func (s *Store) PublishImport(j domain.Job, v domain.Video) error {
	return s.PublishImportWithContext(context.Background(), j, v)
}
func (s *Store) PublishImportWithContext(parent context.Context, j domain.Job, v domain.Video) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = publishImportTx(ctx, tx, j, v); e != nil {
		return e
	}
	return tx.Commit()
}
func publishImportTx(ctx context.Context, tx *sql.Tx, j domain.Job, v domain.Video) error {
	var e error
	if e = requireRunning(ctx, tx, j.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, insertVideo, videoArgs(v)...); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE jobs SET status='completed',progress=1,video_id=$1 WHERE id=$2 AND status='running'`, v.ID, j.ID); e != nil {
		return e
	}
	return nil
}
