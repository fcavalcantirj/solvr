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
