import { DerivedMetrics, EventStats } from '../types';

export function deriveMetrics(stats: EventStats[]): DerivedMetrics {
  const totalEvents = stats.reduce((sum, s) => sum + s.eventCount, 0);
  const totalErrors = stats
    .filter(s => s.level === 'error')
    .reduce((sum, s) => sum + s.eventCount, 0);
  const errorRate = totalEvents > 0 ? (totalErrors / totalEvents) * 100 : 0;
  const activeServices = new Set(stats.map(s => s.service)).size;
  return { totalEvents, errorRate, activeServices };
}
