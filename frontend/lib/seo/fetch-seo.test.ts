import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fetchSEO } from './fetch-seo';
import { UpstreamError } from './read-for-page';

// fetchSEO reads a page's search verdict for server rendering. A refusal is an answer
// (null, which pages render as noindex); a failure is not: SPEC 27.4 says an API failure
// answers a retryable 500, never a "not indexable".

const fetchMock = vi.fn();
const answers = (status: number, body: unknown = null) =>
  fetchMock.mockResolvedValue({ ok: status < 300, status, json: async () => body });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchSEO', () => {
  it('returns the verdict the API served, read without a stored copy', async () => {
    const verdict = { indexable: true, title: 'A post', description: 'What it says.' };
    answers(200, { data: verdict });

    await expect(fetchSEO('/v1/posts/p1/seo')).resolves.toEqual(verdict);

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/v1\/posts\/p1\/seo$/);
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it.each([404, 401, 403])('answers null when the API refuses with %i', async (status) => {
    answers(status, { error: { code: 'REFUSED' } });
    await expect(fetchSEO('/v1/rooms/kestrel/seo')).resolves.toBeNull();
  });

  it.each([500, 502, 503, 504, 429])('throws, instead of answering null, when the API fails with %i', async (status) => {
    answers(status);
    const read = fetchSEO('/v1/posts/p1/seo');
    await expect(read).rejects.toBeInstanceOf(UpstreamError);
    await expect(read).rejects.toThrow(`/v1/posts/p1/seo: API answered ${status}`);
  });

  it('throws, instead of answering null, when the API cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    const read = fetchSEO('/v1/posts/p1/seo');
    await expect(read).rejects.toBeInstanceOf(UpstreamError);
    await expect(read).rejects.toThrow('/v1/posts/p1/seo: API unreachable');
  });

  it('answers null for a 200 that carries no verdict', async () => {
    answers(200, {});
    await expect(fetchSEO('/v1/posts/p1/seo')).resolves.toBeNull();
  });
});
