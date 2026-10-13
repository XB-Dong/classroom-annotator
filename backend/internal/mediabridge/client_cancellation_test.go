package mediabridge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type closeableBlockedSource struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *closeableBlockedSource) Read([]byte) (int, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return 0, io.EOF
}

func (s *closeableBlockedSource) Close() error {
	s.once.Do(func() { close(s.started) })
	select {
	case <-s.release:
	default:
		close(s.release)
	}
	return nil
}

func TestUploadCancellationClosesBlockedSourceAndWaitsForCopy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "synthetic-review-token", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := &closeableBlockedSource{started: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.Upload(ctx, "review-operation", "review-video", "sample.mp4", source, 1024)
		done <- err
	}()
	select {
	case <-source.started:
	case <-time.After(5 * time.Second):
		t.Fatal("upload source did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		_ = source.Close()
		<-done
		t.Fatal("cancelling upload did not interrupt source read")
	}
}

func TestUploadEarlyRemoteRejectionClosesBlockedSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":"UNAUTHORIZED"}`)
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "synthetic-review-token", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := &closeableBlockedSource{started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := client.Upload(context.Background(), "review-operation", "review-video", "sample.mp4", source, 1024)
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		_ = source.Close()
		<-done
		t.Fatal("early remote response did not interrupt source read")
	}
}
