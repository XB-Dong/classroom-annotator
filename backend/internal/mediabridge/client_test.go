package mediabridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestClientRejectsPublicAndCredentialOrigins(t *testing.T) {
	for _, raw := range []string{"https://example.org", "http://user:password@127.0.0.1:19090", "http://127.0.0.1:19090/bridge", "http://127.0.0.1:19090?q=1"} {
		if _, err := New(Config{BaseURL: raw, Token: strings.Repeat("x", 24)}); err == nil {
			t.Errorf("accepted unsafe origin %q", raw)
		}
	}
}

func TestCapabilitiesRequireExactContractAndLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("x", 24) {
			t.Error("missing private token")
		}
		_ = json.NewEncoder(w).Encode(mediaapi.Capabilities{ContractVersion: "0.9.0", MediaRootWritable: true, FFmpegAvailable: true, FFprobeAvailable: true, MaxUploadBytes: 1 << 30, MaxFrames: 1000, MaxControlBytes: 16 << 20})
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Capabilities(context.Background(), Limits{MaxUploadBytes: 1 << 30, MaxFrames: 1000, MaxControlBytes: 16 << 20}); err == nil {
		t.Fatal("accepted mismatched contract")
	}
}

func TestUploadStreamsToAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bridge/v1/uploads" || r.Header.Get("X-Operation-ID") != "op-1" || r.Header.Get("X-Video-ID") != "vid-1" {
			t.Errorf("wrong upload target: %s", r.URL)
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		if part.FormName() != "file" {
			t.Errorf("field %q", part.FormName())
		}
		b, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, bytes.Repeat([]byte("z"), 65536)) {
			t.Fatal("body changed")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mediaapi.Registration{OperationID: "op-1", VideoID: "vid-1", MediaKey: "media-1", SHA256: strings.Repeat("a", 64), SizeBytes: 65536, Width: 10, Height: 10, DurationSeconds: 1})
	}))
	defer server.Close()
	c, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	registration, err := c.Upload(context.Background(), "op-1", "vid-1", "test.webm", io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("z"), 65536))), 1<<20)
	if err != nil || registration.OperationID != "op-1" {
		t.Fatalf("upload: %+v, %v", registration, err)
	}
}

func TestRegistrationReturnsPreparedManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bridge/v1/registrations" {
			t.Errorf("wrong route %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(mediaapi.Registration{OperationID: "op-1", VideoID: "video-1", MediaKey: "key-1", SHA256: strings.Repeat("a", 64), SizeBytes: 10, Width: 2, Height: 3, DurationSeconds: 1})
	}))
	defer server.Close()
	c, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	registration, err := c.Registration(context.Background(), mediaapi.RegistrationRequest{OperationID: "op-1", VideoID: "video-1", SourceID: "source-1"})
	if err != nil || registration.MediaKey != "key-1" {
		t.Fatalf("registration: %+v, %v", registration, err)
	}
}

func TestMediaUsesOpaqueKeyAndSingleRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=2-4" {
			t.Errorf("range %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Range", "bytes 2-4/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("abc"))
	}))
	defer server.Close()
	c, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.OpenMedia(context.Background(), http.MethodGet, "../bad", ""); err == nil {
		t.Fatal("accepted path")
	}
	if _, err = c.OpenMedia(context.Background(), http.MethodGet, "opaque-1", "bytes=1-2,4-5"); err == nil {
		t.Fatal("accepted multirange")
	}
	r, err := c.OpenMedia(context.Background(), http.MethodGet, "opaque-1", "bytes=2-4")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusPartialContent {
		t.Fatal(r.StatusCode)
	}
}

func TestSourcesKeepCursorInQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bridge/v1/sources" || r.URL.Query().Get("cursor") != "opaque-1" {
			t.Errorf("wrong route %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(mediaapi.Sources{ContractVersion: mediaapi.ContractVersion, Items: []mediaapi.Source{}, NextCursor: nil})
	}))
	defer server.Close()
	c, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Sources(context.Background(), "opaque-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLessonsProjectOnlySafeFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page_size") != "20" || r.URL.Query().Get("pageSize") != "" {
			t.Errorf("wrong paging query %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"type":"lesson","items":[{"id":1,"course_name":"course","organization_name":"org","teacher":"teacher","date":"2026-10-13","section_index":1,"videos":[{"id":2,"lesson_id":1,"catalog_file_id":3,"channel":4,"part_index":0,"tags":{"secret":"value"}}],"source_page_url":"private"}],"total":1,"stats":{"secret":"value"}}`)
	}))
	defer server.Close()
	c, err := New(Config{BaseURL: server.URL, Token: strings.Repeat("x", 24)})
	if err != nil {
		t.Fatal(err)
	}
	page, err := c.Lessons(context.Background(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("secret")) || bytes.Contains(b, []byte("source_page_url")) {
		t.Fatalf("unsafe field leaked: %s", b)
	}
}
