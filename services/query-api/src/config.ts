import { z } from 'zod';

const configSchema = z.object({
  databaseUrl: z.string().min(1),
  kafkaBrokers: z.string().min(1),
  kafkaGroupId: z.string().default('query-api-v1'),
  topicEnriched: z.string().default('events.enriched'),
  httpPort: z.coerce.number().int().positive().default(4000),
  metricsPort: z.coerce.number().int().positive().default(4001),
  redisUrl: z.string().default('redis://localhost:6379'),
});

export type Config = z.infer<typeof configSchema>;

export function loadConfig(): Config {
  return configSchema.parse({
    databaseUrl: process.env.DATABASE_URL,
    kafkaBrokers: process.env.KAFKA_BROKERS,
    kafkaGroupId: process.env.KAFKA_GROUP_ID,
    topicEnriched: process.env.TOPIC_ENRICHED,
    httpPort: process.env.HTTP_PORT,
    metricsPort: process.env.METRICS_PORT,
    redisUrl: process.env.REDIS_URL,
  });
}
