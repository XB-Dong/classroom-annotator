package jobs

import (
	"context"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"testing"
	"time"
)

func TestCancellationRetainsRemoteCleanupPendingFailure(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	m := New(s, 1, 15*time.Second)
	defer m.Close()
	entered := make(chan struct{})
	j, err := m.Submit("import", "", "", func(ctx context.Context, _ domain.Job) error {
		close(entered)
		<-ctx.Done()
		return fmt.Errorf("remote cleanup awaiting acknowledgement: %w", ErrRemoteCleanupPending)
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("work did not start")
	}
	if _, err := m.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Job(j.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == "failed" || got.Status == "cancelled" {
			if got.Status != "failed" || got.Error != "REMOTE_CLEANUP_PENDING" {
				t.Fatal("remote cleanup state was lost", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("remote cleanup result did not finish")
}

func TestCancellationWaitsForWorkCleanupAndQueueIsBounded(t *testing.T) {
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
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
func TestTimeoutPersistsFailureAfterCleanup(t *testing.T) {
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	m := New(s, 1, 10*time.Millisecond)
	defer m.Close()
	j, e := m.Submit("import", "", "", func(ctx context.Context, j domain.Job) error { <-ctx.Done(); return ctx.Err() })
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, e := s.Job(j.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.Status == "failed" {
			if got.Error != "TIMEOUT" {
				t.Fatal(got)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout did not finish")
}
