package mediaagent

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestActualFFmpegExtraction(t *testing.T) {
	ffmpeg, ffprobe := os.Getenv("TEST_FFMPEG"), os.Getenv("TEST_FFPROBE")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("explicit runtime paths required")
	}
	input := filepath.Join(t.TempDir(), "synthetic.mp4")
	cmd := exec.Command(ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=64x48:rate=10", "-t", "2", "-threads", "1", "-pix_fmt", "yuv420p", "-y", input)
	if e := cmd.Run(); e != nil {
		t.Fatal(e)
	}
	a, e := New(Config{Root: t.TempDir(), Token: "test-agent-token", FFmpeg: ffmpeg, FFprobe: ffprobe})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	payload, e := os.ReadFile(input)
	if e != nil {
		t.Fatal(e)
	}
	video := uploadFixture(t, a, "actual-upload", payload)
	req := mediaapi.ExtractionRequest{OperationID: "actual-extract", JobID: "job-a", BatchID: "batch-a", VideoID: "video-a", MediaKey: video.MediaKey, TimestampsMs: []int64{0, 500, 1000}, DeadlineSeconds: 600}
	b, _ := json.Marshal(req)
	w := call(a, "POST", "/bridge/v1/extractions", bytes.NewReader(b))
	if w.Code != 202 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	op := waitOperation(t, a, req.OperationID)
	if op.Status != "completed" {
		t.Fatalf("operation %+v", op)
	}
	var manifest mediaapi.ExtractionManifest
	json.Unmarshal(op.Manifest, &manifest)
	if len(manifest.Frames) != 3 {
		t.Fatal(string(op.Manifest))
	}
	for _, frame := range manifest.Frames {
		if frame.Width != 64 || frame.Height != 48 || frame.SizeBytes == 0 {
			t.Fatalf("actual frame %+v", frame)
		}
	}
}
