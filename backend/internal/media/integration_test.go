package media

import (
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFFmpegAccurateSeekTailAndRotation(t *testing.T) {
	ffmpeg, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("FFmpeg unavailable; execute this synthetic test on the cloud host")
	}
	ffprobe, e := exec.LookPath("ffprobe")
	if e != nil {
		t.Skip("ffprobe unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := t.TempDir()
	input := filepath.Join(root, "synthetic.mp4")
	run := func(args ...string) {
		cmd := exec.CommandContext(ctx, ffmpeg, append([]string{"-nostdin", "-v", "error"}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("synthetic fixture generation: %v %s", e, b)
		}
	}
	run("-f", "lavfi", "-i", "color=c=red:size=32x24:rate=10:duration=0.2", "-f", "lavfi", "-i", "color=c=blue:size=32x24:rate=10:duration=0.8", "-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0", "-c:v", "mpeg4", "-threads", "1", "-y", input)
	p := Process{FFmpeg: ffmpeg, FFprobe: ffprobe}
	m, e := p.Probe(ctx, input)
	if e != nil || m.Width != 32 || m.Height != 24 || m.Duration < .99 {
		t.Fatal(m, e)
	}
	out := filepath.Join(root, "target.jpg")
	if e = p.Extract(ctx, input, 150, out); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(out)
	if e != nil {
		t.Fatal(e)
	}
	img, e := jpeg.Decode(f)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	r, _, b, _ := img.At(10, 10).RGBA()
	if b <= r {
		t.Fatal("seek selected earlier red frame; first frame >=150ms must be blue at200ms")
	}
	tail := filepath.Join(root, "tail.jpg")
	if e = p.Extract(ctx, input, 950, tail); e != nil {
		t.Fatal(e)
	}
	if _, _, e = JPEGDimensions(tail); e == nil {
		t.Fatal("unavailable tail unexpectedly produced frame")
	}
	rotated := filepath.Join(root, "rotated.mp4")
	run("-i", input, "-c", "copy", "-metadata:s:v:0", "rotate=90", "-y", rotated)
	rotOut := filepath.Join(root, "rotated.jpg")
	if e = p.Extract(ctx, rotated, 0, rotOut); e != nil {
		t.Fatal(e)
	}
	w, h, e := JPEGDimensions(rotOut)
	if e != nil || w != 24 || h != 32 {
		t.Fatalf("rotation dimensions=%dx%d err=%v", w, h, e)
	}
}
