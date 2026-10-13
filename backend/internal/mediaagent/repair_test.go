package mediaagent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestSameLengthMediaMutationRejectsReadAndExtraction(t *testing.T) {
	a := newTestAgent(t)
	video := uploadFixture(t, a, "mutation-upload", []byte("original-video"))
	a.mu.Lock()
	path := a.state.Resources[video.MediaKey].Path
	a.mu.Unlock()
	if err := os.WriteFile(filepath.Join(a.cfg.Root, filepath.FromSlash(path)), []byte("modified-video"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "HEAD"} {
		for _, rangeValue := range []string{"", "bytes=0-3"} {
			r := httptest.NewRequest(method, "/bridge/v1/media/"+video.MediaKey, nil)
			r.Header.Set("Authorization", "Bearer test-agent-token")
			if rangeValue != "" {
				r.Header.Set("Range", rangeValue)
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != 409 || w.Header().Get("ETag") != "" {
				t.Fatalf("%s %s: %d %v", method, rangeValue, w.Code, w.Header())
			}
		}
	}
	req := mediaapi.ExtractionRequest{OperationID: "mutation-extract", JobID: "job-a", BatchID: "batch-a", VideoID: "video-a", MediaKey: video.MediaKey, TimestampsMs: []int64{0}, DeadlineSeconds: 600}
	b, _ := json.Marshal(req)
	w := call(a, "POST", "/bridge/v1/extractions", bytes.NewReader(b))
	if w.Code != 409 {
		t.Fatalf("extraction accepted modified input: %d %s", w.Code, w.Body.String())
	}
	a.mu.Lock()
	resourceCount := len(a.state.Resources)
	a.mu.Unlock()
	if resourceCount != 1 {
		t.Fatalf("published %d resources", resourceCount)
	}
}

func TestPausedUploadCanBeObservedCancelledAndCleaned(t *testing.T) {
	a := newTestAgent(t)
	reader, writer := io.Pipe()
	m := multipart.NewWriter(writer)
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", reader)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", "paused-upload")
	r.Header.Set("X-Video-ID", "video-a")
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); a.ServeHTTP(w, r); result <- w }()
	part, err := m.CreateFormFile("file", "paused.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("partial-video")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w := call(a, "GET", "/bridge/v1/operations/paused-upload", nil); w.Code == 200 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := call(a, "GET", "/bridge/v1/operations/paused-upload", nil); w.Code != 200 {
		t.Fatalf("upload invisible while receiving: %d", w.Code)
	}
	cancelled := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		cancelled <- call(a, "POST", "/bridge/v1/operations/paused-upload/cancel", bytes.NewBufferString("{}"))
	}()
	select {
	case w := <-cancelled:
		if w.Code != 200 {
			t.Fatalf("cancel %d %s", w.Code, w.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel blocked on paused upload")
	}
	_ = writer.Close()
	select {
	case <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("upload did not stop")
	}
	op := waitOperation(t, a, "paused-upload")
	if op.Status != "cancelled" {
		t.Fatalf("status %s", op.Status)
	}
	a.mu.Lock()
	stage := a.state.Operations["paused-upload"].Staging
	a.mu.Unlock()
	if stage != "" {
		t.Fatalf("retained staging %s", stage)
	}
}

func TestSameUploadOperationChangedBodyConflicts(t *testing.T) {
	a := newTestAgent(t)
	uploadFixture(t, a, "replay-upload", []byte("first-content"))
	op := waitOperation(t, a, "replay-upload")
	if op.RequestFingerprint != fingerprint(mediaapi.UploadIntent{VideoID: "video-a", Name: "synthetic.mp4"}) {
		t.Fatalf("fingerprint changed after upload: %s", op.RequestFingerprint)
	}
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	part, _ := m.CreateFormFile("file", "synthetic.mp4")
	part.Write([]byte("other-content"))
	m.Close()
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", &b)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", "replay-upload")
	r.Header.Set("X-Video-ID", "video-a")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("changed replay %d %s", w.Code, w.Body.String())
	}
}

func TestInterruptedReceivingUploadRecoveryCleansStaging(t *testing.T) {
	a := newTestAgent(t)
	reader, writer := io.Pipe()
	m := multipart.NewWriter(writer)
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", reader)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", "crashed-upload")
	r.Header.Set("X-Video-ID", "video-a")
	completed := make(chan struct{})
	go func() { a.ServeHTTP(httptest.NewRecorder(), r); close(completed) }()
	part, err := m.CreateFormFile("file", "crashed.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("partial-video")); err != nil {
		t.Fatal(err)
	}
	var stage string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		if op := a.state.Operations["crashed-upload"]; op != nil {
			stage = op.Staging
		}
		a.mu.Unlock()
		if stage != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stage == "" {
		t.Fatal("receiving operation was not persisted")
	}
	crashRoot := t.TempDir()
	if err = os.MkdirAll(filepath.Join(crashRoot, filepath.FromSlash(stage)), 0700); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(a.cfg.Root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(crashRoot, "manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(crashRoot, filepath.FromSlash(stage), "video"), []byte("partial-video"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(Config{Root: crashRoot, Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	op := waitOperation(t, restarted, "crashed-upload")
	if op.Status != "failed" || op.ErrorCode == nil || *op.ErrorCode != "PROCESS_FAILED" {
		t.Fatalf("recovered operation %+v", op)
	}
	if _, err = os.Stat(filepath.Join(crashRoot, filepath.FromSlash(stage))); !os.IsNotExist(err) {
		t.Fatalf("partial staging survived restart: %v", err)
	}
	_ = writer.Close()
	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("old receiver did not stop")
	}
}

