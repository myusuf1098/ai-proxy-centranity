package telemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus collectors for ProxyGateway Enterprise
type Metrics struct {
	registry        *prometheus.Registry
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	TokensTotal     *prometheus.CounterVec
	UpstreamErrors  *prometheus.CounterVec
}

// NewMetrics initializes and registers telemetry metrics
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pg_http_requests_total",
			Help: "Total number of HTTP requests processed by ProxyGateway",
		},
		[]string{"method", "path", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pg_request_duration_seconds",
			Help:    "Histogram of request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "model"},
	)

	tokensTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pg_tokens_total",
			Help: "Total tokens consumed by model and client key",
		},
		[]string{"key_id", "model", "type"},
	)

	upstreamErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pg_upstream_errors_total",
			Help: "Total errors returned by upstream providers",
		},
		[]string{"provider", "code"},
	)

	reg.MustRegister(requestsTotal)
	reg.MustRegister(requestDuration)
	reg.MustRegister(tokensTotal)
	reg.MustRegister(upstreamErrors)

	return &Metrics{
		registry:        reg,
		RequestsTotal:   requestsTotal,
		RequestDuration: requestDuration,
		TokensTotal:     tokensTotal,
		UpstreamErrors:  upstreamErrors,
	}
}

// Handler returns the scrapable Prometheus HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware creates telemetry HTTP middleware to record latency and request counts
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			statusStr := fmt.Sprintf("%d", rec.statusCode)

			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
			// Chat handler owns the duration observe for successful requests
			// (model-labeled); the middleware observes only failures so they keep
			// a duration sample without double-counting successes.
			if r.URL.Path != "/v1/chat/completions" || rec.statusCode >= http.StatusBadRequest {
				m.RequestDuration.WithLabelValues(r.URL.Path, "").Observe(duration)
			}
		})
	}
}
