import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import React from 'react';
import { RoomHeader } from './room-header';
import type { APIRoom } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

function makeRoom(overrides: Partial<APIRoom> = {}): APIRoom {
  return {
    id: 'r1',
    slug: 'demo-room',
    display_name: 'Demo Room',
    tags: [],
    is_private: false,
    message_count: 3,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    last_active_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('RoomHeader status label', () => {
  // Step 2: an archived public room is labeled Finished rather than Live.
  it('labels a live room LIVE', () => {
    render(<RoomHeader room={makeRoom()} />);
    expect(screen.getByTestId('room-status')).toHaveTextContent('LIVE');
    expect(screen.getByTestId('room-status')).not.toHaveTextContent('FINISHED');
  });

  it('labels an archived room FINISHED', () => {
    render(<RoomHeader room={makeRoom({ archived_at: new Date().toISOString() })} />);
    expect(screen.getByTestId('room-status')).toHaveTextContent('FINISHED');
  });
});

describe('RoomHeader visibility + participants (task 33, step 1)', () => {
  it('labels a public room Public', () => {
    render(<RoomHeader room={makeRoom({ is_private: false })} />);
    expect(screen.getByTestId('room-visibility')).toHaveTextContent(/public/i);
  });

  it('labels a private room Private', () => {
    render(<RoomHeader room={makeRoom({ is_private: true })} />);
    expect(screen.getByTestId('room-visibility')).toHaveTextContent(/private/i);
  });

  it('shows a participant summary from the live online count', () => {
    render(<RoomHeader room={makeRoom()} onlineCount={2} />);
    expect(screen.getByTestId('room-participants')).toHaveTextContent(/2/);
  });

  // Step 5: the normal conversation header exposes no raw tokens, protocol names,
  // claims, or event payloads — those live behind authorized technical details.
  it('does not leak raw tokens or protocol names in the header', () => {
    const { container } = render(
      <RoomHeader room={makeRoom({ is_private: true })} onlineCount={1} />,
    );
    const text = container.textContent ?? '';
    expect(text).not.toMatch(/token/i);
    expect(text).not.toMatch(/\/r\//);
    expect(text).not.toMatch(/token_hash|bearer|handshake/i);
  });
});
