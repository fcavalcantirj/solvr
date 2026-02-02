/**
 * Tests for sitemap.xml generation per SPEC.md Part 19.2
 * Tests that sitemap includes all required URLs and follows sitemap protocol
 */

// Mock fetch for API calls
const mockPosts = [
  {
    id: 'post-1',
    type: 'question',
    title: 'How to handle async errors?',
    updated_at: '2026-01-15T10:00:00Z',
  },
  {
    id: 'post-2',
    type: 'problem',
    title: 'Race condition bug',
    updated_at: '2026-01-16T14:30:00Z',
  },
  {
    id: 'post-3',
    type: 'idea',
    title: 'New feature suggestion',
    updated_at: '2026-01-17T09:00:00Z',
  },
];

describe('Sitemap Generation', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
    process.env.NEXT_PUBLIC_APP_URL = 'https://solvr.dev';
    process.env.NEXT_PUBLIC_API_URL = 'https://api.solvr.dev';
    global.fetch = jest.fn();
  });

  afterEach(() => {
    process.env = originalEnv;
    jest.resetAllMocks();
  });

  it('generates sitemap with static pages', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    // Check static pages are included per SPEC.md Part 19.2
    const urls = result.map((item) => item.url);

    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls).toContain('https://solvr.dev/search');
  });

  it('includes priority values per SPEC.md Part 19.2', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    // Homepage should have highest priority (1.0)
    const homepage = result.find((item) => item.url === 'https://solvr.dev/');
    expect(homepage?.priority).toBe(1.0);

    // Feed should have high priority (0.9)
    const feed = result.find((item) => item.url === 'https://solvr.dev/feed');
    expect(feed?.priority).toBe(0.9);
  });

  it('includes changefreq values', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    // Homepage should update daily
    const homepage = result.find((item) => item.url === 'https://solvr.dev/');
    expect(homepage?.changeFrequency).toBe('daily');

    // Feed should update hourly
    const feed = result.find((item) => item.url === 'https://solvr.dev/feed');
    expect(feed?.changeFrequency).toBe('hourly');
  });

  it('fetches and includes post URLs', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: mockPosts, meta: { total: 3 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    // Check that posts are included
    const urls = result.map((item) => item.url);
    expect(urls).toContain('https://solvr.dev/posts/post-1');
    expect(urls).toContain('https://solvr.dev/posts/post-2');
    expect(urls).toContain('https://solvr.dev/posts/post-3');
  });

  it('sets lastModified for posts based on updated_at', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: mockPosts, meta: { total: 3 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    const post1 = result.find((item) => item.url === 'https://solvr.dev/posts/post-1');
    expect(post1?.lastModified).toBe('2026-01-15T10:00:00Z');
  });

  it('sets lower priority for posts (0.7)', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: mockPosts, meta: { total: 3 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    const post = result.find((item) => item.url === 'https://solvr.dev/posts/post-1');
    expect(post?.priority).toBe(0.7);
    expect(post?.changeFrequency).toBe('weekly');
  });

  it('handles API errors gracefully (returns static pages only)', async () => {
    (global.fetch as jest.Mock).mockRejectedValueOnce(new Error('API unavailable'));

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    // Should still return static pages
    const urls = result.map((item) => item.url);
    expect(urls).toContain('https://solvr.dev/');
    expect(urls).toContain('https://solvr.dev/feed');
    expect(urls.length).toBeGreaterThan(0);
  });

  it('excludes admin pages from sitemap per SPEC.md robots.txt requirements', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    const urls = result.map((item) => item.url);
    const adminUrls = urls.filter((url) => url.includes('/admin'));
    expect(adminUrls.length).toBe(0);
  });

  it('excludes auth pages from sitemap', async () => {
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0 } }),
    });

    const sitemap = await import('../app/sitemap');
    const result = await sitemap.default();

    const urls = result.map((item) => item.url);
    const authUrls = urls.filter((url) => url.includes('/auth') || url.includes('/login'));
    expect(authUrls.length).toBe(0);
  });
});
