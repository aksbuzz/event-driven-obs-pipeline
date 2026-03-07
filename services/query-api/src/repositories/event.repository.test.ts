import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Pool } from 'pg';
import { EventRepository } from './event.repository';

describe('EventRepository.listEvents', () => {
  let pool: Pool;
  let repo: EventRepository;

  beforeEach(() => {
    pool = { query: vi.fn() } as unknown as Pool;
    repo = new EventRepository(pool);
  });

  it('queries the events table with no filters applied', async () => {
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [] } as any);

    await repo.listEvents({});

    const [sql] = vi.mocked(pool.query).mock.calls[0] as unknown as [string, unknown[]];
    expect(sql).toContain('FROM events');
    expect(sql).toContain('ORDER BY timestamp DESC');
  });

  it('maps snake_case rows to camelCase GqlEvent objects', async () => {
    const row = {
      event_id: 'abc-123',
      schema_version: 1,
      service: 'payments-api',
      environment: 'production',
      instance: null,
      service_version: '1.2.0',
      level: 'error',
      category: 'log',
      timestamp: new Date('2026-03-06T10:00:00Z'),
      payload: { message: 'timeout' },
      enrichment: {},
      ingested_at: new Date('2026-03-06T10:00:01Z'),
    };
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [row] } as any);

    const result = await repo.listEvents({});

    expect(result).toHaveLength(1);
    expect(result[0].eventId).toBe('abc-123');
    expect(result[0].service).toBe('payments-api');
    expect(result[0].timestamp).toBe('2026-03-06T10:00:00.000Z');
    expect(result[0].ingestedAt).toBe('2026-03-06T10:00:01.000Z');
  });

  it('passes service and level filters as query parameters', async () => {
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [] } as any);

    await repo.listEvents({ service: 'payments-api', level: 'error' });

    const [, params] = vi.mocked(pool.query).mock.calls[0] as unknown as [string, unknown[]];
    expect(params).toContain('payments-api');
    expect(params).toContain('error');
  });
});
