import { Pool } from 'pg';
import type { Config } from './config';

export function createPool(config: Config): Pool {
  return new Pool({
    connectionString: config.databaseUrl,
    max: 10,
    idleTimeoutMillis: 30_000,
    connectionTimeoutMillis: 5_000,
  });
}
