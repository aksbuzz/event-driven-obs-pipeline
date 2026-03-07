/**
 * Creates a single KafkaJS consumer. Publishes each message to two mercurius pubsub topics:
 *
 * EVENT_RECEIVED - global (subscriptions with no service filter)
 * EVENT_RECEIVED:{service} - service-scoped
 */

import { Kafka, logLevel } from 'kafkajs';
import { PubSub } from 'mercurius';
import type { Config } from '../config';
import { kafkaMessagesReceived } from '../metrics';
import type { GqlEvent } from '../repositories/event.repository';

export async function startEventStream(config: Config, pubsub: PubSub): Promise<void> {
  const kafka = new Kafka({
    clientId: 'query-api',
    brokers: config.kafkaBrokers.split(','),
    logLevel: logLevel.WARN,
  });

  const consumer = kafka.consumer({ groupId: config.kafkaGroupId });
  await consumer.connect();
  await consumer.subscribe({ topic: config.topicEnriched, fromBeginning: false });

  await consumer.run({
    eachMessage: async ({ message }) => {
      if (!message.value) return;

      let event: GqlEvent;
      try {
        const raw = JSON.parse(message.value.toString());
        // Map from enriched event schema to GqlEvent shape
        event = {
          eventId: raw.eventId ?? '',
          service: raw.source?.service ?? '',
          environment: raw.source?.environment ?? '',
          instance: raw.source?.instance ?? null,
          serviceVersion: raw.source?.version ?? null,
          level: raw.level ?? '',
          category: raw.category ?? null,
          timestamp: raw.timestamp ?? new Date().toISOString(),
          payload: raw.payload ?? null,
          enrichment: raw.enrichment ?? null,
          ingestedAt: raw.enrichment?.enrichedAt ?? new Date().toISOString(),
        };
      } catch {
        return; // malformed message - skip
      }

      kafkaMessagesReceived.inc();

      // Publish to global topic
      pubsub.publish({ topic: 'EVENT_RECEIVED', payload: { eventReceived: event } });
      // Publish to service-scoped topic for filtered subscriptions
      if (event.service) {
        pubsub.publish({
          topic: `EVENT_RECEIVED:${event.service}`,
          payload: { eventReceived: event },
        });
      }
    },
  });
}
