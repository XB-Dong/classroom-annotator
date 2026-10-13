package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/gin-gonic/gin"
)

func remoteMediaLength(resp *http.Response, total int64) bool {
	length, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil || length < 0 || length > total {
		return false
	}
	if resp.StatusCode == http.StatusOK {
		return length == total
	}
	if resp.StatusCode != http.StatusPartialContent {
		return false
	}
	contentRange := resp.Header.Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes ") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(contentRange, "bytes "), "/")
	if len(parts) != 2 {
		return false
	}
	parsedTotal, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || parsedTotal != total {
		return false
	}
	limits := strings.Split(parts[0], "-")
	if len(limits) != 2 {
		return false
	}
	start, err := strconv.ParseInt(limits[0], 10, 64)
	if err != nil {
		return false
	}
	end, err := strconv.ParseInt(limits[1], 10, 64)
	if err != nil {
		return false
	}
	return start >= 0 && end >= start && end < total && end-start+1 == length
}

func (s *SourceService) streamRemoteMedia(c *gin.Context, key, expectedHash, contentType string, total int64, filename string) {
	if s.Bridge == nil {
		Error(c, 503, "MEDIA_SOURCE_UNAVAILABLE", "Media source is unavailable")
		return
	}
	rangeHeader := c.GetHeader("Range")
	resp, err := s.Bridge.OpenMedia(c.Request.Context(), c.Request.Method, key, rangeHeader)
	if err != nil {
		bridgeFailure(c, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		if rangeHeader == "" || resp.Header.Get("Content-Range") != "bytes */"+strconv.FormatInt(total, 10) {
			Error(c, 409, "RESOURCE_CHANGED", "Media identity changed")
			return
		}
		c.Header("Content-Range", resp.Header.Get("Content-Range"))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if (rangeHeader == "" && resp.StatusCode != http.StatusOK) || (rangeHeader != "" && resp.StatusCode != http.StatusPartialContent) || resp.Header.Get("Content-Type") != contentType || resp.Header.Get("ETag") != `"`+expectedHash+`"` || !remoteMediaLength(resp, total) {
		Error(c, 409, "RESOURCE_CHANGED", "Media identity changed")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", resp.Header.Get("Content-Length"))
	c.Header("Accept-Ranges", "bytes")
	c.Header("ETag", resp.Header.Get("ETag"))
	c.Header("X-Content-Type-Options", "nosniff")
	if resp.StatusCode == http.StatusPartialContent {
		c.Header("Content-Range", resp.Header.Get("Content-Range"))
	}
	if filename != "" {
		c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	}
	c.Status(resp.StatusCode)
	if c.Request.Method == http.MethodHead {
		return
	}
	buffer := make([]byte, 32<<10)
	_, _ = io.CopyBuffer(c.Writer, resp.Body, buffer)
}

func (s *SourceService) StreamFrame(c *gin.Context, frame domain.Frame) {
	if frame.StorageProvider != "mac" || frame.MediaSizeBytes <= 0 || frame.MediaSHA256 == "" {
		Error(c, 409, "RESOURCE_CHANGED", "Frame identity unavailable")
		return
	}
	s.streamRemoteMedia(c, frame.StorageKey, frame.MediaSHA256, "image/jpeg", frame.MediaSizeBytes, "")
}
