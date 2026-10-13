package httpapi

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (a *API) jobs(c *gin.Context) {
	j, e := a.Store.Jobs()
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, gin.H{"jobs": j})
}
func (a *API) job(c *gin.Context) {
	j, e := a.Store.Job(c.Param("id"))
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, j)
}
func (a *API) cancel(c *gin.Context) {
	j, e := a.Jobs.Cancel(c.Param("id"))
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, j)
}
func (a *API) lessons(c *gin.Context) {
	page, size := 1, 50
	var e error
	if s := c.Query("page"); s != "" {
		page, e = strconv.Atoi(s)
	}
	if e != nil || page < 1 {
		Error(c, 400, "INVALID_REQUEST", "Invalid page")
		return
	}
	if s := c.Query("pageSize"); s != "" {
		size, e = strconv.Atoi(s)
	}
	if e != nil || size < 1 || size > 100 {
		Error(c, 400, "INVALID_REQUEST", "Invalid page size")
		return
	}
	if a.Sources == nil || a.Sources.Bridge == nil {
		bridgeFailure(c, mediabridge.ErrUnavailable)
		return
	}
	capabilities, e := a.Sources.bridgeCapabilities(c.Request.Context())
	if e != nil || !capabilities.DatasetConfigured {
		Error(c, 503, "DATASET_NOT_CONFIGURED", "Company API is unavailable")
		return
	}
	p, e := a.Sources.Bridge.Lessons(c.Request.Context(), page, size)
	if e != nil {
		bridgeFailure(c, e)
		return
	}
	c.JSON(200, p)
}
func (a *API) importVideo(c *gin.Context) {
	var req struct {
		CatalogFileID string `json:"catalogFileId"`
		Name          string `json:"name"`
	}
	if !Decode(c, &req) {
		return
	}
	if !dataset.ValidCatalogID(req.CatalogFileID) || len(req.Name) > 255 {
		Error(c, 400, "INVALID_REQUEST", "Invalid catalog file ID or name")
		return
	}
	if a.Sources == nil || a.Sources.Bridge == nil {
		bridgeFailure(c, mediabridge.ErrUnavailable)
		return
	}
	capabilities, e := a.Sources.bridgeCapabilities(c.Request.Context())
	if e != nil || !capabilities.DatasetConfigured {
		Error(c, 503, "DATASET_NOT_CONFIGURED", "Company API is unavailable")
		return
	}
	videoID := domain.ID()
	j, e := a.Jobs.Submit("import", "", "", func(ctx context.Context, j domain.Job) error {
		return a.Sources.RunDatasetImport(ctx, j, videoID, req.CatalogFileID, req.Name)
	})
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(202, j)
}
