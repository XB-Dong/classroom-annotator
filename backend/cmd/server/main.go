package main

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/exports"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
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
	if cfg.DatasetAPIKey != "" || cfg.DatasetAPIURL != "" {
		log.Fatal("company dataset credentials must be configured on the Mac media agent")
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
	process := media.Process{FFmpeg: cfg.FFmpeg, FFprobe: cfg.FFprobe}
	v := &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: process}
	x := &extraction.Service{Store: s, Runner: process}
	m := jobs.New(s, cfg.QueueSize, cfg.JobTimeout)
	defer m.Close()
	var bridge *mediabridge.Client
	if cfg.MacBridgeURL != "" {
		token, err := mediabridge.ReadTokenFile(cfg.MacBridgeTokenFile)
		if err != nil {
			log.Fatal("Mac bridge token unavailable")
		}
		bridge, err = mediabridge.New(mediabridge.Config{BaseURL: cfg.MacBridgeURL, Token: token, AllowedOrigins: cfg.AllowedPrivateOrigins, Timeout: cfg.JobTimeout, ControlTimeout: cfg.HTTPTimeout, DownloadTimeout: cfg.MediaDownloadTimeout})
		if err != nil {
			log.Fatal("invalid Mac bridge configuration")
		}
	}
	var catalog []samples.Item
	if cfg.SampleCatalogFile != "" {
		catalog, e = samples.Load(cfg.SampleCatalogFile)
		if e != nil {
			log.Fatal("invalid public sample catalog")
		}
	}
	sources := &httpapi.SourceService{Store: s, Jobs: m, Bridge: bridge, Catalog: catalog, MaxUploadBytes: cfg.MaxUploadBytes, MaxFrames: cfg.MaxFrames}
	recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if e = sources.ReconcileRemote(recoveryCtx); e != nil {
		log.Print("remote reconciliation pending")
	}
	recoveryCancel()
	authRepo := auth.NewPGRepository(s.DB)
	authHTTP, authErr := auth.NewHTTP(auth.NewService(authRepo, cfg.AuthInviteCode), auth.HTTPOptions{TrustedOrigins: cfg.AuthTrustedOrigins, CookieSecure: cfg.AuthCookieSecure})
	if authErr != nil {
		log.Fatal("authentication configuration failed")
	}
	r, e := httpapi.NewWithSecurity(cfg, s, m, v, x, nil, sources, httpapi.Security{Auth: authHTTP, Resources: authRepo}, func(g *gin.RouterGroup, s *store.Store) {
		annotations.Register(g, s, m.Metrics.Registry)
		exports.RegisterWithBridge(g, s, bridge, m.Metrics.Registry, cfg.MaxFrames)
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
