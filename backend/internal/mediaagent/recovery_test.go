package mediaagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestRegistrationRetrySurvivesLegacyServiceOffline(t *testing.T) {
	payload := []byte("legacy-synthetic-video")
	hash := fingerprintBytes(payload)
	offline := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if offline {
			w.WriteHeader(503)
			return
		}
		if r.URL.Path == "/v1/media" {
			json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "fixture.mp4", "url": "/v1/media/fixture.mp4", "size_bytes": len(payload), "sha256": hash, "content_type": "video/mp4"}}})
			return
		}
		w.Write(payload)
	}))
	defer upstream.Close()
	a, e := New(Config{Root: t.TempDir(), Token: "test-agent-token", SourceURL: upstream.URL, SourceToken: "old-secret", Runner: syntheticRunner{}})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	w := call(a, "GET", "/bridge/v1/sources", nil)
	var sources mediaapi.Sources
	if e = json.Unmarshal(w.Body.Bytes(), &sources); e != nil || len(sources.Items) != 1 {
		t.Fatal(w.Body.String())
	}
	req := mediaapi.RegistrationRequest{OperationID: "registration-a", VideoID: "video-a", SourceID: sources.Items[0].SourceID}
	b, _ := json.Marshal(req)
	w = call(a, "POST", "/bridge/v1/registrations", bytes.NewReader(b))
	if w.Code != 200 {
		t.Fatalf("registration %d %s", w.Code, w.Body.String())
	}
	original := w.Body.String()
	offline = true
	w = call(a, "POST", "/bridge/v1/registrations", bytes.NewReader(b))
	if w.Code != 200 || w.Body.String() != original {
		t.Fatalf("retry %d %s", w.Code, w.Body.String())
	}
}
func fingerprintBytes(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func TestDeadlineFailsWithTimeout(t *testing.T) {
	a := newTestAgent(t)
	_, e := a.submit("timeout-a", "extraction", "job-a", "batch-a", struct{}{}, "", 100*time.Millisecond, func(ctx context.Context) (any, error) { <-ctx.Done(); return nil, ctx.Err() })
	if e != nil {
		t.Fatal(e)
	}
	op := waitOperation(t, a, "timeout-a")
	if op.Status != "failed" || op.ErrorCode == nil || *op.ErrorCode != "TIMEOUT" {
		t.Fatalf("deadline %+v", op)
	}
}
func TestFailedManifestCommitDoesNotExposePreparedResources(t *testing.T) {
	a := newTestAgent(t)
	stage, dir, e := a.stage()
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "image.jpg"), []byte("immutable prepared bytes"), 0600)
	_, e = a.submit("persist-a", "archive", "", "", struct{}{}, stage, 0, func(context.Context) (any, error) {
		if e := os.Remove(filepath.Join(a.cfg.Root, "manifest.json")); e != nil {
			return nil, e
		}
		if e := os.Mkdir(filepath.Join(a.cfg.Root, "manifest.json"), 0700); e != nil {
			return nil, e
		}
		return preparedOutput{Manifest: map[string]string{"mediaKey": "prepared-key"}, Resources: map[string]resource{"prepared-key": {Path: "image.jpg", ContentType: "image/jpeg", SizeBytes: 24}}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	op := waitOperation(t, a, "persist-a")
	if op.Status != "failed" || len(op.Manifest) != 0 {
		t.Fatalf("failed operation disclosed outputs %+v", op)
	}
	w := call(a, "GET", "/bridge/v1/media/prepared-key", nil)
	if w.Code != 404 {
		t.Fatalf("uncommitted prepared resource exposed %d", w.Code)
	}
}
func TestRunningCancelWaitsForWorkerCleanup(t *testing.T) {
	a := newTestAgent(t)
	stage, dir, e := a.stage()
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "partial"), []byte("partial"), 0600)
	started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	_, e = a.submit("running-cancel", "extraction", "job-a", "batch-a", struct{}{}, stage, 0, func(ctx context.Context) (any, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		<-release
		return nil, ctx.Err()
	})
	if e != nil {
		t.Fatal(e)
	}
	<-started
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response <- call(a, "POST", "/bridge/v1/operations/running-cancel/cancel", bytes.NewBufferString("{}"))
	}()
	<-stopped
	select {
	case <-response:
		t.Fatal("cancel returned before cleanup")
	default:
	}
	a.mu.Lock()
	status := a.state.Operations["running-cancel"].Status
	a.mu.Unlock()
	if status != "running" {
		t.Fatalf("premature terminal %s", status)
	}
	close(release)
	if w := <-response; w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, e = os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("staging not cleaned")
	}
	if op := waitOperation(t, a, "running-cancel"); op.Status != "cancelled" {
		t.Fatal(op.Status)
	}
}
