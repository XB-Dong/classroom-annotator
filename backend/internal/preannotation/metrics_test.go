package preannotation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func metricValue(t *testing.T, registry *prometheus.Registry, name, label string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if label != "" && (len(metric.Label) != 1 || metric.Label[0].GetValue() != label) {
				continue
			}
			if metric.Counter != nil {
				return metric.Counter.GetValue()
			}
			if metric.Gauge != nil {
				return metric.Gauge.GetValue()
			}
			if metric.Histogram != nil {
				return float64(metric.Histogram.GetSampleCount())
			}
		}
	}
	return 0
}

func TestPipelineMetricsObserveBusyRetryAndActualInferenceAttempts(t *testing.T) {
	for _, alwaysBusy := range []bool{false, true} {
		t.Run(map[bool]string{false: "recovers", true: "bounded"}[alwaysBusy], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if n == 1 || alwaysBusy {
					w.WriteHeader(429)
					return
				}
				json.NewEncoder(w).Encode(pgResult())
			}))
			defer server.Close()
			s := NewService(nil, Client{URL: server.URL, Token: "synthetic-token"})
			registry := prometheus.NewRegistry()
			if err := s.SetMetrics(registry); err != nil {
				t.Fatal(err)
			}
			_, err := s.detect(context.Background(), []byte("synthetic-jpeg"), 100, 80, "synthetic-request")
			if alwaysBusy && !errors.Is(err, ErrBusy) {
				t.Fatal(err)
			}
			if !alwaysBusy && err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 {
				t.Fatalf("attempts=%d", calls.Load())
			}
			busy := float64(1)
			if alwaysBusy {
				busy = 2
			}
			if got := metricValue(t, registry, "classroom_ai_requests_total", "busy"); got != busy {
				t.Fatalf("busy=%v", got)
			}
			if got := metricValue(t, registry, "classroom_ai_inference_duration_seconds", ""); got != 2 {
				t.Fatalf("observed attempts=%v", got)
			}
			if !alwaysBusy && metricValue(t, registry, "classroom_ai_requests_total", "success") != 1 {
				t.Fatal("success missing")
			}
		})
	}
}

func TestPipelineBusyRetryStopsAtContextDeadline(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(429) }))
	defer server.Close()
	s := NewService(nil, Client{URL: server.URL, Token: "synthetic-token"})
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := s.detect(ctx, []byte("synthetic-jpeg"), 100, 80, "request")
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("retry passed deadline: calls=%d err=%v", calls.Load(), err)
	}
	if metricValue(t, registry, "classroom_ai_inference_duration_seconds", "") != 1 {
		t.Fatal("backoff counted as inference")
	}
}

func TestPipelineNilMetricsAndInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"malformed":true}`)) }))
	defer server.Close()
	s := NewService(nil, Client{URL: server.URL, Token: "synthetic-token"})
	if err := s.SetMetrics(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.detect(context.Background(), []byte("synthetic-jpeg"), 100, 80, "request"); !errors.Is(err, ErrInvalidDetection) {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	s.detect(context.Background(), []byte("synthetic-jpeg"), 100, 80, "request")
	if metricValue(t, registry, "classroom_ai_requests_total", "invalid") != 1 {
		t.Fatal("invalid response not observed")
	}
}

func TestPipelineMetricsClassifyTimeoutAndDoNotStartExpiredWork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(503)
	}))
	defer server.Close()
	s := NewService(nil, Client{URL: server.URL, Token: "synthetic-token"})
	registry := prometheus.NewRegistry()
	if err := s.SetMetrics(registry); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	s.detect(ctx, []byte("synthetic-jpeg"), 100, 80, "request")
	if metricValue(t, registry, "classroom_ai_requests_total", "timeout") != 1 {
		t.Fatal("transport timeout misclassified")
	}
	s.detect(ctx, []byte("synthetic-jpeg"), 100, 80, "expired-request")
	if calls.Load() != 1 || metricValue(t, registry, "classroom_ai_inference_duration_seconds", "") != 1 {
		t.Fatal("expired work was started or observed")
	}
}
