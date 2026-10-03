import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';

// Task idx 82: the post page's title is the API's unique title, its JSON-LD describes
// the visible post truthfully (a Person only for a human), its breadcrumbs use the
// canonical URLs, and an outcome post links back to the room it came from.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/post-detail', () => ({
  PostDetail: ({ initial }: { initial?: { sourceRoom?: { slug: string } | null } }) => (
    <div data-testid="detail">{initial?.sourceRoom?.slug ?? 'no-source'}</div>
  ),
}));

import PostPage, { generateMetadata } from './page';

const fetchMock = vi.fn();
const author = (type: 'human' | 'agent') => ({ id: type === 'human' ? 'u-9' : 'a-1', type, display_name: 'Dev Nine' });
function api(type: 'human' | 'agent') {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    const body = u.endsWith('/seo')
      ? { data: { indexable: true, title: 'Hand a plan over — Dev Nine', description: 'Hand a plan over to an executor.' } }
      : u.includes('/replies')
        ? { data: [], meta: { total: 0, page: 1, total_pages: 0 } }
        : u.endsWith('/rooms')
          ? { data: [], source_room: { slug: 'kestrel-room', display_name: 'Kestrel Room' } }
          : {
              data: {
                id: 'p1', title: 'Hand a plan over', description: 'Hand a plan over to an executor.',
                created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-02T00:00:00Z', reply_count: 0,
                author: author(type), tags: [],
              },
            };
    return { ok: true, status: 200, json: async () => body };
  });
}
const params = { params: Promise.resolve({ id: 'p1' }) };

function jsonLd(container: HTMLElement) {
  return [...container.querySelectorAll('script[type="application/ld+json"]')].map((s) => JSON.parse(s.innerHTML));
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('post page structured data', () => {
  it('uses the API unique title', async () => {
    api('agent');
    expect((await generateMetadata(params)).title).toBe('Hand a plan over — Dev Nine');
  });

  it('describes a human post as a DiscussionForumPosting with breadcrumbs', async () => {
    api('human');
    const { container } = render(await PostPage(params));
    const [post, crumbs] = jsonLd(container);
    expect(post['@type']).toBe('DiscussionForumPosting');
    expect(post.headline).toBe('Hand a plan over — Dev Nine');
    expect(post.url).toBe('https://solvr.dev/posts/p1');
    expect(post.author['@type']).toBe('Person');
    expect(crumbs['@type']).toBe('BreadcrumbList');
    expect(crumbs.itemListElement.map((i: { item: string }) => i.item)).toEqual([
      'https://solvr.dev/', 'https://solvr.dev/posts', 'https://solvr.dev/posts/p1',
    ]);
  });

  it('never labels an agent post author a Person', async () => {
    api('agent');
    const { container } = render(await PostPage(params));
    const [post] = jsonLd(container);
    expect(post['@type']).toBe('WebPage');
    expect(JSON.stringify(post)).not.toContain('Person');
  });

  it('hands the source room to the detail so the outcome links back', async () => {
    api('agent');
    const { getByTestId } = render(await PostPage(params));
    expect(getByTestId('detail').textContent).toBe('kestrel-room');
  });
});
