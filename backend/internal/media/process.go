package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"strconv"
)

type Process struct{ FFmpeg, FFprobe string }
type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := b.limit - b.Len(); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}
func (p Process) Probe(ctx context.Context, path string) (Metadata, error) {
	cmd := exec.CommandContext(ctx, p.FFprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height:format=duration", "-of", "json", path)
	out := &cappedBuffer{limit: 65536}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		return Metadata{}, ErrVideo
	}
	var dto struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if e := json.Unmarshal(out.Bytes(), &dto); e != nil || len(dto.Streams) != 1 {
		return Metadata{}, ErrVideo
	}
	d, e := strconv.ParseFloat(dto.Format.Duration, 64)
	if e != nil {
		return Metadata{}, ErrVideo
	}
	return Metadata{dto.Streams[0].Width, dto.Streams[0].Height, d}, nil
}
func (p Process) Extract(ctx context.Context, input string, target int64, output string) error {
	// Output seeking decodes before discarding frames prior to target. No input fast seek.
	cmd := exec.CommandContext(ctx, p.FFmpeg, "-nostdin", "-v", "error", "-i", input, "-ss", fmt.Sprintf("%d.%03d", target/1000, target%1000), "-map", "0:v:0", "-frames:v", "1", "-q:v", "2", "-pix_fmt", "yuvj420p", "-threads", "1", "-y", output)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("EXTRACTION_FAILED")
	}
	return nil
}
func JPEGDimensions(path string) (int, int, error) {
	f, e := os.Open(path)
	if e != nil {
		return 0, 0, errors.New("FRAME_UNAVAILABLE")
	}
	defer f.Close()
	img, e := jpeg.Decode(f)
	if e != nil {
		return 0, 0, errors.New("FRAME_UNAVAILABLE")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return 0, 0, errors.New("FRAME_UNAVAILABLE")
	}
	return b.Dx(), b.Dy(), nil
}
