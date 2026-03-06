import os
from dataclasses import dataclass


@dataclass
class Config:
  kafka_brokers: str
  kafka_group_id: str
  topic_enriched: str
  topic_alerts: str
  database_url: str
  redis_url: str
  webhook_url: str | None
  window_seconds: int
  error_rate_threshold: float
  error_rate_min_events: int
  zscore_threshold: float
  zscore_min_windows: int
  zscore_baseline_size: int
  alert_cooldown_minutes: int
  metrics_port: int


def load_config() -> Config:
  return Config(
    kafka_brokers=os.environ.get("KAFKA_BROKERS", "localhost:9092"),
    kafka_group_id=os.environ.get("KAFKA_GROUP_ID", "detector-v1"),
    topic_enriched=os.environ.get("TOPIC_ENRICHED", "events.enriched"),
    topic_alerts=os.environ.get("TOPIC_ALERTS", "events.alerts"),
    database_url=os.environ["DATABASE_URL"],
    redis_url=os.environ.get("REDIS_URL", "redis://localhost:6379"),
    webhook_url=os.environ.get("WEBHOOK_URL"),
    window_seconds=int(os.environ.get("WINDOW_SECONDS", "60")),
    error_rate_threshold=float(os.environ.get("ERROR_RATE_THRESHOLD", "0.10")),
    error_rate_min_events=int(os.environ.get("ERROR_RATE_MIN_EVENTS", "10")),
    zscore_threshold=float(os.environ.get("ZSCORE_THRESHOLD", "3.0")),
    zscore_min_windows=int(os.environ.get("ZSCORE_MIN_WINDOWS", "5")),
    zscore_baseline_size=int(os.environ.get("ZSCORE_BASELINE_SIZE", "10")),
    alert_cooldown_minutes=int(os.environ.get("ALERT_COOLDOWN_MINUTES", "5")),
    metrics_port=int(os.environ.get("METRICS_PORT", "9091")),
  )
