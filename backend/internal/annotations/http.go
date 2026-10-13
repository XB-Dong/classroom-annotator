package annotations

import (
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
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
		if errors.Is(err, ErrConflict) {
			latest, e := repo.Get(c.Request.Context(), c.Param("id"))
			if e == nil {
				writeConflict(c, latest)
				return
			}
		}
		respondError(c, err)
	})
}

func writeConflict(c *gin.Context, latest domain.Snapshot) {
	c.JSON(409, gin.H{"code": "REVISION_CONFLICT", "message": "Frame was changed; reload before saving", "requestId": c.GetString("requestId"), "currentRevision": latest.Revision, "latestUpdatedBy": latest.UpdatedBy})
}

type writeRequest struct {
	ExpectedRevision *int64             `json:"expectedRevision"`
	Status           *string            `json:"status"`
	Annotations      *[]writeAnnotation `json:"annotations"`
}
type writeAnnotation struct {
	ID          *string     `json:"id"`
	BBox        *[]*float64 `json:"bbox"`
	Source      *string     `json:"source"`
	CandidateID string      `json:"candidateId,omitempty"`
	RunID       string      `json:"runId,omitempty"`
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
		var bbox [4]float64
		for i, n := range *a.BBox {
			if n == nil {
				return Save{}, ErrInvalid
			}
			bbox[i] = *n
		}
		out.Annotations = append(out.Annotations, domain.Annotation{ID: *a.ID, BBox: bbox, Source: *a.Source, CandidateID: a.CandidateID, RunID: a.RunID})
	}
	return out, nil
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpapi.Error(c, 404, "NOT_FOUND", "Frame not found")
	case errors.Is(err, ErrConflict):
		httpapi.Error(c, 409, "REVISION_CONFLICT", "Frame was changed; reload before saving")
	case errors.Is(err, ErrIDCollision):
		httpapi.Error(c, 400, "ANNOTATION_ID_COLLISION", "Annotation ID belongs to another frame")
	case errors.Is(err, ErrInvalid):
		httpapi.Error(c, 422, "INVALID_ANNOTATIONS", "Annotation snapshot is invalid")
	case errors.Is(err, reviewstate.ErrUnresolved):
		httpapi.Error(c, 422, "REVIEW_UNRESOLVED", "Resolve candidates and image risks before completing")
	case errors.Is(err, reviewstate.ErrProvenance):
		httpapi.Error(c, 422, "INVALID_PROVENANCE", "Annotation provenance is invalid")
	case errors.Is(err, errMissing):
		httpapi.Error(c, 400, "INVALID_REQUEST", "Required annotation fields are missing")
	default:
		httpapi.Error(c, 500, "INTERNAL_ERROR", "Annotation request failed")
	}
}
