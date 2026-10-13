package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaagent"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"github.com/gin-gonic/gin"
)

func TestMacRegistrationRenewsLeaseDuringSlowSourceCopy(t *testing.T) {
	payload := []byte("slow-legacy-video")
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/media" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "fixture.mp4", "url": "/v1/media/fixture.mp4", "size_bytes": len(payload), "sha256": hash, "content_type": "video/mp4"}}})
			return
		}
		w.Header().Set("Content-Length", "17")
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(payload)
	}))
	defer upstream.Close()
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	m := jobs.New(s, 2, 15*time.Second)
	defer m.Close()
	agent, err := mediaagent.New(mediaagent.Config{Root: t.TempDir(), Token: "test-agent-token-123", SourceURL: upstream.URL, SourceToken: "old-secret", Runner: flowRunner{}, Lease: 3 * time.Second, MaxOperation: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	var renewals atomic.Int64
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/bridge/v1/capabilities" {
			recorded := httptest.NewRecorder()
			agent.ServeHTTP(recorded, req)
			var capabilities map[string]any
			if json.Unmarshal(recorded.Body.Bytes(), &capabilities) != nil {
				t.Error("invalid capabilities")
			}
			capabilities["ffprobeAvailable"] = true
			_ = json.NewEncoder(w).Encode(capabilities)
			return
		}
		if strings.HasSuffix(req.URL.Path, "/renew") {
			renewals.Add(1)
		}
		agent.ServeHTTP(w, req)
	}))
	defer host.Close()
	bridge, err := mediabridge.New(mediabridge.Config{BaseURL: host.URL, Token: "test-agent-token-123", AllowedOrigins: []string{host.URL}, Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	sources := &SourceService{Store: s, Jobs: m, Bridge: bridge, MaxUploadBytes: 1024, MaxFrames: 10}
	listed, err := bridge.Sources(context.Background(), "")
	if err != nil || len(listed.Items) != 1 {
		t.Fatalf("sources: %+v %v", listed, err)
	}
	r := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/media-sources/mac/imports", strings.NewReader(`{"sourceId":"`+listed.Items[0].SourceID+`"}`))
	request.Header.Set("Content-Type", "application/json")
	// The handler is registered through the production SourceService route.
	router := gin.New()
	sources.Register(router.Group("/api/v1"))
	router.ServeHTTP(r, request)
	if r.Code != 202 {
		t.Fatalf("submit %d %s", r.Code, r.Body.String())
	}
	var submitted struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(r.Body.Bytes(), &submitted) != nil {
		t.Fatal("invalid job response")
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		j, err := s.Job(submitted.ID)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status == "completed" {
			if renewals.Load() < 2 {
				t.Fatalf("registration completed without lease renewals: %d", renewals.Load())
			}
			return
		}
		if j.Status == "failed" {
			t.Fatalf("registration failed after %d renewals: %+v", renewals.Load(), j)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("registration did not complete")
}
