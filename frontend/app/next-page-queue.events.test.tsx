import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';

// Four things happen where an event cannot be sent at once (SPEC.md 27.7):
//
//   an agent is claimed          on /claim, a page Google's tag never sees
//   GitHub or Google signs in    on /auth/callback, never seen either, then a full page load
//   e-mail signs in              on /login, then a full page load
//   someone signs out            anywhere, then a full page load
//
// Each is kept in this tab's sessionStorage and sent from the next page the tag does see,
// carrying the kind of page it happened on. This file runs the REAL lib/analytics.ts, the
// real consent store and the real data layer: only the API and the session are replaced.

const push = vi.fn();
let query = new URLSearchParams();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  useSearchParams: () => query,
}));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

// The pages read a session that the test sets; the logout test runs the real provider.
const session = vi.hoisted(() => ({ current: null as null | Record<string, unknown> }));
vi.mock('@/hooks/use-auth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/hooks/use-auth')>();
  return { ...actual, useAuth: () => session.current ?? actual.useAuth() };
});
vi.mock('@/components/ui/auth-required-modal', () => ({ AuthRequiredModal: () => null }));

vi.mock('@/lib/api', () => ({
  api: {
    getClaimInfo: vi.fn(),
    claimAgent: vi.fn(),
    getMe: vi.fn(),
    setAuthToken: vi.fn(),
    clearAuthToken: vi.fn(),
    onAuthError: vi.fn(),
    offAuthError: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { flushTrackedEvents, PENDING_EVENTS_KEY } from '@/lib/analytics';
import { setConsent } from '@/lib/consent';
import { loadGoogleTag } from '@/lib/google-tag';
import { AuthProvider, useAuth } from '@/hooks/use-auth';
import LoginPage from '@/app/login/page';
import AuthCallbackPage from '@/app/auth/callback/page';
import ClaimPage from '@/app/claim/page';

type TagWindow = Window & { dataLayer?: unknown[] } & Record<string, unknown>;
const w = () => window as unknown as TagWindow;
const sentEvents = () =>
  (w().dataLayer ?? [])
    .map((entry) => Array.from(entry as ArrayLike<unknown>))
    .filter(([name]) => name === 'event')
    .map(([, name, params]) => [name, params]);
const waiting = () => JSON.parse(window.sessionStorage.getItem(PENDING_EVENTS_KEY) ?? '[]') as Array<{ e: string; p: unknown; g: string }>;

const realLocation = window.location;
const place = { pathname: '/', left: '' };

/** The page the tab is on. Assigning href is a full page load: the test records where to. */
function at(pathname: string) {
  place.pathname = pathname;
  place.left = '';
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      get pathname() {
        return place.pathname;
      },
      search: '',
      hash: '',
      origin: 'http://localhost',
      get href() {
        return place.left;
      },
      set href(value: string) {
        place.left = value;
      },
      reload: vi.fn(),
    },
  });
}

/** The next page: a tracked one, where the tag arrives once the page is idle. */
function nextPage(pathname: string) {
  place.pathname = pathname;
  loadGoogleTag('G-TEST', 'home');
  flushTrackedEvents();
}

const fetchMock = vi.fn();
const agent = { id: 'agent-1', display_name: 'Claude Helper', bio: '', reputation: 42, status: 'active', created_at: '2026-01-15T10:00:00Z' };

beforeEach(() => {
  push.mockReset();
  query = new URLSearchParams();
  session.current = null;
  for (const fn of Object.values(api)) vi.mocked(fn as ReturnType<typeof vi.fn>).mockReset();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  window.localStorage.clear();
  window.sessionStorage.clear();
  delete w().dataLayer;
  at('/');
});

afterEach(() => {
  vi.unstubAllGlobals();
  Object.defineProperty(window, 'location', { configurable: true, value: realLocation });
});

async function claimAnAgent() {
  session.current = { isAuthenticated: true, isLoading: false, user: { id: 'u1', type: 'human', displayName: 'Jane' } };
  window.sessionStorage.setItem('solvr_pending_claim_token', 'claim-token-abc');
  vi.mocked(api.getClaimInfo).mockResolvedValue({ token_valid: true, agent, expires_at: new Date(Date.now() + 3600000).toISOString() } as never);
  vi.mocked(api.claimAgent).mockResolvedValue({ agent } as never);
  at('/claim');
  render(<ClaimPage />);
  fireEvent.click(await screen.findByRole('button', { name: /claim this agent/i }));
  await screen.findByText('Successfully Claimed!');
}

async function signInThroughGitHub(isNewUser: boolean) {
  session.current = { setToken: vi.fn().mockResolvedValue(undefined) };
  query = new URLSearchParams('code=solvr_lc_abc');
  fetchMock.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: async () => ({ data: { access_token: 'eyJ.jwt.sig', token_type: 'Bearer', expires_in: 900, is_new_user: isNewUser, provider: 'github' } }),
  });
  at('/auth/callback');
  render(<AuthCallbackPage />);
  await waitFor(() => expect(place.left).toBe('/posts'));
}

