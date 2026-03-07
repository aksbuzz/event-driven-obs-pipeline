import { FastifyBaseLogger } from 'fastify';
import { createServer } from 'http';
import { loadConfig } from './config';
import { registerBindings } from './container';
import { createPool } from './db';
import { startEventStream } from './kafka/event-stream';
import { register } from './metrics';
import { buildServer } from './server';

function startMetricsServer(port: number, logger: FastifyBaseLogger) {
  const metricServer = createServer(async (req, res) => {
    if (req.url === '/metrics' && req.method === 'GET') {
      try {
        const metrics = await register.metrics();
        res.writeHead(200, { 'content-type': register.contentType });
        res.end(metrics);
      } catch (err) {
        logger.error({ err }, 'Failed to collect metrics');
        res.writeHead(500);
        res.end();
      }
    } else {
      res.writeHead(404);
      res.end();
    }
  });

  metricServer.on('error', (err) => {
    logger.error({ err }, 'Metrics server error');
  });

  metricServer.listen(port, '0.0.0.0', () => {
    logger.info(`Metrics listening on :${port}`);
  });

  return metricServer;
}

async function main() {
  const config = loadConfig();
  const pool = createPool(config);

  await pool.query('SELECT 1');
  console.log(`Database connection OK`);

  registerBindings(pool);

  const app = await buildServer();
  const metricServer = startMetricsServer(config.metricsPort, app.log);

  const stopEventStream = await startEventStream(config, app.graphql.pubsub);

  await app.listen({ port: config.httpPort, host: '0.0.0.0' });
  console.log(`Query API listening on :${config.httpPort}`);

  const shutdown = async (signal: string) => {
    console.log(`${signal} received, shutting down`);
    await app.close();
    await stopEventStream();
    await pool.end();
    metricServer.close();
    process.exit(0);
  };

  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));
}

main().catch(err => {
  console.error('Fatal startup error', err);
  process.exit(1);
});
