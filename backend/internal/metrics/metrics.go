package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTP metrics
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests, partitioned by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Job lifecycle metrics
	JobsEnqueuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_enqueued_total",
			Help: "Total number of jobs submitted to the queue, by type.",
		},
		[]string{"job_type"},
	)

	JobsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_processed_total",
			Help: "Total number of jobs processed, by type and final status.",
		},
		[]string{"job_type", "status"},
	)

	JobProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "job_processing_duration_seconds",
			Help:    "Time spent processing a job in seconds.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"job_type"},
	)

	JobRetriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_retries_total",
			Help: "Total number of job retries.",
		},
		[]string{"job_type"},
	)

	WorkerActiveJobs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_active_jobs",
			Help: "Number of jobs currently being processed by this worker.",
		},
	)
)

// Register all metrics with the default Prometheus registry.
func Register() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		JobsEnqueuedTotal,
		JobsProcessedTotal,
		JobProcessingDuration,
		JobRetriesTotal,
		WorkerActiveJobs,
	)
}