async function signInByEmail() {
  session.current = { loginWithEmail: vi.fn().mockResolvedValue({ success: true }), loginWithGitHub: vi.fn(), loginWithGoogle: vi.fn() };
  at('/login');
  render(<LoginPage />);
  fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'jane@example.test' } });
  fireEvent.change(screen.getByPlaceholderText('Enter your password'), { target: { value: 'password123' } });
  fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
  await waitFor(() => expect(place.left).toBe('/'));
}

async function signOut() {
  window.localStorage.setItem('auth_token', 'test-token');
  vi.mocked(api.getMe).mockResolvedValue({ data: { id: 'user-123', type: 'human', display_name: 'Jane', email: 'jane@example.test' } } as never);
  at('/rooms/demo-room');
  const { result } = renderHook(() => useAuth(), { wrapper: ({ children }: { children: React.ReactNode }) => <AuthProvider>{children}</AuthProvider> });
  await waitFor(() => expect(result.current.isAuthenticated).toBe(true));
  act(() => result.current.logout());
  expect(place.left).toBe('/');
}

const CASES: Array<{
  name: string;
  act: () => Promise<void>;
  kept: { e: string; p: Record<string, unknown>; g: string };
  landsOn: string;
}> = [
  { name: 'an agent claimed on /claim', act: claimAnAgent, kept: { e: 'agent_claim', p: { surface: 'claim_page' }, g: 'account' }, landsOn: '/agents/agent-1' },
  { name: 'a sign-up through GitHub on /auth/callback', act: () => signInThroughGitHub(true), kept: { e: 'sign_up', p: { method: 'github' }, g: 'account' }, landsOn: '/posts' },
  { name: 'a login through GitHub on /auth/callback', act: () => signInThroughGitHub(false), kept: { e: 'login', p: { method: 'github' }, g: 'account' }, landsOn: '/posts' },
  { name: 'a login by e-mail on /login', act: signInByEmail, kept: { e: 'login', p: { method: 'email' }, g: 'account' }, landsOn: '/' },
  { name: 'a logout from a room page', act: signOut, kept: { e: 'logout', p: {}, g: 'room' }, landsOn: '/' },
];

describe.each(CASES)('$name', ({ act: perform, kept, landsOn }) => {
  it('is kept in this tab, sends nothing from the page it happened on, and is sent from the next tracked page', async () => {
    setConsent('granted');
    await perform();

    // Kept, not sent: no tag on this page, or the page is about to be left.
    expect(waiting()).toEqual([kept]);
    expect(sentEvents()).toEqual([]);
    const stored = window.sessionStorage.getItem(PENDING_EVENTS_KEY) as string;
    expect(stored).not.toMatch(/claim-token|solvr_lc|eyJ|example\.test|password|test-token|user-123|Jane/);

    // The next page: the tag arrives, and the event goes out with the kind of page it
    // happened on, not the one it is sent from.
    nextPage(landsOn);
    expect(sentEvents()).toEqual([[kept.e, { ...kept.p, content_group: kept.g }]]);
    expect(window.sessionStorage.getItem(PENDING_EVENTS_KEY)).toBeNull();

    // Once.
    flushTrackedEvents();
    expect(sentEvents()).toHaveLength(1);
  });

  it('keeps nothing when the visitor has not accepted analytics', async () => {
    await perform();
    expect(window.sessionStorage.getItem(PENDING_EVENTS_KEY)).toBeNull();

    setConsent('granted');
    nextPage(landsOn);
    expect(sentEvents()).toEqual([]);
  });

  it('is dropped, never sent, when consent was withdrawn before the next page', async () => {
    setConsent('granted');
    await perform();
    expect(waiting()).toHaveLength(1);

    setConsent('denied');
    nextPage(landsOn);
    expect(sentEvents()).toEqual([]);
    expect(window.sessionStorage.getItem(PENDING_EVENTS_KEY)).toBeNull();
  });
});

describe('the queue across pages', () => {
  it('waits through another untracked page, and keeps the order things happened in', async () => {
    setConsent('granted');
    await signInThroughGitHub(true);
    // The visitor lands on /claim next (an agent's claim link was waiting): still unseen.
    place.pathname = '/claim';
    loadGoogleTag('G-TEST', 'home');
    flushTrackedEvents();
    expect(sentEvents()).toEqual([]);
    expect(waiting()).toHaveLength(1);

    document.body.innerHTML = '';
    await claimAnAgent();
    expect(waiting().map((entry) => entry.e)).toEqual(['sign_up', 'agent_claim']);

    nextPage('/agents/agent-1');
    expect(sentEvents()).toEqual([
      ['sign_up', { method: 'github', content_group: 'account' }],
      ['agent_claim', { surface: 'claim_page', content_group: 'account' }],
    ]);
  });
});
