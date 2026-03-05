package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/enricher"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/metrics"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

// kafkaPublisher abstracts the producer dependency for testability.
type kafkaPublisher interface {
	Publish(topic, key string, value any) error
	PublishToDLQ(topic string, payload []byte, reason string) error
}

// eventSink abstracts the database dependency for testability.
type eventSink interface {
	InsertEvent(ctx context.Context, event map[string]any) error
}

type Consumer struct {
	consumer *kafka.Consumer
	producer kafkaPublisher
	db       eventSink
	pipeline *enricher.Pipeline
	m        *metrics.Metrics
	logger   *zap.Logger
	cfg      *config.Config
}

func New(
	cfg *config.Config,
	producer kafkaPublisher,
	db eventSink,
	pipeline *enricher.Pipeline,
	m *metrics.Metrics,
	logger *zap.Logger,
) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": cfg.KafkaBrokers,
		"group.id":          cfg.KafkaGroupID,
		"auto.offset.reset": "earliest",
		// Disable auto-commit — we commit only after successful DB write (or confirmed DLQ routing).
		// This is the key to at-least-once processing. Combined with DB idempotency
		// keys (eventId), the effective guarantee is exactly-once.
		"enable.auto.commit":            false,
		"max.poll.interval.ms":          cfg.MaxPollIntervalMs,
		"session.timeout.ms":            cfg.SessionTimeoutMs,
		"partition.assignment.strategy": "cooperative-sticky",
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewConsumer: %w", err)
	}

	if err := c.Subscribe(cfg.TopicRaw, nil); err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	return &Consumer{
		consumer: c,
		producer: producer,
		db:       db,
		pipeline: pipeline,
		m:        m,
		logger:   logger,
		cfg:      cfg,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.consumer.Close()

	// Fire immediately so lag is reported on startup, then every 30s.
	// updateConsumerLag is called from this goroutine.
	lagTimer := time.NewTimer(0)
	defer lagTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer context cancelled, stopping poll loop")
			return nil
		default:
		}

		select {
		case <-lagTimer.C:
			c.updateConsumerLag()
			lagTimer.Reset(30 * time.Second)
		default:
		}

		msg, err := c.consumer.ReadMessage(500 * time.Millisecond)
		if err != nil {
			if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrTimedOut {
				continue // normal — no messages in poll window
			}
			c.logger.Error("consumer read error", zap.Error(err))
			c.m.ConsumerErrors.Inc()
			continue
		}

		c.m.MessagesConsumed.Inc()
		processingErr := c.processMessage(ctx, msg)
		if processingErr != nil {
			c.logger.Error("processing failed, routing to DLQ",
				zap.String("topic", *msg.TopicPartition.Topic),
				zap.Int32("partition", msg.TopicPartition.Partition),
				zap.Error(processingErr),
			)
			if dlqErr := c.producer.PublishToDLQ(c.cfg.TopicDLQ, msg.Value, processingErr.Error()); dlqErr != nil {
				// This preserves at-least-once semantics: event is neither in DB nor confirmed DLQ.
				c.m.DLQPublishErrors.Inc()
				c.logger.Error("DLQ publish failed — skipping offset commit to allow reprocessing",
					zap.Int32("partition", msg.TopicPartition.Partition),
					zap.Int64("offset", int64(msg.TopicPartition.Offset)),
					zap.Error(dlqErr),
				)
				continue
			}
			c.m.DLQRouted.Inc()
		}

		// Commit offset only after successful processing or confirmed DLQ routing.
		if _, err := c.consumer.CommitMessage(msg); err != nil {
			c.logger.Error("offset commit failed", zap.Error(err))
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg *kafka.Message) error {
	start := time.Now()

	var event map[string]any
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// Run enrichment pipeline — step failures are non-fatal (partial enrichment preferred)
	enriched := c.pipeline.Run(ctx, event)

	// Persist to TimescaleDB — idempotent on eventId
	if err := c.db.InsertEvent(ctx, enriched); err != nil {
		return fmt.Errorf("db insert: %w", err)
	}

	// Partition key = service name for per-service ordering on events.enriched.
	partitionKey := "unknown"
	if src, ok := enriched["source"].(map[string]any); ok {
		if svc, ok := src["service"].(string); ok {
			partitionKey = svc
		}
	}

	if err := c.producer.Publish(c.cfg.TopicEnriched, partitionKey, enriched); err != nil {
		// Non-fatal: event is in DB, downstream will be delayed but not lost.
		eventID, _ := enriched["eventId"].(string)
		c.logger.Warn("failed to publish enriched event to Kafka",
			zap.String("eventId", eventID),
			zap.Error(err),
		)
	}

	c.m.MessagesProcessed.Inc()
	c.m.ProcessingLatency.Observe(time.Since(start).Seconds())
	return nil
}

// updateConsumerLag queries Kafka for committed offsets and high watermarks across all
// assigned partitions and updates the ConsumerLag gauge.
func (c *Consumer) updateConsumerLag() {
	partitions, err := c.consumer.Assignment()
	if err != nil || len(partitions) == 0 {
		return
	}

	committed, err := c.consumer.Committed(partitions, 2_000)
	if err != nil {
		c.logger.Warn("failed to fetch committed offsets for lag calculation", zap.Error(err))
		return
	}

	var totalLag int64
	for _, tp := range committed {
		if tp.Topic == nil {
			continue
		}
		_, hi, err := c.consumer.QueryWatermarkOffsets(*tp.Topic, tp.Partition, 1_000)
		if err != nil {
			continue
		}
		committedOff := int64(tp.Offset)
		if committedOff < 0 {
			committedOff = 0 // offset not yet committed — treat as start of partition
		}
		if lag := hi - committedOff; lag > 0 {
			totalLag += lag
		}
	}

	c.m.ConsumerLag.Set(float64(totalLag))
}
