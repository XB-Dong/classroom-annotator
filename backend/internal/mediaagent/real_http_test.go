package mediaagent

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func realPausedUpload(t *testing.T, a *Agent, id string) (*httptest.Server, net.Conn) {
	t.Helper()
	s := httptest.NewServer(a)
	c, err := net.Dial("tcp", strings.TrimPrefix(s.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(); s.Close(); a.Close() })
	body := "--review-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"paused.mp4\"\r\nContent-Type: video/mp4\r\n\r\npartial-video"
	_, err = fmt.Fprintf(c, "POST /bridge/v1/uploads HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer test-agent-token\r\nContent-Type: multipart/form-data; boundary=review-boundary\r\nContent-Length: 1000000\r\nX-Operation-ID: %s\r\nX-Video-ID: video-a\r\n\r\n%s", id, body)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		op := a.state.Operations[id]
		a.mu.Unlock()
		if op != nil {
			time.Sleep(100 * time.Millisecond)
			return s, c
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("receiving operation not created")
	return nil, nil
}

func TestRealHTTPPausedUploadCancel(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	s, c := realPausedUpload(t, a, "real-http-cancel")
	req, _ := http.NewRequest("POST", s.URL+"/bridge/v1/operations/real-http-cancel/cancel", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer test-agent-token")
	client := &http.Client{Timeout: time.Second}
	response, cancelErr := client.Do(req)
	if response != nil {
		response.Body.Close()
	}
	a.mu.Lock()
	status := a.state.Operations["real-http-cancel"].Status
	stage := a.state.Operations["real-http-cancel"].Staging
	a.mu.Unlock()
	c.Close()
	if cancelErr != nil || response == nil || response.StatusCode != 200 || status != "cancelled" || stage != "" {
		t.Fatalf("real HTTP cancel did not stop receiver: error=%v status=%s stagingRetained=%t", cancelErr, status, stage != "")
	}
}

func TestRealHTTPPausedUploadDeadline(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}, Lease: 200 * time.Millisecond, MaxOperation: 350 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, c := realPausedUpload(t, a, "real-http-deadline")
	time.Sleep(900 * time.Millisecond)
	a.mu.Lock()
	status := a.state.Operations["real-http-deadline"].Status
	stage := a.state.Operations["real-http-deadline"].Staging
	a.mu.Unlock()
	c.Close()
	if status != "failed" || stage != "" {
		t.Fatalf("real HTTP receiver survived lease and deadline: status=%s stagingRetained=%t", status, stage != "")
	}
}

func TestRealHTTPPausedUploadClose(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	_, c := realPausedUpload(t, a, "real-http-close")
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	blocked := false
	select {
	case <-done:
	case <-time.After(time.Second):
		blocked = true
	}
	c.Close()
	<-done
	if blocked {
		t.Fatal("Agent.Close blocked until client TCP connection closed")
	}
}

func TestRealHTTPPausedUploadLeaseExpiry(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}, Lease: 200 * time.Millisecond, MaxOperation: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, c := realPausedUpload(t, a, "real-http-lease")
	time.Sleep(1200 * time.Millisecond)
	a.mu.Lock()
	op := a.state.Operations["real-http-lease"]
	status, stage := op.Status, op.Staging
	code := op.ErrorCode
	a.mu.Unlock()
	c.Close()
	if status != "failed" || stage != "" || code == nil || *code != "TIMEOUT" {
		t.Fatalf("real HTTP lease expired without stopping receiver: status=%s stagingRetained=%t code=%v", status, stage != "", code)
	}
}

func TestRealHTTPPausedUploadRenewThenCancel(t *testing.T) {
	a, err := New(Config{Root: t.TempDir(), Token: "test-agent-token", Runner: syntheticRunner{}, Lease: 3 * time.Second, MaxOperation: 8 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	s, c := realPausedUpload(t, a, "real-http-renew")
	client := &http.Client{Timeout: time.Second}
	control := func(action string) *http.Response {
		t.Helper()
		req, err := http.NewRequest("POST", s.URL+"/bridge/v1/operations/real-http-renew/"+action, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer test-agent-token")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response
	}
	if response := control("renew"); response.StatusCode != 200 {
		t.Fatalf("first renew %d", response.StatusCode)
	}
	time.Sleep(1700 * time.Millisecond)
	if response := control("renew"); response.StatusCode != 200 {
		t.Fatalf("second renew %d", response.StatusCode)
	}
	time.Sleep(1700 * time.Millisecond)
	a.mu.Lock()
	status := a.state.Operations["real-http-renew"].Status
	a.mu.Unlock()
	if status != "running" {
		c.Close()
		t.Fatalf("renewed receiver stopped early: %s", status)
	}
	if response := control("cancel"); response.StatusCode != 200 {
		c.Close()
		t.Fatalf("cancel %d", response.StatusCode)
	}
	a.mu.Lock()
	status = a.state.Operations["real-http-renew"].Status
	stage := a.state.Operations["real-http-renew"].Staging
	a.mu.Unlock()
	c.Close()
	if status != "cancelled" || stage != "" {
		t.Fatalf("cancel after renew: %s stage=%q", status, stage)
	}
}
