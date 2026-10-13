package preannotation

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

// Real PostgreSQL fixtures with synthetic inference; these do not assess model accuracy.
func TestPostgreSQLRiskImageIndexActivePendingResolvedAndSuperseded(t *testing.T) {
	var risky atomic.Bool
	risky.Store(true)
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		result := pgResult()
		if risky.Load() {
			result.Detections[0].Confidence = .3
		}
		json.NewEncoder(w).Encode(result)
	})
	assertIDs := func(want []string) {
		t.Helper()
		stats, err := s.Stats(ctx, "v", "b")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(stats.UnresolvedRiskFrameIDs, want) {
			t.Fatalf("risk IDs=%v want=%v", stats.UnresolvedRiskFrameIDs, want)
		}
		if stats.UnresolvedRiskImages != len(want) {
			t.Fatalf("count and index disagree: %+v", stats)
		}
		raw, err := json.Marshal(stats)
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]json.RawMessage
		if err = json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		if string(wire["unresolvedRiskFrameIds"]) == "null" {
			t.Fatal("empty index must serialize as []")
		}
	}
	assertIDs([]string{})
	startFrame(t, s, ctx, "risk-index-first", 0)
	assertIDs([]string{"f1"})
	risky.Store(false)
	zero := int64(0)
	job, err := s.Enqueue(ctx, "frame:f2", "risk-index-safe", []string{"f2"}, &zero)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.execute(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	assertIDs([]string{"f1"}) // Safe pending candidates do not make an image high-risk.
	snap, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	oldRun := snap.CurrentRun.ID
	snap, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: snap.Candidates[0].ID, Decision: "rejected"}}})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs([]string{})
	startFrame(t, s, ctx, "risk-index-rerun", snap.Revision)
	// Durable historical risks must not re-enter the current image index.
	if _, err = s.Store.DB.ExecContext(ctx, `UPDATE ai_candidates SET review_state='pending' WHERE run_id=$1`, oldRun); err != nil {
		t.Fatal(err)
	}
	assertIDs([]string{})
}

func TestPostgreSQLRiskImageIndexIssuesFailureAndScope(t *testing.T) {
	var failed atomic.Bool
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if failed.Load() {
			w.WriteHeader(503)
			return
		}
		result := pgResult()
		result.Detections = []Detection{}
		json.NewEncoder(w).Encode(result)
	})
	startFrame(t, s, ctx, "risk-index-empty", 0)
	snap, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := s.Stats(ctx, "v", "b")
	if err != nil || !reflect.DeepEqual(stats.UnresolvedRiskFrameIDs, []string{"f1"}) {
		t.Fatal(stats, err)
	}
	snap, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, IssueResolutions: []IssueResolution{{ID: snap.Issues[0].ID, Resolution: "manual_checked"}}})
	if err != nil {
		t.Fatal(err)
	}
	stats, err = s.Stats(ctx, "v", "b")
	if err != nil || len(stats.UnresolvedRiskFrameIDs) != 0 {
		t.Fatal(stats, err)
	}
	failed.Store(true)
	startFrame(t, s, ctx, "risk-index-failed", snap.Revision)
	stats, err = s.Stats(ctx, "v", "b")
	if err != nil || !reflect.DeepEqual(stats.UnresolvedRiskFrameIDs, []string{"f1"}) || stats.FailedImages != 1 {
		t.Fatal(stats, err)
	}
	// Move the second image into another batch while leaving the risk image in b.
	otherJob := domain.Job{ID: "other-extraction", Kind: "extraction", VideoID: "v", BatchID: "other-batch", Status: "running", CreatedAt: domain.Now()}
	if err = s.Store.AddJob(otherJob); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.PublishBatch(otherJob, []domain.Frame{}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Store.DB.ExecContext(ctx, `UPDATE frames SET batch_id='other-batch' WHERE id='f2'`); err != nil {
		t.Fatal(err)
	}
	stats, err = s.Stats(ctx, "v", "other-batch")
	if err != nil || stats.UnresolvedRiskFrameIDs == nil || len(stats.UnresolvedRiskFrameIDs) != 0 {
		t.Fatal(stats, err)
	}
	stats, err = s.Stats(ctx, "v", "")
	if err != nil || !reflect.DeepEqual(stats.UnresolvedRiskFrameIDs, []string{"f1"}) {
		t.Fatal(stats, err)
	}
	if err = s.Store.AddVideo(domain.Video{ID: "other-video", Name: "Other synthetic", Width: 100, Height: 80, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/other"}); err != nil {
		t.Fatal(err)
	}
	stats, err = s.Stats(ctx, "other-video", "")
	if err != nil || stats.UnresolvedRiskFrameIDs == nil || len(stats.UnresolvedRiskFrameIDs) != 0 {
		t.Fatal("risk leaked across video scope", stats, err)
	}
	if _, err = s.Stats(ctx, "other-video", "b"); err == nil {
		t.Fatal("batch from another video accepted")
	}
	if _, err = s.Stats(ctx, "missing-video", ""); err == nil {
		t.Fatal("missing video accepted")
	}
	if _, err = s.Stats(ctx, "v", "missing-batch"); err == nil {
		t.Fatal("out-of-scope batch accepted")
	}
}

func TestPostgreSQLRiskImageIndexIgnoresLatestStaleAttempt(t *testing.T) {
	s, ctx := pgFixture(t, nil)
	startFrame(t, s, ctx, "risk-index-active", 0)
	// A newer stale attempt has risks, but it was never published to the frame.
	if _, err := s.Store.DB.ExecContext(ctx, `INSERT INTO jobs(id,kind,video_id,status,created_at) VALUES('stale-job','preannotation','v','completed',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.DB.ExecContext(ctx, `INSERT INTO ai_runs(id,request_id,job_id,frame_id,base_revision,input_sha256,model_sha256,model_id,pipeline_version,risk_policy_version,options,status,actor_id,created_at)
        SELECT 'stale-run',request_id,'stale-job',frame_id,base_revision,input_sha256,model_sha256,model_id,pipeline_version,risk_policy_version,options,'stale',actor_id,created_at+interval '1 second' FROM ai_runs WHERE frame_id='f1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.DB.ExecContext(ctx, `INSERT INTO ai_candidates(id,run_id,frame_id,x,y,width,height,confidence,risk_reasons) VALUES('stale-candidate','stale-run','f1',1,2,20,20,.3,'["LOW_CONFIDENCE"]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.DB.ExecContext(ctx, `INSERT INTO frame_review_issues(id,run_id,frame_id,reason_code) VALUES('stale-issue','stale-run','f1','NO_DETECTIONS')`); err != nil {
		t.Fatal(err)
	}
	stats, err := s.Stats(ctx, "v", "b")
	if err != nil || stats.UnresolvedRiskFrameIDs == nil || len(stats.UnresolvedRiskFrameIDs) != 0 {
		t.Fatal("stale attempt entered active index", stats, err)
	}
}
