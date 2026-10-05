import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { NOINDEX } from '@/lib/seo/route-policy';

// Task idx 80: content pages render the API's seo verdict as robots and description
// metadata; collection pages keep internal search results and filter combinations
// out of the index while the bare collection stays indexable.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/posts-page-client', () => ({ PostsPageClient: () => null }));
vi.mock('@/components/posts/post-detail', () => ({ PostDetail: () => null }));
vi.mock('@/components/rooms/rooms-browser', () => ({ RoomsBrowser: () => null }));
vi.mock('@/components/rooms/recently-viewed-rooms', () => ({ RecentlyViewedRooms: () => null }));
vi.mock('@/components/rooms/create-room-dialog', () => ({ CreateRoomDialog: () => null }));
vi.mock('@/components/rooms/room-detail-client', () => ({ RoomDetailClient: () => null }));
vi.mock('@/components/rooms/private-room-view', () => ({ PrivateRoomView: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, roomJsonLd: () => ({}), breadcrumbJsonLd: () => ({}) }));

import * as postsPage from './posts/page';
import * as roomsPage from './rooms/page';
import * as postPage from './posts/[id]/page';
import * as roomPage from './rooms/[slug]/page';

const fetchMock = vi.fn();
const answer = (status: number, body: unknown) =>
  fetchMock.mockResolvedValue({ ok: status < 300, status, json: async () => body });
// answerBy routes the parent read and its /seo verdict to their own answers.
const answerBy = (parent: [number, unknown], seo: [number, unknown]) =>
  fetchMock.mockImplementation(async (url: string) => {
    const [status, body] = String(url).endsWith('/seo') ? seo : parent;
    return { ok: status < 300, status, json: async () => body };
  });
// unreachableVerdict answers the parent read and lets the /seo read fail on the network.
const unreachableVerdict = (parent: [number, unknown]) =>
  fetchMock.mockImplementation(async (url: string) => {
    if (String(url).endsWith('/seo')) throw new TypeError('fetch failed');
    const [status, body] = parent;
    return { ok: status < 300, status, json: async () => body };
  });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const query = (q: Record<string, string>) => ({ searchParams: Promise.resolve(q) });

describe('collection pages', () => {
  it('index the bare /posts and /rooms with their canonical', async () => {
    for (const page of [postsPage, roomsPage]) {
      const metadata = await page.generateMetadata(query({}));
      expect(metadata.robots).toBeUndefined();
      expect(metadata.alternates?.canonical).toMatch(/^\/(posts|rooms)$/);
    }
  });

  it('keep search results and filters out of the index', async () => {
    expect((await postsPage.generateMetadata(query({ q: 'planner' }))).robots).toEqual(NOINDEX);
    expect((await roomsPage.generateMetadata(query({ sort: 'top' }))).robots).toEqual(NOINDEX);
  });

  it('treat a tracking-only query as the canonical page', async () => {
    expect((await postsPage.generateMetadata(query({ utm_source: 'x' }))).robots).toBeUndefined();
  });
});

