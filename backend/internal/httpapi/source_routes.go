package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"github.com/gin-gonic/gin"
	"os"
	"time"
)

type SourceService struct {
	Store          *store.Store
	Jobs           *jobs.Manager
	Media          *media.Service
	Dataset        *dataset.Adapter
	Local          *sources.Catalog
	Catalog        []samples.Item
	MaxUploadBytes int64
	MaxFrames      int
	HTTPTimeout    time.Duration
}
type availability struct {
	Available  bool    `json:"available"`
	ReasonCode *string `json:"reasonCode"`
}
type providerAvailability struct {
	Provider string `json:"provider"`
	availability
}
type sourceAvailability struct {
	SourceKind string `json:"sourceKind"`
	availability
}

func reason(code string) *string {
	if code == "" {
		return nil
	}
	return &code
}

type publicSampleDTO struct {
	SampleID        string  `json:"sampleId"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Creator         string  `json:"creator"`
	SourcePageURL   string  `json:"sourcePageUrl"`
	LicenseName     string  `json:"licenseName"`
	LicenseURL      string  `json:"licenseUrl"`
	Availability    string  `json:"availability"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
}

func (s *SourceService) Register(g *gin.RouterGroup) {
	g.GET("/samples", s.listSamples)
	g.POST("/samples/imports", s.importSample)
	g.GET("/sources/local", s.listLocal)
	g.POST("/sources/local/:sourceId/imports", s.importLocal)
	g.GET("/media-storage/status", s.storageStatus)
}
func (s *SourceService) sampleCatalogItem(id string) (samples.Item, bool) {
	for _, item := range s.Catalog {
		if item.SampleID == id {
			return item, true
		}
	}
	return samples.Item{}, false
}
func (s *SourceService) sampleReady(ctx context.Context, item samples.Item) bool {
	row, e := s.Store.PublicSampleByIdentity(ctx, item.SampleID, item.Version, item.SHA256)
	return e == nil && row.CacheStatus == "ready" && row.StorageKey != "" && samples.VerifyCached(s.Store.Root, row.StorageKey, item) == nil
}
func (s *SourceService) listSamples(c *gin.Context) {
	items := make([]publicSampleDTO, 0, len(s.Catalog))
	for _, item := range s.Catalog {
		state := "unavailable"
		if s.sampleReady(c.Request.Context(), item) {
			state = "ready"
		}
		items = append(items, publicSampleDTO{item.SampleID, item.Name, item.Description, item.Creator, item.SourcePageURL, item.LicenseName, item.LicenseURL, state, item.Width, item.Height, item.DurationSeconds})
	}
	c.JSON(200, gin.H{"items": items})
}
func (s *SourceService) importSample(c *gin.Context) {
	var req struct {
		SampleID string `json:"sampleId"`
	}
	if !Decode(c, &req) {
		return
	}
	item, ok := s.sampleCatalogItem(req.SampleID)
	if !ok {
		Error(c, 404, "NOT_FOUND", "Public sample not found")
		return
	}
	if !s.sampleReady(c.Request.Context(), item) {
		Error(c, 503, "SAMPLE_UNAVAILABLE", "Public sample cache is unavailable")
		return
	}
	v, created, e := s.Store.ImportPublicSample(c.Request.Context(), item.SampleID, item.Version, item.SHA256)
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) || errors.Is(e, store.ErrSampleUnavailable) {
			Error(c, 503, "SAMPLE_UNAVAILABLE", "Public sample cache is unavailable")
		} else {
			failure(c, e)
		}
		return
	}
	if created {
		c.JSON(201, v)
	} else {
		c.JSON(200, v)
	}
}
func localWritable(root string) bool {
	info, e := os.Lstat(root)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	f, e := os.CreateTemp(root, ".local-write-*")
	if e != nil {
		return false
	}
	f.Close()
	return os.Remove(f.Name()) == nil
}
func (s *SourceService) storageStatus(c *gin.Context) {
	local := localWritable(s.Store.Root)
	localReason := ""
	if !local {
		localReason = "MEDIA_UNAVAILABLE"
	}
	datasetReady := local && s.Dataset != nil && s.Dataset.Configured()
	datasetReason := ""
	if !datasetReady {
		datasetReason = "DATASET_NOT_CONFIGURED"
	}
	public := false
	if local {
		for _, item := range s.Catalog {
			if s.sampleReady(c.Request.Context(), item) {
				public = true
				break
			}
		}
	}
	publicReason := ""
	if !public {
		publicReason = "SAMPLE_UNAVAILABLE"
	}
	directory := local && s.Local != nil && len(s.Local.Roots) > 0
	directoryReason := ""
	if !directory {
		directoryReason = "LOCAL_SOURCES_NOT_CONFIGURED"
	}
	c.JSON(200, gin.H{"providers": []providerAvailability{{"local", availability{local, reason(localReason)}}, {"mac", availability{false, reason("REMOTE_STORAGE_RETIRED")}}}, "sources": []sourceAvailability{{"public_sample", availability{public, reason(publicReason)}}, {"local_directory", availability{directory, reason(directoryReason)}}, {"upload", availability{local, reason(localReason)}}, {"dataset", availability{datasetReady, reason(datasetReason)}}}})
}
func retiredStorage(c *gin.Context) {
	Error(c, 410, "REMOTE_STORAGE_RETIRED", "Historical remote media storage has been retired")
}
