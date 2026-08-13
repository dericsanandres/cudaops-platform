package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Jobs       *prometheus.CounterVec
	Inflight   prometheus.Gauge
	QueueWait  prometheus.Histogram
	Processing *prometheus.HistogramVec
	Fallback   prometheus.Counter
	Retries    prometheus.Counter
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Jobs:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cudaops_jobs_total", Help: "Jobs by terminal status and processor."}, []string{"status", "device"}),
		Inflight:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "cudaops_jobs_inflight", Help: "Jobs currently being processed."}),
		QueueWait:  prometheus.NewHistogram(prometheus.HistogramOpts{Name: "cudaops_queue_wait_seconds", Help: "Time jobs spend queued.", Buckets: prometheus.DefBuckets}),
		Processing: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "cudaops_processing_seconds", Help: "End-to-end processor latency.", Buckets: prometheus.DefBuckets}, []string{"device"}),
		Fallback:   prometheus.NewCounter(prometheus.CounterOpts{Name: "cudaops_fallback_total", Help: "Automatic CUDA-to-CPU fallbacks."}),
		Retries:    prometheus.NewCounter(prometheus.CounterOpts{Name: "cudaops_worker_retries_total", Help: "Jobs retried after transient launch failures."}),
	}
	reg.MustRegister(m.Jobs, m.Inflight, m.QueueWait, m.Processing, m.Fallback, m.Retries)
	return m
}
