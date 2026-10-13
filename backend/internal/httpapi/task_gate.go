package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// taskOwnerGate follows authentication and resource authorization on the API group.
// Only physical/model work inherits scheduler cancellation; manual APIs stay alive.
func taskOwnerGate(owned func() bool, ownerContexts ...context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !startsOwnedWork(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}
		var ownerCtx context.Context
		if len(ownerContexts) > 0 {
			ownerCtx = ownerContexts[0]
		}
		if (owned != nil && !owned()) || (ownerCtx != nil && ownerCtx.Err() != nil) {
			Error(c, http.StatusServiceUnavailable, "TASK_OWNER_UNAVAILABLE", "Task scheduler is unavailable; manual annotation remains available")
			return
		}
		if ownerCtx != nil {
			ctx, cancel := context.WithCancel(c.Request.Context())
			stop := context.AfterFunc(ownerCtx, cancel)
			defer stop()
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			if ownerCtx.Err() != nil {
				cancel()
			}
		}
		c.Next()
	}
}

func startsOwnedWork(method, path string) bool {
	if method == http.MethodGet && path == "/api/v1/sources/local" {
		return true
	}
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/api/v1/videos", "/api/v1/dataset/imports", "/api/v1/samples/imports":
		return true
	}
	return (strings.HasPrefix(path, "/api/v1/videos/") && strings.HasSuffix(path, "/extractions")) ||
		(strings.HasPrefix(path, "/api/v1/sources/local/") && strings.HasSuffix(path, "/imports")) ||
		((strings.HasPrefix(path, "/api/v1/frames/") || strings.HasPrefix(path, "/api/v1/batches/")) && strings.HasSuffix(path, "/preannotations"))
}
