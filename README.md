# obs-pipeline

An event-driven observability pipeline. Services emit structured events → Kafka ingests → Go enricher adds metadata + persists to TimescaleDB → GraphQL API serves queries → React dashboard visualizes.

## Architecture

```
[Services] → POST /v1/events → [Ingestor] → events.raw (Kafka)
                                                   ↓
                                            [Enricher]
                                            ├── GeoIP tagging
                                            ├── Service metadata lookup
                                            └── Ingest lag calculation
                                                   ↓
                                    ┌──────────────┴──────────────┐
                              TimescaleDB                  events.enriched (Kafka)
                           (hypertable + CAgg)                     ↓
                                    ↑                       [Detector] (Week 3)
                              [Query API]                          ↓
                           (GraphQL + WS)                   events.alerts (Kafka)
                                    ↑
                             [Dashboard]
                             (Next.js)
```

**Kafka topics:**
| Topic | Purpose | Retention |
|---|---|---|
| `events.raw` | Raw validated events from ingestor | 7 days |
| `events.enriched` | Enriched events for downstream consumers | 7 days |
| `events.dlq` | Failed/invalid events for debugging | 30 days |
| `events.alerts` | Anomaly alerts from detector | 24 hours |

## Quick Start

### 1. Start infrastructure
```bash
docker compose up -d kafka timescaledb redis

# Wait for Kafka to be healthy, then create topics
chmod +x infra/kafka-init/create-topics.sh
./infra/kafka-init/create-topics.sh
```

### 2. Run services (local dev)
```bash
# Ingestor
cd services/ingestor
KAFKA_BROKERS=localhost:9092 SCHEMA_PATH=../../libs/event-schema/schema.json go run main.go

# Enricher (separate terminal)
cd services/enricher
KAFKA_BROKERS=localhost:9092 \
DATABASE_URL=postgres://obs_user:obs_password@localhost:5432/obs_pipeline \
SERVICE_REGISTRY_PATH=../../libs/event-schema/service-registry.json \
go run main.go
```

### 3. Or run everything via Docker
```bash
docker compose up -d
```

### 4. Send test events
```bash
# Single event
curl -X POST http://localhost:8080/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "schemaVersion": 1,
    "source": { "service": "payments-api", "environment": "production" },
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "level": "error",
    "category": "log",
    "payload": {
      "message": "charge failed: insufficient funds",
      "statusCode": 402,
      "durationMs": 234.5,
      "numeric": { "durationMs": 234.5 }
    }
  }'

# Load generator (requires: pip install requests faker)
python3 scripts/load-gen.py --rate 20 --duration 120
```

## Ports

| Service | Port | Endpoint |
|---|---|---|
| Ingestor HTTP API | 8080 | `POST /v1/events`, `POST /v1/events/batch` |
| Ingestor metrics | 8080 | `GET /metrics` |
| Enricher metrics | 9090 | `GET /metrics` |
| TimescaleDB | 5432 | postgres |
| Kafka | 9092 | bootstrap server |
| Redis | 6379 | |

## Key Design Decisions

**Why manual offset commits?**
The enricher only commits Kafka offsets after a successful DB write. This gives at-least-once delivery. Combined with `ON CONFLICT DO NOTHING` on `eventId`, the effective guarantee is exactly-once — duplicates from reprocessing are silently dropped.

**Why DLQ instead of blocking on bad events?**
The ingestor returns `202 Accepted` even for invalid events, routing them to `events.dlq`. Blocking producers on validation failures would couple their availability to our schema strictness. The DLQ lets us debug without causing upstream retries.

**Why TimescaleDB over plain PostgreSQL?**
The hypertable gives us automatic time-based partitioning. The continuous aggregate (`events_per_minute`) materializes per-minute rollups, so the dashboard query touches kilobytes instead of gigabytes. The retention policy handles cleanup automatically.

**Why partition by service name?**
Kafka partition key = service name ensures all events from a given service land on the same partition, preserving per-service ordering. This matters when the anomaly detector needs to reason about sequences of events from the same source.
