package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

type Producer struct {
	p      *kafka.Producer
	logger *zap.Logger
}

func NewProducer(brokers string, transactionalID string, logger *zap.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"acks":               "all",
		"enable.idempotence": true,
		"retries":            10,
		"retry.backoff.ms":   100,
		"compression.type":   "gzip",
		"linger.ms":          5,
		"transactional.id":   transactionalID,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewProducer: %w", err)
	}

	if err := p.InitTransactions(context.Background()); err != nil {
		return nil, fmt.Errorf("InitTransactions: %w", err)
	}

	prod := &Producer{p: p, logger: logger}
	go prod.handleDeliveryReports()
	return prod, nil
}

func (p *Producer) Publish(topic, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          payload,
		Headers:        []kafka.Header{{Key: "producedAt", Value: []byte(time.Now().UTC().Format(time.RFC3339))}},
	}, nil)
}

func (p *Producer) PublishToDLQ(topic string, originalPayload []byte, reason string) error {
	dlq := map[string]any{
		"originalPayload": string(originalPayload),
		"failureReason":   reason,
		"failedAt":        time.Now().UTC().Format(time.RFC3339),
		"source":          "enricher",
	}
	payload, err := json.Marshal(dlq)
	if err != nil {
		return fmt.Errorf("marshal dlq: %w", err)
	}
	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
	}, nil)
}

func (p *Producer) Begin() error {
	return p.p.BeginTransaction()
}

func (p *Producer) Commit(ctx context.Context) error {
	return p.p.CommitTransaction(ctx)
}

func (p *Producer) Abort(ctx context.Context) error {
	return p.p.AbortTransaction(ctx)
}

func (p *Producer) SendOffsets(ctx context.Context, msgs []*kafka.Message, consumer *kafka.Consumer) error {
	md, err := consumer.GetConsumerGroupMetadata()
	if err != nil {
		return fmt.Errorf("GetConsumerGroupMetadata: %w", err)
	}

	// Compute highest committed offset (offset+1) per partition across the batch.
	type partKey struct {
		topic     string
		partition int32
	}
	maxOffsets := make(map[partKey]kafka.Offset)
	for _, msg := range msgs {
		if msg.TopicPartition.Topic == nil {
			continue
		}
		pk := partKey{*msg.TopicPartition.Topic, msg.TopicPartition.Partition}
		next := msg.TopicPartition.Offset + 1
		if next > maxOffsets[pk] {
			maxOffsets[pk] = next
		}
	}

	offsets := make([]kafka.TopicPartition, 0, len(maxOffsets))
	for pk, off := range maxOffsets {
		topic := pk.topic
		offsets = append(offsets, kafka.TopicPartition{
			Topic:     &topic,
			Partition: pk.partition,
			Offset:    off,
		})
	}

	return p.p.SendOffsetsToTransaction(ctx, offsets, md)
}

func (p *Producer) Close() {
	p.p.Flush(10_000)
	p.p.Close()
}

func (p *Producer) handleDeliveryReports() {
	for e := range p.p.Events() {
		if msg, ok := e.(*kafka.Message); ok && msg.TopicPartition.Error != nil {
			p.logger.Error("kafka delivery failed",
				zap.String("topic", *msg.TopicPartition.Topic),
				zap.Error(msg.TopicPartition.Error),
			)
		}
	}
}
