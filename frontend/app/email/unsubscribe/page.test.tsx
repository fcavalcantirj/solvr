import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import UnsubscribePage from './page';

// The page an email's footer links (SPEC.md 16.6). It shows the masked address the API put in
// the link and one Unsubscribe button, and makes no call on load: a link scanner or a prefetch
// that opens the page unsubscribes no one. The click POSTs RFC 8058's one-click body.

let search = '';
vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(search),
}));

const API = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';
const LINK = 'email=alice%2Btag%40example.com&masked=a***%40example.com&token=t0k';

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  });
  global.fetch = fn as unknown as typeof fetch;
  return fn;
}

describe('UnsubscribePage', () => {
  const originalFetch = global.fetch;
  beforeEach(() => {
    vi.clearAllMocks();
    search = LINK;
  });
  afterEach(() => {
    global.fetch = originalFetch;
  });

  it('makes no call on load and shows the masked address with one button', async () => {
    const fetchMock = mockFetch(200, {});
    render(<UnsubscribePage />);
    expect(await screen.findByText(/a\*\*\*@example\.com/)).toBeDefined();
    expect(screen.getAllByRole('button')).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Unsubscribe' })).toBeDefined();
    await new Promise((r) => setTimeout(r, 20));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('POSTs the one-click body on click and shows the API message', async () => {
    const fetchMock = mockFetch(200, { status: 'unsubscribed', message: 'You have been unsubscribed from Solvr emails.' });
    render(<UnsubscribePage />);
    fireEvent.click(await screen.findByRole('button', { name: 'Unsubscribe' }));

    await waitFor(() => expect(screen.getByText('You have been unsubscribed from Solvr emails.')).toBeDefined());
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API}/v1/email/unsubscribe?email=alice%2Btag%40example.com&token=t0k`);
    expect(init).toMatchObject({
      method: 'POST',
      body: 'List-Unsubscribe=One-Click',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
  });

  it('shows the message from the standard error envelope', async () => {
    mockFetch(403, { error: { code: 'INVALID_TOKEN', message: 'invalid unsubscribe token', request_id: 'r1' } });
    render(<UnsubscribePage />);
    fireEvent.click(await screen.findByRole('button', { name: 'Unsubscribe' }));
    await waitFor(() => expect(screen.getByText('invalid unsubscribe token')).toBeDefined());
  });

  it('says a network error happened', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('offline')) as unknown as typeof fetch;
    render(<UnsubscribePage />);
    fireEvent.click(await screen.findByRole('button', { name: 'Unsubscribe' }));
    await waitFor(() => expect(screen.getByText('Network error. Please try again later.')).toBeDefined());
  });

  it('says a link without its address or token is invalid, and offers no button', async () => {
    const fetchMock = mockFetch(200, {});
    search = 'email=alice%40example.com';
    render(<UnsubscribePage />);
    expect(await screen.findByText('Invalid unsubscribe link. Missing email or token.')).toBeDefined();
    expect(screen.queryByRole('button')).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
