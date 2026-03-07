import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LiveEventFeed } from './LiveEventFeed';

const fakeEvent = {
  eventId: 'evt-1',
  service: 'payments-api',
  environment: 'prod',
  level: 'error',
  timestamp: '2026-01-01T00:00:00Z',
  enrichment: null,
};

describe('LiveEventFeed', () => {
  it('renders event rows', () => {
    render(<LiveEventFeed events={[fakeEvent]} />);
    expect(screen.getByText('payments-api')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
  });

  it('renders empty state when no events', () => {
    render(<LiveEventFeed events={[]} />);
    expect(screen.getByText(/waiting for events/i)).toBeInTheDocument();
  });
});