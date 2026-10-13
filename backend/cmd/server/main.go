package main

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/exports"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/help"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/preannotation"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
	"github.com/XB-Dong/classroom-annotator/backend/internal/scheduler"
	"github.com/XB-Dong/classroom-annotator/backend/internal/sources"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()
	s, e := store.Open(cfg.DataDir, store.Options{DatabaseURL: cfg.DatabaseURL, MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns, ConnectTimeout: cfg.DBConnectTimeout, QueryTimeout: cfg.DBQueryTimeout, ConnMaxLifetime: cfg.DBConnMaxLifetime, ConnMaxIdleTime: cfg.DBConnMaxIdleTime})
	if e != nil {
		log.Fatal("storage initialization failed")
	}
	defer s.DB.Close()
	lease, e := scheduler.Acquire(ctx, s.DB)
	if e != nil {
		log.Fatal("task ownership acquisition failed")
	}
	defer lease.Close()
	token := ""
	if cfg.AITokenFile != "" {
		info, err := os.Stat(cfg.AITokenFile)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= 4096 {
			raw, err := os.ReadFile(cfg.AITokenFile)
			if err == nil {
				candidate := strings.TrimSpace(string(raw))
				if candidate != "" && !strings.ContainsAny(candidate, "\r\n\x00") {
					token = candidate
				}
			}
		}
		if token == "" {
			log.Print("optional AI credentials unavailable")
		}
	}
	aiService := preannotation.NewService(s, preannotation.Client{URL: cfg.AIURL, Token: token})
	if lease.Owned() {
		go lease.Watch(ctx, func() { cancelWork(); log.Print("task ownership lost; manual HTTP remains available") })
		if e = aiService.Recover(workCtx); e != nil {
			log.Fatal("AI task reconciliation failed")
		}
		if e = lease.Recover(workCtx); e != nil {
			log.Fatal("task reconciliation failed")
		}
		if e = s.RecoverLocalMediaWithContext(workCtx, lease.Conn()); e != nil {
			log.Fatal("local media reconciliation failed")
		}
	} else {
		cancelWork()
		log.Print("manual HTTP available; task scheduler owned by another process")
	}
	process := &sources.SerializedRunner{Runner: media.Process{FFmpeg: cfg.FFmpeg, FFprobe: cfg.FFprobe}}
	v := &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: process}
	x := &extraction.Service{Store: s, Runner: process}
	m := jobs.NewWithContext(workCtx, s, cfg.QueueSize, cfg.JobTimeout, lease.Owned())
	defer m.Close()
	m.Metrics.Registry.MustRegister(collectors.NewDBStatsCollector(s.DB, "business"))
	if e = aiService.SetMetrics(m.Metrics.Registry); e != nil {
		log.Fatal("AI metrics initialization failed")
	}
	aiDone := make(chan struct{})
	if lease.Owned() {
		go func() {
			defer close(aiDone)
			if err := aiService.Run(workCtx); err != nil && workCtx.Err() == nil {
				cancelWork()
				log.Print("AI task worker stopped; physical tasks disabled")
			}
		}()
	} else {
		close(aiDone)
	}
	defer func() { cancelWork(); <-aiDone }()
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
	helpService, helpErr := help.ConfiguredService(os.Getenv)
	if helpService == nil {
		log.Fatal("user manual initialization failed")
	}
	if helpErr != nil {
		log.Print("optional document help provider unavailable")
	}
	r, e := httpapi.NewWithSecurity(cfg, s, m, v, x, d, sourceService, httpapi.Security{Auth: authHTTP, Resources: authRepo, TaskOwner: lease.Owned, TaskContext: workCtx}, func(g *gin.RouterGroup, s *store.Store) {
		annotations.Register(g, s, m.Metrics.Registry)
		exports.Register(g, s, m.Metrics.Registry, cfg.MaxFrames)
		helpService.Register(g)
		preannotation.Register(g, aiService)
	})
	if e != nil {
		log.Fatal("secure router initialization failed")
	}
	helpService.RegisterDocument(r)
	server := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1024 * 1024}
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
