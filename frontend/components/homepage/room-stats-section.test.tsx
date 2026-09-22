import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { OVERVIEW, OFFLINE_OVERVIEW } from './overview-fixture';
import { RoomStatsSection } from './room-stats-section';
import type { APIOverviewRooms } from '@/lib/api-types';

// The public room statistics and their shared time-window selector.
//
// Everything asserted here is an API string or an API decision. The component
// may not decide which windows exist, which one is selected, what a metric
// means, whether a number is missing, or what "now" is — it re-reads the
// section from the API and renders the answer.

vi.mock('@/lib/api', () => ({
  api: { getHomepageRooms: vi.fn() },
}));

import { api } from '@/lib/api';

const ROOMS = OVERVIEW.rooms;

// sevenDayRooms is what the API answers for ?window=7d.
const sevenDayRooms: APIOverviewRooms = {
  ...ROOMS,
  selected_window: '7d',
  window_options: ROOMS.window_options.map((o) => ({ ...o, selected: o.value === '7d' })),
  metrics: ROOMS.metrics.map((m) => ({
    ...m,
    window: 'last 7 days',
    value: m.value * 3,
    display: String(m.value * 3),
  })),
};

beforeEach(() => {
  vi.mocked(api.getHomepageRooms).mockReset();
  vi.mocked(api.getHomepageRooms).mockResolvedValue({ data: sevenDayRooms });
});

