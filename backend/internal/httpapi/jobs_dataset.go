package httpapi

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (a *API) jobs(c *gin.Context) {
	var j []domain.Job
	var e error
	if u, ok := auth.ActorFrom(c.Request.Context()); ok {
		j, e = a.Store.JobsInWorkspace(u.WorkspaceID)
	} else {
		j, e = a.Store.Jobs()
	}
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
	if a.Dataset == nil || !a.Dataset.Configured() {
		failure(c, dataset.ErrConfig)
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
	if a.Dataset == nil || !a.Dataset.Configured() {
		failure(c, dataset.ErrConfig)
		return
	}
	j, e := a.Jobs.Submit("import", "", "", func(ctx context.Context, j domain.Job) error {
		body, e := a.Dataset.Download(ctx, req.CatalogFileID)
		if e != nil {
			return e
		}
		defer body.Close()
		v, e := a.Media.Prepare(ctx, body, req.Name)
		if e != nil {
			return e
		}
		if e = sources.Stamp(ctx, a.Media, &v); e != nil {
			return errors.Join(e, a.Media.Remove(v))
		}
		v.SourceKind = "dataset"
		v.SourceID = req.CatalogFileID
		reused, e := (sources.Repository{DB: a.Store.DB}).PublishImport(ctx, j, v)
		if e != nil {
			return errors.Join(e, a.Media.Remove(v))
		}
		if reused {
			return a.Media.Remove(v)
		}
		return nil
	})
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(202, j)
}
