package config

import "os"

type Config struct {
	KafkaBrokers        string
	KafkaGroupID        string
	TopicRaw            string
	TopicEnriched       string
	TopicDLQ            string
	DatabaseURL         string
	GeoIPDBPath         string
	ServiceRegistryPath string
	MetricsAddr         string
	// Consumer tuning
	MaxPollIntervalMs int
	SessionTimeoutMs  int
}

func Load() *Config {
	return &Config{
		KafkaBrokers:        getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID:        getEnv("KAFKA_GROUP_ID", "enricher-v1"),
		TopicRaw:            getEnv("TOPIC_RAW", "events.raw"),
		TopicEnriched:       getEnv("TOPIC_ENRICHED", "events.enriched"),
		TopicDLQ:            getEnv("TOPIC_DLQ", "events.dlq"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://obs_user:obs_password@localhost:5432/obs_pipeline"),
		GeoIPDBPath:         getEnv("GEOIP_DB_PATH", "/app/GeoLite2-City.mmdb"),
		ServiceRegistryPath: getEnv("SERVICE_REGISTRY_PATH", "/app/service-registry.json"),
		MetricsAddr:         getEnv("METRICS_ADDR", ":9090"),
		MaxPollIntervalMs:   300_000,
		SessionTimeoutMs:    10_000,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
