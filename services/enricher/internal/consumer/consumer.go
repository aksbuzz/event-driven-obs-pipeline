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
	Begin() error
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error

	Publish(topic, key string, value any) error
	PublishToDLQ(topic string, payload []byte, reason string) error
	SendOffsets(ctx context.Context, msgs []*kafka.Message, consumer *kafka.Consumer) error
}

// eventSink abstracts the database dependency for testability.
type eventSink interface {
	InsertEvent(ctx context.Context, event map[string]any) error
	InsertBatch(ctx context.Context, events []map[string]any) error
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
		// Disable auto-commit — offsets are committed transactionally via SendOffsetsToTransaction.
		"enable.auto.commit":            false,
		"max.poll.interval.ms":          cfg.MaxPollIntervalMs,
		"session.timeout.ms":            cfg.SessionTimeoutMs,
		"partition.assignment.strategy": "cooperative-sticky",
		// Fetch tuning: wait for up to 500ms or 64KB before returning a batch.
		// Reduces per-message overhead under load without adding latency at low throughput.
		"fetch.min.bytes":     64 * 1024, // 64KB
		"fetch.wait.max.ms":   500,
		"fetch.max.bytes":     10 * 1024 * 1024, // 10MB
		"queued.min.messages": 1000,
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

	lagTimer := time.NewTimer(0)
	defer lagTimer.Stop()

	batch := make([]*kafka.Message, 0, c.cfg.BatchSize)

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				c.flushBatch(context.Background(), batch)
			}
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
		batch = append(batch, msg)

		if len(batch) >= c.cfg.BatchSize {
			c.flushBatch(ctx, batch)
			batch = batch[:0]
		}
	}
}

// enrichedItem pairs a raw Kafka message with its enriched output and partition key.
type enrichedItem struct {
	msg      *kafka.Message
	enriched map[string]any
	partKey  string
}

// flushBatch processes a full batch atomically: enriches each message, batch-inserts to DB,
// publishes all to events.enriched, and commits one Kafka transaction covering all offsets.
// On any unrecoverable error, the transaction is aborted and offsets are not committed —
// messages will be reprocessed after a restart or rebalance.
func (c *Consumer) flushBatch(ctx context.Context, batch []*kafka.Message) {
	start := time.Now()

	if err := c.producer.Begin(); err != nil {
		c.logger.Error("begin transaction failed", zap.Error(err))
		return
	}

	var items []enrichedItem

	for _, msg := range batch {
		enriched, err := c.enrichMessage(ctx, msg)
		if err != nil {
			c.logger.Error("processing failed, routing to DLQ",
				zap.String("topic", *msg.TopicPartition.Topic),
				zap.Int32("partition", msg.TopicPartition.Partition),
				zap.Error(err),
			)
			if dlqErr := c.producer.PublishToDLQ(c.cfg.TopicDLQ, msg.Value, err.Error()); dlqErr != nil {
				c.producer.Abort(ctx)
				c.m.DLQPublishErrors.Inc()
				c.logger.Error("DLQ publish failed — aborting batch, offsets not committed",
					zap.Int32("partition", msg.TopicPartition.Partition),
					zap.Int64("offset", int64(msg.TopicPartition.Offset)),
					zap.Error(dlqErr),
				)
				return
			}
			c.m.DLQRouted.Inc()
			continue
		}

		partKey := "unknown"
		if src, ok := enriched["source"].(map[string]any); ok {
			if svc, ok := src["service"].(string); ok {
				partKey = svc
			}
		}
		items = append(items, enrichedItem{msg: msg, enriched: enriched, partKey: partKey})
	}

	if len(items) > 0 {
		enrichedMaps := make([]map[string]any, len(items))
		for i, it := range items {
			enrichedMaps[i] = it.enriched
		}

		if err := c.db.InsertBatch(ctx, enrichedMaps); err != nil {
			c.producer.Abort(ctx)
			c.logger.Error("batch db insert failed — aborting transaction", zap.Error(err))
			return
		}

		for _, it := range items {
			if err := c.producer.Publish(c.cfg.TopicEnriched, it.partKey, it.enriched); err != nil {
				c.producer.Abort(ctx)
				c.logger.Error("kafka publish failed — aborting transaction", zap.Error(err))
				return
			}
		}
	}

	if err := c.producer.SendOffsets(ctx, batch, c.consumer); err != nil {
		c.producer.Abort(ctx)
		c.logger.Error("SendOffsetsToTransaction failed", zap.Error(err))
		return
	}

	if err := c.producer.Commit(ctx); err != nil {
		c.logger.Error("commit transaction failed", zap.Error(err))
		return
	}

	c.m.MessagesProcessed.Add(float64(len(items)))
	c.m.ProcessingLatency.Observe(time.Since(start).Seconds())
}

// enrichMessage unmarshals a raw Kafka message and runs the enrichment pipeline.
// It has no I/O side effects — DB and Kafka writes happen in flushBatch.
func (c *Consumer) enrichMessage(ctx context.Context, msg *kafka.Message) (map[string]any, error) {
	var event map[string]any
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return c.pipeline.Run(ctx, event), nil
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
