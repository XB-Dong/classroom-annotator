package preannotation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
	"math"
	"strings"
)

var ErrInvalidReview = errors.New("invalid review request")
var ErrActive = errors.New("preannotation already active or pending")
var ErrIdempotency = errors.New("idempotency payload conflict")
var ErrImage = errors.New("frame image unavailable or changed")
var ErrQueueFull = errors.New("AI queue full")
var ErrQueueTimeout = errors.New("AI queue deadline exceeded")

func (s *Service) Get(ctx context.Context, frame string) (Snapshot, error) {
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, err := s.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	out, err := readReview(ctx, tx, frame)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func readReview(ctx context.Context, tx *sql.Tx, frame string) (Snapshot, error) {
	var err error
	out := Snapshot{FrameID: frame, Annotations: []domain.Annotation{}, Candidates: []Candidate{}, Issues: []Issue{}}
	var run, latest sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT revision,status,active_ai_run_id,(SELECT r.id FROM ai_runs r WHERE r.frame_id=f.id ORDER BY r.created_at DESC,r.id DESC LIMIT 1) FROM frames f WHERE f.id=$1`, frame).Scan(&out.Revision, &out.Status, &run, &latest); err != nil {
		return out, err
	}
	if out.Annotations, err = annotations.ReadAnnotations(ctx, tx, frame); err != nil {
		return out, err
	}
	// The latest attempt explains queued/running/stale/cancelled outcomes. Formal
	// candidate evidence and gates continue to use the last CAS-published run.
	if latest.Valid {
		r := Run{}
		if err = tx.QueryRowContext(ctx, `SELECT id,status,model_id,model_sha256,pipeline_version,base_revision FROM ai_runs WHERE id=$1`, latest.String).Scan(&r.ID, &r.Status, &r.ModelID, &r.ModelSHA256, &r.PipelineVersion, &r.BaseRevision); err != nil {
			return out, err
		}
		out.CurrentRun = &r
	}
	if run.Valid {
		out.ActiveRunID = &run.String
		rows, e := tx.QueryContext(ctx, `SELECT id,run_id,x,y,width,height,confidence,review_state,risk_reasons,annotation_id FROM ai_candidates WHERE run_id=$1 ORDER BY id`, run.String)
		if e != nil {
			return out, e
		}
		for rows.Next() {
			var c Candidate
			var raw []byte
			var aid sql.NullString
			if err = rows.Scan(&c.ID, &c.RunID, &c.BBox[0], &c.BBox[1], &c.BBox[2], &c.BBox[3], &c.Confidence, &c.ReviewState, &raw, &aid); err != nil {
				rows.Close()
				return out, err
			}
			if err = json.Unmarshal(raw, &c.RiskReasons); err != nil {
				rows.Close()
				return out, err
			}
			if aid.Valid {
				c.AnnotationID = &aid.String
			}
			out.Candidates = append(out.Candidates, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
		rows, err = tx.QueryContext(ctx, `SELECT id,reason_code,resolved FROM frame_review_issues WHERE run_id=$1 ORDER BY id`, run.String)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var i Issue
			if err = rows.Scan(&i.ID, &i.ReasonCode, &i.Resolved); err != nil {
				rows.Close()
				return out, err
			}
			out.Issues = append(out.Issues, i)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func ValidateReview(w ReviewWrite, width, height int) error {
	if w.ExpectedRevision < 0 || w.ExpectedRevision == math.MaxInt64 || len(w.Decisions) > MaxCandidates || len(w.IssueResolutions) > 2 || (len(w.Decisions) == 0 && len(w.IssueResolutions) == 0) {
		return ErrInvalidReview
	}
	seen := map[string]bool{}
	for _, d := range w.Decisions {
		if strings.TrimSpace(d.CandidateID) == "" || len(d.CandidateID) > 128 || seen[d.CandidateID] || len(d.ReasonCode) > 128 {
			return ErrInvalidReview
		}
		seen[d.CandidateID] = true
		switch d.Decision {
		case "accepted":
			if d.BBox != nil {
				return ErrInvalidReview
			}
		case "adjusted":
			if d.BBox == nil || !ValidBBox(*d.BBox, width, height) {
				return ErrInvalidReview
			}
		case "rejected":
			if d.BBox != nil {
				return ErrInvalidReview
			}
		default:
			return ErrInvalidReview
		}
	}
	for _, i := range w.IssueResolutions {
		if i.ID == "" || len(i.ID) > 128 || i.Resolution != "manual_checked" || seen[i.ID] {
			return ErrInvalidReview
		}
		seen[i.ID] = true
	}
	return nil
}

func (s *Service) Review(ctx context.Context, frame string, w ReviewWrite) (Snapshot, error) {
	actor, ok := auth.ActorFrom(ctx)
	if !ok {
		return Snapshot{}, auth.ErrUnauthenticated
	}
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	f, err := store.ScanFrame(tx.QueryRowContext(ctx, `SELECT `+store.FrameColumns+` FROM frames f JOIN videos v ON v.id=f.video_id WHERE f.id=$1 AND v.workspace_id=$2 FOR UPDATE OF f`, frame, actor.WorkspaceID))
	if err != nil {
		return Snapshot{}, err
	}
	if f.Revision != w.ExpectedRevision {
		return Snapshot{}, annotations.ErrConflict
	}
	if err = ValidateReview(w, f.Width, f.Height); err != nil {
		return Snapshot{}, err
	}
	existing, err := annotations.ReadAnnotations(ctx, tx, frame)
	if err != nil {
		return Snapshot{}, err
	}
	for _, d := range w.Decisions {
		var c Candidate
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT c.id,c.run_id,c.x,c.y,c.width,c.height,c.risk_reasons,c.review_state FROM ai_candidates c JOIN frames f ON f.active_ai_run_id=c.run_id WHERE f.id=$1 AND c.id=$2`, frame, d.CandidateID).Scan(&c.ID, &c.RunID, &c.BBox[0], &c.BBox[1], &c.BBox[2], &c.BBox[3], &raw, &c.ReviewState)
		if err != nil {
			return Snapshot{}, err
		}
		if c.ReviewState != "pending" {
			return Snapshot{}, ErrInvalidReview
		}
		if d.Decision == "adjusted" && *d.BBox == c.BBox {
			return Snapshot{}, ErrInvalidReview
		}
		if err = json.Unmarshal(raw, &c.RiskReasons); err != nil {
			return Snapshot{}, err
		}
		if len(w.Decisions) > 1 && len(c.RiskReasons) > 0 {
			return Snapshot{}, ErrInvalidReview
		}
		overlap := false
		for _, a := range existing {
			if IoU(c.BBox, a.BBox) >= .35 {
				overlap = true
				break
			}
		}
		if overlap && d.Decision != "rejected" {
			return Snapshot{}, ErrInvalidReview
		}
		if overlap && d.ReasonCode != "ALREADY_LABELED" {
			return Snapshot{}, ErrInvalidReview
		}
		var annotation any
		if d.Decision != "rejected" {
			bbox := c.BBox
			source := "ai"
			if d.Decision == "adjusted" {
				bbox = *d.BBox
				source = "ai_corrected"
			}
			aid := domain.ID()
			annotation = aid
			if _, err = tx.ExecContext(ctx, `INSERT INTO annotations(id,frame_id,x,y,width,height,source) VALUES($1,$2,$3,$4,$5,$6,$7)`, aid, frame, bbox[0], bbox[1], bbox[2], bbox[3], source); err != nil {
				return Snapshot{}, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO annotation_provenance(annotation_id,candidate_id,run_id,frame_id) VALUES($1,$2,$3,$4)`, aid, c.ID, c.RunID, frame); err != nil {
				return Snapshot{}, err
			}
			existing = append(existing, domain.Annotation{ID: aid, BBox: bbox, Source: source, CandidateID: c.ID, RunID: c.RunID})
		}
		if _, err = tx.ExecContext(ctx, `UPDATE ai_candidates SET review_state=$1,annotation_id=$2,reason_code=$3,decided_by=$4,decided_at=now() WHERE id=$5`, d.Decision, annotation, d.ReasonCode, actor.ID, c.ID); err != nil {
			return Snapshot{}, err
		}
	}
	for _, i := range w.IssueResolutions {
		result, e := tx.ExecContext(ctx, `UPDATE frame_review_issues i SET resolved=true,resolved_by=$1,resolved_at=now() FROM frames f WHERE f.id=$2 AND i.run_id=f.active_ai_run_id AND i.id=$3 AND NOT i.resolved`, actor.ID, frame, i.ID)
		if e != nil {
			return Snapshot{}, e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return Snapshot{}, e
		}
		if n != 1 {
			return Snapshot{}, ErrInvalidReview
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE frames SET revision=revision+1,status='in_progress',updated_by=$1,updated_at=now() WHERE id=$2`, actor.ID, frame); err != nil {
		return Snapshot{}, err
	}
	if err = reviewstate.Audit(ctx, tx, frame, actor.ID, "accept", f.Revision+1); err != nil {
		return Snapshot{}, err
	}
	out, err := readReview(ctx, tx, frame)
	if err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	return out, nil
}
