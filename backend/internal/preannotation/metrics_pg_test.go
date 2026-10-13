package preannotation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestPostgreSQLReviewMetricsCountOnlyCommittedCASWinnerAndCurrentRisk(t *testing.T) {
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		result := pgResult()
		result.Detections[0].Confidence = .3
		json.NewEncoder(w).Encode(result)
	})
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	startFrame(t, s, ctx, "metrics", 0)
	if err := s.refreshRisk(ctx); err != nil {
		t.Fatal(err)
	}
	if metricValue(t, registry, "classroom_ai_unresolved_risk", "candidate") != 1 || metricValue(t, registry, "classroom_ai_unresolved_risk", "image") != 1 {
		t.Fatal("current risk missing")
	}
	snap, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, decision := range []string{"accepted", "rejected"} {
		go func(decision string) {
			<-start
			_, err := s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: snap.Candidates[0].ID, Decision: decision}}})
			results <- err
		}(decision)
	}
	close(start)
	wins, losers := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, annotations.ErrConflict) {
			losers++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || losers != 1 {
		t.Fatalf("wins=%d conflicts=%d", wins, losers)
	}
	total := metricValue(t, registry, "classroom_ai_review_decisions_total", "accepted") + metricValue(t, registry, "classroom_ai_review_decisions_total", "rejected")
	if total != 1 {
		t.Fatalf("loser counted: %v", total)
	}
	if err = s.refreshRisk(ctx); err != nil {
		t.Fatal(err)
	}
	if metricValue(t, registry, "classroom_ai_unresolved_risk", "candidate") != 0 || metricValue(t, registry, "classroom_ai_unresolved_risk", "image") != 0 {
		t.Fatal("resolved risk accumulated")
	}
	if metricValue(t, registry, "classroom_ai_queue_wait_seconds", "") != 1 || metricValue(t, registry, "classroom_ai_requests_total", "success") != 1 {
		t.Fatal("scheduler pipeline observations missing")
	}
	// An invalid request must not count any uncommitted item.
	snap, err = s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: "missing", Decision: "accepted"}}}); err == nil {
		t.Fatal("invalid decision committed")
	}
	if metricValue(t, registry, "classroom_ai_review_decisions_total", "accepted")+metricValue(t, registry, "classroom_ai_review_decisions_total", "rejected") != 1 {
		t.Fatal("failed decision counted")
	}
}

func TestPostgreSQLReviewMetricsRollbackDoesNotCountEarlierValidDecision(t *testing.T) {
	s, ctx := pgFixture(t, nil)
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	startFrame(t, s, ctx, "rollback", 0)
	before, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: before.Revision, Decisions: []Decision{
		{CandidateID: before.Candidates[0].ID, Decision: "accepted"}, {CandidateID: "missing", Decision: "accepted"},
	}})
	if err == nil {
		t.Fatal("invalid batch committed")
	}
	after, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.Candidates[0].ReviewState != "pending" || len(after.Annotations) != 0 {
		t.Fatal("partial decision survived rollback", after)
	}
	if metricValue(t, registry, "classroom_ai_review_decisions_total", "accepted") != 0 {
		t.Fatal("rolled-back decision observed")
	}
}

func TestPostgreSQLRiskSnapshotUsesActiveIssuesAndPreservesLastReadOnFailure(t *testing.T) {
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		result := pgResult()
		result.Detections = []Detection{}
		json.NewEncoder(w).Encode(result)
	})
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	startFrame(t, s, ctx, "risk-empty", 0)
	if err := s.refreshRisk(ctx); err != nil {
		t.Fatal(err)
	}
	if metricValue(t, registry, "classroom_ai_unresolved_risk", "image") != 1 {
		t.Fatal("NO_DETECTIONS risk absent")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.refreshRisk(cancelled); err == nil {
		t.Fatal("cancelled risk query succeeded")
	}
	if metricValue(t, registry, "classroom_ai_unresolved_risk", "image") != 1 {
		t.Fatal("failed query invented zero risk")
	}
	snap, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	snap, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, IssueResolutions: []IssueResolution{{ID: snap.Issues[0].ID, Resolution: "manual_checked"}}})
	if err != nil {
		t.Fatal(err)
	}
	startFrame(t, s, ctx, "risk-new", snap.Revision)
	if err = s.refreshRisk(ctx); err != nil {
		t.Fatal(err)
	}
	if metricValue(t, registry, "classroom_ai_unresolved_risk", "image") != 1 {
		t.Fatal("historical issue counted twice")
	}
	if s.Store.DB.Stats().InUse != 0 {
		t.Fatal("risk snapshot retained a pool connection")
	}
}

