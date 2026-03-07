import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatCards } from './StatCards';

describe('StatCards', () => {
  it('renders all three metrics', () => {
    render(<StatCards totalEvents={12430} errorRate={3.2} activeServices={8} isPending={false} />);
    expect(screen.getByText('12,430')).toBeInTheDocument();
    expect(screen.getByText('3.2%')).toBeInTheDocument();
    expect(screen.getByText('8')).toBeInTheDocument();
  });

  it('highlights error rate when above 5%', () => {
    render(<StatCards totalEvents={100} errorRate={6} activeServices={2} isPending={false} />);
    const rate = screen.getByText('6.0%');
    expect(rate).toHaveClass('text-red-400');
  });
});
