package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestActualHTTPUploadPublishesLocalBytesAfterProbe(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	cfg := config.Config{MaxUploadBytes: 1024, MaxFrames: 10, JobTimeout: 30 * time.Second, HTTPTimeout: 5 * time.Second}
	m := jobs.New(s, 2, 30*time.Second)
	defer m.Close()
	sources := &SourceService{Store: s, Jobs: m, MaxUploadBytes: 1024, MaxFrames: 10}
	router := NewWithSources(cfg, s, m, &media.Service{Root: s.Root, Limit: 1024, Runner: flowRunner{}}, &extraction.Service{Store: s, Runner: flowRunner{}}, nil, sources)
	apiHost := httptest.NewServer(router)
	defer apiHost.Close()
	var payload bytes.Buffer
	mw := multipart.NewWriter(&payload)
	part, err := mw.CreateFormFile("file", "actual-http.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("synthetic-video")); err != nil {
		t.Fatal(err)
	}
	if err = mw.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, apiHost.URL+"/api/v1/videos", &payload)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", mw.FormDataContentType())
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != 201 {
		t.Fatalf("upload status %d: %s", response.StatusCode, responseBody)
	}
	var video domain.Video
	if json.Unmarshal(responseBody, &video) != nil || video.ID == "" || video.StorageProvider != "local" {
		t.Fatalf("upload video %+v", video)
	}
	persisted, err := s.Video(video.ID)
	if err != nil || persisted.MediaSHA256 == "" || persisted.StorageProvider != "local" {
		t.Fatalf("not published: %+v %v", persisted, err)
	}
}
