import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { CONNECT_FLOW_ID, CONNECT_START, connectStartFor } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// What a visitor does inside the connection panel, as Google Analytics events (SPEC.md 27.7):
// picking a use case, flipping the visibility, settling an intent. Each is sent once, after
// it took effect, with the place it happened; what was typed is never sent.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage: vi.fn() }));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';

const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);
const sentence = () => screen.getByTestId('prompt-sentence');
const intentSlot = () => within(sentence()).getByRole('textbox');
const visibilitySwitch = () => within(sentence()).getByRole('switch');

// The panel is open once it reported that it opened (connect_start, sent from an effect
// after the first paint); what the tests below count starts from there.
async function renderPanel(variant: 'panel' | 'page' = 'page') {
  const result = render(<ConnectPanel variant={variant} />);
  await screen.findByTestId('connect-panel');
  await waitFor(() => expect(track).toHaveBeenCalledWith('connect_start', expect.anything()));
  track.mockClear();
  return result;
}

function typeIntent(text: string) {
  const slot = intentSlot();
  slot.textContent = text;
  fireEvent.input(slot);
}

/** Lets a promise the test holds settle inside React's act. */
async function settle(release: () => void) {
  await act(async () => {
    release();
    await Promise.resolve();
  });
}

beforeEach(() => {
  track.mockReset();
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
});

describe('use_case_select', () => {
  it.each([
    ['page', 'connect_page'],
    ['panel', 'homepage_panel'],
  ] as const)('is sent when the %s variant switches to another use case, naming it and %s', async (variant, surface) => {
    await renderPanel(variant);
    fireEvent.click(screen.getByRole('radio', { name: /share context/i }));

    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('use_case_select', { preset: 'collaborate', surface });
  });

  it('is sent once per switch, and not for the use case already showing', async () => {
    await renderPanel();
    fireEvent.click(screen.getByRole('radio', { name: /plan & execute/i }));
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('radio', { name: /build & review/i }));
    fireEvent.click(screen.getByRole('radio', { name: /build & review/i }));
    fireEvent.click(screen.getByRole('radio', { name: /plan & execute/i }));
    expect(eventsNamed('use_case_select').map(([, params]) => params.preset)).toEqual(['build-and-review', 'plan-and-build']);
  });

  it('does not count as a visibility change, though each use case carries its own visibility', async () => {
    await renderPanel();
    // Plan & execute is public, Share context is private: the word in the sentence changes.
    fireEvent.click(screen.getByRole('radio', { name: /share context/i }));
    await waitFor(() => expect(visibilitySwitch()).toHaveAttribute('aria-checked', 'true'));

    expect(eventsNamed('visibility_toggle')).toEqual([]);
  });
});

describe('visibility_toggle', () => {
  it('is sent once the API answered with the sentence in the visibility that was asked for', async () => {
    await renderPanel();
    let answer: (value: { data: typeof CONNECT_START }) => void = () => {};
    vi.mocked(api.getConnectStart).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));

    fireEvent.click(visibilitySwitch());
    await waitFor(() =>
      expect(api.getConnectStart).toHaveBeenLastCalledWith({ visibility: 'private', flow: CONNECT_FLOW_ID }),
    );
    // Asked, not yet answered: the sentence still says public, and nothing was sent.
    expect(track).not.toHaveBeenCalled();

    await settle(() => answer({ data: connectStartFor('plan-and-build', '', 'private') }));
    await waitFor(() => expect(visibilitySwitch()).toHaveAttribute('aria-checked', 'true'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('visibility_toggle', { visibility: 'private', surface: 'connect_page' });
  });

  it('says public when the flip goes back, from the panel the home page opens', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', '', 'private') });
    await renderPanel('panel');
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', '', 'public') });

    fireEvent.click(visibilitySwitch());
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('visibility_toggle', { visibility: 'public', surface: 'homepage_panel' });
  });

  it('is not sent when the API could not be read: the sentence did not change', async () => {
    await renderPanel();
    vi.mocked(api.getConnectStart).mockRejectedValueOnce(new Error('down'));

    fireEvent.click(visibilitySwitch());
    await screen.findByRole('alert');
    expect(visibilitySwitch()).toHaveAttribute('aria-checked', 'false');
    expect(track).not.toHaveBeenCalled();
  });
});

describe('intent_set', () => {
  it('is sent once the typed intent settled and the API answered with it: never the words', async () => {
    await renderPanel();
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', 'ship the signup page') });

    typeIntent('ship the signup page');
    // Typing is not settling: the read waits for a pause.
    expect(track).not.toHaveBeenCalled();

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('intent_set', { surface: 'connect_page' });
    expect(api.getConnectStart).toHaveBeenLastCalledWith({ intent: 'ship the signup page', flow: CONNECT_FLOW_ID });
    expect(JSON.stringify(track.mock.calls)).not.toMatch(/ship|signup/);
  });

  it('is sent once per panel, however many times the intent is rewritten', async () => {
    await renderPanel('panel');
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', 'ship the signup page') });
    typeIntent('ship the signup page');
    await waitFor(() => expect(eventsNamed('intent_set')).toHaveLength(1));
    expect(eventsNamed('intent_set')[0][1]).toEqual({ surface: 'homepage_panel' });

    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', 'ship the billing page') });
    typeIntent('ship the billing page');
    await waitFor(() =>
      expect(api.getConnectStart).toHaveBeenLastCalledWith({ intent: 'ship the billing page', flow: CONNECT_FLOW_ID }),
    );
    await act(async () => {});
    expect(eventsNamed('intent_set')).toHaveLength(1);
  });

  it('is not sent for an intent the visitor did not type, such as one a source brought', async () => {
    // The API answers with an intent of its own before anything was typed.
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: connectStartFor('plan-and-build', 'continue the scoreboard') });
    await renderPanel();
    await act(async () => {});

    expect(eventsNamed('intent_set')).toEqual([]);
  });

  it('is not sent for blank typing, nor when the API could not be read', async () => {
    await renderPanel();
    typeIntent('   ');
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 400));
    });
    expect(track).not.toHaveBeenCalled();

    vi.mocked(api.getConnectStart).mockRejectedValue(new Error('down'));
    typeIntent('ship the signup page');
    await screen.findByRole('alert');
    expect(track).not.toHaveBeenCalled();
  });
});
