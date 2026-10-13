package samples

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
)

var cacheKeyPattern = regexp.MustCompile(`^videos/[0-9a-f]{64}\.(webm|mp4)$`)

func safeCachePath(root, key string) (string, error) {
	if !strings.HasPrefix(key, "videos/") || strings.Contains(key, "\\") || path.Clean(key) != key || strings.Count(key, "/") != 1 {
		return "", ErrCatalog
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", ErrCatalog
	}
	videoDir := filepath.Join(root, "videos")
	info, err := os.Lstat(videoDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrCatalog
	}
	return filepath.Join(videoDir, filepath.Base(key)), nil
}

func VerifyCached(root, key string, item Item) error {
	if !cacheKeyPattern.MatchString(key) || !strings.HasPrefix(key, "videos/"+item.SHA256+".") {
		return ErrCatalog
	}
	p, err := safeCachePath(root, key)
	if err != nil {
		return err
	}
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != item.SizeBytes {
		return ErrCatalog
	}
	f, err := os.Open(p)
	if err != nil {
		return ErrCatalog
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, item.SizeBytes+1))
	if err != nil || n != item.SizeBytes || hex.EncodeToString(h.Sum(nil)) != item.SHA256 {
		return ErrCatalog
	}
	return nil
}

// Cache copies verified public media into an immutable local key under Store.Root.
func Cache(ctx context.Context, catalogDir, root string, item Item, runner media.Runner) (string, error) {
	if runner == nil || (filepath.Ext(item.SourceFile) != ".webm" && filepath.Ext(item.SourceFile) != ".mp4") {
		return "", ErrCatalog
	}
	if err := VerifySource(catalogDir, item); err != nil {
		return "", err
	}
	for _, dir := range []string{root, filepath.Join(root, "videos"), filepath.Join(root, "staging")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrCatalog
		}
	}
	key := path.Join("videos", item.SHA256+filepath.Ext(item.SourceFile))
	if err := VerifyCached(root, key, item); err == nil {
		return key, nil
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(key))); err == nil {
		return "", ErrCatalog
	} else if !os.IsNotExist(err) {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Join(root, "staging"), "public-sample-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	src, err := os.Open(filepath.Join(catalogDir, item.SourceFile))
	if err != nil {
		f.Close()
		return "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(&contextReader{ctx: ctx, reader: src}, item.SizeBytes+1))
	src.Close()
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil || n != item.SizeBytes || hex.EncodeToString(h.Sum(nil)) != item.SHA256 {
		return "", ErrCatalog
	}
	metadata, err := runner.Probe(ctx, tmp)
	if err != nil || metadata.Width != item.Width || metadata.Height != item.Height || metadata.Duration <= 0 || math.IsNaN(metadata.Duration) || math.IsInf(metadata.Duration, 0) || math.Abs(metadata.Duration-item.DurationSeconds) > 0.25 {
		return "", ErrCatalog
	}
	preview, err := os.CreateTemp(filepath.Join(root, "staging"), "sample-preview-*.jpg")
	if err != nil {
		return "", err
	}
	previewPath := preview.Name()
	preview.Close()
	defer os.Remove(previewPath)
	if err := runner.Extract(ctx, tmp, 0, previewPath); err != nil {
		return "", ErrCatalog
	}
	if _, _, err := media.JPEGDimensions(previewPath); err != nil {
		return "", ErrCatalog
	}
	destination, err := safeCachePath(root, key)
	if err != nil {
		return "", err
	}
	if err = os.Link(tmp, destination); err != nil {
		if verifyErr := VerifyCached(root, key, item); verifyErr == nil {
			return key, nil
		}
		return "", errors.Join(ErrCatalog, err)
	}
	return key, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(b)
}
