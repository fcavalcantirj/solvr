import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { ApiRateLimits } from './api-rate-limits';

// The limits on /api-docs are the ones the API enforces: per-author hourly creates
// (rate_limit_config), per-IP room writes and stream tickets (router_rooms.go), and no general
// read or search limit. There is no paid tier and no bulk search endpoint.

const backend = (file: string) => readFileSync(resolve(__dirname, '../../../backend/internal/api', file), 'utf8');

describe('ApiRateLimits', () => {
  it('renders the section', () => {
    render(<ApiRateLimits />);
    expect(screen.getByText('RATE LIMITS')).toBeInTheDocument();
    expect(screen.getByText('What is limited')).toBeInTheDocument();
  });

  it('shows the create limits per author per hour', () => {
    render(<ApiRateLimits />);
    expect(screen.getByText('Create a post')).toBeInTheDocument();
    expect(screen.getByText('Create a reply')).toBeInTheDocument();
    expect(screen.getByText('3/hour')).toBeInTheDocument();
    expect(screen.getByText('6/hour')).toBeInTheDocument();
  });

  it('shows the room limits the router enforces', () => {
    const rooms = backend('router_rooms.go');
    expect(rooms).toContain('agentWriteLimit := apimiddleware.LimitByClientIP(60, time.Minute)');
    expect(rooms).toContain('streamTicketLimit := apimiddleware.LimitByClientIP(30, time.Minute)');
    render(<ApiRateLimits />);
    expect(screen.getByText('60/min')).toBeInTheDocument();
    expect(screen.getByText('30/min')).toBeInTheDocument();
  });

  it('says reads and search have no per-minute limit', () => {
    const { container } = render(<ApiRateLimits />);
    expect(container.textContent).toMatch(/Reading and searching are not rate limited/);
  });

  it('offers no paid tier and no bulk search', () => {
    const { container } = render(<ApiRateLimits />);
    expect(container.textContent).not.toMatch(/PRO TIER|\$9|COMING SOON|bulk search|10x/i);
  });

  it('tells clients to read the headers and wait for Retry-After', () => {
    const { container } = render(<ApiRateLimits />);
    expect(container.textContent).toContain('RateLimit-Remaining');
    expect(container.textContent).toContain('Retry-After');
    expect(container.textContent).toContain('429 RATE_LIMITED');
  });
});
