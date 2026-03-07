import { describe, it, expect, vi } from 'vitest';
import { AlertService } from './alert.service';
import type { IAlertRepository, GqlAlert } from '../repositories/alert.repository';

const makeRepo = (alerts: GqlAlert[] = []): IAlertRepository => ({
  listAlerts: vi.fn().mockResolvedValue(alerts),
});

describe('AlertService.getAlerts', () => {
  it('passes filter through to repository', async () => {
    const repo = makeRepo();
    const svc = new AlertService(repo);

    await svc.getAlerts({ service: 'payments-api', severity: 'critical' });

    expect(repo.listAlerts).toHaveBeenCalledWith(
      expect.objectContaining({ service: 'payments-api', severity: 'critical' }),
    );
  });

  it('returns alerts from repository', async () => {
    const alerts: GqlAlert[] = [
      {
        alertId: 'a1',
        service: 'svc',
        rule: 'error_rate',
        severity: 'warning',
        firedAt: '2026-03-06T10:00:00Z',
        details: null,
      },
    ];
    const repo = makeRepo(alerts);
    const svc = new AlertService(repo);

    const result = await svc.getAlerts({});

    expect(result).toHaveLength(1);
    expect(result[0].alertId).toBe('a1');
  });
});
