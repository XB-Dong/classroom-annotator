package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

var remoteSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

func remoteOperationMatches(op mediaapi.Operation, kind, jobID, resourceID, fingerprint string) bool {
	if op.ContractVersion != mediaapi.ContractVersion || op.OperationType != kind || op.RequestFingerprint != fingerprint || !remoteSHA.MatchString(op.RequestFingerprint) {
		return false
	}
	if jobID == "" {
		if op.JobID != nil {
			return false
		}
	} else if op.JobID == nil || *op.JobID != jobID {
		return false
	}
	if resourceID == "" {
		if op.ResourceID != nil {
			return false
		}
	} else if op.ResourceID == nil || *op.ResourceID != resourceID {
		return false
	}
	return true
}

func (s *SourceService) waitRemote(ctx context.Context, id string) (mediaapi.Operation, error) {
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	renew := time.NewTicker(30 * time.Second)
	defer renew.Stop()
	for {
		op, err := s.Bridge.Operation(ctx, id)
		if err != nil {
			return op, err
		}
		if op.Progress < 0 || op.Progress > 1 || math.IsNaN(op.Progress) || math.IsInf(op.Progress, 0) {
			return op, mediabridge.ErrContract
		}
		if op.JobID != nil {
			_ = s.Store.SetProgress(*op.JobID, math.Min(.95, op.Progress*.95))
		}
		switch op.Status {
		case "completed":
			return op, nil
		case "failed", "cancelled":
			if op.ErrorCode != nil {
				return op, errors.New(*op.ErrorCode)
			}
			return op, errors.New("PROCESS_FAILED")
		case "queued", "running":
		default:
			return op, mediabridge.ErrContract
		}
		select {
		case <-ctx.Done():
			return mediaapi.Operation{}, ctx.Err()
		case <-renew.C:
			if _, err := s.Bridge.Renew(ctx, id); err != nil {
				return mediaapi.Operation{}, err
			}
		case <-poll.C:
		}
	}
}

