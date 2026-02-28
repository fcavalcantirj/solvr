"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useBlogPosts, useBlogPost, useBlogFeatured, useBlogTags } from './use-blog';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getBlogPosts: vi.fn(),
    getBlogPost: vi.fn(),
    getBlogFeatured: vi.fn(),
    getBlogTags: vi.fn(),
  },
  formatRelativeTime: (date: string) => '3d ago',
}));

const mockAuthor = {
  id: 'u1',
  type: 'human' as const,
  display_name: 'Alice',
  avatar_url: 'https://example.com/alice.png',
};

const mockAgentAuthor = {
  id: 'a1',
  type: 'agent' as const,
  display_name: 'solver-bot',
};

const mockBlogPost = {
  id: 'bp1',
  slug: 'test-blog-post',
  title: 'Test Blog Post',
  body: 'This is the full body of the blog post with detailed content.',
  excerpt: 'This is a short excerpt',
  tags: ['go', 'testing'],
  cover_image_url: 'https://example.com/cover.jpg',
  posted_by_type: 'human' as const,
  posted_by_id: 'u1',
  status: 'published',
  view_count: 150,
  upvotes: 10,
  downvotes: 2,
  vote_score: 8,
  read_time_minutes: 5,
  meta_description: 'A test blog post',
  published_at: '2026-02-15T10:00:00Z',
  created_at: '2026-02-14T10:00:00Z',
  updated_at: '2026-02-15T10:00:00Z',
  author: mockAuthor,
  user_vote: null as 'up' | 'down' | null,
};

const mockBlogPost2 = {
  ...mockBlogPost,
  id: 'bp2',
  slug: 'second-blog-post',
  title: 'Second Blog Post',
  body: 'Another blog post body content here.',
  excerpt: undefined,
  tags: undefined,
  cover_image_url: undefined,
  posted_by_type: 'agent' as const,
  posted_by_id: 'a1',
  read_time_minutes: 3,
  published_at: undefined,
  author: mockAgentAuthor,
  user_vote: 'up' as const,
};

const mockBlogPostsResponse = {
  data: [mockBlogPost, mockBlogPost2],
  meta: { total: 2, page: 1, per_page: 20, has_more: false },
};

describe('useBlogPosts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches and transforms blog posts correctly', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockBlogPostsResponse);

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.posts).toHaveLength(2);
    expect(result.current.total).toBe(2);
    expect(result.current.hasMore).toBe(false);
    expect(result.current.error).toBeNull();

    // Verify first post transformation
    const post1 = result.current.posts[0];
    expect(post1.slug).toBe('test-blog-post');
    expect(post1.title).toBe('Test Blog Post');
    expect(post1.excerpt).toBe('This is a short excerpt');
    expect(post1.body).toBe('This is the full body of the blog post with detailed content.');
    expect(post1.tags).toEqual(['go', 'testing']);
    expect(post1.coverImageUrl).toBe('https://example.com/cover.jpg');
    expect(post1.author.name).toBe('Alice');
    expect(post1.author.type).toBe('human');
    expect(post1.author.avatar).toBe('https://example.com/alice.png');
    expect(post1.readTime).toBe('5 min read');
    expect(post1.publishedAt).toBe('3d ago');
    expect(post1.voteScore).toBe(8);
    expect(post1.viewCount).toBe(150);
    expect(post1.userVote).toBeNull();

    // Verify second post with agent author and missing fields
    const post2 = result.current.posts[1];
    expect(post2.slug).toBe('second-blog-post');
    expect(post2.author.name).toBe('solver-bot');
    expect(post2.author.type).toBe('ai');
    expect(post2.author.avatar).toBeUndefined();
    expect(post2.tags).toEqual([]);
    expect(post2.coverImageUrl).toBeUndefined();
    expect(post2.readTime).toBe('3 min read');
    expect(post2.userVote).toBe('up');
  });

  it('handles empty response', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.posts).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('handles API errors', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.posts).toEqual([]);
  });

  it('handles null/undefined response data defensively', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockResolvedValue(null);

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.posts).toEqual([]);
    expect(result.current.total).toBe(0);
  });

  it('passes params to API call', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 10, has_more: false },
    });

    renderHook(() => useBlogPosts({ tags: 'go', per_page: 10 }));

    await waitFor(() => {
      expect(api.getBlogPosts).toHaveBeenCalledWith(
        expect.objectContaining({ tags: 'go', page: 1, per_page: 10 })
      );
    });
  });

  it('supports pagination with loadMore', async () => {
    const page1 = {
      data: [mockBlogPost],
      meta: { total: 2, page: 1, per_page: 1, has_more: true },
    };
    const page2 = {
      data: [mockBlogPost2],
      meta: { total: 2, page: 2, per_page: 1, has_more: false },
    };
    (api.getBlogPosts as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(page1)
      .mockResolvedValueOnce(page2);

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.posts).toHaveLength(1);
    expect(result.current.hasMore).toBe(true);

    // Load more
    result.current.loadMore();

    await waitFor(() => {
      expect(result.current.posts).toHaveLength(2);
    });

    expect(result.current.hasMore).toBe(false);
  });

  it('refetch resets to page 1', async () => {
    (api.getBlogPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockBlogPostsResponse);

    const { result } = renderHook(() => useBlogPosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    result.current.refetch();

    await waitFor(() => {
      expect(api.getBlogPosts).toHaveBeenCalledTimes(2);
    });
  });
});

