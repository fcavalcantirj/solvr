import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { NOINDEX } from '@/lib/seo/route-policy';
import { readPreview } from '@/lib/seo/preview-check';

// SPEC.md 27.1: an agent's or a person's profile page renders the API's verdict
// (GET /v1/agents/{id}/seo, GET /v1/users/{id}/seo) as robots, title, description and link
// preview. A profile without public content is `noindex, follow` and still lists the author's
// indexable posts, so a crawler follows them. A failed verdict read is a retryable 500, never
// a false noindex (27.4).

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));
vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/agents/agent-profile-client', () => ({ AgentProfileClient: () => <div data-testid="profile" /> }));
vi.mock('@/components/users/user-profile-client', () => ({ UserProfileClient: () => <div data-testid="profile" /> }));

import * as agentPage from './agents/[id]/page';
import * as userPage from './users/[id]/page';

const fetchMock = vi.fn();
type Answer = [number, unknown] | 'unreachable';
// serve answers the profile read, its verdict and the posts list apart.
function serve(profile: Answer, verdict: Answer, posts: Answer = [200, { data: [], meta: { total: 0 } }]) {
  fetchMock.mockImplementation(async (url: string) => {
    const path = new URL(String(url)).pathname;
    const answer = path.endsWith('/seo') ? verdict : path === '/v1/posts' ? posts : profile;
    if (answer === 'unreachable') throw new TypeError('fetch failed');
    const [status, body] = answer;
    return { ok: status < 300, status, json: async () => body };
  });
}

const AGENT = { data: { agent: { id: 'agent_one', display_name: 'Listed Agent', bio: 'raw **bio**' }, stats: {} } };
const USER = { data: { id: 'u1', username: 'listed', display_name: 'Listed User', bio: 'raw **bio**' } };
const verdict = (indexable: boolean, title: string, description: string) => [200, { data: { indexable, title, description } }] as Answer;
const POSTS = [200, {
  data: [{ id: 'p1', title: 'A listed post', created_at: '2026-09-01T00:00:00Z', author: { id: 'agent_one', type: 'agent', display_name: 'Listed Agent' } }],
  meta: { total: 1, page: 1, per_page: 50, total_pages: 1, has_more: false },
}] as Answer;

const pages = {
  agent: {
    mod: agentPage,
    params: () => ({ params: Promise.resolve({ id: 'agent_one' }) }),
    profile: [200, AGENT] as Answer,
    seoPath: '/v1/agents/agent_one/seo',
    title: 'Listed Agent (AI agent)',
    description: 'Listed Agent, an AI agent on Solvr: 1 post. Plans builds.',
    canonical: '/agents/agent_one',
  },
  user: {
    mod: userPage,
    params: () => ({ params: Promise.resolve({ id: 'u1' }) }),
    profile: [200, USER] as Answer,
    seoPath: '/v1/users/u1/seo',
    title: 'Listed User (@listed)',
    description: 'Listed User on Solvr: 1 post. Reviews plans.',
    canonical: '/users/u1',
  },
};

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe.each(Object.entries(pages))('the %s profile page', (_kind, page) => {
  it("renders the API's title and description, indexable when the API says so", async () => {
    serve(page.profile, verdict(true, page.title, page.description));
    const metadata = await page.mod.generateMetadata(page.params());
    expect(metadata.robots).toBeUndefined();
    expect(metadata.title).toBe(page.title);
    expect(metadata.description).toBe(page.description);
    expect(metadata.alternates?.canonical).toBe(page.canonical);
    expect(readPreview(metadata)).toMatchObject({ title: `${page.title} | Solvr`, description: page.description, type: 'profile' });
    const seoCall = fetchMock.mock.calls.find(([u]) => new URL(String(u)).pathname === page.seoPath);
    expect(seoCall?.[1]).toMatchObject({ cache: 'no-store' });
  });

  it('is noindex, follow when the API says the profile holds nothing', async () => {
    serve(page.profile, verdict(false, page.title, page.description));
    const metadata = await page.mod.generateMetadata(page.params());
    expect(metadata.robots).toEqual(NOINDEX);
    expect(NOINDEX).toEqual({ index: false, follow: true });
    // A profile that may not be indexed is still a page someone can paste: it keeps its preview.
    expect(readPreview(metadata)).toMatchObject({ title: `${page.title} | Solvr`, description: page.description });
  });

  // The verdict and the profile are read apart: a profile gone between the two reads is noindex.
  it('is noindex when the API refuses the verdict', async () => {
    serve(page.profile, [404, { error: { code: 'NOT_FOUND' } }]);
    expect((await page.mod.generateMetadata(page.params())).robots).toEqual(NOINDEX);
  });

  it.each([500, 502, 503, 429])('fails retryably, never as noindex, when the verdict read answers %i', async (status) => {
    serve(page.profile, [status, null]);
    await expect(page.mod.generateMetadata(page.params())).rejects.toThrow(new RegExp(`${page.seoPath}.*${status}`));
    await expect(page.mod.default(page.params())).rejects.toThrow(new RegExp(`${status}`));
    expect(notFound).not.toHaveBeenCalled();
  });

  it('fails retryably, never as noindex, when the API cannot be reached for the verdict', async () => {
    serve(page.profile, 'unreachable');
    await expect(page.mod.generateMetadata(page.params())).rejects.toThrow(/unreachable/);
  });

  // A noindex profile still links its author's indexable posts: "noindex, follow".
  it("lists the author's posts in its HTML when the profile is noindex", async () => {
    serve(page.profile, verdict(false, page.title, page.description), POSTS);
    const html = renderToStaticMarkup(await page.mod.default(page.params()));
    expect(html).toContain('href="/posts/p1"');
  });

  it("gives its structured data the API's description, and a ProfilePage", async () => {
    serve(page.profile, verdict(true, page.title, page.description));
    const html = renderToStaticMarkup(await page.mod.default(page.params()));
    const ld = JSON.parse(html.match(/<script type="application\/ld\+json">(.*?)<\/script>/)?.[1] ?? '{}');
    expect(ld['@type']).toBe('ProfilePage');
    expect(ld.mainEntity.description).toBe(page.description);
    expect(JSON.stringify(ld)).not.toContain('raw');
    expect(JSON.stringify(ld)).not.toContain('SoftwareApplication');
  });
});
