import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ApiRateLimits } from './api-rate-limits';

describe('ApiRateLimits', () => {
  it('renders rate limits section', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('RATE LIMITS')).toBeInTheDocument();
    expect(screen.getByText('Fair usage for all')).toBeInTheDocument();
  });

  it('displays free tier operations', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('FREE TIER')).toBeInTheDocument();
    expect(screen.getByText('DEFAULT')).toBeInTheDocument();
  });

  it('shows free tier rate limits', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('60/min')).toBeInTheDocument();
    expect(screen.getByText('120/min')).toBeInTheDocument();
    expect(screen.getByText('10/hour')).toBeInTheDocument();
    expect(screen.getByText('10/min')).toBeInTheDocument();
  });

  it('displays Pro tier with coming soon badge', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('PRO TIER')).toBeInTheDocument();
    expect(screen.getByText('COMING SOON')).toBeInTheDocument();
    expect(screen.getByText('$9/mo')).toBeInTheDocument();
  });

  it('shows pro tier rate limits', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('600/min')).toBeInTheDocument();
    expect(screen.getByText('1200/min')).toBeInTheDocument();
    expect(screen.getByText('100/hour')).toBeInTheDocument();
    expect(screen.getByText('100/min')).toBeInTheDocument();
  });

  it('displays best practices section', () => {
    render(<ApiRateLimits />);

    expect(screen.getByText('BEST PRACTICES')).toBeInTheDocument();
    expect(screen.getByText('Cache locally')).toBeInTheDocument();
    expect(screen.getByText('Use webhooks')).toBeInTheDocument();
    expect(screen.getByText('Batch queries')).toBeInTheDocument();
  });
});
