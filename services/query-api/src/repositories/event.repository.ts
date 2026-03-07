import { injectable, inject } from 'inversify';
import type { Pool } from 'pg';
import { TYPES } from '../types';

export interface GqlEvent {
  eventId: string;
  service: string;
  environment: string;
  instance: string | null;
  serviceVersion: string | null;
  level: string;
  category: string | null;
  timestamp: string;
  payload: Record<string, unknown> | null;
  enrichment: Record<string, unknown> | null;
  ingestedAt: string;
}

export interface EventsFilter {
  service?: string;
  level?: string;
  environment?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export interface IEventRepository {
  listEvents(filter: EventsFilter): Promise<GqlEvent[]>;
}

interface DbEventRow {
  event_id: string;
  service: string;
  environment: string;
  instance: string | null;
  service_version: string | null;
  level: string;
  category: string | null;
  timestamp: Date;
  payload: Record<string, unknown>;
  enrichment: Record<string, unknown>;
  ingested_at: Date;
}

function mapRow(row: DbEventRow): GqlEvent {
  return {
    eventId: row.event_id,
    service: row.service,
    environment: row.environment,
    instance: row.instance,
    serviceVersion: row.service_version,
    level: row.level,
    category: row.category,
    timestamp: row.timestamp.toISOString(),
    payload: row.payload,
    enrichment: row.enrichment,
    ingestedAt: row.ingested_at.toISOString(),
  };
}

@injectable()
export class EventRepository implements IEventRepository {
  constructor(@inject(TYPES.Pool) private pool: Pool) {}

  async listEvents(filter: EventsFilter): Promise<GqlEvent[]> {
    const now = new Date();
    const from = filter.from ?? new Date(now.getTime() - 60 * 60 * 1000).toISOString();
    const to = filter.to ?? now.toISOString();
    const limit = Math.min(filter.limit ?? 50, 500);
    const offset = filter.offset ?? 0;

    const sql = `
      SELECT event_id, service, environment, instance, service_version,
             level, category, timestamp, payload, enrichment, ingested_at
      FROM events
      WHERE ($1::text IS NULL OR service = $1)
        AND ($2::text IS NULL OR level = $2)
        AND ($3::text IS NULL OR environment = $3)
        AND timestamp >= $4::timestamptz
        AND timestamp <= $5::timestamptz
      ORDER BY timestamp DESC
      LIMIT $6 OFFSET $7
    `;
    const result = await this.pool.query<DbEventRow>(sql, [
      filter.service ?? null,
      filter.level ?? null,
      filter.environment ?? null,
      from,
      to,
      limit,
      offset,
    ]);
    return result.rows.map(mapRow);
  }
}
