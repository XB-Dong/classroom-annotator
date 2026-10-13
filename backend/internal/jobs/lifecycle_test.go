package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestPostgresManagerOwnershipCancellationStopsActiveWorkAndPublication(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewWithContext(ctx, s, 1, 10*time.Second, true)
	defer m.Close()
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	j, err := m.Submit("import", "", "", func(workCtx context.Context, job domain.Job) error {
		close(entered)
		<-workCtx.Done()
		if err := s.PublishImportWithContext(workCtx, job, domain.Video{}); err != context.Canceled {
			t.Errorf("old worker published after ownership loss: %v", err)
		}
		close(cancelled)
		return workCtx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("work did not start")
	}
	cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("active work did not observe ownership cancellation")
	}
	select {
	case <-m.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not exit")
	}
	got, err := s.Job(j.ID)
	if err != nil || got.Status != "running" {
		t.Fatal("cancelled parent wrote terminal state through pool", got, err)
	}
	if _, err = m.Submit("import", "", "", func(context.Context, domain.Job) error { t.Fatal("fresh work after loss"); return nil }); err != ErrQueue {
		t.Fatal(err)
	}
}

func TestManagerNonOwnerCannotCreateWorkAndCloseReturns(t *testing.T) {
	m := NewWithContext(context.Background(), nil, 1, time.Second, false)
	_, err := m.SubmitWithPersistence("import", "", "", func(_ domain.Job, allow bool) (domain.Job, bool, error) {
		if allow {
			t.Fatal("non-owner allowed creation")
		}
		return domain.Job{}, false, ErrQueue
	}, func(context.Context, domain.Job) error { t.Fatal("non-owner started work"); return nil })
	if err != ErrQueue {
		t.Fatal(err)
	}
	select {
	case <-m.done:
	default:
		t.Fatal("non-owner started worker loop")
	}
	m.Close()
}

func TestManagerParentCancellationStopsWorkerAndRejectsNewWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewWithContext(ctx, nil, 1, time.Second, true)
	cancel()
	select {
	case <-m.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	_, err := m.SubmitWithPersistence("import", "", "", func(_ domain.Job, allow bool) (domain.Job, bool, error) {
		if allow {
			t.Fatal("cancelled owner allowed creation")
		}
		return domain.Job{}, false, ErrQueue
	}, func(context.Context, domain.Job) error { t.Fatal("cancelled owner started work"); return nil })
	if err != ErrQueue {
		t.Fatal(err)
	}
	m.Close()
}
