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
		var path string
		if err == nil {
			path, err = service.Create(c.Request.Context(), selection)
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
		httpapi.Error(c, 422, "INVALID_EXPORT_SELECTION", "Select a video scope or a unique nonempty frame list")
	case errors.Is(err, ErrEmpty):
		httpapi.Error(c, 422, "NO_EXPORTABLE_FRAMES", "Selected scope has no completed frames")
	case errors.Is(err, ErrLimit):
		httpapi.Error(c, 422, "EXPORT_LIMIT_EXCEEDED", "Export exceeds supported limits; select a smaller scope")
	default:
		httpapi.Error(c, 500, "EXPORT_FAILED", "Export could not be created")
	}
}
