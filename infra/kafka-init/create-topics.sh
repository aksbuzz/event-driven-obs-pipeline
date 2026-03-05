#!/bin/bash
# Creates all required Kafka topics.
# Run once after `docker compose up -d kafka`

set -e

KAFKA_CONTAINER="obs-kafka"
BOOTSTRAP="kafka:29092"

wait_for_kafka() {
  echo "Waiting for Kafka to be ready..."
  for i in {1..30}; do
    if docker exec "$KAFKA_CONTAINER" kafka-topics --bootstrap-server "$BOOTSTRAP" --list &>/dev/null; then
      echo "Kafka is ready."
      return 0
    fi
    echo "  attempt $i/30..."
    sleep 2
  done
  echo "Kafka did not become ready in time."
  exit 1
}

create_topic() {
  local name=$1
  local partitions=${2:-3}
  local retention_ms=${3:-604800000}  # 7 days default

  echo "Creating topic: $name (partitions=$partitions, retention=${retention_ms}ms)"

  docker exec "$KAFKA_CONTAINER" kafka-topics \
    --bootstrap-server "$BOOTSTRAP" \
    --create \
    --if-not-exists \
    --topic "$name" \
    --partitions "$partitions" \
    --replication-factor 1 \
    --config retention.ms="$retention_ms" \
    --config cleanup.policy=delete
}

wait_for_kafka

# Raw events — high partition count for parallelism
create_topic "events.raw"       3   604800000   # 7 days

# Enriched events — downstream consumers (detector, query-api)
create_topic "events.enriched"  3   604800000   # 7 days

# Dead letter queue — long retention for debugging
create_topic "events.dlq"       1   2592000000  # 30 days

# Anomaly alerts — short retention, actioned quickly
create_topic "events.alerts"    1   86400000    # 24 hours

echo ""
echo "All topics created:"
docker exec "$KAFKA_CONTAINER" kafka-topics \
  --bootstrap-server "$BOOTSTRAP" \
  --list
