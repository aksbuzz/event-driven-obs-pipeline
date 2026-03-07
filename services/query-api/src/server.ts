import Fastify from 'fastify';
import { readFileSync } from 'fs';
import { join } from 'path';
import { container } from './container';
import { httpDurationSeconds, httpRequestsTotal } from './metrics';
import { buildResolvers } from './resolvers';
import { subscriptionResolvers } from './resolvers/subscription.resolver';
import { IAlertService } from './services/alert.service';
import { IEventService } from './services/event.service';
import { IStatsService } from './services/stats.service';
import { TYPES } from './types';
import fastifyWebsocket from '@fastify/websocket';
import mercurius from 'mercurius';

const schemaPath = join(__dirname, 'schema', 'schema.graphql');
const schema = readFileSync(schemaPath, 'utf-8');

export async function buildServer() {
  const app = Fastify({ logger: true });

  const resolvers = {
    ...buildResolvers(
      container.get<IEventService>(TYPES.EventService),
      container.get<IStatsService>(TYPES.StatsService),
      container.get<IAlertService>(TYPES.AlertService),
    ),
    ...subscriptionResolvers,
  };

  app.addHook('onRequest', async (req, reply) => {
    (req as any).startTime = process.hrtime.bigint();
  });
  app.addHook('onResponse', async (req, reply) => {
    const duration = Number(process.hrtime.bigint() - (req as any).startTime) / 1e9;
    const path = req.routeOptions?.url || req.url;
    httpRequestsTotal.inc({ method: req.method, path, status: String(reply.statusCode) });
    httpDurationSeconds.observe({ method: req.method, path }, duration);
  });
  app.addHook('onClose', async () => {
    app.log.info('Server shutting down');
  });

  app.get('/healthz', async () => ({ status: 'ok' }));

  await app.register(fastifyWebsocket, { options: { maxPayload: 1048576 } });
  await app.register(mercurius, {
    schema,
    resolvers,
    subscription: true,
    graphiql: true,
  });

  return app;
}
