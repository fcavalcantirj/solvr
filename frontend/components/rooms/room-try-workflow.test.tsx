import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import type { APIRoom } from '@/lib/api-types';

vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({ isSharing: false, shared: false, error: null, share: vi.fn() }),
}));
vi.mock('@/lib/api', () => ({
  api: { getRoomConnect: vi.fn(), getRoomShare: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { RoomHeaderActions } from './room-header-actions';
import { ConnectAgentPanel } from './connect-agent-panel';

// "Try this workflow" (idx 88) and "Start a new room" (idx 92): the link is the API's
// try_workflow_url; the client renders it, and offers nothing when the API offers nothing.

const room: APIRoom = {
  id: 'room-1',
  slug: 'ttt-room',
  display_name: 'TTT',
  tags: [],
  is_private: false,
  message_count: 3,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  last_active_at: '2026-09-01T00:00:00Z',
};

describe('Try this workflow', () => {
  it('appears in the room header with the API link', () => {
    render(<RoomHeaderActions slug="ttt-room" tryWorkflowUrl="/connect?from_room=ttt-room" />);
    expect(screen.getByRole('link', { name: /try this workflow/i })).toHaveAttribute('href', '/connect?from_room=ttt-room');
    expect(screen.getByRole('button', { name: /copy outcome/i })).toBeInTheDocument();
  });

  it('is absent, with no outcome copy, when the API offers none (a private room)', () => {
    render(<RoomHeaderActions slug="hidden" isPrivate tryWorkflowUrl={null} />);
    expect(screen.queryByRole('link', { name: /try this workflow/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /copy outcome/i })).not.toBeInTheDocument();
  });

  it('starts a finished room again from its own public task', () => {
    render(
      <ConnectAgentPanel
        room={{ ...room, archived_at: '2026-09-02T00:00:00Z' }}
        tryWorkflowUrl="/connect?from_room=ttt-room"
      />,
    );
    expect(screen.getByRole('link', { name: /start a new room/i })).toHaveAttribute('href', '/connect?from_room=ttt-room');
  });
});
