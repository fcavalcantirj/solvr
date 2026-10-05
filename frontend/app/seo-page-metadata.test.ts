import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Metadata } from 'next';
import { INDEXABLE_PATTERNS, NOINDEX } from '@/lib/seo/route-policy';
import { PREVIEW_CARD_PATH } from '@/lib/seo/link-preview';
import { previewProblems, readPreview } from '@/lib/seo/preview-check';

// Task idx 80: content pages render the API's seo verdict as robots and description
// metadata; collection pages keep internal search results and filter combinations
// out of the index while the bare collection stays indexable.
//
// Link previews (recon finding F04): every kind of content page states its own preview,
// under the title its title tag shows, its canonical as the address, and a picture.

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
vi.mock('@/components/seo/json-ld', () => ({
  JsonLd: () => null,
  roomJsonLd: () => ({}),
  breadcrumbJsonLd: () => ({}),
  postPageJsonLd: () => ({}),
  agentJsonLd: () => ({}),
  userJsonLd: () => ({}),
  blogPostJsonLd: () => ({}),
}));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('@/components/agents/agent-profile-client', () => ({ AgentProfileClient: () => null }));
vi.mock('@/components/users/user-profile-client', () => ({ UserProfileClient: () => null }));
vi.mock('@/components/prompt/guide-prompt', () => ({ GuidePrompt: () => null }));
vi.mock('./blog/[slug]/blog-post-content', () => ({ BlogPostContent: () => null }));
vi.mock('next/font/google', () => ({ Inter: () => ({}), JetBrains_Mono: () => ({}) }));
vi.mock('@next/third-parties/google', () => ({ GoogleAnalytics: () => null }));

import { metadata as rootLayout } from './layout';
import * as postsPage from './posts/page';
import * as roomsPage from './rooms/page';
import * as postPage from './posts/[id]/page';
import * as repliesPage from './posts/[id]/replies/[page]/page';
import * as archivePage from './posts/page/[n]/page';
import * as roomPage from './rooms/[slug]/page';
import * as transcriptPage from './rooms/[slug]/history/[page]/page';
import * as agentPage from './agents/[id]/page';
import * as userPage from './users/[id]/page';
import * as blogPostPage from './blog/[slug]/page';
import * as guidePage from './docs/guides/[slug]/page';

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

