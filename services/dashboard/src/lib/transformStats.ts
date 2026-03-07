import { EventStats } from '../types';

export interface ChartRow {
  bucket: string;
  info: number;
  warn: number;
  error: number;
}

export function transformStats(stats: EventStats[]): ChartRow[] {
  const map = new Map<string, ChartRow>();

  for (const s of stats) {
    if (!map.has(s.bucket)) {
      map.set(s.bucket, { bucket: s.bucket, info: 0, warn: 0, error: 0 });
    }
    const row = map.get(s.bucket)!;
    if (s.level === 'info') row.info += s.eventCount;
    else if (s.level === 'warn') row.warn += s.eventCount;
    else if (s.level === 'error') row.error += s.eventCount;
  }

  return Array.from(map.values()).sort((a, b) => (a.bucket < b.bucket ? -1 : 1));
}
