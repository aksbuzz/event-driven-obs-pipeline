import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AlertsTable } from './AlertsTable';

const fakeAlert = {
  alertId: 'alert-1',
  service: 'payments-api',
  rule: 'error_rate',
  severity: 'critical',
  firedAt: '2026-01-01T00:00:00Z',
  details: null,
};

describe('AlertsTable', () => {
  it('renders alert rows', () => {
    render(<AlertsTable alerts={[fakeAlert]} />);
    expect(screen.getByText('payments-api')).toBeInTheDocument();
    expect(screen.getByText('error_rate')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
  });

  it('renders empty state when no alerts', () => {
    render(<AlertsTable alerts={[]} />);
    expect(screen.getByText(/no alerts/i)).toBeInTheDocument();
  });
});
