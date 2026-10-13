package preannotation

import (
	"testing"
	"time"
)

func TestExecutionBudgetStopsExactlyAtHardDeadline(t *testing.T) {
	started := time.Unix(1000, 0)
	now := started
	s := NewService(nil, Client{})
	s.clock = func() time.Time { return now }
	for _, item := range []struct{ elapsed, remaining time.Duration }{
		{0, 120 * time.Second}, {119 * time.Second, time.Second}, {120 * time.Second, 0}, {121 * time.Second, 0},
	} {
		now = started.Add(item.elapsed)
		if got := s.remaining(started); got != item.remaining {
			t.Errorf("at %v remaining=%v want %v", item.elapsed, got, item.remaining)
		}
	}
}
