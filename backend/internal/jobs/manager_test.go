package jobs

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"testing"
	"time"
)

func TestCancellationWaitsForWorkCleanupAndQueueIsBounded(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	m := New(s, 1, time.Second)
	defer m.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	cleaned := make(chan struct{})
	j, e := m.Submit("import", "", "", func(ctx context.Context, j domain.Job) error {
		close(entered)
		<-ctx.Done()
		<-release
		close(cleaned)
		return ctx.Err()
	})
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("work not started")
	}
	if _, e = m.Submit("import", "", "", func(context.Context, domain.Job) error { return nil }); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Submit("import", "", "", func(context.Context, domain.Job) error { return nil }); e == nil {
		t.Fatal("queue unbounded")
	}
	got, e := m.Cancel(j.ID)
	if e != nil || got.Status != "cancelling" {
		t.Fatal(got, e)
	}
	got, _ = s.Job(j.ID)
	if got.Status != "cancelling" {
		t.Fatal("terminal before cleanup", got)
	}
	close(release)
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("work not cancelled")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ = s.Job(j.ID)
		if got.Status == "cancelled" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(got)
}
