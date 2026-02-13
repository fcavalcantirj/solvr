import { describe, it, expect, vi, beforeEach } from 'vitest';
import sitemap, { generateSitemaps } from './sitemap';

// Mock the API client
vi.mock('@/lib/api', () => ({
  api: {
    getSitemapUrls: vi.fn(),
    getSitemapCounts: vi.fn(),
  },
}));

import { api } from '@/lib/api';

describe('generateSitemaps', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calculates correct number of sitemaps from counts', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 12000, agents: 3000, users: 8000 },
    });

    const result = await generateSitemaps();

    // 1 static + ceil(12000/2500)=5 posts + ceil(3000/2500)=2 agents + ceil(8000/2500)=4 users = 12
    expect(result).toHaveLength(12);
    expect(result).toEqual(
      Array.from({ length: 12 }, (_, i) => ({ id: i }))
    );
  });

  it('returns single sitemap when all counts are zero', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 0, agents: 0, users: 0 },
    });

    const result = await generateSitemaps();

    // 1 static + 0 + 0 + 0 = 1
    expect(result).toEqual([{ id: 0 }]);
  });

  it('falls back to [{ id: 0 }] on API error', async () => {
    vi.mocked(api.getSitemapCounts).mockRejectedValue(new Error('API down'));

    const result = await generateSitemaps();

    expect(result).toEqual([{ id: 0 }]);
  });
});

describe('sitemap', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sitemap({ id: 0 }) returns static pages with correct priorities', async () => {
    const result = await sitemap({ id: 0 });

    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/problems');
    expect(urls).toContain('https://solvr.dev/questions');
    expect(urls).toContain('https://solvr.dev/ideas');
    expect(urls).toContain('https://solvr.dev/agents');
    expect(urls).toContain('https://solvr.dev/users');
    expect(urls).toContain('https://solvr.dev/about');
    expect(urls).toContain('https://solvr.dev/api-docs');
    expect(urls).toContain('https://solvr.dev/mcp');
    expect(result).toHaveLength(11);

    const homepage = result.find((entry) => entry.url === 'https://solvr.dev/');
    expect(homepage?.priority).toBe(1.0);
  });

  it('sitemap({ id: 0 }) does not call any API', async () => {
    await sitemap({ id: 0 });

    expect(api.getSitemapCounts).not.toHaveBeenCalled();
    expect(api.getSitemapUrls).not.toHaveBeenCalled();
  });

  it('sitemap({ id: 1 }) fetches posts page 1', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 8000, agents: 100, users: 50 },
    });
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

    const result = await sitemap({ id: 1 });

    expect(api.getSitemapUrls).toHaveBeenCalledWith({ type: 'posts', page: 1, per_page: 2500 });
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/problems/p1');
    expect(urls).toContain('https://solvr.dev/questions/q1');
    expect(urls).toContain('https://solvr.dev/ideas/i1');
  });

  it('sitemap({ id: 2 }) fetches posts page 2 when posts span 2 pages', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 8000, agents: 100, users: 50 },
    });
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [{ id: 'p2', type: 'problem', updated_at: '2026-02-01T00:00:00Z' }], agents: [], users: [] },
    });

    const result = await sitemap({ id: 2 });

    // posts need ceil(8000/2500)=4 pages, so id=2 is posts page 2
    expect(api.getSitemapUrls).toHaveBeenCalledWith({ type: 'posts', page: 2, per_page: 2500 });
    expect(result).toHaveLength(1);
  });

  it('sitemap fetches agents for correct id range', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 8000, agents: 6000, users: 50 },
    });
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [{ id: 'a1', updated_at: '2026-02-01T00:00:00Z' }], users: [] },
    });

    // posts: ceil(8000/2500)=4, so agents start at id=5
    const result = await sitemap({ id: 5 });

    expect(api.getSitemapUrls).toHaveBeenCalledWith({ type: 'agents', page: 1, per_page: 2500 });
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/agents/a1');
    expect(result[0]?.priority).toBe(0.6);
  });

  it('sitemap fetches users for correct id range', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 8000, agents: 100, users: 6000 },
    });
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [], users: [{ id: 'u1', updated_at: '2026-02-01T00:00:00Z' }] },
    });

    // posts: ceil(8000/2500)=4, agents: ceil(100/2500)=1, so users start at id=6
    const result = await sitemap({ id: 6 });

    expect(api.getSitemapUrls).toHaveBeenCalledWith({ type: 'users', page: 1, per_page: 2500 });
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/users/u1');
    expect(result[0]?.priority).toBe(0.5);
  });

  it('returns empty array for ID beyond total sitemaps', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 100, agents: 50, users: 30 },
    });

    // posts: ceil(100/2500)=1, agents: ceil(50/2500)=1, users: ceil(30/2500)=1
    // total sitemaps: 1 static + 1 + 1 + 1 = 4 (ids 0-3)
    // id=4 is out of range
    const result = await sitemap({ id: 4 });

    expect(result).toEqual([]);
  });

  it('skips zero-count types in ID offset calculation', async () => {
    // posts=0 means no post sitemaps, agents should start at id=1
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 0, agents: 2500, users: 0 },
    });
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: { posts: [], agents: [{ id: 'a1', updated_at: '2026-02-01T00:00:00Z' }], users: [] },
    });

    // posts: ceil(0/2500)=0 pages, so agents start at id=1 (not id=2)
    const result = await sitemap({ id: 1 });

    expect(api.getSitemapUrls).toHaveBeenCalledWith({ type: 'agents', page: 1, per_page: 2500 });
    const urls = result.map((entry) => entry.url);
    expect(urls).toContain('https://solvr.dev/agents/a1');
  });

  it('returns empty array on API error for dynamic sitemaps', async () => {
    vi.mocked(api.getSitemapCounts).mockRejectedValue(new Error('API unavailable'));

    const result = await sitemap({ id: 1 });

    expect(result).toEqual([]);
  });

  it('sets lastModified from API updated_at', async () => {
    vi.mocked(api.getSitemapCounts).mockResolvedValue({
      data: { posts: 100, agents: 0, users: 0 },
    });
    vi.mocked(api.getSitemapUrls).mockResolvedValue({
      data: {
        posts: [{ id: 'p1', type: 'problem', updated_at: '2026-02-01T12:00:00Z' }],
        agents: [],
        users: [],
      },
    });

    const result = await sitemap({ id: 1 });

    const entry = result.find((e) => e.url === 'https://solvr.dev/problems/p1');
    expect(entry?.lastModified).toBe('2026-02-01T12:00:00Z');
  });
});
