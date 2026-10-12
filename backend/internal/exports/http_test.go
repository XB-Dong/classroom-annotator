package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func fixture(t *testing.T) (*store.Store, *gin.Engine, map[string][]byte) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	for _, id := range []string{"v", "other"} {
		if err = s.AddVideo(domain.Video{ID: id, Name: "synthetic", Width: 30, Height: 20, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/" + id}); err != nil {
			t.Fatal(err)
		}
	}
	images := map[string][]byte{}
	for _, batch := range []struct {
		id, video string
		frames    []string
	}{{"b", "v", []string{"f1", "zero", "unfinished"}}, {"b2", "v", []string{"f2"}}, {"foreign", "other", []string{"otherframe"}}} {
		j := domain.Job{ID: "j" + batch.id, Kind: "extraction", VideoID: batch.video, BatchID: batch.id, Status: "running", CreatedAt: domain.Now()}
		if err = s.AddJob(j); err != nil {
			t.Fatal(err)
		}
		frames := []domain.Frame{}
		for i, id := range batch.frames {
			key := filepath.Join("frames", batch.id, id+".jpg")
			frames = append(frames, domain.Frame{ID: id, VideoID: batch.video, BatchID: batch.id, TimestampMs: int64(i), Width: 30, Height: 20, StorageKey: key})
			img := image.NewRGBA(image.Rect(0, 0, 30, 20))
			img.Set(0, 0, color.RGBA{R: 255, A: 255})
			var buf bytes.Buffer
			if err = jpeg.Encode(&buf, img, nil); err != nil {
				t.Fatal(err)
			}
			images[id] = append([]byte(nil), buf.Bytes()...)
			if err = os.MkdirAll(filepath.Dir(filepath.Join(s.Root, key)), 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(s.Root, key), buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.PublishBatch(j, frames); err != nil {
			t.Fatal(err)
		}
	}
	repo := annotations.Repository{Store: s}
	for _, id := range []string{"f1", "f2", "zero", "otherframe"} {
		anns := []domain.Annotation{}
		if id != "zero" {
			anns = append(anns, domain.Annotation{ID: "a-" + id, BBox: [4]float64{1, 2, 3.5, 4}, Source: "manual"})
		}
		if _, err = repo.Save(context.Background(), id, annotations.Save{ExpectedRevision: 0, Status: "completed", Annotations: anns}); err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r.Group("/api/v1"), s, prometheus.NewRegistry())
	return s, r, images
}

func export(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/exports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// This independent consumer deliberately does not use the producer's COCO types.
type readCOCO struct {
	Images []struct {
		ID       int    `json:"id"`
		FileName string `json:"file_name"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	} `json:"images"`
	Annotations []struct {
		ID         int       `json:"id"`
		ImageID    int       `json:"image_id"`
		CategoryID int       `json:"category_id"`
		BBox       []float64 `json:"bbox"`
		Area       float64   `json:"area"`
		IsCrowd    int       `json:"iscrowd"`
	} `json:"annotations"`
	Categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

func inspectZIP(t *testing.T, w *httptest.ResponseRecorder, expected map[string][]byte, wantImages, wantAnnotations int) readCOCO {
	t.Helper()
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export status %d: %s", w.Code, w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rc, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(rc)
		rc.Close()
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := files[f.Name]; ok {
			t.Fatal("duplicate ZIP filename")
		}
		files[f.Name] = data
	}
	var doc readCOCO
	if err = json.Unmarshal(files["annotations.json"], &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Images) != wantImages || (wantAnnotations >= 0 && len(doc.Annotations) != wantAnnotations) || len(files) != wantImages+1 {
		t.Fatalf("counts images=%d annotations=%d entries=%d", len(doc.Images), len(doc.Annotations), len(files))
	}
	if len(doc.Categories) != 1 || doc.Categories[0].ID != 1 || doc.Categories[0].Name != "head" {
		t.Fatal(doc.Categories)
	}
	ids := map[int]bool{}
	annIDs := map[int]bool{}
	for _, im := range doc.Images {
		if im.ID <= 0 || ids[im.ID] {
			t.Fatal("invalid image ID", im.ID)
		}
		ids[im.ID] = true
		if !strings.HasPrefix(im.FileName, "images/") || !strings.HasSuffix(im.FileName, ".jpg") {
			t.Fatal(im.FileName)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(im.FileName, "images/"), ".jpg")
		if expected[id] == nil || !bytes.Equal(files[im.FileName], expected[id]) {
			t.Fatal("image association or bytes", id)
		}
		img, e := jpeg.Decode(bytes.NewReader(files[im.FileName]))
		if e != nil {
			t.Fatal(e)
		}
		if img.Bounds().Dx() != im.Width || img.Bounds().Dy() != im.Height || im.Width != 30 || im.Height != 20 {
			t.Fatal("actual image dimensions", im)
		}
	}
	for _, a := range doc.Annotations {
		if a.ID <= 0 || annIDs[a.ID] || !ids[a.ImageID] || a.CategoryID != 1 || a.IsCrowd != 0 || len(a.BBox) != 4 || a.Area != a.BBox[2]*a.BBox[3] {
			t.Fatal("annotation invariant", a)
		}
		annIDs[a.ID] = true
		if a.BBox[0] < 0 || a.BBox[1] < 0 || a.BBox[2] <= 0 || a.BBox[3] <= 0 || a.BBox[0]+a.BBox[2] > 30 || a.BBox[1]+a.BBox[3] > 20 {
			t.Fatal("bbox", a)
		}
	}
	return doc
}

func TestExportCompletedFramesAndZeroHeadWithIndependentReader(t *testing.T) {
	s, r, images := fixture(t)
	doc := inspectZIP(t, export(r, `{"videoId":"v","batchId":"b"}`), images, 2, 1)
	for _, im := range doc.Images {
		if im.FileName == "images/unfinished.jpg" {
			t.Fatal("uncompleted exported")
		}
	}
	inspectZIP(t, export(r, `{"videoId":"v"}`), images, 3, 2)
	inspectZIP(t, export(r, `{"frameIds":["zero","unfinished"]}`), images, 1, 0)
	entries, err := os.ReadDir(filepath.Join(s.Root, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatal("temporary ZIP not cleaned", entries, err)
	}
}

func TestExportSelectorsAndMissingImagesFailBeforeZIPHeaders(t *testing.T) {
	s, r, _ := fixture(t)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{}`, 422}, {`{"videoId":"v","frameIds":["f1"]}`, 422}, {`{"batchId":"b","frameIds":["f1"]}`, 422},
		{`{"frameIds":[]}`, 422}, {`{"frameIds":["f1","f1"]}`, 422}, {`{"videoId":""}`, 422},
		{`{"videoId":"absent"}`, 404}, {`{"videoId":"v","batchId":"foreign"}`, 404}, {`{"frameIds":["f1","absent"]}`, 404},
		{`{"frameIds":["unfinished"]}`, 422}, {`{"videoId":"v","batchId":"missing"}`, 404},
		{`{"videoId":"v","unknown":true}`, 400},
	} {
		w := export(r, tc.body)
		if w.Code != tc.code {
			t.Fatalf("%s status %d want %d: %s", tc.body, w.Code, tc.code, w.Body.String())
		}
		if w.Header().Get("Content-Type") == "application/zip" || w.Header().Get("Content-Disposition") != "" {
			t.Fatal("ZIP headers on error")
		}
	}
	f, err := s.Frame("f1")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(s.Root, f.StorageKey)); err != nil {
		t.Fatal(err)
	}
	w := export(r, `{"frameIds":["f1"]}`)
	if w.Code != 500 || strings.Contains(w.Body.String(), s.Root) || w.Header().Get("Content-Type") == "application/zip" {
		t.Fatal(w.Code, w.Body.String(), w.Header())
	}
	entries, err := os.ReadDir(filepath.Join(s.Root, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}

func TestConcurrentExportsAndSavesNeverMixSnapshots(t *testing.T) {
	s, r, images := fixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		repo := annotations.Repository{Store: s}
		for rev := int64(1); rev <= 30; rev++ {
			width := float64(3 + rev%2)
			_, err := repo.Save(context.Background(), "f1", annotations.Save{ExpectedRevision: rev, Status: "completed", Annotations: []domain.Annotation{{ID: "a-f1", BBox: [4]float64{1, 2, width, 4}, Source: "manual"}, {ID: "pair-f1", BBox: [4]float64{2, 3, width, 4}, Source: "manual"}}})
			if err != nil {
				errs <- err
				return
			}
		}
	}()
	for i := 0; i < 30; i++ {
		doc := inspectZIP(t, export(r, `{"frameIds":["f1"]}`), images, 1, -1)
		if len(doc.Annotations) != 1 && len(doc.Annotations) != 2 {
			t.Fatal("partial replace", doc.Annotations)
		}
		if len(doc.Annotations) == 2 && doc.Annotations[0].BBox[2] != doc.Annotations[1].BBox[2] {
			t.Fatal("mixed snapshot", doc.Annotations)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
