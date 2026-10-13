// Package metrics records bounded, aggregate AI workflow observations.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type Metrics struct {
	duration  prometheus.Histogram
	requests  *prometheus.CounterVec
	queue     prometheus.Histogram
	decisions *prometheus.CounterVec
	risk      *prometheus.GaugeVec
}

func New(registry prometheus.Registerer) (*Metrics, error) {
	if registry == nil {
		return nil, nil
	}
	m := &Metrics{
		duration:  prometheus.NewHistogram(prometheus.HistogramOpts{Name: "classroom_ai_inference_duration_seconds", Help: "Elapsed Go AI inference attempt duration, including transport and response validation.", Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 20, 30, 120}}),
		requests:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_ai_requests_total", Help: "Actual AI inference attempts by bounded outcome, including busy retries."}, []string{"result"}),
		queue:     prometheus.NewHistogram(prometheus.HistogramOpts{Name: "classroom_ai_queue_wait_seconds", Help: "AI job queue wait measured after durable scheduler claim.", Buckets: []float64{.1, 1, 5, 10, 30, 60, 120}}),
		decisions: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_ai_review_decisions_total", Help: "Candidate review decisions after successful PostgreSQL commit."}, []string{"decision"}),
		risk:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "classroom_ai_unresolved_risk", Help: "Current unresolved risk candidates and images from active runs only."}, []string{"scope"}),
	}
	registered := []prometheus.Collector{}
	for _, collector := range []prometheus.Collector{m.duration, m.requests, m.queue, m.decisions, m.risk} {
		if err := registry.Register(collector); err != nil {
			for _, previous := range registered {
				registry.Unregister(previous)
			}
			return nil, err
		}
		registered = append(registered, collector)
	}
	for _, result := range []string{"success", "failure", "busy", "timeout", "cancelled", "invalid"} {
		m.requests.WithLabelValues(result)
	}
	for _, decision := range []string{"accepted", "adjusted", "rejected", "unknown"} {
		m.decisions.WithLabelValues(decision)
	}
	// Risk series are created only after a successful database snapshot.
	return m, nil
}

func (m *Metrics) Attempt(result string, elapsed time.Duration) {
	if m == nil {
		return
	}
	switch result {
	case "success", "failure", "busy", "timeout", "cancelled", "invalid":
	default:
		result = "invalid"
	}
	m.requests.WithLabelValues(result).Inc()
	m.duration.Observe(seconds(elapsed))
}
func (m *Metrics) QueueWait(elapsed time.Duration) {
	if m != nil {
		m.queue.Observe(seconds(elapsed))
	}
}
func (m *Metrics) Decision(decision string) {
	if m == nil {
		return
	}
	switch decision {
	case "accepted", "adjusted", "rejected":
	default:
		decision = "unknown"
	}
	m.decisions.WithLabelValues(decision).Inc()
}
func (m *Metrics) Risk(candidates, images int64) {
	if m == nil {
		return
	}
	m.risk.WithLabelValues("candidate").Set(float64(max(candidates, 0)))
	m.risk.WithLabelValues("image").Set(float64(max(images, 0)))
}
func seconds(elapsed time.Duration) float64 { return max(elapsed.Seconds(), 0) }
