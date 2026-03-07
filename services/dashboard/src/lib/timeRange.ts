import { TimeRange } from '../types';

const RANGE_MS: Record<TimeRange, number> = {
  '15m': 15 * 60 * 1000,
  '1h': 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '24h': 24 * 60 * 60 * 1000,
};

export function toFromTo(range: TimeRange): { from: string; to: string } {
  const now = new Date();
  const from = new Date(now.getTime() - RANGE_MS[range]);
  return { from: from.toISOString(), to: now.toISOString() };
}
