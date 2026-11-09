import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// SEO audit 2026-11-08: a blog post linked no other blog post. The page reads the published
// posts sharing a tag with it, drops itself, and hands up to four to the content view. It
// also previews with its own generated card when it has no cover image.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('./blog-post-content', () => ({
  BlogPostContent: ({ related }: { related?: { slug: string }[] }) => (
    <div data-testid="related">{(related ?? []).map((p) => p.slug).join(',')}</div>
  ),
}));

import * as blogPostPage from './page';

const fetchMock = vi.fn();
const params = () => ({ params: Promise.resolve({ slug: 'a-post' }) });
const author = { id: 'helper_bot', type: 'agent', display_name: 'Helper Bot' };
const post = (slug: string, tags = ['agents']) => ({
  slug, title: `Post ${slug}`, body: 'Body', excerpt: 'Excerpt', meta_description: 'Description',
  created_at: '2026-09-01T00:00:00Z', published_at: '2026-09-03T00:00:00Z', updated_at: '2026-09-04T00:00:00Z',
  tags, cover_image_url: '', read_time_minutes: 1, vote_score: 1, view_count: 2, author,
});
function serve(tags: string[], related: unknown[]) {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    const body = u.includes('/v1/blog?') ? { data: related, meta: { total: related.length } } : { data: post('a-post', tags) };
    return { ok: true, status: 200, json: async () => body };
  });
}
const listCalls = () => fetchMock.mock.calls.map(([u]) => String(u)).filter((u) => u.includes('/v1/blog?'));

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('a blog post links the posts that share its tags', () => {
  it('reads the published posts sharing a tag, drops itself and keeps four at most', async () => {
    serve(['agents', 'esp32'], [post('a-post'), post('b'), post('c'), post('d'), post('e'), post('f')]);
    render(await blogPostPage.default(params()));

    const u = new URL(listCalls()[0]);
    expect(u.pathname).toBe('/v1/blog');
    expect(u.searchParams.get('tags')).toBe('agents,esp32');
    expect(u.searchParams.get('per_page')).toBe('5');
    expect(screen.getByTestId('related').textContent).toBe('b,c,d,e');
  });

  it('asks for none when the post has no tags', async () => {
    serve([], []);
    render(await blogPostPage.default(params()));
    expect(listCalls()).toHaveLength(0);
  });

  it('previews with its own generated card when it has no cover image', async () => {
    serve(['agents'], []);
    const metadata = await blogPostPage.generateMetadata(params());
    const images = (metadata.openGraph as { images?: { url: string }[] } | undefined)?.images ?? [];
    expect(images).toHaveLength(1);
    expect(images[0].url).toBe('https://solvr.dev/blog/a-post/og.png');
  });
});
