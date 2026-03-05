package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	IngestTotal             prometheus.Counter
	IngestValidationErrors  prometheus.Counter
	IngestPublishErrors     prometheus.Counter
	DLQPublishErrors        prometheus.Counter
	IngestLatency           prometheus.Histogram
	HTTPRequests            *prometheus.CounterVec
	HTTPDuration            *prometheus.HistogramVec
}

func New(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		IngestTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_total",
			Help: "Total events successfully published to Kafka",
		}),
		IngestValidationErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_validation_errors_total",
			Help: "Total events that failed schema validation and were routed to DLQ",
		}),
		IngestPublishErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_publish_errors_total",
			Help: "Total events that failed Kafka publish and were routed to DLQ",
		}),
		DLQPublishErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "ingestor_events_dlq_publish_errors_total",
			Help: "Total events that could not be published to the DLQ (event silently lost)",
		}),
		IngestLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "ingestor_event_latency_seconds",
			Help:    "End-to-end latency from request receipt to Kafka ack",
			Buckets: prometheus.DefBuckets,
		}),
		HTTPRequests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "ingestor_http_requests_total",
			Help: "HTTP requests by method, path, and status code",
		}, []string{"method", "path", "status"}),
		HTTPDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
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
