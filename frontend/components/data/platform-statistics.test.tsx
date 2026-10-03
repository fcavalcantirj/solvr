import { render, screen, within, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { PlatformStatistics } from './platform-statistics';
import { OVERVIEW, STALE_META } from '@/components/homepage/overview-fixture';
import { api } from '@/lib/api';

// /data is the statistics page: the room statistics, API usage, search
// statistics and all-time totals that used to make the index one long page,
// read from the same GET /v1/overview and rendered by the same section components.

const mockUseOverview = vi.fn();
vi.mock('@/hooks/use-homepage-overview', () => ({
  useHomepageOverview: () => mockUseOverview(),
}));

vi.mock('@/lib/api', () => ({
  api: {
    getHomepageOverview: vi.fn(),
    getOverview: vi.fn(),
    getHomepageRooms: vi.fn(),
    getHomepageSearch: vi.fn(),
    getHomepageApiUsage: vi.fn(),
  },
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

beforeEach(() => {
  vi.clearAllMocks();
  mockUseOverview.mockReturnValue({ overview: OVERVIEW, meta: null, loading: false, error: null });
  vi.mocked(api.getHomepageRooms).mockResolvedValue({
    data: { ...OVERVIEW.rooms, selected_window: '30d' },
  });
});

describe('PlatformStatistics', () => {
  it('shows the statistics the index no longer carries, in order', () => {
    render(<PlatformStatistics />);
    const order = screen
      .getAllByTestId(/^overview-section-/)
      .map((node) => node.getAttribute('data-testid'));
    expect(order).toEqual([
      'overview-section-rooms',
      'overview-section-api',
      'overview-section-search',
      'overview-section-community',
    ]);
  });

  // Moved from live-overview.test.tsx ('carries the public statistics itself,
  // so the Data page is not a detour'): the breakdown now lives on /data.
  it('carries the search breakdown: totals by searcher, the terms, and the series', () => {
    render(<PlatformStatistics />);
    expect(screen.getByTestId('search-top-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-recent-list')).toBeInTheDocument();
    expect(screen.getByTestId('search-series-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-metrics')).toBeInTheDocument();
  });

  // Moved from live-overview.test.tsx unchanged: the window control belongs to
  // the ACTIVITY sections, so switching to 30 days cannot make Solvr's scale move.
  it('keeps the all-time totals out of the activity window', async () => {
    render(<PlatformStatistics />);

    const before = screen.getByTestId('overview-section-community').textContent;

    fireEvent.click(
      within(screen.getByTestId('overview-section-rooms')).getByRole('button', {
        name: '30 days',
      }),
    );

    await waitFor(() =>
      expect(api.getHomepageRooms).toHaveBeenCalledWith('30d'),
    );
    expect(api.getHomepageOverview).not.toHaveBeenCalled();
    expect(api.getOverview).not.toHaveBeenCalled();
    expect(screen.getByTestId('overview-section-community').textContent).toBe(
      before,
    );
  });

  it('states a degraded snapshot with the same notice the index uses', () => {
    mockUseOverview.mockReturnValue({ overview: OVERVIEW, meta: STALE_META, loading: false, error: null });
    render(<PlatformStatistics />);
    expect(screen.getByTestId('overview-stale-label')).toHaveTextContent(STALE_META.stale_label);
  });

  it('shows a loading state instead of empty numbers while the API answers', () => {
    mockUseOverview.mockReturnValue({ overview: null, meta: null, loading: true, error: null });
    render(<PlatformStatistics />);
    expect(screen.getByTestId('statistics-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('overview-section-rooms')).not.toBeInTheDocument();
  });

  it('reports a failed read instead of inventing numbers', () => {
    mockUseOverview.mockReturnValue({ overview: null, meta: null, loading: false, error: 'boom' });
    render(<PlatformStatistics />);
    expect(screen.getByRole('alert')).toHaveTextContent('boom');
    expect(screen.queryByTestId('overview-section-community')).not.toBeInTheDocument();
  });

  it('never invents a number of its own', () => {
    const source = read('components/data/platform-statistics.tsx');
    expect(source).not.toMatch(/(?<![\w-])\d{3,}(?![\w%-])/);
    expect(source).not.toMatch(/\.(toFixed|sort)\(/);
  });
});
