import { injectable, inject } from 'inversify';
import type { Pool } from 'pg';
import { TYPES } from '../types';

export interface GqlAlert {
  alertId: string;
  service: string;
  rule: string;
  severity: string;
  firedAt: string;
  details: Record<string, unknown> | null;
}

export interface AlertsFilter {
  service?: string;
  severity?: string;
  rule?: string;
  from?: string;
  to?: string;
  limit?: number;
}

export interface IAlertRepository {
  listAlerts(filter: AlertsFilter): Promise<GqlAlert[]>;
}

interface DbAlertRow {
  alert_id: string;
  service: string;
  rule: string;
  severity: string;
  fired_at: Date;
  details: Record<string, unknown> | null;
}

function mapRow(row: DbAlertRow): GqlAlert {
  return {
    alertId: row.alert_id,
    service: row.service,
    rule: row.rule,
    severity: row.severity,
    firedAt: row.fired_at.toISOString(),
    details: row.details,
  };
}

@injectable()
export class AlertRepository implements IAlertRepository {
  constructor(@inject(TYPES.Pool) private pool: Pool) {}

  async listAlerts(filter: AlertsFilter): Promise<GqlAlert[]> {
    const limit = Math.min(filter.limit ?? 50, 500);

    const sql = `
      SELECT alert_id, service, rule, severity, fired_at, details
      FROM alerts
      WHERE ($1::text IS NULL OR service = $1)
        AND ($2::text IS NULL OR severity = $2)
        AND ($3::text IS NULL OR rule = $3)
        AND ($4::timestamptz IS NULL OR fired_at >= $4::timestamptz)
        AND ($5::timestamptz IS NULL OR fired_at <= $5::timestamptz)
      ORDER BY fired_at DESC
      LIMIT $6
    `;
    const result = await this.pool.query<DbAlertRow>(sql, [
      filter.service ?? null,
      filter.severity ?? null,
      filter.rule ?? null,
      filter.from ?? null,
      filter.to ?? null,
      limit,
    ]);
    return result.rows.map(mapRow);
  }
}
