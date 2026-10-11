import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { tagTitle, readPreview } from '@/lib/seo/preview-check';

// SPEC.md 27.3: a room, post or blog post page's <title> is the API's search title as
// written: at most 60 characters that lead with the problem the page resolves, with no
// " | Solvr" after it. Its <meta name="description"> is the API's description, at most 160
// characters of the page's visible text. The page derives neither.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('next/navigation', () => ({ notFound: () => { throw new Error('NEXT_NOT_FOUND'); } }));

import * as roomPage from './rooms/[slug]/page';
import * as postPage from './posts/[id]/page';
import * as blogPage from './blog/[slug]/page';

const ROOM_TITLE = 'fix-openclaw-oauth-cli: Helping other agents to configure…';
const ROOM_DESCRIPTION = 'Helping other agents to configure new models auth login --provider anthropic --method cli --set-default command';
const POST_TITLE = "How We Bypassed Anthropic's Third-Party OAuth Block";
const POST_DESCRIPTION =
  'Third-party tools started getting rejected with HTTP 401. Solution: openclaw-billing-proxy. A proxy that makes requests look like Claude Code.';
const BLOG_TITLE = 'What AI provider APIs actually expose for usage monitoring';
const BLOG_DESCRIPTION = 'If you want to build a dashboard showing how much of your AI subscriptions and credits you have left, the first question is what each provider exposes.';

const fetchMock = vi.fn();
const ok = (body: unknown) => ({ ok: true, status: 200, json: async () => body });

function api() {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    if (u.endsWith('/rooms/fix-openclaw-oauth-cli/seo')) {
      return ok({ data: { indexable: true, title: ROOM_TITLE, description: ROOM_DESCRIPTION } });
    }
    if (u.endsWith('/rooms/fix-openclaw-oauth-cli')) {
      return ok({ data: { room: { id: 'r1', slug: 'fix-openclaw-oauth-cli', display_name: 'fix-openclaw-oauth-cli' } } });
    }
    if (u.endsWith('/posts/p1/seo')) {
      return ok({ data: { indexable: true, title: POST_TITLE, description: POST_DESCRIPTION } });
    }
    if (u.endsWith('/posts/p1')) {
      return ok({
        data: {
          id: 'p1', title: `${POST_TITLE}: A Billing Proxy Guide for OpenClaw (Humans & Agents)`, description: 'body',
          created_at: '2026-04-07T00:00:00Z', updated_at: '2026-04-08T00:00:00Z', tags: [],
          author: { id: 'a-1', type: 'agent', display_name: 'msv' },
        },
      });
    }
    if (u.includes('/blog/usage-apis')) {
      return ok({
        data: {
          slug: 'usage-apis', title: `${BLOG_TITLE} (Claude, ChatGPT/Codex, OpenRouter, Groq)`, meta_title: BLOG_TITLE,
          meta_description: BLOG_DESCRIPTION, body: 'body', created_at: '2026-09-01T00:00:00Z',
          updated_at: '2026-09-02T00:00:00Z', tags: [], author: { id: 'a-1', type: 'agent', display_name: 'msv' },
        },
      });
    }
    return ok({ data: [] });
  });
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  api();
});
afterEach(() => vi.unstubAllGlobals());

describe.each([
  ['room', () => roomPage.generateMetadata({ params: Promise.resolve({ slug: 'fix-openclaw-oauth-cli' }) }), ROOM_TITLE, ROOM_DESCRIPTION],
  ['post', () => postPage.generateMetadata({ params: Promise.resolve({ id: 'p1' }) }), POST_TITLE, POST_DESCRIPTION],
  ['blog post', () => blogPage.generateMetadata({ params: Promise.resolve({ slug: 'usage-apis' }) }), BLOG_TITLE, BLOG_DESCRIPTION],
])('a %s page in a search result', (_kind, metadata, title, description) => {
  it("shows the API's search title as its whole <title>, with no site name after it", async () => {
    const meta = await metadata();
    expect(tagTitle(meta.title)).toBe(title);
    expect(tagTitle(meta.title).length).toBeLessThanOrEqual(60);
  });

  it("shows the API's description as its <meta name=description>", async () => {
    const meta = await metadata();
    expect(meta.description).toBe(description);
    expect((meta.description ?? '').length).toBeLessThanOrEqual(160);
  });

  it('previews under the same title and description', async () => {
    const preview = readPreview(await metadata());
    expect(preview.title).toBe(title);
    expect(preview.description).toBe(description);
  });
});
