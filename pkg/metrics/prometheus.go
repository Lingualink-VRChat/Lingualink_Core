package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lingualink_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lingualink_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	llmRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lingualink_llm_request_duration_seconds",
			Help:    "LLM request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend", "model"},
	)

	audioProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "lingualink_audio_processing_seconds",
			Help:    "Audio processing duration",
			Buckets: prometheus.DefBuckets,
		},
	)
)

var prometheusRegisterOnce sync.Once

// RegisterPrometheus registers all Prometheus metrics in this package with the default registry.
// It is safe to call multiple times.
func RegisterPrometheus() {
	prometheusRegisterOnce.Do(func() {
		prometheus.MustRegister(
			httpRequestsTotal,
			httpRequestDuration,
			llmRequestDuration,
			audioProcessingDuration,
			translationsTotal,
			transcriptionsTotal,
			languagePairUsage,
			jsonParseSuccessRate,
		)
	})
}

// PrometheusHandler returns an HTTP handler that serves the Prometheus metrics endpoint.
func PrometheusHandler() http.Handler {
	RegisterPrometheus()
	return promhttp.Handler()
}

// ObserveHTTPRequest records a single HTTP request with labels.
func ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	if method == "" {
		method = "UNKNOWN"
	}
	if path == "" {
		path = "unknown"
	}
	statusLabel := statusCodeToGroup(status)

	httpRequestsTotal.WithLabelValues(method, path, statusLabel).Inc()
	httpRequestDuration.WithLabelValues(method, path, statusLabel).Observe(duration.Seconds())
}

// ObserveLLMRequestDuration records the duration of a single LLM request.
func ObserveLLMRequestDuration(backend, model string, duration time.Duration) {
	if backend == "" {
		backend = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	llmRequestDuration.WithLabelValues(backend, model).Observe(duration.Seconds())
}

// ObserveAudioProcessingDuration records the overall processing duration of a single audio request.
func ObserveAudioProcessingDuration(duration time.Duration) {
	if duration <= 0 {
		return
	}
	audioProcessingDuration.Observe(duration.Seconds())
}

func statusCodeToGroup(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
