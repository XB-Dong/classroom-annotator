package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrUnavailable = errors.New("LOCAL_SOURCES_NOT_CONFIGURED")
var ErrChanged = errors.New("SOURCE_CHANGED")
var ErrNotFound = errors.New("SOURCE_NOT_FOUND")

type Item struct {
	SourceID        string  `json:"sourceId"`
	Title           string  `json:"title"`
	SizeBytes       int64   `json:"sizeBytes"`
	SHA256          string  `json:"sha256"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
}
type Catalog struct {
	Roots []string
	Media *media.Service
}
type entry struct {
	Item
	root, relative string
}
type ctxReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r ctxReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(b)
}
func Fingerprint(ctx context.Context, path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, ctxReader{ctx, f})
	return hex.EncodeToString(h.Sum(nil)), n, e
}
func Stamp(ctx context.Context, s *media.Service, v *domain.Video) error {
	hash, n, e := Fingerprint(ctx, filepath.Join(s.Root, v.StorageKey))
	if e != nil {
		return e
	}
	v.MediaSHA256 = hash
	v.MediaSizeBytes = n
	v.StorageProvider = "local"
	return nil
}
func (c *Catalog) scan(ctx context.Context) ([]entry, error) {
	if len(c.Roots) == 0 {
		return nil, ErrUnavailable
	}
	if c.Media == nil || c.Media.Runner == nil {
		return nil, ErrUnavailable
	}
	out := []entry{}
	seen := map[string]bool{}
	for _, configured := range c.Roots {
		if !filepath.IsAbs(configured) {
			return nil, ErrUnavailable
		}
		info, e := os.Lstat(configured)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrUnavailable
		}
		canonical, e := filepath.EvalSymlinks(configured)
		if e != nil {
			return nil, ErrUnavailable
		}
		root, e := os.OpenRoot(canonical)
		if e != nil {
			return nil, ErrUnavailable
		}
		e = fs.WalkDir(root.FS(), ".", func(relative string, d fs.DirEntry, walkErr error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(relative)) {
			case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v":
			default:
				return nil
			}
			f, e := root.Open(relative)
			if e != nil {
				return e
			}
			defer f.Close()
			info, e := f.Stat()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > c.Media.Limit {
				return nil
			}
			stage := filepath.Join(c.Media.Root, "staging")
			if e = os.MkdirAll(stage, 0700); e != nil {
				return e
			}
			copy, e := os.CreateTemp(stage, "source-probe-*")
			if e != nil {
				return e
			}
			defer os.Remove(copy.Name())
			hash := sha256.New()
			n, e := io.Copy(io.MultiWriter(copy, hash), io.LimitReader(ctxReader{ctx, f}, c.Media.Limit+1))
			closeErr := copy.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
			if n != info.Size() || n > c.Media.Limit {
				return ErrChanged
			}
			metadata, e := c.Media.Runner.Probe(ctx, copy.Name())
			if e != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil
			}
			if metadata.Width <= 0 || metadata.Height <= 0 || metadata.Duration <= 0 || math.IsNaN(metadata.Duration) || math.IsInf(metadata.Duration, 0) {
				return nil
			}
			sha := hex.EncodeToString(hash.Sum(nil))
			identity := sha256.Sum256([]byte(filepath.Clean(canonical) + "\x00" + filepath.ToSlash(relative) + "\x00" + sha))
			id := hex.EncodeToString(identity[:])
			if seen[id] {
				return nil
			}
			seen[id] = true
			if len(out) >= 1000 {
				return ErrUnavailable
			}
			out = append(out, entry{Item{id, filepath.Base(relative), n, sha, metadata.Width, metadata.Height, metadata.Duration}, canonical, relative})
			return nil
		})
		root.Close()
		if e != nil {
			return nil, e
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceID < out[j].SourceID })
	return out, nil
}
func (c *Catalog) List(ctx context.Context) ([]Item, error) {
	entries, e := c.scan(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]Item, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Item)
	}
	return out, nil
}
func (c *Catalog) Prepare(ctx context.Context, id string) (domain.Video, error) {
	if len(id) != 64 {
		return domain.Video{}, ErrNotFound
	}
	entries, e := c.scan(ctx)
	if e != nil {
		return domain.Video{}, e
	}
	for _, entry := range entries {
		if entry.SourceID != id {
			continue
		}
		root, e := os.OpenRoot(entry.root)
		if e != nil {
			return domain.Video{}, ErrChanged
		}
		defer root.Close()
		f, e := root.Open(entry.relative)
		if e != nil {
			return domain.Video{}, ErrChanged
		}
		defer f.Close()
		v, e := c.Media.Prepare(ctx, f, entry.Title)
		if e != nil {
			return v, e
		}
		if e = Stamp(ctx, c.Media, &v); e != nil {
			return domain.Video{}, errors.Join(e, c.Media.Remove(v))
		}
		if v.MediaSHA256 != entry.SHA256 || v.MediaSizeBytes != entry.SizeBytes {
			return domain.Video{}, errors.Join(ErrChanged, c.Media.Remove(v))
		}
		v.SourceKind = "local_directory"
		v.SourceID = id
		return v, nil
	}
	return domain.Video{}, ErrNotFound
}
