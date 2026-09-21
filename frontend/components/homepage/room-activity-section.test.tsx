import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { OVERVIEW } from './overview-fixture';
import { RoomActivitySection } from './room-activity-section';
import type { APIOverviewActivity, APIOverviewActivityGroup } from '@/lib/api-types';

// The activity stream renders what the API decided: the groups it grouped, the
// action each entry was given, the label each author was given, and the link
// each entry opens. The page never slices, re-words, re-groups or re-links
// anything, never decides whether there is more to load, and — the point of
// this section — never moves the list underneath somebody who is reading it.

const mockGetActivity = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    getHomepageActivity: (...args: unknown[]) => mockGetActivity(...args),
  },
}));

const group = (
  prefix: string,
  count: number,
  overrides: Partial<APIOverviewActivityGroup> = {},
): APIOverviewActivityGroup => ({
  room_slug: `${prefix}-room`,
  room_name: `${prefix} Room`,
  room_url: `/rooms/${prefix}-room`,
  time_label: '2 hours ago',
  entry_count: count,
  count_label: count === 1 ? '1 update' : `${count} updates`,
  burst_note: count > 1 ? `${count} updates in a row from this room` : undefined,
  items: Array.from({ length: count }, (_, i) => ({
    id: `message-${prefix}-${i}`,
    kind: 'message' as const,
    room_slug: `${prefix}-room`,
    room_name: `${prefix} Room`,
    room_url: `/rooms/${prefix}-room`,
    author: `${prefix}_agent_${i}`,
    author_role: 'agent',
    author_label: 'Agent',
    action: 'Posted a message',
    action_stated: false,
    excerpt: `${prefix} message ${i}`,
    is_excerpt: false,
    link_url: `/rooms/${prefix}-room#message-${i + 1}`,
    link_label: 'Open the original message',
    time_label: `${i + 1} hours ago`,
    timestamp: `2026-09-21T0${i}:00:00Z`,
  })),
  ...overrides,
});

const page = (
  prefix: string,
  overrides: Partial<APIOverviewActivity> = {},
): APIOverviewActivity => ({
  ...OVERVIEW.activity,
  groups: [group(prefix, 3), group(`${prefix}-other`, 1)],
  entry_count: 4,
  cursor: `2026-09-21T10:00:00Z-${prefix}`,
  has_more: true,
  next_offset: 6,
  load_more_url: '/v1/homepage/activity?offset=6&limit=6',
  has_new: false,
  new_count: 0,
  ...overrides,
});

const FIRST_PAGE = page('first');
const SECOND_PAGE = page('second', {
  offset: 6,
  has_more: false,
  next_offset: 0,
  load_more_url: undefined,
});

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('RoomActivitySection — what the API decided', () => {
  it('renders one block per group, with the room, the count and the burst note', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    const groups = screen.getAllByTestId('overview-activity-group');
    expect(groups).toHaveLength(2);
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);

    expect(screen.getByRole('link', { name: /first Room$/ })).toHaveAttribute(
      'href',
      '/rooms/first-room',
    );
    expect(screen.getByText('3 updates in a row from this room')).toBeInTheDocument();
    expect(screen.getByText(/1 update/)).toBeInTheDocument();
  });

  it('shows each entry under the author, label and action the API worded', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    expect(screen.getAllByText('Posted a message').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/first_agent_0/)[0]).toBeInTheDocument();
    expect(screen.getAllByText(/Agent/)[0]).toBeInTheDocument();
  });

  it('flags an unverified name and says nothing when the identity was proven', () => {
    const mixed = page('mixed', {
      groups: [
        group('mixed', 2, {
          items: [
            {
              ...group('mixed', 1).items[0],
              id: 'message-unverified',
              author_note: 'Name stated by the poster, unverified',
            },
            { ...group('mixed', 1).items[0], id: 'message-proven' },
          ],
        }),
      ],
    });
    render(<RoomActivitySection initial={mixed} />);

    expect(
      screen.getAllByText('Name stated by the poster, unverified'),
    ).toHaveLength(1);
  });

  it('renders a typed event as its summary, with no message body and the API link label', () => {
    const events = page('events', {
      groups: [
        group('events', 1, {
          items: [
            {
              id: 'event-12',
              kind: 'event',
              room_slug: 'events-room',
              room_name: 'events Room',
              room_url: '/rooms/events-room',
              author: 'executor_one',
              author_role: 'agent',
              author_label: 'Agent',
              action: 'Claimed render the board',
              action_stated: true,
              is_excerpt: false,
              link_url: '/rooms/events-room',
              link_label: 'Open the room',
              time_label: 'moments ago',
              timestamp: '2026-09-21T10:00:00Z',
            },
          ],
        }),
      ],
    });
    render(<RoomActivitySection initial={events} />);

    expect(screen.getByText('Claimed render the board')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open the room' })).toHaveAttribute(
      'href',
      '/rooms/events-room',
    );
  });

  it('carries the machine timestamp beside the API wording', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);
    const stamps = screen.getAllByText('1 hours ago');
    expect(stamps[0].closest('time')).toHaveAttribute('datetime', '2026-09-21T00:00:00Z');
  });

  it('labels an excerpt with the API note', () => {
    const excerpted = page('cut', {
      groups: [
        group('cut', 1, {
          items: [
            {
              ...group('cut', 1).items[0],
              is_excerpt: true,
              excerpt_note: 'Excerpt — the original message is 640 characters',
            },
          ],
        }),
      ],
    });
    render(<RoomActivitySection initial={excerpted} />);
    expect(
      screen.getByText('Excerpt — the original message is 640 characters'),
    ).toBeInTheDocument();
  });

  it('shows the API heading, intro, definition and the note about outcomes', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      OVERVIEW.activity.heading,
    );
    expect(screen.getByTitle(OVERVIEW.activity.definition)).toBeInTheDocument();
    expect(screen.getByText(OVERVIEW.activity.outcome_note)).toBeInTheDocument();
  });

  it('renders the API empty note when the stream is empty', () => {
    render(
      <RoomActivitySection initial={page('empty', { groups: [], entry_count: 0, has_more: false })} />,
    );
    expect(screen.getByText(OVERVIEW.activity.empty_note)).toBeInTheDocument();
  });
});

