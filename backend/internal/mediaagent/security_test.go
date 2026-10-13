package mediaagent

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestUploadLimitDoesNotPublish(t *testing.T) {
	a := newTestAgent(t)
	a.cfg.MaxUploadBytes = 8
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	part, _ := m.CreateFormFile("file", "a.mp4")
	part.Write([]byte("0123456789"))
	m.Close()
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", &b)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", "oversize")
	r.Header.Set("X-Video-ID", "video-a")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatalf("upload limit %d %s", w.Code, w.Body.String())
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.state.Resources) != 0 || len(a.state.Operations) != 0 {
		t.Fatal("oversize input published")
	}
}
func TestArchiveTTLOnlyRemovesZIP(t *testing.T) {
	a := newTestAgent(t)
	video := uploadFixture(t, a, "ttl-upload", []byte("video"))
	a.mu.Lock()
	r := a.state.Resources[video.MediaKey]
	r.ExpiresAt = time.Now().Add(-time.Hour)
	a.state.Resources[video.MediaKey] = r
	p := filepath.Join(a.cfg.Root, "objects", "expired.zip")
	os.WriteFile(p, []byte("zip"), 0600)
	a.state.Resources["expired-zip"] = resource{Path: "objects/expired.zip", ContentType: "application/zip", SizeBytes: 3, ExpiresAt: time.Now().Add(-time.Hour)}
	a.mu.Unlock()
	time.Sleep(350 * time.Millisecond)
	a.mu.Lock()
	_, zipExists := a.state.Resources["expired-zip"]
	_, videoExists := a.state.Resources[video.MediaKey]
	a.mu.Unlock()
	if zipExists || !videoExists {
		t.Fatal("TTL cleaned non-export resource or retained expired ZIP")
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("expired ZIP file retained")
	}
	if w := call(a, "GET", "/bridge/v1/media/"+video.MediaKey, nil); w.Code != 200 {
		t.Fatalf("video expired by ZIP TTL %d", w.Code)
	}
}
func TestMediaRejectsUnregisteredPathsAndMultipleRanges(t *testing.T) {
	a := newTestAgent(t)
	for _, path := range []string{"../manifest.json", "%2e%2e%2fmanifest.json", "unknown"} {
		w := call(a, "GET", "/bridge/v1/media/"+path, nil)
		if w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
	video := uploadFixture(t, a, "range-upload", []byte("video"))
	r := httptest.NewRequest("GET", "/bridge/v1/media/"+video.MediaKey, nil)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Range", "bytes=0-1,3-4")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 416 {
		t.Fatal(w.Code)
	}
}
func TestCompanyAdapterProjectsSafeDTOAndKeepsKeyOffMediaRequest(t *testing.T) {
	key := "synthetic-company-key"
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			t.Error("company key sent to media host")
		}
		w.Write([]byte("synthetic-video"))
	}))
	defer mediaServer.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != key {
			t.Error("missing configured key")
		}
		if strings.HasSuffix(r.URL.Path, "play-url") {
			json.NewEncoder(w).Encode(map[string]string{"url": mediaServer.URL + "/media?signature=private-synthetic-value"})
			return
		}
		w.Write([]byte(`{"type":"lessons","items":[{"id":1,"course_name":"synthetic","organization_name":"org","teacher":"teacher","date":"2026-10-13","section_index":1,"source_page_url":"private-synthetic-url","videos":[{"id":2,"lesson_id":1,"catalog_file_id":3,"channel":4,"part_index":1,"tags":{"apiKey":"private-synthetic-value"}}]}],"total":1,"stats":{"secret":"private-synthetic-value"}}`))
	}))
	defer api.Close()
	a := newTestAgent(t)
	a.cfg.Dataset = NewDatasetAdapter(api.URL, "synthetic-dataset", key, []string{api.URL, mediaServer.URL}, []string{mediaServer.URL})
	w := call(a, "GET", "/bridge/v1/dataset/lessons?page=1&page_size=20", nil)
	if w.Code != 200 {
		t.Fatalf("list %d %s", w.Code, w.Body.String())
	}
	for _, forbidden := range []string{"tags", "stats", "source_page_url", "private-synthetic", "play-url", "apiKey"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("unsafe field returned %s", forbidden)
		}
	}
	req := mediaapi.DatasetImportRequest{OperationID: "dataset-a", JobID: "job-a", VideoID: "video-a", CatalogFileID: "3", Name: "synthetic"}
	b, _ := json.Marshal(req)
	w = call(a, "POST", "/bridge/v1/dataset/imports", bytes.NewReader(b))
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	op := waitOperation(t, a, req.OperationID)
	if op.Status != "completed" || op.OperationType != "dataset_import" || op.JobID == nil {
		t.Fatalf("import %+v", op)
	}
}
