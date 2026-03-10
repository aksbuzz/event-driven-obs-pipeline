# ADR-001: Kafka as the Event Backbone

*Accepted — 2026-03-05*

---

We needed something that could sit between the ingestor and all downstream consumers (enricher, detector, query API) without any of them knowing about each other. Kafka fit because it lets multiple consumers read the same stream independently, replays events after a crash, and handles burst traffic naturally through buffering. Options like RabbitMQ don't support replay once a message is consumed, and direct HTTP fan-out from the ingestor would create tight availability coupling — if the enricher is down, the ingestor would start failing.

## Topics

Four topics are provisioned at startup via `infra/kafka-init/create-topics.sh`:

| Topic | Partitions | Retention | Purpose |
|---|---|---|---|
| `events.raw` | 3 | 7 days | Validated events from ingestor |
| `events.enriched` | 3 | 7 days | Enriched events for detector and query API |
| `events.dlq` | 1 | 30 days | Invalid/failed events for debugging |
| `events.alerts` | 1 | 24 hours | Anomaly alerts from detector |

3 partitions on raw/enriched allows up to 3 parallel enricher instances. The DLQ gets 30 days because those messages represent failures that need investigation and possible replay — 7 days isn't always enough time to notice, fix, and replay.

## Partition key

Both producers key messages on **service name** (e.g., `payments-api`). This ensures all events from one service land on the same partition, preserving per-service ordering. The detector computes rolling statistics per service, so out-of-order events would corrupt those calculations.

## Producer settings

Both producers share:

```
acks = all                                   # wait for all in-sync replicas before ack
enable.idempotence = true                    # prevent duplicate records on retry
max.in.flight.requests.per.connection = 5   # safe upper limit with idempotence
retries = 10, retry.backoff.ms = 100
linger.ms = 5, batch.size = 65536           # micro-batch to reduce produce requests
```

The ingestor uses `compression.type = snappy` (low CPU, good ratio for JSON). The enricher uses `compression.type = gzip` because its output on `events.enriched` is consumed by the detector, which uses librdkafka (supports gzip). The enricher also sets `transactional.id = enricher-transactional` to enable the Kafka transactions API — see ADR-004.

`acks=all` + idempotence prevents both data loss and duplicates at the producer level. The 5ms linger is there because the ingestor accepts batch HTTP requests of up to 500 events; without it, those would all hit Kafka as individual produce requests.

## Consumer settings (enricher)

```
group.id = enricher-v1
auto.offset.reset = earliest        # on first boot, read from the beginning
enable.auto.commit = false          # offsets committed via SendOffsetsToTransaction
session.timeout.ms = 10000          # 10s for broker to detect a dead consumer
max.poll.interval.ms = 300000       # 5 min max for a batch processing cycle
partition.assignment.strategy = cooperative-sticky
```

Offsets are committed via `SendOffsetsToTransaction`, not `CommitOffset`. This makes the offset advance atomic with the transaction commit — see ADR-004 for the full rationale. `cooperative-sticky` avoids the stop-the-world rebalance pause that the default eager strategy causes.

## Consumer settings (detector)

```
group.id = detector-v1
enable.auto.commit = false          # commit only after successful window evaluation
isolation.level = read_committed    # only consume messages from committed transactions
```

`isolation.level = read_committed` is required because `events.enriched` is produced transactionally by the enricher. Without it, the detector could read messages from an aborted transaction before the broker marks them as aborted.

## Delivery guarantee

The enricher wraps each batch of 100 messages in a single Kafka transaction:

1. `BeginTransaction()`
2. Enrich all 100 messages; route failures to DLQ
3. `InsertBatch` to TimescaleDB — `ON CONFLICT (event_id, timestamp) DO NOTHING`
4. Publish all enriched events to `events.enriched`
5. `SendOffsetsToTransaction` — atomically bind consumer offset advance to the transaction
6. `CommitTransaction()`

If the enricher crashes at any step, the broker times out the transaction and aborts it. The consumer offset is not advanced. On restart, the same batch is reprocessed from scratch. The DB insert is idempotent; the produces are retried inside the new transaction. This gives true end-to-end exactly-once semantics for the enriched event stream.