func (s *SourceService) cleanupRemote(operationID string, workErr error) error {
	if workErr == nil {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	op, err := s.Bridge.Cancel(cleanupCtx, operationID)
	if err != nil || op.Status != "cancelled" {
		_ = s.Store.MarkFailed(cleanupCtx, operationID, "REMOTE_CLEANUP_PENDING")
		return errors.Join(jobs.ErrRemoteCleanupPending, workErr)
	}
	_ = s.Store.MarkFailed(cleanupCtx, operationID, "PROCESS_FAILED")
	return workErr
}

func validateExtractionManifest(req mediaapi.ExtractionRequest, manifest mediaapi.ExtractionManifest) ([]domain.Frame, error) {
	if manifest.BatchID != req.BatchID || len(manifest.Frames) != len(req.TimestampsMs) {
		return nil, mediabridge.ErrContract
	}
	frames := make([]domain.Frame, 0, len(manifest.Frames))
	seenID, seenKey := map[string]bool{}, map[string]bool{}
	for i, f := range manifest.Frames {
		if !sourceIDPattern.MatchString(f.FrameID) || !sourceIDPattern.MatchString(f.MediaKey) || seenID[f.FrameID] || seenKey[f.MediaKey] || f.TimestampMs != req.TimestampsMs[i] || f.Width <= 0 || f.Height <= 0 || f.SizeBytes <= 0 || !remoteSHA.MatchString(f.SHA256) {
			return nil, mediabridge.ErrContract
		}
		seenID[f.FrameID] = true
		seenKey[f.MediaKey] = true
		frames = append(frames, domain.Frame{ID: f.FrameID, VideoID: req.VideoID, BatchID: req.BatchID, TimestampMs: f.TimestampMs, Width: f.Width, Height: f.Height, StorageKey: f.MediaKey, StorageProvider: "mac", MediaSHA256: f.SHA256, MediaSizeBytes: f.SizeBytes})
	}
	return frames, nil
}

func (s *SourceService) RunExtraction(ctx context.Context, j domain.Job, v domain.Video, targets []int64) (result error) {
	if s.Bridge == nil || v.StorageProvider != "mac" || len(targets) == 0 || len(targets) > s.MaxFrames {
		return mediabridge.ErrInvalid
	}
	id := domain.ID()
	req := mediaapi.ExtractionRequest{OperationID: id, JobID: j.ID, BatchID: j.BatchID, VideoID: v.ID, MediaKey: v.StorageKey, TimestampsMs: targets, DeadlineSeconds: 600}
	fingerprint := mediabridge.Fingerprint(req)
	if err := s.Store.PersistOperation(ctx, store.MediaOperation{OperationID: id, OperationType: "extraction", JobID: j.ID, ResourceID: j.BatchID, Provider: "mac", RequestFingerprint: fingerprint, Status: "queued"}); err != nil {
		return err
	}
	submitted := false
	defer func() {
		if result != nil && submitted {
			result = s.cleanupRemote(id, result)
		}
	}()
	submitted = true
	accepted, err := s.Bridge.Extraction(ctx, req)
	if err != nil {
		return err
	}
	if accepted.OperationID != id || !remoteOperationMatches(accepted, "extraction", j.ID, j.BatchID, fingerprint) {
		return mediabridge.ErrContract
	}
	completed, err := s.waitRemote(ctx, id)
	if err != nil {
		return err
	}
	if !remoteOperationMatches(completed, "extraction", j.ID, j.BatchID, fingerprint) {
		return mediabridge.ErrContract
	}
	var manifest mediaapi.ExtractionManifest
	if json.Unmarshal(completed.Manifest, &manifest) != nil {
		return mediabridge.ErrContract
	}
	frames, err := validateExtractionManifest(req, manifest)
	if err != nil {
		return err
	}
	revision, err := s.Store.PublishRemoteBatch(ctx, j, frames, id, mediabridge.Fingerprint(manifest))
	if err != nil {
		return err
	}
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ackCancel()
	if _, err = s.Bridge.Ack(ackCtx, id, revision); err == nil {
		_ = s.Store.MarkAcknowledged(ackCtx, id, revision)
	}
	return nil
}

func (s *SourceService) RunDatasetImport(ctx context.Context, j domain.Job, videoID, catalogFileID, name string) (result error) {
	if s.Bridge == nil {
		return mediabridge.ErrUnavailable
	}
	id := domain.ID()
	req := mediaapi.DatasetImportRequest{OperationID: id, JobID: j.ID, VideoID: videoID, CatalogFileID: catalogFileID, Name: name}
	fingerprint := mediabridge.Fingerprint(req)
	if err := s.Store.PersistOperation(ctx, store.MediaOperation{OperationID: id, OperationType: "dataset_import", JobID: j.ID, ResourceID: videoID, Provider: "mac", RequestFingerprint: fingerprint, Status: "queued"}); err != nil {
		return err
	}
	submitted := false
	defer func() {
		if result != nil && submitted {
			result = s.cleanupRemote(id, result)
		}
	}()
	submitted = true
	accepted, err := s.Bridge.DatasetImport(ctx, req)
	if err != nil {
		return err
	}
	if accepted.OperationID != id || !remoteOperationMatches(accepted, "dataset_import", j.ID, videoID, fingerprint) {
		return mediabridge.ErrContract
	}
	completed, err := s.waitRemote(ctx, id)
	if err != nil {
		return err
	}
	if !remoteOperationMatches(completed, "dataset_import", j.ID, videoID, fingerprint) {
		return mediabridge.ErrContract
	}
	var registration mediaapi.Registration
	if json.Unmarshal(completed.Manifest, &registration) != nil || !validRemoteRegistration(registration, id, videoID) {
		return mediabridge.ErrContract
	}
	v := domain.Video{ID: videoID, Name: name, Width: registration.Width, Height: registration.Height, DurationSeconds: registration.DurationSeconds, Status: "ready", CreatedAt: domain.Now(), StorageKey: registration.MediaKey, SourceKind: "dataset", SourceID: catalogFileID, StorageProvider: "mac", MediaSHA256: registration.SHA256, MediaSizeBytes: registration.SizeBytes}
	revision, err := s.Store.PublishRemoteImport(ctx, j, v, id, mediabridge.Fingerprint(registration))
	if err != nil {
		return err
	}
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ackCancel()
	if _, err = s.Bridge.Ack(ackCtx, id, revision); err == nil {
		_ = s.Store.MarkAcknowledged(ackCtx, id, revision)
	}
	return nil
}

func validRemoteRegistration(r mediaapi.Registration, opID, videoID string) bool {
	return r.OperationID == opID && r.VideoID == videoID && sourceIDPattern.MatchString(r.MediaKey) && remoteSHA.MatchString(r.SHA256) && r.SizeBytes > 0 && r.Width > 0 && r.Height > 0 && r.DurationSeconds > 0 && !math.IsNaN(r.DurationSeconds) && !math.IsInf(r.DurationSeconds, 0)
}
