import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';

// SPEC.md 27.2: a user's profile lists, in the server HTML, the first fifty posts of that
// person a search engine may index, as plain links. The profile's Posts tab loads in the
// browser, so without this list the profile linked none of them.

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
vi.mock('@/components/users/user-profile-client', () => ({ UserProfileClient: () => <div data-testid="profile" /> }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, userJsonLd: () => ({}) }));

import UserPage from './page';

const USER = { data: { id: 'user-1', username: 'listed', display_name: 'Listed User', bio: 'Bio' } };
const author = { id: 'user-1', type: 'human', display_name: 'Listed User' };
const list = (count: number, total = count) => ({
  data: Array.from({ length: count }, (_, i) => ({ id: `post-${i + 1}`, title: `User post ${i + 1}`, created_at: '2026-09-01T00:00:00Z', author })),
  meta: { total, page: 1, per_page: 50, total_pages: Math.ceil(total / 50), has_more: total > count },
});

const fetchMock = vi.fn();
function serve(user: [number, unknown], posts: [number, unknown]) {
  fetchMock.mockImplementation(async (url: string) => {
    const [status, body] = String(url).includes('/v1/posts') ? posts : user;
    return { ok: status < 300, status, json: async () => body };
  });
}
const params = { params: Promise.resolve({ id: 'user-1' }) };
const listCalls = () => fetchMock.mock.calls.filter(([url]) => String(url).includes('/v1/posts'));

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("a user profile lists the person's posts in server HTML", () => {
  it("reads the person's indexable posts, newest first, fifty at most, on every request", async () => {
    serve([200, USER], [200, list(2)]);
    await UserPage(params);
    expect(listCalls()).toHaveLength(1);
    const [url, init] = listCalls()[0];
    expect(new URL(String(url)).pathname + new URL(String(url)).search).toBe(
      '/v1/posts?indexable=true&sort=new&page=1&per_page=50&author_type=human&author_id=user-1',
    );
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('renders each post as a plain link, after the profile', async () => {
    serve([200, USER], [200, list(3)]);
    const html = renderToStaticMarkup(await UserPage(params));
    for (const n of [1, 2, 3]) {
      expect(html).toMatch(new RegExp(`<a\\b[^>]*href="/posts/post-${n}"[^>]*>User post ${n}</a>`));
    }
    expect(html).toContain('Posts by Listed User');
    expect(html.indexOf('data-testid="profile"')).toBeLessThan(html.indexOf('href="/posts/post-1"'));
  });

  it('names the person by their username when they set no display name', async () => {
    serve([200, { data: { id: 'user-1', username: 'listed', display_name: '' } }], [200, list(1)]);
    expect(renderToStaticMarkup(await UserPage(params))).toContain('Posts by listed');
  });

  it('shows no list for a person with no indexable post', async () => {
    serve([200, USER], [200, list(0)]);
    const html = renderToStaticMarkup(await UserPage(params));
    expect(html).toContain('data-testid="profile"');
    expect(html).not.toContain('Posts by');
  });

  it('answers 404 for a user the API refuses, without reading a list', async () => {
    serve([404, {}], [200, list(3)]);
    await expect(UserPage(params)).rejects.toThrow('NEXT_NOT_FOUND');
    expect(listCalls()).toHaveLength(0);
  });

  it.each([500, 503, 429, 404])('fails retryably, never as a profile without its list, when the list read answers %i', async (status) => {
    serve([200, USER], [status, {}]);
    await expect(UserPage(params)).rejects.toThrow(new RegExp(`/v1/posts.*${status}`));
    expect(notFound).not.toHaveBeenCalled();
  });
});