describe('post page', () => {
  const post = { data: { id: 'p1', title: 'Hand a plan to an executor', description: '## raw *markdown*' } };
  const seo = (indexable: boolean, description: string) => ({ data: { indexable, description } });
  const params = { params: Promise.resolve({ id: 'p1' }) };

  it('uses the API description and stays indexable when the API says so', async () => {
    answerBy([200, post], [200, seo(true, 'Hand a plan from the API')]);
    const metadata = await postPage.generateMetadata(params);
    expect(metadata.robots).toBeUndefined();
    expect(metadata.description).toBe('Hand a plan from the API');
    expect(metadata.alternates?.canonical).toBe('/posts/p1');
    const seoCall = fetchMock.mock.calls.find(([u]) => String(u).endsWith('/v1/posts/p1/seo'));
    expect(seoCall?.[1]).toMatchObject({ cache: 'no-store' });
  });

  it('is noindex when the API says the post may not be indexed', async () => {
    answerBy([200, post], [200, seo(false, 'rejected')]);
    expect((await postPage.generateMetadata(params)).robots).toEqual(NOINDEX);
  });

  // SPEC 27.4: an API failure answers a retryable 500, never a "not indexable". A failed
  // verdict read used to come back as null and was rendered as noindex: one bad minute
  // of the API told a crawler to drop a healthy page.
  it.each([500, 502, 503, 429])('fails retryably, never as noindex, when the verdict read answers %i', async (status) => {
    answerBy([200, post], [status, null]);
    await expect(postPage.generateMetadata(params)).rejects.toThrow(new RegExp(`/v1/posts/p1/seo.*${status}`));
    await expect(postPage.default(params)).rejects.toThrow(new RegExp(`${status}`));
  });

  it('fails retryably, never as noindex, when the API cannot be reached for the verdict', async () => {
    unreachableVerdict([200, post]);
    await expect(postPage.generateMetadata(params)).rejects.toThrow(/unreachable/);
    await expect(postPage.default(params)).rejects.toThrow(/unreachable/);
  });

  // A refusal is an answer, not a failure: the API answers 404 for the verdict exactly
  // when it does for the post, so a post withdrawn between the two reads is noindex.
  it('is noindex when the API refuses the verdict', async () => {
    answerBy([200, post], [404, { error: { code: 'NOT_FOUND' } }]);
    expect((await postPage.generateMetadata(params)).robots).toEqual(NOINDEX);
  });
});

describe('room page', () => {
  const roomBody = { data: { room: { display_name: 'raw name', description: 'raw *desc*' } } };
  const room = (seo: { indexable: boolean; title: string; description: string }) =>
    answerBy([200, roomBody], [200, { data: seo }]);
  const params = { params: Promise.resolve({ slug: 'kestrel' }) };

  it('uses the API title and description and stays indexable when the API says so', async () => {
    room({ indexable: true, title: 'Kestrel build', description: 'Plan and ship kestrel' });
    const metadata = await roomPage.generateMetadata(params);
    expect(metadata.robots).toBeUndefined();
    expect(metadata.title).toBe('Kestrel build');
    expect(metadata.description).toBe('Plan and ship kestrel');
    expect(metadata.alternates?.canonical).toBe('/rooms/kestrel');
  });

  it('is noindex for a thin room', async () => {
    room({ indexable: false, title: 'Kestrel build', description: 'x' });
    expect((await roomPage.generateMetadata(params)).robots).toEqual(NOINDEX);
  });

  it('is noindex for a private room the server cannot read', async () => {
    answer(403, null);
    expect((await roomPage.generateMetadata(params)).robots).toEqual(NOINDEX);
  });

  // SPEC 27.4, as for a post: a failed verdict read is a retryable 500, never noindex.
  it.each([500, 502, 503, 429])('fails retryably, never as noindex, when the verdict read answers %i', async (status) => {
    answerBy([200, roomBody], [status, null]);
    await expect(roomPage.generateMetadata(params)).rejects.toThrow(new RegExp(`/v1/rooms/kestrel/seo.*${status}`));
    await expect(roomPage.default(params)).rejects.toThrow(new RegExp(`${status}`));
  });

  it('fails retryably, never as noindex, when the API cannot be reached for the verdict', async () => {
    unreachableVerdict([200, roomBody]);
    await expect(roomPage.generateMetadata(params)).rejects.toThrow(/unreachable/);
    await expect(roomPage.default(params)).rejects.toThrow(/unreachable/);
  });

  // The room turned private or was deleted between the two reads: a refusal, so noindex.
  it.each([403, 404])('is noindex when the API refuses the verdict with %i', async (status) => {
    answerBy([200, roomBody], [status, null]);
    expect((await roomPage.generateMetadata(params)).robots).toEqual(NOINDEX);
  });
});
