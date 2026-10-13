package preannotation

import (
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
)

func Register(g *gin.RouterGroup, s *Service) {
	g.GET("/frames/:id/review", func(c *gin.Context) {
		if !s.authorized(c, "frame", c.Param("id")) {
			return
		}
		out, e := s.Get(c.Request.Context(), c.Param("id"))
		if e != nil {
			s.respond(c, e)
			return
		}
		c.JSON(200, out)
	})
	g.PUT("/frames/:id/review", func(c *gin.Context) {
		if !s.authorized(c, "frame", c.Param("id")) {
			return
		}
		var wire struct {
			ExpectedRevision *int64             `json:"expectedRevision"`
			Decisions        *[]Decision        `json:"decisions"`
			Issues           *[]IssueResolution `json:"issueResolutions"`
		}
		if !httpapi.Decode(c, &wire) {
			return
		}
		if wire.ExpectedRevision == nil || wire.Decisions == nil || wire.Issues == nil {
			s.respond(c, ErrInvalidReview)
			return
		}
		out, e := s.Review(c.Request.Context(), c.Param("id"), ReviewWrite{*wire.ExpectedRevision, *wire.Decisions, *wire.Issues})
		if e != nil {
			s.respond(c, e)
			return
		}
		c.JSON(200, out)
	})
	g.POST("/frames/:id/preannotations", func(c *gin.Context) {
		if !s.authorized(c, "frame", c.Param("id")) {
			return
		}
		var body struct {
			ExpectedRevision *int64 `json:"expectedRevision"`
		}
		if !httpapi.Decode(c, &body) {
			return
		}
		if body.ExpectedRevision == nil {
			s.respond(c, ErrInvalidReview)
			return
		}
		job, e := s.Enqueue(c.Request.Context(), "frame:"+c.Param("id"), c.GetHeader("Idempotency-Key"), []string{c.Param("id")}, body.ExpectedRevision)
		if e != nil {
			s.respond(c, e)
			return
		}
		c.JSON(202, job)
	})
	g.POST("/batches/:id/preannotations", func(c *gin.Context) {
		if !s.authorized(c, "batch", c.Param("id")) {
			return
		}
		var body struct {
			FrameIDs []string `json:"frameIds"`
		}
		if !httpapi.Decode(c, &body) {
			return
		}
		job, e := s.Enqueue(c.Request.Context(), "batch:"+c.Param("id"), c.GetHeader("Idempotency-Key"), body.FrameIDs, nil)
		if e != nil {
			s.respond(c, e)
			return
		}
		c.JSON(202, job)
	})
	g.GET("/videos/:id/review-stats", func(c *gin.Context) {
		if !s.authorized(c, "video", c.Param("id")) {
			return
		}
		if c.Query("batchId") != "" {
			if !s.authorized(c, "batch", c.Query("batchId")) {
				return
			}
		}
		out, e := s.Stats(c.Request.Context(), c.Param("id"), c.Query("batchId"))
		if e != nil {
			s.respond(c, e)
			return
		}
		c.JSON(200, out)
	})
}
func (s *Service) authorized(c *gin.Context, kind, id string) bool {
	actor, ok := auth.ActorFrom(c.Request.Context())
	if !ok {
		httpapi.Error(c, 401, "AUTH_REQUIRED", "Authentication required")
		return false
	}
	if e := auth.NewPGRepository(s.Store.DB).AuthorizeResource(c.Request.Context(), actor, kind, id); e != nil {
		if errors.Is(e, auth.ErrForbidden) {
			httpapi.Error(c, 403, "WORKSPACE_FORBIDDEN", "Resource is not in your workspace")
		} else {
			httpapi.Error(c, 503, "AUTH_UNAVAILABLE", "Authorization service unavailable")
		}
		return false
	}
	return true
}
func (s *Service) respond(c *gin.Context, e error) {
	switch {
	case errors.Is(e, annotations.ErrConflict):
		latest, err := (annotations.Repository{Store: s.Store}).Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			httpapi.Error(c, 409, "REVISION_CONFLICT", "Frame was changed; reload before saving")
			return
		}
		c.JSON(409, gin.H{"code": "REVISION_CONFLICT", "message": "Frame was changed; reload before saving", "currentRevision": latest.Revision, "latestUpdatedBy": latest.UpdatedBy})
	case errors.Is(e, sql.ErrNoRows):
		httpapi.Error(c, 404, "NOT_FOUND", "Resource not found")
	case errors.Is(e, ErrIdempotency):
		httpapi.Error(c, 409, "IDEMPOTENCY_CONFLICT", "Request key has a different payload")
	case errors.Is(e, ErrActive):
		httpapi.Error(c, 409, "AI_ACTIVE", "Finish the active task and pending candidates before regenerating")
	case errors.Is(e, ErrQueueFull):
		httpapi.Error(c, 429, "AI_QUEUE_FULL", "AI queue is full")
	case errors.Is(e, ErrImage):
		httpapi.Error(c, 422, "FRAME_UNAVAILABLE", "Frame image is unavailable or changed")
	case errors.Is(e, ErrInvalidReview):
		httpapi.Error(c, 422, "INVALID_REVIEW", "Review request is invalid")
	case errors.Is(e, auth.ErrUnauthenticated):
		httpapi.Error(c, 401, "AUTH_REQUIRED", "Authentication required")
	default:
		httpapi.Error(c, 500, "INTERNAL_ERROR", "Review request failed")
	}
}
