package httpapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type invalidLocalProbe struct{ called *bool }

func (p invalidLocalProbe) Probe(context.Context, string) (media.Metadata, error) {
	*p.called = true
	return media.Metadata{}, errors.New("invalid fixture")
}
func (p invalidLocalProbe) Extract(context.Context, string, int64, string) error { return nil }

func TestLocalUploadReachesSameHostProbeWithoutBridge(t *testing.T) {
	called := false
	a := API{Cfg: config.Config{MaxUploadBytes: 1024, JobTimeout: time.Second}, Media: &media.Service{Root: t.TempDir(), Limit: 1024, Runner: invalidLocalProbe{&called}}}
	r := gin.New()
	r.POST("/videos", a.upload)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	p, _ := w.CreateFormFile("file", "fixture.mp4")
	p.Write([]byte("synthetic"))
	w.Close()
	req := httptest.NewRequest("POST", "/videos", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	out := httptest.NewRecorder()
	r.ServeHTTP(out, req)
	if !called || out.Code != 422 || !strings.Contains(out.Body.String(), "INVALID_VIDEO") {
		t.Fatalf("local probe=%v response=%d %s", called, out.Code, out.Body.String())
	}
}

func TestLocalStorageAvailableWithoutMacBridge(t *testing.T) {
	s := &SourceService{Store: &store.Store{Root: t.TempDir()}}
	r := gin.New()
	s.Register(r.Group("/api/v1"))
	out := httptest.NewRecorder()
	r.ServeHTTP(out, httptest.NewRequest("GET", "/api/v1/media-storage/status", nil))
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"sourceKind":"upload","available":true`) || !strings.Contains(out.Body.String(), "REMOTE_STORAGE_RETIRED") {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
}

func TestLocalCatalogRouteReturnsExplicitConfigurationFailure(t *testing.T) {
	s := &SourceService{}
	r := gin.New()
	s.Register(r.Group("/api/v1"))
	out := httptest.NewRecorder()
	r.ServeHTTP(out, httptest.NewRequest("GET", "/api/v1/sources/local", nil))
	if out.Code != 503 || !strings.Contains(out.Body.String(), "LOCAL_SOURCES_NOT_CONFIGURED") {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
}
