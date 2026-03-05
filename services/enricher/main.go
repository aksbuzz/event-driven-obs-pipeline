package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/consumer"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/enricher"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/kafka"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/metrics"
	"github.com/aksbuzz/obs-pipeline/services/enricher/internal/sink"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()
	m := metrics.New()

	// Kafka producer (for enriched topic output)
	producer, err := kafka.NewProducer(cfg.KafkaBrokers, logger)
	if err != nil {
		logger.Fatal("failed to create producer", zap.Error(err))
	}
	defer producer.Close()

	// TimescaleDB sink
	dbSink, err := sink.NewTimescale(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal("failed to connect to timescaledb", zap.Error(err))
	}
	defer dbSink.Close()

	// Run schema migrations on startup
	if err := dbSink.Migrate(context.Background()); err != nil {
		logger.Fatal("migration failed", zap.Error(err))
	}

	// Enrichment pipeline (stateless, composable steps)
	pipeline := enricher.NewPipeline(
		enricher.NewGeoIPEnricher(cfg.GeoIPDBPath, logger),
		enricher.NewServiceMetaEnricher(cfg.ServiceRegistryPath, logger),
		enricher.NewTimestampEnricher(),
	)

	// Kafka consumer
	cons, err := consumer.New(cfg, producer, dbSink, pipeline, m, logger)
	if err != nil {
		logger.Fatal("failed to create consumer", zap.Error(err))
	}

	// Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
		logger.Info("metrics server listening", zap.String("addr", cfg.MetricsAddr))
		http.ListenAndServe(cfg.MetricsAddr, nil)
	}()

	ctx, cancel := context.WithCancel(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutdown signal received")
		cancel()
	}()

	logger.Info("enricher starting",
		zap.String("inputTopic", cfg.TopicRaw),
		zap.String("outputTopic", cfg.TopicEnriched),
	)

	if err := cons.Run(ctx); err != nil {
		logger.Error("consumer exited with error", zap.Error(err))
	}

	logger.Info("enricher stopped")
}
