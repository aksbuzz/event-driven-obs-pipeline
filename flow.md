Here's a focused breakdown of how Kafka, DB, and Redis are used across all three services.

Event Flow

[HTTP Client]
    ↓ POST /v1/events
[Ingestor] → events.raw → [Enricher] → events.enriched → [Detector] → events.alerts
                ↓                  ↓
             events.dlq       TimescaleDB
Kafka
Topics
Topic	Partitions	Retention	Owner
events.raw	3	7 days	Ingestor produces, Enricher consumes
events.enriched	3	7 days	Enricher produces, Detector consumes
events.dlq	1	30 days	Both produce (failures)
events.alerts	1	24 hours	Detector produces
Partition Key: Service Name
Every message on events.raw and events.enriched is keyed by service name. This guarantees all events from a given service land on the same partition → processed in order by consumers. Critical for the detector's per-service rolling statistics to be meaningful.

Producer Config (Ingestor + Enricher)

acks = all                              # wait for all ISR replicas
enable.idempotence = true               # exactly-once at producer level
max.in.flight.requests = 5             # safe with idempotence
retries = 10, retry.backoff.ms = 100
compression.type = snappy              # good ratio + speed
linger.ms = 5, batch.size = 65536     # micro-batching
Why acks=all + idempotence? The ingestor is the entry point. Losing raw events is unacceptable. Idempotence prevents duplicate messages on retries. The cost is slightly higher latency, acceptable for an observability pipeline.

Consumer Config (Enricher)

auto.offset.reset = earliest           # catch up from beginning if new group
enable.auto.commit = false             # CRITICAL: manual commits
partition.assignment.strategy = cooperative-sticky  # smooth rebalances
max.poll.interval.ms = 300000          # 5min — allows slow DB writes
session.timeout.ms = 10000            # detect dead consumer in 10s
Why manual commits? The enricher writes to TimescaleDB and then commits the offset. If it commits first and then the DB write fails, the event is silently lost. Manual commit = at-least-once delivery. Combined with ON CONFLICT DO NOTHING in the DB, this gives effectively exactly-once.

Consumer Config (Detector)
Same as enricher. The detector does a manual commit after the entire 60-second evaluation window fires — not per-message. This means on restart, it replays the last partial window and re-buffers those events, which is fine since duplicate alerts are suppressed via Redis cooldowns.

DLQ Pattern
The ingestor returns 202 Accepted for invalid events — it never rejects at the HTTP layer. Invalid events go to events.dlq with the failure reason and original payload. Why? Calling services shouldn't be blocked or need to handle schema validation errors. The 30-day DLQ retention allows debugging and replaying.

TimescaleDB
Used by: Enricher (write), Detector (write alerts), Query API (read)
Why TimescaleDB over Postgres?
Hypertables partition the events table by time (7-day chunks). Queries scoped to recent time ranges only touch recent chunks — huge performance win for SELECT ... WHERE timestamp > now() - interval '1 hour' queries. Also gets continuous aggregates and retention policies for free.

Schema

events (
  event_id TEXT, timestamp TIMESTAMPTZ  -- composite PK
  service, environment, level, category TEXT
  payload JSONB, enrichment JSONB        -- GeoIP, service meta
)
-- hypertable: 7-day chunks
-- 30-day retention policy
-- GIN indexes on payload + enrichment for JSONB queries

alerts (
  alert_id UUID PRIMARY KEY
  service, rule, severity TEXT
  fired_at TIMESTAMPTZ
  details JSONB
)
Idempotency

-- Enricher insert:
INSERT INTO events ... ON CONFLICT (event_id, timestamp) DO NOTHING

-- Detector insert:
INSERT INTO alerts ... ON CONFLICT (alert_id) DO NOTHING
Both services can be restarted and replay Kafka messages without creating duplicates.

Continuous Aggregate

CREATE MATERIALIZED VIEW events_per_minute AS
  SELECT time_bucket('1 minute', timestamp) AS bucket, service, level,
    COUNT(*), AVG(payload->>'durationMs'), MAX(payload->>'durationMs')
  FROM events GROUP BY bucket, service, level
-- Refreshes every 1 minute, lag behind by 1 minute
This pre-aggregates per-minute stats for the Query API / dashboard, avoiding expensive real-time aggregations on raw events.

Redis
Used by: Detector only
Two key patterns:


detector:baseline:{service}          → JSON list of last N window mean durations  TTL: 2h
detector:cooldown:{service}:{rule}   → timestamp of last alert                    TTL: cooldown_minutes × 60
Baseline (for Z-score rule)
The detector computes a 60-second tumbling window. At the end of each window it calculates the mean durationMs and appends it to the Redis list, keeping the last 10 values (zscore_baseline_size). The Z-score rule only fires when at least 5 windows (zscore_min_windows) of history exist — prevents false positives during warmup.

Why Redis and not in-memory? If the detector restarts, the baseline history survives. Without this, every restart would need 5+ minutes to rebuild the baseline before the Z-score rule could fire. The 2h TTL handles cleanup if a service goes quiet.

Cooldown (alert suppression)
After an alert fires, a TTL key is set. Any evaluation within that TTL returns "cooling down → skip." Default: 5 minutes. This prevents alert storms where a service degradation causes an alert every 60 seconds.

Why Redis and not in-memory? Same reason — survives restarts. Without persistence, a crash-restart cycle could re-trigger alerts immediately.

Key Design Decisions Summary
Decision	What	Why
enable.auto.commit=false	Both consumers	DB write must succeed before offset advances
acks=all + idempotence	Both producers	No data loss + no duplicates on retry
ON CONFLICT DO NOTHING	Enricher + Detector DB writes	Kafka replay safety
Service name as partition key	All topics	Per-service event ordering
Redis for baseline/cooldown	Detector only	Survives restarts; avoids warmup period
202 for invalid events + DLQ	Ingestor	Decouple caller from schema evolution
Manual timer window (60s)	Detector	Simple; avoids streaming framework complexity
Composable enrichment pipeline	Enricher	Partial enrichment > total failure
