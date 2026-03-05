package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	IngestTotal   prometheus.Counter
	IngestErrors  prometheus.Counter
	IngestLatency prometheus.Histogram
	HTTPRequests  *prometheus.CounterVec
	HTTPDuration  *prometheus.HistogramVec
}

func New() *Metrics {
	return &Metrics{
		IngestTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_total",
			Help: "Total events successfully published to Kafka",
		}),
		IngestErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_errors_total",
			Help: "Total events routed to DLQ due to validation or publish errors",
		}),
		IngestLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "ingestor_event_latency_seconds",
			Help:    "End-to-end latency from request receipt to Kafka ack",
			Buckets: prometheus.DefBuckets,
		}),
		HTTPRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ingestor_http_requests_total",
			Help: "HTTP requests by method, path, and status code",
		}, []string{"method", "path", "status"}),
		HTTPDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ingestor_http_duration_seconds",
			Help:    "HTTP request duration by method and path",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}
}

func GinMiddleware(m *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		m.HTTPRequests.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		m.HTTPDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}
