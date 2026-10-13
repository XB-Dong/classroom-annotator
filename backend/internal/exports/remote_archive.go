package exports

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

var ErrMixedProviders = errors.New("MIXED_MEDIA_PROVIDERS")
var archiveKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var archiveSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

func providerOf(records []Record) (string, error) {
	if len(records) == 0 {
		return "", ErrEmpty
	}
	provider := records[0].Frame.StorageProvider
	if provider == "" {
		provider = "local"
	}
	for _, record := range records[1:] {
		candidate := record.Frame.StorageProvider
		if candidate == "" {
			candidate = "local"
		}
		if candidate != provider {
			return "", ErrMixedProviders
		}
	}
	if provider != "local" && provider != "mac" {
		return "", ErrInvalid
	}
	return provider, nil
}

func archiveRequest(records []Record, id string) (mediaapi.ArchiveRequest, error) {
	doc := coco{Images: []cocoImage{}, Annotations: []cocoAnnotation{}, Categories: []cocoCategory{{ID: 1, Name: "head"}}}
	images := make([]mediaapi.ArchiveImage, 0, len(records))
	var total int64
	for i, record := range records {
		f := record.Frame
		if !archiveKey.MatchString(f.ID) || !archiveKey.MatchString(f.StorageKey) || !archiveSHA.MatchString(f.MediaSHA256) || f.MediaSizeBytes <= 0 || f.MediaSizeBytes > MaxArchiveImageBytes-total || f.Width <= 0 || f.Height <= 0 {
			return mediaapi.ArchiveRequest{}, ErrLimit
		}
		total += f.MediaSizeBytes
		images = append(images, mediaapi.ArchiveImage{FrameID: f.ID, MediaKey: f.StorageKey, SHA256: f.MediaSHA256, SizeBytes: f.MediaSizeBytes, Width: f.Width, Height: f.Height})
		imageID := i + 1
		doc.Images = append(doc.Images, cocoImage{ID: imageID, FileName: "images/" + f.ID + ".jpg", Width: f.Width, Height: f.Height})
		for _, annotation := range record.Annotations {
			area := annotation.BBox[2] * annotation.BBox[3]
			if math.IsNaN(area) || math.IsInf(area, 0) || area < 0 {
				return mediaapi.ArchiveRequest{}, ErrInvalid
			}
			doc.Annotations = append(doc.Annotations, cocoAnnotation{ID: len(doc.Annotations) + 1, ImageID: imageID, CategoryID: 1, BBox: annotation.BBox, Area: area, IsCrowd: 0})
		}
	}
	serialized, err := json.Marshal(doc)
	if err != nil {
		return mediaapi.ArchiveRequest{}, err
	}
	if len(serialized) > 16<<20 {
		return mediaapi.ArchiveRequest{}, ErrLimit
	}
	return mediaapi.ArchiveRequest{OperationID: id, Images: images, COCO: serialized}, nil
}

func waitArchive(ctx context.Context, bridge *mediabridge.Client, id string) (mediaapi.Operation, error) {
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	renew := time.NewTicker(30 * time.Second)
	defer renew.Stop()
	for {
		op, err := bridge.Operation(ctx, id)
		if err != nil {
			return op, err
		}
		switch op.Status {
		case "completed":
			return op, nil
		case "failed", "cancelled":
			return op, errors.New("PROCESS_FAILED")
		case "queued", "running":
		default:
			return op, mediabridge.ErrContract
		}
		select {
		case <-ctx.Done():
			return mediaapi.Operation{}, ctx.Err()
		case <-renew.C:
			if _, err := bridge.Renew(ctx, id); err != nil {
				return mediaapi.Operation{}, err
			}
		case <-poll.C:
		}
	}
}

func BuildRemote(ctx context.Context, s *store.Store, bridge *mediabridge.Client, records []Record) (manifest mediaapi.ArchiveManifest, result error) {
	if bridge == nil {
		return manifest, mediabridge.ErrUnavailable
	}
	if provider, err := providerOf(records); err != nil || provider != "mac" {
		if err != nil {
			return manifest, err
		}
		return manifest, ErrInvalid
	}
	id := domain.ID()
	req, err := archiveRequest(records, id)
	if err != nil {
		return manifest, err
	}
	fingerprint := mediabridge.Fingerprint(req)
	if err = s.PersistOperation(ctx, store.MediaOperation{OperationID: id, OperationType: "archive", Provider: "mac", RequestFingerprint: fingerprint, Status: "queued"}); err != nil {
		return manifest, err
	}
	submitted := false
	defer func() {
		if result != nil && submitted {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remote, err := bridge.Cancel(cleanupCtx, id)
			if err != nil || remote.Status != "cancelled" {
				_ = s.MarkFailed(cleanupCtx, id, "REMOTE_CLEANUP_PENDING")
				result = errors.Join(result, jobs.ErrRemoteCleanupPending)
			} else {
				_ = s.MarkFailed(cleanupCtx, id, "PROCESS_FAILED")
			}
		}
	}()
	submitted = true
	accepted, err := bridge.Archive(ctx, req)
	if err != nil {
		return manifest, err
	}
	if accepted.OperationID != id || accepted.OperationType != "archive" || accepted.RequestFingerprint != fingerprint || accepted.JobID != nil || accepted.ResourceID != nil {
		return manifest, mediabridge.ErrContract
	}
	completed, err := waitArchive(ctx, bridge, id)
	if err != nil {
		return manifest, err
	}
	if completed.OperationID != id || completed.OperationType != "archive" || completed.RequestFingerprint != fingerprint || completed.JobID != nil || completed.ResourceID != nil {
		return manifest, mediabridge.ErrContract
	}
	if json.Unmarshal(completed.Manifest, &manifest) != nil || !archiveKey.MatchString(manifest.MediaKey) || !archiveSHA.MatchString(manifest.SHA256) || manifest.SizeBytes <= 0 {
		return mediaapi.ArchiveManifest{}, mediabridge.ErrContract
	}
	expires, err := time.Parse(time.RFC3339Nano, manifest.ExpiresAt)
	if err != nil || time.Until(expires) <= 0 {
		return mediaapi.ArchiveManifest{}, mediabridge.ErrContract
	}
	revision, err := s.CommitOperation(ctx, id, mediabridge.Fingerprint(manifest))
	if err != nil {
		return manifest, err
	}
	ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err = bridge.Ack(ackCtx, id, revision); err == nil {
		_ = s.MarkAcknowledged(ackCtx, id, revision)
	}
	return manifest, nil
}
