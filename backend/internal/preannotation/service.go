package preannotation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	aimetrics "github.com/XB-Dong/classroom-annotator/backend/internal/preannotation/metrics"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Service struct {
	Store   *store.Store
	Client  Client
	metrics *aimetrics.Metrics
	clock   func() time.Time
}

func NewService(s *store.Store, c Client) *Service { return &Service{Store: s, Client: c} }

type target struct {
	Frame domain.Frame
	Base  int64
}

func (s *Service) Enqueue(ctx context.Context, scope, key string, ids []string, expected *int64) (domain.Job, error) {
	actor, ok := auth.ActorFrom(ctx)
	if !ok {
		return domain.Job{}, auth.ErrUnauthenticated
	}
	if key == "" || len(key) > 128 || strings.TrimSpace(key) != key || len(ids) == 0 || len(ids) > 20 {
		return domain.Job{}, ErrInvalidReview
	}
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	targets := make([]target, 0, len(ids))
	video := ""
	for i, id := range ids {
		if id == "" || len(id) > 128 || (i > 0 && id == ids[i-1]) {
			return domain.Job{}, ErrInvalidReview
		}
		f, e := s.Store.Frame(id)
		if e != nil {
			return domain.Job{}, e
		}
		if f.StorageProvider != "local" || len(f.MediaSHA256) != 64 || f.MediaSizeBytes <= 0 {
			return domain.Job{}, ErrImage
		}
		if strings.HasPrefix(scope, "batch:") && f.BatchID != strings.TrimPrefix(scope, "batch:") {
			return domain.Job{}, ErrInvalidReview
		}
		if video != "" && video != f.VideoID {
			return domain.Job{}, ErrInvalidReview
		}
		video = f.VideoID
		base := f.Revision
		if expected != nil {
			base = *expected
		}
		if base < 0 || base == math.MaxInt64 {
			return domain.Job{}, ErrInvalidReview
		}
		targets = append(targets, target{f, base})
	}
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return domain.Job{}, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(8941202)`); e != nil {
		return domain.Job{}, e
	}
	var workspace string
	if e = tx.QueryRowContext(ctx, `SELECT workspace_id FROM videos WHERE id=$1`, video).Scan(&workspace); e != nil {
		return domain.Job{}, e
	}
	if workspace != actor.WorkspaceID {
		return domain.Job{}, auth.ErrUnauthenticated
	}
	var oldJob, oldFingerprint string
	e = tx.QueryRowContext(ctx, `SELECT job_id,fingerprint FROM ai_requests WHERE workspace_id=$1 AND scope=$2 AND request_key=$3`, workspace, scope, key).Scan(&oldJob, &oldFingerprint)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return domain.Job{}, e
	}
	if oldJob != "" && expected == nil {
		for i := range targets {
			if e = tx.QueryRowContext(ctx, `SELECT base_revision FROM ai_runs WHERE job_id=$1 AND frame_id=$2`, oldJob, targets[i].Frame.ID).Scan(&targets[i].Base); e != nil {
				return domain.Job{}, ErrIdempotency
			}
		}
	}
	fingerprint := requestFingerprint(targets)
	if oldJob != "" {
		if fingerprint != oldFingerprint {
			return domain.Job{}, ErrIdempotency
		}
		if e = tx.Commit(); e != nil {
			return domain.Job{}, e
		}
		return s.Store.Job(oldJob)
	}
	var queued int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM jobs WHERE kind='preannotation' AND status IN ('queued','running','cancelling')`).Scan(&queued); e != nil {
		return domain.Job{}, e
	}
	if queued >= 20 {
		return domain.Job{}, ErrQueueFull
	}
	for _, t := range targets {
		var revision int64
		var pending bool
		if e = tx.QueryRowContext(ctx, `SELECT revision FROM frames WHERE id=$1 FOR UPDATE`, t.Frame.ID).Scan(&revision); e != nil {
			return domain.Job{}, e
		}
		if revision != t.Base {
			return domain.Job{}, annotations.ErrConflict
		}
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ai_runs WHERE frame_id=$1 AND status IN ('queued','running')) OR EXISTS(SELECT 1 FROM ai_candidates c JOIN frames f ON f.active_ai_run_id=c.run_id WHERE f.id=$1 AND c.review_state='pending')`, t.Frame.ID).Scan(&pending); e != nil {
			return domain.Job{}, e
		}
		if pending {
			return domain.Job{}, ErrActive
		}
	}
	j := domain.Job{ID: domain.ID(), Kind: "preannotation", VideoID: video, Status: "queued", CreatedAt: domain.Now()}
	request := domain.ID()
	if _, e = tx.ExecContext(ctx, `INSERT INTO jobs(id,kind,video_id,status,created_at) VALUES($1,'preannotation',$2,'queued',$3)`, j.ID, video, j.CreatedAt); e != nil {
		return domain.Job{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO ai_requests(id,workspace_id,scope,request_key,fingerprint,job_id,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, request, workspace, scope, key, fingerprint, j.ID, actor.ID); e != nil {
		return domain.Job{}, enqueueError(e)
	}
	for _, t := range targets {
		if _, e = tx.ExecContext(ctx, `INSERT INTO ai_runs(id,request_id,job_id,frame_id,base_revision,input_sha256,model_sha256,pipeline_version,risk_policy_version,options,status,actor_id,model_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'{"confidenceThreshold":0.25,"nmsThreshold":0.5}','queued',$10,$11)`, domain.ID(), request, j.ID, t.Frame.ID, t.Base, t.Frame.MediaSHA256, ModelSHA256, PipelineVersion, RiskPolicyVersion, actor.ID, ModelID); e != nil {
			return domain.Job{}, enqueueError(e)
		}
	}
	if e = tx.Commit(); e != nil {
		return domain.Job{}, enqueueError(e)
	}
	return j, nil
}
func enqueueError(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" {
		return ErrActive
	}
	return e
}
func requestFingerprint(targets []target) string {
	type input struct {
		ID, SHA  string
		Revision int64
	}
	items := []input{}
	for _, t := range targets {
		items = append(items, input{t.Frame.ID, t.Frame.MediaSHA256, t.Base})
	}
	raw, _ := json.Marshal(struct {
		Frames                []input
		Model, Pipeline, Risk string
		Confidence, NMS       float64
	}{items, ModelSHA256, PipelineVersion, RiskPolicyVersion, .25, .5})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Run is a single sequential AI scheduler. The caller must hold the application's
// PostgreSQL scheduler advisory lock for its entire lifetime.
func (s *Service) Run(ctx context.Context) error {
	if s.metrics != nil {
		metricsCtx, cancelMetrics := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); s.pollRisk(metricsCtx) }()
		defer func() { cancelMetrics(); <-done }()
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, e := s.Store.DB.ExecContext(ctx, `WITH terminal AS (UPDATE jobs j SET finished_at=clock_timestamp() WHERE kind='preannotation' AND status='cancelled' AND finished_at IS NULL AND EXISTS(SELECT 1 FROM ai_runs r WHERE r.job_id=j.id AND r.status IN ('queued','running')) RETURNING id) UPDATE ai_runs r SET status='cancelled',finished_at=now() FROM terminal j WHERE j.id=r.job_id AND r.status IN ('queued','running')`); e != nil {
				return e
			}
			var job string
			e := s.Store.DB.QueryRowContext(ctx, `SELECT j.id FROM jobs j JOIN ai_requests q ON q.job_id=j.id WHERE j.status='queued' ORDER BY q.created_at,q.id LIMIT 1`).Scan(&job)
			if errors.Is(e, sql.ErrNoRows) {
				continue
			}
			if e != nil {
				return e
			}
			if e = s.execute(ctx, job); e != nil {
				return e
			}
		}
	}
}
func (s *Service) Recover(ctx context.Context) error {
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `UPDATE jobs j SET status=CASE WHEN status IN ('cancelling','cancelled') THEN 'cancelled' ELSE 'failed' END,error=CASE WHEN status IN ('cancelling','cancelled') THEN '' ELSE 'INTERRUPTED' END,finished_at=clock_timestamp() WHERE kind='preannotation' AND (status IN ('queued','running','cancelling') OR (status='cancelled' AND EXISTS(SELECT 1 FROM ai_runs r WHERE r.job_id=j.id AND r.status IN ('queued','running'))))`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE ai_runs r SET status=CASE WHEN j.status='cancelled' THEN 'cancelled' ELSE 'failed' END,error_code=CASE WHEN j.status='cancelled' THEN '' ELSE 'INTERRUPTED' END,finished_at=now() FROM jobs j WHERE j.id=r.job_id AND r.status IN ('queued','running')`); e != nil {
		return e
	}
	return tx.Commit()
}

