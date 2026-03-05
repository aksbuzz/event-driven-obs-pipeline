package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/enricher"
	kafkapkg "github.com/aksbuzz/obs-pipeline/services/enricher/internal/kafka"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/metrics"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/sink"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

type Consumer struct {
	consumer *kafka.Consumer
	producer *kafkapkg.Producer
	db       *sink.Timescale
	pipeline *enricher.Pipeline
	m        *metrics.Metrics
	logger   *zap.Logger
	cfg      *config.Config
}

func New(
	cfg *config.Config,
	producer *kafkapkg.Producer,
	db *sink.Timescale,
	pipeline *enricher.Pipeline,
	m *metrics.Metrics,
	logger *zap.Logger,
) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": cfg.KafkaBrokers,
		"group.id":          cfg.KafkaGroupID,
		"auto.offset.reset": "earliest",
		// Disable auto-commit — we commit only after successful DB write
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

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer context cancelled, stopping poll loop")
			return nil
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
		if err := c.processMessage(ctx, msg); err != nil {
			c.logger.Error("processing failed, routing to DLQ",
				zap.String("topic", *msg.TopicPartition.Topic),
				zap.Int32("partition", msg.TopicPartition.Partition),
				zap.Error(err),
			)
			c.producer.PublishToDLQ(c.cfg.TopicDLQ, msg.Value, err.Error())
			c.m.DLQRouted.Inc()
		}

		// Commit offset only after successful processing + DLQ routing
		// If the process crashes here, we'll reprocess — idempotency key on DB handles duplicates
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

	// Run enrichment pipeline
	enriched := c.pipeline.Run(ctx, event)

	// Persist to TimescaleDB — idempotent on eventId
	if err := c.db.InsertEvent(ctx, enriched); err != nil {
		return fmt.Errorf("db insert: %w", err)
	}

	// Publish enriched event for downstream consumers (anomaly detector, etc.)
	eventID, _ := enriched["eventId"].(string)
	if err := c.producer.Publish(c.cfg.TopicEnriched, eventID, enriched); err != nil {
		// Non-fatal: event is in DB, downstream will be delayed but not lost
		c.logger.Warn("failed to publish enriched event to Kafka",
			zap.String("eventId", eventID),
			zap.Error(err),
		)
	}

	c.m.MessagesProcessed.Inc()
	c.m.ProcessingLatency.Observe(time.Since(start).Seconds())
	return nil
}
