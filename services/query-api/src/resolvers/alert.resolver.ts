import { IResolvers } from 'mercurius';
import type { IAlertService } from '../services/alert.service';
import type { AlertsFilter } from '../repositories/alert.repository';

export function buildAlertResolvers(alertService: IAlertService): IResolvers {
  return {
    Query: {
      alerts: (_: unknown, { filter }: { filter?: AlertsFilter }) =>
        alertService.getAlerts(filter ?? {}),
    },
  };
}
