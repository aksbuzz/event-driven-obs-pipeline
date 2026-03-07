package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/config"
	"github.com/aksbuzz/obs-pipeline/services/ingestor/internal/metrics"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// sentinel errors to distinguish failure modes for metric routing
var (
	errValidation = errors.New("validation")
	errPublish    = errors.New("publish")
)

type publisher interface {
	Publish(topic, key string, value any) error
	PublishToDLQ(topic string, originalPayload []byte, reason string) error
}

type validatorIface interface {
	Validate(doc any) error
}

type Handler struct {
	producer publisher
	v        validatorIface
	m        *metrics.Metrics
	logger   *zap.Logger
	cfg      *config.Config
}

func New(producer publisher, v validatorIface, m *metrics.Metrics, logger *zap.Logger, cfg *config.Config) *Handler {
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

	eventID, processErr := h.processEvent(body)

	h.m.IngestLatency.Observe(time.Since(start).Seconds())

	if processErr != nil {
		if errors.Is(processErr, errValidation) {
			h.m.IngestValidationErrors.Inc()
		} else {
			h.m.IngestPublishErrors.Inc()
		}
		h.logger.Warn("event failed, routed to DLQ",
			zap.String("eventId", eventID),
			zap.Error(processErr),
		)
		if dlqErr := h.producer.PublishToDLQ(h.cfg.TopicDLQ, body, processErr.Error()); dlqErr != nil {
			h.m.DLQPublishErrors.Inc()
			h.logger.Error("failed to publish to DLQ — event lost",
				zap.String("eventId", eventID),
				zap.Error(dlqErr),
			)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "event could not be persisted, please retry",
				"eventId": eventID,
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "accepted", "eventId": eventID, "warning": "routed to DLQ"})
		return
	}

	h.m.IngestTotal.Inc()
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

	if len(rawEvents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "batch must not be empty"})
		return
	}

	if len(rawEvents) > h.cfg.MaxBatchSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("batch exceeds max size of %d", h.cfg.MaxBatchSize),
		})
		return
	}

	results := make([]gin.H, 0, len(rawEvents))
	accepted, validationRejected, publishRejected := 0, 0, 0

	for _, raw := range rawEvents {
		eventID, err := h.processEvent(raw)
		if err != nil {
			if errors.Is(err, errValidation) {
				validationRejected++
			} else {
				publishRejected++
			}
			if dlqErr := h.producer.PublishToDLQ(h.cfg.TopicDLQ, raw, err.Error()); dlqErr != nil {
				h.m.DLQPublishErrors.Inc()
				h.logger.Error("failed to publish batch event to DLQ — event lost",
					zap.String("eventId", eventID),
					zap.Error(dlqErr),
				)
				results = append(results, gin.H{"eventId": eventID, "status": "lost", "reason": dlqErr.Error()})
			} else {
				results = append(results, gin.H{"eventId": eventID, "status": "dlq", "reason": err.Error()})
			}
		} else {
			results = append(results, gin.H{"eventId": eventID, "status": "accepted"})
			accepted++
		}
	}

	h.m.IngestTotal.Add(float64(accepted))
	h.m.IngestValidationErrors.Add(float64(validationRejected))
	h.m.IngestPublishErrors.Add(float64(publishRejected))
	h.m.IngestLatency.Observe(time.Since(start).Seconds())

	c.JSON(http.StatusMultiStatus, gin.H{
		"accepted": accepted,
		"rejected": validationRejected + publishRejected,
		"results":  results,
	})
}

// processEvent validates, stamps, and publishes a raw event. Returns the eventId.
// Errors are wrapped with errValidation or errPublish for metric routing.
func (h *Handler) processEvent(raw []byte) (string, error) {
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		return uuid.NewString(), fmt.Errorf("%w: invalid JSON: %w", errValidation, err)
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
		return eventID, fmt.Errorf("%w: %w", errValidation, err)
	}

	// Partition key = service name for per-service ordering
	partitionKey := "unknown"
	if src, ok := event["source"].(map[string]any); ok {
		if svc, ok := src["service"].(string); ok {
			partitionKey = svc
		}
	}

	if err := h.producer.Publish(h.cfg.TopicRaw, partitionKey, event); err != nil {
		return eventID, fmt.Errorf("%w: kafka publish: %w", errPublish, err)
	}

	return eventID, nil
}
