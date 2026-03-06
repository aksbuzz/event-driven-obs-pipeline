import concurrent.futures
import json
import logging

import httpx
from confluent_kafka import Producer

from src.metrics import Metrics
from src.rules import Alert

logger = logging.getLogger(__name__)


def _alert_to_dict(alert: Alert) -> dict:
  return {
    "alertId": alert.alert_id,
    "service": alert.service,
    "rule": alert.rule,
    "severity": alert.severity,
    "firedAt": alert.fired_at.isoformat(),
    "details": alert.details,
  }


class AlertDispatcher:
  def __init__(
    self,
    producer: Producer,
    topic_alerts: str,
    db_conn,
    webhook_url: str | None,
    metrics: Metrics,
  ):
    self._producer = producer
    self._topic_alerts = topic_alerts
    self._db = db_conn
    self._webhook_url = webhook_url
    self._metrics = metrics
    self._executor = concurrent.futures.ThreadPoolExecutor(
      max_workers=2, thread_name_prefix="webhook"
    )

  def dispatch(self, alert: Alert) -> None:
    payload = _alert_to_dict(alert)
    self._publish_kafka(alert, payload)
    self._insert_postgres(alert)
    if self._webhook_url:
      self._post_webhook(payload)
    self._metrics.alerts_fired.labels(rule=alert.rule, severity=alert.severity).inc()

  def _publish_kafka(self, alert: Alert, payload: dict) -> None:
    try:
      self._producer.produce(
        self._topic_alerts,
        key=alert.service,
        value=json.dumps(payload).encode(),
      )
      self._producer.poll(0)
    except Exception as exc:
      logger.error("Kafka publish failed for alert %s: %s", alert.alert_id, exc)
      self._metrics.dispatcher_errors.labels(destination="kafka").inc()
      raise

  def _insert_postgres(self, alert: Alert) -> None:
    try:
      with self._db.cursor() as cur:
        cur.execute(
          """
          INSERT INTO alerts (alert_id, service, rule, severity, fired_at, details)
          VALUES (%s, %s, %s, %s, %s, %s)
          ON CONFLICT (alert_id) DO NOTHING
          """,
          (
            alert.alert_id,
            alert.service,
            alert.rule,
            alert.severity,
            alert.fired_at,
            json.dumps(alert.details),
          ),
        )
      self._db.commit()
    except Exception as exc:
      logger.error("DB insert failed for alert %s: %s", alert.alert_id, exc)
      self._metrics.dispatcher_errors.labels(destination="db").inc()
      raise

  def _post_webhook(self, payload: dict) -> None:
    self._executor.submit(self._send_webhook, payload)

  def _send_webhook(self, payload: dict) -> None:
    try:
      with httpx.Client(timeout=5.0) as client:
        resp = client.post(self._webhook_url, json=payload)
        resp.raise_for_status()
    except Exception as exc:
      logger.warning("Webhook delivery failed: %s", exc)
      self._metrics.dispatcher_errors.labels(destination="webhook").inc()
