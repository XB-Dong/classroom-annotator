package mediaagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

type preparedOutput struct {
	Manifest  any
	Resources map[string]resource
}

var errUploadLimit = errors.New("UPLOAD_LIMIT")

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	n, e := r.r.Read(b)
	if r.ctx.Err() != nil {
		return n, r.ctx.Err()
	}
	return n, e
}
func (a *Agent) stage() (string, string, error) {
	rel := "staging/" + opaqueID()
	p, e := a.safePath(rel)
	if e != nil {
		return "", "", e
	}
	if e = os.Mkdir(p, 0700); e != nil {
		return "", "", errors.New("NO_SPACE")
	}
	return rel, p, nil
}
func copyFile(ctx context.Context, path string, r io.Reader, limit int64) (string, int64, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", 0, errors.New("NO_SPACE")
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(contextReader{ctx, r}, limit+1))
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		var fileError *os.PathError
		if errors.As(e, &fileError) {
			return "", n, errors.New("NO_SPACE")
		}
		return "", n, errors.New("PROCESS_FAILED")
	}
	if n > limit {
		return "", n, errUploadLimit
	}
	if n == 0 {
		return "", n, errors.New("INVALID_REQUEST")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func hashFile(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, errors.New("RESOURCE_NOT_FOUND")
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	if e != nil {
		return "", n, errors.New("RESOURCE_CHANGED")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func (a *Agent) probeVideo(ctx context.Context, stage, videoID, operationID, expectedHash string, expectedSize int64) (preparedOutput, error) {
	p, e := a.safePath(stage + "/video")
	if e != nil {
		return preparedOutput{}, e
	}
	if hash, size, err := hashFile(p); err != nil || hash != expectedHash || size != expectedSize {
		return preparedOutput{}, errors.New("RESOURCE_CHANGED")
	}
	m, e := a.cfg.Runner.Probe(ctx, p)
	if e != nil || m.Width <= 0 || m.Height <= 0 || m.Duration <= 0 || math.IsNaN(m.Duration) || math.IsInf(m.Duration, 0) {
		return preparedOutput{}, errors.New("PROCESS_FAILED")
	}
	hash, size, e := hashFile(p)
	if e != nil {
		return preparedOutput{}, e
	}
	if hash != expectedHash || size != expectedSize {
		return preparedOutput{}, errors.New("RESOURCE_CHANGED")
	}
	key := opaqueID()
	return preparedOutput{Manifest: mediaapi.Registration{OperationID: operationID, VideoID: videoID, MediaKey: key, SHA256: hash, SizeBytes: size, Width: m.Width, Height: m.Height, DurationSeconds: m.Duration}, Resources: map[string]resource{key: {Path: "video", ContentType: "video/mp4", SHA256: hash, SizeBytes: size, Width: m.Width, Height: m.Height, DurationSeconds: m.Duration}}}, nil
}
func (a *Agent) getResource(key string) (resource, string, error) {
	r, p, e := a.lookupResource(key)
	if e != nil { return resource{}, "", e }
	f, e := openVerifiedResource(p, r)
	if e != nil { return resource{}, "", e }
	f.Close()
	return r, p, nil
}
func (a *Agent) lookupResource(key string) (resource, string, error) {
	a.mu.Lock()
	r, ok := a.state.Resources[key]
	a.mu.Unlock()
	if !ok || !validID(key) || (r.ContentType == "application/zip" && !r.ExpiresAt.IsZero() && time.Now().After(r.ExpiresAt)) {
		return resource{}, "", errors.New("RESOURCE_NOT_FOUND")
	}
	p, e := a.safePath(r.Path)
	if e != nil {
		return resource{}, "", e
	}
	return r, p, nil
}
func openVerifiedResource(path string, r resource) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("RESOURCE_NOT_FOUND")
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != r.SizeBytes {
		f.Close()
		return nil, errors.New("RESOURCE_CHANGED")
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil || n != r.SizeBytes || hex.EncodeToString(h.Sum(nil)) != r.SHA256 {
		f.Close()
		return nil, errors.New("RESOURCE_CHANGED")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, errors.New("RESOURCE_CHANGED")
	}
	return f, nil
}
func (a *Agent) extract(ctx context.Context, req mediaapi.ExtractionRequest, stage string) (any, error) {
	video, input, e := a.getResource(req.MediaKey)
	if e != nil {
		return nil, e
	}
	dir, e := a.safePath(stage)
	if e != nil {
		return nil, e
	}
	manifest := mediaapi.ExtractionManifest{BatchID: req.BatchID, Frames: []mediaapi.Frame{}}
	resources := map[string]resource{}
	for _, ms := range req.TimestampsMs {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		key := opaqueID()
		filename := key + ".jpg"
		p := filepath.Join(dir, filename)
		if e = a.cfg.Runner.Extract(ctx, input, ms, p); e != nil {
			return nil, e
		}
		width, height, e := media.JPEGDimensions(p)
		if e != nil {
			return nil, e
		}
		hash, size, e := hashFile(p)
		if e != nil {
			return nil, e
		}
		frame := mediaapi.Frame{FrameID: opaqueID(), TimestampMs: ms, Width: width, Height: height, MediaKey: key, SHA256: hash, SizeBytes: size}
		manifest.Frames = append(manifest.Frames, frame)
		resources[key] = resource{Path: filename, ContentType: "image/jpeg", SHA256: hash, SizeBytes: size, Width: width, Height: height}
	}
	verified, e := openVerifiedResource(input, video)
	if e != nil {
		return nil, e
	}
	verified.Close()
	return preparedOutput{manifest, resources}, nil
}
