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
      const metrics = await register.metrics();
      res.writeHead(200, { 'content-type': register.contentType });
      res.end(metrics);
    } else {
      res.writeHead(404);
      res.end();
    }
  });

  metricServer.listen(port, '0.0.0.0', () => {
    logger.info(`Metrics listening on :${port}`);
  });
  
  return metricServer
}

async function main() {
  const config = loadConfig()
  const pool = createPool(config)

  await pool.query('SELECT 1')
  console.log(`Database connection OK`)

  registerBindings(pool)

  const app = await buildServer()
  const metricServer = startMetricsServer(config.metricsPort, app.log)

  startEventStream(config, app.graphql.pubsub).catch(err => {
    console.error('Kafka event stream error:', err)
    process.exit(1)
  })

  await app.listen({ port: config.httpPort, host: '0.0.0.0' })
  console.log(`Query API listening on :${config.httpPort}`)

  const shutdown = async (signal:string) => {
    console.log(`${signal} received, shutting down`)
    await app.close()
    await pool.end()
    metricServer.close()
    process.exit(0)
  }

  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));
}

main().catch(err => {
  console.error('Fatal startup error', err);
  process.exit(1);
});
