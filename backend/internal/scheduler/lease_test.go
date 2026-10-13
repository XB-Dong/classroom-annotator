package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestPostgresLostRecoverySessionCannotRemoveMediaThroughPool(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lease, err := Acquire(ctx, s.DB)
	if err != nil || !lease.Owned() {
		t.Fatal(err)
	}
	defer lease.Close()
	held := lease.Conn()
	var pid int
	if err = held.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(s.Root, "staging", "unprocessed")
	if err = os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	var terminated bool
	if err = s.DB.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated); err != nil || !terminated {
		t.Fatal(err)
	}
	if err = s.RecoverLocalMediaWithContext(ctx, held); err == nil {
		t.Fatal("lost recovery session silently reconnected")
	}
	if _, err = os.Stat(stage); err != nil {
		t.Fatal("lost recovery removed media", err)
	}
	if err = s.DB.PingContext(ctx); err != nil {
		t.Fatal("ordinary pool unavailable", err)
	}
}

func TestPostgresLeaseHasOneOwnerAndReleasesForNextStartup(t *testing.T) {
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := Acquire(ctx, db)
	if err != nil || !first.Owned() {
		t.Fatal("first owner", err)
	}
	defer first.Close()
	second, err := Acquire(ctx, db)
	if err != nil || second.Owned() {
		t.Fatal("second startup took ownership", err)
	}
	defer second.Close()
	if db.Stats().InUse != 1 {
		t.Fatal("non-owner retained dedicated connection", db.Stats())
	}
	if err = second.Recover(ctx); !errors.Is(err, ErrNotOwner) {
		t.Fatal("non-owner recovered jobs", err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := Acquire(ctx, db)
	if err != nil || !third.Owned() {
		t.Fatal("released ownership not reusable", err)
	}
	defer third.Close()
}

func TestPostgresLeaseLossCancelsWorkWithoutReacquiring(t *testing.T) {
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lease, err := Acquire(ctx, db)
	if err != nil || !lease.Owned() {
		t.Fatal(err)
	}
	defer lease.Close()
	var pid int
	if err = lease.conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	workCtx, cancelWork := context.WithCancel(context.Background())
	defer cancelWork()
	watchDone := make(chan struct{})
	go func() { lease.Watch(ctx, cancelWork); close(watchDone) }()
	var terminated bool
	if err = db.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated); err != nil || !terminated {
		t.Fatal("terminate owned session", err)
	}
	select {
	case <-workCtx.Done():
	case <-time.After(4 * time.Second):
		t.Fatal("work was not cancelled after lease loss")
	}
	if lease.Owned() {
		t.Fatal("lost lease still reports ownership")
	}
	if err = db.PingContext(ctx); err != nil {
		t.Fatal("ordinary pool stopped serving", err)
	}
	if err = lease.Recover(ctx); !errors.Is(err, ErrNotOwner) {
		t.Fatal("lost owner recovered through another connection", err)
	}
	select {
	case <-watchDone:
	case <-time.After(time.Second):
		t.Fatal("lost lease watcher did not stop")
	}
	replacement, err := Acquire(ctx, db)
	if err != nil || !replacement.Owned() {
		t.Fatal("new operator startup cannot take released lease", err)
	}
	defer replacement.Close()
	if lease.Owned() {
		t.Fatal("old API automatically reacquired")
	}
}

func TestPostgresLeaseRecoveryDoesNotCancelOtherAPIsJobs(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := Acquire(ctx, s.DB)
	if err != nil || !first.Owned() {
		t.Fatal(err)
	}
	defer first.Close()
	job := domain.Job{ID: domain.ID(), Kind: "import", Status: "running", CreatedAt: domain.Now()}
	if err = s.AddJob(job); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(ctx, s.DB)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err = second.Recover(ctx); !errors.Is(err, ErrNotOwner) {
		t.Fatal(err)
	}
	got, err := s.Job(job.ID)
	if err != nil || got.Status != "running" {
		t.Fatal("second API interrupted existing owner", got, err)
	}
	if err = first.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = s.Job(job.ID)
	if err != nil || got.Status != "failed" || got.Error != "INTERRUPTED" {
		t.Fatal("owner recovery", got, err)
	}
}