// serve answers each API path in the table with its body, and 404 for any other path.
const serve = (table: Record<string, unknown>) =>
  fetchMock.mockImplementation(async (url: string) => {
    const { pathname, search } = new URL(String(url));
    const body = table[`${pathname}${search}`];
    return body === undefined
      ? { ok: false, status: 404, json: async () => ({ error: { code: 'NOT_FOUND' } }) }
      : { ok: true, status: 200, json: async () => body };
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

describe('link previews of content pages', () => {
  const HOME_TITLE = (rootLayout.title as { default: string }).default;
  const CARD = `https://solvr.dev${PREVIEW_CARD_PATH}`;

  const POST = {
    data: {
      id: 'p1',
      title: 'Hand a plan to an executor',
      description: '## raw *markdown*',
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-02T00:00:00Z',
      tags: ['agents', 'planning'],
    },
  };
  // What the API's /seo read answers: the title and the description the page shows.
  const POST_SEO = { data: { indexable: true, title: 'Hand a plan to an executor — Dev Nine', description: 'Hand a plan from the API' } };
  const REPLIES = {
    data: [{ id: 'r2a', body: 'reply', author: { id: 'agent_x', type: 'agent', display_name: 'executor' }, created_at: '2026-09-01T00:00:00Z' }],
    meta: { total: 250, page: 2, per_page: 100, total_pages: 3, has_more: true },
  };
  // Page 2 of the indexable posts, as GET /v1/posts?indexable=true answers it (SPEC.md 27.2).
  const ARCHIVE = {
    data: [{ id: 'p51', title: 'Post 51', created_at: '2026-09-01T00:00:00Z', author: { id: 'agent_x', type: 'agent', display_name: 'executor' } }],
    meta: { total: 467, page: 2, per_page: 50, total_pages: 10, has_more: true },
  };
  const ROOM = { data: { room: { slug: 'kestrel', display_name: 'raw name', description: 'raw *desc*' } } };
  const ROOM_SEO = { data: { indexable: true, title: 'Kestrel build', description: 'Plan and ship kestrel' } };
  const HISTORY = {
    data: {
      page: 2,
      page_size: 100,
      from_sequence: 101,
      to_sequence: 200,
      total_pages: 3,
      prev_page: 1,
      next_page: 3,
      messages: [{ id: 7, author_type: 'agent', author_id: 'agent_planner', agent_name: 'planner', content: 'Plan', sequence_num: 101, created_at: '2026-09-01T10:00:00Z' }],
    },
  };
  const BLOG_POST = {
    slug: 'a-post',
    title: 'A listed blog post',
    body: 'Body',
    excerpt: 'An excerpt',
    meta_description: 'What the post says',
    created_at: '2026-09-01T00:00:00Z',
    published_at: '2026-09-03T00:00:00Z',
    updated_at: '2026-09-04T00:00:00Z',
    tags: ['agents'],
    cover_image_url: '',
  };

  interface ContentPage {
    api: Record<string, unknown>;
    load: () => Promise<Metadata>;
    path: string;
    title: string;
    description: string;
    type: string;
  }

  const pages: Record<string, ContentPage> = {
    post: {
      api: { '/v1/posts/p1': POST, '/v1/posts/p1/seo': POST_SEO },
      load: () => postPage.generateMetadata({ params: Promise.resolve({ id: 'p1' }) }),
      path: '/posts/p1',
      title: 'Hand a plan to an executor — Dev Nine | Solvr',
      description: 'Hand a plan from the API',
      type: 'article',
    },
    'replies page': {
      api: { '/v1/posts/p1': POST, '/v1/posts/p1/seo': POST_SEO, '/v1/posts/p1/replies?page=2': REPLIES },
      load: () => repliesPage.generateMetadata({ params: Promise.resolve({ id: 'p1', page: '2' }) }),
      path: '/posts/p1/replies/2',
      title: 'Hand a plan to an executor: replies, page 2 of 3 | Solvr',
      description: 'Replies page 2 of 3 on "Hand a plan to an executor", a Solvr post.',
      type: 'website',
    },
    'post archive page': {
      api: { '/v1/posts?indexable=true&sort=new&page=2&per_page=50': ARCHIVE },
      load: () => archivePage.generateMetadata({ params: Promise.resolve({ n: '2' }) }),
      path: '/posts/page/2',
      title: 'Posts, page 2 of 10 | Solvr',
      description: 'Page 2 of 10 of every post on Solvr, newest first: problems, questions and ideas from humans and AI agents.',
      type: 'website',
    },
    room: {
      api: { '/v1/rooms/kestrel': ROOM, '/v1/rooms/kestrel/seo': ROOM_SEO },
      load: () => roomPage.generateMetadata({ params: Promise.resolve({ slug: 'kestrel' }) }),
      path: '/rooms/kestrel',
      title: 'Kestrel build | Solvr',
      description: 'Plan and ship kestrel',
      type: 'website',
    },
    'transcript page': {
      api: { '/v1/rooms/kestrel': ROOM, '/v1/rooms/kestrel/seo': ROOM_SEO, '/v1/rooms/kestrel/history/2': HISTORY },
      load: () => transcriptPage.generateMetadata({ params: Promise.resolve({ slug: 'kestrel', page: '2' }) }),
      path: '/rooms/kestrel/history/2',
      title: 'Kestrel build: transcript, messages 101–200 | Solvr',
      description: 'Page 2 of 3 of the Kestrel build room transcript on Solvr, messages 101–200.',
      type: 'website',
    },
    // A profile's title and description are its /seo read's (SPEC.md 27.1), never the raw bio.
    agent: {
      api: {
        '/v1/agents/agent_one': { data: { agent: { id: 'agent_one', display_name: 'Listed Agent', bio: 'Plans **builds**' } } },
        '/v1/agents/agent_one/seo': { data: { indexable: true, title: 'Listed Agent (AI agent)', description: 'Listed Agent, an AI agent on Solvr: 2 posts. Plans builds' } },
      },
      load: () => agentPage.generateMetadata({ params: Promise.resolve({ id: 'agent_one' }) }),
      path: '/agents/agent_one',
      title: 'Listed Agent (AI agent) | Solvr',
      description: 'Listed Agent, an AI agent on Solvr: 2 posts. Plans builds',
      type: 'profile',
    },
    user: {
      api: {
        '/v1/users/u1': { data: { id: 'u1', username: 'listed', display_name: 'Listed User', bio: 'Reviews plans' } },
        '/v1/users/u1/seo': { data: { indexable: true, title: 'Listed User (@listed)', description: 'Listed User on Solvr: 1 post. Reviews plans' } },
      },
      load: () => userPage.generateMetadata({ params: Promise.resolve({ id: 'u1' }) }),
      path: '/users/u1',
      title: 'Listed User (@listed) | Solvr',
      description: 'Listed User on Solvr: 1 post. Reviews plans',
      type: 'profile',
    },
    'blog post': {
      api: { '/v1/blog/a-post': { data: BLOG_POST } },
      load: () => blogPostPage.generateMetadata({ params: Promise.resolve({ slug: 'a-post' }) }),
      path: '/blog/a-post',
      title: 'A listed blog post | Solvr',
      description: 'What the post says',
      type: 'article',
    },
    guide: {
      api: {},
      load: () => guidePage.generateMetadata({ params: Promise.resolve({ slug: 'connect-planner-executor' }) }),
      path: '/docs/guides/connect-planner-executor',
      title: 'Connect a planner and an executor | Solvr',
      description: 'One agent plans and gives orders; the other builds and reports back.',
      type: 'website',
    },
  };

  it.each(Object.entries(pages))('a %s previews under its own title, description and address, with the card', async (_kind, page) => {
    serve(page.api);
    const metadata = await page.load();
    expect(metadata.alternates?.canonical).toBe(page.path);
    expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
    const preview = readPreview(metadata);
    expect(preview).toMatchObject({
      title: page.title,
      description: page.description,
      url: `https://solvr.dev${page.path}`,
      type: page.type,
      image: CARD,
      twitterCard: 'summary_large_image',
      twitterImage: CARD,
    });
    expect(preview.images[0]).toMatchObject({ width: 1200, height: 630, type: 'image/png' });
  });

  // The policy's indexable patterns (lib/seo/route-policy.ts) are pages the API counts; each
  // is one of the kinds above, indexable under its own canonical.
  it('checks every indexable pattern of the route policy', async () => {
    const checked = Object.values(pages).filter((page) => INDEXABLE_PATTERNS.some((pattern) => pattern.matches(page.path)));
    expect(checked.map((page) => page.path)).toEqual(['/posts/page/2']);
    expect(INDEXABLE_PATTERNS).toHaveLength(checked.length);
    for (const page of checked) {
      serve(page.api);
      expect((await page.load()).robots, page.path).toBeUndefined();
    }
  });

  // The API composes what a post or a room is called and how it is described (/seo);
  // the preview repeats that, never the raw post or room.
  it('a post and a room preview under what the API\'s /seo read answers', async () => {
    serve(pages.post.api);
    const post = readPreview(await pages.post.load());
    expect(post.title).toContain('Dev Nine');
    expect(post.description).not.toContain('raw');
    serve(pages.room.api);
    const room = readPreview(await pages.room.load());
    expect(room.title).not.toContain('raw name');
    expect(room.description).not.toContain('raw');
  });

  it('a post keeps its dates and tags', async () => {
    serve(pages.post.api);
    expect((await pages.post.load()).openGraph).toMatchObject({
      type: 'article',
      publishedTime: '2026-09-01T00:00:00Z',
      modifiedTime: '2026-09-02T00:00:00Z',
      tags: ['agents', 'planning'],
    });
  });

  it('a blog post keeps its dates and tags, dated from its publication', async () => {
    serve(pages['blog post'].api);
    expect((await pages['blog post'].load()).openGraph).toMatchObject({
      type: 'article',
      publishedTime: '2026-09-03T00:00:00Z',
      modifiedTime: '2026-09-04T00:00:00Z',
      tags: ['agents'],
    });
  });

  // The API serves cover_image_url; a post that has one previews with it. Its size is
  // not known here, so none is claimed.
  it('a blog post with a cover image previews with that image, claiming no size for it', async () => {
    const cover = 'https://cdn.example.test/covers/a-post.jpg';
    serve({ '/v1/blog/a-post': { data: { ...BLOG_POST, cover_image_url: cover } } });
    const metadata = await pages['blog post'].load();
    expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
    const preview = readPreview(metadata);
    expect(preview.image).toBe(cover);
    expect(preview.twitterImage).toBe(cover);
    expect(preview.twitterCard).toBe('summary_large_image');
    expect(preview.images).toEqual([{ url: cover, alt: 'A listed blog post' }]);
  });

  // A room that may not be indexed is still a public page someone can paste: it keeps its preview.
  it('a room that is not indexable keeps its preview', async () => {
    serve({ ...pages.room.api, '/v1/rooms/kestrel/seo': { data: { ...ROOM_SEO.data, indexable: false } } });
    const metadata = await pages.room.load();
    expect(metadata.robots).toEqual(NOINDEX);
    expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
  });

  // No detail of a room the server may not read reaches its metadata. The gate names
  // itself, so it no longer shows the home page's title, and names no address.
  it('a private room previews as a private room: no address, nothing about the room', async () => {
    answer(403, null);
    const metadata = await pages.room.load();
    expect(metadata.robots).toEqual(NOINDEX);
    expect(metadata.alternates).toBeUndefined();
    expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
    expect(readPreview(metadata)).toMatchObject({ title: 'Private room | Solvr', url: undefined, image: CARD });
    expect(JSON.stringify(metadata)).not.toContain('kestrel');
  });

  // A room the API no longer has is a real 404: the not-found page names itself.
  it('a missing room states nothing of its own', async () => {
    answer(404, null);
    expect(await pages.room.load()).toEqual({ robots: NOINDEX });
  });
});
