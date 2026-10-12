package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type badProbe struct{}

func (badProbe) Probe(context.Context, string) (Metadata, error) {
	return Metadata{}, errors.New("private failure details")
}
func (badProbe) Extract(context.Context, string, int64, string) error { return nil }
func TestInvalidAndOversizeUploadLeavesNoMedia(t *testing.T) {
	root := t.TempDir()
	s := Service{Root: root, Limit: 3, Runner: badProbe{}}
	for _, body := range []string{"ab", "abcd"} {
		if _, e := s.Prepare(context.Background(), strings.NewReader(body), "../../escape.mp4"); e == nil {
			t.Fatal("invalid input accepted")
		}
	}
	for _, dir := range []string{"staging", "videos"} {
		files, e := os.ReadDir(filepath.Join(root, dir))
		if e != nil && !os.IsNotExist(e) {
			t.Fatal(e)
		}
		if len(files) > 0 {
			t.Fatal("leaked files", dir)
		}
	}
}
