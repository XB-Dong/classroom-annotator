package httpapi

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
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
	p, e := a.Dataset.Lessons(c.Request.Context(), page, size)
	if e != nil {
		failure(c, e)
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
	if !a.Dataset.Configured() {
		failure(c, dataset.ErrConfig)
		return
	}
	j, e := a.Jobs.Submit("import", "", "", func(ctx context.Context, j domain.Job) error {
		r, e := a.Dataset.Download(ctx, req.CatalogFileID)
		if e != nil {
			return e
		}
		defer r.Close()
		v, e := a.Media.Prepare(ctx, r, req.Name)
		if e != nil {
			return e
		}
		if e = a.Store.PublishImport(j, v); e != nil {
			if cleanupErr := a.Media.Remove(v); cleanupErr != nil {
				return cleanupErr
			}
			return e
		}
		return nil
	})
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(202, j)
}
