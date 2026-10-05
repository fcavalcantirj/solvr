import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';

// What happens to an account, as Google Analytics events (SPEC.md 27.7): sign_up, login,
// agent_claim, api_key_create and api_key_copy. Each is sent once, after the API accepted the
// action, and carries how it was done: never an e-mail address, a name, a token or a key.
//
// track() is for an action the visitor stays on the page after; trackOnNextPage() for one
// followed by a full page load, or taken on a page Google's tag never sees.

const track = vi.hoisted(() => vi.fn());
const trackOnNextPage = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage }));

const push = vi.fn();
let query = new URLSearchParams();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  useSearchParams: () => query,
  usePathname: () => '/settings/api-keys',
}));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const auth = vi.hoisted(() => ({
  isAuthenticated: false,
  isLoading: false,
  user: null as null | { id: string; type: string; displayName: string },
  loginWithGitHub: vi.fn(),
  loginWithGoogle: vi.fn(),
  loginWithEmail: vi.fn(),
  register: vi.fn(),
  setToken: vi.fn(),
}));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => auth }));

const keys = vi.hoisted(() => ({ createKey: vi.fn(), revokeKey: vi.fn(), regenerateKey: vi.fn() }));
vi.mock('@/hooks/use-api-keys', () => ({
  useAPIKeys: () => ({ keys: [], loading: false, error: null, refetch: vi.fn(), ...keys }),
}));
vi.mock('@/components/settings/settings-layout', () => ({
  SettingsLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock('@/lib/api', () => ({
  api: { getClaimInfo: vi.fn(), claimAgent: vi.fn() },
  formatRelativeTime: () => '2d ago',
}));

import { api } from '@/lib/api';
import JoinPage from '@/app/join/page';
import LoginPage from '@/app/login/page';
import AuthCallbackPage from '@/app/auth/callback/page';
import ClaimPage from '@/app/claim/page';
import APIKeysPage from '@/app/settings/api-keys/page';
import { ClaimAgentForm } from '@/components/claim-agent-form';

const SECRET = /@|example\.test|password|eyJ|solvr_|Jane|token/i;
const everythingSent = () => JSON.stringify([track.mock.calls, trackOnNextPage.mock.calls]);

const fetchMock = vi.fn();
let landedOn = '';
// How many events were already kept for the next page at the moment the page was left.
let keptBeforeLeaving = -1;
const realLocation = window.location;

/** A page that ends with a full page load: capture where it goes instead of going there. */
function stubLocation(pathname: string, search = '') {
  landedOn = '';
  keptBeforeLeaving = -1;
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      get href() {
        return landedOn;
      },
      set href(value: string) {
        landedOn = value;
        keptBeforeLeaving = trackOnNextPage.mock.calls.length;
      },
      pathname,
      search,
      hash: '',
      origin: 'http://localhost',
      reload: vi.fn(),
    },
  });
}

function exchangeAnswer(extra: Record<string, unknown>) {
  return { ok: true, status: 200, json: async () => ({ data: { access_token: 'eyJ.jwt.sig', token_type: 'Bearer', expires_in: 900, ...extra } }) };
}

beforeEach(() => {
  track.mockReset();
  trackOnNextPage.mockReset();
  push.mockReset();
  query = new URLSearchParams();
  Object.assign(auth, { isAuthenticated: false, isLoading: false, user: null });
  for (const fn of [auth.loginWithEmail, auth.register, auth.setToken, keys.createKey, keys.regenerateKey]) fn.mockReset();
  auth.setToken.mockResolvedValue(undefined);
  vi.mocked(api.getClaimInfo).mockReset();
  vi.mocked(api.claimAgent).mockReset();
  fetchMock.mockReset();
  // The join page asks the API how many people signed up; it is not what is under test.
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: { humans_count: 10 } }) });
  vi.stubGlobal('fetch', fetchMock);
  window.localStorage.clear();
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  Object.defineProperty(window, 'location', { configurable: true, value: realLocation });
});

