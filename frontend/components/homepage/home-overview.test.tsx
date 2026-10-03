import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { HomeOverview } from './home-overview';
import { OVERVIEW, STALE_META } from './overview-fixture';
import type { APIOverviewResponse } from '@/lib/api-types';

// HomeOverview owns the ONE read of the overview the index makes in the browser.
// It starts from what the server read (so the first HTML already carries the
// hero numbers) and refreshes once; the hero and the sections below both follow
// the refreshed answer, so a page served from a cache does not keep old numbers.

const mockGetOverview = vi.fn();
vi.mock('@/lib/api', () => ({
  api: { getOverview: () => mockGetOverview() },
}));

vi.mock('@/components/connect/connect-panel', () => ({
  ConnectPanel: () => <div data-testid="connect-panel" />,
}));

vi.mock('./live-overview', () => ({
  LiveOverview: ({ overview, loading }: { overview: unknown; loading: boolean }) => (
    <div data-testid="live-overview" data-has-overview={String(Boolean(overview))} data-loading={String(loading)} />
  ),
}));

const SERVER: APIOverviewResponse = { data: OVERVIEW, meta: STALE_META };
const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();
const heroItems = () =>
  screen
    .getAllByRole('listitem')
    .filter((li) => li.closest('[aria-label="Solvr in numbers"]'))
    .map((li) => squish(li.textContent));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('HomeOverview', () => {
  it('renders the server-read hero numbers on the first render, before the browser answers', () => {
    mockGetOverview.mockReturnValue(new Promise(() => {}));
    render(<HomeOverview initial={SERVER} />);

    expect(heroItems()).toEqual(OVERVIEW.hero_numbers.map((n) => `${n.display} ${n.label} ${n.window}`));
    expect(screen.getByTestId('live-overview')).toHaveAttribute('data-has-overview', 'true');
    expect(screen.getByTestId('live-overview')).toHaveAttribute('data-loading', 'false');
  });

  it('moves the hero numbers to the refreshed answer', async () => {
    const refreshed = {
      ...OVERVIEW,
      hero_numbers: [
        { key: 'registered_agents', value: 205, display: '205', label: 'agents connected', window: 'all time' },
      ],
    };
    mockGetOverview.mockResolvedValue({ data: refreshed, meta: STALE_META });
    render(<HomeOverview initial={SERVER} />);

    await waitFor(() => expect(heroItems()).toEqual(['205 agents connected all time']));
    expect(mockGetOverview).toHaveBeenCalledTimes(1);
  });

  it('puts the use cases right under the hero, above the live overview', () => {
    mockGetOverview.mockReturnValue(new Promise(() => {}));
    render(<HomeOverview initial={SERVER} />);

    const hero = screen.getByRole('heading', { level: 1 });
    const useCases = screen.getByTestId('use-cases-section');
    const live = screen.getByTestId('live-overview');
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(hero.compareDocumentPosition(useCases) & 4).toBeTruthy();
    expect(useCases.compareDocumentPosition(live) & 4).toBeTruthy();
  });

  it('falls back to the browser read when the server read failed', () => {
    mockGetOverview.mockReturnValue(new Promise(() => {}));
    render(<HomeOverview initial={null} />);

    expect(screen.queryByRole('list', { name: /solvr in numbers/i })).not.toBeInTheDocument();
    expect(screen.getByTestId('live-overview')).toHaveAttribute('data-loading', 'true');
    expect(mockGetOverview).toHaveBeenCalledTimes(1);
  });
});
