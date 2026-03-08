package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Timescale struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTimescale(connStr string, logger *zap.Logger) (*Timescale, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping timescaledb: %w", err)
	}

	logger.Info("connected to timescaledb")
	return &Timescale{pool: pool, logger: logger}, nil
}

// Migrate runs idempotent schema migrations.
// In production you'd use a proper migration tool (goose, atlas); this is clean enough for a portfolio project.
func (t *Timescale) Migrate(ctx context.Context) error {
	migrations := []string{
		// Enable TimescaleDB extension
		`CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE`,

		// Core events table — partitioned by time via TimescaleDB hypertable
		`CREATE TABLE IF NOT EXISTS events (
			event_id         TEXT        NOT NULL,
			schema_version   INTEGER     NOT NULL DEFAULT 1,
			service          TEXT        NOT NULL,
			environment      TEXT        NOT NULL,
			instance         TEXT,
			service_version  TEXT,
			level            TEXT        NOT NULL,
			category         TEXT,
			timestamp        TIMESTAMPTZ NOT NULL,
			payload          JSONB       NOT NULL DEFAULT '{}',
			enrichment       JSONB       NOT NULL DEFAULT '{}',
			ingested_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (event_id, timestamp)  -- timestamp required for hypertable partitioning
		)`,

		// Convert to hypertable partitioned by timestamp (7-day chunks)
		`SELECT create_hypertable('events', 'timestamp', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days')`,

		// Indexes for common query patterns
		`CREATE INDEX IF NOT EXISTS idx_events_service     ON events (service, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_level       ON events (level, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_category    ON events (category, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_payload_gin ON events USING GIN (payload)`,
		`CREATE INDEX IF NOT EXISTS idx_events_enrichment_gin ON events USING GIN (enrichment)`,

		// Continuous aggregate: per-minute event counts by service and level
		// This is what makes the dashboard fast — queries hit the materialized view, not raw data
		`CREATE MATERIALIZED VIEW IF NOT EXISTS events_per_minute
			WITH (timescaledb.continuous) AS
			SELECT
				time_bucket('1 minute', timestamp) AS bucket,
				service,
				level,
				COUNT(*) AS event_count,
				AVG((payload->>'durationMs')::numeric) FILTER (WHERE payload->>'durationMs' IS NOT NULL) AS avg_duration_ms,
				MAX((payload->>'durationMs')::numeric) FILTER (WHERE payload->>'durationMs' IS NOT NULL) AS max_duration_ms
			FROM events
			GROUP BY bucket, service, level
			WITH NO DATA`,

		// Refresh policy: keep the aggregate up to date
		`SELECT add_continuous_aggregate_policy('events_per_minute',
			start_offset => INTERVAL '10 minutes',
			end_offset   => INTERVAL '1 minute',
			schedule_interval => INTERVAL '1 minute',
			if_not_exists => TRUE
		)`,

		// Retention: drop raw data older than 30 days, keep aggregates longer
		`SELECT add_retention_policy('events', INTERVAL '30 days', if_not_exists => TRUE)`,
	}

	for _, m := range migrations {
		if _, err := t.pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration failed [%.60s...]: %w", m, err)
		}
	}

	t.logger.Info("timescaledb migrations complete")
	return nil
}

// InsertEvent persists an enriched event. ON CONFLICT DO NOTHING provides idempotency —
// reprocessed events (after a crash before offset commit) are silently skipped.
func (t *Timescale) InsertEvent(ctx context.Context, event map[string]any) error {
	eventID, _ := event["eventId"].(string)
	schemaVersion, _ := event["schemaVersion"].(float64)
	level, _ := event["level"].(string)
	category, _ := event["category"].(string)
	tsStr, _ := event["timestamp"].(string)

	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		ts = time.Now().UTC()
	}

	// Extract source fields
	service, environment, instance, serviceVersion := "", "", "", ""
	if src, ok := event["source"].(map[string]any); ok {
		service, _ = src["service"].(string)
		environment, _ = src["environment"].(string)
		instance, _ = src["instance"].(string)
		serviceVersion, _ = src["version"].(string)
	}

	// Marshal JSONB fields
	payload, _ := event["payload"].(map[string]any)
	enrichment, _ := event["enrichment"].(map[string]any)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	enrichmentJSON, err := json.Marshal(enrichment)
	if err != nil {
		return fmt.Errorf("marshal enrichment: %w", err)
	}

	_, err = t.pool.Exec(ctx, `
		INSERT INTO events (
			event_id, schema_version, service, environment, instance, service_version,
			level, category, timestamp, payload, enrichment
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (event_id, timestamp) DO NOTHING
	`,
		eventID, int(schemaVersion), service, environment, instance, serviceVersion,
		level, category, ts, payloadJSON, enrichmentJSON,
	)

	return err
}

func (t *Timescale) Close() {
	t.pool.Close()
}
