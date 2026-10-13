package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

// Cancelled requests must not dial PostgreSQL, even after the pool could reconnect.
func TestOwnershipContextFencesDatabaseWork(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Store{DB: db}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, call := range map[string]func() error{
		"enqueue":       func() error { return s.AddJobWithContext(ctx, domain.Job{}) },
		"read":          func() error { _, e := s.JobWithContext(ctx, "job"); return e },
		"start":         func() error { _, e := s.StartJobWithContext(ctx, "job"); return e },
		"finish":        func() error { return s.FinishJobWithContext(ctx, "job", "failed", "INTERRUPTED") },
		"progress":      func() error { return s.SetProgressWithContext(ctx, "job", .5) },
		"publishBatch":  func() error { return s.PublishBatchWithContext(ctx, domain.Job{}, nil) },
		"publishImport": func() error { return s.PublishImportWithContext(ctx, domain.Job{}, domain.Video{}) },
		"upload":        func() error { return s.AddVideoWithContext(ctx, domain.Video{}) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled ownership not propagated", err)
			}
		})
	}
	if db.Stats().OpenConnections != 0 {
		t.Fatal("cancelled work opened connection", db.Stats())
	}
}

func TestCancelledLocalRecoveryLeavesFilesUntouched(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "staging", "still-owned")
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	s := &Store{Root: root}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.RecoverLocalMediaWithContext(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatal("cancelled recovery removed files", err)
	}
}

func TestLocalRecoveryRefusesMissingSchedulerSession(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.RecoverLocalMediaWithContext(context.Background(), nil); err == nil {
		t.Fatal("production recovery fell back to pool without a held session")
	}
}
