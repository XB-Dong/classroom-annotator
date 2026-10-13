package sources

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"sync"
)

// SerializedRunner shares one process slot across catalogue probes, uploads and extraction.
type SerializedRunner struct {
	Runner media.Runner
	once   sync.Once
	slot   chan struct{}
}

func (r *SerializedRunner) acquire(ctx context.Context) error {
	r.once.Do(func() { r.slot = make(chan struct{}, 1) })
	select {
	case r.slot <- struct{}{}:
		if e := ctx.Err(); e != nil {
			<-r.slot
			return e
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (r *SerializedRunner) Probe(ctx context.Context, path string) (media.Metadata, error) {
	if e := r.acquire(ctx); e != nil {
		return media.Metadata{}, e
	}
	defer func() { <-r.slot }()
	return r.Runner.Probe(ctx, path)
}
func (r *SerializedRunner) Extract(ctx context.Context, path string, ms int64, out string) error {
	if e := r.acquire(ctx); e != nil {
		return e
	}
	defer func() { <-r.slot }()
	return r.Runner.Extract(ctx, path, ms, out)
}
