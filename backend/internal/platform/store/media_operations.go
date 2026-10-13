package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

var ErrOperationConflict = errors.New("media operation conflict")
var operationID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var operationHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

type MediaOperation struct {
	OperationID         string
	OperationType       string
	JobID               string
	ResourceID          string
	Provider            string
	RequestFingerprint  string
	Status              string
	PublicationStatus   string
	PublicationRevision int64
	ManifestSHA256      string
	Acknowledged        bool
	ErrorCode           string
	CreatedAt           string
	UpdatedAt           string
}

func validateMediaOperation(op MediaOperation) error {
	if !operationID.MatchString(op.OperationID) || op.Provider != "mac" || !operationHash.MatchString(op.RequestFingerprint) || (op.Status != "" && op.Status != "queued") {
		return ErrOperationConflict
	}
	if op.ResourceID != "" && !operationID.MatchString(op.ResourceID) {
		return ErrOperationConflict
	}
	if op.JobID != "" && !operationID.MatchString(op.JobID) {
		return ErrOperationConflict
	}
	switch op.OperationType {
	case "extraction", "dataset_import":
		if op.JobID == "" || op.ResourceID == "" {
			return ErrOperationConflict
		}
	case "registration":
		if op.JobID == "" || op.ResourceID == "" {
			return ErrOperationConflict
		}
	case "upload":
		if op.JobID != "" || op.ResourceID == "" {
			return ErrOperationConflict
		}
	case "archive":
		if op.JobID != "" || op.ResourceID != "" {
			return ErrOperationConflict
		}
	default:
		return ErrOperationConflict
	}
	return nil
}

const operationColumns = `operation_id,operation_type,coalesce(job_id,''),coalesce(resource_id,''),provider,request_fingerprint,status,publication_status,publication_revision,manifest_sha256,acknowledged,error_code,created_at,updated_at`

func scanOperation(row interface{ Scan(...any) error }) (MediaOperation, error) {
	var op MediaOperation
	err := row.Scan(&op.OperationID, &op.OperationType, &op.JobID, &op.ResourceID, &op.Provider, &op.RequestFingerprint, &op.Status, &op.PublicationStatus, &op.PublicationRevision, &op.ManifestSHA256, &op.Acknowledged, &op.ErrorCode, &op.CreatedAt, &op.UpdatedAt)
	return op, err
}

// PersistOperation commits the stable operation identity before the first remote call.
func (s *Store) PersistOperation(ctx context.Context, op MediaOperation) error {
	if err := validateMediaOperation(op); err != nil {
		return err
	}
	now := domain.Now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO media_operations(operation_id,operation_type,job_id,resource_id,provider,request_fingerprint,status,created_at,updated_at) VALUES($1,$2,nullif($3,''),nullif($4,''),$5,$6,'queued',$7,$7) ON CONFLICT(operation_id) DO NOTHING`, op.OperationID, op.OperationType, op.JobID, op.ResourceID, op.Provider, op.RequestFingerprint, now)
	if err != nil {
		return err
	}
	previous, err := s.GetOperation(ctx, op.OperationID)
	if err != nil {
		return err
	}
	if previous.OperationType != op.OperationType || previous.JobID != op.JobID || previous.ResourceID != op.ResourceID || previous.Provider != op.Provider || previous.RequestFingerprint != op.RequestFingerprint {
		return ErrOperationConflict
	}
	return nil
}

func (s *Store) GetOperation(ctx context.Context, id string) (MediaOperation, error) {
	if !operationID.MatchString(id) {
		return MediaOperation{}, ErrOperationConflict
	}
	return scanOperation(s.DB.QueryRowContext(ctx, `SELECT `+operationColumns+` FROM media_operations WHERE operation_id=$1`, id))
}

func (s *Store) PendingOperations(ctx context.Context) ([]MediaOperation, error) {
	return s.PendingOperationsAfter(ctx, "", "")
}

func (s *Store) PendingOperationsAfter(ctx context.Context, afterCreatedAt, afterID string) ([]MediaOperation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+operationColumns+` FROM media_operations WHERE (publication_status='unpublished' OR (publication_status='committed' AND NOT acknowledged)) AND (created_at,operation_id)>($1,$2) ORDER BY created_at,operation_id LIMIT 1000`, afterCreatedAt, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MediaOperation{}
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

func (s *Store) SetOperationStatus(ctx context.Context, id, status string) error {
	if !operationID.MatchString(id) {
		return ErrOperationConflict
	}
	switch status {
	case "queued", "running", "completed", "failed", "cancelled":
	default:
		return ErrOperationConflict
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE media_operations SET status=$2,updated_at=$3 WHERE operation_id=$1 AND publication_status='unpublished'`, id, status, domain.Now())
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id, code string) error {
	if !operationID.MatchString(id) || code == "" || len(code) > 64 {
		return ErrOperationConflict
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE media_operations SET status='failed',error_code=$2,updated_at=$3 WHERE operation_id=$1 AND publication_status='unpublished'`, id, code, domain.Now())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrOperationConflict
	}
	return nil
}

func (s *Store) MarkAcknowledged(ctx context.Context, id string, revision int64) error {
	if !operationID.MatchString(id) || revision <= 0 {
		return ErrOperationConflict
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE media_operations SET acknowledged=TRUE,updated_at=$3 WHERE operation_id=$1 AND publication_status='committed' AND publication_revision=$2`, id, revision, domain.Now())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrOperationConflict
	}
	return nil
}

