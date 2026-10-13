package preannotation

import (
	"context"
	"database/sql"
)

type Rates struct {
	AcceptedUnchanged    *float64 `json:"acceptedUnchanged"`
	Adjusted             *float64 `json:"adjusted"`
	Rejected             *float64 `json:"rejected"`
	SuspiciousCandidates *float64 `json:"suspiciousCandidates"`
	SuspiciousImages     *float64 `json:"suspiciousImages"`
	Reviewed             *float64 `json:"reviewed"`
}
type Stats struct {
	ValidImages              int      `json:"validImages"`
	SuspiciousImages         int      `json:"suspiciousImages"`
	FailedImages             int      `json:"failedImages"`
	Candidates               int      `json:"candidates"`
	Pending                  int      `json:"pending"`
	Accepted                 int      `json:"accepted"`
	Adjusted                 int      `json:"adjusted"`
	Rejected                 int      `json:"rejected"`
	RiskyCandidates          int      `json:"riskyCandidates"`
	UnresolvedRiskCandidates int      `json:"unresolvedRiskCandidates"`
	UnresolvedRiskImages     int      `json:"unresolvedRiskImages"`
	UnresolvedRiskFrameIDs   []string `json:"unresolvedRiskFrameIds"`
	ManualAdded              int      `json:"manualAdded"`
	Rates                    Rates    `json:"rates"`
}

func fraction(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	f := float64(n) / float64(d)
	return &f
}
func (s *Service) Stats(ctx context.Context, video, batch string) (Stats, error) {
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	tx, e := s.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return Stats{}, e
	}
	defer tx.Rollback()
	out := Stats{UnresolvedRiskFrameIDs: []string{}}
	var id string
	if e = tx.QueryRowContext(ctx, `SELECT id FROM videos WHERE id=$1`, video).Scan(&id); e != nil {
		return out, e
	}
	if batch != "" {
		if e = tx.QueryRowContext(ctx, `SELECT id FROM batches WHERE id=$1 AND video_id=$2`, batch, video).Scan(&id); e != nil {
			return out, e
		}
	}
	where := ` FROM frames f JOIN ai_runs r ON r.id=f.active_ai_run_id WHERE f.video_id=$1 AND ($2='' OR f.batch_id=$2)`
	e = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE r.status='completed'),count(*) FILTER(WHERE r.status='completed' AND (EXISTS(SELECT 1 FROM ai_candidates c WHERE c.run_id=r.id AND jsonb_array_length(c.risk_reasons)>0) OR EXISTS(SELECT 1 FROM frame_review_issues i WHERE i.run_id=r.id AND i.reason_code='NO_DETECTIONS'))),count(*) FILTER(WHERE r.status='failed'),count(*) FILTER(WHERE EXISTS(SELECT 1 FROM frame_review_issues i WHERE i.run_id=r.id AND NOT i.resolved) OR EXISTS(SELECT 1 FROM ai_candidates c WHERE c.run_id=r.id AND c.review_state='pending' AND jsonb_array_length(c.risk_reasons)>0))`+where, video, batch).Scan(&out.ValidImages, &out.SuspiciousImages, &out.FailedImages, &out.UnresolvedRiskImages)
	if e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT f.id`+where+` AND (EXISTS(SELECT 1 FROM frame_review_issues i WHERE i.run_id=r.id AND NOT i.resolved) OR EXISTS(SELECT 1 FROM ai_candidates c WHERE c.run_id=r.id AND c.review_state='pending' AND jsonb_array_length(c.risk_reasons)>0)) ORDER BY f.id`, video, batch)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var frameID string
		if e = rows.Scan(&frameID); e != nil {
			rows.Close()
			return out, e
		}
		out.UnresolvedRiskFrameIDs = append(out.UnresolvedRiskFrameIDs, frameID)
	}
	e = rows.Err()
	if closeErr := rows.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return out, e
	}
	e = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE c.review_state='pending'),count(*) FILTER(WHERE c.review_state='accepted'),count(*) FILTER(WHERE c.review_state='adjusted'),count(*) FILTER(WHERE c.review_state='rejected'),count(*) FILTER(WHERE jsonb_array_length(c.risk_reasons)>0),count(*) FILTER(WHERE jsonb_array_length(c.risk_reasons)>0 AND c.review_state='pending') FROM ai_candidates c JOIN frames f ON f.active_ai_run_id=c.run_id WHERE f.video_id=$1 AND ($2='' OR f.batch_id=$2)`, video, batch).Scan(&out.Candidates, &out.Pending, &out.Accepted, &out.Adjusted, &out.Rejected, &out.RiskyCandidates, &out.UnresolvedRiskCandidates)
	if e != nil {
		return out, e
	}
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM annotations a JOIN frames f ON f.id=a.frame_id JOIN ai_runs r ON r.id=f.active_ai_run_id WHERE f.video_id=$1 AND ($2='' OR f.batch_id=$2) AND a.source='manual' AND a.created_at>r.created_at`, video, batch).Scan(&out.ManualAdded)
	if e != nil {
		return out, e
	}
	reviewed := out.Accepted + out.Adjusted + out.Rejected
	out.Rates = Rates{fraction(out.Accepted, reviewed), fraction(out.Adjusted, reviewed), fraction(out.Rejected, reviewed), fraction(out.RiskyCandidates, out.Candidates), fraction(out.SuspiciousImages, out.ValidImages), fraction(reviewed, out.Candidates)}
	return out, tx.Commit()
}
