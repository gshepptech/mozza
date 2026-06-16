package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mozza_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mozza_http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds.",
		Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"method", "path"})

	deploymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mozza_deployments_total",
		Help: "Total number of deployments.",
	}, []string{"target", "status"})

	rateLimitRejectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mozza_rate_limit_rejections_total",
		Help: "Total number of rate limit rejections.",
	})

	activeSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mozza_active_sessions",
		Help: "Number of active user sessions.",
	})
)

// RecordDeployment records a deployment metric.
func RecordDeployment(target, status string) {
	deploymentsTotal.WithLabelValues(target, status).Inc()
}

// SessionCreated increments the active sessions gauge.
func SessionCreated() {
	activeSessions.Inc()
}

// SessionDestroyed decrements the active sessions gauge.
func SessionDestroyed() {
	activeSessions.Dec()
}

// metricsMiddleware records HTTP request metrics.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()
		// Use chi route pattern (e.g., "/api/v1/deployments/{id}") to avoid
		// unbounded cardinality from dynamic URL path segments.
		path := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			path = rctx.RoutePattern()
		}
		method := r.Method
		status := strconv.Itoa(rec.statusCode)

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}

// metricsResponseWriter captures the status code for metrics recording.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
