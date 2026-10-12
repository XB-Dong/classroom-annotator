package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": message, "requestId": c.GetString("requestId")})
}
func failure(c *gin.Context, e error) {
	switch {
	case errors.Is(e, sql.ErrNoRows):
		Error(c, 404, "NOT_FOUND", "Resource not found")
	case errors.Is(e, jobs.ErrQueue):
		Error(c, 429, "JOB_QUEUE_FULL", "Job queue is full")
	case errors.Is(e, media.ErrSize):
		Error(c, 413, "UPLOAD_TOO_LARGE", "Upload exceeds configured limit")
	case errors.Is(e, media.ErrVideo):
		Error(c, 422, "INVALID_VIDEO", "Video could not be decoded")
	case errors.Is(e, dataset.ErrConfig):
		Error(c, 503, "DATASET_NOT_CONFIGURED", "Company dataset is not configured")
	case errors.Is(e, dataset.ErrResponse):
		Error(c, 502, "DATASET_INVALID_RESPONSE", "Company API response is invalid")
	case errors.Is(e, dataset.ErrUpstream):
		Error(c, 502, "DATASET_UPSTREAM_FAILED", "Company API request failed")
	default:
		var limit *http.MaxBytesError
		if errors.As(e, &limit) {
			Error(c, 413, "UPLOAD_TOO_LARGE", "Upload exceeds configured limit")
		} else {
			Error(c, 500, "INTERNAL_ERROR", "Request failed")
		}
	}
}
func Decode(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		Error(c, 400, "INVALID_REQUEST", "Invalid JSON request")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		Error(c, 400, "INVALID_REQUEST", "Invalid JSON request")
		return false
	}
	return true
}
