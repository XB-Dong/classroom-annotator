package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/observability"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
)

type taskGateSessions struct{ auth.Repository }

func (taskGateSessions) SessionByDigest(context.Context, string) (auth.Session, error) {
	return auth.Session{User: auth.User{ID: "user", WorkspaceID: auth.DefaultWorkspaceID}, ExpiresAt: time.Now().Add(time.Hour), CSRFToken: "synthetic-csrf"}, nil
}

type taskGateForbiddenResources struct{}

func (taskGateForbiddenResources) AuthorizeResource(context.Context, auth.User, string, string) error {
	return errors.New("forbidden")
}

func TestTaskOwnerGateRunsAfterAuthenticationAndResourceScope(t *testing.T) {
	h, err := auth.NewHTTP(auth.NewService(taskGateSessions{}, "invite"), auth.HTTPOptions{TrustedOrigins: []string{"https://app.test"}, CookieSecure: true})
	if err != nil {
		t.Fatal(err)
	}
	m := &jobs.Manager{Metrics: observability.New()}
	r, err := NewWithSecurity(config.Config{}, &store.Store{}, m, nil, nil, nil, nil, Security{
		Auth: h, Resources: taskGateForbiddenResources{}, TaskOwner: func() bool { t.Fatal("ownership checked before auth or resource scope"); return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/api/v1/videos/video/extractions", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("anonymous request bypassed authentication", response.Code)
	}
	request = httptest.NewRequest("POST", "/api/v1/videos/video/extractions", nil)
	request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: base64.RawURLEncoding.EncodeToString(make([]byte, 32))})
	request.Header.Set("Origin", "https://app.test")
	request.Header.Set("X-CSRF-Token", "synthetic-csrf")
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("foreign resource bypassed scope", response.Code, response.Body.String())
	}
}

func TestTaskOwnerContextCancelsOnlyPhysicalRequests(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	for _, route := range []struct {
		method, path string
		physical     bool
	}{
		{"POST", "/api/v1/videos", true}, {"PUT", "/api/v1/frames/f/annotations", false},
	} {
		t.Run(route.path, func(t *testing.T) {
			ownerCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := gin.New()
			r.Use(taskOwnerGate(func() bool { return true }, ownerCtx))
			r.Handle(route.method, route.path, func(c *gin.Context) {
				cancel()
				if route.physical {
					select {
					case <-c.Request.Context().Done():
					case <-time.After(time.Second):
						t.Fatal("in-flight work survived ownership cancellation")
					}
				} else if c.Request.Context().Err() != nil {
					t.Fatal("manual request was cancelled")
				}
				c.Status(204)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
			if w.Code != 204 {
				t.Fatal(w.Code)
			}
		})
	}
}

func TestTaskOwnerGateBlocksPhysicalAndModelWork(t *testing.T) {
	for _, route := range []struct{ method, path, url string }{
		{"POST", "/api/v1/videos", "/api/v1/videos"},
		{"POST", "/api/v1/videos/:id/extractions", "/api/v1/videos/v/extractions"},
		{"POST", "/api/v1/dataset/imports", "/api/v1/dataset/imports"},
		{"POST", "/api/v1/samples/imports", "/api/v1/samples/imports"},
		{"POST", "/api/v1/sources/local/:sourceId/imports", "/api/v1/sources/local/s/imports"},
		{"GET", "/api/v1/sources/local", "/api/v1/sources/local"},
		{"POST", "/api/v1/frames/:id/preannotations", "/api/v1/frames/f/preannotations"},
		{"POST", "/api/v1/batches/:id/preannotations", "/api/v1/batches/b/preannotations"},
	} {
		t.Run(route.path, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			r := gin.New()
			r.Use(taskOwnerGate(func() bool { return false }))
			r.Handle(route.method, route.path, func(c *gin.Context) { t.Fatal("non-owner performed work") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(route.method, route.url, nil))
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "TASK_OWNER_UNAVAILABLE") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}

func TestTaskOwnerGateKeepsManualCapabilitiesAndRespondsToLoss(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	owned := true
	r := gin.New()
	r.Use(taskOwnerGate(func() bool { return owned }))
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/videos"}, {"PUT", "/api/v1/frames/f/annotations"},
		{"PUT", "/api/v1/frames/f/review"}, {"POST", "/api/v1/exports"},
		{"POST", "/api/v1/jobs/j/cancel"}, {"POST", "/api/v1/help/questions"},
		{"GET", "/api/v1/dataset/lessons"},
	} {
		r.Handle(route.method, route.path, func(c *gin.Context) { c.Status(204) })
		owned = false
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != 204 {
			t.Fatal(route, w.Code)
		}
	}
	r.POST("/api/v1/videos", func(c *gin.Context) { c.Status(204) })
	owned = true
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/videos", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	owned = false
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/videos", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}

func TestTaskOwnerGateNilRetainsLegacyHarness(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(taskOwnerGate(nil))
	r.POST("/api/v1/videos", func(c *gin.Context) { c.Status(204) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/videos", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
