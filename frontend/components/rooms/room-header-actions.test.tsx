import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import React from 'react';

const shareMock = vi.fn();
vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({ isSharing: false, shared: false, error: null, share: shareMock }),
}));
vi.mock('@/lib/api', () => ({ api: { getRoomShare: vi.fn(), postFunnelEvent: vi.fn() } }));

import { api } from '@/lib/api';
import { RoomHeaderActions } from './room-header-actions';

// GET /v1/rooms/{slug}/share for a public room (backend handlers/rooms_share.go).
const SHARE = {
  data: {
    room_url: 'https://solvr.dev/rooms/demo-room',
    share_url: 'https://solvr.dev/rooms/demo-room?via=share',
    try_url: 'https://solvr.dev/connect?from_room=demo-room',
    excerpt: { title: 'Demo room', text: 'Shipped a scoreboard.', source: 'result' },
    copy_text: 'Demo room\nShipped a scoreboard.\nhttps://solvr.dev/rooms/demo-room?via=share',
    note: 'Copy only — Solvr never posts this anywhere.',
  },
};

// What may never ride in a link a person hands to someone else.
const CREDENTIAL = /token|access_token|api_key|secret|ticket|flow_id|solvr_/i;

const shareButton = () => screen.getByRole('button', { name: /share/i });
const sharedUrl = (call = 0) => shareMock.mock.calls[call][1] as string;

beforeEach(() => {
  shareMock.mockReset();
  shareMock.mockResolvedValue(true);
  vi.mocked(api.getRoomShare).mockReset();
  vi.mocked(api.getRoomShare).mockResolvedValue(SHARE);
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
});

