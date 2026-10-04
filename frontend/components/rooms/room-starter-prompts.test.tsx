import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RoomStarterPrompts } from './room-starter-prompts';
import type { APIRoom } from '@/lib/api-types';

// When a human creates a room directly, they land on its page flagged
// ?created=1. RoomStarterPrompts then shows the two copyable prompts — planner
// and executor — tied to the real room, both fetched from the API-owned connect
// contract. On any other room view it renders nothing and fetches nothing.

vi.mock('@/lib/api', () => ({
  api: {
    getRoomConnect: vi.fn(),
  },
}));

import { api } from '@/lib/api';

const room = {
  slug: 'debug-the-parser',
  display_name: 'Debug the parser',
  is_private: false,
  message_count: 0,
  archived_at: null,
} as unknown as APIRoom;

function envelope(role: 'planner' | 'executor') {
  return {
    data: {
      instruction_version: 'v1',
      room_slug: room.slug,
      room_url: 'https://solvr.dev/rooms/debug-the-parser',
      private: false,
      task: '',
      prompt: {
        text: `${role.toUpperCase()} PROMPT for debug-the-parser`,
        segments: [{ kind: 'text' as const, text: `${role.toUpperCase()} PROMPT for debug-the-parser` }],
        word_count: 4,
      },
      role,
    },
  };
}

describe('RoomStarterPrompts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  it('renders nothing and fetches nothing when the room was not just created', () => {
    const { container } = render(<RoomStarterPrompts room={room} justCreated={false} />);
    expect(container).toBeEmptyDOMElement();
    expect(api.getRoomConnect).not.toHaveBeenCalled();
  });

  it('tells the human the executor prompt goes into every other agent, not into a fixed pair', async () => {
    vi.mocked(api.getRoomConnect).mockImplementation(async (_slug: string, role = 'collaborator') =>
      envelope(role as 'planner' | 'executor'),
    );

    render(<RoomStarterPrompts room={room} justCreated={true} />);

    const instructions = await screen.findByTestId('starter-instructions');
    expect(instructions).toHaveTextContent('Paste the planner prompt into one agent');
    expect(instructions).toHaveTextContent('the executor prompt into every other agent you want in this room');
    expect(instructions.textContent).not.toMatch(/two agents/);
  });

  it('shows copyable planner and executor prompts tied to the room after creation', async () => {
    vi.mocked(api.getRoomConnect).mockImplementation(async (_slug: string, role = 'collaborator') =>
      envelope(role as 'planner' | 'executor'),
    );

    render(<RoomStarterPrompts room={room} justCreated={true} />);

    await waitFor(() => {
      expect(screen.getByTestId('starter-planner-prompt')).toHaveTextContent(
        'PLANNER PROMPT for debug-the-parser',
      );
    });
    expect(screen.getByTestId('starter-executor-prompt')).toHaveTextContent(
      'EXECUTOR PROMPT for debug-the-parser',
    );
    expect(api.getRoomConnect).toHaveBeenCalledWith(room.slug, 'planner');
    expect(api.getRoomConnect).toHaveBeenCalledWith(room.slug, 'executor');

    fireEvent.click(screen.getByTestId('starter-copy-planner'));
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith('PLANNER PROMPT for debug-the-parser');
    });
  });
});
