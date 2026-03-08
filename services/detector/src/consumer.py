import json
import logging
import threading
import time
from collections import defaultdict
from datetime import datetime, timezone
from statistics import mean

from confluent_kafka import Consumer, KafkaError

from src.config import Config
from src.dispatcher import AlertDispatcher
from src.metrics import Metrics
from src.rules import compute_window_stats, check_error_rate, check_duration_spike
from src.state import StateStore

logger = logging.getLogger(__name__)


class DetectorConsumer:
  def __init__(self, config: Config, state: StateStore, dispatcher: AlertDispatcher, metrics: Metrics):
    self._config = config
    self._state = state
    self._dispatcher = dispatcher
    self._metrics = metrics
    self._buffers: dict[str, list[dict]] = defaultdict(list)
    self._lock = threading.Lock()
    self._messages_since_tick = 0
    self._consumer = Consumer(
      {
        "bootstrap.servers": config.kafka_brokers,
        "group.id": config.kafka_group_id,
        "auto.offset.reset": "earliest",
        "enable.auto.commit": "false",
      }
    )

  def run(self) -> None:
    """
    2-thread loop + timer

    Main thread: runs kafka consumer loop:
      - `poll()` every second
        - parse json event
        - extract `source.service`
        - append into `self._buffers[service]` under a `Lock`

    Timer thread: fires every 60s via `threading.Timer`
    """
    self._consumer.subscribe([self._config.topic_enriched])
    self._schedule_tick()
    logger.info("Consumer started, subscribed to %s", self._config.topic_enriched)

    try:
      while True:
        msg = self._consumer.poll(1.0)
        if msg is None:
          continue

        if msg.error():
          if msg.error().code() != KafkaError._PARTITION_EOF:
            logger.error("Consumer error: %s", msg.error())
            self._metrics.consumer_errors.inc()
          continue

        self._metrics.messages_consumed.inc()
        self._messages_since_tick += 1

        try:
          event = json.loads(msg.value())
          service = event.get("source", {}).get("service", "unknown")
          with self._lock:
            self._buffers[service].append(event)
        except Exception as exc:
          logger.warning("Failed to parse message: %s", exc)
          self._metrics.parse_errors.inc()
    finally:
      self._consumer.close()

  def _schedule_tick(self) -> None:
    t = threading.Timer(self._config.window_seconds, self._tick)
    t.daemon = True
    t.start()

  def _tick(self) -> None:
    try:
      start = time.monotonic()
      self._evaluate()
      self._metrics.evaluation_latency.observe(time.monotonic() - start)
      self._metrics.windows_evaluated.inc()
      if self._messages_since_tick > 0:
        self._consumer.commit(asynchronous=False)
        self._messages_since_tick = 0
    except Exception as exc:
      logger.error("Evaluation tick failed: %s", exc)
    finally:
      self._schedule_tick()

  def _evaluate(self) -> None:
    """
    1. Grabs the lock, makes snapshot copy of all buffers
    2. For each service in snapshot:
      - Computes `window_start` as earliest event timestamp
      - Calls `compute_window_stats` to get summary
      - Runs `check_error_rate` -> if alert and not cooling down -> dispatch + set cooldown
      - Reads baseline from Redis -> runs `check_duration_spike` -> dispatch pattern
      - Update baseline in Redis with this window's mean duration
    3. Clear buffer after successful evaluation
    """
    window_end = datetime.now(timezone.utc)

    with self._lock:
      snapshot = {k: list(v) for k, v in self._buffers.items()}

    for service, events in snapshot.items():
      if not events:
        continue

      timestamps = [
        datetime.fromisoformat(e["timestamp"]) for e in events if e.get("timestamp")
      ]

      window_start = min(timestamps) if timestamps else window_end
      stats = compute_window_stats(service, events, window_start, window_end)

      logger.debug(
        "Window [%s] service=%s total=%d errors=%d",
        window_end.isoformat(),
        service,
        stats.total_events,
        stats.error_events,
      )

      # error rate rule
      alert = check_error_rate(
        stats,
        self._config.error_rate_threshold,
        self._config.error_rate_min_events,
      )
      if alert and not self._state.is_cooling_down(service, "error_rate"):
        logger.info("Firing error_rate alert for %s (rate=%.2f)", service, alert.details["errorRate"])
        self._dispatcher.dispatch(alert)
        self._state.set_cooldown(service, "error_rate")

      # duration spike rule
      baseline = self._state.get_baseline(service)
      alert = check_duration_spike(
        stats,
        baseline,
        self._config.zscore_threshold,
        self._config.zscore_min_windows,
      )
      if alert and not self._state.is_cooling_down(service, "duration_spike"):
        logger.info("Firing duration_spike alert for %s (z=%.2f)", service, alert.details["zScore"])
        self._dispatcher.dispatch(alert)
        self._state.set_cooldown(service, "duration_spike")

      # update baseline for next window
      if stats.duration_values:
        self._state.update_baseline(service, mean(stats.duration_values))

    # clear buffer after full evaluation
    self._buffers.clear()
