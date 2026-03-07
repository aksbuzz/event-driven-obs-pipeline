export interface EventStats {
  bucket: string;
  service: string;
  level: string;
  eventCount: number;
  avgDurationMs: number | null;
  maxDurationMs: number | null;
}

export interface Alert {
  alertId: string;
  service: string;
  rule: string;
  severity: string;
  firedAt: string;
  details: Record<string, unknown> | null;
}

export interface LiveEvent {
  eventId: string;
  service: string;
  environment: string;
  level: string;
  timestamp: string;
  enrichment: Record<string, unknown> | null;
}

export type TimeRange = '15m' | '1h' | '6h' | '24h';

export interface FilterState {
  service: string | null; // null = all services
  timeRange: TimeRange;
}

export interface FilterContextValue {
  filter: FilterState;
  setFilter: (f: FilterState) => void;
}

// Derived metrics computed from EventStats[]
export interface DerivedMetrics {
  totalEvents: number;
  errorRate: number; // 0-100
  activeServices: number;
}
