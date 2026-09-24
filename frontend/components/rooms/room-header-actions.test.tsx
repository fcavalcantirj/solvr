import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';

const shareMock = vi.fn();
vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({ isSharing: false, shared: false, error: null, share: shareMock }),
}));

import { RoomHeaderActions } from './room-header-actions';

describe('RoomHeaderActions (task 33, step 1)', () => {
  beforeEach(() => {
    shareMock.mockClear();
  });

  it('renders a Share action and a Connect an agent action', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(screen.getByRole('button', { name: /share/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /connect an agent/i })).toBeInTheDocument();
  });

  // The Connect action points at the in-page connection panel, keeping the reader
  // in the room rather than routing them away.
  it('links Connect an agent to the in-room connection panel', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(screen.getByRole('link', { name: /connect an agent/i })).toHaveAttribute('href', '#connect-agent');
  });

  // Share copies the CANONICAL room URL (no tokens, no session params).
  it('shares the canonical room URL', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    fireEvent.click(screen.getByRole('button', { name: /share/i }));
    expect(shareMock).toHaveBeenCalledTimes(1);
    const url = shareMock.mock.calls[0][1] as string;
    expect(url).toMatch(/\/rooms\/demo-room$/);
    expect(url).not.toMatch(/token|access_token/i);
  });

  // Task ~513 step 4: sharing a PRIVATE room must explain that recipients still
  // need authorization — the public link is a viewing link, never a bearer
  // credential or invitation.
  it('explains recipients still need authorization when the room is private', () => {
    render(<RoomHeaderActions slug="demo-room" isPrivate />);
    const note = screen.getByTestId('share-private-note');
    expect(note).toBeInTheDocument();
    expect(note).toHaveTextContent(/authoriz/i);
  });

  // A public room carries no such note — anyone with the link can read it.
  it('shows no authorization note for a public room', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(screen.queryByTestId('share-private-note')).not.toBeInTheDocument();
  });

  // Even for a private room, Share copies the clean canonical URL — never a
  // token or session parameter (step 3 holds across visibilities).
  it('shares the canonical URL without credentials for a private room', () => {
    render(<RoomHeaderActions slug="demo-room" isPrivate />);
    fireEvent.click(screen.getByRole('button', { name: /share/i }));
    const url = shareMock.mock.calls[0][1] as string;
    expect(url).toMatch(/\/rooms\/demo-room$/);
    expect(url).not.toMatch(/token|access_token/i);
  });
});
