import { injectable, inject } from 'inversify';
import type { Pool } from 'pg';
import { TYPES } from '../types';

export interface StatsFilter {
  service?: string;
  level?: string;
  from: string;
  to: string;
}

export interface GqlEventStats {
  bucket: string;
  service: string;
  level: string;
  eventCount: number;
  avgDurationMs: number | null;
  maxDurationMs: number | null;
}

export interface IStatsService {
  getStats(filter: StatsFilter): Promise<GqlEventStats[]>;
}

interface DbStatsRow {
  bucket: Date;
  service: string;
  level: string;
  event_count: number;
  avg_duration_ms: string | null;
  max_duration_ms: string | null;
}

// StatsService queries `events_per_minute` continuous aggregate directly
// No separate stats repository - CAgg is read-only view
@injectable()
export class StatsService implements IStatsService {
  constructor(@inject(TYPES.Pool) private pool: Pool) {}

  async getStats(filter: StatsFilter): Promise<GqlEventStats[]> {
    const sql = `
      SELECT bucket, service, level, event_count, avg_duration_ms, max_duration_ms
      FROM events_per_minute
      WHERE ($1::text IS NULL OR service = $1)
        AND ($2::text IS NULL OR level = $2)
        AND bucket >= $3::timestamptz
        AND bucket <= $4::timestamptz
      ORDER BY bucket ASC
    `;
    const result = await this.pool.query<DbStatsRow>(sql, [
      filter.service ?? null,
      filter.level ?? null,
      filter.from,
      filter.to,
    ]);
    return result.rows.map(row => ({
      bucket: row.bucket.toISOString(),
      service: row.service,
      level: row.level,
      eventCount: row.event_count,
      avgDurationMs: row.avg_duration_ms != null ? parseFloat(row.avg_duration_ms) : null,
      maxDurationMs: row.max_duration_ms != null ? parseFloat(row.max_duration_ms) : null,
    }));
  }
}
