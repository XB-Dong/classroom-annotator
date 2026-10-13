package mediaagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
)

// DatasetAdapter uses the observed company API protocol. Its credentials remain in this process.
type DatasetAdapter struct{ adapter *dataset.Adapter }

func NewDatasetAdapter(base, id, key string, privateOrigins, mediaOrigins []string) *DatasetAdapter {
	return &DatasetAdapter{dataset.New(config.Config{DatasetAPIURL: base, DatasetID: id, DatasetAPIKey: key, AllowedPrivateOrigins: privateOrigins, AllowedMediaOrigins: mediaOrigins, HTTPTimeout: 600 * time.Second})}
}
func (d *DatasetAdapter) Lessons(ctx context.Context, page, size int) (any, error) {
	return d.adapter.Lessons(ctx, page, size)
}
func (d *DatasetAdapter) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	return d.adapter.Download(ctx, id)
}

type safeDatasetVideo struct {
	ID            dataset.SafeID `json:"id"`
	LessonID      dataset.SafeID `json:"lesson_id"`
	CatalogFileID dataset.SafeID `json:"catalog_file_id"`
	Channel       int            `json:"channel"`
	PartIndex     int            `json:"part_index"`
}
type safeDatasetLesson struct {
	ID               dataset.SafeID     `json:"id"`
	CourseName       string             `json:"course_name"`
	OrganizationName string             `json:"organization_name"`
	Teacher          string             `json:"teacher"`
	Date             string             `json:"date"`
	SectionIndex     int                `json:"section_index"`
	Name             *string            `json:"name,omitempty"`
	Room             *string            `json:"room,omitempty"`
	StartTime        *string            `json:"start_time,omitempty"`
	EndTime          *string            `json:"end_time,omitempty"`
	Videos           []safeDatasetVideo `json:"videos"`
}
type safeDatasetPage struct {
	Type  string              `json:"type"`
	Items []safeDatasetLesson `json:"items"`
	Total int                 `json:"total"`
}

func (a *Agent) lessons(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Dataset == nil {
		a.fail(w, 503, "DATASET_NOT_CONFIGURED")
		return
	}
	page, size := 1, 20
	var e error
	if s := r.URL.Query().Get("page"); s != "" {
		page, e = strconv.Atoi(s)
		if e != nil {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
	}
	if s := r.URL.Query().Get("page_size"); s != "" {
		size, e = strconv.Atoi(s)
		if e != nil {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
	}
	if page < 1 || size < 1 || size > 100 {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	raw, e := a.cfg.Dataset.Lessons(r.Context(), page, size)
	if e != nil {
		a.fail(w, 502, "PROCESS_FAILED")
		return
	}
	b, e := json.Marshal(raw)
	var safe safeDatasetPage
	if e != nil || int64(len(b)) > a.cfg.MaxControlBytes || json.Unmarshal(b, &safe) != nil || safe.Items == nil || safe.Total < 0 || safe.Type == "" {
		a.fail(w, 502, "PROCESS_FAILED")
		return
	}
	a.json(w, 200, safe)
}
func (a *Agent) datasetImport(w http.ResponseWriter, r *http.Request) {
	var req mediaapi.DatasetImportRequest
	if !a.decode(w, r, &req) {
		return
	}
	if !validID(req.OperationID) || !validID(req.JobID) || !validID(req.VideoID) || !dataset.ValidCatalogID(req.CatalogFileID) || len(req.Name) > 255 {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	if op, ok, e := a.existing(req.OperationID, "dataset_import", req); ok {
		if e != nil {
			a.respondError(w, e)
		} else {
			a.json(w, 202, op)
		}
		return
	}
	if a.cfg.Dataset == nil {
		a.fail(w, 503, "DATASET_NOT_CONFIGURED")
		return
	}
	if !a.probeReady() {
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	stage, dir, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	op, e := a.submitStaged(req.OperationID, "dataset_import", req.JobID, req.VideoID, req, stage, 0, func(ctx context.Context) (any, error) {
		body, e := a.cfg.Dataset.Download(ctx, req.CatalogFileID)
		if e != nil {
			return nil, errors.New("PROCESS_FAILED")
		}
		defer body.Close()
		if _, _, e = copyFile(ctx, filepath.Join(dir, "video"), body, a.cfg.MaxUploadBytes); e != nil {
			return nil, e
		}
		return a.probeVideo(ctx, stage, req.VideoID, req.OperationID)
	})
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.json(w, 202, op)
}
