import logging

import psycopg
from confluent_kafka import Producer
from prometheus_client import start_http_server

from config import load_config
from consumer import DetectorConsumer
from dispatcher import AlertDispatcher
from metrics import Metrics
from state import StateStore

logging.basicConfig(
  level=logging.INFO,
  format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

_MIGRATE_SQL = """
CREATE TABLE IF NOT EXISTS alerts (
  alert_id  UUID        PRIMARY KEY,
  service   TEXT        NOT NULL,
  rule      TEXT        NOT NULL,
  severity  TEXT        NOT NULL,
  fired_at  TIMESTAMPTZ NOT NULL,
  details   JSONB
);
CREATE INDEX IF NOT EXISTS alerts_service_fired_at ON alerts (service, fired_at DESC);
"""


def main() -> None:
  config = load_config()

  m = Metrics()
  start_http_server(config.metrics_port)
  logger.info("Metrics server listening on :%d", config.metrics_port)

  logger.info("Connecting to database...")
  db = psycopg.connect(config.database_url)
  with db.cursor() as cur:
      cur.execute(_MIGRATE_SQL)
  db.commit()
  logger.info("alerts table ready")

  producer = Producer(
    {
      "bootstrap.servers": config.kafka_brokers,
      "acks": "all",
      "enable.idempotence": "true",
      "compression.type": "snappy",
      "linger.ms": "5",
    }
  )

  state = StateStore(config.redis_url, config.zscore_baseline_size, config.alert_cooldown_minutes)
  dispatcher = AlertDispatcher(producer, config.topic_alerts, db, config.webhook_url, m)
  consumer = DetectorConsumer(config, state, dispatcher, m)

  logger.info(
    "Detector started — window=%ds error_threshold=%.0f%% zscore_threshold=%.1f",
    config.window_seconds,
    config.error_rate_threshold * 100,
    config.zscore_threshold,
  )
  consumer.run()


if __name__ == "__main__":
  main()
