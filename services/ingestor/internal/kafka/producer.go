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
		"bootstrap.servers":                     brokers,
		"acks":                                  "all", // strongest durability guarantee
		"enable.idempotence":                    true,  // exactly-once at producer level
		"max.in.flight.requests.per.connection": 5,     // safe with idempotence
		"retries":                               10,
		"retry.backoff.ms":                      100,
		"compression.type":                      "snappy",
		"linger.ms":                             5, // micro-batching for throughput
		"batch.size":                            65536,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewProducer: %w", err)
	}

	prod := &Producer{p: p, logger: logger}

	// Async delivery report handler — logs failures, doesn't block callers
	go prod.handleDeliveryReports()

	return prod, nil
}

// Publish sends a message to the given topic.
// key is used for partition routing — use service name for ordering guarantees per service.
func (p *Producer) Publish(topic, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          payload,
		Headers: []kafka.Header{
			{Key: "producedAt", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}, nil) // nil = fire-and-forget with async delivery reports
}

// PublishToDLQ sends a failed/invalid event to the dead-letter queue with failure context.
func (p *Producer) PublishToDLQ(topic string, originalPayload []byte, reason string) error {
	dlqEnvelope := map[string]any{
		"originalPayload": string(originalPayload),
		"failureReason":   reason,
		"failedAt":        time.Now().UTC().Format(time.RFC3339),
	}
	payload, _ := json.Marshal(dlqEnvelope)

	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
		Headers: []kafka.Header{
			{Key: "dlq-reason", Value: []byte(reason)},
		},
	}, nil)
}

func (p *Producer) Close() {
	// Flush remaining messages before shutdown (up to 10s)
	remaining := p.p.Flush(10_000)
	if remaining > 0 {
		p.logger.Warn("kafka flush: messages not delivered on shutdown",
			zap.Int("remaining", remaining))
	}
	p.p.Close()
}

func (p *Producer) handleDeliveryReports() {
	for e := range p.p.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logger.Error("kafka delivery failed",
					zap.String("topic", *ev.TopicPartition.Topic),
					zap.Error(ev.TopicPartition.Error),
				)
			}
		}
	}
}
