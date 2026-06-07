package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds a dedicated Prometheus registry and the service's HTTP metrics.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics registers the default Go/process collectors plus custom HTTP
// metrics on a private registry.
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "route", "status"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	registry.MustRegister(requests, duration)

	return &Metrics{registry: registry, requests: requests, duration: duration}
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Instrument records the request count and latency for a single route, using
// the static route pattern as a label to keep cardinality bounded.
func (m *Metrics) Instrument(method, route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		labels := prometheus.Labels{
			"method": method,
			"route":  route,
			"status": strconv.Itoa(rec.status),
		}
		m.requests.With(labels).Inc()
		m.duration.With(labels).Observe(time.Since(start).Seconds())
	})
}
