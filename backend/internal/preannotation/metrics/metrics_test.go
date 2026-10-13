package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"testing"
	"time"
)

func TestMetricsRecordAttemptsAndReplaceCurrentRiskWithoutSensitiveLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, err := New(registry)
	if err != nil {
		t.Fatal(err)
	}
	m.Attempt("busy", 250*time.Millisecond)
	m.Attempt("success", time.Second)
	m.Attempt("frame-secret-user-model-sha", time.Second)
	m.QueueWait(3 * time.Second)
	m.Decision("accepted")
	m.Decision("adjusted")
	m.Decision("rejected")
	m.Decision("frame-secret-user-model-sha")
	m.Risk(10, 4)
	m.Risk(2, 1)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]float64{}
	for _, family := range families {
		name := family.GetName()
		for _, metric := range family.Metric {
			key := name
			for _, label := range metric.Label {
				if label.GetName() != "result" && label.GetName() != "decision" && label.GetName() != "scope" {
					t.Fatalf("unbounded label %s", label)
				}
				if strings.Contains(label.GetValue(), "secret") {
					t.Fatalf("sensitive label %s", label)
				}
				allowed := map[string]map[string]bool{
					"result":   {"success": true, "failure": true, "busy": true, "timeout": true, "cancelled": true, "invalid": true},
					"decision": {"accepted": true, "adjusted": true, "rejected": true, "unknown": true},
					"scope":    {"candidate": true, "image": true},
				}
				if !allowed[label.GetName()][label.GetValue()] {
					t.Fatalf("unbounded enum %s", label)
				}
				key += ":" + label.GetValue()
			}
			if metric.Counter != nil {
				counts[key] = metric.Counter.GetValue()
			}
			if metric.Gauge != nil {
				counts[key] = metric.Gauge.GetValue()
			}
			if metric.Histogram != nil {
				counts[key] = float64(metric.Histogram.GetSampleCount())
			}
		}
	}
	for key, want := range map[string]float64{
		"classroom_ai_requests_total:busy": 1, "classroom_ai_requests_total:success": 1, "classroom_ai_requests_total:invalid": 1,
		"classroom_ai_inference_duration_seconds": 3, "classroom_ai_queue_wait_seconds": 1,
		"classroom_ai_review_decisions_total:accepted": 1, "classroom_ai_review_decisions_total:adjusted": 1, "classroom_ai_review_decisions_total:rejected": 1, "classroom_ai_review_decisions_total:unknown": 1,
		"classroom_ai_unresolved_risk:candidate": 2, "classroom_ai_unresolved_risk:image": 1,
	} {
		if counts[key] != want {
			t.Errorf("%s=%v want %v", key, counts[key], want)
		}
	}
}

func TestMetricsFailedRegistrationRollsBackOnlyNewCollectors(t *testing.T) {
	registry := prometheus.NewRegistry()
	existing := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_ai_requests_total", Help: "Existing incompatible descriptor."}, []string{"result"})
	if err := registry.Register(existing); err != nil {
		t.Fatal(err)
	}
	existing.WithLabelValues("success").Inc()
	if _, err := New(registry); err == nil {
		t.Fatal("conflicting descriptor accepted")
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 || families[0].GetName() != "classroom_ai_requests_total" || families[0].Metric[0].Counter.GetValue() != 1 {
		t.Fatal("failed construction left partial metrics or removed previous observations")
	}
}

func TestMetricsNilAndDuplicateRegistrationAreSafe(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Attempt("success", time.Second)
	m.QueueWait(time.Second)
	m.Decision("accepted")
	m.Risk(1, 1)
	registry := prometheus.NewRegistry()
	if _, err = New(registry); err != nil {
		t.Fatal(err)
	}
	if _, err = New(registry); err == nil {
		t.Fatal("duplicate registration silently replaced live metrics")
	}
}

func TestMetricsClampNegativeObservations(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, err := New(registry)
	if err != nil {
		t.Fatal(err)
	}
	m.Attempt("timeout", -time.Second)
	m.QueueWait(-time.Second)
	m.Risk(-1, -2)
	families, _ := registry.Gather()
	for _, family := range families {
		for _, metric := range family.Metric {
			if metric.Gauge != nil && metric.Gauge.GetValue() < 0 {
				t.Fatal("negative risk")
			}
			if metric.Histogram != nil && metric.Histogram.GetSampleSum() < 0 {
				t.Fatal("negative elapsed time")
			}
		}
	}
}