func lockOperation(ctx context.Context, tx *sql.Tx, id, kind, jobID, resourceID, manifestHash string) (MediaOperation, error) {
	if !operationID.MatchString(id) || !operationHash.MatchString(manifestHash) {
		return MediaOperation{}, ErrOperationConflict
	}
	op, err := scanOperation(tx.QueryRowContext(ctx, `SELECT `+operationColumns+` FROM media_operations WHERE operation_id=$1 FOR UPDATE`, id))
	if err != nil {
		return op, err
	}
	if op.OperationType != kind || op.JobID != jobID || op.ResourceID != resourceID || op.Provider != "mac" {
		return op, ErrOperationConflict
	}
	if op.PublicationStatus == "committed" && op.ManifestSHA256 != manifestHash {
		return op, ErrOperationConflict
	}
	if op.PublicationStatus == "unpublished" && (op.Status == "failed" || op.Status == "cancelled") {
		return op, ErrOperationConflict
	}
	return op, nil
}

func markPublished(ctx context.Context, tx *sql.Tx, id, manifestHash string) (int64, error) {
	var revision int64
	err := tx.QueryRowContext(ctx, `UPDATE media_operations SET status='completed',publication_status='committed',publication_revision=1,manifest_sha256=$2,error_code='',updated_at=$3 WHERE operation_id=$1 AND publication_status='unpublished' RETURNING publication_revision`, id, manifestHash, domain.Now()).Scan(&revision)
	if err != nil {
		return 0, err
	}
	return revision, nil
}

// PublishRemoteBatch locks the job before the operation, then commits frames, job and marker together.
func (s *Store) PublishRemoteBatch(ctx context.Context, j domain.Job, frames []domain.Frame, operationID, manifestHash string) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, j.ID).Scan(&status); err != nil {
		return 0, err
	}
	op, err := lockOperation(ctx, tx, operationID, "extraction", j.ID, j.BatchID, manifestHash)
	if err != nil {
		return 0, err
	}
	if op.PublicationStatus == "committed" {
		if status != "completed" {
			return 0, ErrOperationConflict
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return op.PublicationRevision, nil
	}
	if status != "running" {
		return 0, ErrState
	}
	if err = publishBatchTx(ctx, tx, j, frames); err != nil {
		return 0, err
	}
	revision, err := markPublished(ctx, tx, operationID, manifestHash)
	if err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}

func (s *Store) PublishRemoteImport(ctx context.Context, j domain.Job, v domain.Video, operationID, manifestHash string) (int64, error) {
	if v.StorageProvider != "mac" || (v.SourceKind != "mac" && v.SourceKind != "dataset") || v.StorageKey == "" || v.MediaSHA256 == "" || v.MediaSizeBytes <= 0 {
		return 0, ErrOperationConflict
	}
	kind := "registration"
	if v.SourceKind == "dataset" {
		kind = "dataset_import"
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, j.ID).Scan(&status); err != nil {
		return 0, err
	}
	op, err := lockOperation(ctx, tx, operationID, kind, j.ID, v.ID, manifestHash)
	if err != nil {
		return 0, err
	}
	if op.PublicationStatus == "committed" {
		if status != "completed" {
			return 0, ErrOperationConflict
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return op.PublicationRevision, nil
	}
	if status != "running" {
		return 0, ErrState
	}
	if err = publishImportTx(ctx, tx, j, v); err != nil {
		return 0, err
	}
	revision, err := markPublished(ctx, tx, operationID, manifestHash)
	if err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}

func (s *Store) PublishRemoteVideo(ctx context.Context, v domain.Video, operationID, manifestHash string) (int64, error) {
	if v.StorageProvider != "mac" || v.SourceKind != "upload" || v.StorageKey == "" || v.MediaSHA256 == "" || v.MediaSizeBytes <= 0 {
		return 0, ErrOperationConflict
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	op, err := lockOperation(ctx, tx, operationID, "upload", "", v.ID, manifestHash)
	if err != nil {
		return 0, err
	}
	if op.PublicationStatus == "committed" {
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return op.PublicationRevision, nil
	}
	if _, err = tx.ExecContext(ctx, insertVideo, videoArgs(v)...); err != nil {
		return 0, err
	}
	revision, err := markPublished(ctx, tx, operationID, manifestHash)
	if err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}

func (s *Store) CommitOperation(ctx context.Context, operationID, manifestHash string) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	op, err := lockOperation(ctx, tx, operationID, "archive", "", "", manifestHash)
	if err != nil {
		return 0, err
	}
	if op.PublicationStatus == "committed" {
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return op.PublicationRevision, nil
	}
	revision, err := markPublished(ctx, tx, operationID, manifestHash)
	if err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}
