package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
)

func (s *SourceService) findMacSource(ctx context.Context, id string) (mediaapi.Source, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Bridge.ControlTimeout())
	defer cancel()
	cursor := ""
	for page := 0; page < 100; page++ {
		result, err := s.Bridge.Sources(ctx, cursor)
		if err != nil {
			return mediaapi.Source{}, err
		}
		for _, source := range result.Items {
			if source.SourceID == id {
				if source.MediaKind != "video" {
					return mediaapi.Source{}, store.ErrSampleUnavailable
				}
				return source, nil
			}
		}
		if result.NextCursor == nil {
			return mediaapi.Source{}, store.ErrSampleUnavailable
		}
		cursor = *result.NextCursor
	}
	return mediaapi.Source{}, mediabridge.ErrContract
}

func (s *SourceService) importMacSource(c *gin.Context) {
	var req struct {
		SourceID string `json:"sourceId"`
	}
	if !Decode(c, &req) {
		return
	}
	if !sourceIDPattern.MatchString(req.SourceID) {
		Error(c, 400, "INVALID_REQUEST", "Invalid source ID")
		return
	}
	capabilities, err := s.bridgeCapabilities(c.Request.Context())
	if err != nil || !capabilities.SourceConfigured || !capabilities.FFprobeAvailable {
		Error(c, 503, "MEDIA_SOURCE_UNAVAILABLE", "Mac source catalog is unavailable")
		return
	}
	source, err := s.findMacSource(c.Request.Context(), req.SourceID)
	if err != nil {
		if errors.Is(err, store.ErrSampleUnavailable) {
			Error(c, 404, "NOT_FOUND", "Mac source not found")
		} else {
			bridgeFailure(c, err)
		}
		return
	}
	videoID, operationID := domain.ID(), domain.ID()
	j, err := s.Jobs.Submit("import", "", "", func(ctx context.Context, j domain.Job) (result error) {
		request := mediaapi.RegistrationRequest{OperationID: operationID, VideoID: videoID, SourceID: req.SourceID}
		intentHash := mediabridge.Fingerprint(request)
		if err := s.Store.PersistOperation(ctx, store.MediaOperation{OperationID: operationID, OperationType: "registration", JobID: j.ID, ResourceID: videoID, Provider: "mac", RequestFingerprint: intentHash, Status: "queued"}); err != nil {
			return err
		}
		submitted := false
		defer func() {
			if result != nil && submitted {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				remote, err := s.Bridge.Cancel(cleanupCtx, operationID)
				if err != nil || remote.Status != "cancelled" {
					_ = s.Store.MarkFailed(cleanupCtx, operationID, "REMOTE_CLEANUP_PENDING")
					result = errors.Join(jobs.ErrRemoteCleanupPending, result)
					return
				}
				_ = s.Store.MarkFailed(cleanupCtx, operationID, "PROCESS_FAILED")
			}
		}()
		submitted = true
		registrationCtx, cancelRegistration := context.WithCancel(ctx)
		stopRenew := make(chan struct{})
		renewDone := make(chan error, 1)
		interval := time.Duration(capabilities.LeaseSeconds) * time.Second / 3
		if interval > 30*time.Second {
			interval = 30 * time.Second
		}
		if interval < time.Second {
			interval = time.Second
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-stopRenew:
					renewDone <- nil
					return
				case <-registrationCtx.Done():
					renewDone <- nil
					return
				case <-ticker.C:
					renewCtx, cancel := context.WithTimeout(registrationCtx, 5*time.Second)
					op, renewErr := s.Bridge.Renew(renewCtx, operationID)
					cancel()
					var remoteErr *mediabridge.RemoteError
					if errors.As(renewErr, &remoteErr) && remoteErr.Code == "RESOURCE_NOT_FOUND" {
						continue
					}
					if renewErr == nil && (op.Status == "queued" || op.Status == "running" || op.Status == "completed") {
						continue
					}
					if renewErr == nil {
						renewErr = mediabridge.ErrContract
					}
					cancelRegistration()
					renewDone <- renewErr
					return
				}
			}
		}()
		registration, err := s.Bridge.Registration(registrationCtx, request)
		close(stopRenew)
		renewErr := <-renewDone
		cancelRegistration()
		if renewErr != nil {
			err = errors.Join(err, renewErr)
		}
		if err != nil {
			return err
		}
		if !validRemoteRegistration(registration, operationID, videoID) || registration.SHA256 != source.SHA256 || registration.SizeBytes != source.SizeBytes {
			return mediabridge.ErrContract
		}
		remote, err := s.Bridge.Operation(ctx, operationID)
		if err != nil {
			return err
		}
		if remote.Status != "completed" || !remoteOperationMatches(remote, "registration", "", videoID, intentHash) {
			return mediabridge.ErrContract
		}
		var observed mediaapi.Registration
		if json.Unmarshal(remote.Manifest, &observed) != nil || !reflect.DeepEqual(observed, registration) {
			return mediabridge.ErrContract
		}
		v := domain.Video{ID: videoID, Name: source.Name, Width: registration.Width, Height: registration.Height, DurationSeconds: registration.DurationSeconds, Status: "ready", CreatedAt: domain.Now(), StorageKey: registration.MediaKey, SourceKind: "mac", SourceID: req.SourceID, StorageProvider: "mac", MediaSHA256: registration.SHA256, MediaSizeBytes: registration.SizeBytes}
		revision, err := s.Store.PublishRemoteImport(ctx, j, v, operationID, mediabridge.Fingerprint(registration))
		if err != nil {
			return err
		}
		ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ackCancel()
		if _, err = s.Bridge.Ack(ackCtx, operationID, revision); err == nil {
			_ = s.Store.MarkAcknowledged(ackCtx, operationID, revision)
		}
		return nil
	})
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(202, j)
}
