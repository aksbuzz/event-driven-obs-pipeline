import { describe, it, expect, vi } from 'vitest';
import { StatsService } from './stats.service';

describe('StatsService.getStats', () => {
  it('queries events_per_minute with service and level filters', async () => {
    const mockQuery = vi.fn().mockResolvedValue({ rows: [] });
    const pool = { query: mockQuery } as any;
    const svc = new StatsService(pool);

    await svc.getStats({
      from: '2026-03-06T09:00:00Z',
      to: '2026-03-06T10:00:00Z',
      service: 'svc-a',
    });

    const [sql, params] = mockQuery.mock.calls[0] as [string, unknown[]];
    expect(sql).toContain('events_per_minute');
    expect(params).toContain('svc-a');
  });

  it('maps rows to GqlEventStats objects', async () => {
    const row = {
      bucket: new Date('2026-03-06T09:01:00Z'),
      service: 'svc-a',
      level: 'error',
      event_count: 15,
      avg_duration_ms: '120.5',
      max_duration_ms: '340.0',
    };
    const pool = { query: vi.fn().mockResolvedValue({ rows: [row] }) } as any;
    const svc = new StatsService(pool);

    const result = await svc.getStats({ from: '2026-03-06T09:00:00Z', to: '2026-03-06T10:00:00Z' });

    expect(result[0].eventCount).toBe(15);
    expect(result[0].avgDurationMs).toBeCloseTo(120.5);
    expect(result[0].bucket).toBe('2026-03-06T09:01:00.000Z');
  });
});
