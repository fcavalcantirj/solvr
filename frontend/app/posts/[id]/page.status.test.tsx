import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Task idx 83: a post the API does not have answers a real 404 (it used to render a
// 200 "Could not load" page); an API failure answers a retryable 5xx.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/post-detail', () => ({ PostDetail: () => null }));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));

import PostPage from './page';

const fetchMock = vi.fn();
const params = { params: Promise.resolve({ id: 'p1' }) };
beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('post page status', () => {
  it('answers a real 404 for a post the API does not have', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, json: async () => ({}) });
    await expect(PostPage(params)).rejects.toThrow('NEXT_NOT_FOUND');
  });

  it('fails retryably, never as a 404, when the API fails or is unreachable', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 502, json: async () => ({}) });
    await expect(PostPage(params)).rejects.toThrow(/502/);
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(PostPage(params)).rejects.toThrow(/unreachable/);
    expect(notFound).not.toHaveBeenCalled();
  });
});
