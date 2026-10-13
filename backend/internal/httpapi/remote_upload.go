package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

type byteCounter struct{ n int64 }

func (c *byteCounter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// UploadToMac streams bytes to the agent while hashing them; no cloud media file is created.
func (s *SourceService) UploadToMac(ctx context.Context, source io.Reader, filename string) (video domain.Video, result error) {
	if s.Bridge == nil || source == nil {
		return video, mediabridge.ErrUnavailable
	}
	capabilities, err := s.bridgeCapabilities(ctx)
	if err != nil {
		return video, err
	}
	if !capabilities.FFprobeAvailable {
		return video, mediabridge.ErrUnavailable
	}
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" || filename == "." || filename == ".." || len(filename) > 255 {
		return video, mediabridge.ErrInvalid
	}
	videoID, operationID := domain.ID(), domain.ID()
	intent := mediaapi.UploadIntent{VideoID: videoID, Name: filename}
	fingerprint := mediabridge.Fingerprint(intent)
	if err := s.Store.PersistOperation(ctx, store.MediaOperation{OperationID: operationID, OperationType: "upload", ResourceID: videoID, Provider: "mac", RequestFingerprint: fingerprint, Status: "queued"}); err != nil {
		return video, err
	}
	hash := sha256.New()
	count := &byteCounter{}
	tee := io.TeeReader(source, io.MultiWriter(hash, count))
	submitted := false
	defer func() {
		if result != nil && submitted {
			result = s.cleanupRemote(operationID, result)
		}
	}()
	submitted = true
	uploadCtx, cancelUpload := context.WithCancel(ctx)
	leaseInterval := time.Duration(capabilities.LeaseSeconds) * time.Second / 3
	if leaseInterval > 30*time.Second {
		leaseInterval = 30 * time.Second
	}
	if leaseInterval < time.Second {
		leaseInterval = time.Second
	}
	stopRenew := make(chan struct{})
	renewDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(leaseInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopRenew:
				renewDone <- nil
				return
			case <-uploadCtx.Done():
				renewDone <- nil
				return
			case <-ticker.C:
				renewCtx, cancel := context.WithTimeout(uploadCtx, 5*time.Second)
				op, err := s.Bridge.Renew(renewCtx, operationID)
				cancel()
				if err != nil || (op.Status != "running" && op.Status != "queued" && op.Status != "completed") {
					if err == nil {
						err = mediabridge.ErrContract
					}
					cancelUpload()
					renewDone <- err
					return
				}
			}
		}
	}()
	registration, err := s.Bridge.Upload(uploadCtx, operationID, videoID, filename, tee, s.MaxUploadBytes)
	close(stopRenew)
	renewErr := <-renewDone
	cancelUpload()
	if renewErr != nil {
		err = errors.Join(err, renewErr)
	}
	if err != nil {
		return video, err
	}
	if !validRemoteRegistration(registration, operationID, videoID) || count.n != registration.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != registration.SHA256 {
		return video, mediabridge.ErrContract
	}
	remote, err := s.Bridge.Operation(ctx, operationID)
	if err != nil {
		return video, err
	}
	if remote.Status != "completed" || !remoteOperationMatches(remote, "upload", "", videoID, fingerprint) {
		return video, mediabridge.ErrContract
	}
	var observed mediaapi.Registration
	if json.Unmarshal(remote.Manifest, &observed) != nil || !reflect.DeepEqual(observed, registration) {
		return video, mediabridge.ErrContract
	}
	video = domain.Video{ID: videoID, Name: filename, Width: registration.Width, Height: registration.Height, DurationSeconds: registration.DurationSeconds, Status: "ready", CreatedAt: domain.Now(), StorageKey: registration.MediaKey, SourceKind: "upload", StorageProvider: "mac", MediaSHA256: registration.SHA256, MediaSizeBytes: registration.SizeBytes}
	revision, err := s.Store.PublishRemoteVideo(ctx, video, operationID, mediabridge.Fingerprint(registration))
	if err != nil {
		return domain.Video{}, err
	}
	ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err = s.Bridge.Ack(ackCtx, operationID, revision); err == nil {
		_ = s.Store.MarkAcknowledged(ackCtx, operationID, revision)
	}
	return video, nil
}
