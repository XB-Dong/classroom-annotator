package samples

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
)

type probeStub struct{ metadata media.Metadata }

func (p probeStub) Probe(context.Context, string) (media.Metadata, error) { return p.metadata, nil }
func (p probeStub) Extract(_ context.Context, _ string, _ int64, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil)
}

func TestCacheIsImmutableAndIdempotent(t *testing.T) {
	sourceRoot, mediaRoot := t.TempDir(), t.TempDir()
	data := []byte("video payload")
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.webm"), data, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	item := Item{SourceFile: "a.webm", SHA256: hex.EncodeToString(h[:]), SizeBytes: int64(len(data)), Width: 32, Height: 24, DurationSeconds: 2}
	key, err := Cache(context.Background(), sourceRoot, mediaRoot, item, probeStub{media.Metadata{Width: 32, Height: 24, Duration: 2}})
	if err != nil || key != "videos/"+item.SHA256+".webm" {
		t.Fatalf("cache: %q, %v", key, err)
	}
	key2, err := Cache(context.Background(), sourceRoot, mediaRoot, item, probeStub{media.Metadata{Width: 32, Height: 24, Duration: 2}})
	if err != nil || key2 != key {
		t.Fatalf("repeat: %q, %v", key2, err)
	}
	if err := os.WriteFile(filepath.Join(mediaRoot, filepath.FromSlash(key)), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Cache(context.Background(), sourceRoot, mediaRoot, item, probeStub{media.Metadata{Width: 32, Height: 24, Duration: 2}}); err == nil {
		t.Fatal("accepted changed cache")
	}
}
