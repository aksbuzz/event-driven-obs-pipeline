package kafka

import (
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

func NewProducer(brokers string, logger *zap.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"acks":               "all",
		"enable.idempotence": true,
		"retries":            10,
		"retry.backoff.ms":   100,
		"compression.type":   "snappy",
		"linger.ms":          5,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewProducer: %w", err)
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
	payload, _ := json.Marshal(dlq)
	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
	}, nil)
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
