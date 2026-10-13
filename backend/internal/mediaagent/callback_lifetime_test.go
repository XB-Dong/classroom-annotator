package mediaagent

import (
	"bytes"
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type lifetimeRunner struct{ syntheticRunner }

func (lifetimeRunner) Probe(ctx context.Context, _ string) (media.Metadata, error) {
	<-ctx.Done()
	return media.Metadata{}, ctx.Err()
}

type lifetimeWriter struct {
	http.ResponseWriter
	entered  chan struct{}
	release  chan struct{}
	after    chan struct{}
	returned chan struct{}
}

func (w lifetimeWriter) SetReadDeadline(deadline time.Time) error {
	close(w.entered)
	<-w.release
	select {
	case <-w.returned:
		close(w.after)
	default:
	}
	return http.NewResponseController(w.ResponseWriter).SetReadDeadline(deadline)
}
func TestReviewDeadlineCallbackLifetime(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: lifetimeRunner{}, MaxOperation: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	entered, release, after, returned := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.ServeHTTP(lifetimeWriter{w, entered, release, after, returned}, r)
		close(returned)
	}))
	defer s.Close()
	defer a.Close()
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	p, _ := m.CreateFormFile("file", "complete.mp4")
	p.Write([]byte("synthetic-payload"))
	m.Close()
	req, _ := http.NewRequest("POST", s.URL+"/bridge/v1/uploads", &body)
	req.Header.Set("Content-Type", m.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-agent-token")
	req.Header.Set("X-Operation-ID", "callback-lifetime")
	req.Header.Set("X-Video-ID", "video-a")
	done := make(chan error, 1)
	go func() {
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("callback did not start")
	}
	callbackOutlivedHandler := false
	select {
	case <-returned:
		callbackOutlivedHandler = true
	case <-time.After(500 * time.Millisecond):
	}
	close(release)
	<-done
	if callbackOutlivedHandler {
		t.Fatal("handler returned while cancellation callback still owned ResponseController; late SetReadDeadline executed after ServeHTTP return")
	}
}
