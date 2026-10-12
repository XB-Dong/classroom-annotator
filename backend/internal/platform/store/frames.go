package store

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

const FrameColumns = `f.id,f.video_id,f.batch_id,f.timestamp_ms,f.width,f.height,f.status,f.revision,f.storage_key`

func ScanFrame(r interface{ Scan(...any) error }) (f domain.Frame, e error) {
	e = r.Scan(&f.ID, &f.VideoID, &f.BatchID, &f.TimestampMs, &f.Width, &f.Height, &f.Status, &f.Revision, &f.StorageKey)
	f.ImageURL = "/media/frames/" + f.ID + ".jpg"
	return
}
func (s *Store) Frame(id string) (domain.Frame, error) {
	return ScanFrame(s.DB.QueryRow(`SELECT `+FrameColumns+` FROM frames f WHERE f.id=?`, id))
}
func (s *Store) CheckScope(video, batch string) error {
	var id string
	if e := s.DB.QueryRow(`SELECT id FROM videos WHERE id=?`, video).Scan(&id); e != nil {
		return e
	}
	if batch != "" {
		return s.DB.QueryRow(`SELECT id FROM batches WHERE id=? AND video_id=?`, batch, video).Scan(&id)
	}
	return nil
}
func (s *Store) Frames(video, batch string) ([]domain.Frame, error) {
	if e := s.CheckScope(video, batch); e != nil {
		return nil, e
	}
	out := []domain.Frame{}
	r, e := s.DB.Query(`SELECT `+FrameColumns+` FROM frames f JOIN batches b ON b.id=f.batch_id WHERE f.video_id=? AND (?='' OR f.batch_id=?) ORDER BY b.created_at DESC,b.id,f.timestamp_ms`, video, batch, batch)
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
	if e := s.CheckScope(video, ""); e != nil {
		return nil, e
	}
	out := []domain.Batch{}
	r, e := s.DB.Query(`SELECT b.id,b.video_id,b.job_id,b.created_at,count(f.id) FROM batches b LEFT JOIN frames f ON f.batch_id=b.id WHERE b.video_id=? GROUP BY b.id ORDER BY b.created_at DESC,b.id`, video)
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
	if e := s.CheckScope(video, batch); e != nil {
		return domain.Progress{}, e
	}
	var p domain.Progress
	e := s.DB.QueryRow(`SELECT count(*),coalesce(sum(status='not_started'),0),coalesce(sum(status='in_progress'),0),coalesce(sum(status='completed'),0) FROM frames WHERE video_id=? AND (?='' OR batch_id=?)`, video, batch, batch).Scan(&p.Total, &p.NotStarted, &p.InProgress, &p.Completed)
	return p, e
}
