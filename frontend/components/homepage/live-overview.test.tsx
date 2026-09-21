import { render, screen, within, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { LiveOverview } from './live-overview';
import { api } from '@/lib/api';

// LiveOverview is the one component that fetches the overview and lays the
// whole index out. The order it renders in IS the specification of the page.

const mockUseOverview = vi.fn();
vi.mock('@/hooks/use-homepage-overview', () => ({
  useHomepageOverview: () => mockUseOverview(),
}));

vi.mock('@/components/collaboration-example', () => ({
  CollaborationExample: () => <section data-testid="collab-example" />,
}));

vi.mock('@/lib/api', () => ({
  api: {
    getHomepageOverview: vi.fn(),
    getHomepageRooms: vi.fn(),
    getHomepageSearch: vi.fn(),
  },
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

beforeEach(() => {
  vi.clearAllMocks();
  mockUseOverview.mockReturnValue({ overview: OVERVIEW, loading: false, error: null });
  vi.mocked(api.getHomepageRooms).mockResolvedValue({
    data: { ...OVERVIEW.rooms, selected_window: '30d' },
  });
});

describe('LiveOverview layout', () => {
  it('lays the index out in the specified order', () => {
    render(<LiveOverview />);
    const order = screen
      .getAllByTestId(/^overview-section-|^collab-example$/)
      .map((node) => node.getAttribute('data-testid'));

    expect(order).toEqual([
      'overview-section-rooms',
      'overview-section-activity',
      'overview-section-previews',
      'overview-section-api',
      'overview-section-search',
      'overview-section-community',
      'collab-example',
      'overview-section-posts',
      'overview-section-closing',
    ]);
  });

  it('ends on the connection control', () => {
    render(<LiveOverview />);
    const sections = screen.getAllByTestId(/^overview-section-/);
    expect(sections[sections.length - 1]).toHaveAttribute(
      'data-testid',
      'overview-section-closing',
    );
    expect(
      screen.getByRole('link', { name: OVERVIEW.closing.connect_label }),
    ).toBeInTheDocument();
  });

  it('carries the public statistics itself, so the Data page is not a detour', () => {
    render(<LiveOverview />);
    // What /data exists to show: the search totals broken down by who searched,
    // the terms themselves, and the series behind them.
    expect(screen.getByTestId('search-top-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-recent-list')).toBeInTheDocument();
    expect(screen.getByTestId('search-series-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-metrics')).toBeInTheDocument();
  });

  it('shows a loading state instead of empty numbers while the API answers', () => {
    mockUseOverview.mockReturnValue({ overview: null, loading: true, error: null });
    render(<LiveOverview />);
    expect(screen.getByTestId('overview-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('overview-section-rooms')).not.toBeInTheDocument();
  });

  it('reports a failed read and still offers the connection control', () => {
    mockUseOverview.mockReturnValue({ overview: null, loading: false, error: 'boom' });
    render(<LiveOverview />);
    expect(screen.getByRole('alert')).toHaveTextContent('boom');
    expect(screen.getByRole('link', { name: /connect agents/i })).toHaveAttribute(
      'href',
      '/connect',
    );
  });

  // The window control belongs to the ACTIVITY sections. The All time totals
  // are not fetched with it and are not re-read when it changes, so a visitor
  // switching to 30 days cannot make Solvr's scale move.
  it('keeps the all-time totals out of the activity window', async () => {
    render(<LiveOverview />);

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
    expect(screen.getByTestId('overview-section-community').textContent).toBe(
      before,
    );
  });

  it('never invents a number of its own', () => {
    const source = read('components/homepage/live-overview.tsx');
    expect(source).not.toMatch(/(?<![\w-])\d{3,}(?![\w%-])/);
    expect(source).not.toMatch(/\.(toFixed|sort)\(/);
  });
});
