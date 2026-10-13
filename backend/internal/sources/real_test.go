package sources

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealFFmpegLocalCatalogueManagedCopyAndJPEG(t *testing.T) {
	ffmpeg, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("FFmpeg unavailable")
	}
	ffprobe, e := exec.LookPath("ffprobe")
	if e != nil {
		t.Skip("ffprobe unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := t.TempDir()
	input := filepath.Join(root, "fixture.mp4")
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:size=32x24:rate=10:duration=1", "-c:v", "mpeg4", "-threads", "1", "-y", input)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("fixture generation %v %s", e, out)
	}
	runner := &SerializedRunner{Runner: media.Process{FFmpeg: ffmpeg, FFprobe: ffprobe}}
	svc := &media.Service{Root: t.TempDir(), Limit: 1024 * 1024, Runner: runner}
	catalog := &Catalog{Roots: []string{root}, Media: svc}
	items, e := catalog.List(ctx)
	if e != nil || len(items) != 1 || items[0].Width != 32 || items[0].Height != 24 {
		t.Fatal(items, e)
	}
	v, e := catalog.Prepare(ctx, items[0].SourceID)
	if e != nil || v.MediaSHA256 != items[0].SHA256 {
		t.Fatal(v, e)
	}
	output := filepath.Join(t.TempDir(), "frame.jpg")
	if e = runner.Extract(ctx, filepath.Join(svc.Root, v.StorageKey), 200, output); e != nil {
		t.Fatal(e)
	}
	w, h, e := media.JPEGDimensions(output)
	if e != nil || w != 32 || h != 24 {
		t.Fatal(w, h, e)
	}
	hash, n, e := Fingerprint(ctx, output)
	if e != nil || len(hash) != 64 || n <= 0 {
		t.Fatal(hash, n, e)
	}
	t.Logf("real managed video size=%d dimensions=%dx%d JPEG size=%d", v.MediaSizeBytes, w, h, n)
}
