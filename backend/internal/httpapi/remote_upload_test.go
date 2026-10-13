package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaagent"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

type delayedBytes struct {
	data  []byte
	delay time.Duration
}

func (r *delayedBytes) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	time.Sleep(r.delay)
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

func TestStreamedUploadRenewsShortAgentLease(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	agent, err := mediaagent.New(mediaagent.Config{Root: t.TempDir(), Token: "test-agent-token-123", Runner: flowRunner{}, Lease: 3 * time.Second, MaxOperation: 20 * time.Second, MaxUploadBytes: 1024, MaxFrames: 10})
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
				t.Error("invalid Agent capabilities")
			}
			capabilities["ffprobeAvailable"] = true // Injected runner replaces ffprobe in this fixture.
			_ = json.NewEncoder(w).Encode(capabilities)
			return
		}
		if len(req.URL.Path) > len("/renew") && req.URL.Path[len(req.URL.Path)-len("/renew"):] == "/renew" {
			renewals.Add(1)
		}
		agent.ServeHTTP(w, req)
	}))
	defer host.Close()
	bridge, err := mediabridge.New(mediabridge.Config{BaseURL: host.URL, Token: "test-agent-token-123", AllowedOrigins: []string{host.URL}, Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	sources := &SourceService{Store: s, Bridge: bridge, MaxUploadBytes: 1024, MaxFrames: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	video, err := sources.UploadToMac(ctx, &delayedBytes{data: []byte("synthetic-video"), delay: 400 * time.Millisecond}, "slow.mp4")
	if err != nil || video.StorageProvider != "mac" || renewals.Load() < 2 {
		t.Fatalf("lease renewal: video=%+v renewals=%d error=%v", video, renewals.Load(), err)
	}
}
