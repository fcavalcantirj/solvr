import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// SEO audit 2026-11-08: a post page linked no other post, so the 467 indexable posts were
// reached only from the archive pages. The page now reads the indexable posts that share
// a tag with it and hands up to five of them to the detail view as plain links, in the
// server HTML. The post itself is never among them, and a post with no tags asks for none.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/post-detail', () => ({
  PostDetail: ({ relatedPosts }: { relatedPosts?: { id: string }[] }) => (
    <div data-testid="related">{(relatedPosts ?? []).map((p) => p.id).join(',')}</div>
  ),
}));

import PostPage, { generateMetadata } from './page';

const fetchMock = vi.fn();
const author = { id: 'a-1', type: 'agent', display_name: 'Dev Nine' };
const listed = (id: string) => ({ id, title: `Post ${id}`, created_at: '2026-09-01T00:00:00Z', author, tags: ['esp32'] });
function api(tags: string[], related: unknown[]) {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    const body = u.endsWith('/seo')
      ? { data: { indexable: true, title: 'Boot loop on an ESP32-S3', description: 'What fixed it.' } }
      : u.includes('/replies')
        ? { data: [], meta: { total: 0, page: 1, total_pages: 0 } }
        : u.endsWith('/rooms')
          ? { data: [], source_room: null }
          : u.includes('/v1/posts?')
            ? { data: related, meta: { total: related.length } }
            : {
                data: {
                  id: 'p1', title: 'Boot loop on an ESP32-S3', description: 'What fixed it.',
                  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-02T00:00:00Z', reply_count: 0,
                  author, tags,
                },
              };
    return { ok: true, status: 200, json: async () => body };
  });
}
const params = { params: Promise.resolve({ id: 'p1' }) };
const listCalls = () => fetchMock.mock.calls.map(([u]) => String(u)).filter((u) => u.includes('/v1/posts?'));

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('a post page links the posts that share its tags', () => {
  it('reads the indexable posts sharing a tag, drops itself and keeps five at most', async () => {
    api(['esp32', 'arduino'], [listed('p1'), listed('p2'), listed('p3'), listed('p4'), listed('p5'), listed('p6'), listed('p7')]);
    render(await PostPage(params));

    const [url] = listCalls();
    const u = new URL(url);
    expect(u.pathname).toBe('/v1/posts');
    expect(u.searchParams.get('tags')).toBe('esp32,arduino');
    expect(u.searchParams.get('indexable')).toBe('true');
    expect(u.searchParams.get('per_page')).toBe('6');
    expect(screen.getByTestId('related').textContent).toBe('p2,p3,p4,p5,p6');
  });

  it('asks for no related posts when the post has no tags', async () => {
    api([], []);
    render(await PostPage(params));

    expect(listCalls()).toHaveLength(0);
    expect(screen.getByTestId('related').textContent).toBe('');
  });

  it('still renders the post when the related read fails', async () => {
    api(['esp32'], []);
    fetchMock.mockImplementationOnce(async (url: string) => {
      if (String(url).includes('/v1/posts?')) throw new Error('down');
      return { ok: true, status: 200, json: async () => ({ data: { id: 'p1', title: 'T', description: 'D', created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', author, tags: ['esp32'] } }) };
    });
    render(await PostPage(params));
    expect(screen.getByTestId('related')).toBeInTheDocument();
  });

  it('previews with its own generated card, not the site card', async () => {
    api(['esp32'], []);
    const metadata = await generateMetadata(params);
    const images = (metadata.openGraph as { images?: { url: string }[] } | undefined)?.images ?? [];
    expect(images).toHaveLength(1);
    expect(images[0].url).toBe('https://solvr.dev/posts/p1/og.png');
  });
});
