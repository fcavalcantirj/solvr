import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { OVERVIEW } from './overview-fixture';
import { SearchStatsSection } from './search-stats-section';
import type { APIOverviewSearch } from '@/lib/api-types';

// What people and agents search for.
//
// The component renders API strings and re-reads the section when a window is
// chosen. It never decides what a searcher type means, whether a term may be
// shown, how a link is built, or how tall a bar is — every one of those is an
// API answer, and these tests exist to keep it that way.

vi.mock('@/lib/api', () => ({
  api: { getHomepageSearch: vi.fn() },
}));

import { api } from '@/lib/api';

const SEARCH = OVERVIEW.search;

// sevenDaySearch is what the API answers for ?window=7d.
const sevenDaySearch: APIOverviewSearch = {
  ...SEARCH,
  selected_window: '7d',
  window_options: SEARCH.window_options.map((o) => ({ ...o, selected: o.value === '7d' })),
  metrics: SEARCH.metrics.map((m) => ({
    ...m,
    window: 'last 7 days',
    value: m.value * 4,
    display: String(m.value * 4),
  })),
  top: {
    ...SEARCH.top,
    rows: [{ ...SEARCH.top.rows[0], query: 'seven day term', search_url: '/posts?q=seven+day+term' }],
  },
};

beforeEach(() => {
  vi.mocked(api.getHomepageSearch).mockReset();
  vi.mocked(api.getHomepageSearch).mockResolvedValue({ data: sevenDaySearch });
});

