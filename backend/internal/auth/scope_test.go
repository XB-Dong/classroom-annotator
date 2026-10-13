package auth

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type scopedResources struct{}

func (scopedResources) AuthorizeResource(_ context.Context, u User, kind, id string) error {
	if id == "mine" && u.WorkspaceID == DefaultWorkspaceID {
		return nil
	}
	return ErrForbidden
}
func TestScopeProtectsFrameVideoJobAndExportBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), actorKey{}, User{ID: "alice", WorkspaceID: DefaultWorkspaceID}))
		c.Next()
	})
	r.Use(Scope(scopedResources{}))
	handled := func(c *gin.Context) { c.Status(204) }
	r.GET("/api/v1/frames/:id/annotations", handled)
	r.GET("/api/v1/videos/:id/frames", handled)
	r.POST("/api/v1/jobs/:id/cancel", handled)
	r.POST("/api/v1/exports", handled)
	r.HEAD("/media/frames/:filename", handled)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/api/v1/frames/other/annotations", "", 403}, {"GET", "/api/v1/videos/other/frames", "", 403}, {"POST", "/api/v1/jobs/other/cancel", "", 403}, {"POST", "/api/v1/exports", `{"frameIds":["mine","other"]}`, 403}, {"HEAD", "/media/frames/other.jpg", "", 403}, {"GET", "/api/v1/frames/mine/annotations", "", 204}, {"POST", "/api/v1/exports", `{"videoId":"mine"}`, 204}} {
		w := httptest.NewRecorder()
		q := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		r.ServeHTTP(w, q)
		if w.Code != test.status {
			t.Fatalf("%s: got%d want%d", test.path, w.Code, test.status)
		}
	}
}
