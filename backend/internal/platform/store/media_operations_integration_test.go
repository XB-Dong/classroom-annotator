package store

import (
	"context"
	"errors"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestRemoteUploadPublicationAndImmutableReplay(t *testing.T) {
	s, err := Open(t.TempDir(), Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	intent := MediaOperation{OperationID: "upload-op", OperationType: "upload", ResourceID: "video-id", Provider: "mac", RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: "queued"}
	if err = s.PersistOperation(ctx, intent); err != nil {
		t.Fatal(err)
	}
	changed := intent
	changed.RequestFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err = s.PersistOperation(ctx, changed); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("request identity changed: %v", err)
	}
	v := domain.Video{ID: "video-id", Name: "remote.mp4", Width: 64, Height: 48, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "remote-key", SourceKind: "upload", StorageProvider: "mac", MediaSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", MediaSizeBytes: 123}
	manifestHash := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	revision, err := s.PublishRemoteVideo(ctx, v, intent.OperationID, manifestHash)
	if err != nil || revision != 1 {
		t.Fatalf("publish: revision=%d error=%v", revision, err)
	}
	got, err := s.Video(v.ID)
	if err != nil || got.StorageProvider != "mac" || got.StorageKey != v.StorageKey || got.MediaSHA256 != v.MediaSHA256 {
		t.Fatalf("published identity: %+v %v", got, err)
	}
	op, err := s.GetOperation(ctx, intent.OperationID)
	if err != nil || op.PublicationStatus != "committed" || op.ManifestSHA256 != manifestHash || op.RequestFingerprint != intent.RequestFingerprint {
		t.Fatalf("marker: %+v %v", op, err)
	}
	if again, err := s.PublishRemoteVideo(ctx, v, intent.OperationID, manifestHash); err != nil || again != revision {
		t.Fatalf("same manifest replay: %d %v", again, err)
	}
	if _, err = s.PublishRemoteVideo(ctx, v, intent.OperationID, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("changed manifest replay: %v", err)
	}
	if err = s.MarkAcknowledged(ctx, intent.OperationID, revision); err != nil {
		t.Fatal(err)
	}
}

func TestPublicSampleImportKeepsVersionAndVideo(t *testing.T) {
	s, err := Open(t.TempDir(), Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	sample := PublicSample{SampleID: "example", Version: "1.0", Name: "Example", Description: "test", Creator: "Creator", LicenseName: "CC BY 4.0", LicenseURL: "https://example.org/license", SourcePageURL: "https://example.org/source", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 123, Width: 64, Height: 48, DurationSeconds: 1, StorageKey: "videos/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.webm", CacheStatus: "ready"}
	if err = s.UpsertPublicSample(ctx, sample); err != nil {
		t.Fatal(err)
	}
	v, created, err := s.ImportPublicSample(ctx, sample.SampleID, sample.Version, sample.SHA256)
	if err != nil || !created || v.SourceKind != "public_sample" || v.StorageProvider != "local" {
		t.Fatalf("first import: %+v %t %v", v, created, err)
	}
	again, created, err := s.ImportPublicSample(ctx, sample.SampleID, sample.Version, sample.SHA256)
	if err != nil || created || again.ID != v.ID {
		t.Fatalf("repeat import: %+v %t %v", again, created, err)
	}
	if err = s.UpsertPublicSample(ctx, sample); err != nil {
		t.Fatal(err)
	}
	changed := sample
	changed.SHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err = s.UpsertPublicSample(ctx, changed); !errors.Is(err, ErrSampleConflict) {
		t.Fatalf("version reused for changed content: %v", err)
	}
}
