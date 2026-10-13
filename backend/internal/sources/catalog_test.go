package sources

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type probe struct{}

func (probe) Probe(context.Context, string) (media.Metadata, error) {
	return media.Metadata{Width: 16, Height: 12, Duration: 1}, nil
}
func (probe) Extract(context.Context, string, int64, string) error { return nil }
func fixture(t *testing.T) (*Catalog, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "fixture.mp4")
	if err := os.WriteFile(path, []byte("synthetic video"), 0600); err != nil {
		t.Fatal(err)
	}
	return &Catalog{Roots: []string{root}, Media: &media.Service{Root: t.TempDir(), Limit: 1024, Runner: probe{}}}, path
}
func TestCatalogOpaqueStableIdentityAndManagedCopy(t *testing.T) {
	c, path := fixture(t)
	items, err := c.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	other, err := c.List(context.Background())
	if err != nil || other[0].SourceID != items[0].SourceID {
		t.Fatal("unstable identity", other, err)
	}
	wire, _ := json.Marshal(items)
	if strings.Contains(string(wire), filepath.Dir(path)) {
		t.Fatal("physical path exposed")
	}
	v, err := c.Prepare(context.Background(), items[0].SourceID)
	if err != nil || v.StorageProvider != "local" || v.SourceKind != "local_directory" || v.MediaSHA256 != items[0].SHA256 || v.MediaSizeBytes != items[0].SizeBytes {
		t.Fatalf("video=%+v err=%v", v, err)
	}
	os.WriteFile(path, []byte("changed"), 0600)
	bytes, err := os.ReadFile(filepath.Join(c.Media.Root, v.StorageKey))
	if err != nil || string(bytes) != "synthetic video" {
		t.Fatal("not immutable managed copy", err)
	}
	if _, err = c.Prepare(context.Background(), items[0].SourceID); err == nil {
		t.Fatal("changed source imported with old identity")
	}
}
func TestCatalogRejectsTraversalAndSymlinkEscape(t *testing.T) {
	c, path := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	os.WriteFile(outside, []byte("outside"), 0600)
	link := filepath.Join(filepath.Dir(path), "escape.mp4")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal("required symlink security fixture failed", err)
		}
		t.Log("symlink escape branch not_run on Windows without privilege", err)
	}
	items, err := c.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatal("symlink enumerated", items, err)
	}
	for _, id := range []string{"../outside.mp4", path, "unknown"} {
		if _, err := c.Prepare(context.Background(), id); err == nil {
			t.Fatal("untrusted ID accepted", id)
		}
	}
}
func TestCatalogUnconfiguredFailsExplicitly(t *testing.T) {
	if _, err := (&Catalog{}).List(context.Background()); err != ErrUnavailable {
		t.Fatal("missing config", err)
	}
}

type cancelledProbe struct{}

func (cancelledProbe) Probe(ctx context.Context, path string) (media.Metadata, error) {
	<-ctx.Done()
	return media.Metadata{}, ctx.Err()
}
func (cancelledProbe) Extract(context.Context, string, int64, string) error { return nil }
func TestCatalogTimeoutIsReportedAndTemporaryCopyRemoved(t *testing.T) {
	c, _ := fixture(t)
	c.Media.Runner = cancelledProbe{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.List(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("catalogue timeout was hidden", err)
	}
	files, err := os.ReadDir(filepath.Join(c.Media.Root, "staging"))
	if err != nil || len(files) != 0 {
		t.Fatal("catalogue timeout left a copy", files, err)
	}
}
