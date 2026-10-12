package httpapi

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/gin-gonic/gin"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (a *API) videos(c *gin.Context) {
	v, e := a.Store.Videos()
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, gin.H{"videos": v})
}
func (a *API) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, a.Cfg.MaxUploadBytes+1024*1024)
	reader, e := c.Request.MultipartReader()
	if e != nil {
		Error(c, 400, "INVALID_UPLOAD", "Expected multipart file")
		return
	}
	var body io.Reader
	var name string
	for {
		part, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				failure(c, e)
			} else {
				Error(c, 400, "INVALID_UPLOAD", "Malformed multipart upload")
			}
			return
		}
		if part.FormName() == "file" && part.FileName() != "" {
			body = part
			name = part.FileName()
			break
		}
		part.Close()
	}
	if body == nil {
		Error(c, 400, "INVALID_UPLOAD", "Missing file field")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.Cfg.JobTimeout)
	defer cancel()
	v, e := a.Media.Prepare(ctx, body, name)
	if e != nil {
		failure(c, e)
		return
	}
	if e = a.Store.AddVideo(v); e != nil {
		a.Media.Remove(v)
		failure(c, e)
		return
	}
	c.JSON(201, v)
}
func (a *API) extract(c *gin.Context) {
	v, e := a.Store.Video(c.Param("id"))
	if e != nil {
		failure(c, e)
		return
	}
	var req struct {
		IntervalSeconds *float64 `json:"intervalSeconds"`
		StartSeconds    *float64 `json:"startSeconds"`
		EndSeconds      *float64 `json:"endSeconds"`
	}
	if !Decode(c, &req) {
		return
	}
	if req.IntervalSeconds == nil {
		Error(c, 400, "INVALID_REQUEST", "intervalSeconds is required")
		return
	}
	start := 0.0
	end := math.Ceil(v.DurationSeconds*1000) / 1000
	if req.StartSeconds != nil {
		start = *req.StartSeconds
	}
	if req.EndSeconds != nil {
		end = *req.EndSeconds
	}
	targets, e := extraction.Plan(*req.IntervalSeconds, start, end, v.DurationSeconds, a.Cfg.MaxFrames)
	if e != nil {
		Error(c, 422, "INVALID_EXTRACTION", "Invalid sampling interval or frame limit")
		return
	}
	j, e := a.Jobs.Submit("extraction", v.ID, domain.ID(), func(ctx context.Context, j domain.Job) error { return a.Extraction.Run(ctx, j, v, targets) })
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(202, j)
}
func (a *API) batches(c *gin.Context) {
	v, e := a.Store.Batches(c.Param("id"))
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, gin.H{"batches": v})
}
func (a *API) frames(c *gin.Context) {
	v, e := a.Store.Frames(c.Param("id"), c.Query("batchId"))
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, gin.H{"frames": v})
}
func (a *API) progress(c *gin.Context) {
	v, e := a.Store.Progress(c.Param("id"), c.Query("batchId"))
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, v)
}
func (a *API) frameMedia(c *gin.Context) {
	name := c.Param("filename")
	if !strings.HasSuffix(name, ".jpg") {
		Error(c, 404, "NOT_FOUND", "Resource not found")
		return
	}
	f, e := a.Store.Frame(strings.TrimSuffix(name, ".jpg"))
	if e != nil {
		failure(c, e)
		return
	}
	path := filepath.Join(a.Store.Root, f.StorageKey)
	file, e := os.Open(path)
	if e != nil {
		Error(c, 404, "MEDIA_UNAVAILABLE", "Image unavailable")
		return
	}
	defer file.Close()
	st, e := file.Stat()
	if e != nil {
		failure(c, e)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Content-Type", "image/jpeg")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, f.ID+".jpg", st.ModTime(), file)
}
