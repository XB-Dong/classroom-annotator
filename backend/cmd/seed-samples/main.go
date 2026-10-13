package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/samples"
)

type result struct {
	SampleID   string `json:"sampleId"`
	Version    string `json:"version"`
	SHA256     string `json:"sha256"`
	Status     string `json:"status"`
	StorageKey string `json:"storageKey,omitempty"`
}
type report struct {
	CatalogVersion string   `json:"catalogVersion"`
	StartedAt      string   `json:"startedAt"`
	FinishedAt     string   `json:"finishedAt"`
	Results        []result `json:"results"`
}

func outsideCheckout(path string) bool {
	dir := filepath.Dir(path)
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			for current := resolved; ; current = filepath.Dir(current) {
				if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
					return false
				}
				if filepath.Dir(current) == current {
					break
				}
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return false
		}
		if filepath.Dir(dir) == dir {
			return true
		}
		dir = filepath.Dir(dir)
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal("public sample seed failed: ", err)
	}
}

func run() error {
	catalogPath := flag.String("catalog", "", "absolute path to publisher catalog colocated with public media")
	reportPath := flag.String("report", "", "absolute path to external seed report")
	flag.Parse()
	if !filepath.IsAbs(*catalogPath) || !filepath.IsAbs(*reportPath) || !outsideCheckout(*reportPath) {
		return errors.New("catalog and external report paths are required")
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return errors.New("invalid application configuration")
	}
	items, err := samples.Load(*catalogPath)
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.DataDir, store.Options{DatabaseURL: cfg.DatabaseURL, MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns, ConnectTimeout: cfg.DBConnectTimeout, QueryTimeout: cfg.DBQueryTimeout, ConnMaxLifetime: cfg.DBConnMaxLifetime, ConnMaxIdleTime: cfg.DBConnMaxIdleTime})
	if err != nil {
		return errors.New("database unavailable or schema mismatched")
	}
	defer s.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	started := time.Now().UTC().Format(time.RFC3339Nano)
	results := make([]result, 0, len(items))
	process := media.Process{FFmpeg: cfg.FFmpeg, FFprobe: cfg.FFprobe}
	var seedErr error
	for _, item := range items {
		row := result{SampleID: item.SampleID, Version: item.Version, SHA256: item.SHA256, Status: "failed"}
		key, err := samples.Cache(ctx, filepath.Dir(*catalogPath), s.Root, item, process)
		if err == nil {
			err = s.UpsertPublicSample(ctx, store.PublicSample{SampleID: item.SampleID, Version: item.Version, Name: item.Name, Description: item.Description, Creator: item.Creator, LicenseName: item.LicenseName, LicenseURL: item.LicenseURL, SourcePageURL: item.SourcePageURL, SHA256: item.SHA256, SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height, DurationSeconds: item.DurationSeconds, StorageKey: key, CacheStatus: "ready"})
		}
		if err == nil {
			row.Status = "ready"
			row.StorageKey = key
		} else {
			seedErr = errors.Join(seedErr, errors.New(item.SampleID+": verification or catalog registration failed"))
		}
		results = append(results, row)
	}
	rep := report{CatalogVersion: "1.0.0", StartedAt: started, FinishedAt: time.Now().UTC().Format(time.RFC3339Nano), Results: results}
	if err := os.MkdirAll(filepath.Dir(*reportPath), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(*reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(f).Encode(rep); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return seedErr
}
