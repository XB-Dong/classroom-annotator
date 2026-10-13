package mediabridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestMetadataTimesOutAfterFlushedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24), Timeout: time.Second, ControlTimeout: 80 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range []func() error{
		func() error { _, err := client.Sources(context.Background(), ""); return err },
		func() error { _, err := client.Lessons(context.Background(), 1, 20); return err },
		func() error { _, err := client.Operation(context.Background(), "op-1"); return err },
	} {
		start := time.Now()
		if err := read(); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("stalled metadata body: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
			t.Fatalf("metadata body waited %v", elapsed)
		}
	}
}

func TestRegistrationKeepsLongPreparationBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(mediaapi.Registration{OperationID: "op-1", VideoID: "video-1", MediaKey: "key-1", SHA256: strings.Repeat("a", 64), SizeBytes: 10, Width: 2, Height: 3, DurationSeconds: 1})
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24), Timeout: time.Second, ControlTimeout: 80 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Registration(context.Background(), mediaapi.RegistrationRequest{OperationID: "op-1", VideoID: "video-1", SourceID: "source-1"})
	if err != nil || got.MediaKey != "key-1" {
		t.Fatalf("registration: %+v %v", got, err)
	}
}

func TestMediaBodyIdleDeadlineAndProgress(t *testing.T) {
	for _, progressing := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "3")
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("a"))
			w.(http.Flusher).Flush()
			if !progressing {
				<-r.Context().Done()
				return
			}
			for _, b := range []byte("bc") {
				time.Sleep(50 * time.Millisecond)
				_, _ = w.Write([]byte{b})
				w.(http.Flusher).Flush()
			}
		}))
		client, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24), DownloadTimeout: time.Second, MediaIdleTimeout: 80 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.OpenMedia(context.Background(), http.MethodGet, "media-1", "")
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		b, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		server.Close()
		if progressing {
			if readErr != nil || string(b) != "abc" {
				t.Fatalf("progressing stream: %q %v", b, readErr)
			}
		} else {
			if readErr == nil || errors.Is(readErr, io.EOF) {
				t.Fatalf("stalled stream looked complete: %q %v", b, readErr)
			}
			if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
				t.Fatalf("idle download waited %v", elapsed)
			}
		}
	}
}
