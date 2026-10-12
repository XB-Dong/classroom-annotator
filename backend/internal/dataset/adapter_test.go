package dataset

import (
	"context"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMediaRedirectValidatesEveryDestination(t *testing.T) {
	reached := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer destination.Close()
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer media.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, `{"url":%q}`, media.URL) }))
	defer api.Close()
	a := New(config.Config{DatasetAPIURL: api.URL, DatasetID: "fixture", AllowedPrivateOrigins: []string{api.URL, media.URL}})
	if r, e := a.Download(context.Background(), "9"); e == nil {
		r.Close()
		t.Fatal("redirect to unapproved private destination accepted")
	}
	if reached {
		t.Fatal("unapproved destination reached")
	}
}
func TestPrivateOriginExceptionDoesNotAllowAnotherScheme(t *testing.T) {
	u, e := url.Parse("https://127.0.0.1:12345")
	if e != nil {
		t.Fatal(e)
	}
	p := Policy{PrivateOrigins: []string{"http://127.0.0.1:12345"}}
	if e = p.Validate(u, true); e == nil {
		t.Fatal("private origin exception crossed scheme")
	}
}
func TestPrivateOriginExceptionCannotCrossSchemeViaDefaultPort(t *testing.T) {
	u, _ := url.Parse("https://127.0.0.1:80")
	p := Policy{PrivateOrigins: []string{"http://127.0.0.1"}}
	if e := p.Validate(u, true); e == nil {
		t.Fatal("default-port host spelling crossed scheme")
	}
}

func TestObservedDTOAndMediaNeverReceivesAPIKey(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" {
			t.Error("key leaked")
		}
		fmt.Fprint(w, "video")
	}))
	defer media.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "secret" {
			t.Error("API key missing")
		}
		switch r.URL.Path {
		case "/api/v1/datasets/fixture/lessons":
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("page_size") != "50" {
				t.Error("pagination missing")
			}
			fmt.Fprint(w, `{"type":"lessons","items":[{"id":7,"videos":[{"id":8,"lesson_id":7,"catalog_file_id":9,"channel":4}]}],"total":1,"stats":{}}`)
		case "/api/v1/catalog-files/9/play-url":
			fmt.Fprintf(w, `{"url":%q}`, media.URL)
		default:
			t.Error("wrong path", r.URL.Path)
		}
	}))
	defer api.Close()
	a := New(config.Config{DatasetAPIURL: api.URL, DatasetID: "fixture", DatasetAPIKey: "secret", AllowedPrivateOrigins: []string{api.URL, media.URL}})
	p, e := a.Lessons(context.Background(), 2, 50)
	if e != nil || len(p.Items) != 1 || p.Items[0].Videos[0].CatalogFileID != 9 {
		t.Fatal(p, e)
	}
	r, e := a.Download(context.Background(), "9")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	if string(b) != "video" {
		t.Fatal(string(b))
	}
}
func TestRejectUnsafeNumericIDsAndCrossOriginAPIRedirect(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("cross-origin request sent") }))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"type":"lessons","items":[{"id":9007199254740992,"videos":[]}],"total":1,"stats":{}}`)
		} else {
			http.Redirect(w, r, other.URL, 302)
		}
	}))
	defer api.Close()
	a := New(config.Config{DatasetAPIURL: api.URL, DatasetID: "fixture", AllowedPrivateOrigins: []string{api.URL, other.URL}})
	for _, page := range []int{1, 2} {
		if _, e := a.Lessons(context.Background(), page, 50); e == nil {
			t.Fatal("unsafe response/redirect accepted")
		}
	}
	b := New(config.Config{DatasetAPIURL: api.URL})
	if _, e := b.Lessons(context.Background(), 1, 50); e == nil {
		t.Fatal("private origin accepted by default")
	}
}
