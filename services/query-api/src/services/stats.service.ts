import { injectable, inject } from 'inversify';
import { TYPES } from '../types';
import type { IStatsRepository, StatsFilter, GqlEventStats } from '../repositories/stats.repository';

export type { StatsFilter, GqlEventStats };

export interface IStatsService {
  getStats(filter: StatsFilter): Promise<GqlEventStats[]>;
}

@injectable()
export class StatsService implements IStatsService {
  constructor(@inject(TYPES.StatsRepository) private statsRepo: IStatsRepository) {}

  getStats(filter: StatsFilter): Promise<GqlEventStats[]> {
    return this.statsRepo.getStats(filter);
  }
}
