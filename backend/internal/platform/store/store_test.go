package store

import (
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestCancelWinsPublicationAndRestartInterrupts(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	v := domain.Video{ID: "v", Name: "test", Width: 2, Height: 3, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
	if e = s.AddVideo(v); e != nil {
		t.Fatal(e)
	}
	j := domain.Job{ID: "j", Kind: "extraction", VideoID: "v", BatchID: "b", Status: "queued", CreatedAt: domain.Now()}
	if e = s.AddJob(j); e != nil {
		t.Fatal(e)
	}
	if ok, e := s.StartJob("j"); e != nil || !ok {
		t.Fatal(ok, e)
	}
	if _, e = s.CancelJob("j"); e != nil {
		t.Fatal(e)
	}
	f := domain.Frame{ID: "f", VideoID: "v", BatchID: "b", Width: 3, Height: 2, StorageKey: filepath.Join("frames", "b", "f.jpg")}
	if e = s.PublishBatch(j, []domain.Frame{f}); e == nil {
		t.Fatal("cancelled job published")
	}
	got, _ := s.Job("j")
	if got.Status != "cancelling" {
		t.Fatal(got)
	}
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	got, _ = s.Job("j")
	if got.Status != "failed" || got.Error != "INTERRUPTED" {
		t.Fatal(got)
	}
	frames, e := s.Frames("v", "")
	if e != nil || len(frames) != 0 {
		t.Fatal(frames, e)
	}
}
func TestRestartReconcilesOnlyUnpublishedMedia(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	v := domain.Video{ID: "v", Name: "test", Width: 2, Height: 3, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: filepath.Join("videos", "v")}
	if e = s.AddVideo(v); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(s.Root, "videos", "v"), []byte("synthetic"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{"staging", "frames", "videos"} {
		if e = os.Mkdir(filepath.Join(s.Root, dir, "orphan"), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{"staging", "frames", "videos"} {
		if _, e = os.Stat(filepath.Join(s.Root, dir, "orphan")); !os.IsNotExist(e) {
			t.Fatal("orphan retained", dir, e)
		}
	}
	if _, e = os.Stat(filepath.Join(s.Root, "videos", "v")); e != nil {
		t.Fatal("committed media removed", e)
	}
}
func TestCompletedPublicationWinsLateCancelAndPersists(t *testing.T) {
	root := t.TempDir()
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	v := domain.Video{ID: "v", Name: "test", Width: 2, Height: 3, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
	if e = s.AddVideo(v); e != nil {
		t.Fatal(e)
	}
	j := domain.Job{ID: "j", Kind: "extraction", VideoID: "v", BatchID: "b", Status: "queued", CreatedAt: domain.Now()}
	s.AddJob(j)
	s.StartJob(j.ID)
	f := domain.Frame{ID: "f", VideoID: "v", BatchID: "b", Width: 3, Height: 2, StorageKey: "frames/b/f.jpg"}
	if e = s.PublishBatch(j, []domain.Frame{f}); e != nil {
		t.Fatal(e)
	}
	got, e := s.CancelJob(j.ID)
	if e != nil || got.Status != "completed" {
		t.Fatal(got, e)
	}
	s.DB.Close()
	s, e = Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	got, e = s.Job(j.ID)
	if e != nil || got.Status != "completed" {
		t.Fatal(got, e)
	}
	p, e := s.Progress("v", "b")
	if e != nil || p.Total != 1 || p.NotStarted != 1 {
		t.Fatal(p, e)
	}
	if _, e = s.Frames("v", "wrong-batch"); e == nil {
		t.Fatal("wrong batch accepted")
	}
}
func TestJobDiscoveryReturnsNewest100AndImportFieldsAreConditional(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	for n := 0; n < 105; n++ {
		j := domain.Job{ID: fmt.Sprintf("%03d", n), Kind: "import", Status: "queued", CreatedAt: fmt.Sprintf("2026-01-01T00:00:%03dZ", n)}
		if e = s.AddJob(j); e != nil {
			t.Fatal(e)
		}
	}
	out, e := s.Jobs()
	if e != nil || len(out) != 100 || out[0].ID != "104" || out[99].ID != "005" {
		t.Fatal(len(out), e)
	}
	for _, j := range out {
		if j.BatchID != "" || j.VideoID != "" {
			t.Fatal(j)
		}
	}
}
