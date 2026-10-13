package sources

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"strings"
	"sync"
	"testing"
)

func repositoryFixture(t *testing.T) (Repository, *store.Store) {
	t.Helper()
	s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	return Repository{DB: s.DB}, s
}
func TestPostgreSQLLocalImportClaimIsAtomicDurableAndScoped(t *testing.T) {
	r, s := repositoryFixture(t)
	ctx := context.Background()
	source := strings.Repeat("a", 64)
	start := make(chan struct{})
	var group sync.WaitGroup
	ids := make(chan string, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			j, _, e := r.Claim(ctx, auth.DefaultWorkspaceID, "idempotency-fixture", source, domain.Job{ID: domain.ID(), Kind: "import", Status: "queued", CreatedAt: domain.Now()}, true)
			if e != nil {
				t.Error(e)
				return
			}
			ids <- j.ID
		}()
	}
	close(start)
	group.Wait()
	close(ids)
	id := ""
	for found := range ids {
		if id != "" && found != id {
			t.Fatal("duplicate job", id, found)
		}
		id = found
	}
	var count int
	if e := s.DB.QueryRow(`SELECT count(*) FROM jobs`).Scan(&count); e != nil || count != 1 {
		t.Fatal("non-atomic request", count, e)
	}
	j, created, e := r.Claim(ctx, auth.DefaultWorkspaceID, "idempotency-fixture", source, domain.Job{}, false)
	if e != nil || created || j.ID != id {
		t.Fatal("retry after capacity/restart", j, created, e)
	}
	if _, _, e = r.Claim(ctx, auth.DefaultWorkspaceID, "idempotency-fixture", strings.Repeat("b", 64), domain.Job{}, true); !errors.Is(e, ErrIdempotency) {
		t.Fatal("intent conflict", e)
	}
	if _, _, e = r.Claim(ctx, auth.DefaultWorkspaceID, "fresh-capacity-key", source, domain.Job{}, false); !errors.Is(e, jobs.ErrQueue) {
		t.Fatal("capacity claim", e)
	}
	if e = s.DB.QueryRow(`SELECT count(*) FROM jobs`).Scan(&count); e != nil || count != 1 {
		t.Fatal("capacity rejection created job", count, e)
	}
}

func TestPostgreSQLLocalPublicationReusesIdentityAndHonorsCancellation(t *testing.T) {
	r, s := repositoryFixture(t)
	ctx := context.Background()
	source := strings.Repeat("a", 64)
	sha := strings.Repeat("b", 64)
	makeJob := func(key string) domain.Job {
		candidate := domain.Job{ID: domain.ID(), Kind: "import", Status: "queued", CreatedAt: domain.Now()}
		j, _, e := r.Claim(ctx, auth.DefaultWorkspaceID, key, source, candidate, true)
		if e != nil {
			t.Fatal(e)
		}
		if ok, e := s.StartJob(j.ID); e != nil || !ok {
			t.Fatal(ok, e)
		}
		return j
	}
	video := func() domain.Video {
		return domain.Video{ID: domain.ID(), Name: "fixture", Width: 16, Height: 12, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/" + domain.ID(), SourceKind: "local_directory", SourceID: source, StorageProvider: "local", MediaSHA256: sha, MediaSizeBytes: 100}
	}
	first := makeJob("first-key")
	if reused, e := r.PublishImport(ctx, first, video()); e != nil || reused {
		t.Fatal(reused, e)
	}
	second := makeJob("second-key")
	if reused, e := r.PublishImport(ctx, second, video()); e != nil || !reused {
		t.Fatal("source identity not reused", reused, e)
	}
	a, e := s.Job(first.ID)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.Job(second.ID)
	if e != nil || a.VideoID == "" || a.VideoID != b.VideoID || b.Status != "completed" {
		t.Fatal(a, b, e)
	}
	cancelled := makeJob("cancel-key")
	s.CancelJob(cancelled.ID)
	if _, e = r.PublishImport(ctx, cancelled, video()); !errors.Is(e, store.ErrState) {
		t.Fatal("cancelled import published", e)
	}
	var count int
	if e = s.DB.QueryRow(`SELECT count(*) FROM videos`).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}
func TestPostgreSQLLocalProviderMigrationPreservesHistoricalMac(t *testing.T) {
	_, s := repositoryFixture(t)
	for _, tc := range []struct{ kind, provider string }{{"upload", "local"}, {"dataset", "local"}, {"mac", "mac"}, {"dataset", "mac"}, {"upload", "mac"}} {
		v := domain.Video{ID: domain.ID(), Name: "fixture", Width: 16, Height: 12, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: domain.ID(), SourceKind: tc.kind, StorageProvider: tc.provider}
		if e := s.AddVideo(v); e != nil {
			t.Fatal(tc, e)
		}
	}
}
