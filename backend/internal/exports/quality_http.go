package exports

import (
	"context"

	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
)

func registerQualityCheck(g *gin.RouterGroup, service Service) {
	g.POST("/exports/quality-check", func(c *gin.Context) {
		var body exportRequest
		if !httpapi.Decode(c, &body) {
			return
		}
		selection, err := body.selection()
		if err == nil {
			err = selection.validate(service.frameLimit())
		}
		if err == nil {
			err = qualitySelectionProviders(c.Request.Context(), service.Store, selection)
		}
		var records []Record
		if err == nil {
			records, err = service.Snapshot(c.Request.Context(), selection)
		}
		if err == nil {
			_, err = providerOf(records)
		}
		var report QualityReport
		if err == nil {
			report, err = CheckQuality(c.Request.Context(), records)
		}
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, report)
	})
}

// Keep explicit-selection provider checks bounded by the caller's cancellation.
func qualitySelectionProviders(ctx context.Context, s *store.Store, selection Selection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(selection.FrameIDs) < 2 {
		return nil
	}
	ctx, cancel := s.Context(ctx)
	defer cancel()
	provider := ""
	for _, id := range selection.FrameIDs {
		var candidate string
		if err := s.DB.QueryRowContext(ctx, `SELECT storage_provider FROM frames WHERE id=$1`, id).Scan(&candidate); err != nil {
			return err
		}
		if candidate == "mac" {
			return ErrRetired
		}
		if candidate == "" {
			candidate = "local"
		}
		if provider != "" && candidate != provider {
			return ErrMixedProviders
		}
		provider = candidate
	}
	return ctx.Err()
}
