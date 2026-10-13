package jobs

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"testing"
)

func TestPersistenceRetryReturnsExistingWhenQueueFullOrClosed(t *testing.T) {
	for _, closed := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if closed {
			cancel()
		}
		m := &Manager{ctx: ctx, queue: make(chan task, 1)}
		m.queue <- task{}
		existing := domain.Job{ID: "durable-job", Status: "completed"}
		called := false
		j, e := m.SubmitWithPersistence("import", "", "", func(candidate domain.Job, allow bool) (domain.Job, bool, error) {
			called = true
			if allow {
				t.Fatal("full queue allowed new claim")
			}
			return existing, false, nil
		}, func(context.Context, domain.Job) error { t.Fatal("existing job re-enqueued"); return nil })
		if e != nil || !called || j.ID != existing.ID || len(m.queue) != 1 {
			t.Fatal("retry contract", j, e)
		}
	}
}
