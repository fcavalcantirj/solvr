import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { ApiUsageSection } from './api-usage-section';
import type { APIOverviewAPIUsage } from '@/lib/api-types';

// Aggregate API usage, as the index shows it.
//
// Every number, window, definition, caveat, bar height and note here is a
// string the API already decided. The browser sends the chosen window back
// and renders the new answer; it never sums calls, never turns a count into a
// rate, never words a caveat and never decides that a missing measurement is
// a zero.

vi.mock('@/lib/api', () => ({
  api: { getHomepageApiUsage: vi.fn() },
}));

import { api } from '@/lib/api';

const USAGE = OVERVIEW.api_usage;

// sevenDayUsage is what the API answers for ?window=7d.
const sevenDayUsage: APIOverviewAPIUsage = {
  ...USAGE,
  selected_window: '7d',
  window_options: USAGE.window_options.map((o) => ({ ...o, selected: o.value === '7d' })),
  metrics: USAGE.metrics.map((m) => ({
    ...m,
    window: 'last 7 days',
    value: m.value * 5,
    display: String(m.value * 5),
  })),
};

beforeEach(() => {
  vi.mocked(api.getHomepageApiUsage).mockReset();
  vi.mocked(api.getHomepageApiUsage).mockResolvedValue({ data: sevenDayUsage });
});

describe('ApiUsageSection', () => {
  it('renders the measured call volumes with the window each was measured over', () => {
    render(<ApiUsageSection initial={USAGE} />);

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(USAGE.heading);
    expect(screen.getByText(USAGE.intro)).toBeInTheDocument();

    const grid = screen.getByTestId('api-usage-metrics');
    for (const metric of USAGE.metrics) {
      const tile = within(grid).getByText(metric.label).closest('[data-testid="overview-metric"]');
      expect(tile).not.toBeNull();
      expect(within(tile as HTMLElement).getByText(metric.display)).toBeInTheDocument();
      expect(within(tile as HTMLElement).getByText(metric.window)).toBeInTheDocument();
      expect(within(tile as HTMLElement).getByText(metric.display)).toHaveAttribute(
        'title',
        metric.definition,
      );
    }
    expect(within(grid).getAllByTestId('overview-metric')).toHaveLength(USAGE.metrics.length);
  });

  it('states what the section aggregates and what it excludes', () => {
    render(<ApiUsageSection initial={USAGE} />);
    expect(screen.getByText(USAGE.scope_note)).toBeInTheDocument();
  });

  it('draws the series the API normalised, with a table carrying the same rows', () => {
    render(<ApiUsageSection initial={USAGE} />);

    const bars = screen.getAllByTestId('api-usage-spark-bar');
    expect(bars).toHaveLength(USAGE.series.sparkline!.points.length);
    bars.forEach((bar, i) => {
      expect(bar).toHaveStyle({ height: USAGE.series.sparkline!.points[i].height });
    });

    const table = screen.getByTestId('api-usage-series-table');
    for (const row of USAGE.series.rows) {
      expect(within(table).getByText(row.label)).toBeInTheDocument();
    }
    expect(within(table).getByText(USAGE.series.period_header)).toBeInTheDocument();
    expect(within(table).getByText(USAGE.series.count_header)).toBeInTheDocument();
  });

  it('re-reads the section from the API when a window is chosen', async () => {
    render(<ApiUsageSection initial={USAGE} />);

    fireEvent.click(screen.getByRole('button', { name: '7 days' }));

    await waitFor(() => {
      expect(api.getHomepageApiUsage).toHaveBeenCalledWith('7d');
    });
    await waitFor(() => {
      expect(screen.getByText(sevenDayUsage.metrics[0].display)).toBeInTheDocument();
    });
    // The window text came back from the API; the browser did not relabel it.
    const grid = screen.getByTestId('api-usage-metrics');
    expect(within(grid).getAllByText('last 7 days').length).toBeGreaterThan(0);
  });

  it('does not re-read when the window already selected is clicked again', () => {
    render(<ApiUsageSection initial={USAGE} />);
    fireEvent.click(screen.getByRole('button', { name: '24 hours' }));
    expect(api.getHomepageApiUsage).not.toHaveBeenCalled();
  });

  it('reports a failed re-read instead of showing a stale window as current', async () => {
    vi.mocked(api.getHomepageApiUsage).mockRejectedValue(new Error('api-usage is down'));
    render(<ApiUsageSection initial={USAGE} />);

    fireEvent.click(screen.getByRole('button', { name: '30 days' }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('api-usage is down');
    });
    // The numbers on screen are still the ones the API sent for 24 hours.
    expect(screen.getByRole('button', { name: '24 hours' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('renders an unavailable measurement as the API worded it, never as a zero', () => {
    const unmeasured: APIOverviewAPIUsage = {
      ...USAGE,
      start_note: 'Solvr has not recorded a request at the API boundary yet.',
      metrics: USAGE.metrics.map((m) => ({
        ...m,
        value: 0,
        display: '—',
        unavailable: true,
        qualifier: 'not measured yet: this is unavailable rather than zero',
      })),
      series: { ...USAGE.series, sparkline: undefined, rows: [] },
    };

    render(<ApiUsageSection initial={unmeasured} />);

    const grid = screen.getByTestId('api-usage-metrics');
    expect(within(grid).getAllByText('—')).toHaveLength(unmeasured.metrics.length);
    expect(within(grid).queryByText('0')).not.toBeInTheDocument();
    expect(screen.getByText(unmeasured.start_note!)).toBeInTheDocument();
    expect(screen.queryAllByTestId('api-usage-spark-bar')).toHaveLength(0);
  });

  it('shows the caveat the API put on a figure', () => {
    render(<ApiUsageSection initial={USAGE} />);
    const qualified = USAGE.metrics.find((m) => m.qualifier)!;
    expect(screen.getByText(qualified.qualifier!)).toBeInTheDocument();
  });

  it('lists the endpoints the API named and links its documentation', () => {
    render(<ApiUsageSection initial={USAGE} />);

    const rows = screen.getAllByTestId('overview-endpoint');
    expect(rows).toHaveLength(USAGE.endpoints.length);
    rows.forEach((row, i) => {
      const endpoint = USAGE.endpoints[i];
      const text = (row.textContent ?? '').replace(/\s+/g, ' ').trim();
      expect(text).toContain(endpoint.method);
      expect(text).toContain(endpoint.path);
      expect(text).toContain(endpoint.summary);
    });

    expect(screen.getByRole('link', { name: USAGE.docs_label })).toHaveAttribute(
      'href',
      USAGE.docs_url,
    );
  });

  // The component is a dumb terminal: no arithmetic, no thresholds, no
  // vocabulary of its own for what a number means.
  it('computes nothing and invents no wording', () => {
    const source = readFileSync(
      join(process.cwd(), 'components/homepage/api-usage-section.tsx'),
      'utf8',
    );

    for (const forbidden of [
      'toLocaleString',
      'Math.',
      '.reduce(',
      'percent',
      'unavailable ?',
      'calls',
      'requests',
    ]) {
      expect(source).not.toContain(forbidden);
    }
    // It reads the endpoint it was told to read, and nothing else.
    expect(source).not.toMatch(/\/v1\//);
  });
});
