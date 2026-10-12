package annotations

import (
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func Register(g *gin.RouterGroup, s *store.Store, registry prometheus.Registerer) {
	conflicts := prometheus.NewCounter(prometheus.CounterOpts{Name: "classroom_annotation_save_conflicts_total", Help: "Rejected annotation revision conflicts."})
	failures := prometheus.NewCounter(prometheus.CounterOpts{Name: "classroom_annotation_save_errors_total", Help: "Failed annotation save requests excluding conflicts."})
	registry.MustRegister(conflicts, failures)
	repo := Repository{Store: s}
	g.GET("/frames/:id/annotations", func(c *gin.Context) {
		result, err := repo.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, result)
	})
	g.PUT("/frames/:id/annotations", func(c *gin.Context) {
		var body writeRequest
		if !httpapi.Decode(c, &body) {
			failures.Inc()
			return
		}
		save, err := body.save()
		if err == nil {
			var result domain.Snapshot
			result, err = repo.Save(c.Request.Context(), c.Param("id"), save)
			if err == nil {
				c.JSON(200, result)
				return
			}
		}
		if errors.Is(err, ErrConflict) {
			conflicts.Inc()
		} else {
			failures.Inc()
		}
		respondError(c, err)
	})
}

type writeRequest struct {
	ExpectedRevision *int64             `json:"expectedRevision"`
	Status           *string            `json:"status"`
	Annotations      *[]writeAnnotation `json:"annotations"`
}
type writeAnnotation struct {
	ID     *string    `json:"id"`
	BBox   *[]float64 `json:"bbox"`
	Source *string    `json:"source"`
}

var errMissing = errors.New("required field missing")

func (w writeRequest) save() (Save, error) {
	if w.ExpectedRevision == nil || w.Status == nil || w.Annotations == nil {
		return Save{}, errMissing
	}
	if len(*w.Annotations) > MaxAnnotations {
		return Save{}, ErrInvalid
	}
	out := Save{ExpectedRevision: *w.ExpectedRevision, Status: *w.Status, Annotations: make([]domain.Annotation, 0, len(*w.Annotations))}
	for _, a := range *w.Annotations {
		if a.ID == nil || a.BBox == nil || a.Source == nil {
			return Save{}, errMissing
		}
		if len(*a.BBox) != 4 {
			return Save{}, ErrInvalid
		}
		out.Annotations = append(out.Annotations, domain.Annotation{ID: *a.ID, BBox: [4]float64(*a.BBox), Source: *a.Source})
	}
	return out, nil
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpapi.Error(c, 404, "NOT_FOUND", "Frame not found")
	case errors.Is(err, ErrConflict):
		httpapi.Error(c, 409, "REVISION_CONFLICT", "Frame was changed; reload before saving")
	case errors.Is(err, ErrInvalid):
		httpapi.Error(c, 422, "INVALID_ANNOTATIONS", "Annotation snapshot is invalid")
	case errors.Is(err, errMissing):
		httpapi.Error(c, 400, "INVALID_REQUEST", "Required annotation fields are missing")
	default:
		httpapi.Error(c, 500, "INTERNAL_ERROR", "Annotation request failed")
	}
}
