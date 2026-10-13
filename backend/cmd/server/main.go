package main

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/exports"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		log.Fatal(e)
	}
	if cfg.Version == "dev" {
		cfg.Version = version
	}
	s, e := store.Open(cfg.DataDir, store.Options{DatabaseURL: cfg.DatabaseURL, MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns, ConnectTimeout: cfg.DBConnectTimeout, QueryTimeout: cfg.DBQueryTimeout, ConnMaxLifetime: cfg.DBConnMaxLifetime, ConnMaxIdleTime: cfg.DBConnMaxIdleTime})
	if e != nil {
		log.Fatal("storage initialization failed")
	}
	defer s.DB.Close()
	if e = s.Recover(); e != nil {
		log.Fatal("storage reconciliation failed")
	}
	if e = s.RecoverLocalMedia(); e != nil {
		log.Fatal("local media reconciliation failed")
	}
	process := &sources.SerializedRunner{Runner: media.Process{FFmpeg: cfg.FFmpeg, FFprobe: cfg.FFprobe}}
	v := &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: process}
	x := &extraction.Service{Store: s, Runner: process}
	m := jobs.New(s, cfg.QueueSize, cfg.JobTimeout)
	defer m.Close()
	d := dataset.New(cfg)
	var catalog []samples.Item
	if cfg.SampleCatalogFile != "" {
		catalog, e = samples.Load(cfg.SampleCatalogFile)
		if e != nil {
			log.Fatal("invalid public sample catalog")
		}
	}
	sourceService := &httpapi.SourceService{Store: s, Jobs: m, Media: v, Dataset: d, Local: &sources.Catalog{Roots: cfg.LocalSourceDirs, Media: v}, Catalog: catalog, MaxUploadBytes: cfg.MaxUploadBytes, MaxFrames: cfg.MaxFrames, HTTPTimeout: cfg.HTTPTimeout}
	authRepo := auth.NewPGRepository(s.DB)
	authHTTP, authErr := auth.NewHTTP(auth.NewService(authRepo, cfg.AuthInviteCode), auth.HTTPOptions{TrustedOrigins: cfg.AuthTrustedOrigins, CookieSecure: cfg.AuthCookieSecure})
	if authErr != nil {
		log.Fatal("authentication configuration failed")
	}
	r, e := httpapi.NewWithSecurity(cfg, s, m, v, x, d, sourceService, httpapi.Security{Auth: authHTTP, Resources: authRepo}, func(g *gin.RouterGroup, s *store.Store) {
		annotations.Register(g, s, m.Metrics.Registry)
		exports.Register(g, s, m.Metrics.Registry, cfg.MaxFrames)
	})
	if e != nil {
		log.Fatal("secure router initialization failed")
	}
	server := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1024 * 1024}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Print("HTTP server failed")
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}
