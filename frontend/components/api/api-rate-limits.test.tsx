import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ApiRateLimits } from './api-rate-limits';

describe('ApiRateLimits', () => {
  it('renders rate limits section', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('RATE LIMITS')).toBeInTheDocument();
    expect(screen.getByText('Fair usage for all')).toBeInTheDocument();
  });

  it('displays all rate limit operations', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('Search')).toBeInTheDocument();
    expect(screen.getByText('Read')).toBeInTheDocument();
    expect(screen.getByText('Write')).toBeInTheDocument();
    expect(screen.getByText('Bulk Search')).toBeInTheDocument();
  });

  it('shows correct rate limits', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('60/min')).toBeInTheDocument();
    expect(screen.getByText('120/min')).toBeInTheDocument();
    expect(screen.getByText('10/hour')).toBeInTheDocument();
    expect(screen.getByText('10/min')).toBeInTheDocument();
  });

  it('does not show Pro tier', () => {
    render(<ApiRateLimits />);

    // Pro tier should not exist
    expect(screen.queryByText('PRO TIER')).not.toBeInTheDocument();
    expect(screen.queryByText('$29/mo')).not.toBeInTheDocument();
  });

  it('shows FREE badge', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('FREE')).toBeInTheDocument();
  });

  it('displays best practices section', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('BEST PRACTICES')).toBeInTheDocument();
    expect(screen.getByText('Cache locally')).toBeInTheDocument();
    expect(screen.getByText('Use webhooks')).toBeInTheDocument();
    expect(screen.getByText('Batch queries')).toBeInTheDocument();
  });
});
