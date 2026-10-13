package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaagent"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type flowRunner struct{}

func (flowRunner) Probe(context.Context, string) (media.Metadata, error) {
	return media.Metadata{Width: 16, Height: 12, Duration: 1}, nil
}
func (flowRunner) Extract(ctx context.Context, in string, ms int64, out string) error {
	f, e := os.Create(out)
	if e != nil {
		return e
	}
	defer f.Close()
	return jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 12, 16)), nil)
}
func TestSyntheticHTTPUploadExtractionAndCompanyImport(t *testing.T) {
	mediaHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			t.Error("key forwarded to media")
		}
		fmt.Fprint(w, "synthetic-video")
	}))
	defer mediaHost.Close()
	apiHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/catalog-files/9/play-url" {
			t.Error("wrong play path")
		}
		if r.Header.Get("X-API-Key") != "fixture-key" {
			t.Error("API key missing")
		}
		fmt.Fprintf(w, `{"url":%q}`, mediaHost.URL)
	}))
	defer apiHost.Close()
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	cfg := config.Config{MaxUploadBytes: 1024, MaxFrames: 10, JobTimeout: 15 * time.Second, HTTPTimeout: time.Second}
	m := jobs.New(s, 3, 15*time.Second)
	defer m.Close()
	vsvc := &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: flowRunner{}}
	agent, e := mediaagent.New(mediaagent.Config{Root: t.TempDir(), Token: "test-agent-token-123", Runner: flowRunner{}, Dataset: mediaagent.NewDatasetAdapter(apiHost.URL, "fixture", "fixture-key", []string{apiHost.URL, mediaHost.URL}, []string{mediaHost.URL})})
	if e != nil {
		t.Fatal(e)
	}
	defer agent.Close()
	agentHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorded := httptest.NewRecorder()
		agent.ServeHTTP(recorded, req)
		if req.URL.Path == "/bridge/v1/capabilities" && recorded.Code == 200 {
			var capabilities map[string]any
			if json.Unmarshal(recorded.Body.Bytes(), &capabilities) != nil {
				t.Fatal("invalid capabilities")
			}
			capabilities["ffmpegAvailable"] = true
			capabilities["ffprobeAvailable"] = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(capabilities)
			return
		}
		for key, values := range recorded.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(recorded.Code)
		_, _ = w.Write(recorded.Body.Bytes())
	}))
	defer agentHost.Close()
	bridge, e := mediabridge.New(mediabridge.Config{BaseURL: agentHost.URL, Token: "test-agent-token-123", AllowedOrigins: []string{agentHost.URL}, Timeout: 15 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	sources := &SourceService{Store: s, Jobs: m, Bridge: bridge, MaxUploadBytes: cfg.MaxUploadBytes, MaxFrames: cfg.MaxFrames}
	r := NewWithSources(cfg, s, m, vsvc, &extraction.Service{Store: s, Runner: flowRunner{}}, nil, sources)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	var payload bytes.Buffer
	mw := multipart.NewWriter(&payload)
	part, e := mw.CreateFormFile("file", "../../unsafe.mp4")
	if e != nil {
		t.Fatal(e)
	}
	part.Write([]byte("synthetic-video"))
	mw.Close()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/videos", &payload)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var video domain.Video
	if e = json.Unmarshal(w.Body.Bytes(), &video); e != nil {
		t.Fatal(e)
	}
	if video.Name != "unsafe.mp4" || strings.Contains(w.Body.String(), s.Root) {
		t.Fatal(video, w.Body.String())
	}
	w = call("POST", "/api/v1/videos/"+video.ID+"/extractions", `{"intervalSeconds":0.25}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var job domain.Job
	json.Unmarshal(w.Body.Bytes(), &job)
	wait := func(id string) domain.Job {
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			j, e := s.Job(id)
			if e != nil {
				t.Fatal(e)
			}
			if j.Status == "completed" {
				return j
			}
			if j.Status == "failed" {
				t.Fatal(j)
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("job did not finish")
		return domain.Job{}
	}
	wait(job.ID)
	frames, e := s.Frames(video.ID, job.BatchID)
	if e != nil || len(frames) != 4 || frames[3].TimestampMs != 750 || frames[0].Width != 12 || frames[0].Height != 16 {
		t.Fatal(frames, e)
	}
	w = call("GET", frames[0].ImageURL, "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Header())
	}
	w = call("GET", "/api/v1/videos/"+video.ID+"/progress?batchId="+job.BatchID, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"notStarted":4`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/api/v1/dataset/imports", `{"catalogFileId":"9","name":"fixture import"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "videoId") || strings.Contains(w.Body.String(), "batchId") {
		t.Fatal("import fields exposed before success", w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &job)
	done := wait(job.ID)
	if done.VideoID == "" || done.BatchID != "" {
		t.Fatal(done)
	}
	w = call("POST", "/api/v1/videos/"+video.ID+"/extractions", `{"intervalSeconds":1,"unknown":true}`)
	if w.Code != 400 {
		t.Fatal("unknown property accepted", w.Code)
	}
}
