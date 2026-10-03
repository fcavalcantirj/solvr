import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { CONNECT_START } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// A start flow seeded from a public room says so, links back to it, and attributes
// its connection_started step to that source (idx 88).

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';

const ROOM_SOURCED = {
  ...CONNECT_START,
  selected: { ...CONNECT_START.selected, source_room: 'ttt-room' },
  source: {
    kind: 'room',
    room_slug: 'ttt-room',
    title: 'Tic-tac-toe build',
    url: '/rooms/ttt-room',
    detail: 'This collaboration reuses the public task of a Solvr room.',
  },
};

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
});

describe('ConnectPanel seeded from a source', () => {
  it('shows where the task came from, with a link back', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: ROOM_SOURCED });
    render(<ConnectPanel variant="page" />);
    const banner = await screen.findByTestId('connect-source');
    expect(banner).toHaveTextContent('Tic-tac-toe build');
    expect(banner).toHaveTextContent('reuses the public task');
    expect(screen.getByRole('link', { name: /tic-tac-toe build/i })).toHaveAttribute('href', '/rooms/ttt-room');
  });

  it('attributes connection_started to the source room', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: ROOM_SOURCED });
    render(<ConnectPanel variant="page" />);
    await waitFor(() =>
      expect(api.postFunnelEvent).toHaveBeenCalledWith(
        expect.objectContaining({ event: 'connection_started', source: { kind: 'room', ref: 'ttt-room' } }),
      ),
    );
  });

  it('attributes a post-seeded flow to the post', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({
      data: {
        ...CONNECT_START,
        source: { kind: 'post', post_id: 'p-1', title: 'A post', url: '/posts/p-1', detail: 'd' },
      },
    });
    render(<ConnectPanel variant="page" />);
    await waitFor(() =>
      expect(api.postFunnelEvent).toHaveBeenCalledWith(
        expect.objectContaining({ event: 'connection_started', source: { kind: 'post', ref: 'p-1' } }),
      ),
    );
  });

  it('shows no source and sends none for an ordinary start', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
    render(<ConnectPanel variant="page" />);
    await screen.findByTestId('connect-panel');
    expect(screen.queryByTestId('connect-source')).not.toBeInTheDocument();
    await waitFor(() => expect(api.postFunnelEvent).toHaveBeenCalled());
    expect(vi.mocked(api.postFunnelEvent).mock.calls[0][0]).not.toHaveProperty('source');
  });
});
