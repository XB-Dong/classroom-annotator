package preannotation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/exports"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
	"github.com/gin-gonic/gin"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These tests use a real isolated PostgreSQL schema and synthetic HTTP inference
// responses. They validate persistence/concurrency, never model accuracy.
func pgFixture(t *testing.T, handler http.HandlerFunc) (*Service, context.Context) {
	t.Helper()
	db, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.DB.Close() })
	v := domain.Video{ID: "v", Name: "Synthetic", Width: 100, Height: 80, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
	if e = db.AddVideo(v); e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = jpeg.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 100, 80)), nil); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw.Bytes())
	digest := hex.EncodeToString(sum[:])
	j := domain.Job{ID: "extraction", Kind: "extraction", VideoID: "v", BatchID: "b", Status: "running", CreatedAt: domain.Now()}
	if e = db.AddJob(j); e != nil {
		t.Fatal(e)
	}
	frames := []domain.Frame{}
	for i, id := range []string{"f1", "f2"} {
		key := filepath.Join("frames", "b", id+".jpg")
		if e = os.MkdirAll(filepath.Dir(filepath.Join(db.Root, key)), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(db.Root, key), raw.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		frames = append(frames, domain.Frame{ID: id, VideoID: "v", BatchID: "b", Width: 100, Height: 80, TimestampMs: int64(i), StorageKey: key, MediaSHA256: digest, MediaSizeBytes: int64(raw.Len())})
	}
	if e = db.PublishBatch(j, frames); e != nil {
		t.Fatal(e)
	}
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(pgResult()) }
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	s := NewService(db, Client{server.URL, "internal-token"})
	return s, pgActor(t, db, "alice")
}
func pgResult() DetectionResult {
	r := DetectionResult{ContractVersion: "1.0.0", ModelID: "synthetic-inference-fixture", ModelSHA256: ModelSHA256, PipelineVersion: PipelineVersion, Detections: []Detection{{BBox: [4]float64{1, 2, 20, 20}, Confidence: .8, ClassID: 7}}}
	r.Image.Width = 100
	r.Image.Height = 80
	return r
}
func pgActor(t *testing.T, s *store.Store, name string) context.Context {
	t.Helper()
	svc := auth.NewService(auth.NewPGRepository(s.DB), "invite")
	if _, e := svc.Register(context.Background(), name, "long-password-123", "invite"); e != nil {
		t.Fatal(e)
	}
	token, _, e := svc.Login(context.Background(), name, "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	h, e := auth.NewHTTP(svc, auth.HTTPOptions{TrustedOrigins: []string{"http://localhost"}})
	if e != nil {
		t.Fatal(e)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var ctx context.Context
	router.GET("/actor", h.Protect(), func(c *gin.Context) { ctx = c.Request.Context(); c.Status(204) })
	req := httptest.NewRequest("GET", "/actor", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 204 || ctx == nil {
		t.Fatal("actor fixture", w.Code)
	}
	return ctx
}
func startFrame(t *testing.T, s *Service, ctx context.Context, key string, revision int64) domain.Job {
	t.Helper()
	j, e := s.Enqueue(ctx, "frame:f1", key, []string{"f1"}, &revision)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.execute(context.Background(), j.ID); e != nil {
		t.Fatal(e)
	}
	return j
}

func awaitInference(t *testing.T, started <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-started:
	case e := <-done:
		t.Fatal("AI worker ended before request", e)
	case <-time.After(5 * time.Second):
		t.Fatal("AI request did not start")
	}
}
func releaseOnCleanup(t *testing.T, release chan struct{}) {
	t.Helper()
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
}

func TestPostgreSQLReviewLifecycleGateUndoProvenanceAndRerun(t *testing.T) {
	s, ctx := pgFixture(t, nil)
	job := startFrame(t, s, ctx, "first", 0)
	snap, e := s.Get(ctx, "f1")
	if e != nil || snap.Revision != 1 || len(snap.Candidates) != 1 || len(snap.Annotations) != 0 {
		t.Fatal(snap, e)
	}
	repo := annotations.Repository{Store: s.Store}
	if _, e = repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: 1, Status: "completed"}); !errors.Is(e, reviewstate.ErrUnresolved) {
		t.Fatal("pending completed", e)
	}
	saved, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: 1, Status: "in_progress"})
	if e != nil {
		t.Fatal("pending blocked manual save", e)
	}
	snap, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: saved.Revision, Decisions: []Decision{{CandidateID: snap.Candidates[0].ID, Decision: "accepted"}}})
	if e != nil || len(snap.Annotations) != 1 || snap.Annotations[0].Source != "ai" {
		t.Fatal(snap, e)
	}
	original := snap.Annotations[0]
	fake := original
	fake.Source = "manual"
	fake.CandidateID = ""
	fake.RunID = ""
	if _, e = repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "in_progress", Annotations: []domain.Annotation{fake}}); !errors.Is(e, reviewstate.ErrProvenance) {
		t.Fatal("manual cast accepted", e)
	}
	deleted, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "in_progress"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e = s.Get(ctx, "f1")
	if e != nil || snap.Candidates[0].ReviewState != "rejected" {
		t.Fatal(snap, e)
	}
	restored, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: deleted.Revision, Status: "in_progress", Annotations: []domain.Annotation{original}})
	if e != nil || restored.Annotations[0].Source != "ai" {
		t.Fatal(restored, e)
	}
	moved := original
	moved.BBox[0] = 7
	moved.BBox[1] = 8
	adjusted, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: restored.Revision, Status: "in_progress", Annotations: []domain.Annotation{moved}})
	if e != nil || adjusted.Annotations[0].Source != "ai_corrected" {
		t.Fatal(adjusted, e)
	}
	complete, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: adjusted.Revision, Status: "completed", Annotations: adjusted.Annotations})
	if e != nil {
		t.Fatal(e)
	}
	records, e := (exports.Service{Store: s.Store}).Snapshot(ctx, exports.Selection{FrameIDs: []string{"f1"}})
	if e != nil || len(records) != 1 || len(records[0].Annotations) != 1 {
		t.Fatal(records, e)
	}
	stats, e := s.Stats(ctx, "v", "")
	if e != nil || stats.Adjusted != 1 || stats.Accepted != 0 || stats.Pending != 0 || stats.Rates.Adjusted == nil || *stats.Rates.Adjusted != 1 {
		t.Fatal(stats, e)
	}
	manual := domain.Annotation{ID: "manual", Source: "manual", BBox: [4]float64{70, 50, 10, 10}}
	anns := append(complete.Annotations, manual)
	added, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: complete.Revision, Status: "in_progress", Annotations: anns})
	if e != nil {
		t.Fatal(e)
	}
	stats, e = s.Stats(ctx, "v", "")
	if e != nil || stats.ManualAdded != 1 {
		t.Fatal(stats, e)
	}
	startFrame(t, s, ctx, "rerun", added.Revision)
	snap, e = s.Get(ctx, "f1")
	if e != nil || len(snap.Annotations) != 2 || snap.Candidates[0].ReviewState != "pending" {
		t.Fatal("rerun erased formal work", snap, e)
	}
	stats, e = s.Stats(ctx, "v", "")
	if e != nil || stats.ManualAdded != 0 || stats.Pending != 1 {
		t.Fatal("rerun double counted", stats, e)
	}
	zero := int64(0)
	same, e := s.Enqueue(ctx, "frame:f1", "first", []string{"f1"}, &zero)
	if e != nil || same.ID != job.ID || same.Status != "completed" {
		t.Fatal("terminal idempotency", same, e)
	}
	one := int64(1)
	if _, e = s.Enqueue(ctx, "frame:f1", "first", []string{"f1"}, &one); !errors.Is(e, ErrIdempotency) {
		t.Fatal("key payload mismatch", e)
	}
	var events int
	if e = s.Store.DB.QueryRow(`SELECT count(*) FROM annotation_events WHERE frame_id='f1'`).Scan(&events); e != nil || int64(events) != snap.Revision {
		t.Fatal("audit missing or duplicated", events, snap.Revision, e)
	}
}

