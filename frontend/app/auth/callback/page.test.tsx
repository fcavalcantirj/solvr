import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import AuthCallbackPage from './page';

// idx 75 step 5: the OAuth redirect carries a one-time login code, never a JWT. The page trades
// the code for a token with a POST (the token travels in a response body, where no URL, history
// entry or analytics page view records it) and takes the code out of the address bar at once.

let mockQuery = new URLSearchParams();
vi.mock('next/navigation', () => ({
  useSearchParams: () => mockQuery,
}));

const mockSetToken = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({ setToken: mockSetToken }),
}));

const mockFetch = vi.fn();

function exchangeOk(token = 'jwt-from-exchange') {
  return { ok: true, status: 200, json: async () => ({ data: { access_token: token, token_type: 'Bearer', expires_in: 900 } }) };
}

describe('AuthCallbackPage', () => {
  let replaceState: ReturnType<typeof vi.spyOn>;
  let locationState: { href: string };

  beforeEach(() => {
    vi.clearAllMocks();
    mockQuery = new URLSearchParams();
    mockSetToken.mockResolvedValue(undefined);
    vi.stubGlobal('fetch', mockFetch);
    replaceState = vi.spyOn(window.history, 'replaceState');
    locationState = { href: '' };
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { get href() { return locationState.href; }, set href(v: string) { locationState.href = v; }, pathname: '/auth/callback' },
    });
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    replaceState.mockRestore();
  });

  it('exchanges the login code by POST and signs in with the token from the response body', async () => {
    mockQuery = new URLSearchParams('code=solvr_lc_abc');
    mockFetch.mockResolvedValueOnce(exchangeOk());

    render(<AuthCallbackPage />);

    await waitFor(() => expect(mockSetToken).toHaveBeenCalledWith('jwt-from-exchange'));
    const [url, init] = mockFetch.mock.calls[0];
    expect(String(url)).toMatch(/\/v1\/auth\/oauth\/exchange$/);
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ login_code: 'solvr_lc_abc' });
    expect(String(url)).not.toContain('solvr_lc_abc');
    await waitFor(() => expect(locationState.href).toBe('/feed'));
  });

  it('takes the code out of the address bar before it does anything else', async () => {
    mockQuery = new URLSearchParams('code=solvr_lc_abc');
    mockFetch.mockResolvedValueOnce(exchangeOk());

    render(<AuthCallbackPage />);

    await waitFor(() => expect(replaceState).toHaveBeenCalled());
    const cleaned = String(replaceState.mock.calls[0][2]);
    expect(cleaned).toBe('/auth/callback');
    expect(cleaned).not.toContain('code');
    expect(replaceState.mock.invocationCallOrder[0]).toBeLessThan(mockFetch.mock.invocationCallOrder[0]);
  });

  it('no longer accepts a token in the URL: it is not exchanged, stored or trusted', async () => {
    mockQuery = new URLSearchParams('token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln');

    render(<AuthCallbackPage />);

    expect(await screen.findByText('AUTHENTICATION_FAILED')).toBeTruthy();
    expect(mockSetToken).not.toHaveBeenCalled();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('shows the API refusal and signs nobody in when the code is invalid, expired or used', async () => {
    mockQuery = new URLSearchParams('code=solvr_lc_used');
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: { code: 'INVALID_LOGIN_CODE', message: 'This login link is invalid, expired or already used. Sign in again.' } }),
    });

    render(<AuthCallbackPage />);

    expect(await screen.findByText('This login link is invalid, expired or already used. Sign in again.')).toBeTruthy();
    expect(mockSetToken).not.toHaveBeenCalled();
    expect(locationState.href).toBe('');
  });

  it('shows a failure when the exchange cannot be reached', async () => {
    mockQuery = new URLSearchParams('code=solvr_lc_abc');
    mockFetch.mockRejectedValueOnce(new Error('network down'));

    render(<AuthCallbackPage />);

    expect(await screen.findByText('AUTHENTICATION_FAILED')).toBeTruthy();
    expect(mockSetToken).not.toHaveBeenCalled();
  });

  it('shows the provider error when the redirect carries one', async () => {
    mockQuery = new URLSearchParams('error=access_denied');

    render(<AuthCallbackPage />);

    expect(await screen.findByText('access_denied')).toBeTruthy();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('claims a stored referral with the token from the exchange, then returns to the saved page', async () => {
    mockQuery = new URLSearchParams('code=solvr_lc_abc');
    localStorage.setItem('solvr_referral_code', 'REF12345');
    localStorage.setItem('auth_return_url', '/rooms');
    mockFetch.mockResolvedValueOnce(exchangeOk('jwt-2')).mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({}) });

    render(<AuthCallbackPage />);

    await waitFor(() => expect(locationState.href).toBe('/rooms'));
    const [claimUrl, claimInit] = mockFetch.mock.calls[1];
    expect(String(claimUrl)).toMatch(/\/v1\/auth\/claim-referral$/);
    expect(claimInit.headers.Authorization).toBe('Bearer jwt-2');
    expect(localStorage.getItem('solvr_referral_code')).toBeNull();
  });
});
