import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { readPreview } from '@/lib/seo/preview-check';

// SPEC.md 27.1 and 27.3 (recon finding F08): a blog post's description is the API's served
// meta_description: the author's, or one the API composed from the body (Markdown removed, cut
// at a word, at most 160 characters). The page states it as its description, its preview and
// its BlogPosting's description, and derives none from the excerpt or the raw body. The
// BlogPosting names the author as the API does.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('next/navigation', () => ({ notFound: () => { throw new Error('NEXT_NOT_FOUND'); } }));
vi.mock('./blog-post-content', () => ({ BlogPostContent: () => null }));

import * as blogPostPage from './page';

const fetchMock = vi.fn();
const params = () => ({ params: Promise.resolve({ slug: 'a-post' }) });
const POST = {
  slug: 'a-post',
  title: 'A listed blog post',
  body: '## Raw **markdown** body with a [link](https://x.test)',
  excerpt: 'Raw **excerpt**',
  meta_description: 'Raw markdown body with a link',
  created_at: '2026-09-01T00:00:00Z',
  published_at: '2026-09-03T00:00:00Z',
  updated_at: '2026-09-04T00:00:00Z',
  tags: ['agents'],
  cover_image_url: '',
  read_time_minutes: 1,
  vote_score: 1,
  view_count: 2,
  author: { id: 'helper_bot', type: 'agent', display_name: 'Helper Bot' },
};
const serve = (post: unknown) =>
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: post }) });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('blog post page description', () => {
  it("states the API's meta_description as its description and preview", async () => {
    serve(POST);
    const metadata = await blogPostPage.generateMetadata(params());
    expect(metadata.description).toBe('Raw markdown body with a link');
    expect(readPreview(metadata).description).toBe('Raw markdown body with a link');
  });

  it("gives its BlogPosting the same description and the author the API names", async () => {
    serve(POST);
    const html = renderToStaticMarkup(await blogPostPage.default(params()));
    const ld = JSON.parse(html.match(/<script type="application\/ld\+json">(.*?)<\/script>/)?.[1] ?? '{}');
    expect(ld['@type']).toBe('BlogPosting');
    expect(ld.description).toBe('Raw markdown body with a link');
    expect(ld.author).toEqual({ '@type': 'Thing', name: 'Helper Bot', url: 'https://solvr.dev/agents/helper_bot', identifier: 'helper_bot' });
    expect(html).not.toContain('**');
  });

  it("names a person who wrote the post as a Person with their profile", async () => {
    serve({ ...POST, author: { id: 'u-1', type: 'human', display_name: 'Ana Lima' } });
    const html = renderToStaticMarkup(await blogPostPage.default(params()));
    const ld = JSON.parse(html.match(/<script type="application\/ld\+json">(.*?)<\/script>/)?.[1] ?? '{}');
    expect(ld.author).toEqual({ '@type': 'Person', name: 'Ana Lima', url: 'https://solvr.dev/users/u-1' });
  });
});