func TestPostgreSQLStatsDenominatorsNullAndHistoricalRuns(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		result := pgResult()
		result.Detections = []Detection{}
		if calls.Add(1) == 1 {
			result.Detections = []Detection{
				{BBox: [4]float64{1, 2, 20, 20}, Confidence: .3, ClassID: 7},
				{BBox: [4]float64{40, 2, 20, 20}, Confidence: .8, ClassID: 7},
				{BBox: [4]float64{1, 40, 20, 20}, Confidence: .8, ClassID: 7},
				{BBox: [4]float64{40, 40, 20, 20}, Confidence: .8, ClassID: 7},
			}
		}
		json.NewEncoder(w).Encode(result)
	})
	stats, err := s.Stats(ctx, "v", "")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rates.AcceptedUnchanged != nil || stats.Rates.SuspiciousImages != nil || stats.Rates.Reviewed != nil {
		t.Fatal("zero denominator must be null", stats)
	}
	startFrame(t, s, ctx, "stats", 0)
	snap, err := s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	candidates := map[float64]Candidate{}
	for _, candidate := range snap.Candidates {
		candidates[candidate.BBox[0]+100*candidate.BBox[1]] = candidate
	}
	for _, item := range []struct {
		key      float64
		decision string
		bbox     *[4]float64
	}{
		{201, "rejected", nil}, {240, "accepted", nil}, {4001, "adjusted", &[4]float64{2, 40, 20, 20}},
	} {
		snap, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: candidates[item.key].ID, Decision: item.decision, BBox: item.bbox}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	zero := int64(0)
	job, err := s.Enqueue(ctx, "frame:f2", "empty", []string{"f2"}, &zero)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.execute(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	stats, err = s.Stats(ctx, "v", "b")
	if err != nil {
		t.Fatal(err)
	}
	if stats.ValidImages != 2 || stats.SuspiciousImages != 2 || stats.Candidates != 4 || stats.Accepted != 1 || stats.Adjusted != 1 || stats.Rejected != 1 || stats.Pending != 1 || stats.RiskyCandidates != 1 || stats.UnresolvedRiskCandidates != 0 || stats.UnresolvedRiskImages != 1 {
		t.Fatal("wrong active denominators", stats)
	}
	for name, pair := range map[string]struct {
		value *float64
		want  float64
	}{
		"accepted": {stats.Rates.AcceptedUnchanged, 1.0 / 3}, "adjusted": {stats.Rates.Adjusted, 1.0 / 3}, "rejected": {stats.Rates.Rejected, 1.0 / 3}, "risk": {stats.Rates.SuspiciousCandidates, .25}, "images": {stats.Rates.SuspiciousImages, 1}, "reviewed": {stats.Rates.Reviewed, .75},
	} {
		if pair.value == nil || *pair.value != pair.want {
			t.Fatalf("%s=%v want %v", name, pair.value, pair.want)
		}
	}
	// Finish pending evidence, then rerun with zero candidates. Historical risks
	// and decisions stay durable but no longer inflate current-run denominators.
	snap, err = s.Review(ctx, "f1", ReviewWrite{ExpectedRevision: snap.Revision, Decisions: []Decision{{CandidateID: candidates[4040].ID, Decision: "accepted"}}})
	if err != nil {
		t.Fatal(err)
	}
	startFrame(t, s, ctx, "stats-rerun", snap.Revision)
	stats, err = s.Stats(ctx, "v", "")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Candidates != 0 || stats.Rates.AcceptedUnchanged != nil || stats.Rates.SuspiciousCandidates != nil || stats.Rates.Reviewed != nil || stats.ValidImages != 2 || stats.SuspiciousImages != 2 {
		t.Fatal("history counted as active", stats)
	}
	var historical int
	if err = s.Store.DB.QueryRow(`SELECT count(*) FROM ai_candidates`).Scan(&historical); err != nil || historical != 4 {
		t.Fatal("history lost", historical, err)
	}
	snap, err = s.Get(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	startFrame(t, s, ctx, "stats-failure", snap.Revision)
	stats, err = s.Stats(ctx, "v", "")
	if err != nil {
		t.Fatal(err)
	}
	if stats.ValidImages != 1 || stats.FailedImages != 1 || stats.SuspiciousImages != 1 || stats.Rates.SuspiciousImages == nil || *stats.Rates.SuspiciousImages != 1 {
		t.Fatal("failed image entered valid denominator", stats)
	}
}

func TestPostgreSQLBatchPartialFailurePreservesSuccessfulCandidatesAndBusyOnce(t *testing.T) {
	var calls atomic.Int32
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		if n == 3 {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(pgResult())
	})
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	job, err := s.Enqueue(ctx, "batch:b", "partial", []string{"f1", "f2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.execute(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	job, err = s.Store.Job(job.ID)
	if err != nil || job.Status != "failed" {
		t.Fatal("partial batch claimed complete", job, err)
	}
	if len(job.PartialResults) != 2 || job.PartialResults[0].FrameID != "f1" || job.PartialResults[0].Status != "completed" || !job.PartialResults[0].Applied || job.PartialResults[1].FrameID != "f2" || job.PartialResults[1].Status != "failed" || job.PartialResults[1].Applied || job.PartialResults[1].ErrorCode != "AI_FAILED" {
		t.Fatal("partial results misleading", job)
	}
	started, err := time.Parse(time.RFC3339Nano, job.StartedAt)
	if err != nil {
		t.Fatal("missing actual start", job, err)
	}
	finished, err := time.Parse(time.RFC3339Nano, job.FinishedAt)
	if err != nil || finished.Before(started) {
		t.Fatal("missing actual finish", job, err)
	}
	jobs, err := s.Store.Jobs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, listed := range jobs {
		if listed.ID == job.ID {
			found = true
			if len(listed.PartialResults) != 2 || listed.StartedAt != job.StartedAt || listed.FinishedAt != job.FinishedAt {
				t.Fatal("Jobs omitted metadata", listed)
			}
		}
	}
	if !found {
		t.Fatal("job not listed")
	}
	first, err := s.Get(ctx, "f1")
	if err != nil || len(first.Candidates) != 1 || first.CurrentRun.Status != "completed" {
		t.Fatal("successful partial result lost", first, err)
	}
	second, err := s.Get(ctx, "f2")
	if err != nil || len(second.Candidates) != 0 || second.CurrentRun.Status != "failed" || len(second.Issues) != 1 {
		t.Fatal(second, err)
	}
	if calls.Load() != 3 || metricValue(t, registry, "classroom_ai_requests_total", "busy") != 1 || metricValue(t, registry, "classroom_ai_requests_total", "failure") != 1 {
		t.Fatal("busy/partial observations wrong")
	}
}

func TestPostgreSQLQueueAndExecutionDeadlinesUseControlledClock(t *testing.T) {
	for _, expiredQueue := range []bool{true, false} {
		t.Run(map[bool]string{true: "queue", false: "execution"}[expiredQueue], func(t *testing.T) {
			var offset atomic.Int64
			var calls atomic.Int32
			s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				offset.Store(int64(120 * time.Second))
				json.NewEncoder(w).Encode(pgResult())
			})
			registry := prometheus.NewRegistry()
			if err := s.SetMetrics(registry); err != nil {
				t.Fatal(err)
			}
			job, err := s.Enqueue(ctx, "batch:b", "clock", []string{"f1", "f2"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			queued, err := s.Store.Job(job.ID)
			if err != nil || queued.StartedAt != "" || queued.FinishedAt != "" || len(queued.PartialResults) != 2 || queued.PartialResults[0].Applied || queued.PartialResults[0].Status != "queued" {
				t.Fatal("queued metadata implies execution", queued, err)
			}
			var created time.Time
			if err = s.Store.DB.QueryRow(`SELECT created_at FROM ai_requests WHERE job_id=$1`, job.ID).Scan(&created); err != nil {
				t.Fatal(err)
			}
			s.clock = func() time.Time { return created.Add(time.Duration(offset.Load())) }
			if expiredQueue {
				offset.Store(int64(120 * time.Second))
			}
			if err = s.execute(context.Background(), job.ID); err != nil {
				t.Fatal(err)
			}
			job, err = s.Store.Job(job.ID)
			if err != nil || job.Status != "failed" {
				t.Fatal(job, err)
			}
			wantCalls := int32(1)
			if expiredQueue {
				wantCalls = 0
				if job.Error != "QUEUE_TIMEOUT" {
					t.Fatal(job)
				}
			}
			if calls.Load() != wantCalls {
				t.Fatalf("deadline started extra work: calls=%d", calls.Load())
			}
			started, err := time.Parse(time.RFC3339Nano, job.StartedAt)
			if err != nil {
				t.Fatal(err)
			}
			finished, err := time.Parse(time.RFC3339Nano, job.FinishedAt)
			if err != nil || finished.Before(started) || finished.Sub(started) > 5*time.Second {
				t.Fatal("clock-controlled test waited real deadline", job, err)
			}
			if len(job.PartialResults) != 2 || job.PartialResults[1].Applied || job.PartialResults[1].Status != "failed" {
				t.Fatal("deadline partial result wrong", job)
			}
			wantCode := "TIMEOUT"
			if expiredQueue {
				wantCode = "QUEUE_TIMEOUT"
			}
			if job.PartialResults[1].ErrorCode != wantCode {
				t.Fatal("deadline reason lost", job)
			}
			first, err := s.Get(ctx, "f1")
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.Get(ctx, "f2")
			if err != nil {
				t.Fatal(err)
			}
			if !expiredQueue && (len(first.Candidates) != 1 || first.CurrentRun.Status != "completed" || second.CurrentRun.Status != "failed") {
				t.Fatal("execution timeout erased partial commit", first, second)
			}
		})
	}
}

func TestPostgreSQLRecoveryFinishesQueuedCancellationWithoutStartingInference(t *testing.T) {
	var calls atomic.Int32
	s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); json.NewEncoder(w).Encode(pgResult()) })
	zero := int64(0)
	job, err := s.Enqueue(ctx, "frame:f1", "recover-cancel", []string{"f1"}, &zero)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Store.CancelJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	job, err = s.Store.Job(job.ID)
	if err != nil || calls.Load() != 0 || job.Status != "cancelled" || job.StartedAt != "" || job.FinishedAt == "" || len(job.PartialResults) != 1 || job.PartialResults[0].Status != "cancelled" || job.PartialResults[0].Applied {
		t.Fatal("queued cancel recovery metadata", job, err)
	}
	if err = s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	again, err := s.Store.Job(job.ID)
	if err != nil || again.FinishedAt != job.FinishedAt {
		t.Fatal("historical terminal timestamp rewritten", again, err)
	}
	legacy, err := s.Store.Job("extraction")
	if err != nil || legacy.StartedAt != "" || legacy.FinishedAt != "" || len(legacy.PartialResults) != 0 {
		t.Fatal("non-AI job backfilled", legacy, err)
	}
}