describe('RoomStatsSection', () => {
  it('renders the API heading, scope and every metric it was sent', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(ROOMS.heading);
    expect(screen.getByText(ROOMS.scope_label)).toBeInTheDocument();
    expect(screen.getByText(ROOMS.scope_note)).toBeInTheDocument();

    for (const metric of [...ROOMS.presence_metrics, ...ROOMS.metrics]) {
      expect(screen.getByText(metric.label)).toBeInTheDocument();
    }
  });

  it('shows the window and the definition of every number, not just the number', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const metrics = screen.getAllByTestId('overview-metric');
    expect(metrics).toHaveLength(ROOMS.presence_metrics.length + ROOMS.metrics.length);

    const all = [...ROOMS.presence_metrics, ...ROOMS.metrics];
    metrics.forEach((node, i) => {
      expect(node.textContent?.replace(/\s+/g, ' ')).toContain(all[i].window);
      expect(within(node).getByTitle(all[i].definition)).toBeInTheDocument();
    });
  });

  it('separates the Now figures from the windowed ones under the API headings', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const presence = screen.getByTestId('overview-presence-metrics');
    expect(within(presence).getAllByTestId('overview-metric')).toHaveLength(
      ROOMS.presence_metrics.length,
    );
    for (const metric of ROOMS.presence_metrics) {
      expect(within(presence).getByText(metric.label)).toBeInTheDocument();
    }

    expect(screen.getByText(ROOMS.presence_heading)).toBeInTheDocument();
    expect(screen.getByText(ROOMS.presence_note)).toBeInTheDocument();
    expect(screen.getByText(ROOMS.window_heading)).toBeInTheDocument();
  });

  it('offers exactly the windows the API sent, with the API selection marked', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const group = screen.getByRole('group', { name: ROOMS.window_label });
    const buttons = within(group).getAllByRole('button');

    expect(buttons.map((b) => b.textContent)).toEqual(
      ROOMS.window_options.map((o) => o.label),
    );
    buttons.forEach((button, i) => {
      expect(button).toHaveAttribute(
        'aria-pressed',
        String(ROOMS.window_options[i].selected),
      );
    });
  });

  it('re-reads the section from the API when another window is chosen', async () => {
    render(<RoomStatsSection initial={ROOMS} />);

    fireEvent.click(screen.getByRole('button', { name: '7 days' }));

    expect(api.getHomepageRooms).toHaveBeenCalledWith('7d');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '7 days' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
    });

    // The new numbers and their new window labels come from the API answer.
    const conversation = sevenDayRooms.metrics[0];
    expect(screen.getByText(conversation.display)).toBeInTheDocument();
    expect(screen.getAllByText('last 7 days').length).toBe(sevenDayRooms.metrics.length);
  });

  it('never re-words a presence metric when the window changes', async () => {
    render(<RoomStatsSection initial={ROOMS} />);

    fireEvent.click(screen.getByRole('button', { name: '30 days' }));

    await waitFor(() => expect(api.getHomepageRooms).toHaveBeenCalledWith('30d'));
    const presence = screen.getByTestId('overview-presence-metrics');
    const windows = within(presence)
      .getAllByTestId('overview-metric')
      .map((node) => within(node).getByText(/^now$/).textContent);
    expect(windows).toEqual(sevenDayRooms.presence_metrics.map((m) => m.window));
  });

  it('reports a failed window read and keeps showing the numbers it has', async () => {
    vi.mocked(api.getHomepageRooms).mockRejectedValue(new Error('network is down'));

    render(<RoomStatsSection initial={ROOMS} />);
    fireEvent.click(screen.getByRole('button', { name: '7 days' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('network is down');
    expect(screen.getByText(ROOMS.metrics[0].display)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '24 hours' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('does not re-read anything when the selected window is clicked again', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    fireEvent.click(screen.getByRole('button', { name: '24 hours' }));

    expect(api.getHomepageRooms).not.toHaveBeenCalled();
  });

  it('falls back to its own wording when the failure carries no message', async () => {
    vi.mocked(api.getHomepageRooms).mockRejectedValue('not an Error');

    render(<RoomStatsSection initial={ROOMS} />);
    fireEvent.click(screen.getByRole('button', { name: '7 days' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not read the room statistics',
    );
  });

  it('shows the qualifier the API attached to a number', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const qualified = [...ROOMS.presence_metrics, ...ROOMS.metrics].filter((m) => m.qualifier);
    expect(qualified.length).toBeGreaterThan(0);
    for (const metric of qualified) {
      expect(screen.getByText(metric.qualifier!)).toBeInTheDocument();
    }
  });

  it('renders an unavailable number as the API worded it, never as a zero', () => {
    const unavailable: APIOverviewRooms = {
      ...ROOMS,
      metrics: ROOMS.metrics.map((m) =>
        m.key === 'rooms_with_two_way_exchanges'
          ? {
              ...m,
              value: 0,
              display: '—',
              unavailable: true,
              qualifier: 'not measured yet',
            }
          : m,
      ),
    };

    render(<RoomStatsSection initial={unavailable} />);

    expect(screen.getByText('—')).toBeInTheDocument();
    expect(screen.getByText('not measured yet')).toBeInTheDocument();
    const node = screen.getByText('ROOMS WITH TWO-WAY EXCHANGES').closest('[data-testid="overview-metric"]');
    expect(node?.textContent).not.toMatch(/\b0\b/);
  });

  it('draws the sparkline from the heights the API normalised', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const bars = screen.getAllByTestId('overview-spark-bar');
    expect(bars).toHaveLength(ROOMS.sparkline!.points.length);
    expect(bars.map((b) => b.style.height)).toEqual(['0%', '50%', '100%']);
    expect(screen.getByText(ROOMS.sparkline!.label)).toBeInTheDocument();
  });

  it('omits the sparkline entirely when the API sent none', () => {
    render(<RoomStatsSection initial={{ ...ROOMS, sparkline: undefined }} />);
    expect(screen.queryByTestId('overview-spark-bar')).not.toBeInTheDocument();
  });

  it('renders without an intro when the API sent none', () => {
    render(<RoomStatsSection initial={{ ...ROOMS, intro: '' }} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(ROOMS.heading);
    expect(screen.queryByText(ROOMS.intro)).not.toBeInTheDocument();
  });

  it('links on to all rooms with the API label', () => {
    render(<RoomStatsSection initial={ROOMS} />);
    const link = screen.getByRole('link', { name: ROOMS.rooms_label });
    expect(link).toHaveAttribute('href', '/rooms');
  });

  // --- Task 16: green live marker and offline fallback ---

  it('renders a green live marker with text when agents are online', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    const marker = screen.getByTestId('overview-live-marker');
    expect(marker).toBeInTheDocument();

    const dot = screen.getByTestId('overview-live-dot');
    expect(dot).toHaveClass('bg-green-700', 'dark:bg-green-400', 'animate-pulse');

    const label = screen.getByTestId('overview-live-label');
    expect(label).toHaveTextContent(ROOMS.live_marker!.label);
  });

  it('renders a grey dot and offline text when no agents are online', () => {
    render(<RoomStatsSection initial={OFFLINE_OVERVIEW.rooms} />);

    const marker = screen.getByTestId('overview-live-marker');
    expect(marker).toBeInTheDocument();

    const dot = screen.getByTestId('overview-live-dot');
    expect(dot).toHaveClass('bg-muted');
    expect(dot).not.toHaveClass('animate-pulse');

    const label = screen.getByTestId('overview-live-label');
    expect(label).toHaveTextContent(OFFLINE_OVERVIEW.rooms.live_marker!.label);
  });

  it('shows recent completed collaborations when no agents are online', () => {
    render(<RoomStatsSection initial={OFFLINE_OVERVIEW.rooms} />);

    const section = screen.getByTestId('overview-recent-collaborations');
    expect(section).toBeInTheDocument();

    for (const room of OFFLINE_OVERVIEW.rooms.recent_collaborations!) {
      const link = screen.getByRole('link', { name: room.display_name });
      expect(link).toHaveAttribute('href', room.room_url);
      expect(screen.getByText(room.message_count_label)).toBeInTheDocument();
      if (room.purpose) {
        expect(screen.getByText(room.purpose)).toBeInTheDocument();
      }
      expect(screen.getByText(room.last_activity_label)).toBeInTheDocument();
    }
  });

  it('omits the live marker when the API did not send one', () => {
    const withoutMarker: APIOverviewRooms = {
      ...ROOMS,
      live_marker: undefined,
    };
    render(<RoomStatsSection initial={withoutMarker} />);

    expect(screen.queryByTestId('overview-live-marker')).not.toBeInTheDocument();
  });

  it('omits recent collaborations when agents are online', () => {
    render(<RoomStatsSection initial={ROOMS} />);

    expect(screen.queryByTestId('overview-recent-collaborations')).not.toBeInTheDocument();
  });
});