describe('RoomActivitySection — Load more', () => {
  it('appends the next page and asks the API for it by offset', async () => {
    mockGetActivity.mockResolvedValue({ data: SECOND_PAGE });
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() =>
      expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(8),
    );
    expect(mockGetActivity).toHaveBeenCalledWith(6, OVERVIEW.activity.limit);
    expect(screen.getByText('first message 0')).toBeInTheDocument();
    expect(screen.getByText('second message 0')).toBeInTheDocument();
  });

  it('hides Load more once the API says there is nothing more', async () => {
    mockGetActivity.mockResolvedValue({ data: SECOND_PAGE });
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument(),
    );
  });

  it('never offers Load more when the API said there is no more to load', () => {
    render(<RoomActivitySection initial={page('done', { has_more: false })} />);
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
  });

  it('keeps what it already has and reports the failure when Load more fails', async () => {
    mockGetActivity.mockRejectedValue(new Error('network down'));
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);
    expect(screen.getByRole('button', { name: 'Load more' })).toBeEnabled();
  });

  it('disables Load more while the request is in flight', async () => {
    let resolve: ((v: unknown) => void) | undefined;
    mockGetActivity.mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: /load/i }));
    expect(screen.getByRole('button', { name: /load/i })).toBeDisabled();

    resolve?.({ data: SECOND_PAGE });
    await waitFor(() =>
      expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(8),
    );
  });

  it('reports a non-Error rejection too', async () => {
    mockGetActivity.mockRejectedValue('something odd');
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);
  });
});

describe('RoomActivitySection — fresh activity never moves the page', () => {
  it('checks the API with its cursor and offers the API label without touching the list', async () => {
    vi.useFakeTimers();
    mockGetActivity.mockResolvedValue({
      data: page('fresh', { has_new: true, new_count: 3, new_label: '3 new updates' }),
    });

    render(<RoomActivitySection initial={FIRST_PAGE} />);
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(mockGetActivity).toHaveBeenCalledWith(0, OVERVIEW.activity.limit, FIRST_PAGE.cursor);

    expect(screen.getByRole('button', { name: '3 new updates' })).toBeInTheDocument();
    // The reader's list is exactly as it was.
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);
    expect(screen.getByText('first message 0')).toBeInTheDocument();
    expect(screen.queryByText('fresh message 0')).not.toBeInTheDocument();
  });

  it('takes the fresh entries only when the reader asks for them', async () => {
    vi.useFakeTimers();
    mockGetActivity.mockResolvedValue({
      data: page('fresh', { has_new: true, new_count: 1, new_label: '1 new update' }),
    });

    render(<RoomActivitySection initial={FIRST_PAGE} />);
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    fireEvent.click(screen.getByRole('button', { name: '1 new update' }));

    expect(screen.getByText('fresh message 0')).toBeInTheDocument();
    expect(screen.queryByText('first message 0')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '1 new update' })).not.toBeInTheDocument();
  });

  it('offers nothing when the API says nothing is new', async () => {
    vi.useFakeTimers();
    mockGetActivity.mockResolvedValue({ data: page('quiet', { has_new: false }) });

    render(<RoomActivitySection initial={FIRST_PAGE} />);
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(screen.queryByTestId('overview-activity-new')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);
  });

  it('stays quiet when the check itself fails', async () => {
    vi.useFakeTimers();
    mockGetActivity.mockRejectedValue(new Error('offline'));

    render(<RoomActivitySection initial={FIRST_PAGE} />);
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(4);
  });

  it('does not check while the page is hidden', async () => {
    vi.useFakeTimers();
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');

    render(<RoomActivitySection initial={FIRST_PAGE} />);
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(mockGetActivity).not.toHaveBeenCalled();
    visibility.mockRestore();
  });
});
