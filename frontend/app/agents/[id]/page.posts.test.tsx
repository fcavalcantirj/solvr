import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';

// SPEC.md 27.2: an agent's profile lists, in the server HTML, the first fifty posts of that
// agent a search engine may index, as plain links. The profile's activity loads in the
// browser, so without this list the profile linked none of its agent's posts.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => {
    // What a link asks of the router (prefetch) is not an attribute of the element.
    const attributes = { ...props };
    delete attributes.prefetch;
    return <a href={href} {...attributes}>{children}</a>;
  },
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/agents/agent-profile-client', () => ({ AgentProfileClient: () => <div data-testid="profile" /> }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, agentJsonLd: () => ({}) }));

import AgentPage from './page';

const AGENT = { data: { agent: { id: 'agent_one', display_name: 'Listed Agent', bio: 'Bio' }, stats: {} } };
const author = { id: 'agent_one', type: 'agent', display_name: 'Listed Agent' };
const list = (count: number, total = count) => ({
  data: Array.from({ length: count }, (_, i) => ({ id: `post-${i + 1}`, title: `Agent post ${i + 1}`, created_at: '2026-09-01T00:00:00Z', author })),
  meta: { total, page: 1, per_page: 50, total_pages: Math.ceil(total / 50), has_more: total > count },
});

const fetchMock = vi.fn();
// serve answers the profile read and the list read apart.
function serve(agent: [number, unknown], posts: [number, unknown]) {
  fetchMock.mockImplementation(async (url: string) => {
    const [status, body] = String(url).includes('/v1/posts') ? posts : agent;
    return { ok: status < 300, status, json: async () => body };
  });
}
const params = { params: Promise.resolve({ id: 'agent_one' }) };
const listCalls = () => fetchMock.mock.calls.filter(([url]) => String(url).includes('/v1/posts'));

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("an agent profile lists the agent's posts in server HTML", () => {
  it("reads the agent's indexable posts, newest first, fifty at most, on every request", async () => {
    serve([200, AGENT], [200, list(2)]);
    await AgentPage(params);
    expect(listCalls()).toHaveLength(1);
    const [url, init] = listCalls()[0];
    expect(new URL(String(url)).pathname + new URL(String(url)).search).toBe(
      '/v1/posts?indexable=true&sort=new&page=1&per_page=50&author_type=agent&author_id=agent_one',
    );
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('renders each post as a plain link, after the profile', async () => {
    serve([200, AGENT], [200, list(3)]);
    const html = renderToStaticMarkup(await AgentPage(params));
    for (const n of [1, 2, 3]) {
      expect(html).toMatch(new RegExp(`<a\\b[^>]*href="/posts/post-${n}"[^>]*>Agent post ${n}</a>`));
    }
    expect(html).toContain('Posts by Listed Agent');
    expect(html.indexOf('data-testid="profile"')).toBeLessThan(html.indexOf('href="/posts/post-1"'));
  });

  it('lists fifty and says there are more', async () => {
    serve([200, AGENT], [200, list(50, 123)]);
    const html = renderToStaticMarkup(await AgentPage(params));
    expect(html.match(/href="\/posts\/post-\d+"/g)).toHaveLength(50);
    expect(html).toContain('The newest 50 of 123 posts.');
  });

  it('shows no list for an agent with no indexable post', async () => {
    serve([200, AGENT], [200, list(0)]);
    const html = renderToStaticMarkup(await AgentPage(params));
    expect(html).toContain('data-testid="profile"');
    expect(html).not.toContain('Posts by');
    expect(html).not.toContain('href="/posts/');
  });

  it('answers 404 for an agent the API refuses, without reading a list', async () => {
    serve([404, {}], [200, list(3)]);
    await expect(AgentPage(params)).rejects.toThrow('NEXT_NOT_FOUND');
    expect(listCalls()).toHaveLength(0);
  });

  // A profile rendered without its list would drop every link it carries, without a sign.
  it.each([500, 503, 429, 404])('fails retryably, never as a profile without its list, when the list read answers %i', async (status) => {
    serve([200, AGENT], [status, {}]);
    await expect(AgentPage(params)).rejects.toThrow(new RegExp(`/v1/posts.*${status}`));
    expect(notFound).not.toHaveBeenCalled();
  });
});
