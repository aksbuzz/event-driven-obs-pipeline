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
	DLQPublishErrors  prometheus.Counter
	ProcessingLatency prometheus.Histogram
	ConsumerLag       prometheus.Gauge
	StepErrors        *prometheus.CounterVec
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
		DLQPublishErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "enricher_dlq_publish_errors_total",
			Help: "Messages that could not be published to the DLQ (offset not committed — will be retried)",
		}),
		ProcessingLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "enricher_processing_latency_seconds",
			Help:    "Time from message read to DB write + Kafka publish",
			Buckets: prometheus.DefBuckets,
		}),
		ConsumerLag: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "enricher_consumer_lag_messages",
			Help: "Current consumer group lag (sum across all assigned partitions)",
		}),
		StepErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "enricher_step_errors_total",
			Help: "Enrichment step failures labelled by step name",
		}, []string{"step"}),
	}
}
