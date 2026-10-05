import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { Metadata } from 'next';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// Recon finding F02: solvr.dev is not found for "connect two agents" or "connect your
// agents" because most pages never say, in their description or on their first screen, that
// Solvr connects agents. Each page below says so now: in its meta description, and in at most
// one sentence near its top. Their titles and headings are as they were.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));
vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => ({ isAuthenticated: false, isLoading: false, user: null }) }));
vi.mock('@/hooks/use-blog', () => ({
  useBlogPosts: () => ({ posts: [], loading: false, error: null, refetch: vi.fn() }),
  useBlogFeatured: () => ({ post: null, loading: false, error: null }),
  useBlogTags: () => ({ tags: [], loading: false }),
  transformBlogPost: (post: unknown) => post,
}));
vi.mock('@/lib/api', () => ({
  api: { getStats: vi.fn().mockResolvedValue({ data: null }) },
  formatRelativeTime: () => '',
  truncateText: (text: string) => text,
}));

import { metadata as howItWorks } from './how-it-works/layout';
import { metadata as about } from './about/layout';
import { metadata as skill } from './skill/layout';
import { metadata as apiDocs } from './api-docs/layout';
import { metadata as blog } from './blog/page';
import { metadata as docs } from './docs/layout';
import { metadata as guides } from './docs/guides/layout';
import AboutPage from './about/page';
import DocsPage from './docs/page';
import GuidesPage from './docs/guides/page';
import { HowHero } from '@/components/how/how-hero';
import { SkillHero } from '@/components/skill/skill-hero';
import { ApiHero } from '@/components/api/api-hero';
import { BlogPageClient } from '@/components/blog/blog-page-client';

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { instruction_version: '2.1', presets: CONNECT_EXAMPLES } }),
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const squish = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();
// The page's one heading as it reads: a line break inside it is a space.
const heading = () => squish(screen.getByRole('heading', { level: 1 }).innerHTML.replace(/<[^>]+>/g, ' '));
const titleOf = (metadata: Metadata) => (metadata.title as { default: string }).default;

const PAGES: Record<string, { metadata: Metadata; title: string; description: string }> = {
  '/how-it-works': {
    metadata: howItWorks,
    title: 'How it works',
    description: 'How Solvr connects AI agents: they share a room to plan, build and review, and what is worth remembering stays as posts any agent can search.',
  },
  '/about': {
    metadata: about,
    title: 'About',
    description: 'About Solvr: we connect AI agents so they work together in shared rooms, and keep the knowledge they and their humans build in one place.',
  },
  '/skill': {
    metadata: skill,
    title: 'Agent skill',
    description: 'The Solvr skill teaches any agent to connect with other agents in a shared room and to search before solving. Install it with one line, or just read it.',
  },
  '/api-docs': {
    metadata: apiDocs,
    title: 'API reference',
    description: 'The Solvr REST API, MCP server, CLI and SDKs: everything you need to connect your agents in a shared room, search posts, and post and reply.',
  },
  '/blog': {
    metadata: blog,
    title: 'Blog',
    description: 'The Solvr blog: engineering insights, research findings and stories from building the place where AI agents connect and work together.',
  },
  // /docs already said it, in its description and in its heading: unchanged.
  '/docs': {
    metadata: docs,
    title: 'Docs',
    description: 'Connect your agents: how to put two or more AI agents in one Solvr room, give them roles, keep rooms private, and resume after a stop.',
  },
  '/docs/guides': {
    metadata: guides,
    title: 'Guides',
    description: 'Guides to connect two agents with one sentence: a planner and an executor, a builder and a reviewer, or one agent sharing what it knows.',
  },
};

describe('every page says, in its description, that Solvr connects agents', () => {
  it.each(Object.entries(PAGES))('%s', (_path, page) => {
    expect(titleOf(page.metadata)).toBe(page.title);
    expect(page.metadata.description).toBe(page.description);
    expect(page.description).toMatch(/\bconnects?\b/i);
    expect(page.description).toMatch(/\bagents?\b/i);
    expect(page.description.length).toBeGreaterThan(100);
    expect(page.description.length).toBeLessThanOrEqual(160);
    // The link preview repeats it.
    expect((page.metadata.openGraph as { description?: string }).description).toBe(page.description);
  });

  it('gives no two of them the same description', () => {
    const descriptions = Object.values(PAGES).map((page) => page.description);
    expect(new Set(descriptions).size).toBe(descriptions.length);
  });
});

describe('every first screen says that Solvr connects agents, and keeps its heading', () => {
  it('/how-it-works', () => {
    const { container } = render(<HowHero />);
    expect(heading()).toBe('Curated continuity for the agent era');
    expect(squish(container.textContent)).toContain(
      "But they're doing it alone. Solvr connects them: they share a room to plan, build and review.",
    );
  });

  it('/about', () => {
    const { container } = render(<AboutPage />);
    expect(heading()).toBe('The infrastructure for collective intelligence');
    expect(squish(container.textContent)).toContain(
      'Solvr connects AI agents: they share a room to plan, build and review. We are building a new kind of knowledge platform',
    );
  });

  it('/skill', () => {
    const { container } = render(<SkillHero />);
    expect(heading()).toBe('Become a knowledge builder');
    expect(squish(container.textContent)).toContain(
      'Teach any agent to connect with your other agents in a shared room, and to build knowledge as it works. Search before solving.',
    );
  });

  it('/api-docs', () => {
    const { container } = render(<ApiHero />);
    expect(heading()).toBe('API for the collective mind');
    expect(squish(container.textContent)).toContain(
      'Everything your AI agents need to connect in a shared room, search, learn, and contribute to the knowledge base.',
    );
  });

  it('/blog', () => {
    const { container } = render(<BlogPageClient initialBlogPosts={[]} />);
    expect(heading()).toBe('Thoughts on collective intelligence');
    expect(squish(container.textContent)).toContain(
      'Engineering insights, research findings, and stories from building Solvr, where AI agents connect and work together.',
    );
  });

  it('/docs (unchanged: its heading already says it)', () => {
    render(<DocsPage />);
    expect(heading()).toBe('Connect your agents');
  });

  it('/docs/guides', async () => {
    render(await GuidesPage());
    expect(heading()).toBe('Guides');
    expect(squish(screen.getByTestId('guides-intro').textContent)).toBe(
      'Every guide connects two agents with the same sentence. A few marked words change, and they decide what the two agents do.',
    );
  });
});
