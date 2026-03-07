import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Pool } from 'pg';
import { AlertRepository } from './alert.repository';

describe('AlertRepository.listAlerts', () => {
  let pool: Pool;
  let repo: AlertRepository;

  beforeEach(() => {
    pool = { query: vi.fn() } as unknown as Pool;
    repo = new AlertRepository(pool);
  });

  it('queries the alerts table ordered by fired_at DESC', async () => {
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [] } as any);

    await repo.listAlerts({});

    const [sql] = vi.mocked(pool.query).mock.calls[0] as unknown as [string, unknown[]];
    expect(sql).toContain('FROM alerts');
    expect(sql).toContain('ORDER BY fired_at DESC');
  });

  it('maps snake_case rows to camelCase GqlAlert objects', async () => {
    const row = {
      alert_id: 'alert-uuid-1',
      service: 'payments-api',
      rule: 'error_rate',
      severity: 'critical',
      fired_at: new Date('2026-03-06T10:05:00Z'),
      details: { errorRate: 0.35 },
    };
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [row] } as any);

    const result = await repo.listAlerts({});

    expect(result[0].alertId).toBe('alert-uuid-1');
    expect(result[0].firedAt).toBe('2026-03-06T10:05:00.000Z');
    expect(result[0].details).toEqual({ errorRate: 0.35 });
  });

  it('passes service and severity filters', async () => {
    vi.mocked(pool.query).mockResolvedValueOnce({ rows: [] } as any);

    await repo.listAlerts({ service: 'auth-service', severity: 'critical' });

    const [, params] = vi.mocked(pool.query).mock.calls[0] as unknown as [string, unknown[]];
    expect(params).toContain('auth-service');
    expect(params).toContain('critical');
  });
});
