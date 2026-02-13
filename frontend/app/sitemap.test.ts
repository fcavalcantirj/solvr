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

  it('returns static pages combined with dynamic URLs from API', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [
          { id: 'p1', type: 'problem', updated_at: '2026-02-01T00:00:00Z' },
          { id: 'q1', type: 'question', updated_at: '2026-02-02T00:00:00Z' },
          { id: 'i1', type: 'idea', updated_at: '2026-02-03T00:00:00Z' },
        ],
        agents: [
          { id: 'a1', updated_at: '2026-02-01T00:00:00Z' },
        ],
        users: [
          { id: 'u1', updated_at: '2026-02-01T00:00:00Z' },
        ],
      },
    });

    const result = await sitemap();

    // 11 static + 3 posts + 1 agent + 1 user = 16
    expect(result).toHaveLength(16);

    const urls = result.map((entry) => entry.url);
    // Static pages
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/problems');
    // Dynamic pages
    expect(urls).toContain('https://solvr.dev/problems/p1');
    expect(urls).toContain('https://solvr.dev/questions/q1');
    expect(urls).toContain('https://solvr.dev/ideas/i1');
    expect(urls).toContain('https://solvr.dev/agents/a1');
    expect(urls).toContain('https://solvr.dev/users/u1');
  });

  it('maps post types to correct URL paths', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [
          { id: '1', type: 'problem', updated_at: '2026-02-01T00:00:00Z' },
          { id: '2', type: 'question', updated_at: '2026-02-01T00:00:00Z' },
          { id: '3', type: 'idea', updated_at: '2026-02-01T00:00:00Z' },
          { id: '4', type: 'unknown', updated_at: '2026-02-01T00:00:00Z' },
        ],
        agents: [],
        users: [],
      },
    });

    const result = await sitemap();
    const urls = result.map((entry) => entry.url);

    expect(urls).toContain('https://solvr.dev/problems/1');
    expect(urls).toContain('https://solvr.dev/questions/2');
    expect(urls).toContain('https://solvr.dev/ideas/3');
    expect(urls).toContain('https://solvr.dev/posts/4');
  });

  it('sets correct priorities for each type', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [{ id: 'p1', type: 'problem', updated_at: '2026-02-01T00:00:00Z' }],
        agents: [{ id: 'a1', updated_at: '2026-02-01T00:00:00Z' }],
        users: [{ id: 'u1', updated_at: '2026-02-01T00:00:00Z' }],
      },
    });

    const result = await sitemap();

    const homepage = result.find((e) => e.url === 'https://solvr.dev/');
    expect(homepage?.priority).toBe(1.0);

    const post = result.find((e) => e.url === 'https://solvr.dev/problems/p1');
    expect(post?.priority).toBe(0.7);

    const agent = result.find((e) => e.url === 'https://solvr.dev/agents/a1');
    expect(agent?.priority).toBe(0.6);

    const user = result.find((e) => e.url === 'https://solvr.dev/users/u1');
    expect(user?.priority).toBe(0.5);
  });

  it('sets lastModified from API updated_at', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [{ id: 'p1', type: 'problem', updated_at: '2026-02-01T12:00:00Z' }],
        agents: [],
        users: [],
      },
    });

    const result = await sitemap();

    const entry = result.find((e) => e.url === 'https://solvr.dev/problems/p1');
    expect(entry?.lastModified).toBe('2026-02-01T12:00:00Z');
  });

  it('calls getSitemapUrls with no params', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [], users: [] },
    });

    await sitemap();

    expect(api.getSitemapUrls).toHaveBeenCalledOnce();
    expect(api.getSitemapUrls).toHaveBeenCalledWith();
  });

  it('returns only static pages on API error', async () => {
    vi.mocked(api.getSitemapUrls).mockRejectedValue(new Error('API down'));

    const result = await sitemap();

    expect(result).toHaveLength(11);
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/problems');
    expect(urls).toContain('https://solvr.dev/questions');
    expect(urls).toContain('https://solvr.dev/ideas');
    expect(urls).toContain('https://solvr.dev/agents');
    expect(urls).toContain('https://solvr.dev/users');
    expect(urls).toContain('https://solvr.dev/about');
    expect(urls).toContain('https://solvr.dev/how-it-works');
    expect(urls).toContain('https://solvr.dev/api-docs');
    expect(urls).toContain('https://solvr.dev/mcp');
  });

  it('returns only static pages when API returns empty data', async () => {
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [], users: [] },
    });

    const result = await sitemap();

    expect(result).toHaveLength(11);
  });
});
