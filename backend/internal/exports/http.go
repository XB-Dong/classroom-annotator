package exports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func Register(g *gin.RouterGroup, s *store.Store, registry prometheus.Registerer, limits ...int) {
	RegisterWithBridge(g, s, nil, registry, limits...)
}

func RegisterWithBridge(g *gin.RouterGroup, s *store.Store, bridge *mediabridge.Client, registry prometheus.Registerer, limits ...int) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_exports_total", Help: "Completed export requests by result."}, []string{"result"})
	registry.MustRegister(counter)
	maxFrames := DefaultMaxFrames
	if len(limits) > 0 && limits[0] > 0 && limits[0] < maxFrames {
		maxFrames = limits[0]
	}
	service := Service{Store: s, MaxFrames: maxFrames}
	g.POST("/exports", func(c *gin.Context) {
		var body exportRequest
		if !httpapi.Decode(c, &body) {
			counter.WithLabelValues("invalid").Inc()
			return
		}
		selection, err := body.selection()
		if err == nil {
			err = selectedMixedProviders(s, selection)
		}
		var records []Record
		if err == nil {
			records, err = service.Snapshot(c.Request.Context(), selection)
		}
		var provider string
		if err == nil {
			provider, err = providerOf(records)
		}
		if err == nil && provider == "mac" {
			archiveCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
			manifest, remoteErr := BuildRemote(archiveCtx, s, bridge, records)
			cancel()
			if remoteErr != nil {
				err = remoteErr
			} else {
				var response *http.Response
				response, err = bridge.OpenMedia(c.Request.Context(), http.MethodGet, manifest.MediaKey, "")
				if err == nil {
					defer response.Body.Close()
					if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" || response.Header.Get("ETag") != `"`+manifest.SHA256+`"` || response.Header.Get("Content-Length") != strconv.FormatInt(manifest.SizeBytes, 10) {
						err = mediabridge.ErrContract
					} else {
						counter.WithLabelValues("success").Inc()
						c.Header("Content-Type", "application/zip")
						c.Header("Content-Disposition", `attachment; filename="annotations.zip"`)
						c.Header("Content-Length", strconv.FormatInt(manifest.SizeBytes, 10))
						c.Header("Cache-Control", "no-store")
						c.Status(http.StatusOK)
						_, _ = io.CopyBuffer(c.Writer, response.Body, make([]byte, 32<<10))
						return
					}
				}
			}
		}
		var path string
		if err == nil {
			path, err = service.Build(c.Request.Context(), records)
		}
		if err != nil {
			respondError(c, err)
			counter.WithLabelValues(resultLabel(err)).Inc()
			return
		}
		defer os.Remove(path)
		// The archive was fully written and closed before any download headers.
		counter.WithLabelValues("success").Inc()
		c.Header("Content-Type", "application/zip")
		c.FileAttachment(path, "annotations.zip")
	})
}

func selectedMixedProviders(s *store.Store, selection Selection) error {
	if len(selection.FrameIDs) < 2 {
		return nil
	}
	provider := ""
	for _, id := range selection.FrameIDs {
		frame, err := s.Frame(id)
		if err != nil {
			return err
		}
		candidate := frame.StorageProvider
		if candidate == "" {
			candidate = "local"
		}
		if provider != "" && candidate != provider {
			return ErrMixedProviders
		}
		provider = candidate
	}
	return nil
}

type exportRequest struct {
	VideoID  json.RawMessage `json:"videoId"`
	BatchID  json.RawMessage `json:"batchId"`
	FrameIDs json.RawMessage `json:"frameIds"`
}

func (r exportRequest) selection() (Selection, error) {
	var out Selection
	if len(r.FrameIDs) > 0 {
		if len(r.VideoID) > 0 || len(r.BatchID) > 0 || string(r.FrameIDs) == "null" {
			return out, ErrInvalid
		}
		if json.Unmarshal(r.FrameIDs, &out.FrameIDs) != nil {
			return out, ErrInvalid
		}
	} else {
		if len(r.VideoID) == 0 || string(r.VideoID) == "null" || json.Unmarshal(r.VideoID, &out.VideoID) != nil {
			return out, ErrInvalid
		}
		if len(r.BatchID) > 0 && (string(r.BatchID) == "null" || json.Unmarshal(r.BatchID, &out.BatchID) != nil || out.BatchID == "") {
			return out, ErrInvalid
		}
	}
	return out, out.validate(DefaultMaxFrames)
}

func resultLabel(err error) string {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "not_found"
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrEmpty), errors.Is(err, ErrLimit):
		return "invalid"
	default:
		return "error"
	}
}
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpapi.Error(c, 404, "NOT_FOUND", "Selected resource not found")
	case errors.Is(err, ErrInvalid):
		httpapi.Error(c, 400, "INVALID_REQUEST", "Select a video scope or a unique nonempty frame list")
	case errors.Is(err, ErrEmpty):
		httpapi.Error(c, 422, "NO_EXPORTABLE_FRAMES", "Selected scope has no completed frames")
	case errors.Is(err, ErrLimit):
		httpapi.Error(c, 422, "EXPORT_LIMIT_EXCEEDED", "Export exceeds supported limits; select a smaller scope")
	case errors.Is(err, ErrMixedProviders):
		httpapi.Error(c, 422, "MIXED_MEDIA_PROVIDERS", "Selected frames use different media providers")
	case errors.Is(err, jobs.ErrRemoteCleanupPending):
		httpapi.Error(c, 503, "REMOTE_CLEANUP_PENDING", "Remote media cleanup requires reconciliation")
	case errors.Is(err, mediabridge.ErrUnavailable):
		httpapi.Error(c, 503, "MEDIA_SOURCE_UNAVAILABLE", "Mac media source is unavailable")
	case errors.Is(err, mediabridge.ErrContract):
		httpapi.Error(c, 409, "BRIDGE_CONTRACT_MISMATCH", "Mac media response did not match the operation")
	default:
		httpapi.Error(c, 500, "EXPORT_FAILED", "Export could not be created")
	}
}