describe('sign_up by e-mail (/join)', () => {
  function fillAndSubmit() {
    render(<JoinPage />);
    fireEvent.click(screen.getByText('CONTINUE WITH EMAIL'));
    fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: 'Jane' } });
    fireEvent.change(screen.getByPlaceholderText('Doe'), { target: { value: 'Doe' } });
    fireEvent.change(screen.getByPlaceholderText('janedoe'), { target: { value: 'janedoe' } });
    fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'jane@example.test' } });
    fireEvent.change(screen.getByPlaceholderText('Min. 8 characters'), { target: { value: 'password123' } });
    fireEvent.click(screen.getByRole('checkbox'));
    fireEvent.click(screen.getByText('CREATE ACCOUNT'));
  }

  it('is sent once the API accepted the account, with the method and nothing about the person', async () => {
    auth.register.mockResolvedValue({ success: true });
    fillAndSubmit();

    await waitFor(() => expect(push).toHaveBeenCalledWith('/'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('sign_up', { method: 'email' });
    // The visitor stays in the app (a client-side move home): no need to wait for a next page.
    expect(trackOnNextPage).not.toHaveBeenCalled();
    expect(everythingSent()).not.toMatch(SECRET);
  });

  it('is not sent when the API refused the account', async () => {
    auth.register.mockResolvedValue({ success: false, error: 'Email already registered' });
    fillAndSubmit();

    expect(await screen.findByText('Email already registered')).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();
    expect(trackOnNextPage).not.toHaveBeenCalled();
  });

  it('is not sent for only moving to the second step, nor for pressing a provider button', () => {
    render(<JoinPage />);
    fireEvent.click(screen.getByText('CONTINUE WITH GITHUB'));
    fireEvent.click(screen.getByText('CONTINUE WITH EMAIL'));
    expect(track).not.toHaveBeenCalled();
    expect(trackOnNextPage).not.toHaveBeenCalled();
  });
});

describe('login by e-mail (/login)', () => {
  function submit() {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'jane@example.test' } });
    fireEvent.change(screen.getByPlaceholderText('Enter your password'), { target: { value: 'password123' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
  }

  it('is kept for the next page, because a full page load follows', async () => {
    stubLocation('/login');
    auth.loginWithEmail.mockResolvedValue({ success: true });
    submit();

    await waitFor(() => expect(landedOn).toBe('/'));
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage).toHaveBeenCalledWith('login', { method: 'email' });
    expect(track).not.toHaveBeenCalled();
    expect(everythingSent()).not.toMatch(SECRET);
    // Kept before the page is left: after it, nothing of this page runs any more.
    expect(keptBeforeLeaving).toBe(1);
  });

  it('is not sent when the password is wrong', async () => {
    stubLocation('/login');
    auth.loginWithEmail.mockResolvedValue({ success: false, error: 'Invalid email or password' });
    submit();

    expect(await screen.findByText('Invalid email or password')).toBeInTheDocument();
    expect(trackOnNextPage).not.toHaveBeenCalled();
    expect(landedOn).toBe('');
  });
});

