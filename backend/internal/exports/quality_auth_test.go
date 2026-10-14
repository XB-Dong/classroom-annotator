package exports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestQualityCheckRealPGAuthorizationAndCSRF(t *testing.T) {
	s, _, _ := fixture(t)
	service := auth.NewService(auth.NewPGRepository(s.DB), "invite")
	if _, err := service.Register(context.Background(), "alice", "long-password-123", "invite"); err != nil {
		t.Fatal(err)
	}
	token, session, err := service.Login(context.Background(), "alice", "long-password-123")
	if err != nil {
		t.Fatal(err)
	}
	security, err := auth.NewHTTP(service, auth.HTTPOptions{TrustedOrigins: []string{"http://localhost"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO workspaces(id,name) VALUES('foreign-workspace','Foreign'); UPDATE videos SET workspace_id='foreign-workspace' WHERE id='other'`); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	Register(r.Group("/api/v1", security.Protect(), auth.Scope(auth.NewPGRepository(s.DB))), s, prometheus.NewRegistry())
	for _, tc := range []struct {
		body, csrf, origin string
		login              bool
		code               int
	}{
		{`{"frameIds":["f1"]}`, session.CSRFToken, "http://localhost", false, 401},
		{`{"frameIds":["f1"]}`, "wrong", "http://localhost", true, 403},
		{`{"frameIds":["f1"]}`, session.CSRFToken, "http://evil.test", true, 403},
		{`{"frameIds":["f1","otherframe"]}`, session.CSRFToken, "http://localhost", true, 403},
		{`{"videoId":"other"}`, session.CSRFToken, "http://localhost", true, 403},
		{`{"videoId":"v","batchId":"foreign"}`, session.CSRFToken, "http://localhost", true, 403},
		{`{"frameIds":["f1"]}`, session.CSRFToken, "http://localhost", true, 200},
	} {
		q := httptest.NewRequest("POST", "/api/v1/exports/quality-check", strings.NewReader(tc.body))
		q.Header.Set("Origin", tc.origin)
		q.Header.Set("X-CSRF-Token", tc.csrf)
		if tc.login {
			q.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		if w.Code != tc.code {
			t.Fatal(tc.body, w.Code, w.Body.String())
		}
	}
}
