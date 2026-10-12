package extraction

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

type jpegRunner struct {
	targets []int64
	missing bool
}

func (r *jpegRunner) Probe(context.Context, string) (media.Metadata, error) {
	return media.Metadata{}, nil
}
func (r *jpegRunner) Extract(ctx context.Context, in string, ms int64, out string) error {
	r.targets = append(r.targets, ms)
	if r.missing {
		return nil
	}
	f, e := os.Create(out)
	if e != nil {
		return e
	}
	defer f.Close()
	return jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil)
}
func TestBatchPublishesActualJPEGDimensionsAndUnavailableTailFails(t *testing.T) {
	for _, missing := range []bool{false, true} {
		s, e := store.Open(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		defer s.DB.Close()
		v := domain.Video{ID: "v", Name: "test", Width: 2, Height: 3, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
		if e = s.AddVideo(v); e != nil {
			t.Fatal(e)
		}
		j := domain.Job{ID: "j", Kind: "extraction", VideoID: "v", BatchID: "b", Status: "queued", CreatedAt: domain.Now()}
		s.AddJob(j)
		s.StartJob(j.ID)
		r := &jpegRunner{missing: missing}
		svc := Service{Store: s, Runner: r}
		e = svc.Run(context.Background(), j, v, []int64{1, 4})
		if missing {
			if e == nil || e.Error() != "FRAME_UNAVAILABLE" {
				t.Fatal("tail silently accepted", e)
			}
			frames, _ := s.Frames(v.ID, "")
			if len(frames) > 0 {
				t.Fatal("partial batch published")
			}
		} else {
			if e != nil {
				t.Fatal(e)
			}
			frames, e := s.Frames(v.ID, "b")
			if e != nil || len(frames) != 2 || frames[0].Width != 3 || frames[0].Height != 2 || frames[1].TimestampMs != 4 {
				t.Fatal(frames, e)
			}
		}
		entries, _ := os.ReadDir(filepath.Join(s.Root, "staging"))
		if len(entries) > 0 {
			t.Fatal("staging leaked")
		}
		s.DB.Close()
	}
}
