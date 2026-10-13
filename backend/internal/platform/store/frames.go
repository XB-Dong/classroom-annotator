package store

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

const FrameColumns = `f.id,f.video_id,f.batch_id,f.timestamp_ms,f.width,f.height,f.status,f.revision,f.storage_key,f.storage_provider,f.media_sha256,f.media_size_bytes`

func ScanFrame(r interface{ Scan(...any) error }) (f domain.Frame, e error) {
	e = r.Scan(&f.ID, &f.VideoID, &f.BatchID, &f.TimestampMs, &f.Width, &f.Height, &f.Status, &f.Revision, &f.StorageKey, &f.StorageProvider, &f.MediaSHA256, &f.MediaSizeBytes)
	f.ImageURL = "/media/frames/" + f.ID + ".jpg"
	return
}
func (s *Store) Frame(id string) (domain.Frame, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	return ScanFrame(s.DB.QueryRowContext(ctx, `SELECT `+FrameColumns+` FROM frames f WHERE f.id=$1`, id))
}
func (s *Store) CheckScope(video, batch string) error {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	var id string
	if e := s.DB.QueryRowContext(ctx, `SELECT id FROM videos WHERE id=$1`, video).Scan(&id); e != nil {
		return e
	}
	if batch != "" {
		return s.DB.QueryRowContext(ctx, `SELECT id FROM batches WHERE id=$1 AND video_id=$2`, batch, video).Scan(&id)
	}
	return nil
}
func (s *Store) Frames(video, batch string) ([]domain.Frame, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	if e := s.CheckScope(video, batch); e != nil {
		return nil, e
	}
	out := []domain.Frame{}
	r, e := s.DB.QueryContext(ctx, `SELECT `+FrameColumns+` FROM frames f JOIN batches b ON b.id=f.batch_id WHERE f.video_id=$1 AND ($2='' OR f.batch_id=$3) ORDER BY b.created_at DESC,b.id,f.timestamp_ms`, video, batch, batch)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	for r.Next() {
		f, e := ScanFrame(r)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, r.Err()
}
func (s *Store) Batches(video string) ([]domain.Batch, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	if e := s.CheckScope(video, ""); e != nil {
		return nil, e
	}
	out := []domain.Batch{}
	r, e := s.DB.QueryContext(ctx, `SELECT b.id,b.video_id,b.job_id,b.created_at,count(f.id) FROM batches b LEFT JOIN frames f ON f.batch_id=b.id WHERE b.video_id=$1 GROUP BY b.id ORDER BY b.created_at DESC,b.id`, video)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	for r.Next() {
		var b domain.Batch
		if e = r.Scan(&b.ID, &b.VideoID, &b.JobID, &b.CreatedAt, &b.FrameCount); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, r.Err()
}
func (s *Store) Progress(video, batch string) (domain.Progress, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	if e := s.CheckScope(video, batch); e != nil {
		return domain.Progress{}, e
	}
	var p domain.Progress
	e := s.DB.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE status='not_started'),count(*) FILTER(WHERE status='in_progress'),count(*) FILTER(WHERE status='completed') FROM frames WHERE video_id=$1 AND ($2='' OR batch_id=$3)`, video, batch, batch).Scan(&p.Total, &p.NotStarted, &p.InProgress, &p.Completed)
	return p, e
}