describe('RoomHeaderActions (task 33, step 1)', () => {
  it('renders a Share action and a Connect an agent action', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(shareButton()).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /connect an agent/i })).toBeInTheDocument();
  });

  // The Connect action points at the in-page connection panel, keeping the reader
  // in the room rather than routing them away.
  it('links Connect an agent to the in-room connection panel', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(screen.getByRole('link', { name: /connect an agent/i })).toHaveAttribute('href', '#connect-agent');
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

  it('asks the API for nothing just for showing the room', () => {
    render(<RoomHeaderActions slug="demo-room" />);
    expect(api.getRoomShare).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

// The header's Share button used to build /rooms/{slug} itself and report nothing: a
// share through the main button was invisible, and so was the visit it brought. It now
// shares the API's share link (the room page with ?via=share, which the page counts
// once per tab as a share_visit) and reports share_link_copied once the share succeeded.
describe('RoomHeaderActions Share', () => {
  it('shares the share link the API composed, never a link of its own', async () => {
    render(<RoomHeaderActions slug="demo-room" displayName="Demo room" />);

    fireEvent.click(shareButton());

    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    expect(api.getRoomShare).toHaveBeenCalledWith('demo-room');
    expect(shareMock).toHaveBeenCalledWith('Demo room', 'https://solvr.dev/rooms/demo-room?via=share');
  });

  it('reports share_link_copied for the room after a successful share or copy', async () => {
    render(<RoomHeaderActions slug="demo-room" />);

    fireEvent.click(shareButton());

    await waitFor(() => expect(api.postFunnelEvent).toHaveBeenCalledTimes(1));
    expect(api.postFunnelEvent).toHaveBeenCalledWith({
      event: 'share_link_copied',
      entry_surface: 'room_page',
      source: { kind: 'room', ref: 'demo-room' },
    });
  });

  it('reports nothing when the share was dismissed or the clipboard refused', async () => {
    shareMock.mockResolvedValue(false);
    render(<RoomHeaderActions slug="demo-room" />);

    fireEvent.click(shareButton());

    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    // Let the handler finish what it would do after the share settled.
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });

  // The API refuses to compose a share link for a private room (409) and a read can
  // fail: the button then shares the clean canonical link, as before, and reports nothing.
  it.each([
    ['a private room', true],
    ['a room whose share link could not be read', false],
  ])('falls back to the canonical URL and reports nothing for %s', async (_label, isPrivate) => {
    vi.mocked(api.getRoomShare).mockRejectedValue(new Error('a private room cannot be shared'));
    render(<RoomHeaderActions slug="demo-room" isPrivate={isPrivate} />);

    fireEvent.click(shareButton());

    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    expect(sharedUrl()).toMatch(/\/rooms\/demo-room$/);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });

  // Whatever is shared, it names only the public room: no token, key, ticket or flow id.
  // The share marker (?via=share) is not a credential.
  it('never puts a token or credential in the shared URL', async () => {
    const first = render(<RoomHeaderActions slug="demo-room" />);
    fireEvent.click(shareButton());
    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    expect(sharedUrl(0)).not.toMatch(CREDENTIAL);
    expect(new URL(sharedUrl(0)).search).toBe('?via=share');
    first.unmount();

    vi.mocked(api.getRoomShare).mockRejectedValue(new Error('refused'));
    render(<RoomHeaderActions slug="demo-room" isPrivate />);
    fireEvent.click(shareButton());
    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(2));
    expect(sharedUrl(1)).not.toMatch(CREDENTIAL);
    expect(new URL(sharedUrl(1)).search).toBe('');
  });

  // Browsers allow a share sheet or a clipboard write only inside the click itself. The
  // link is therefore read when the visitor reaches for the button, so the click can
  // share at once instead of waiting for the API.
  it('reads the share link once, when the visitor reaches for the button', async () => {
    render(<RoomHeaderActions slug="demo-room" />);

    fireEvent.pointerEnter(shareButton());
    fireEvent.focus(shareButton());
    await waitFor(() => expect(api.getRoomShare).toHaveBeenCalledTimes(1));
    // Let the read settle before the click.
    await new Promise((resolve) => setTimeout(resolve, 20));

    fireEvent.click(shareButton());

    // The link is already known: the share happens inside the click, with no wait.
    expect(shareMock).toHaveBeenCalledTimes(1);
    expect(sharedUrl()).toBe('https://solvr.dev/rooms/demo-room?via=share');
    await waitFor(() => expect(api.postFunnelEvent).toHaveBeenCalledTimes(1));
    expect(api.getRoomShare).toHaveBeenCalledTimes(1);
  });

  it('shares the canonical link at once when the API already refused a share link', async () => {
    vi.mocked(api.getRoomShare).mockRejectedValue(new Error('a private room cannot be shared'));
    render(<RoomHeaderActions slug="demo-room" isPrivate />);

    fireEvent.pointerEnter(shareButton());
    await waitFor(() => expect(api.getRoomShare).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));

    fireEvent.click(shareButton());

    expect(shareMock).toHaveBeenCalledTimes(1);
    expect(sharedUrl()).toMatch(/\/rooms\/demo-room$/);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
    expect(api.getRoomShare).toHaveBeenCalledTimes(1);
  });

  it('reads the share link of the room it shows now, not of the one it showed before', async () => {
    const { rerender } = render(<RoomHeaderActions slug="demo-room" />);
    fireEvent.pointerEnter(shareButton());
    await waitFor(() => expect(api.getRoomShare).toHaveBeenCalledWith('demo-room'));
    await new Promise((resolve) => setTimeout(resolve, 20));

    vi.mocked(api.getRoomShare).mockResolvedValue({
      data: { ...SHARE.data, share_url: 'https://solvr.dev/rooms/other-room?via=share' },
    });
    rerender(<RoomHeaderActions slug="other-room" />);
    fireEvent.click(shareButton());

    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    expect(api.getRoomShare).toHaveBeenLastCalledWith('other-room');
    expect(sharedUrl()).toBe('https://solvr.dev/rooms/other-room?via=share');
    await waitFor(() =>
      expect(api.postFunnelEvent).toHaveBeenCalledWith(
        expect.objectContaining({ source: { kind: 'room', ref: 'other-room' } }),
      ),
    );
  });
});