func TestPostgreSQLTwentySameRevisionRacesHaveOneActorAndAudit(t *testing.T) {
	s, a := pgFixture(t, nil)
	b := pgActor(t, s.Store, "bravo")
	repo := annotations.Repository{Store: s.Store}
	for i := int64(0); i < 20; i++ {
		start := make(chan struct{})
		results := make(chan error, 2)
		for n, actor := range []context.Context{a, b} {
			go func(ctx context.Context, n int) {
				<-start
				_, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: i, Status: "in_progress", Annotations: []domain.Annotation{{ID: fmt.Sprintf("a-%d-%d", i, n), BBox: [4]float64{1, 2, 12, 13}, Source: "manual"}}})
				results <- e
			}(actor, n)
		}
		close(start)
		wins, conflicts := 0, 0
		for n := 0; n < 2; n++ {
			e := <-results
			if e == nil {
				wins++
			} else if errors.Is(e, annotations.ErrConflict) {
				conflicts++
			} else {
				t.Fatal(e)
			}
		}
		if wins != 1 || conflicts != 1 {
			t.Fatal("CAS race", i, wins, conflicts)
		}
	}
	got, e := repo.Get(a, "f1")
	if e != nil || got.Revision != 20 || got.UpdatedBy == "" || len(got.Annotations) != 1 {
		t.Fatal(got, e)
	}
	var events int
	if e = s.Store.DB.QueryRow(`SELECT count(*) FROM annotation_events WHERE frame_id='f1'`).Scan(&events); e != nil || events != 20 {
		t.Fatal("loser audit", events, e)
	}
}

