package preannotation

import (
	"context"
	"errors"
	aimetrics "github.com/XB-Dong/classroom-annotator/backend/internal/preannotation/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

// SetMetrics configures aggregate observations. Call once before serving or Run.
// A nil registerer disables observations for unit tests and optional harnesses.
func (s *Service) SetMetrics(registry prometheus.Registerer) error {
	metrics, err := aimetrics.New(registry)
	if err != nil {
		return err
	}
	s.metrics = metrics
	return nil
}
func (s *Service) detect(ctx context.Context, image []byte, width, height int, requestID string) (DetectionResult, error) {
	result, err := s.attempt(ctx, image, width, height, requestID)
	if !errors.Is(err, ErrBusy) {
		return result, err
	}
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return DetectionResult{}, ctx.Err()
	case <-timer.C:
		if ctx.Err() != nil {
			return DetectionResult{}, ctx.Err()
		}
		return s.attempt(ctx, image, width, height, requestID)
	}
}

func (s *Service) attempt(ctx context.Context, image []byte, width, height int, requestID string) (DetectionResult, error) {
	if ctx.Err() != nil {
		return DetectionResult{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	result, err := s.Client.Detect(ctx, image, width, height, requestID)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	outcome := "success"
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		outcome = "cancelled"
	case errors.Is(err, ErrBusy):
		outcome = "busy"
	case errors.Is(err, ErrInvalidDetection):
		outcome = "invalid"
	case err != nil:
		outcome = "failure"
	}
	s.metrics.Attempt(outcome, time.Since(started))
	return result, err
}

// Both counts come from one PostgreSQL statement snapshot. QueryRow.Scan releases
// its connection before the gauge update or any wait for the next poll.
func (s *Service) refreshRisk(ctx context.Context) error {
	if s.metrics == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var candidates, images int64
	err := s.Store.DB.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM ai_candidates c JOIN frames f ON f.active_ai_run_id=c.run_id WHERE c.review_state='pending' AND jsonb_array_length(c.risk_reasons)>0),
 (SELECT count(*) FROM frames f WHERE EXISTS(SELECT 1 FROM ai_candidates c WHERE c.run_id=f.active_ai_run_id AND c.review_state='pending' AND jsonb_array_length(c.risk_reasons)>0) OR EXISTS(SELECT 1 FROM frame_review_issues i WHERE i.run_id=f.active_ai_run_id AND NOT i.resolved))`).Scan(&candidates, &images)
	if err != nil {
		return err
	}
	s.metrics.Risk(candidates, images)
	return nil
}

func (s *Service) pollRisk(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		// A failed poll preserves the last successfully read snapshot.
		_ = s.refreshRisk(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
