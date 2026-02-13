import { describe, it, expect, vi, beforeEach } from 'vitest';
import sitemap from './sitemap';

// Mock the API client
vi.mock('@/lib/api', () => ({
  api: {
    getSitemapUrls: vi.fn(),
  },
}));

import { api } from '@/lib/api';

describe('sitemap', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('includes static pages with correct priorities', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [], users: [] },
    });

    const result = await sitemap();

    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/problems');
    expect(urls).toContain('https://solvr.dev/questions');
    expect(urls).toContain('https://solvr.dev/ideas');
    expect(urls).toContain('https://solvr.dev/agents');

    // Check homepage priority
    const homepage = result.find((entry) => entry.url === 'https://solvr.dev/');
    expect(homepage?.priority).toBe(1.0);
  });

  it('generates URLs for dynamic posts by type', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [
          { id: 'p1', type: 'problem', updated_at: '2026-02-01T00:00:00Z' },
          { id: 'q1', type: 'question', updated_at: '2026-02-02T00:00:00Z' },
          { id: 'i1', type: 'idea', updated_at: '2026-02-03T00:00:00Z' },
        ],
        agents: [],
        users: [],
      },
    });

    const result = await sitemap();

    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/problems/p1');
    expect(urls).toContain('https://solvr.dev/questions/q1');
    expect(urls).toContain('https://solvr.dev/ideas/i1');
  });

  it('generates URLs for dynamic agents', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [],
        agents: [
          { id: 'agent-1', updated_at: '2026-02-01T00:00:00Z' },
        ],
        users: [],
      },
    });

    const result = await sitemap();

    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/agents/agent-1');
  });

  it('generates URLs for dynamic users', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [],
        agents: [],
        users: [
          { id: 'user-1', updated_at: '2026-02-01T00:00:00Z' },
        ],
      },
    });

    const result = await sitemap();

    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/users/user-1');
  });

  it('sets lastModified from API updated_at', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [
          { id: 'p1', type: 'problem', updated_at: '2026-02-01T12:00:00Z' },
        ],
        agents: [],
        users: [],
      },
    });

    const result = await sitemap();

    const problemEntry = result.find((entry) => entry.url === 'https://solvr.dev/problems/p1');
    expect(problemEntry?.lastModified).toBe('2026-02-01T12:00:00Z');
  });

  it('sets correct priorities for dynamic content', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [{ id: 'p1', type: 'problem', updated_at: '2026-02-01T00:00:00Z' }],
        agents: [{ id: 'a1', updated_at: '2026-02-01T00:00:00Z' }],
        users: [{ id: 'u1', updated_at: '2026-02-01T00:00:00Z' }],
      },
    });

    const result = await sitemap();

    const problem = result.find((entry) => entry.url === 'https://solvr.dev/problems/p1');
    expect(problem?.priority).toBe(0.7);

    const agent = result.find((entry) => entry.url === 'https://solvr.dev/agents/a1');
    expect(agent?.priority).toBe(0.6);

    const user = result.find((entry) => entry.url === 'https://solvr.dev/users/u1');
    expect(user?.priority).toBe(0.5);
  });

  it('returns static pages only when API fails (graceful fallback)', async () => {
    vi.mocked(api.getSitemapUrls).mockRejectedValue(new Error('API unavailable'));

    const result = await sitemap();

    // Should still have static pages
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/problems');

    // Should NOT have dynamic content
    const dynamicUrls = urls.filter((url) =>
      url.match(/\/(problems|questions|ideas|agents|users)\/[a-zA-Z0-9-]+$/)
    );
    expect(dynamicUrls).toHaveLength(0);
  });
});
