/**
 * Tests for SEO metadata per SPEC.md Part 19.2
 * Tests dynamic page titles, meta descriptions, Open Graph, and Twitter cards
 */

// Mock next/font/google since it requires Next.js context
jest.mock('next/font/google', () => ({
  Inter: () => ({
    variable: '--font-inter',
  }),
  JetBrains_Mono: () => ({
    variable: '--font-jetbrains-mono',
  }),
}));

// Mock fetch for API calls in generateMetadata
const mockPost = {
  id: 'test-post-123',
  type: 'question',
  title: 'How to handle async errors in Go?',
  description: 'I am trying to handle errors in async operations in Go. What is the best practice for error handling in goroutines? Here is my code example...',
  tags: ['go', 'async', 'error-handling'],
  status: 'open',
  posted_by_type: 'human',
  posted_by_id: 'user-1',
  upvotes: 10,
  downvotes: 2,
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
  author: {
    type: 'human',
    id: 'user-1',
    display_name: 'John Doe',
  },
  vote_score: 8,
};

const mockAgent = {
  id: 'claude-assistant',
  display_name: 'Claude Assistant',
  bio: 'An AI assistant that helps with coding questions and problem solving.',
  specialties: ['go', 'python', 'typescript'],
  created_at: '2026-01-01T00:00:00Z',
  reputation: 1250,
  stats: {
    problems_solved: 10,
    questions_answered: 50,
    answers_accepted: 30,
  },
};

const mockUser = {
  id: 'user-1',
  username: 'johndoe',
  display_name: 'John Doe',
  bio: 'Full-stack developer interested in Go and distributed systems.',
  created_at: '2026-01-01T00:00:00Z',
  reputation: 500,
};

// Helper to import metadata generation functions
describe('SEO - Dynamic Page Titles', () => {
  beforeEach(() => {
    // Reset fetch mock before each test
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe('Post Detail Page Metadata', () => {
    it('generates title with post title per SPEC.md Part 19.2', async () => {
      // Mock successful API response
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockPost }),
      });

      const { generateMetadata } = await import('../app/posts/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'test-post-123' }) });

      expect(metadata.title).toBe('How to handle async errors in Go? | Solvr');
    });

    it('generates description from post description (first 160 chars)', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockPost }),
      });

      const { generateMetadata } = await import('../app/posts/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'test-post-123' }) });

      expect(metadata.description).toBeDefined();
      expect(metadata.description!.length).toBeLessThanOrEqual(160);
      expect(metadata.description).toContain('async');
    });

    it('includes Open Graph tags with post data', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockPost }),
      });

      const { generateMetadata } = await import('../app/posts/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'test-post-123' }) });

      expect(metadata.openGraph).toBeDefined();
      const og = metadata.openGraph as Record<string, unknown>;
      expect(og.title).toBe('How to handle async errors in Go? | Solvr');
      expect(og.type).toBe('article');
    });

    it('includes Twitter card tags', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockPost }),
      });

      const { generateMetadata } = await import('../app/posts/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'test-post-123' }) });

      expect(metadata.twitter).toBeDefined();
      const twitter = metadata.twitter as Record<string, unknown>;
      expect(twitter.card).toBe('summary_large_image');
      expect(twitter.title).toBe('How to handle async errors in Go? | Solvr');
    });

    it('returns fallback metadata when post not found', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: false,
        status: 404,
      });

      const { generateMetadata } = await import('../app/posts/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'nonexistent' }) });

      expect(metadata.title).toBe('Post Not Found | Solvr');
    });
  });

  describe('Agent Profile Page Metadata', () => {
    it('generates title with agent display name', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockAgent }),
      });

      const { generateMetadata } = await import('../app/agents/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'claude-assistant' }) });

      expect(metadata.title).toBe('Claude Assistant - AI Agent | Solvr');
    });

    it('includes agent bio in description', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockAgent }),
      });

      const { generateMetadata } = await import('../app/agents/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'claude-assistant' }) });

      expect(metadata.description).toContain('AI assistant');
    });

    it('returns fallback for missing agent', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: false,
        status: 404,
      });

      const { generateMetadata } = await import('../app/agents/[id]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ id: 'nonexistent' }) });

      expect(metadata.title).toBe('Agent Not Found | Solvr');
    });
  });

  describe('User Profile Page Metadata', () => {
    it('generates title with user display name', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockUser }),
      });

      const { generateMetadata } = await import('../app/users/[username]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ username: 'johndoe' }) });

      expect(metadata.title).toBe('John Doe | Solvr');
    });

    it('includes user bio in description', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: mockUser }),
      });

      const { generateMetadata } = await import('../app/users/[username]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ username: 'johndoe' }) });

      expect(metadata.description).toContain('Full-stack developer');
    });

    it('returns fallback for missing user', async () => {
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: false,
        status: 404,
      });

      const { generateMetadata } = await import('../app/users/[username]/metadata');
      const metadata = await generateMetadata({ params: Promise.resolve({ username: 'nonexistent' }) });

      expect(metadata.title).toBe('User Not Found | Solvr');
    });
  });
});

describe('SEO - Static Page Titles', () => {
  describe('Feed Page', () => {
    it('has correct static title', async () => {
      const { metadata } = await import('../app/feed/metadata');
      expect(metadata.title).toBe('Feed | Solvr');
    });

    it('has correct description', async () => {
      const { metadata } = await import('../app/feed/metadata');
      expect(metadata.description).toContain('latest');
    });
  });

  describe('Search Page', () => {
    it('has correct static title', async () => {
      const { metadata } = await import('../app/search/metadata');
      expect(metadata.title).toBe('Search | Solvr');
    });
  });

  describe('Dashboard Page', () => {
    it('has correct static title', async () => {
      const { metadata } = await import('../app/dashboard/metadata');
      expect(metadata.title).toBe('Dashboard | Solvr');
    });
  });

  describe('Login Page', () => {
    it('has correct static title', async () => {
      const { metadata } = await import('../app/login/metadata');
      expect(metadata.title).toBe('Login | Solvr');
    });
  });

  describe('Settings Page', () => {
    it('has correct static title', async () => {
      const { metadata } = await import('../app/settings/metadata');
      expect(metadata.title).toBe('Settings | Solvr');
    });
  });
});
