package httpapi

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type API struct {
	Cfg        config.Config
	Store      *store.Store
	Jobs       *jobs.Manager
	Media      *media.Service
	Extraction *extraction.Service
	Dataset    *dataset.Adapter
	Sources    *SourceService
}

// Extensions receive the existing group so annotation/export capabilities can register their routes.
type Extension func(*gin.RouterGroup, *store.Store)
type Security struct {
	Auth      *auth.HTTP
	Resources auth.ResourceAuthorizer
}

func NewWithSecurity(cfg config.Config, s *store.Store, m *jobs.Manager, v *media.Service, x *extraction.Service, d *dataset.Adapter, sources *SourceService, security Security, extensions ...Extension) (*gin.Engine, error) {
	if security.Auth == nil || security.Resources == nil {
		return nil, errors.New("authentication is required")
	}
	r := newRouter(cfg, s, m, v, x, d, sources, &security, extensions...)
	if e := r.SetTrustedProxies(cfg.AuthTrustedProxies); e != nil {
		return nil, errors.New("invalid trusted proxy configuration")
	}
	return r, nil
}

func New(cfg config.Config, s *store.Store, m *jobs.Manager, v *media.Service, x *extraction.Service, d *dataset.Adapter, extensions ...Extension) *gin.Engine {
	return NewWithSources(cfg, s, m, v, x, d, nil, extensions...)
}

func NewWithSources(cfg config.Config, s *store.Store, m *jobs.Manager, v *media.Service, x *extraction.Service, d *dataset.Adapter, sources *SourceService, extensions ...Extension) *gin.Engine {
	return newRouter(cfg, s, m, v, x, d, sources, nil, extensions...)
}

// Legacy constructors are explicit isolated test harnesses. Production uses NewWithSecurity.
func newRouter(cfg config.Config, s *store.Store, m *jobs.Manager, v *media.Service, x *extraction.Service, d *dataset.Adapter, sources *SourceService, security *Security, extensions ...Extension) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("requestId", domain.ID())
		c.Header("X-Request-ID", c.GetString("requestId"))
		start := time.Now()
		defer func() {
			if recover() != nil {
				Error(c, 500, "INTERNAL_ERROR", "Request failed")
			}
			path := c.FullPath()
			if path == "" {
				path = "unmatched"
			}
			method := metricMethod(c.Request.Method)
			m.Metrics.Requests.WithLabelValues(method, path, strconv.Itoa(c.Writer.Status())).Inc()
			m.Metrics.RequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
		}()
		c.Next()
	})
	if security != nil {
		r.Use(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/media/") {
				security.Auth.Protect()(c)
			} else {
				c.Next()
			}
		}, func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/media/") {
				auth.Scope(security.Resources)(c)
			} else {
				c.Next()
			}
		})
		security.Auth.Register(r.Group("/api/v1"))
	}
	r.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "version": cfg.Version}) })
	r.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if s.Ready(ctx) != nil {
			Error(c, 503, "NOT_READY", "Storage unavailable")
			return
		}
		c.JSON(200, gin.H{"status": "ok", "version": cfg.Version})
	})
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(m.Metrics.Registry, promhttp.HandlerOpts{})))
	a := &API{Cfg: cfg, Store: s, Jobs: m, Media: v, Extraction: x, Dataset: d, Sources: sources}
	g := r.Group("/api/v1")
	if security != nil {
		g.Use(security.Auth.Protect(), auth.Scope(security.Resources))
	}
	g.GET("/videos", a.videos)
	g.POST("/videos", a.upload)
	g.POST("/videos/:id/extractions", a.extract)
	g.GET("/videos/:id/batches", a.batches)
	g.GET("/videos/:id/frames", a.frames)
	g.GET("/videos/:id/progress", a.progress)
	g.GET("/jobs", a.jobs)
	g.GET("/jobs/:id", a.job)
	g.POST("/jobs/:id/cancel", a.cancel)
	g.GET("/dataset/lessons", a.lessons)
	g.POST("/dataset/imports", a.importVideo)
	if sources != nil {
		sources.Register(g)
	}
	r.GET("/media/frames/:filename", a.frameMedia)
	r.HEAD("/media/frames/:filename", a.frameMedia)
	if len(extensions) == 0 {
		pending := func(c *gin.Context) {
			Error(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Capability pending implementation")
		}
		g.GET("/frames/:id/annotations", pending)
		g.PUT("/frames/:id/annotations", pending)
		g.POST("/exports", pending)
	} else {
		for _, extend := range extensions {
			extend(g, s)
		}
	}
	r.NoRoute(func(c *gin.Context) { Error(c, 404, "NOT_FOUND", "Resource not found") })
	return r
}
func metricMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return method
	default:
		return "other"
	}
}
