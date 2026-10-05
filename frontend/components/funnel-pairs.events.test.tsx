import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, renderHook, screen, waitFor, within } from '@testing-library/react';

// Every place that reports a browser step of the connection funnel also sends the Google
// Analytics event that pairs with it (SPEC.md 27.7), through lib/funnel.ts: once, only after
// the action succeeded, with exactly its parameters. The sentence that was copied, the room
// and the link never ride in the event.

const track = vi.hoisted(() => vi.fn());
const trackOnNextPage = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage }));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const shareMock = vi.hoisted(() => vi.fn());
vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({ isSharing: false, shared: false, error: null, share: shareMock }),
}));

vi.mock('@/lib/api', () => ({
  api: {
    getConnectStart: vi.fn(),
    getRoomConnect: vi.fn(),
    getRoomShare: vi.fn(),
    postFunnelEvent: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import type { APIRoom } from '@/lib/api-types';
import { CONNECT_EXAMPLES, CONNECT_FLOW_ID, CONNECT_START } from '@/components/connect/connect-fixture';
import { ConnectPanel } from '@/components/connect/connect-panel';
import { UseCasesSection } from '@/components/homepage/use-cases-section';
import { GuidePrompt } from '@/components/prompt/guide-prompt';
import { roles } from '@/components/prompt/prompt-align';
import { ConnectAgentPanel } from '@/components/rooms/connect-agent-panel';
import { RoomHeaderActions } from '@/components/rooms/room-header-actions';
import { RoomStarterPrompts } from '@/components/rooms/room-starter-prompts';
import { ShareOutcome } from '@/components/rooms/share-outcome';
import { useShareVisit } from '@/hooks/use-share-visit';

const SENTENCE = /skill\.md|Learn Solvr|https?:/i;

const funnelSteps = () => vi.mocked(api.postFunnelEvent).mock.calls.map(([step]) => step);
const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);

function clipboard(writeText: ReturnType<typeof vi.fn>) {
  Object.assign(navigator, { clipboard: { writeText } });
  return writeText;
}

const room = (over: Partial<APIRoom> = {}) =>
  ({ id: 'room-1', slug: 'demo-room', display_name: 'Demo room', is_private: false, message_count: 0, archived_at: null, ...over }) as unknown as APIRoom;

const roomConnect = (role: string) => ({
  data: {
    instruction_version: '2.1',
    room_slug: 'demo-room',
    room_url: 'https://solvr.dev/rooms/demo-room',
    private: false,
    task: '',
    prompt: {
      text: `Learn Solvr from https://solvr.dev/skill.md. Join as ${role}.`,
      segments: [{ kind: 'text' as const, text: `Learn Solvr from https://solvr.dev/skill.md. Join as ${role}.` }],
      word_count: 7,
    },
    role,
  },
});

const SHARE = {
  data: {
    room_url: 'https://solvr.dev/rooms/demo-room',
    share_url: 'https://solvr.dev/rooms/demo-room?via=share',
    try_url: 'https://solvr.dev/connect?from_room=demo-room',
    excerpt: { title: 'Demo room', text: 'Shipped a scoreboard.', source: 'result' },
    copy_text: 'Demo room\nShipped a scoreboard.\nhttps://solvr.dev/rooms/demo-room?via=share',
    note: 'Copy only.',
  },
};

beforeEach(() => {
  track.mockReset();
  trackOnNextPage.mockReset();
  shareMock.mockReset();
  for (const fn of Object.values(api)) vi.mocked(fn as ReturnType<typeof vi.fn>).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
  vi.mocked(api.getRoomShare).mockResolvedValue(SHARE as never);
  vi.mocked(api.getRoomConnect).mockImplementation(async (_slug: string, role = 'collaborator') => roomConnect(role) as never);
  clipboard(vi.fn().mockResolvedValue(undefined));
  window.sessionStorage.clear();
  window.history.replaceState(null, '', '/');
});

describe('connect_start, with connection_started', () => {
  it.each([
    ['page', 'connect_page'],
    ['panel', 'homepage_panel'],
  ] as const)('is sent once when the %s variant opens, naming %s', async (variant, surface) => {
    render(<ConnectPanel variant={variant} />);
    await screen.findByTestId('connect-panel');

    await waitFor(() => expect(eventsNamed('connect_start')).toHaveLength(1));
    expect(eventsNamed('connect_start')[0]).toEqual(['connect_start', { surface }]);
    expect(funnelSteps().filter((s) => s.event === 'connection_started')).toEqual([
      expect.objectContaining({ event: 'connection_started', entry_surface: surface, flow_id: CONNECT_FLOW_ID }),
    ]);
  });

  it('is not sent when the sentence could not be read: nothing opened', async () => {
    vi.mocked(api.getConnectStart).mockRejectedValue(new Error('down'));
    render(<ConnectPanel variant="page" />);
    await screen.findByRole('alert');

    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('prompt_copy, with starter_prompt_copied', () => {
  it('is sent from the connection panel after the clipboard took the sentence', async () => {
    render(<ConnectPanel variant="page" />);
    await screen.findByTestId('connect-panel');
    await waitFor(() => expect(eventsNamed('connect_start')).toHaveLength(1));
    track.mockClear();

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => expect(eventsNamed('prompt_copy')).toHaveLength(1));
    const active = CONNECT_START.presets.find((p) => p.selected) ?? CONNECT_START.presets[0];
    expect(track.mock.calls).toEqual([
      ['prompt_copy', { surface: 'connect_page', preset: active.value, role: roles(active.prompt).a?.toLowerCase() }],
    ]);
    expect(funnelSteps().filter((s) => s.event === 'starter_prompt_copied')).toHaveLength(1);
    expect(JSON.stringify(track.mock.calls)).not.toMatch(SENTENCE);
  });

  it('names the home page panel when the copy happens there', async () => {
    render(<ConnectPanel variant="panel" />);
    await screen.findByTestId('connect-panel');
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => expect(eventsNamed('prompt_copy')).toHaveLength(1));
    expect(eventsNamed('prompt_copy')[0][1]).toMatchObject({ surface: 'homepage_panel' });
  });

  it('is sent from a home page card, for that card', async () => {
    render(<UseCasesSection examples={CONNECT_EXAMPLES} />);
    const card = screen.getAllByTestId('use-case-card')[1];
    fireEvent.click(within(card).getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('prompt_copy', {
      surface: 'homepage_use_cases',
      preset: CONNECT_EXAMPLES[1].value,
      role: roles(CONNECT_EXAMPLES[1].prompt).a?.toLowerCase(),
    });
    expect(funnelSteps()).toEqual([expect.objectContaining({ event: 'starter_prompt_copied', entry_surface: 'homepage_use_cases' })]);
  });

  it('is sent from a guide', async () => {
    render(<GuidePrompt example={CONNECT_EXAMPLES[0]} />);
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('prompt_copy', {
      surface: 'guide_page',
      preset: CONNECT_EXAMPLES[0].value,
      role: roles(CONNECT_EXAMPLES[0].prompt).a?.toLowerCase(),
    });
  });

  it.each([
    ['connection panel', () => render(<ConnectPanel variant="page" />)],
    ['home page cards', () => render(<UseCasesSection examples={CONNECT_EXAMPLES} />)],
    ['guide', () => render(<GuidePrompt example={CONNECT_EXAMPLES[0]} />)],
  ])('is not sent from the %s when the browser blocks the clipboard', async (_name, mount) => {
    const writeText = clipboard(vi.fn().mockRejectedValue(new Error('denied')));
    mount();
    const button = (await screen.findAllByRole('button', { name: /copy prompt/i }))[0];
    // Let the panel report that it opened before counting what the press reports.
    await act(async () => {});
    track.mockClear();
    vi.mocked(api.postFunnelEvent).mockClear();

    fireEvent.click(button);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(eventsNamed('prompt_copy')).toEqual([]);
    expect(funnelSteps().filter((s) => s.event === 'starter_prompt_copied')).toEqual([]);
  });

  // One event per action: prompt_copy, sent once the copy worked, replaces the cta_click
  // these buttons sent on every press, whether or not anything was copied.
  it.each([
    ['connection panel', () => render(<ConnectPanel variant="page" />)],
    ['home page cards', () => render(<UseCasesSection examples={CONNECT_EXAMPLES} />)],
    ['guide', () => render(<GuidePrompt example={CONNECT_EXAMPLES[0]} />)],
  ])('is the only event of a Copy prompt button in the %s: the button carries no click mark', async (_name, mount) => {
    mount();
    const buttons = await screen.findAllByRole('button', { name: /copy prompt/i });
    for (const button of buttons) {
      expect(button).not.toHaveAttribute('data-track');
      expect(button.closest('[data-track]')).toBeNull();
    }
  });
});

describe("prompt_copy for a new room's starter prompts, with starter_prompt_copied", () => {
  it.each(['planner', 'executor'] as const)('reports the %s prompt: the funnel step with the public room as source, and the event', async (role) => {
    render(<RoomStarterPrompts room={room()} justCreated />);
    fireEvent.click(await screen.findByTestId(`starter-copy-${role}`));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('prompt_copy', { surface: 'room_starter_prompts', role });
    expect(funnelSteps()).toEqual([
      { event: 'starter_prompt_copied', entry_surface: 'room_starter_prompts', role, source: { kind: 'room', ref: 'demo-room' } },
    ]);
    expect(JSON.stringify(track.mock.calls)).not.toMatch(/demo-room|skill\.md/);
  });

  it('names no source for a private room: its slug is never sent as one', async () => {
    render(<RoomStarterPrompts room={room({ is_private: true, slug: 'secret-room' })} justCreated />);
    fireEvent.click(await screen.findByTestId('starter-copy-planner'));

    await waitFor(() => expect(api.postFunnelEvent).toHaveBeenCalledTimes(1));
    expect(funnelSteps()).toEqual([{ event: 'starter_prompt_copied', entry_surface: 'room_starter_prompts', role: 'planner' }]);
    expect(JSON.stringify([funnelSteps(), track.mock.calls])).not.toContain('secret-room');
  });

  it('reports nothing when the browser blocks the clipboard, and does not say COPIED', async () => {
    const writeText = clipboard(vi.fn().mockRejectedValue(new Error('denied')));
    render(<RoomStarterPrompts room={room()} justCreated />);
    fireEvent.click(await screen.findByTestId('starter-copy-planner'));
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
    expect(screen.getByTestId('starter-copy-planner')).not.toHaveTextContent('COPIED');
  });

  it('reports each copy once, however long the button says COPIED', async () => {
    render(<RoomStarterPrompts room={room()} justCreated />);
    fireEvent.click(await screen.findByTestId('starter-copy-planner'));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByTestId('starter-copy-executor'));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(2));

    expect(track.mock.calls.map(([, params]) => params.role)).toEqual(['planner', 'executor']);
    expect(api.postFunnelEvent).toHaveBeenCalledTimes(2);
  });
});

describe('join_prompt_copy, with join_prompt_copied', () => {
  it('is sent after the join prompt is on the clipboard', async () => {
    render(<ConnectAgentPanel room={room()} />);
    fireEvent.click(screen.getByRole('button', { name: /get join prompt/i }));
    fireEvent.click(await screen.findByRole('button', { name: /copy join prompt/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('join_prompt_copy', { surface: 'room_page' });
    expect(funnelSteps()).toEqual([{ event: 'join_prompt_copied', role: 'collaborator', entry_surface: 'room_page' }]);
  });

  it('is not sent when the clipboard is blocked, nor for only reading the prompt', async () => {
    const writeText = clipboard(vi.fn().mockRejectedValue(new Error('denied')));
    render(<ConnectAgentPanel room={room()} />);
    fireEvent.click(screen.getByRole('button', { name: /get join prompt/i }));
    const copy = await screen.findByRole('button', { name: /copy join prompt/i });
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(copy);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('room_share, with share_link_copied', () => {
  it.each(['share_sheet', 'clipboard'] as const)('says the link left through the %s', async (method) => {
    shareMock.mockResolvedValue(method);
    render(<RoomHeaderActions slug="demo-room" displayName="Demo room" />);
    fireEvent.click(screen.getByRole('button', { name: /^share$/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('room_share', { method });
    expect(funnelSteps()).toEqual([{ event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } }]);
    expect(JSON.stringify(track.mock.calls)).not.toMatch(/demo-room|via=share/);
  });

  it('is not sent when the visitor dismisses the share sheet or the browser refuses', async () => {
    shareMock.mockResolvedValue(false);
    render(<RoomHeaderActions slug="demo-room" />);
    fireEvent.click(screen.getByRole('button', { name: /^share$/i }));
    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });

  it('is not sent when the API composed no share link (a private room): the clean link is shared, nothing is reported', async () => {
    vi.mocked(api.getRoomShare).mockRejectedValue(new Error('409'));
    shareMock.mockResolvedValue('clipboard');
    render(<RoomHeaderActions slug="demo-room" isPrivate />);
    fireEvent.click(screen.getByRole('button', { name: /^share$/i }));
    await waitFor(() => expect(shareMock).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('outcome_copy, with share_link_copied', () => {
  it('is sent, with no parameter of its own, after the outcome is on the clipboard', async () => {
    render(<ShareOutcome slug="demo-room" />);
    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    fireEvent.click(await screen.findByRole('button', { name: /^copy$/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track.mock.calls[0][0]).toBe('outcome_copy');
    expect(track.mock.calls[0][1]).toBeUndefined();
    expect(funnelSteps()).toEqual([{ event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } }]);
  });

  it('is not sent for opening the panel, nor when the clipboard is blocked', async () => {
    const writeText = clipboard(vi.fn().mockRejectedValue(new Error('denied')));
    render(<ShareOutcome slug="demo-room" />);
    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    const copy = await screen.findByRole('button', { name: /^copy$/i });
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(copy);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('share_visit, with share_visit', () => {
  it('is sent once per tab for a page opened from a share link', () => {
    window.history.replaceState(null, '', '/rooms/demo-room?via=share');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'demo-room' }, 'room_page'));

    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('share_visit', { surface: 'room_page' });
    expect(funnelSteps()).toEqual([{ event: 'share_visit', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } }]);

    // The same tab opens the link again: counted already.
    window.history.replaceState(null, '', '/rooms/demo-room?via=share');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'demo-room' }, 'room_page'));
    expect(track).toHaveBeenCalledTimes(1);
  });

  it('is not sent for an ordinary visit, nor for a page that cannot be attributed', () => {
    window.history.replaceState(null, '', '/rooms/demo-room');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'demo-room' }, 'room_page'));
    window.history.replaceState(null, '', '/rooms/secret-room?via=share');
    renderHook(() => useShareVisit(null, 'room_page'));

    expect(track).not.toHaveBeenCalled();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});