describe('sign_up and login through GitHub or Google (/auth/callback)', () => {
  it.each([
    [{ is_new_user: true, provider: 'github' }, 'sign_up', { method: 'github' }],
    [{ is_new_user: true, provider: 'google' }, 'sign_up', { method: 'google' }],
    [{ is_new_user: false, provider: 'github' }, 'login', { method: 'github' }],
    [{ is_new_user: false, provider: 'google' }, 'login', { method: 'google' }],
  ])('reads %j from the exchange and keeps %s for the next page', async (answer, event, params) => {
    stubLocation('/auth/callback');
    query = new URLSearchParams('code=solvr_lc_abc');
    fetchMock.mockResolvedValueOnce(exchangeAnswer(answer));
    render(<AuthCallbackPage />);

    await waitFor(() => expect(landedOn).toBe('/posts'));
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage).toHaveBeenCalledWith(event, params);
    expect(keptBeforeLeaving).toBe(1);
    // /auth/callback is a page Google's tag never sees: nothing is sent from it directly.
    expect(track).not.toHaveBeenCalled();
    expect(everythingSent()).not.toMatch(/solvr_lc|eyJ|code/);
  });

  it('calls it a login, with no method, when an older API says neither', async () => {
    stubLocation('/auth/callback');
    query = new URLSearchParams('code=solvr_lc_abc');
    fetchMock.mockResolvedValueOnce(exchangeAnswer({}));
    render(<AuthCallbackPage />);

    await waitFor(() => expect(landedOn).toBe('/posts'));
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage.mock.calls[0][0]).toBe('login');
    expect(trackOnNextPage.mock.calls[0][1]).toEqual({ method: undefined });
  });

  it('does not take a truthy string for the flag, nor a provider it does not know', async () => {
    stubLocation('/auth/callback');
    query = new URLSearchParams('code=solvr_lc_abc');
    fetchMock.mockResolvedValueOnce(exchangeAnswer({ is_new_user: 'false', provider: 'someone@example.test' }));
    render(<AuthCallbackPage />);

    await waitFor(() => expect(landedOn).toBe('/posts'));
    expect(trackOnNextPage).toHaveBeenCalledWith('login', { method: undefined });
    expect(everythingSent()).not.toMatch(/example\.test/);
  });

  it('is not sent when the code is refused, when the session cannot be stored, or when the provider said no', async () => {
    stubLocation('/auth/callback');
    query = new URLSearchParams('code=solvr_lc_used');
    fetchMock.mockResolvedValueOnce({ ok: false, status: 401, json: async () => ({ error: { code: 'INVALID_LOGIN_CODE', message: 'Sign in again.' } }) });
    const refused = render(<AuthCallbackPage />);
    expect(await screen.findByText('Sign in again.')).toBeInTheDocument();
    refused.unmount();

    fetchMock.mockResolvedValueOnce(exchangeAnswer({ is_new_user: true, provider: 'github' }));
    auth.setToken.mockRejectedValueOnce(new Error('storage'));
    const broken = render(<AuthCallbackPage />);
    expect(await screen.findByText('AUTHENTICATION_FAILED')).toBeInTheDocument();
    broken.unmount();

    query = new URLSearchParams('error=access_denied');
    render(<AuthCallbackPage />);
    expect(await screen.findByText('access_denied')).toBeInTheDocument();

    expect(trackOnNextPage).not.toHaveBeenCalled();
    expect(track).not.toHaveBeenCalled();
  });
});

describe('agent_claim', () => {
  const agent = { id: 'agent-1', display_name: 'Claude Helper', bio: '', reputation: 42, status: 'active', created_at: '2026-01-15T10:00:00Z' };

  it('is kept for the next page when the claim link is used: /claim is never seen by the tag', async () => {
    Object.assign(auth, { isAuthenticated: true, user: { id: 'u1', type: 'human', displayName: 'Jane' } });
    window.sessionStorage.setItem('solvr_pending_claim_token', 'claim-token-abc');
    vi.mocked(api.getClaimInfo).mockResolvedValue({ token_valid: true, agent, expires_at: new Date(Date.now() + 3600000).toISOString() } as never);
    vi.mocked(api.claimAgent).mockResolvedValue({ agent } as never);
    render(<ClaimPage />);

    fireEvent.click(await screen.findByRole('button', { name: /claim this agent/i }));
    expect(await screen.findByText('Successfully Claimed!')).toBeInTheDocument();
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage).toHaveBeenCalledWith('agent_claim', { surface: 'claim_page' });
    expect(track).not.toHaveBeenCalled();
    expect(everythingSent()).not.toMatch(/claim-token|agent-1|Claude/);
  });

  it('is not sent when the API refused the claim', async () => {
    Object.assign(auth, { isAuthenticated: true, user: { id: 'u1', type: 'human', displayName: 'Jane' } });
    window.sessionStorage.setItem('solvr_pending_claim_token', 'claim-token-abc');
    vi.mocked(api.getClaimInfo).mockResolvedValue({ token_valid: true, agent, expires_at: new Date(Date.now() + 3600000).toISOString() } as never);
    vi.mocked(api.claimAgent).mockRejectedValue(new Error('Token already used'));
    render(<ClaimPage />);

    fireEvent.click(await screen.findByRole('button', { name: /claim this agent/i }));
    expect(await screen.findByText('Token already used')).toBeInTheDocument();
    expect(trackOnNextPage).not.toHaveBeenCalled();
  });

  it('is kept for the next page when the token is pasted in settings: the page reloads right after', async () => {
    stubLocation('/settings/agents');
    Object.assign(auth, { isAuthenticated: true, user: { id: 'u1', type: 'human', displayName: 'Jane' } });
    vi.mocked(api.claimAgent).mockResolvedValue({ agent } as never);
    render(<ClaimAgentForm />);

    fireEvent.change(screen.getByPlaceholderText('Paste your claim token here'), { target: { value: 'claim-token-abc' } });
    fireEvent.click(screen.getByRole('button', { name: /claim agent/i }));

    expect(await screen.findByText(/Successfully claimed/)).toBeInTheDocument();
    expect(trackOnNextPage).toHaveBeenCalledTimes(1);
    expect(trackOnNextPage).toHaveBeenCalledWith('agent_claim', { surface: 'settings' });
    expect(everythingSent()).not.toMatch(/claim-token|Claude/);
  });

  it('is not sent from settings when the API refused the token', async () => {
    Object.assign(auth, { isAuthenticated: true, user: { id: 'u1', type: 'human', displayName: 'Jane' } });
    vi.mocked(api.claimAgent).mockRejectedValue(new Error('Invalid token'));
    render(<ClaimAgentForm />);

    fireEvent.change(screen.getByPlaceholderText('Paste your claim token here'), { target: { value: 'nope' } });
    fireEvent.click(screen.getByRole('button', { name: /claim agent/i }));
    expect(await screen.findByText('Invalid token')).toBeInTheDocument();
    expect(trackOnNextPage).not.toHaveBeenCalled();
    expect(track).not.toHaveBeenCalled();
  });
});

