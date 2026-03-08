#!/bin/bash
# Creates all required Kafka topics.
# Works in two modes:
#   - Inside the Kafka container (kafka-topics available in PATH)
#   - From the host via docker exec

set -e

KAFKA_CONTAINER="obs-kafka"
BOOTSTRAP="kafka:29092"

# Auto-detect: if kafka-topics is directly available, we are inside the container
if command -v kafka-topics &>/dev/null; then
  RUN="kafka-topics"
else
  RUN="docker exec $KAFKA_CONTAINER kafka-topics"
fi

wait_for_kafka() {
  echo "Waiting for Kafka to be ready..."
  for i in {1..30}; do
    if $RUN --bootstrap-server "$BOOTSTRAP" --list &>/dev/null; then
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
  local retention_ms=${3:-604800000}

  echo "Creating topic: $name (partitions=$partitions, retention=${retention_ms}ms)"

  $RUN --bootstrap-server "$BOOTSTRAP" --create --if-not-exists --topic "$name" --partitions "$partitions" --replication-factor 1 --config retention.ms="$retention_ms" --config cleanup.policy=delete
}

wait_for_kafka

create_topic "events.raw"       3   604800000
create_topic "events.enriched"  3   604800000
create_topic "events.dlq"       1   2592000000
create_topic "events.alerts"    1   86400000

echo ""
echo "All topics created:"
$RUN --bootstrap-server "$BOOTSTRAP" --list
