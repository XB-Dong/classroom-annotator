// Package reviewstate holds shared transactional review invariants without API dependencies.
package reviewstate

import (
	"context"
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

var ErrUnresolved = errors.New("unresolved review work")
var ErrProvenance = errors.New("invalid annotation provenance")

func Gate(ctx context.Context, tx *sql.Tx, frame string) error {
	var blocked bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ai_candidates c JOIN frames f ON f.active_ai_run_id=c.run_id WHERE f.id=$1 AND c.review_state='pending') OR EXISTS(SELECT 1 FROM frame_review_issues i JOIN frames f ON f.active_ai_run_id=i.run_id WHERE f.id=$1 AND NOT i.resolved)`, frame).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrUnresolved
	}
	return nil
}

// Sync validates immutable references and synchronizes deletion/undo/geometry decisions.
// The caller holds the frame lock and writes the formal snapshot in this transaction.
func Sync(ctx context.Context, tx *sql.Tx, frame, actor string, anns []domain.Annotation) error {
	keep := map[string]bool{}
	for index, a := range anns {
		keep[a.ID] = true
		var candidate, run, owner string
		var original [4]float64
		err := tx.QueryRowContext(ctx, `SELECT p.candidate_id,p.run_id,p.frame_id,c.x,c.y,c.width,c.height FROM annotation_provenance p JOIN ai_candidates c ON c.id=p.candidate_id WHERE p.annotation_id=$1`, a.ID).Scan(&candidate, &run, &owner, &original[0], &original[1], &original[2], &original[3])
		if errors.Is(err, sql.ErrNoRows) {
			if a.Source != "manual" || a.CandidateID != "" || a.RunID != "" {
				return ErrProvenance
			}
			continue
		}
		if err != nil {
			return err
		}
		if owner != frame || a.CandidateID != candidate || a.RunID != run || a.Source == "manual" {
			return ErrProvenance
		}
		state, source := "accepted", "ai"
		if a.BBox != original {
			state, source = "adjusted", "ai_corrected"
		}
		anns[index].Source = source
		if _, err = tx.ExecContext(ctx, `UPDATE ai_candidates SET review_state=$1,reason_code='',decided_by=nullif($2,''),decided_at=now() WHERE id=$3`, state, actor, candidate); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE annotation_provenance SET deleted=false,updated_at=now() WHERE annotation_id=$1`, a.ID); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT annotation_id,candidate_id FROM annotation_provenance WHERE frame_id=$1 AND NOT deleted`, frame)
	if err != nil {
		return err
	}
	type deleted struct{ annotation, candidate string }
	items := []deleted{}
	for rows.Next() {
		var d deleted
		if err = rows.Scan(&d.annotation, &d.candidate); err != nil {
			rows.Close()
			return err
		}
		if !keep[d.annotation] {
			items = append(items, d)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range items {
		if _, err = tx.ExecContext(ctx, `UPDATE ai_candidates SET review_state='rejected',reason_code='DELETED',decided_by=nullif($1,''),decided_at=now() WHERE id=$2`, actor, d.candidate); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE annotation_provenance SET deleted=true,updated_at=now() WHERE annotation_id=$1`, d.annotation); err != nil {
			return err
		}
	}
	return nil
}

func Audit(ctx context.Context, tx *sql.Tx, frame, actor, action string, revision int64) error {
	if actor == "" {
		return nil
	} // Repository-only legacy fixtures have no authenticated actor.
	_, err := tx.ExecContext(ctx, `INSERT INTO annotation_events(frame_id,user_id,action,revision) VALUES($1,$2,$3,$4)`, frame, actor, action, revision)
	return err
}

// Verify checks persisted formal labels without changing the export snapshot.
func Verify(ctx context.Context, tx *sql.Tx, frame string, anns []domain.Annotation) error {
	for _, a := range anns {
		var candidate, run, owner, state string
		var deleted bool
		var original [4]float64
		err := tx.QueryRowContext(ctx, `SELECT p.candidate_id,p.run_id,p.frame_id,p.deleted,c.review_state,c.x,c.y,c.width,c.height FROM annotation_provenance p JOIN ai_candidates c ON c.id=p.candidate_id WHERE p.annotation_id=$1`, a.ID).Scan(&candidate, &run, &owner, &deleted, &state, &original[0], &original[1], &original[2], &original[3])
		if errors.Is(err, sql.ErrNoRows) {
			if a.Source != "manual" || a.CandidateID != "" || a.RunID != "" {
				return ErrProvenance
			}
			continue
		}
		if err != nil {
			return err
		}
		if owner != frame || deleted || a.CandidateID != candidate || a.RunID != run {
			return ErrProvenance
		}
		if a.BBox == original {
			if a.Source != "ai" || state != "accepted" {
				return ErrProvenance
			}
		} else if a.Source != "ai_corrected" || state != "adjusted" {
			return ErrProvenance
		}
	}
	return nil
}
