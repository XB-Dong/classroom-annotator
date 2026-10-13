package sources

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

var ErrIdempotency = errors.New("IDEMPOTENCY_CONFLICT")

type Repository struct{ DB *sql.DB }

func (r Repository) Claim(ctx context.Context, workspace, key, source string, candidate domain.Job, allow bool) (domain.Job, bool, error) {
	digest := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(digest[:])
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return domain.Job{}, false, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, workspace+":"+keyHash); e != nil {
		return domain.Job{}, false, e
	}
	var previousSource string
	var j domain.Job
	e = tx.QueryRowContext(ctx, `SELECT r.source_id,j.id,j.kind,coalesce(j.video_id,''),coalesce(j.batch_id,''),j.status,j.progress,j.error,j.created_at FROM local_import_requests r JOIN jobs j ON j.id=r.job_id WHERE r.workspace_id=$1 AND r.idempotency_key_hash=$2`, workspace, keyHash).Scan(&previousSource, &j.ID, &j.Kind, &j.VideoID, &j.BatchID, &j.Status, &j.Progress, &j.Error, &j.CreatedAt)
	if e == nil {
		if previousSource != source {
			return domain.Job{}, false, ErrIdempotency
		}
		return j, false, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return domain.Job{}, false, e
	}
	if !allow {
		return domain.Job{}, false, jobs.ErrQueue
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO jobs(id,kind,status,progress,error,created_at) VALUES($1,$2,$3,$4,$5,$6)`, candidate.ID, candidate.Kind, candidate.Status, candidate.Progress, candidate.Error, candidate.CreatedAt); e != nil {
		return domain.Job{}, false, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO local_import_requests(workspace_id,idempotency_key_hash,source_id,job_id) VALUES($1,$2,$3,$4)`, workspace, keyHash, source, candidate.ID); e != nil {
		return domain.Job{}, false, e
	}
	return candidate, true, tx.Commit()
}

// PublishImport commits the selected immutable video identity and the running job together.
// An existing identity is reused; the caller removes its redundant freshly prepared copy.
func (r Repository) PublishImport(ctx context.Context, j domain.Job, v domain.Video) (reused bool, err error) {
	if v.StorageProvider != "local" || (v.SourceKind != "local_directory" && v.SourceKind != "dataset") || v.SourceID == "" || v.MediaSHA256 == "" || v.MediaSizeBytes <= 0 {
		return false, ErrChanged
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	var status string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, j.ID).Scan(&status); e != nil {
		return false, e
	}
	if status != "running" {
		return false, store.ErrState
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, v.SourceKind+":"+v.SourceID+":"+v.MediaSHA256); e != nil {
		return false, e
	}
	var existing string
	e = tx.QueryRowContext(ctx, `SELECT id FROM videos WHERE storage_provider='local' AND source_kind=$1 AND source_id=$2 AND media_sha256=$3`, v.SourceKind, v.SourceID, v.MediaSHA256).Scan(&existing)
	if e == nil {
		reused = true
		v.ID = existing
	} else if errors.Is(e, sql.ErrNoRows) {
		_, e = tx.ExecContext(ctx, `INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key,source_kind,source_id,storage_provider,media_sha256,media_size_bytes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'local',$11,$12)`, v.ID, v.Name, v.Width, v.Height, v.DurationSeconds, v.Status, v.CreatedAt, v.StorageKey, v.SourceKind, v.SourceID, v.MediaSHA256, v.MediaSizeBytes)
	}
	if e != nil {
		return false, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE jobs SET video_id=$1,status='completed',progress=1 WHERE id=$2 AND status='running'`, v.ID, j.ID); e != nil {
		return false, e
	}
	return reused, tx.Commit()
}
