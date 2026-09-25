import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import UnsubscribePage from './page';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams('email=a%40b.c&token=t'),
}));

function mockFetch(status: number, body: unknown) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  }) as unknown as typeof fetch;
}

describe('UnsubscribePage', () => {
  const originalFetch = global.fetch;
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => {
    global.fetch = originalFetch;
  });

  it('shows the API success message', async () => {
    mockFetch(200, { status: 'unsubscribed', message: 'You have been unsubscribed from Solvr emails.' });
    render(<UnsubscribePage />);
    await waitFor(() => expect(screen.getByText('You have been unsubscribed from Solvr emails.')).toBeDefined());
  });

  it('shows the message from the standard error envelope', async () => {
    mockFetch(403, { error: { code: 'INVALID_TOKEN', message: 'invalid unsubscribe token', request_id: 'r1' } });
    render(<UnsubscribePage />);
    await waitFor(() => expect(screen.getByText('invalid unsubscribe token')).toBeDefined());
  });
});
