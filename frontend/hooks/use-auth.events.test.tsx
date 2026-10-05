import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';

// The session's own events (SPEC.md 27.7):
//   logout           kept for the next page, because signing out ends with a full page load
//   auth_wall_shown  the login dialog opened because an action needed a session, with the
//                    action that asked for it. Once per opening.

const track = vi.hoisted(() => vi.fn());
const trackOnNextPage = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage }));

vi.mock('@/lib/api', () => ({
  api: {
    getMe: vi.fn(),
    setAuthToken: vi.fn(),
    clearAuthToken: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    onAuthError: vi.fn(),
    offAuthError: vi.fn(),
  },
}));
vi.mock('@/components/ui/auth-required-modal', () => ({ AuthRequiredModal: () => null }));

import { api } from '@/lib/api';
import { useAuth, AuthProvider } from './use-auth';

const wrapper = ({ children }: { children: React.ReactNode }) => <AuthProvider>{children}</AuthProvider>;

const realLocation = window.location;
let landedOn = '';
let keptBeforeLeaving = -1;

function at(pathname: string) {
  landedOn = '';
  keptBeforeLeaving = -1;
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      pathname,
      search: '',
      get href() {
        return landedOn;
      },
      set href(value: string) {
        landedOn = value;
        keptBeforeLeaving = trackOnNextPage.mock.calls.length;
      },
    },
  });
}

async function signedIn() {
  window.localStorage.setItem('auth_token', 'test-token');
  vi.mocked(api.getMe).mockResolvedValue({ data: { id: 'user-123', type: 'human', display_name: 'Jane', email: 'jane@example.test' } } as never);
  const hook = renderHook(() => useAuth(), { wrapper });
  await waitFor(() => expect(hook.result.current.isAuthenticated).toBe(true));
  return hook;
}

async function anonymous() {
  vi.mocked(api.getMe).mockRejectedValue(new Error('Not authenticated'));
  const hook = renderHook(() => useAuth(), { wrapper });
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
  return hook;
}

beforeEach(() => {
  track.mockReset();
  trackOnNextPage.mockReset();
  for (const fn of Object.values(api)) vi.mocked(fn as ReturnType<typeof vi.fn>).mockReset();
  window.localStorage.clear();
  at('/rooms');
});

afterEach(() => {
  Object.defineProperty(window, 'location', { configurable: true, value: realLocation });
});

describe('logout', () => {
  it('is kept for the next page before the page is left, and says nothing about who signed out', async () => {
    const { result } = await signedIn();

    act(() => result.current.logout());

    expect(landedOn).toBe('/');
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage.mock.calls[0][0]).toBe('logout');
    expect(trackOnNextPage.mock.calls[0][1]).toBeUndefined();
    expect(keptBeforeLeaving).toBe(1);
    expect(track).not.toHaveBeenCalled();
    expect(window.localStorage.getItem('auth_token')).toBeNull();
    expect(JSON.stringify(trackOnNextPage.mock.calls)).not.toMatch(/user-123|Jane|example\.test|test-token/);
  });

  it('is not sent for a session that simply ended: a stale token is not a logout', async () => {
    window.localStorage.setItem('auth_token', 'stale-token');
    await anonymous();

    expect(window.localStorage.getItem('auth_token')).toBeNull();
    expect(trackOnNextPage).not.toHaveBeenCalled();
    expect(track).not.toHaveBeenCalled();
  });
});

describe('auth_wall_shown', () => {
  it('is sent when an action asks for a session, with the action that asked', async () => {
    const { result } = await anonymous();

    act(() => result.current.showAuthWall('room_create'));

    expect(result.current.showAuthModal).toBe(true);
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('auth_wall_shown', { surface: 'room_create' });
  });

  it('is sent once per opening: asking again while the dialog is open adds nothing', async () => {
    const { result } = await anonymous();

    act(() => result.current.showAuthWall('blog_vote'));
    act(() => result.current.showAuthWall('blog_vote'));
    expect(track).toHaveBeenCalledTimes(1);

    act(() => result.current.setShowAuthModal(false));
    expect(result.current.showAuthModal).toBe(false);
    act(() => result.current.showAuthWall('room_create'));
    expect(track.mock.calls).toEqual([
      ['auth_wall_shown', { surface: 'blog_vote' }],
      ['auth_wall_shown', { surface: 'room_create' }],
    ]);
  });

  it('is sent as api_request when the API refuses an anonymous request and the dialog opens', async () => {
    const { result } = await anonymous();
    const refused = vi.mocked(api.onAuthError).mock.calls[0][0] as () => void;

    act(() => refused());

    expect(result.current.showAuthModal).toBe(true);
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('auth_wall_shown', { surface: 'api_request' });
  });

  it('is not sent where the refusal opens no dialog: on a sign-in page, or for someone signed in', async () => {
    at('/login');
    const anon = await anonymous();
    act(() => (vi.mocked(api.onAuthError).mock.calls[0][0] as () => void)());
    expect(anon.result.current.showAuthModal).toBe(false);
    anon.unmount();

    at('/rooms');
    vi.mocked(api.onAuthError).mockClear();
    const member = await signedIn();
    act(() => (vi.mocked(api.onAuthError).mock.calls[0][0] as () => void)());
    expect(member.result.current.showAuthModal).toBe(false);

    expect(track).not.toHaveBeenCalled();
  });
});
