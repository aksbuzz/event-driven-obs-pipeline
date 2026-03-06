import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from statistics import mean, stdev
from typing import Optional

ERROR_LEVELS = {"error", "critical"}


@dataclass
class WindowStats:
  service: str
  window_start: datetime
  window_end: datetime
  total_events: int
  error_events: int
  duration_values: list[float]


@dataclass
class Alert:
  alert_id: str
  service: str
  rule: str
  severity: str
  fired_at: datetime
  details: dict


def compute_window_stats(
  service: str,
  events: list[dict],
  window_start: datetime,
  window_end: datetime,
) -> WindowStats:
  """
  Takes a raw list of events and collapse them into a WindowStats summary
    - counts total events
    - counts errors (levels is `error` or `critical`)
    - extracts `durationMs` into plain float list
  """
  total = len(events)
  errors = sum(1 for e in events if e.get("level") in ERROR_LEVELS)

  durations = [
    float(e["payload"]["durationMs"]) for e in events if e.get("payload", {}).get("durationMs") is not None
  ]

  return WindowStats(
    service=service,
    window_start=window_start,
    window_end=window_end,
    total_events=total,
    error_events=errors,
    duration_values=durations,
  )


def check_error_rate(
  stats: WindowStats,
  threshold: float,
  min_events: int,
) -> Optional[Alert]:
  """
  Takes a `WindowStats` and checks
    - enough events available (default 10)
    - error fraction above thresold (default 10%)

    - Returns `Alert` class
  """
  if stats.total_events < min_events:
    return None

  rate = stats.error_events / stats.total_events
  if rate <= threshold:
    return None

  severity = "critical" if rate > 0.25 else "warning"

  return Alert(
    alert_id=str(uuid.uuid4()),
    service=stats.service,
    rule="error_rate",
    severity=severity,
    fired_at=datetime.now(timezone.utc),
    details={
      "windowStart": stats.window_start.isoformat(),
      "windowEnd": stats.window_end.isoformat(),
      "errorRate": round(rate, 4),
      "totalEvents": stats.total_events,
      "errorEvents": stats.error_events,
    },
  )


def check_duration_spike(
  stats: WindowStats,
  baseline: list[float],
  zscore_threshold: float,
  min_windows: int,
) -> Optional[Alert]:
  """
  Implements a rolling Z-score.
  Returns Alert if there's a spike (z > 3/0)
  """
  if len(baseline) < min_windows:
    return None

  if not stats.duration_values:
    return None

  if len(baseline) < max(min_windows, 2):
    return None

  current_mean = mean(stats.duration_values)
  baseline_mean = mean(baseline)
  baseline_std = stdev(baseline)
  if baseline_std == 0:
    return None

  z = (current_mean - baseline_mean) / baseline_std
  if z <= zscore_threshold:
    return None

  severity = "critical" if z > 5.0 else "warning"

  return Alert(
    alert_id=str(uuid.uuid4()),
    service=stats.service,
    rule="duration_spike",
    severity=severity,
    fired_at=datetime.now(timezone.utc),
    details={
      "windowStart": stats.window_start.isoformat(),
      "windowEnd": stats.window_end.isoformat(),
      "zScore": round(z, 3),
      "currentMeanMs": round(current_mean, 2),
      "baselineMeanMs": round(baseline_mean, 2),
      "windowEventCount": len(stats.duration_values),
    },
  )
