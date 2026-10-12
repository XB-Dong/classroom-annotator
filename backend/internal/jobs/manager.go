package jobs

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/observability"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"sync"
	"time"
)

var ErrQueue = errors.New("JOB_QUEUE_FULL")

type task struct {
	job  domain.Job
	work func(context.Context, domain.Job) error
}
type Manager struct {
	s       *store.Store
	queue   chan task
	ctx     context.Context
	stop    context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	timeout time.Duration
	Metrics *observability.Metrics
}

func New(s *store.Store, n int, timeout time.Duration) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{s: s, queue: make(chan task, n), ctx: ctx, stop: cancel, done: make(chan struct{}), cancels: map[string]context.CancelFunc{}, timeout: timeout, Metrics: observability.New()}
	go m.loop()
	return m
}
func (m *Manager) Close() { m.stop(); <-m.done }
func (m *Manager) Submit(kind, video, batch string, work func(context.Context, domain.Job) error) (domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil || len(m.queue) >= cap(m.queue) {
		return domain.Job{}, ErrQueue
	}
	j := domain.Job{ID: domain.ID(), Kind: kind, VideoID: video, BatchID: batch, Status: "queued", CreatedAt: domain.Now()}
	if e := m.s.AddJob(j); e != nil {
		return domain.Job{}, e
	}
	m.queue <- task{j, work}
	m.Metrics.Queue.Set(float64(len(m.queue)))
	return j, nil
}
func (m *Manager) Cancel(id string) (domain.Job, error) {
	j, e := m.s.CancelJob(id)
	if e != nil {
		return j, e
	}
	m.mu.Lock()
	if cancel := m.cancels[id]; cancel != nil {
		cancel()
	}
	m.mu.Unlock()
	return j, nil
}
func (m *Manager) loop() {
	defer close(m.done)
	for {
		select {
		case <-m.ctx.Done():
			return
		case t := <-m.queue:
			m.Metrics.Queue.Set(float64(len(m.queue)))
			m.run(t)
		}
	}
}
func (m *Manager) run(t task) {
	ok, e := m.s.StartJob(t.job.ID)
	if e != nil || !ok {
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, m.timeout)
	defer cancel()
	m.mu.Lock()
	m.cancels[t.job.ID] = cancel
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.cancels, t.job.ID); m.mu.Unlock() }()
	j, e := m.s.Job(t.job.ID)
	if e != nil || j.Status != "running" {
		cancel()
	}
	start := time.Now()
	e = t.work(ctx, t.job)
	code := "JOB_FAILED"
	if e != nil {
		if ctx.Err() == context.DeadlineExceeded {
			code = "TIMEOUT"
		} else if e.Error() == "FRAME_UNAVAILABLE" {
			code = "FRAME_UNAVAILABLE"
		} else if e.Error() == "UPLOAD_TOO_LARGE" {
			code = "UPLOAD_TOO_LARGE"
		} else if e.Error() == "INVALID_VIDEO" {
			code = "INVALID_VIDEO"
		}
		_ = m.s.FinishJob(t.job.ID, "failed", code)
	} else {
		current, _ := m.s.Job(t.job.ID)
		if current.Status != "completed" {
			_ = m.s.FinishJob(t.job.ID, "failed", "JOB_NOT_PUBLISHED")
		}
	}
	current, e := m.s.Job(t.job.ID)
	if e == nil {
		m.Metrics.Jobs.WithLabelValues(t.job.Kind, current.Status).Inc()
		m.Metrics.JobDuration.WithLabelValues(t.job.Kind, current.Status).Observe(time.Since(start).Seconds())
	}
}
