package httpapi

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthAndUnconfiguredDatasetReturnContract(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	m := jobs.New(s, 2, time.Second)
	defer m.Close()
	cfg := config.Config{Version: "test", HTTPTimeout: time.Second}
	r := New(cfg, s, m, &media.Service{}, &extraction.Service{}, dataset.New(cfg))
	for _, tc := range []struct {
		path     string
		code     int
		fragment string
	}{{"/health/ready", 200, `"version":"test"`}, {"/api/v1/dataset/lessons", 503, "DATASET_NOT_CONFIGURED"}, {"/api/v1/jobs", 200, `"jobs":[]`}, {"/metrics", 200, "classroom_job_queue_depth"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.fragment) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}
func TestMalformedMultipartReturnsClientError(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	m := jobs.New(s, 1, time.Second)
	defer m.Close()
	cfg := config.Config{MaxUploadBytes: 100, JobTimeout: time.Second}
	r := New(cfg, s, m, &media.Service{}, &extraction.Service{}, dataset.New(cfg))
	req := httptest.NewRequest("POST", "/api/v1/videos", strings.NewReader("--x\r\nBROKEN HEADER\r\n\r\nvideo\r\n--x--\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
}
