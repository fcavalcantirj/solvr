import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ARCHIVE_PAGE_SIZE, archivePageNumber, readIndexablePosts } from './indexable-posts';
import { UpstreamError } from './read-for-page';

// SPEC.md 27.2: the post archive and the profile pages read GET /v1/posts?indexable=true,
// the list of exactly the posts the post sitemap lists. The read is a server read of a
// page's own content: a failure must fail the page (a retryable 5xx), never render it empty,
// because a page without its links would silently take every post it carried out of reach.

const fetchMock = vi.fn();
const ok = (body: unknown) => fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => body });
const LIST = { data: [{ id: 'p1', title: 'A post' }], meta: { total: 1, page: 1, per_page: 50, total_pages: 1, has_more: false } };

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('readIndexablePosts', () => {
  it('reads one archive page: indexable posts, newest first, fifty a page, never from a stored copy', async () => {
    ok(LIST);
    expect(await readIndexablePosts({ page: 3 })).toEqual(LIST);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(new URL(String(url)).pathname + new URL(String(url)).search).toBe(
      '/v1/posts?indexable=true&sort=new&page=3&per_page=50',
    );
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(ARCHIVE_PAGE_SIZE).toBe(50);
  });

  it("reads one author's first page for a profile, the author's id escaped", async () => {
    ok(LIST);
    await readIndexablePosts({ page: 1, author: { type: 'agent', id: 'agent one&x' } });
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname + url.search).toBe(
      '/v1/posts?indexable=true&sort=new&page=1&per_page=50&author_type=agent&author_id=agent%20one%26x',
    );
    expect(url.searchParams.get('author_id')).toBe('agent one&x');
  });

  it.each([500, 502, 503, 429])('fails, retryably, when the API answers %i', async (status) => {
    fetchMock.mockResolvedValue({ ok: false, status, json: async () => ({}) });
    await expect(readIndexablePosts({ page: 1 })).rejects.toBeInstanceOf(UpstreamError);
  });

  // The list refuses nothing a page could turn into a 404: a 4xx here is the API and the
  // page disagreeing, and an empty archive would drop every link it carries.
  it.each([400, 404])('fails rather than list nothing when the API answers %i', async (status) => {
    fetchMock.mockResolvedValue({ ok: false, status, json: async () => ({}) });
    await expect(readIndexablePosts({ page: 1 })).rejects.toThrow(new RegExp(`/v1/posts.*${status}`));
    await expect(readIndexablePosts({ page: 1 })).rejects.toBeInstanceOf(UpstreamError);
  });

  it('fails when the API cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(readIndexablePosts({ page: 1 })).rejects.toThrow(/unreachable/);
  });

  it('fails when the answer is not a list', async () => {
    ok({ data: null });
    await expect(readIndexablePosts({ page: 1 })).rejects.toBeInstanceOf(UpstreamError);
  });
});

describe('archivePageNumber', () => {
  it('reads a page number written the one canonical way', () => {
    expect(archivePageNumber('1')).toBe(1);
    expect(archivePageNumber('12')).toBe(12);
    expect(archivePageNumber('467')).toBe(467);
  });

  it.each(['0', '01', '-1', '1.5', 'x', '', '1x', ' 1', '1e3', '9999999999'])('refuses %j', (raw) => {
    expect(archivePageNumber(raw)).toBeNull();
  });
});