func TestCloseStopsPausedUploadBeforeReturning(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	m := multipart.NewWriter(writer)
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", reader)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", "closing-upload")
	r.Header.Set("X-Video-ID", "video-a")
	completed := make(chan struct{})
	go func() { a.ServeHTTP(httptest.NewRecorder(), r); close(completed) }()
	part, err := m.CreateFormFile("file", "closing.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("partial-video")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if w := call(a, "GET", "/bridge/v1/operations/closing-upload", nil); w.Code == 200 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { a.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("close blocked on client body")
	}
	select {
	case <-completed:
	case <-time.After(3 * time.Second):
		t.Fatal("receiver survived close")
	}
	_ = writer.Close()
	a.mu.Lock()
	op := a.state.Operations["closing-upload"]
	a.mu.Unlock()
	if op == nil || op.Status != "failed" || op.ErrorCode == nil || *op.ErrorCode != "PROCESS_FAILED" || op.Staging != "" {
		t.Fatalf("close outcome %+v", op)
	}
}

func TestFailedCleanupRetainsOwnershipUntilRestartRetry(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows open-handle deletion fixture")
	}
	root := t.TempDir()
	a, err := New(Config{Root: root, Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	stage, dir, err := a.stage()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "held"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.submit("cleanup-retry", "archive", "", "", struct{}{}, stage, 0, func(context.Context) (any, error) { return nil, os.ErrInvalid }); err != nil {
		t.Fatal(err)
	}
	op := waitOperation(t, a, "cleanup-retry")
	if op.Status != "failed" || op.ErrorCode == nil || *op.ErrorCode != "CLEANUP_FAILED" {
		t.Fatalf("cleanup result %+v", op)
	}
	a.mu.Lock()
	retained := a.state.Operations["cleanup-retry"].Staging
	a.mu.Unlock()
	if retained != stage {
		t.Fatalf("cleanup ownership lost: %q", retained)
	}
	a.Close()
	f.Close()
	restarted, err := New(Config{Root: root, Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.mu.Lock()
	retained = restarted.state.Operations["cleanup-retry"].Staging
	restarted.mu.Unlock()
	if retained != "" {
		t.Fatalf("restart retained stage after deletion became possible: %q", retained)
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}
}

func TestFailedCleanupCanBeExplicitlyRetried(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows open-handle deletion fixture")
	}
	a := newTestAgent(t)
	stage, dir, err := a.stage()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "held"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.submit("cleanup-explicit", "archive", "", "", struct{}{}, stage, 0, func(context.Context) (any, error) { return nil, os.ErrInvalid }); err != nil {
		t.Fatal(err)
	}
	op := waitOperation(t, a, "cleanup-explicit")
	if op.ErrorCode == nil || *op.ErrorCode != "CLEANUP_FAILED" {
		t.Fatalf("cleanup result %+v", op)
	}
	a.stop()
	a.wg.Wait()
	f.Close()
	w := call(a, "POST", "/bridge/v1/operations/cleanup-explicit/cancel", bytes.NewBufferString("{}"))
	if w.Code != 200 {
		t.Fatalf("retry %d %s", w.Code, w.Body.String())
	}
	a.mu.Lock()
	retained := a.state.Operations["cleanup-explicit"].Staging
	status := a.state.Operations["cleanup-explicit"].Status
	a.mu.Unlock()
	if retained != "" || status != "failed" {
		t.Fatalf("retry claimed false cancellation: %q %s", retained, status)
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}
}

type configuredDataset struct{}

func (configuredDataset) Lessons(context.Context, int, int) (any, error)          { return nil, nil }
func (configuredDataset) Download(context.Context, string) (io.ReadCloser, error) { return nil, nil }

func TestCapabilitiesExposeConfiguredEntrancesWithoutUpstreamRequest(t *testing.T) {
	a := newTestAgent(t)
	if c := a.capabilities(); c.DatasetConfigured || c.SourceConfigured {
		t.Fatalf("unexpected configured entrance %+v", c)
	}
	b, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}, SourceURL: "http://127.0.0.1:1", SourceToken: "synthetic", Dataset: configuredDataset{}})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if c := b.capabilities(); !c.DatasetConfigured || !c.SourceConfigured {
		t.Fatalf("missing configured entrance %+v", c)
	}
}
