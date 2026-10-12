package main

import (
	"context"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/httpapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
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
	s, e := store.Open(cfg.DataDir)
	if e != nil {
		log.Fatal("storage initialization failed")
	}
	defer s.DB.Close()
	if e = s.Recover(); e != nil {
		log.Fatal("storage reconciliation failed")
	}
	process := media.Process{FFmpeg: cfg.FFmpeg, FFprobe: cfg.FFprobe}
	v := &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: process}
	x := &extraction.Service{Store: s, Runner: process}
	m := jobs.New(s, cfg.QueueSize, cfg.JobTimeout)
	defer m.Close()
	r := httpapi.New(cfg, s, m, v, x, dataset.New(cfg))
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
