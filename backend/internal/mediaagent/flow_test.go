package mediaagent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

type syntheticRunner struct{}

func (syntheticRunner) Probe(context.Context, string) (media.Metadata, error) {
	return media.Metadata{Width: 32, Height: 24, Duration: 2}, nil
}
func (syntheticRunner) Extract(ctx context.Context, input string, ms int64, output string) error {
	f, e := os.Create(output)
	if e != nil {
		return e
	}
	defer f.Close()
	im := image.NewRGBA(image.Rect(0, 0, 32, 24))
	im.Set(0, 0, color.RGBA{R: uint8(ms / 10), A: 255})
	return jpeg.Encode(f, im, nil)
}
func newTestAgent(t *testing.T) *Agent {
	t.Helper()
	a, e := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func call(a *Agent, method, path string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func waitOperation(t *testing.T, a *Agent, id string) mediaapi.Operation {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		w := call(a, "GET", "/bridge/v1/operations/"+id, nil)
		if w.Code != 200 {
			t.Fatalf("operation %d: %s", w.Code, w.Body.String())
		}
		var op mediaapi.Operation
		if e := json.Unmarshal(w.Body.Bytes(), &op); e != nil {
			t.Fatal(e)
		}
		if op.Status != "queued" && op.Status != "running" {
			return op
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation did not finish")
	return mediaapi.Operation{}
}
func uploadFixture(t *testing.T, a *Agent, id string, payload []byte) mediaapi.Registration {
	t.Helper()
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	part, e := m.CreateFormFile("file", "synthetic.mp4")
	if e != nil {
		t.Fatal(e)
	}
	part.Write(payload)
	m.Close()
	r := httptest.NewRequest("POST", "/bridge/v1/uploads", &b)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("X-Operation-ID", id)
	r.Header.Set("X-Video-ID", "video-a")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("upload %d: %s", w.Code, w.Body.String())
	}
	var v mediaapi.Registration
	if e = json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func TestUploadExtractionArchiveRangeAndRestart(t *testing.T) {
	a := newTestAgent(t)
	video := uploadFixture(t, a, "upload-a", []byte("synthetic-video"))
	if video.MediaKey == "" || video.Width != 32 {
		t.Fatalf("video %+v", video)
	}
	video2 := uploadFixture(t, a, "upload-a", []byte("synthetic-video"))
	if video.MediaKey != video2.MediaKey {
		t.Fatal("upload retry changed identity")
	}
	req := mediaapi.ExtractionRequest{OperationID: "extract-a", JobID: "job-a", BatchID: "batch-a", VideoID: "video-a", MediaKey: video.MediaKey, TimestampsMs: []int64{0, 1000}, DeadlineSeconds: 600}
	b, _ := json.Marshal(req)
	w := call(a, "POST", "/bridge/v1/extractions", bytes.NewReader(b))
	if w.Code != 202 {
		t.Fatalf("extract %d %s", w.Code, w.Body.String())
	}
	op := waitOperation(t, a, "extract-a")
	if op.Status != "completed" {
		t.Fatalf("extraction %+v", op)
	}
	var frames mediaapi.ExtractionManifest
	json.Unmarshal(op.Manifest, &frames)
	if len(frames.Frames) != 2 {
		t.Fatal(string(op.Manifest))
	}
	frame := frames.Frames[0]
	w = call(a, "GET", "/bridge/v1/media/"+frame.MediaKey, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	img, err := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != frame.Width {
		t.Fatal("invalid actual frame bytes")
	}
	images := []mediaapi.ArchiveImage{}
	for _, f := range frames.Frames {
		images = append(images, mediaapi.ArchiveImage{FrameID: f.FrameID, MediaKey: f.MediaKey, SHA256: f.SHA256, SizeBytes: f.SizeBytes, Width: f.Width, Height: f.Height})
	}
	ar := mediaapi.ArchiveRequest{OperationID: "archive-a", Images: images, COCO: json.RawMessage(`{"images":[],"annotations":[],"categories":[]}`)}
	b, _ = json.Marshal(ar)
	w = call(a, "POST", "/bridge/v1/archives", bytes.NewReader(b))
	if w.Code != 202 {
		t.Fatalf("archive %d %s", w.Code, w.Body.String())
	}
	op = waitOperation(t, a, "archive-a")
	if op.Status != "completed" {
		t.Fatalf("archive %+v", op)
	}
	var archive mediaapi.ArchiveManifest
	json.Unmarshal(op.Manifest, &archive)
	w = call(a, "GET", "/bridge/v1/media/"+archive.MediaKey, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(zr.File) != 3 {
		t.Fatalf("zip %+v %v", zr, err)
	}
	r := httptest.NewRequest("GET", "/bridge/v1/media/"+archive.MediaKey, nil)
	r.Header.Set("Authorization", "Bearer test-agent-token")
	r.Header.Set("Range", "bytes=0-9")
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.Len() != 10 {
		t.Fatalf("range %d %d", w.Code, w.Body.Len())
	}
	w = call(a, "HEAD", "/bridge/v1/media/"+archive.MediaKey, nil)
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatalf("head %d %v", w.Code, w.Header())
	}
	root := a.cfg.Root
	a.Close()
	restarted, e := New(Config{Root: root, Token: "test-agent-token", Runner: syntheticRunner{}})
	if e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()
	w = call(restarted, "GET", "/bridge/v1/media/"+frame.MediaKey, nil)
	if w.Code != 200 {
		t.Fatal("frame lost after restart")
	}
}

func TestQueuedCancelAndExpiredLeaseCannotExecute(t *testing.T) {
	a := newTestAgent(t)
	block := make(chan struct{})
	started := make(chan struct{})
	_, e := a.submit("block", "extraction", "job-a", "batch-a", struct{}{}, "", time.Minute, func(ctx context.Context) (any, error) {
		close(started)
		select {
		case <-block:
			return struct{}{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	<-started
	executed := false
	_, e = a.submit("queued", "archive", "", "", struct{}{}, "", time.Minute, func(context.Context) (any, error) { executed = true; return struct{}{}, nil })
	if e != nil {
		t.Fatal(e)
	}
	w := call(a, "POST", "/bridge/v1/operations/queued/cancel", bytes.NewBufferString("{}"))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	_, e = a.submit("expired", "archive", "", "", struct{}{}, "", time.Minute, func(context.Context) (any, error) { executed = true; return struct{}{}, nil })
	if e != nil {
		t.Fatal(e)
	}
	a.mu.Lock()
	a.state.Operations["expired"].LeaseUntil = time.Now().Add(-time.Second)
	a.mu.Unlock()
	close(block)
	if op := waitOperation(t, a, "expired"); op.Status != "failed" {
		t.Fatalf("expired %+v", op)
	}
	if executed {
		t.Fatal("cancelled or expired queued task executed")
	}
}

func TestControlRejectsOversizeAndUnknownFields(t *testing.T) {
	a := newTestAgent(t)
	a.cfg.MaxControlBytes = 32
	w := call(a, "POST", "/bridge/v1/extractions", bytes.NewBufferString(`{"operationId":"a","unexpected":"`+string(bytes.Repeat([]byte("x"), 100))+`"}`))
	if w.Code != 413 {
		t.Fatalf("oversize %d", w.Code)
	}
}
