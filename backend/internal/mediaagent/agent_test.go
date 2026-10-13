package mediaagent

import (
	"bytes"
	"encoding/json"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestControlRequiresBearerAndHonestCapabilities(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Lease: 120 * time.Second, QueueSize: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {"wrong", 401}, {"test-agent-token", 200}} {
		r := httptest.NewRequest("GET", "/bridge/v1/capabilities", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
		}
		if w.Code == 200 {
			var caps mediaapi.Capabilities
			if e := json.Unmarshal(w.Body.Bytes(), &caps); e != nil {
				t.Fatal(e)
			}
			if caps.FFprobeAvailable || caps.FFmpegAvailable || caps.ContractVersion != "1.0.0" || caps.QueueSize != 8 || caps.LeaseSeconds != 120 || caps.MaxOperationSeconds != 600 {
				t.Fatalf("capabilities %+v", caps)
			}
		}
	}
	_ = http.StatusOK
}

func TestLegacySourceIDsPersistAndRejectPaths(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer old-secret" {
			t.Error("missing legacy bearer")
		}
		w.Write([]byte(`{"items":[{"id":"fixture.mp4","url":"/v1/media/fixture.mp4","size_bytes":7,"content_type":"video/mp4","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},{"id":"../private.mp4","url":"/v1/media/../private.mp4","size_bytes":7,"content_type":"video/mp4","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`))
	}))
	defer upstream.Close()
	cfg := Config{Root: t.TempDir(), Token: "test-agent-token", SourceURL: upstream.URL, SourceToken: "old-secret"}
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	list := func(a *Agent) map[string]any {
		r := httptest.NewRequest("GET", "/bridge/v1/sources", nil)
		r.Header.Set("Authorization", "Bearer test-agent-token")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("sources %d %s", w.Code, w.Body.String())
		}
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	v := list(a)
	items := v["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items %v", items)
	}
	id := items[0].(map[string]any)["sourceId"]
	if id == "fixture.mp4" || id == "" {
		t.Fatal("opaque source identity missing")
	}
	a.Close()
	a, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if list(a)["items"].([]any)[0].(map[string]any)["sourceId"] != id {
		t.Fatal("source identity changed on restart")
	}
}

func TestAckConflictAndCompletedCancelRetainResource(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token"})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.mu.Lock()
	a.state.Operations["op-a"] = &operationRecord{Operation: mediaapi.Operation{ContractVersion: mediaapi.ContractVersion, OperationID: "op-a", OperationType: "archive", Status: "completed", PublicationStatus: "unpublished"}}
	a.mu.Unlock()
	request := func(path, body string) int {
		r := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer test-agent-token")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	if n := request("/bridge/v1/operations/op-a/cancel", "{}"); n != 200 {
		t.Fatal(n)
	}
	if n := request("/bridge/v1/operations/op-a/ack", `{"publicationRevision":1}`); n != 200 {
		t.Fatal(n)
	}
	if n := request("/bridge/v1/operations/op-a/ack", `{"publicationRevision":1}`); n != 200 {
		t.Fatal(n)
	}
	if n := request("/bridge/v1/operations/op-a/ack", `{"publicationRevision":2}`); n != 409 {
		t.Fatal(n)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Operations["op-a"].Status != "completed" {
		t.Fatal("prepared output cancelled")
	}
}