describe('useBlogPost', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches single post by slug', async () => {
    (api.getBlogPost as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockBlogPost,
    });

    const { result } = renderHook(() => useBlogPost('test-blog-post'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getBlogPost).toHaveBeenCalledWith('test-blog-post');
    expect(result.current.post).not.toBeNull();
    expect(result.current.post!.slug).toBe('test-blog-post');
    expect(result.current.post!.title).toBe('Test Blog Post');
    expect(result.current.post!.body).toBe('This is the full body of the blog post with detailed content.');
    expect(result.current.error).toBeNull();
  });

  it('handles not found (API error)', async () => {
    (api.getBlogPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not found'));

    const { result } = renderHook(() => useBlogPost('nonexistent-slug'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.post).toBeNull();
    expect(result.current.error).toBe('Not found');
  });

  it('does not fetch when slug is empty', async () => {
    const { result } = renderHook(() => useBlogPost(''));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getBlogPost).not.toHaveBeenCalled();
    expect(result.current.post).toBeNull();
  });
});

describe('useBlogFeatured', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches featured post', async () => {
    (api.getBlogFeatured as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockBlogPost,
    });

    const { result } = renderHook(() => useBlogFeatured());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.post).not.toBeNull();
    expect(result.current.post!.slug).toBe('test-blog-post');
    expect(result.current.post!.title).toBe('Test Blog Post');
    expect(result.current.error).toBeNull();
  });

  it('handles no featured post (null data)', async () => {
    (api.getBlogFeatured as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: null,
    });

    const { result } = renderHook(() => useBlogFeatured());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.post).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('handles API error gracefully', async () => {
    (api.getBlogFeatured as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Server error'));

    const { result } = renderHook(() => useBlogFeatured());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.post).toBeNull();
    expect(result.current.error).toBe('Server error');
  });
});

describe('useBlogTags', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches tags with counts', async () => {
    (api.getBlogTags as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [
        { name: 'go', count: 5 },
        { name: 'testing', count: 3 },
        { name: 'react', count: 1 },
      ],
    });

    const { result } = renderHook(() => useBlogTags());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.tags).toHaveLength(3);
    expect(result.current.tags[0]).toEqual({ name: 'go', count: 5 });
    expect(result.current.tags[1]).toEqual({ name: 'testing', count: 3 });
    expect(result.current.error).toBeNull();
  });

  it('handles empty tags response', async () => {
    (api.getBlogTags as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
    });

    const { result } = renderHook(() => useBlogTags());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.tags).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('handles API error', async () => {
    (api.getBlogTags as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Failed'));

    const { result } = renderHook(() => useBlogTags());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.tags).toEqual([]);
    expect(result.current.error).toBe('Failed');
  });
});
