package config

import "os"

type Config struct {
	HTTPAddr     string
	KafkaBrokers string
	TopicRaw     string
	TopicDLQ     string
	SchemaPath   string
	MaxBatchSize int
}

func Load() *Config {
	return &Config{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8080"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		TopicRaw:     getEnv("TOPIC_RAW", "events.raw"),
		TopicDLQ:     getEnv("TOPIC_DLQ", "events.dlq"),
		SchemaPath:   getEnv("SCHEMA_PATH", "/app/schema.json"),
		MaxBatchSize: 500,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
