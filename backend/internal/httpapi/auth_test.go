package httpapi

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/observability"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecuredRouterRejectsAnonymousBusinessAndMediaRoutes(t *testing.T) {
	repo := auth.NewPGRepository(nil)
	h, e := auth.NewHTTP(auth.NewService(repo, "invite"), auth.HTTPOptions{TrustedOrigins: []string{"https://app.test"}, CookieSecure: true})
	if e != nil {
		t.Fatal(e)
	}
	m := &jobs.Manager{Metrics: observability.New()}
	r, e := NewWithSecurity(config.Config{}, &store.Store{}, m, nil, nil, nil, &SourceService{}, Security{Auth: h, Resources: repo})
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct{ method, path string }{{"GET", "/api/v1/videos"}, {"GET", "/api/v1/jobs"}, {"POST", "/api/v1/exports"}, {"GET", "/media/frames/id.jpg"}, {"HEAD", "/media/frames/id.jpg"}, {"GET", "/api/v1/frames/id/annotations"}} {
		w := httptest.NewRecorder()
		q := httptest.NewRequest(test.method, test.path, nil)
		q.Header.Set("Range", "bytes=0-9")
		r.ServeHTTP(w, q)
		if w.Code != 401 {
			t.Fatalf("%s %s got%d", test.method, test.path, w.Code)
		}
	}
	for _, route := range r.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") && !strings.HasPrefix(route.Path, "/media/") {
			continue
		}
		if route.Path == "/api/v1/auth/register" || route.Path == "/api/v1/auth/login" {
			continue
		}
		w := httptest.NewRecorder()
		q := httptest.NewRequest(route.Method, route.Path, nil)
		r.ServeHTTP(w, q)
		if w.Code != 401 {
			t.Fatalf("registered %s %s bypassed auth: %d", route.Method, route.Path, w.Code)
		}
	}
}
func TestSecuredRouterRefusesMissingAuth(t *testing.T) {
	if _, e := NewWithSecurity(config.Config{}, nil, &jobs.Manager{Metrics: observability.New()}, nil, nil, nil, nil, Security{}); e == nil {
		t.Fatal("production constructor allowed missing auth")
	}
}
