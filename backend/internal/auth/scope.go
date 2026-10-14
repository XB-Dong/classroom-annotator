package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"strings"
)

type ResourceAuthorizer interface {
	AuthorizeResource(context.Context, User, string, string) error
}

func Scope(r ResourceAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := ActorFrom(c.Request.Context())
		if !ok || r == nil {
			authError(c, 401, "AUTH_REQUIRED", "Authentication required")
			return
		}
		check := func(kind, id string) bool {
			if id == "" {
				return true
			}
			if e := r.AuthorizeResource(c.Request.Context(), u, kind, id); e != nil {
				if errors.Is(e, ErrUnavailable) {
					authError(c, 503, "AUTH_UNAVAILABLE", "Authorization temporarily unavailable")
					return false
				}
				authError(c, 403, "WORKSPACE_FORBIDDEN", "Resource is not in your workspace")
				return false
			}
			return true
		}
		path := c.FullPath()
		switch {
		case strings.HasPrefix(path, "/api/v1/videos/"):
			if !check("video", c.Param("id")) {
				return
			}
		case strings.HasPrefix(path, "/api/v1/frames/"):
			if !check("frame", c.Param("id")) {
				return
			}
		case strings.HasPrefix(path, "/api/v1/batches/"):
			batchID := c.Param("batchId")
			if batchID == "" {
				batchID = c.Param("id")
			}
			if !check("batch", batchID) {
				return
			}
		case strings.HasPrefix(path, "/api/v1/jobs/"):
			if !check("job", c.Param("id")) {
				return
			}
		case strings.HasPrefix(path, "/media/frames/"):
			if !check("frame", strings.TrimSuffix(c.Param("filename"), ".jpg")) {
				return
			}
		}
		if !check("batch", c.Query("batchId")) {
			return
		}
		if (path == "/api/v1/exports" || path == "/api/v1/exports/quality-check") && c.Request.Method == "POST" {
			raw, e := io.ReadAll(io.LimitReader(c.Request.Body, 1024*1024+1))
			if e != nil || len(raw) > 1024*1024 {
				authError(c, 400, "INVALID_REQUEST", "Invalid export request")
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			var body struct {
				VideoID  string   `json:"videoId"`
				BatchID  string   `json:"batchId"`
				FrameIDs []string `json:"frameIds"`
			}
			if json.Unmarshal(raw, &body) != nil {
				authError(c, 400, "INVALID_REQUEST", "Invalid export request")
				return
			}
			if !check("video", body.VideoID) || !check("batch", body.BatchID) {
				return
			}
			for _, id := range body.FrameIDs {
				if !check("frame", id) {
					return
				}
			}
		}
		c.Next()
	}
}
