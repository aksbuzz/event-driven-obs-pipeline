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

Both the ingestor and enricher producers use:

```
acks = all                                   # wait for all in-sync replicas before ack
enable.idempotence = true                    # prevent duplicate records on retry
max.in.flight.requests.per.connection = 5   # safe upper limit with idempotence
retries = 10, retry.backoff.ms = 100
compression.type = snappy                    # good compression ratio, low CPU cost
linger.ms = 5, batch.size = 65536           # micro-batch to reduce produce requests
```

`acks=all` + idempotence is the combination that prevents both data loss and duplicates at the producer level. Snappy compression is a good fit for JSON payloads — they compress 3-4x with negligible CPU overhead. The 5ms linger is there because the ingestor accepts batch HTTP requests of up to 500 events; without batching, those would all hit Kafka as individual produce requests.

## Consumer settings (enricher)

```
group.id = enricher-v1
auto.offset.reset = earliest        # on first boot, read from the beginning
enable.auto.commit = false          # commit only after successful DB write
session.timeout.ms = 10000          # 10s for broker to detect a dead consumer
max.poll.interval.ms = 300000       # 5 min max for a single DB write cycle
partition.assignment.strategy = cooperative-sticky
```

Manual commits are the critical one here. If auto-commit were on, Kafka would advance the offset on a timer regardless of whether the DB write succeeded. A crash between the commit and the write would silently drop events. With manual commits, we only advance the offset after TimescaleDB confirms the write — giving us at-least-once delivery.

`cooperative-sticky` avoids the stop-the-world rebalance pause that the default eager strategy causes. When a new enricher instance joins, only the partitions that need to move are briefly paused — the rest keep processing.

## Delivery guarantee

At-least-once from Kafka plus idempotent DB writes effectively gives exactly-once end-to-end:

1. Read message from `events.raw`
2. Enrich and write to TimescaleDB — `ON CONFLICT (event_id, timestamp) DO NOTHING`
3. Publish to `events.enriched`
4. Commit Kafka offset

If the enricher crashes between step 2 and step 4, the message is re-delivered. The second DB insert is a no-op due to the conflict clause. The second publish to `events.enriched` is harmless because downstream consumers key on `eventId`.