describe('api_key_create and api_key_copy (/settings/api-keys)', () => {
  const created = { data: { id: 'key-1', name: 'Production', key: 'solvr_sk_live_SECRETSECRET', key_preview: 'solvr_sk_…RET', created_at: '2026-10-05T00:00:00Z' } };

  async function createKey() {
    render(<APIKeysPage />);
    fireEvent.click(screen.getByRole('button', { name: /create your first key/i }));
    fireEvent.change(screen.getByLabelText('KEY NAME'), { target: { value: 'Production' } });
    // Two buttons say CREATE KEY: the one that opens the dialog, and the dialog's own.
    fireEvent.click(screen.getAllByRole('button', { name: /^create key$/i }).at(-1)!);
  }

  it('sends api_key_create once the API issued the key: never the key or its name', async () => {
    keys.createKey.mockResolvedValue(created);
    await createKey();

    expect(await screen.findByText('KEY CREATED')).toBeInTheDocument();
    expect(track).toHaveBeenCalledTimes(1);
    expect(track.mock.calls[0][0]).toBe('api_key_create');
    expect(track.mock.calls[0][1]).toBeUndefined();
    expect(everythingSent()).not.toMatch(/solvr_sk|SECRET|Production/);
  });

  it('does not send api_key_create when the API refused', async () => {
    keys.createKey.mockRejectedValue(new Error('Key limit reached'));
    await createKey();

    expect(await screen.findByText('Key limit reached')).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();
  });

  it('sends api_key_copy, name and surface only, once the clipboard took the key', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    keys.createKey.mockResolvedValue(created);
    await createKey();
    await screen.findByText('KEY CREATED');
    track.mockClear();

    const copy = screen.getByDisplayValue('solvr_sk_live_SECRETSECRET').parentElement!.querySelector('button')!;
    fireEvent.click(copy);

    expect(await screen.findByText('Copied to clipboard!')).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith('solvr_sk_live_SECRETSECRET');
    expect(track.mock.calls).toEqual([['api_key_copy', { surface: 'settings' }]]);
    expect(everythingSent()).not.toMatch(/solvr_sk|SECRET|Production/);
  });

  it('does not send api_key_copy, and does not say Copied, when the clipboard is blocked', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    Object.assign(navigator, { clipboard: { writeText } });
    keys.createKey.mockResolvedValue(created);
    await createKey();
    await screen.findByText('KEY CREATED');
    track.mockClear();

    fireEvent.click(screen.getByDisplayValue('solvr_sk_live_SECRETSECRET').parentElement!.querySelector('button')!);
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
    expect(screen.queryByText('Copied to clipboard!')).toBeNull();
  });
});
