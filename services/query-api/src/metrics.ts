import { Counter, Histogram, Gauge, Registry, collectDefaultMetrics } from 'prom-client';

export const register = new Registry();
collectDefaultMetrics({ register });

export const httpRequestsTotal = new Counter({
  name: 'query_api_http_requests_total',
  help: 'Total HTTP requests',
  labelNames: ['method', 'path', 'status'] as const,
  registers: [register],
});

export const httpDurationSeconds = new Histogram({
  name: 'query_api_http_duration_seconds',
  help: 'HTTP request duration in seconds',
  labelNames: ['method', 'path', 'status'] as const,
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5],
  registers: [register],
});

export const kafkaMessagesReceived = new Counter({
  name: 'query_api_kafka_messages_received_total',
  help: 'Total Kafka messages received from events.enriched',
  registers: [register],
});

export const kafkaMessagesMalformed = new Counter({
  name: 'query_api_kafka_messages_malformed_total',
  help: 'Total malformed Kafka messages skipped',
  registers: [register],
});

export const activeSubscriptions = new Gauge({
  name: 'query_api_active_subscriptions',
  help: 'Current number of active GraphQL subscriptions',
  registers: [register],
});