func TestPostgreSQLAIStaleFailureAndCancellationDoNotOverwriteManual(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure-%t", failure), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-release
				if failure {
					w.WriteHeader(503)
				} else {
					json.NewEncoder(w).Encode(pgResult())
				}
			})
			releaseOnCleanup(t, release)
			revision := int64(0)
			j, e := s.Enqueue(ctx, "frame:f1", "key", []string{"f1"}, &revision)
			if e != nil {
				t.Fatal(e)
			}
			done := make(chan error, 1)
			go func() { done <- s.execute(context.Background(), j.ID) }()
			awaitInference(t, started, done)
			saved, e := (annotations.Repository{Store: s.Store}).Save(ctx, "f1", annotations.Save{ExpectedRevision: 0, Status: "completed", Annotations: []domain.Annotation{{ID: "manual", Source: "manual", BBox: [4]float64{60, 50, 10, 10}}}})
			if e != nil {
				t.Fatal(e)
			}
			close(release)
			if e = <-done; e != nil {
				t.Fatal(e)
			}
			got, e := s.Get(ctx, "f1")
			if e != nil || got.Revision != saved.Revision || got.Status != "completed" || len(got.Annotations) != 1 || got.CurrentRun == nil || len(got.Issues) != 0 {
				t.Fatal("late AI overwrote manual", got, e)
			}
			var state string
			if e = s.Store.DB.QueryRow(`SELECT status FROM ai_runs WHERE job_id=$1`, j.ID).Scan(&state); e != nil {
				t.Fatal(e)
			}
			want := "stale"
			if failure {
				want = "failed"
			}
			if state != want {
				t.Fatal(state, want)
			}
			if got.CurrentRun.Status != want {
				t.Fatal("late outcome invisible", got.CurrentRun)
			}
		})
	}
	t.Run("cancel", func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			json.NewEncoder(w).Encode(pgResult())
		})
		releaseOnCleanup(t, release)
		zero := int64(0)
		j, e := s.Enqueue(ctx, "frame:f1", "cancel", []string{"f1"}, &zero)
		if e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { done <- s.execute(context.Background(), j.ID) }()
		awaitInference(t, started, done)
		if _, e = s.Store.CancelJob(j.ID); e != nil {
			t.Fatal(e)
		}
		close(release)
		if e = <-done; e != nil {
			t.Fatal(e)
		}
		snap, e := s.Get(ctx, "f1")
		if e != nil || snap.Revision != 0 || snap.CurrentRun == nil || snap.CurrentRun.Status != "cancelled" || len(snap.Issues) > 0 {
			t.Fatal("cancel published", snap, e)
		}
		job, e := s.Store.Job(j.ID)
		if e != nil || job.Status != "cancelled" {
			t.Fatal(job, e)
		}
	})
}

