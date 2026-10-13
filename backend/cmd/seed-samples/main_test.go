package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
)

// The release fixture is external because its original media and previews are not source files.
func TestActualPublicSampleCatalog(t *testing.T) {
	catalogPath := os.Getenv("P01_TEST_SAMPLE_CATALOG")
	ffmpeg := os.Getenv("P01_TEST_FFMPEG")
	ffprobe := os.Getenv("P01_TEST_FFPROBE")
	if catalogPath == "" || ffmpeg == "" || ffprobe == "" {
		t.Skip("external public-media fixture not configured")
	}
	items, err := samples.Load(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two publisher-approved examples, got %d", len(items))
	}
	root := t.TempDir()
	runner := media.Process{FFmpeg: ffmpeg, FFprobe: ffprobe}
	for _, item := range items {
		key, err := samples.Cache(context.Background(), filepath.Dir(catalogPath), root, item, runner)
		if err != nil {
			t.Fatalf("%s: %v", item.SampleID, err)
		}
		if err = samples.VerifyCached(root, key, item); err != nil {
			t.Fatalf("%s cache: %v", item.SampleID, err)
		}
		again, err := samples.Cache(context.Background(), filepath.Dir(catalogPath), root, item, runner)
		if err != nil || again != key {
			t.Fatalf("%s idempotence: %s %v", item.SampleID, again, err)
		}
	}
}
