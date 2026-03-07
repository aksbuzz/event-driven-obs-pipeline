import { describe, expect, it } from 'vitest';
import { deriveMetrics } from './deriveMetrics';

describe('deriveMetrics', () => {
  it('returns zeros for empty data', () => {
    const result = deriveMetrics([]);
    expect(result).toEqual({ totalEvents: 0, errorRate: 0, activeServices: 0 });
  });

  it('counts total events across all stats', () => {
    const stats = [
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'info',
        eventCount: 10,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'error',
        eventCount: 5,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    expect(deriveMetrics(stats).totalEvents).toBe(15);
  });

  it('calculates error rate as percentage', () => {
    const stats = [
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'info',
        eventCount: 90,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'error',
        eventCount: 10,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    expect(deriveMetrics(stats).errorRate).toBeCloseTo(10);
  });

  it('counts distinct services', () => {
    const stats = [
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'info',
        eventCount: 5,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: 'b',
        service: 'svc-b',
        level: 'info',
        eventCount: 5,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: 'b',
        service: 'svc-a',
        level: 'error',
        eventCount: 2,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    expect(deriveMetrics(stats).activeServices).toBe(2);
  });
});
