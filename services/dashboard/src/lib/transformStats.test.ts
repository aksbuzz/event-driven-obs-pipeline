import { describe, expect, it } from 'vitest';
import { transformStats } from './transformStats';

describe('transformStats', () => {
  it('returns empty array for no data', () => {
    expect(transformStats([])).toEqual([]);
  });

  it('pivots level columns per bucket', () => {
    const stats = [
      {
        bucket: '2026-01-01T00:00:00Z',
        service: 'svc',
        level: 'info',
        eventCount: 10,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: '2026-01-01T00:00:00Z',
        service: 'svc',
        level: 'error',
        eventCount: 3,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    const result = transformStats(stats);
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ info: 10, error: 3, warn: 0 });
  });

  it('aggregates multiple services in the same bucket', () => {
    const stats = [
      {
        bucket: '2026-01-01T00:00:00Z',
        service: 'svc-a',
        level: 'info',
        eventCount: 5,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: '2026-01-01T00:00:00Z',
        service: 'svc-b',
        level: 'info',
        eventCount: 8,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    const result = transformStats(stats);
    expect(result[0].info).toBe(13);
  });

  it('sorts by bucket ascending', () => {
    const stats = [
      {
        bucket: '2026-01-01T00:01:00Z',
        service: 'svc',
        level: 'info',
        eventCount: 2,
        avgDurationMs: null,
        maxDurationMs: null,
      },
      {
        bucket: '2026-01-01T00:00:00Z',
        service: 'svc',
        level: 'info',
        eventCount: 1,
        avgDurationMs: null,
        maxDurationMs: null,
      },
    ];
    const result = transformStats(stats);
    expect(result[0].bucket).toBe('2026-01-01T00:00:00Z');
    expect(result[1].bucket).toBe('2026-01-01T00:01:00Z');
  });
});