describe('SearchStatsSection', () => {
  it('renders the API heading, intro and the four totals it was sent', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(SEARCH.heading);
    expect(screen.getByText(SEARCH.intro)).toBeInTheDocument();

    const metrics = screen.getByTestId('search-metrics');
    for (const metric of SEARCH.metrics) {
      expect(within(metrics).getByText(metric.label)).toBeInTheDocument();
      expect(within(metrics).getByText(metric.display)).toBeInTheDocument();
    }
  });

  it('shows the window and the definition of every number', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    const grid = screen.getByTestId('search-metrics');
    expect(within(grid).getAllByTestId('overview-metric')).toHaveLength(SEARCH.metrics.length);

    for (const metric of SEARCH.metrics) {
      const tile = within(grid).getByText(metric.label).closest('[data-testid="overview-metric"]');
      expect(tile).not.toBeNull();
      expect(within(tile as HTMLElement).getByText(metric.window)).toBeInTheDocument();
      expect(within(tile as HTMLElement).getByText(metric.display)).toHaveAttribute(
        'title',
        metric.definition,
      );
    }
  });

  it('states the count-only share on the total rather than leaving it implied', () => {
    render(<SearchStatsSection initial={SEARCH} />);
    expect(screen.getByText(SEARCH.metrics[0].qualifier!)).toBeInTheDocument();
  });

  it('shows monitoring apart from the totals, with its own definition', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    const monitoring = screen.getByTestId('search-monitoring');
    expect(within(monitoring).getByText(SEARCH.monitoring!.label)).toBeInTheDocument();
    expect(within(monitoring).getByText(SEARCH.monitoring!.display)).toBeInTheDocument();
    expect(within(monitoring).getByText(SEARCH.monitoring!.definition)).toBeInTheDocument();
  });

  it('renders the publishing rule the API states', () => {
    render(<SearchStatsSection initial={SEARCH} />);
    expect(screen.getByText(SEARCH.privacy_note)).toBeInTheDocument();
  });

  it('links every top term to canonical Posts search with no type filter', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    const table = screen.getByTestId('search-top-table');
    for (const row of SEARCH.top.rows) {
      const link = within(table).getByRole('link', { name: new RegExp(row.query, 'i') });
      expect(link).toHaveAttribute('href', row.search_url);
      expect(link.getAttribute('href')).toContain('/posts?q=');
      expect(link.getAttribute('href')).not.toContain('type=');
      expect(within(table).getByText(row.count_label)).toBeInTheDocument();
      expect(within(table).getByText(row.with_results_label)).toBeInTheDocument();
    }

    expect(within(table).getByText(SEARCH.top.query_header)).toBeInTheDocument();
    expect(within(table).getByText(SEARCH.top.count_header)).toBeInTheDocument();
    expect(within(table).getByText(SEARCH.top.with_results_header)).toBeInTheDocument();
  });

  it('says how many terms were withheld', () => {
    render(<SearchStatsSection initial={SEARCH} />);
    expect(screen.getByText(SEARCH.top.withheld_note!)).toBeInTheDocument();
  });

  it('shows no withheld note when the API sent none', () => {
    const noneWithheld: APIOverviewSearch = {
      ...SEARCH,
      top: { ...SEARCH.top, withheld_note: undefined },
    };
    render(<SearchStatsSection initial={noneWithheld} />);
    expect(screen.queryByTestId('search-withheld-note')).not.toBeInTheDocument();
  });

  it('shows the recent list with its relative time and searcher type', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    const recent = screen.getByTestId('search-recent-list');
    for (const row of SEARCH.recent.rows) {
      const item = within(recent).getByText(row.query).closest('li');
      expect(item).not.toBeNull();
      expect(within(item as HTMLElement).getByText(row.searcher_label)).toBeInTheDocument();
      expect(within(item as HTMLElement).getByText(row.time_label)).toBeInTheDocument();
      expect(within(item as HTMLElement).getByText(row.results_label)).toBeInTheDocument();
    }

    // The privacy floor is stated on the list, not left to the reader.
    expect(within(recent).getByText(SEARCH.recent.definition)).toBeInTheDocument();
  });

  it('never invents a searcher type: it renders the recorded label', () => {
    render(<SearchStatsSection initial={SEARCH} />);
    const recent = screen.getByTestId('search-recent-list');
    expect(within(recent).getByText('Anonymous')).toBeInTheDocument();
    expect(within(recent).queryByText('Human')).not.toBeInTheDocument();
  });

  it('draws the chart from API heights and offers the same data as a table', () => {
    render(<SearchStatsSection initial={SEARCH} />);

    const bars = screen.getAllByTestId('search-spark-bar');
    expect(bars).toHaveLength(SEARCH.series.sparkline!.points.length);
    bars.forEach((bar, i) => {
      expect(bar).toHaveStyle({ height: SEARCH.series.sparkline!.points[i].height });
    });

    const table = screen.getByTestId('search-series-table');
    expect(within(table).getByText(SEARCH.series.period_header)).toBeInTheDocument();
    expect(within(table).getByText(SEARCH.series.count_header)).toBeInTheDocument();
    for (const row of SEARCH.series.rows) {
      expect(within(table).getByText(row.label)).toBeInTheDocument();
    }
    expect(within(table).getByText(SEARCH.series.table_caption)).toBeInTheDocument();
  });

  it('re-reads the section from the API when a window is chosen', async () => {
    render(<SearchStatsSection initial={SEARCH} />);

    fireEvent.click(screen.getByRole('button', { name: '7 days' }));

    await waitFor(() => {
      expect(screen.getByText('seven day term')).toBeInTheDocument();
    });
    expect(api.getHomepageSearch).toHaveBeenCalledWith('7d');
    expect(screen.getByText('480')).toBeInTheDocument();
    expect(screen.getAllByText('last 7 days').length).toBeGreaterThan(0);
  });

  it('does not re-read the window already selected', () => {
    render(<SearchStatsSection initial={SEARCH} />);
    fireEvent.click(screen.getByRole('button', { name: '24 hours' }));
    expect(api.getHomepageSearch).not.toHaveBeenCalled();
  });

  it('shows an error and keeps the previous answer when the re-read fails', async () => {
    vi.mocked(api.getHomepageSearch).mockRejectedValue(new Error('network down'));
    render(<SearchStatsSection initial={SEARCH} />);

    fireEvent.click(screen.getByRole('button', { name: '30 days' }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('network down');
    });
    expect(
      within(screen.getByTestId('search-top-table')).getByText(SEARCH.top.rows[0].query),
    ).toBeInTheDocument();
  });

  it('renders the API empty notes rather than an empty section', () => {
    const empty: APIOverviewSearch = {
      ...SEARCH,
      monitoring: undefined,
      series: { ...SEARCH.series, sparkline: undefined, rows: [] },
      top: { ...SEARCH.top, rows: [], withheld_note: undefined },
      recent: { ...SEARCH.recent, rows: [] },
    };
    render(<SearchStatsSection initial={empty} />);

    expect(screen.getByText(SEARCH.top.empty_note)).toBeInTheDocument();
    expect(screen.getByText(SEARCH.recent.empty_note)).toBeInTheDocument();
    expect(screen.queryByTestId('search-spark-bar')).not.toBeInTheDocument();
    expect(screen.queryByTestId('search-series-table')).not.toBeInTheDocument();
    expect(screen.queryByTestId('search-monitoring')).not.toBeInTheDocument();
  });
});
