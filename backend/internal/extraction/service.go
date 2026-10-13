package extraction

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"os"
	"path/filepath"
)

type Service struct {
	Store  *store.Store
	Runner media.Runner
}

func (s *Service) Run(ctx context.Context, j domain.Job, v domain.Video, targets []int64) (result error) {
	if len(targets) == 0 {
		return errors.New("no sampling targets")
	}
	stage := filepath.Join(s.Store.Root, "staging", j.ID)
	if e := os.Mkdir(stage, 0700); e != nil {
		return e
	}
	defer func() {
		if e := os.RemoveAll(stage); e != nil {
			result = errors.Join(media.ErrCleanup, result)
		}
	}()
	frames := make([]domain.Frame, 0, len(targets))
	for i, ms := range targets {
		if e := ctx.Err(); e != nil {
			return e
		}
		id := domain.ID()
		out := filepath.Join(stage, id+".jpg")
		if e := s.Runner.Extract(ctx, filepath.Join(s.Store.Root, v.StorageKey), ms, out); e != nil {
			return e
		}
		w, h, e := media.JPEGDimensions(out)
		if e != nil {
			return e
		}
		hash, size, e := sources.Fingerprint(ctx, out)
		if e != nil {
			return e
		}
		frames = append(frames, domain.Frame{ID: id, VideoID: v.ID, BatchID: j.BatchID, TimestampMs: ms, Width: w, Height: h, StorageKey: filepath.Join("frames", j.BatchID, id+".jpg"), MediaSHA256: hash, MediaSizeBytes: size})
		if e = s.Store.SetProgressWithContext(ctx, j.ID, .9*float64(i+1)/float64(len(targets))); e != nil {
			return e
		}
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	final := filepath.Join(s.Store.Root, "frames", j.BatchID)
	if e := os.Rename(stage, final); e != nil {
		return e
	}
	if e := s.Store.PublishBatchWithContext(ctx, j, frames); e != nil {
		if cleanupErr := os.RemoveAll(final); cleanupErr != nil {
			return errors.Join(media.ErrCleanup, e)
		}
		return e
	}
	return nil
}
