package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredMacCannotReadMatchingLocalKey(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "staging"), 0700)
	os.WriteFile(filepath.Join(root, "private-key"), []byte("matching local bytes"), 0600)
	s := Service{Store: &store.Store{Root: root}}
	_, err := s.Build(context.Background(), []Record{{Frame: domain.Frame{ID: "f", StorageProvider: "mac", StorageKey: "private-key"}}})
	if err == nil || err.Error() != "REMOTE_STORAGE_RETIRED" {
		t.Fatal("retired mac was treated as local", err)
	}
}
func TestLocalExportVerifiesExactBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f.jpg")
	var jpegBytes bytes.Buffer
	jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 16, 12)), nil)
	original := jpegBytes.Bytes()
	hash := sha256.Sum256(original)
	os.WriteFile(path, append(append([]byte{}, original...), []byte("changed-but-still-valid-jpeg")...), 0600)
	s := Service{Store: &store.Store{Root: root}}
	var wire bytes.Buffer
	archive := zip.NewWriter(&wire)
	record := Record{Frame: domain.Frame{ID: "f", StorageKey: "f.jpg", StorageProvider: "local", Width: 16, Height: 12, MediaSHA256: hex.EncodeToString(hash[:]), MediaSizeBytes: int64(len(original))}}
	_, err := s.copyImage(context.Background(), archive, record, "images/f.jpg", 1024*1024)
	if err == nil {
		t.Fatal("changed JPEG bytes exported")
	}
	if errors.Is(err, ErrLimit) {
		t.Fatal("fixture unexpectedly exceeds limit")
	}
}

func TestLocalExportWithoutIntegrityMetadataRequiresBackfill(t *testing.T) {
	s := Service{Store: &store.Store{Root: t.TempDir()}}
	var wire bytes.Buffer
	archive := zip.NewWriter(&wire)
	_, err := s.copyImage(context.Background(), archive, Record{Frame: domain.Frame{ID: "legacy", StorageProvider: "local", StorageKey: "missing.jpg"}}, "images/legacy.jpg", 1024)
	if !errors.Is(err, ErrBackfill) {
		t.Fatal("legacy export did not fail before media lookup", err)
	}
}
