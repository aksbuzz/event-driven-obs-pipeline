package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	MessagesConsumed  prometheus.Counter
	MessagesProcessed prometheus.Counter
	ConsumerErrors    prometheus.Counter
	DLQRouted         prometheus.Counter
	ProcessingLatency prometheus.Histogram
	ConsumerLag       prometheus.Gauge
}

func New() *Metrics {
	return &Metrics{
		MessagesConsumed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "enricher_messages_consumed_total",
			Help: "Total messages read from Kafka",
		}),
		MessagesProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "enricher_messages_processed_total",
			Help: "Total messages successfully enriched and persisted",
		}),
		ConsumerErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "enricher_consumer_errors_total",
			Help: "Kafka consumer errors (not processing errors)",
		}),
		DLQRouted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "enricher_dlq_routed_total",
			Help: "Messages routed to DLQ due to processing failure",
		}),
		ProcessingLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "enricher_processing_latency_seconds",
			Help:    "Time from message read to DB write + Kafka publish",
			Buckets: prometheus.DefBuckets,
		}),
		ConsumerLag: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "enricher_consumer_lag_messages",
			Help: "Current consumer group lag — key backpressure signal",
		}),
	}
}
