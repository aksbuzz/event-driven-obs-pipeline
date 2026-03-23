#!/usr/bin/env python3
"""
Load generator — simulates 3 services emitting realistic events to the ingestor.
Useful for local dev and for demoing the pipeline end-to-end.

Usage:
    pip install requests faker
    python3 scripts/load-gen.py --rate 10 --duration 60
"""

import argparse
import json
import random
import time
import uuid
from datetime import datetime, timezone

import requests
from faker import Faker

fake = Faker()

INGESTOR_URL = "http://localhost/api"  # k8s Ingress — change to http://localhost:8080 for docker-compose

SERVICES = [
    {
        "service": "payments-api",
        "environment": "production",
        "version": "2.4.1",
        "endpoints": ["/v1/charge", "/v1/refund", "/v1/subscription"],
        "error_rate": 0.05,
    },
    {
        "service": "auth-service",
        "environment": "production",
        "version": "1.8.0",
        "endpoints": ["/v1/login", "/v1/logout", "/v1/token/refresh"],
        "error_rate": 0.02,
    },
    {
        "service": "inventory-api",
        "environment": "staging",
        "version": "3.1.0-rc1",
        "endpoints": ["/v1/products", "/v1/stock", "/v1/reservations"],
        "error_rate": 0.08,
    },
]

LEVELS = ["debug", "info", "info", "info", "warn"]  # weighted; errors come from error_rate only


def make_event(svc: dict) -> dict:
    is_error = random.random() < svc["error_rate"]
    level = "error" if is_error else random.choice(LEVELS)
    endpoint = random.choice(svc["endpoints"])
    duration_ms = random.lognormvariate(4.5, 0.8)  # realistic latency distribution
    status_code = random.choice([500, 503]) if is_error else random.choice([200, 200, 200, 201, 204])

    return {
        "eventId": str(uuid.uuid4()),
        "schemaVersion": 1,
        "source": {
            "service": svc["service"],
            "environment": svc["environment"],
            "version": svc["version"],
            "instance": f"{svc['service']}-{random.randint(1, 3)}",
        },
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "level": level,
        "category": "log",
        "payload": {
            "message": f"{'ERROR: request failed' if is_error else 'request processed'} {endpoint}",
            "durationMs": round(duration_ms, 2),
            "statusCode": status_code,
            "traceId": str(uuid.uuid4()).replace("-", ""),
            "spanId": str(uuid.uuid4())[:16].replace("-", ""),
            "tags": {
                "endpoint": endpoint,
                "method": random.choice(["GET", "POST", "PUT", "DELETE"]),
                "clientIp": fake.ipv4_public(),
            },
            "numeric": {
                "durationMs": round(duration_ms, 2),
                "requestSizeBytes": random.randint(100, 10000),
                "responseSizeBytes": random.randint(50, 50000),
            },
        },
    }


def send_batch(events: list[dict]) -> None:
    try:
        resp = requests.post(
            f"{INGESTOR_URL}/v1/events/batch",
            json=events,
            timeout=5,
        )
        data = resp.json()
        print(f"  batch {len(events)} events -> accepted={data.get('accepted', '?')} rejected={data.get('rejected', '?')}")
    except Exception as e:
        print(f"  ERROR sending batch: {e}")


def main():
    parser = argparse.ArgumentParser(description="Obs Pipeline load generator")
    parser.add_argument("--rate", type=int, default=10, help="Events per second")
    parser.add_argument("--duration", type=int, default=60, help="Seconds to run (0 = forever)")
    parser.add_argument("--batch-size", type=int, default=20, help="Events per HTTP request")
    args = parser.parse_args()

    print(f"Starting load generator: {args.rate} events/sec for {args.duration or '∞'}s")
    print(f"Target: {INGESTOR_URL}")
    print(f"Services: {[s['service'] for s in SERVICES]}")
    print()

    start = time.time()
    events_sent = 0
    batch = []

    while True:
        if args.duration > 0 and time.time() - start > args.duration:
            break

        svc = random.choice(SERVICES)
        batch.append(make_event(svc))

        if len(batch) >= args.batch_size:
            send_batch(batch)
            events_sent += len(batch)
            time.sleep(len(batch) / max(args.rate, 1))  # throttle to target rate
            batch = []

    if batch:
        send_batch(batch)
        events_sent += len(batch)

    elapsed = time.time() - start
    print(f"\nDone. Sent {events_sent} events in {elapsed:.1f}s ({events_sent/elapsed:.1f} events/sec)")


if __name__ == "__main__":
    main()
