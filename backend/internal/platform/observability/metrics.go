package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Registry        *prometheus.Registry
	Requests        *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	Queue           prometheus.Gauge
	Jobs            *prometheus.CounterVec
	JobDuration     *prometheus.HistogramVec
}

func New() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(), Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_http_requests_total", Help: "Business HTTP requests."}, []string{"method", "path", "status"}), RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "classroom_http_request_duration_seconds", Help: "Business request duration."}, []string{"method", "path"}), Queue: prometheus.NewGauge(prometheus.GaugeOpts{Name: "classroom_job_queue_depth", Help: "Waiting jobs."}), Jobs: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "classroom_jobs_total", Help: "Finished jobs."}, []string{"kind", "status"}), JobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "classroom_job_duration_seconds", Help: "Job duration."}, []string{"kind", "status"})}
	m.Registry.MustRegister(m.Requests, m.RequestDuration, m.Queue, m.Jobs, m.JobDuration, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}
