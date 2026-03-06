import json
from datetime import datetime, timezone

import redis as redis_lib


class StateStore:
  def __init__(self, redis_url: str, baseline_size: int, cooldown_minutes: int):
    self._r = redis_lib.from_url(redis_url)
    self._baseline_size = baseline_size
    self._cooldown_seconds = cooldown_minutes * 60

  def get_baseline(self, service: str) -> list[float]:
    raw = self._r.get(f"detector:baseline:{service}")
    if raw is None:
      return []

    return json.loads(raw)

  def update_baseline(self, service: str, new_mean: float) -> None:
    key = f"detector:baseline:{service}"
    baseline = self.get_baseline(service)
    baseline.append(new_mean)
    if len(baseline) > self._baseline_size:
      baseline = baseline[-self._baseline_size:]
    self._r.setex(key, 7200, json.dumps(baseline))

  def is_cooling_down(self, service: str, rule: str) -> bool:
    return self._r.exists(f"detector:cooldown:{service}:{rule}") == 1

  def set_cooldown(self, service: str, rule: str) -> None:
    self._r.setex(
      f"detector:cooldown:{service}:{rule}",
      self._cooldown_seconds,
      datetime.now(timezone.utc).isoformat(),
    )
