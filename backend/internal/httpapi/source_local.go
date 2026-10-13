package httpapi

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"github.com/gin-gonic/gin"
	"regexp"
	"time"
)

var localID = regexp.MustCompile(`^[0-9a-f]{64}$`)
var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func localSourceFailure(c *gin.Context, e error) {
	switch {
	case errors.Is(e, sources.ErrUnavailable):
		Error(c, 503, "LOCAL_SOURCES_NOT_CONFIGURED", "Local directory sources are unavailable")
	case errors.Is(e, sources.ErrNotFound):
		Error(c, 404, "NOT_FOUND", "Local source not found or changed")
	case errors.Is(e, sources.ErrChanged):
		Error(c, 409, "SOURCE_CHANGED", "Local source changed before import")
	case errors.Is(e, sources.ErrIdempotency):
		Error(c, 409, "IDEMPOTENCY_CONFLICT", "Idempotency key was used for another source")
	default:
		failure(c, e)
	}
}
func (s *SourceService) listLocal(c *gin.Context) {
	if s.Local == nil {
		localSourceFailure(c, sources.ErrUnavailable)
		return
	}
	timeout := s.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	items, e := s.Local.List(ctx)
	if e != nil {
		localSourceFailure(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (s *SourceService) importLocal(c *gin.Context) {
	id := c.Param("sourceId")
	key := c.GetHeader("Idempotency-Key")
	if !localID.MatchString(id) || !idempotencyKey.MatchString(key) {
		Error(c, 400, "INVALID_REQUEST", "Valid source ID and Idempotency-Key are required")
		return
	}
	if s.Local == nil || len(s.Local.Roots) == 0 {
		localSourceFailure(c, sources.ErrUnavailable)
		return
	}
	actor, ok := auth.ActorFrom(c.Request.Context())
	if !ok {
		Error(c, 401, "AUTH_REQUIRED", "Authentication required")
		return
	}
	repo := sources.Repository{DB: s.Store.DB}
	claimContext, cancelClaim := s.Store.Context(c.Request.Context())
	defer cancelClaim()
	j, e := s.Jobs.SubmitWithPersistence("import", "", "", func(candidate domain.Job, allow bool) (domain.Job, bool, error) {
		return repo.Claim(claimContext, actor.WorkspaceID, key, id, candidate, allow)
	}, func(ctx context.Context, j domain.Job) error {
		v, e := s.Local.Prepare(ctx, id)
		if e != nil {
			return e
		}
		reused, e := repo.PublishImport(ctx, j, v)
		if e != nil {
			return errors.Join(e, s.Local.Media.Remove(v))
		}
		if reused {
			return s.Local.Media.Remove(v)
		}
		return nil
	})
	if e != nil {
		localSourceFailure(c, e)
		return
	}
	c.JSON(202, j)
}
