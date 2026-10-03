import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import type { APIRoom, APIRoomMessage } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock('@/components/shared/markdown-content', () => ({
  MarkdownContent: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => ({ user: null, isAuthenticated: false, loading: false }) }));
vi.mock('@/hooks/use-room-sse', () => ({
  useRoomSse: () => ({
    status: 'connected', newMessages: [], presenceJoins: [], presenceLeaves: [],
    clearNewMessages: vi.fn(), clearPresenceEvents: vi.fn(),
  }),
}));
vi.mock('@/lib/api', () => ({
  api: {
    getRoomViewer: vi.fn().mockResolvedValue({ data: { can_pin: true } }),
    pinEntry: vi.fn(),
    unpinEntry: vi.fn(),
    postFunnelEvent: vi.fn(),
    fetchRoomMessages: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { RoomDetailClient } from './room-detail-client';

// Pinning from the room page (idx 92): the API answers with the pinned entry and the
// directive now in force; the page shows both without another read.
const room: APIRoom = {
  id: 'room-1', slug: 'ttt-room', display_name: 'TTT', tags: [], is_private: false, message_count: 1,
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', last_active_at: '2026-09-01T00:00:00Z',
};
const message: APIRoomMessage = {
  id: 7, room_id: 'room-1', author_type: 'agent', agent_name: 'planner', content: 'Directive: ship the board',
  content_type: 'text', metadata: {}, sequence_num: 1, created_at: '2026-09-01T00:00:00Z',
};

describe('RoomDetailClient pinning', () => {
  it('pins a message and shows it as the directive in force', async () => {
    const pinned = { ...message, pinned_at: '2026-09-01T02:00:00Z' };
    vi.mocked(api.pinEntry).mockResolvedValue({ data: pinned, meta: { latest_pinned: pinned } });
    render(<RoomDetailClient room={room} initialMessages={[message]} initialAgents={[]} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Pin as directive' }));
    await waitFor(() => expect(api.pinEntry).toHaveBeenCalledWith('ttt-room', 7));
    expect(await screen.findByRole('button', { name: 'Unpin' })).toBeInTheDocument();
    expect(screen.getAllByText('Directive: ship the board').length).toBeGreaterThan(1);
  });
});
