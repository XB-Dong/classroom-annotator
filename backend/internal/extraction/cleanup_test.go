package extraction

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type lockedCancellationRunner struct {
	jpegRunner
	locked  *os.File
	entered chan struct{}
}

func (r *lockedCancellationRunner) Extract(ctx context.Context, in string, ms int64, out string) error {
	if e := r.jpegRunner.Extract(ctx, in, ms, out); e != nil {
		return e
	}
	var e error
	r.locked, e = os.Open(out)
	if e != nil {
		return e
	}
	close(r.entered)
	<-ctx.Done()
	return ctx.Err()
}
func TestWindowsLockedJPEGCleanupFailureIsNotCancelled(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows denies deletion of an open file")
	}
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	v := domain.Video{ID: "v", Name: "fixture", Width: 3, Height: 2, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
	if e = s.AddVideo(v); e != nil {
		t.Fatal(e)
	}
	r := &lockedCancellationRunner{entered: make(chan struct{})}
	defer func() {
		if r.locked != nil {
			r.locked.Close()
		}
	}()
	m := jobs.New(s, 1, time.Second*5)
	defer m.Close()
	svc := Service{Store: s, Runner: r}
	j, e := m.Submit("extraction", v.ID, "batch", func(ctx context.Context, j domain.Job) error { return svc.Run(ctx, j, v, []int64{0}) })
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-r.entered:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	if _, e = m.Cancel(j.ID); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, e := s.Job(j.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.Status == "cancelled" || got.Status == "failed" {
			if got.Status != "failed" || got.Error != "CLEANUP_FAILED" {
				t.Fatal("cleanup falsely acknowledged", got)
			}
			entries, e := os.ReadDir(filepath.Join(s.Root, "staging", j.ID))
			if e != nil || len(entries) != 1 {
				t.Fatal("locked evidence not retained", entries, e)
			}
			frames, e := s.Frames(v.ID, "")
			if e != nil || len(frames) > 0 {
				t.Fatal(frames, e)
			}
			if e = r.locked.Close(); e != nil {
				t.Fatal(e)
			}
			if e = s.Recover(); e != nil {
				t.Fatal(e)
			}
			entries, e = os.ReadDir(filepath.Join(s.Root, "staging"))
			if e != nil || len(entries) != 0 {
				t.Fatal("restart did not reconcile retained staging", entries, e)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not reach safe failure")
}
