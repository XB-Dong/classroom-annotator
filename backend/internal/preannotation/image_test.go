package preannotation

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"os"
	"path/filepath"
	"testing"
)

func TestImmutableImageReaderRejectsChangedBytesAndEscapedKeys(t *testing.T) {
	root := t.TempDir()
	raw := []byte("synthetic immutable bytes")
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	if e := os.WriteFile(filepath.Join(root, "frame.jpg"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	s := NewService(&store.Store{Root: root}, Client{})
	f := domain.Frame{StorageKey: "frame.jpg", StorageProvider: "local", MediaSHA256: sha, MediaSizeBytes: int64(len(raw))}
	got, e := s.readImage(f, sha)
	if e != nil || string(got) != string(raw) {
		t.Fatal(got, e)
	}
	if e = os.WriteFile(filepath.Join(root, "frame.jpg"), []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.readImage(f, sha); e == nil {
		t.Fatal("changed image accepted")
	}
	f.StorageKey = filepath.Join("..", "outside.jpg")
	if _, e = s.readImage(f, sha); e == nil {
		t.Fatal("escaped key accepted")
	}
}

func TestFingerprintIncludesExpectedRevisionAndRiskInputs(t *testing.T) {
	targets := []target{{Frame: domain.Frame{ID: "f", MediaSHA256: "original-sha"}, Base: 1}}
	a := requestFingerprint(targets)
	targets[0].Base = 2
	if requestFingerprint(targets) == a {
		t.Fatal("revision omitted")
	}
	targets[0].Base = 1
	targets[0].Frame.MediaSHA256 = "changed-sha"
	if requestFingerprint(targets) == a {
		t.Fatal("image hash omitted")
	}
}
