import { injectable, inject } from 'inversify';
import { TYPES } from '../types';
import type { IAlertRepository, GqlAlert, AlertsFilter } from '../repositories/alert.repository';

export interface IAlertService {
  getAlerts(filter: AlertsFilter): Promise<GqlAlert[]>;
}

@injectable()
export class AlertService implements IAlertService {
  constructor(@inject(TYPES.AlertRepository) private alertRepo: IAlertRepository) {}

  getAlerts(filter: AlertsFilter): Promise<GqlAlert[]> {
    return this.alertRepo.listAlerts(filter);
  }
}
