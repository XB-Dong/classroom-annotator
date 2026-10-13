package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
	"github.com/gin-gonic/gin"
)

type SourceService struct {
	Store          *store.Store
	Jobs           *jobs.Manager
	Bridge         *mediabridge.Client
	Catalog        []samples.Item
	MaxUploadBytes int64
	MaxFrames      int
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
	g.GET("/media-sources/mac", s.listMacSources)
	g.POST("/media-sources/mac/imports", s.importMacSource)
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
	row, err := s.Store.PublicSampleByIdentity(ctx, item.SampleID, item.Version, item.SHA256)
	if err != nil || row.CacheStatus != "ready" || row.StorageKey == "" {
		return false
	}
	return samples.VerifyCached(s.Store.Root, row.StorageKey, item) == nil
}

func (s *SourceService) listSamples(c *gin.Context) {
	items := make([]publicSampleDTO, 0, len(s.Catalog))
	for _, item := range s.Catalog {
		state := "unavailable"
		if s.sampleReady(c.Request.Context(), item) {
			state = "ready"
		}
		items = append(items, publicSampleDTO{SampleID: item.SampleID, Name: item.Name, Description: item.Description, Creator: item.Creator, SourcePageURL: item.SourcePageURL, LicenseName: item.LicenseName, LicenseURL: item.LicenseURL, Availability: state, Width: item.Width, Height: item.Height, DurationSeconds: item.DurationSeconds})
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
	v, created, err := s.Store.ImportPublicSample(c.Request.Context(), item.SampleID, item.Version, item.SHA256)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrSampleUnavailable) {
			Error(c, 503, "SAMPLE_UNAVAILABLE", "Public sample cache is unavailable")
		} else {
			failure(c, err)
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
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	f, err := os.CreateTemp(root, ".sample-write-*")
	if err != nil {
		return false
	}
	f.Close()
	return os.Remove(f.Name()) == nil
}

func (s *SourceService) bridgeCapabilities(ctx context.Context) (mediaapi.Capabilities, error) {
	if s.Bridge == nil {
		return mediaapi.Capabilities{}, mediabridge.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.Bridge.Capabilities(ctx, mediabridge.Limits{MaxUploadBytes: s.MaxUploadBytes, MaxFrames: s.MaxFrames, MaxControlBytes: 16 << 20})
}

func (s *SourceService) storageStatus(c *gin.Context) {
	local := localWritable(s.Store.Root)
	localReason := ""
	if !local {
		localReason = "SAMPLE_UNAVAILABLE"
	}
	capabilities, err := s.bridgeCapabilities(c.Request.Context())
	mac := err == nil && capabilities.MediaRootWritable && capabilities.FFmpegAvailable && capabilities.FFprobeAvailable
	macReason := ""
	if !mac {
		macReason = "MEDIA_SOURCE_UNAVAILABLE"
	}
	macSource := mac && capabilities.SourceConfigured
	macSourceReason := ""
	if !macSource {
		macSourceReason = "MEDIA_SOURCE_UNAVAILABLE"
	}
	dataset := mac && capabilities.DatasetConfigured
	datasetReason := ""
	if !dataset {
		if mac {
			datasetReason = "DATASET_NOT_CONFIGURED"
		} else {
			datasetReason = "MEDIA_SOURCE_UNAVAILABLE"
		}
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
	c.JSON(200, gin.H{"providers": []providerAvailability{{Provider: "local", availability: availability{local, reason(localReason)}}, {Provider: "mac", availability: availability{mac, reason(macReason)}}}, "sources": []sourceAvailability{{SourceKind: "public_sample", availability: availability{public, reason(publicReason)}}, {SourceKind: "mac", availability: availability{macSource, reason(macSourceReason)}}, {SourceKind: "upload", availability: availability{mac, reason(macReason)}}, {SourceKind: "dataset", availability: availability{dataset, reason(datasetReason)}}}})
}

var sourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (s *SourceService) listMacSources(c *gin.Context) {
	capabilities, err := s.bridgeCapabilities(c.Request.Context())
	if err != nil || !capabilities.SourceConfigured {
		Error(c, 503, "MEDIA_SOURCE_UNAVAILABLE", "Mac source catalog is unavailable")
		return
	}
	cursor := c.Query("cursor")
	if cursor != "" && !sourceIDPattern.MatchString(cursor) {
		Error(c, 400, "INVALID_REQUEST", "Invalid source cursor")
		return
	}
	items, err := s.Bridge.Sources(c.Request.Context(), cursor)
	if err != nil {
		bridgeFailure(c, err)
		return
	}
	c.JSON(200, items)
}

func bridgeFailure(c *gin.Context, err error) {
	if errors.Is(err, mediabridge.ErrInvalid) {
		Error(c, 400, "INVALID_REQUEST", "Invalid media request")
		return
	}
	if errors.Is(err, mediabridge.ErrContract) {
		Error(c, 409, "BRIDGE_CONTRACT_MISMATCH", "Mac media response did not match the operation")
		return
	}
	var remote *mediabridge.RemoteError
	if errors.As(err, &remote) {
		switch remote.Code {
		case "INVALID_REQUEST":
			Error(c, 400, "INVALID_REQUEST", "Invalid media request")
			return
		case "RESOURCE_NOT_FOUND":
			Error(c, 404, "NOT_FOUND", "Media resource not found")
			return
		case "RESOURCE_CHANGED", "OPERATION_CONFLICT":
			Error(c, 409, remote.Code, "Media resource changed")
			return
		case "QUEUE_FULL":
			Error(c, 429, "QUEUE_FULL", "Media queue is full")
			return
		case "DATASET_NOT_CONFIGURED":
			Error(c, 503, "DATASET_NOT_CONFIGURED", "Company API is unavailable")
			return
		}
	}
	Error(c, 503, "MEDIA_SOURCE_UNAVAILABLE", "Media source is unavailable")
}