func TestPostgreSQLEmptyAndFailedImagesRequireExplicitCASResolution(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure-%t", failure), func(t *testing.T) {
			s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if failure {
					w.WriteHeader(503)
					return
				}
				result := pgResult()
				result.Detections = []Detection{}
				json.NewEncoder(w).Encode(result)
			})
			startFrame(t, s, ctx, "issue", 0)
			snap, e := s.Get(ctx, "f1")
			if e != nil || len(snap.Issues) != 1 || snap.Issues[0].Resolved {
				t.Fatal(snap, e)
			}
			repo := annotations.Repository{Store: s.Store}
			if _, e = repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "completed"}); !errors.Is(e, reviewstate.ErrUnresolved) {
				t.Fatal("unresolved image complete", e)
			}
			if _, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: 0, IssueResolutions: []IssueResolution{{ID: snap.Issues[0].ID, Resolution: "manual_checked"}}}); !errors.Is(e, annotations.ErrConflict) {
				t.Fatal("issue stale write", e)
			}
			snap, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, IssueResolutions: []IssueResolution{{ID: snap.Issues[0].ID, Resolution: "manual_checked"}}})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "completed"}); e != nil {
				t.Fatal("explicit zero completion", e)
			}
			records, e := (exports.Service{Store: s.Store}).Snapshot(ctx, exports.Selection{FrameIDs: []string{"f1"}})
			if e != nil || len(records) != 1 || len(records[0].Annotations) != 0 {
				t.Fatal(records, e)
			}
		})
	}
}

func TestPostgreSQLOverlapAndHighRiskRequireIndividualDecisions(t *testing.T) {
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		result := pgResult()
		result.Detections = append(result.Detections, Detection{BBox: [4]float64{60, 30, 8, 8}, Confidence: .3, ClassID: 7})
		json.NewEncoder(w).Encode(result)
	})
	repo := annotations.Repository{Store: s.Store}
	saved, e := repo.Save(ctx, "f1", annotations.Save{Status: "in_progress", Annotations: []domain.Annotation{{ID: "existing", Source: "manual", BBox: [4]float64{1, 2, 20, 20}}}})
	if e != nil {
		t.Fatal(e)
	}
	startFrame(t, s, ctx, "overlap", saved.Revision)
	snap, e := s.Get(ctx, "f1")
	if e != nil || len(snap.Candidates) != 2 {
		t.Fatal(snap, e)
	}
	var overlap, small Candidate
	for _, c := range snap.Candidates {
		for _, risk := range c.RiskReasons {
			if risk == "OVERLAP_EXISTING" {
				overlap = c
			}
			if risk == "SMALL_HEAD" {
				small = c
			}
		}
	}
	if overlap.ID == "" || small.ID == "" {
		t.Fatal("risk absent", snap)
	}
	if _, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: overlap.ID, Decision: "accepted"}}}); !errors.Is(e, ErrInvalidReview) {
		t.Fatal("duplicate label accepted", e)
	}
	if _, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: overlap.ID, Decision: "rejected", ReasonCode: "ALREADY_LABELED"}, {CandidateID: small.ID, Decision: "accepted"}}}); !errors.Is(e, ErrInvalidReview) {
		t.Fatal("risk bulk accepted", e)
	}
	removed, e := repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "in_progress"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: removed.Revision, Decisions: []Decision{{CandidateID: overlap.ID, Decision: "accepted"}}})
	if e != nil || len(snap.Annotations) != 1 {
		t.Fatal("removed original still blocks", snap, e)
	}
	if _, e = repo.Save(ctx, "f1", annotations.Save{ExpectedRevision: snap.Revision, Status: "completed", Annotations: snap.Annotations}); !errors.Is(e, reviewstate.ErrUnresolved) {
		t.Fatal("second pending bypass", e)
	}
	// Even a corrupt direct status update cannot bypass the independent export gate.
	if _, e = s.Store.DB.Exec(`UPDATE frames SET status='completed' WHERE id='f1'`); e != nil {
		t.Fatal(e)
	}
	if _, e = (exports.Service{Store: s.Store}).Snapshot(ctx, exports.Selection{FrameIDs: []string{"f1"}}); !errors.Is(e, exports.ErrInvalid) {
		t.Fatal("direct export bypass", e)
	}
}

