package httpx

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP server metrics. Labels are intentionally low-cardinality: we use the
// matched route (FullPath) rather than the raw URL to avoid label explosion
// from path-encoded ids.
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests handled by the API",
	}, []string{"method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds",
		Buckets: prometheus.ExponentialBuckets(0.005, 2, 12), // ~5ms..20s
	}, []string{"method", "route"})

	httpInflight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being handled",
	})
)

// Metrics returns a Gin middleware that records request counts and
// latencies into the global Prometheus registry. Routes that have no
// matched FullPath (404s) are reported as "unknown" to keep cardinality
// bounded.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpInflight.Inc()
		start := time.Now()
		c.Next()
		httpInflight.Dec()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequests.WithLabelValues(c.Request.Method, route, status).Inc()
		httpDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
