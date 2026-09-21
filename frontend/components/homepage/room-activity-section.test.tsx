import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { OVERVIEW } from './overview-fixture';
import { RoomActivitySection } from './room-activity-section';
import type { APIOverviewActivity } from '@/lib/api-types';

// The activity stream shows six recent public-room activities and loads more
// from the API. The page never slices a longer list client-side and never
// decides whether there is more to load — the API says so.

const mockGetActivity = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    getHomepageActivity: (...args: unknown[]) => mockGetActivity(...args),
  },
}));

const sixItems = (prefix: string): APIOverviewActivity['items'] =>
  Array.from({ length: 6 }, (_, i) => ({
    room_slug: `${prefix}-room-${i}`,
    room_name: `${prefix} Room ${i}`,
    room_url: `/rooms/${prefix}-room-${i}`,
    author: `${prefix}_agent_${i}`,
    author_role: 'agent',
    excerpt: `${prefix} message ${i}`,
    is_excerpt: false,
    message_url: `/rooms/${prefix}-room-${i}#message-${i + 1}`,
    time_label: `${i + 1} hours ago`,
  }));

const FIRST_PAGE: APIOverviewActivity = {
  ...OVERVIEW.activity,
  items: sixItems('first'),
  has_more: true,
  next_offset: 6,
  load_more_url: '/v1/homepage/activity?offset=6&limit=6',
};

const SECOND_PAGE: APIOverviewActivity = {
  ...OVERVIEW.activity,
  items: sixItems('second'),
  offset: 6,
  has_more: false,
  next_offset: 0,
  load_more_url: undefined,
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('RoomActivitySection', () => {
  it('renders exactly the items the API sent, with its links and time labels', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    const items = screen.getAllByTestId('overview-activity-item');
    expect(items).toHaveLength(6);
    expect(screen.getByText('first message 0')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /first Room 0/ })).toHaveAttribute(
      'href',
      '/rooms/first-room-0',
    );
    expect(screen.getByText('1 hours ago')).toBeInTheDocument();
  });

  it('shows the API heading, intro and definition', () => {
    render(<RoomActivitySection initial={FIRST_PAGE} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      OVERVIEW.activity.heading,
    );
    expect(screen.getByTitle(OVERVIEW.activity.definition)).toBeInTheDocument();
  });

  it('labels an excerpt with the API note', () => {
    render(<RoomActivitySection initial={OVERVIEW.activity} />);
    expect(
      screen.getByText(OVERVIEW.activity.items[0].excerpt_note!),
    ).toBeInTheDocument();
  });

  it('appends the next page when Load more is pressed', async () => {
    mockGetActivity.mockResolvedValue({ data: SECOND_PAGE });
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() =>
      expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(12),
    );
    expect(mockGetActivity).toHaveBeenCalledWith(6, 6);
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
    render(<RoomActivitySection initial={{ ...FIRST_PAGE, has_more: false }} />);
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
  });

  it('keeps what it already has and reports the failure when Load more fails', async () => {
    mockGetActivity.mockRejectedValue(new Error('network down'));
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(6);
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
      expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(12),
    );
  });

  it('reports a non-Error rejection too', async () => {
    mockGetActivity.mockRejectedValue('something odd');
    render(<RoomActivitySection initial={FIRST_PAGE} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getAllByTestId('overview-activity-item')).toHaveLength(6);
  });

  it('renders an item with no anchor without a message link', () => {
    const anchorless = {
      ...FIRST_PAGE,
      items: [{ ...FIRST_PAGE.items[0], message_url: undefined }],
    };
    render(<RoomActivitySection initial={anchorless} />);
    expect(
      screen.queryByRole('link', { name: 'Open the original message' }),
    ).not.toBeInTheDocument();
  });

  it('renders the API empty note when the stream is empty', () => {
    render(
      <RoomActivitySection
        initial={{ ...FIRST_PAGE, items: [], has_more: false }}
      />,
    );
    expect(screen.getByText(OVERVIEW.activity.empty_note)).toBeInTheDocument();
  });
});
