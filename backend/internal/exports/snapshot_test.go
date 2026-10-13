package exports

import (
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
)

func TestSnapshotReleasedBeforeZIPAndRemainsImmutable(t *testing.T) {
	s, _, images := fixture(t)
	service := Service{Store: s}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := service.Snapshot(ctx, Selection{FrameIDs: []string{"f1", "zero"}})
	if err != nil {
		t.Fatal(err)
	}
	if inUse := s.DB.Stats().InUse; inUse != 0 {
		t.Fatalf("snapshot retained %d database connections", inUse)
	}
	_, err = (annotations.Repository{Store: s}).Save(ctx, "f1", annotations.Save{ExpectedRevision: 1, Status: "in_progress", Annotations: []domain.Annotation{{ID: "new", Source: "manual", BBox: [4]float64{0, 0, 9, 9}}}})
	if err != nil {
		t.Fatal("snapshot held connection during archive phase", err)
	}
	path, err := service.Build(ctx, records)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/zip")
	w.Write(data)
	doc := inspectZIP(t, w, images, 2, 1)
	if doc.Annotations[0].BBox[2] != 3.5 || doc.Annotations[0].Area != 14 {
		t.Fatal("archive mixed new annotation into captured completed snapshot", doc.Annotations)
	}
}

func TestFrameLimitIsEnforcedWithoutLargeArrays(t *testing.T) {
	s, _, _ := fixture(t)
	service := Service{Store: s, MaxFrames: 1}
	for _, selection := range []Selection{{VideoID: "v"}, {FrameIDs: []string{"f1", "f2"}}} {
		_, err := service.Snapshot(context.Background(), selection)
		if !errors.Is(err, ErrLimit) {
			t.Fatal("frame limit", err)
		}
	}
}

func TestWrongDimensionsFailSafelyAndCleanTemporaryZIP(t *testing.T) {
	s, r, _ := fixture(t)
	f, err := s.Frame("f1")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(s.Root, f.StorageKey))
	if err != nil {
		t.Fatal(err)
	}
	err = jpeg.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	w := export(r, `{"frameIds":["f1"]}`)
	if w.Code != 500 || w.Header().Get("Content-Type") == "application/zip" || strings.Contains(w.Body.String(), s.Root) {
		t.Fatal(w.Code, w.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(s.Root, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}

func TestMetricsUseApplicationRegistryAndFiniteResultLabels(t *testing.T) {
	s, _, _ := fixture(t)
	manager := jobs.New(s, 1, time.Second)
	defer manager.Close()
	r := httpapi.New(config.Config{Version: "test"}, s, manager, nil, nil, nil, func(g *gin.RouterGroup, s *store.Store) {
		annotations.Register(g, s, manager.Metrics.Registry)
		Register(g, s, manager.Metrics.Registry)
	})
	for _, tc := range []struct {
		body string
		code int
	}{{`{"expectedRevision":0,"status":"completed","annotations":[]}`, 409}, {`{"expectedRevision":1,"status":"bad","annotations":[]}`, 422}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/frames/f1/annotations", strings.NewReader(tc.body))
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := export(r, `{"frameIds":["zero"]}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := export(r, `{"frameIds":["missing"]}`); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	for _, line := range []string{"classroom_annotation_save_conflicts_total 1", "classroom_annotation_save_errors_total 1", `classroom_exports_total{result="success"} 1`, `classroom_exports_total{result="not_found"} 1`} {
		if !strings.Contains(w.Body.String(), line) {
			t.Fatalf("metric missing %s", line)
		}
	}
	if strings.Contains(w.Body.String(), s.Root) {
		t.Fatal("path leaked to metrics")
	}
}
