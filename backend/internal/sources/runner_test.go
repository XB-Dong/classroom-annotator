package sources

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"testing"
	"time"
)

type blockingRunner struct {
	entered chan string
	release chan struct{}
}

func (r blockingRunner) Probe(ctx context.Context, path string) (media.Metadata, error) {
	r.entered <- "probe"
	select {
	case <-r.release:
		return media.Metadata{}, nil
	case <-ctx.Done():
		return media.Metadata{}, ctx.Err()
	}
}
func (r blockingRunner) Extract(ctx context.Context, path string, ms int64, out string) error {
	r.entered <- "extract"
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func TestSerializedRunnerOneSlotAndCancellationWhileWaiting(t *testing.T) {
	entered := make(chan string, 2)
	release := make(chan struct{})
	defer close(release)
	runner := &SerializedRunner{Runner: blockingRunner{entered, release}}
	firstDone := make(chan struct{})
	go func() { defer close(firstDone); runner.Probe(context.Background(), "first") }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first process did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() { secondDone <- runner.Extract(ctx, "second", 0, "out") }()
	select {
	case <-entered:
		t.Fatal("second process started while probe owned slot")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case e := <-secondDone:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting process did not cancel")
	}
}