func TestPostgreSQLConcurrentIdempotencyAndBatchRetriesKeepOneJob(t *testing.T) {
	s, ctx := pgFixture(t, nil)
	zero := int64(0)
	start := make(chan struct{})
	results := make(chan struct {
		Job domain.Job
		Err error
	}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			j, e := s.Enqueue(ctx, "frame:f1", "same", []string{"f1"}, &zero)
			results <- struct {
				Job domain.Job
				Err error
			}{j, e}
		}()
	}
	close(start)
	a, b := <-results, <-results
	if a.Err != nil || b.Err != nil || a.Job.ID != b.Job.ID {
		t.Fatal("concurrent idempotency", a, b)
	}
	var count int
	if e := s.Store.DB.QueryRow(`SELECT count(*) FROM ai_runs WHERE frame_id='f1'`).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	if _, e := s.Store.CancelJob(a.Job.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Store.DB.Exec(`UPDATE ai_runs SET status='cancelled' WHERE job_id=$1`, a.Job.ID); e != nil {
		t.Fatal(e)
	}
	j, e := s.Enqueue(ctx, "batch:b", "batch-key", []string{"f1", "f2"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = (annotations.Repository{Store: s.Store}).Save(ctx, "f1", annotations.Save{Status: "in_progress"}); e != nil {
		t.Fatal(e)
	}
	same, e := s.Enqueue(ctx, "batch:b", "batch-key", []string{"f2", "f1"}, nil)
	if e != nil || same.ID != j.ID {
		t.Fatal("batch remapped child runs after retry", same, e)
	}
	if _, e = s.Enqueue(ctx, "batch:b", "batch-key", []string{"f1"}, nil); !errors.Is(e, ErrIdempotency) {
		t.Fatal("batch payload mismatch", e)
	}
}

func TestPostgreSQLQueueTimeoutAndRecoveryNeverStartOldModelWork(t *testing.T) {
	calls := 0
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; json.NewEncoder(w).Encode(pgResult()) })
	zero := int64(0)
	job, e := s.Enqueue(ctx, "frame:f1", "expired", []string{"f1"}, &zero)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.DB.Exec(`UPDATE ai_requests SET created_at=now()-interval '121 seconds' WHERE job_id=$1`, job.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.execute(context.Background(), job.ID); e != nil {
		t.Fatal(e)
	}
	job, e = s.Store.Job(job.ID)
	if e != nil || job.Status != "failed" || job.Error != "QUEUE_TIMEOUT" || calls != 0 {
		t.Fatal("expired work started", job, calls, e)
	}
	snap, e := s.Get(ctx, "f1")
	if e != nil || len(snap.Issues) != 1 || snap.CurrentRun.Status != "failed" {
		t.Fatal(snap, e)
	}
	job, e = s.Enqueue(ctx, "frame:f2", "restart", []string{"f2"}, &zero)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.DB.Exec(`UPDATE jobs SET status='cancelling' WHERE id=$1`, job.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Recover(context.Background()); e != nil {
		t.Fatal(e)
	}
	job, e = s.Store.Job(job.ID)
	if e != nil || job.Status != "cancelled" || calls != 0 {
		t.Fatal("recovered old model work", job, calls, e)
	}
	var state string
	if e = s.Store.DB.QueryRow(`SELECT status FROM ai_runs WHERE job_id=$1`, job.ID).Scan(&state); e != nil || state != "cancelled" {
		t.Fatal(state, e)
	}
}
