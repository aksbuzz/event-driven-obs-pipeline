# ADR-002: Other Major Architecture Decisions

*Accepted — 2026-03-05*

---

## TimescaleDB over plain PostgreSQL

The pipeline writes time-series event data continuously, and the dashboard needs fast rollup queries (events per minute by service and level). Plain Postgres can do this, but it requires manual table partitioning, manual partition pruning in queries, and a separate cron job to expire old data. TimescaleDB automates all of that through hypertables, continuous aggregates, and retention policies — and it's still just Postgres under the hood, so the `pgx` driver and standard SQL work unchanged.

The enricher runs these on startup:

- Creates an `events` hypertable partitioned into 7-day chunks. Queries scoped to a time range skip irrelevant chunks entirely.
- Adds GIN indexes on the `payload` and `enrichment` JSONB columns so dashboard queries can filter on arbitrary nested fields without a fixed schema.
- Creates an `events_per_minute` continuous aggregate (materialized view) grouping by 1-minute bucket, service, and level. Dashboard queries hit this view — kilobytes — instead of scanning the raw table.
- Sets a 30-day retention policy. Old chunks drop automatically.

The tradeoff is that rollup data has up to 1-minute staleness, which is fine for a dashboard.

## Non-blocking ingestor with a DLQ for invalid events

When an event fails JSON Schema validation, we return `202 Accepted` and route it to `events.dlq` instead of returning a 4xx error. The reason: upstream producers are production services (payments API, auth service, etc.) and they should not be tightly coupled to the observability pipeline's schema version. If we returned 400s, a schema update would immediately start breaking all producers simultaneously. And retrying a schema-invalid event will never succeed — the producer would just loop.

Silent dropping was also rejected. The DLQ preserves the full payload and a `reason` header so the team can debug schema mismatches and replay events after fixing them. The Prometheus metric `ingestor_events_errors_total` tracks DLQ routing rate so a sudden spike is visible.

## Composable, graceful enrichment pipeline

The enricher runs three steps in sequence: GeoIP lookup → service metadata lookup → timestamp metadata. Each step is independent — if one fails, it logs a warning and passes the event along with that field empty rather than aborting. GeoIP in particular is optional at startup; if the MaxMind `.mmdb` file isn't present, the service starts fine and just skips that enrichment step. This keeps local development simple and means a missing GeoIP database doesn't take down the whole pipeline.

The consequence is that some events in TimescaleDB will have partial enrichment. Dashboard queries on `enrichment.geoIp` need to handle null fields. Adding a new enrichment step is just implementing the `Enricher` interface and appending it to the pipeline slice.

## `eventId` as the idempotency key

Every event carries a UUID `eventId`. The ingestor assigns one if the producer doesn't include it. This single field does three things:

1. **Deduplication in TimescaleDB** — `ON CONFLICT (event_id, timestamp) DO NOTHING` makes duplicate inserts safe after a crash-recovery re-delivery.
2. **Kafka partition key for the enricher** — ensures the enriched event for a given `eventId` lands on a predictable partition.
3. **Cross-pipeline correlation** — the detector and query API can join raw ↔ enriched ↔ alert data on `eventId`.

The downside: if a producer sends the same event twice without supplying their own `eventId`, the ingestor generates two different UUIDs and both get stored. Producers that need true idempotent re-submission must generate and supply their own stable `eventId`.
