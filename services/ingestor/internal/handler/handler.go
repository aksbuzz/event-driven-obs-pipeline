package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/metrics"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/validator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type publisher interface {
	Publish(topic, key string, value any) error
	PublishToDLQ(topic string, originalPayload []byte, reason string) error
}

type Handler struct {
	producer publisher
	v        *validator.Validator
	m        *metrics.Metrics
	logger   *zap.Logger
	cfg      *config.Config
}

func New(producer publisher, v *validator.Validator, m *metrics.Metrics, logger *zap.Logger, cfg *config.Config) *Handler {
	return &Handler{producer: producer, v: v, m: m, logger: logger, cfg: cfg}
}

// IngestEvent handles POST /v1/events — single event ingestion
func (h *Handler) IngestEvent(c *gin.Context) {
	start := time.Now()

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	eventID, err := h.processEvent(body)
	if err != nil {
		h.m.IngestErrors.Inc()
		// Don't expose internal errors — event goes to DLQ, caller gets accepted
		// This is intentional: we don't want producers to retry on validation failures
		h.logger.Warn("event failed validation, routed to DLQ",
			zap.String("eventId", eventID),
			zap.Error(err),
		)
		h.producer.PublishToDLQ(h.cfg.TopicDLQ, body, err.Error())
		// Still return 202 — the event was received, just not valid
		c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "eventId": eventID, "warning": "routed to DLQ"})
		return
	}

	h.m.IngestTotal.Inc()
	h.m.IngestLatency.Observe(time.Since(start).Seconds())
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "eventId": eventID})
}

// IngestBatch handles POST /v1/events/batch — up to MaxBatchSize events
func (h *Handler) IngestBatch(c *gin.Context) {
	start := time.Now()

	var rawEvents []json.RawMessage
	if err := c.ShouldBindJSON(&rawEvents); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be a JSON array"})
		return
	}

	if len(rawEvents) > h.cfg.MaxBatchSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("batch exceeds max size of %d", h.cfg.MaxBatchSize),
		})
		return
	}

	results := make([]gin.H, 0, len(rawEvents))
	accepted, rejected := 0, 0

	for _, raw := range rawEvents {
		eventID, err := h.processEvent(raw)
		if err != nil {
			h.producer.PublishToDLQ(h.cfg.TopicDLQ, raw, err.Error())
			results = append(results, gin.H{"eventId": eventID, "status": "dlq", "reason": err.Error()})
			rejected++
		} else {
			results = append(results, gin.H{"eventId": eventID, "status": "accepted"})
			accepted++
		}
	}

	h.m.IngestTotal.Add(float64(accepted))
	h.m.IngestErrors.Add(float64(rejected))
	h.m.IngestLatency.Observe(time.Since(start).Seconds())

	c.JSON(http.StatusMultiStatus, gin.H{
		"accepted": accepted,
		"rejected": rejected,
		"results":  results,
	})
}

// processEvent validates, stamps, and publishes a raw event. Returns the eventId.
func (h *Handler) processEvent(raw []byte) (string, error) {
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		return uuid.NewString(), fmt.Errorf("invalid JSON: %w", err)
	}

	// Stamp a server-side eventId if missing (idempotency key)
	eventID, _ := event["eventId"].(string)
	if eventID == "" {
		eventID = uuid.NewString()
		event["eventId"] = eventID
	}

	// Stamp schemaVersion default
	if _, ok := event["schemaVersion"]; !ok {
		event["schemaVersion"] = 1
	}

	if err := h.v.Validate(event); err != nil {
		return eventID, err
	}

	// Partition key = service name for per-service ordering
	partitionKey := "unknown"
	if src, ok := event["source"].(map[string]any); ok {
		if svc, ok := src["service"].(string); ok {
			partitionKey = svc
		}
	}

	if err := h.producer.Publish(h.cfg.TopicRaw, partitionKey, event); err != nil {
		return eventID, fmt.Errorf("kafka publish: %w", err)
	}

	return eventID, nil
}
