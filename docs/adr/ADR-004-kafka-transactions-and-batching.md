# ADR-004: Kafka Transactions and Enricher Batching

*Accepted — 2026-03-10*

---

The enricher originally used manual offset commits for at-least-once delivery, with `ON CONFLICT DO NOTHING` on `eventId` making DB inserts idempotent. This left one gap: if the enricher crashed after publishing to `events.enriched` but before committing the offset, the message would be re-delivered and the enriched event published a second time. Downstream consumers on `events.enriched` key on `eventId`, but nothing guaranteed the duplicate wouldn't land on a different partition or confuse the detector's window stats. Kafka transactions close that gap.

## Kafka transactions in the enricher

The enricher producer is initialised with a `transactional.id` and calls `InitTransactions` at startup. Each batch is wrapped in a single transaction:

```
BeginTransaction()
  → enrich each message
  → InsertBatch to TimescaleDB
  → Produce each enriched event to events.enriched
  → Produce DLQ messages for failures
  → SendOffsetsToTransaction (commits consumer offsets atomically)
CommitTransaction()
```

`SendOffsetsToTransaction` replaces manual `CommitOffset`. The consumer offset and the producer outputs are committed atomically by the Kafka broker. Either all of them land — the DB write, the enriched event, and the offset advance — or none of them do. There is no longer a window where the enriched event is visible but the offset isn't committed (or vice versa).

The detector consumer sets `isolation.level = read_committed` so it only sees messages from committed transactions. Without this, an aborted transaction's uncommitted messages could be read before the abort is confirmed.

If the enricher crashes mid-transaction, the broker eventually times out the transaction and aborts it. The consumer offset is not advanced. On restart, the enricher re-processes the same batch from the last committed offset. The DB insert is idempotent; the Kafka produce is retried inside the new transaction.

## Enricher batching

Per-message processing had two per-message costs:

- One `INSERT` statement per DB round-trip to TimescaleDB
- One `BeginTransaction / CommitTransaction` pair per message

Both are amortised by batching 100 messages per transaction.

**DB:** `InsertBatch` uses `pgx.Batch` + `pool.SendBatch` to queue 100 parameterised `INSERT ... ON CONFLICT DO NOTHING` statements and send them in a single network round-trip. This is 1 round-trip regardless of batch size instead of N.

**Transactions:** One `BeginTransaction / CommitTransaction` wraps all 100 messages. The broker overhead for transaction coordination is paid once per 100 messages instead of once per message.

The batch size of 100 is a hardcoded default. At the current load-gen rate of 5 events/s across 3 services, a batch fills in ~20 seconds. At higher throughput (e.g. 500 events/s), a batch fills in 200ms.

## Partial batch on shutdown

When context is cancelled (SIGTERM), the in-progress partial batch is flushed before the consumer closes. This avoids leaving partially-processed messages uncommitted and requiring a full replay on the next start.

## DLQ messages are transactional

If enrichment fails for a message, the DLQ produce is inside the same transaction as the rest of the batch. The DLQ message only becomes visible to consumers when the transaction commits. If the DLQ produce itself fails, the transaction is aborted — the failing message will be reprocessed in the next batch.
