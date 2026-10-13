package help

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderSendsEntireManualAndOnlyQuestion(t *testing.T) {
	d, e := EmbeddedDocument()
	if e != nil {
		t.Fatal(e)
	}
	var seen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testing-secret" {
			t.Error("missing provider authorization")
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		if body.Model != "glm-4.7-flash" || len(body.Messages) != 3 || body.Messages[1].Content != d.Content || body.Messages[2].Content != "如何处理高风险框？" {
			t.Error("entire exact manual and only question must be sent")
		}
		seen = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"answer":"逐个检查风险框。","references":["risk"]}`}}}})
	}))
	defer server.Close()
	got, e := (HTTPProvider{server.URL, "testing-secret", "glm-4.7-flash"}).Answer(context.Background(), d, "如何处理高风险框？")
	if e != nil || !seen || got.Answer != "逐个检查风险框。" {
		t.Fatalf("response=%+v error=%v seen=%v", got, e, seen)
	}
}
func TestProviderRejectsMalformedResponseAndNeverForwardsOnRedirect(t *testing.T) {
	d, _ := EmbeddedDocument()
	t.Run("malformed", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
		}))
		defer s.Close()
		_, e := (HTTPProvider{s.URL, "secret", "glm-4.7-flash"}).Answer(context.Background(), d, "问题")
		if e != ErrInvalidResponse {
			t.Fatalf("error=%v", e)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		var forwarded bool
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
		defer target.Close()
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
		defer s.Close()
		_, e := (HTTPProvider{s.URL, "secret", "glm-4.7-flash"}).Answer(context.Background(), d, "问题")
		if e == nil || forwarded {
			t.Fatalf("forwarded=%v error=%v", forwarded, e)
		}
	})
}
func TestProviderHonorsRequestDeadlineAndDiscardsSensitiveErrors(t *testing.T) {
	d, _ := EmbeddedDocument()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.Write([]byte("secret provider data"))
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, e := (HTTPProvider{s.URL, "secret", "glm-4.7-flash"}).Answer(ctx, d, "问题")
	if e != ErrUnavailable || strings.Contains(e.Error(), "secret") {
		t.Fatalf("error=%v", e)
	}
}
func TestManualHasStableUniqueSectionsAndExactHash(t *testing.T) {
	d, e := EmbeddedDocument()
	if e != nil {
		t.Fatal(e)
	}
	if len(d.SHA256) != 64 || len(d.Sections) != 11 || d.Content != manual {
		t.Fatalf("document=%+v", d)
	}
	if _, e = ParseDocument("文档版本：test。\n## a: A\n## a：B"); e == nil {
		t.Fatal("duplicate section accepted")
	}
}