type execution struct {
	ID, Frame, Actor, InputSHA string
	Base                       int64
}

func (s *Service) execute(parent context.Context, job string) error {
	tx, e := s.Store.DB.BeginTx(parent, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var status string
	var created time.Time
	if e = tx.QueryRowContext(parent, `SELECT j.status,q.created_at FROM jobs j JOIN ai_requests q ON q.job_id=j.id WHERE j.id=$1 FOR UPDATE OF j`, job).Scan(&status, &created); e != nil {
		return e
	}
	if status != "queued" {
		return nil
	}
	queueWait := s.now().Sub(created)
	queueExpired := queueWait >= 120*time.Second
	if _, e = tx.ExecContext(parent, `UPDATE jobs SET status='running',started_at=clock_timestamp() WHERE id=$1`, job); e != nil {
		return e
	}
	if _, e = tx.ExecContext(parent, `UPDATE ai_runs SET status='running' WHERE job_id=$1 AND status='queued'`, job); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.metrics.QueueWait(queueWait)
	started := s.now()
	rows, e := s.Store.DB.QueryContext(parent, `SELECT id,frame_id,actor_id,input_sha256,base_revision FROM ai_runs WHERE job_id=$1 ORDER BY frame_id`, job)
	if e != nil {
		return e
	}
	runs := []execution{}
	for rows.Next() {
		var r execution
		if e = rows.Scan(&r.ID, &r.Frame, &r.Actor, &r.InputSHA, &r.Base); e != nil {
			rows.Close()
			return e
		}
		runs = append(runs, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(parent, s.remaining(started))
	defer cancel()
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				j, err := s.Store.JobWithContext(ctx, job)
				if err != nil || j.Status != "running" {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-watchDone }()
	failed := false
	for _, r := range runs {
		var result DetectionResult
		f, err := s.Store.Frame(r.Frame)
		if queueExpired {
			err = ErrQueueTimeout
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if !queueExpired && s.remaining(started) == 0 {
			err = context.DeadlineExceeded
			cancel()
		}
		if err == nil {
			var image []byte
			image, err = s.readImage(f, r.InputSHA)
			if err == nil {
				result, err = s.detect(ctx, image, f.Width, f.Height, r.ID)
			}
		}
		if err != nil {
			failed = true
		}
		if e = s.publish(parent, job, r, f, result, err); e != nil {
			return e
		}
	}
	tx, e = s.Store.DB.BeginTx(parent, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(parent, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, job).Scan(&status); e != nil {
		return e
	}
	final, code := "completed", ""
	if failed {
		final, code = "failed", "AI_FAILED"
	}
	if queueExpired {
		final, code = "failed", "QUEUE_TIMEOUT"
	}
	if status == "cancelling" || status == "cancelled" {
		final, code = "cancelled", ""
	}
	if _, e = tx.ExecContext(parent, `UPDATE jobs SET status=$1,error=$2,progress=1,finished_at=clock_timestamp() WHERE id=$3`, final, code, job); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Service) readImage(f domain.Frame, want string) ([]byte, error) {
	if f.StorageProvider != "local" || f.MediaSHA256 != want || f.MediaSizeBytes <= 0 || f.MediaSizeBytes > 15*1024*1024 {
		return nil, ErrImage
	}
	root, e := filepath.Abs(s.Store.Root)
	if e != nil {
		return nil, ErrImage
	}
	path := filepath.Join(root, f.StorageKey)
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		return nil, ErrImage
	}
	relative, e := filepath.Rel(root, resolved)
	if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, ErrImage
	}
	file, e := os.Open(resolved)
	if e != nil {
		return nil, ErrImage
	}
	defer file.Close()
	raw, e := io.ReadAll(io.LimitReader(file, 15*1024*1024+1))
	if e != nil || int64(len(raw)) != f.MediaSizeBytes {
		return nil, ErrImage
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != want {
		return nil, ErrImage
	}
	return raw, nil
}

func (s *Service) publish(ctx context.Context, job string, r execution, f domain.Frame, result DetectionResult, computation error) error {
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var jobStatus string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=$1 FOR UPDATE`, job).Scan(&jobStatus); e != nil {
		return e
	}
	var revision int64
	if e = tx.QueryRowContext(ctx, `SELECT revision FROM frames WHERE id=$1 FOR UPDATE`, r.Frame).Scan(&revision); e != nil {
		return e
	}
	state, code := "completed", ""
	if computation != nil {
		state, code = "failed", "AI_FAILED"
	}
	if errors.Is(computation, ErrQueueTimeout) {
		code = "QUEUE_TIMEOUT"
	} else if errors.Is(computation, context.DeadlineExceeded) {
		code = "TIMEOUT"
	}
	if revision != r.Base && computation == nil {
		state, code = "stale", ""
	}
	if jobStatus != "running" {
		state, code = "cancelled", ""
	}
	changed, e := tx.ExecContext(ctx, `UPDATE ai_runs SET status=$1,error_code=$2,model_id=coalesce(nullif($3,''),model_id),finished_at=now() WHERE id=$4 AND status='running'`, state, code, result.ModelID, r.ID)
	if e != nil {
		return e
	}
	count, e := changed.RowsAffected()
	if e != nil {
		return e
	}
	if count != 1 {
		return store.ErrState
	}
	if state == "stale" || state == "cancelled" || revision != r.Base {
		return tx.Commit()
	}
	if state == "completed" {
		existing, err := annotations.ReadAnnotations(ctx, tx, r.Frame)
		if err != nil {
			return err
		}
		risks := RiskReasons(result.Detections, existing)
		for i, d := range result.Detections {
			raw, _ := json.Marshal(risks[i])
			if _, e = tx.ExecContext(ctx, `INSERT INTO ai_candidates(id,run_id,frame_id,x,y,width,height,confidence,risk_reasons) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, domain.ID(), r.ID, r.Frame, d.BBox[0], d.BBox[1], d.BBox[2], d.BBox[3], d.Confidence, string(raw)); e != nil {
				return e
			}
		}
	}
	issue := ""
	if state == "failed" {
		issue = "AI_FAILED"
	} else if len(result.Detections) == 0 {
		issue = "NO_DETECTIONS"
	}
	if issue != "" {
		if _, e = tx.ExecContext(ctx, `INSERT INTO frame_review_issues(id,run_id,frame_id,reason_code) VALUES($1,$2,$3,$4)`, domain.ID(), r.ID, r.Frame, issue); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE frames SET active_ai_run_id=$1,revision=revision+1,status='in_progress',updated_by=$2,updated_at=now() WHERE id=$3`, r.ID, r.Actor, r.Frame); e != nil {
		return e
	}
	if e = reviewstate.Audit(ctx, tx, r.Frame, r.Actor, "ai_publish", revision+1); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE jobs SET progress=(SELECT count(*) FILTER(WHERE status NOT IN ('queued','running'))::float8/count(*) FROM ai_runs WHERE job_id=$1) WHERE id=$1`, job); e != nil {
		return e
	}
	return tx.Commit()
}
