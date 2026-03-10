# ADR-003: Anomaly Detector

*Accepted — 2026-03-05*

---

The detector consumes `events.enriched`, evaluates two rules per service per 1-minute window, and dispatches alerts to Kafka `events.alerts`, a Postgres `alerts` table, and an optional HTTP webhook.

## Python over Go

The ingestor and enricher are Go. The detector is Python. Go would work, but the detection logic — rolling means, standard deviations, Z-scores — is natural Python using the standard library's `statistics` module. More importantly, future rules will likely reach for `numpy`/`scipy` or a purpose-built anomaly detection library. Python keeps that door open, and it's the language data engineers are more likely to extend.

## In-process windowing over a stream framework

Faust or Kafka Streams would give native tumbling windows and built-in state stores, but that's significant complexity for two aggregation rules. Faust is also largely unmaintained (last release 2021). Instead, the consumer accumulates events into per-service in-memory lists and a `threading.Timer` drains them every 60 seconds. The lock is held only for the snapshot copy, so the Kafka poll loop is never blocked by rule computation.

The downside: buffer contents are lost on a crash. The manual-commit strategy below limits the blast radius.

## Manual Kafka commits after evaluation

`enable.auto.commit = false`. The offset is committed only after `_evaluate()` succeeds. If the detector crashes mid-window, Kafka re-delivers all messages from the last committed offset. Alert dispatch is idempotent (`ON CONFLICT (alert_id) DO NOTHING` in Postgres), so re-processing the same window is safe.

Without manual commits, auto-commit would advance the offset every ~5 seconds regardless of whether evaluation ran. A crash between commits would silently drop up to 60 seconds of events with no recovery path.

The one known gap: `alert_id` is a fresh `uuid4()` on each evaluation, so if the detector crashes after inserting to Postgres but before committing the offset, re-evaluation produces a second row with a different `alert_id`. True exactly-once would require a deterministic ID (e.g., a hash of `service + rule + window_start`). The duplicate rate is bounded by crash frequency and is acceptable for now.

## Redis for Z-score baseline and cooldown

The Z-score rule needs a rolling history of past window means. Keeping this in memory means a restart resets the baseline and the rule is blind for the first 5 windows. Redis survives restarts, and it's already in the stack.

```
detector:baseline:{service}        → JSON list of last N window means   TTL: 2h
detector:cooldown:{service}:{rule} → timestamp of last alert fired       TTL: cooldown duration
```

The 2-hour TTL on the baseline means a service that goes silent long enough has its history evicted. When it comes back, the rule warms up again rather than firing against stale data.

## Cooldown to suppress repeat alerts

Without a cooldown, a sustained error spike fires an alert every window. `ALERT_COOLDOWN_MINUTES` (default: 5) controls how long the same rule+service pair is suppressed after the first fire. It's stored as a Redis key with a TTL — when it expires, the next triggering window fires again.

## Non-blocking alert dispatch

`dispatch()` previously ran Kafka publish and Postgres insert synchronously on the timer thread. With multiple services, each with a 5-second I/O timeout, a slow broker or DB could delay the next evaluation tick by tens of seconds, causing missed windows.

`dispatch()` now puts the alert on a `queue.SimpleQueue` and returns immediately. A single daemon thread (`alert-dispatcher`) drains the queue and does all I/O: Kafka publish, Postgres insert, and the webhook. The timer thread is never blocked on network I/O.

The queue is unbounded. If the dispatcher thread falls behind (e.g. sustained broker latency), alerts queue up in memory rather than blocking evaluation. This is acceptable: the queue depth would only grow under conditions where Kafka itself is the bottleneck.

The webhook remains fire-and-forget via a `ThreadPoolExecutor` (2 workers). Failures are logged as warnings; a missed webhook has no effect on the Kafka or Postgres sinks.
