import { Container } from 'inversify';
import { Pool } from 'pg';

import { TYPES } from './types';
import { EventRepository } from './repositories/event.repository';
import { AlertRepository } from './repositories/alert.repository';
import { StatsRepository } from './repositories/stats.repository';
import { EventService } from './services/event.service';
import { StatsService } from './services/stats.service';
import { AlertService } from './services/alert.service';

export const container = new Container();

export function registerBindings(pool: Pool): void {
  container.bind(TYPES.Pool).toConstantValue(pool);
  container.bind(TYPES.EventRepository).to(EventRepository).inSingletonScope();
  container.bind(TYPES.AlertRepository).to(AlertRepository).inSingletonScope();
  container.bind(TYPES.StatsRepository).to(StatsRepository).inSingletonScope();
  container.bind(TYPES.EventService).to(EventService).inSingletonScope();
  container.bind(TYPES.StatsService).to(StatsService).inSingletonScope();
  container.bind(TYPES.AlertService).to(AlertService).inSingletonScope();
}
