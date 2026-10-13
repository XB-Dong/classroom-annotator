package exports

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"os"
)

func Register(g *gin.RouterGroup, s *store.Store, registry prometheus.Registerer, limits ...int) {
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
		if err == nil {
			_, err = providerOf(records)
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
	case errors.Is(err, ErrRetired):
		httpapi.Error(c, 410, "REMOTE_STORAGE_RETIRED", "Historical remote media storage has been retired")
	case errors.Is(err, ErrBackfill):
		httpapi.Error(c, 409, "MEDIA_BACKFILL_REQUIRED", "Media integrity metadata requires verified backfill")
	case errors.Is(err, ErrChanged):
		httpapi.Error(c, 409, "MEDIA_CHANGED", "Media bytes differ from their registered identity")
	default:
		httpapi.Error(c, 500, "EXPORT_FAILED", "Export could not be created")
	}
}
