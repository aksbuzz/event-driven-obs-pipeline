package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/handler"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/kafka"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/metrics"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/validator"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	// Kafka producer
	producer, err := kafka.NewProducer(cfg.KafkaBrokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// Schema validator
	v, err := validator.New(cfg.SchemaPath)
	if err != nil {
		logger.Fatal("failed to load event schema", zap.Error(err))
	}

	// Prometheus metrics
	m := metrics.New(prometheus.DefaultRegisterer)

	// HTTP server
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(metrics.GinMiddleware(m))

	h := handler.New(producer, v, m, logger, cfg)

	r.POST("/v1/events", h.IngestEvent)
	r.POST("/v1/events/batch", h.IngestBatch)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	// Graceful shutdown
	go func() {
		logger.Info("ingestor listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("shutdown complete")
}
