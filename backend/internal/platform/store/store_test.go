package store

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
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
