import type { ComponentProps } from 'react';
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW, STALE_META, HEALTHY_META_NULL_ERRORS } from './overview-fixture';
import { LiveOverview } from './live-overview';
import { api } from '@/lib/api';

// LiveOverview lays the index out below the hero, from the overview state
// HomeOverview hands it (one read of GET /v1/overview, seeded on the server).
// The order it renders in IS the specification of the page.

type OverviewState = ComponentProps<typeof LiveOverview>;
let state: OverviewState;
const renderOverview = () => render(<LiveOverview {...state} />);

vi.mock('@/components/collaboration-example', () => ({
  CollaborationExample: () => <section data-testid="collab-example" />,
}));

vi.mock('@/lib/api', () => ({
  api: {
    getHomepageOverview: vi.fn(),
    getOverview: vi.fn(),
    getHomepageRooms: vi.fn(),
    getHomepageSearch: vi.fn(),
  },
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

beforeEach(() => {
  vi.clearAllMocks();
  state = { overview: OVERVIEW, meta: null, loading: false, error: null };
  vi.mocked(api.getHomepageRooms).mockResolvedValue({
    data: { ...OVERVIEW.rooms, selected_window: '30d' },
  });
});

describe('LiveOverview layout', () => {
  it('lays the index out in the specified order', () => {
    renderOverview();
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
    renderOverview();
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
    renderOverview();
    // What /data exists to show: the search totals broken down by who searched,
    // the terms themselves, and the series behind them.
    expect(screen.getByTestId('search-top-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-recent-list')).toBeInTheDocument();
    expect(screen.getByTestId('search-series-table')).toBeInTheDocument();
    expect(screen.getByTestId('search-metrics')).toBeInTheDocument();
  });

  it('shows a loading state instead of empty numbers while the API answers', () => {
    state = { overview: null, meta: null, loading: true, error: null };
    renderOverview();
    expect(screen.getByTestId('overview-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('overview-section-rooms')).not.toBeInTheDocument();
  });

  it('reports a failed read and still offers the connection control', () => {
    state = { overview: null, meta: null, loading: false, error: 'boom' };
    renderOverview();
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
    renderOverview();

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

  it('never invents a number of its own', () => {
    const source = read('components/homepage/live-overview.tsx');
    expect(source).not.toMatch(/(?<![\w-])\d{3,}(?![\w%-])/);
    expect(source).not.toMatch(/\.(toFixed|sort)\(/);
  });
});

describe('LiveOverview meta banner', () => {
  // Replaces 'renders the meta banner when the API sends partial_errors as null'.
  // The null guard stays proven (no throw); a healthy snapshot now shows no banner,
  // because the banner's only healthy content was the removed "Updated" label.
  it('renders no banner for a healthy snapshot, even when partial_errors is null', () => {
    // A healthy Solvr once sent partial_errors: null (Go nil slice). Reading .length off
    // it threw "Cannot read properties of null" and took the whole homepage down —
    // on a perfectly healthy system. Measured against the live API 2026-09-29.
    state = { overview: OVERVIEW, meta: HEALTHY_META_NULL_ERRORS, loading: false, error: null };

    expect(() => renderOverview()).not.toThrow();
    expect(screen.queryByTestId('overview-meta-banner')).not.toBeInTheDocument();
    expect(screen.queryByTestId('overview-partial-errors')).not.toBeInTheDocument();
  });

  // Replaces 'renders the Last updated timestamp when meta is present': the label
  // was server UTC time read as local time, and the API no longer sends it.
  it('never renders an Updated label', () => {
    state = { overview: OVERVIEW, meta: STALE_META, loading: false, error: null };
    const { container } = renderOverview();

    expect(screen.queryByTestId('overview-last-updated')).not.toBeInTheDocument();
    expect(container.textContent).not.toMatch(/\bUpdated\b/);
  });

  it('shows the stale label and partial errors when a refresh degraded', () => {
    state = { overview: OVERVIEW, meta: STALE_META, loading: false, error: null };
    renderOverview();

    expect(screen.getByTestId('overview-stale-label')).toHaveTextContent(
      STALE_META.stale_label,
    );
    expect(screen.getByTestId('overview-stale-icon')).toBeInTheDocument();
    expect(screen.getByTestId('overview-stale-label')).toHaveAttribute('role', 'status');

    const errorNodes = screen.getAllByTestId('overview-partial-errors')[0];
    expect(errorNodes).toBeInTheDocument();
    for (const err of STALE_META.partial_errors ?? []) {
      expect(errorNodes).toHaveTextContent('Temporarily unavailable: ' + err);
    }
  });

  it('does not render a stale label when the snapshot is fresh', () => {
    const freshMeta = { ...STALE_META, stale: false, stale_label: '', partial_errors: [] };
    state = { overview: OVERVIEW, meta: freshMeta, loading: false, error: null };
    renderOverview();

    expect(screen.queryByTestId('overview-stale-label')).not.toBeInTheDocument();
    expect(screen.queryByTestId('overview-partial-errors')).not.toBeInTheDocument();
    // Replaces the final `overview-last-updated` assertion: a fresh snapshot
    // has nothing to report, so no banner renders at all.
    expect(screen.queryByTestId('overview-meta-banner')).not.toBeInTheDocument();
  });
});
