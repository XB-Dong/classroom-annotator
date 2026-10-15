package auth

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQualityCheckScopeAuthorizesEverySelectedResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), actorKey{}, User{ID: "alice", WorkspaceID: DefaultWorkspaceID}))
		c.Next()
	}, Scope(scopedResources{}))
	r.POST("/api/v1/exports/quality-check", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.Data(200, "application/json", body)
	})
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"videoId":"other"}`, 403},
		{`{"videoId":"mine","batchId":"other"}`, 403},
		{`{"frameIds":["mine","other"]}`, 403},
		{`{"frameIds":["mine"]}`, 200},
		{`{"videoId":"mine","batchId":"mine"}`, 200},
		{`{`, 400},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/exports/quality-check", strings.NewReader(tc.body)))
		if w.Code != tc.code || (tc.code == 200 && w.Body.String() != tc.body) {
			t.Fatalf("%s: status %d, body %s", tc.body, w.Code, w.Body.String())
		}
	}
}
