package media

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type Metadata struct {
	Width, Height int
	Duration      float64
}
type Runner interface {
	Probe(context.Context, string) (Metadata, error)
	Extract(context.Context, string, int64, string) error
}
type Service struct {
	Root   string
	Limit  int64
	Runner Runner
}

var ErrSize = errors.New("UPLOAD_TOO_LARGE")
var ErrVideo = errors.New("INVALID_VIDEO")
var ErrCleanup = errors.New("CLEANUP_FAILED")

func (s *Service) Prepare(ctx context.Context, r io.Reader, name string) (video domain.Video, result error) {
	for _, dir := range []string{"staging", "videos"} {
		if e := os.MkdirAll(filepath.Join(s.Root, dir), 0700); e != nil {
			return domain.Video{}, e
		}
	}
	f, e := os.CreateTemp(filepath.Join(s.Root, "staging"), "upload-*")
	if e != nil {
		return domain.Video{}, e
	}
	tmp := f.Name()
	defer func() {
		if e := os.Remove(tmp); e != nil && !os.IsNotExist(e) {
			result = errors.Join(ErrCleanup, result)
		}
	}()
	n, e := io.Copy(f, io.LimitReader(contextReader{ctx: ctx, reader: r}, s.Limit+1))
	closeErr := f.Close()
	if e != nil {
		return domain.Video{}, e
	}
	if closeErr != nil {
		return domain.Video{}, closeErr
	}
	if n > s.Limit {
		return domain.Video{}, ErrSize
	}
	if n == 0 {
		return domain.Video{}, ErrVideo
	}
	if e = ctx.Err(); e != nil {
		return domain.Video{}, e
	}
	m, e := s.Runner.Probe(ctx, tmp)
	if e != nil || m.Width <= 0 || m.Height <= 0 || m.Duration <= 0 || math.IsNaN(m.Duration) || math.IsInf(m.Duration, 0) {
		return domain.Video{}, ErrVideo
	}
	if e = ctx.Err(); e != nil {
		return domain.Video{}, e
	}
	id := domain.ID()
	key := filepath.Join("videos", id)
	if e = os.Rename(tmp, filepath.Join(s.Root, key)); e != nil {
		return domain.Video{}, e
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "" {
		name = "Imported video"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return domain.Video{ID: id, Name: name, Width: m.Width, Height: m.Height, DurationSeconds: m.Duration, Status: "ready", CreatedAt: domain.Now(), StorageKey: key}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	n, e := r.reader.Read(b)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, e
}
func (s *Service) Remove(v domain.Video) error {
	if e := os.Remove(filepath.Join(s.Root, v.StorageKey)); e != nil && !os.IsNotExist(e) {
		return ErrCleanup
	}
	return nil
}
