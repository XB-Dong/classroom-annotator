package auth

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestBatchPathScopeRejectsForeignResource(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), actorKey{}, User{WorkspaceID: DefaultWorkspaceID}))
		c.Next()
	}, Scope(scopedResources{}))
	r.GET("/api/v1/batches/:batchId/review", func(c *gin.Context) { c.Status(204) })
	for _, tc := range []struct {
		id     string
		status int
	}{{"other", 403}, {"mine", 204}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/batches/"+tc.id+"/review", nil))
		if w.Code != tc.status {
			t.Fatalf("%s %d want %d", tc.id, w.Code, tc.status)
		}
	}
}
