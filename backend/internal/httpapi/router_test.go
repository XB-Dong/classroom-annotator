package httpapi

import (
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthAndUnconfiguredDatasetReturnContract(t *testing.T) {
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
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
	}{{"/health/ready", 200, `"version":"test"`}, {"/api/v1/dataset/lessons", 503, "MEDIA_SOURCE_UNAVAILABLE"}, {"/api/v1/jobs", 200, `"jobs":[]`}, {"/metrics", 200, "classroom_job_queue_depth"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.fragment) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	if err := s.DB.Close(); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "NOT_READY") {
		t.Fatal("database outage did not fail readiness", w.Code, w.Body.String())
	}
}
func TestCustomMethodsShareBoundedMetricLabel(t *testing.T) {
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	m := jobs.New(s, 1, time.Second)
	defer m.Close()
	cfg := config.Config{}
	r := New(cfg, s, m, &media.Service{}, &extraction.Service{}, dataset.New(cfg))
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(fmt.Sprintf("CUSTOM%d", i), "/missing", nil))
	}
	families, e := m.Metrics.Registry.Gather()
	if e != nil {
		t.Fatal(e)
	}
	for _, family := range families {
		if family.GetName() == "classroom_http_requests_total" || family.GetName() == "classroom_http_request_duration_seconds" {
			if len(family.Metric) != 1 {
				t.Fatalf("%s contains %d series for custom verbs", family.GetName(), len(family.Metric))
			}
			for _, label := range family.Metric[0].Label {
				if label.GetName() == "method" && label.GetValue() != "other" {
					t.Fatal(label)
				}
			}
		}
	}
}
func TestMalformedMultipartReturnsClientError(t *testing.T) {
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
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
