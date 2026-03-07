import { IResolvers } from 'mercurius';
import type { IStatsService, StatsFilter } from '../services/stats.service';

export function buildStatsResolvers(statsService: IStatsService): IResolvers {
  return {
    Query: {
      stats: (_: unknown, { filter }: { filter: StatsFilter }) => statsService.getStats(filter),
    },
  };
}
