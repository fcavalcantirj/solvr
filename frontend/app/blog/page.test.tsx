import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import BlogPage from './page';

// Mock hooks
const mockUseBlogPosts = vi.fn();
const mockUseBlogFeatured = vi.fn();
const mockUseBlogTags = vi.fn();

vi.mock('@/hooks/use-blog', () => ({
  useBlogPosts: (...args: unknown[]) => mockUseBlogPosts(...args),
  useBlogFeatured: () => mockUseBlogFeatured(),
  useBlogTags: () => mockUseBlogTags(),
}));

const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

// Mock Header/Footer to isolate
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));
vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Test data
const mockPosts = [
  {
    slug: 'test-post-1',
    title: 'Test Post One',
    excerpt: 'This is the first test post excerpt',
    body: 'Full body',
    tags: ['engineering', 'go'],
    author: { name: 'Alice', type: 'human' as const },
    readTime: '5 min read',
    publishedAt: 'Feb 1, 2026',
    voteScore: 10,
    viewCount: 100,
  },
  {
    slug: 'test-post-2',
    title: 'Test Post Two',
    excerpt: 'This is the second test post excerpt',
    body: 'Full body 2',
    tags: ['research', 'ai'],
    author: { name: 'BotAgent', type: 'ai' as const },
    readTime: '8 min read',
    publishedAt: 'Jan 28, 2026',
    voteScore: 5,
    viewCount: 50,
  },
];

const mockFeaturedPost = {
  slug: 'featured-post',
  title: 'Featured Post Title',
  excerpt: 'This is the featured post excerpt',
  body: 'Featured body',
  tags: ['mcp', 'open-source'],
  author: { name: 'Sarah Chen', type: 'human' as const },
  readTime: '12 min read',
  publishedAt: 'Feb 1, 2026',
  voteScore: 42,
  viewCount: 500,
};

const mockTags = [
  { name: 'engineering', count: 18 },
  { name: 'research', count: 9 },
  { name: 'ai', count: 7 },
  { name: 'go', count: 5 },
];

function setupDefaultMocks() {
  mockUseAuth.mockReturnValue({ isAuthenticated: false, user: null, loading: false });
  mockUseBlogPosts.mockReturnValue({
    posts: mockPosts,
    loading: false,
    error: null,
    total: 2,
    hasMore: false,
    page: 1,
    refetch: vi.fn(),
    loadMore: vi.fn(),
  });
  mockUseBlogFeatured.mockReturnValue({
    post: mockFeaturedPost,
    loading: false,
    error: null,
  });
  mockUseBlogTags.mockReturnValue({
    tags: mockTags,
    loading: false,
    error: null,
  });
}

describe('BlogPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading skeleton initially', () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false, user: null, loading: false });
    mockUseBlogPosts.mockReturnValue({
      posts: [],
      loading: true,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });
    mockUseBlogFeatured.mockReturnValue({
      post: null,
      loading: true,
      error: null,
    });
    mockUseBlogTags.mockReturnValue({
      tags: [],
      loading: true,
      error: null,
    });

    render(<BlogPage />);

    // Should show skeleton placeholders
    expect(screen.getByTestId('featured-skeleton')).toBeInTheDocument();
    expect(screen.getByTestId('posts-skeleton')).toBeInTheDocument();
  });

  it('renders blog posts from API after loading', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    expect(screen.getByText('Test Post One')).toBeInTheDocument();
    expect(screen.getByText('Test Post Two')).toBeInTheDocument();
    expect(screen.getByText('This is the first test post excerpt')).toBeInTheDocument();
    expect(screen.getByText('5 min read')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('renders featured post from API', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    expect(screen.getByText('Featured Post Title')).toBeInTheDocument();
    expect(screen.getByText('This is the featured post excerpt')).toBeInTheDocument();
    expect(screen.getByText('Sarah Chen')).toBeInTheDocument();
    // Featured post links to /blog/{slug}
    const featuredLink = screen.getByText('Featured Post Title').closest('a');
    expect(featuredLink).toHaveAttribute('href', '/blog/featured-post');
  });

  it('renders tags from API', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    expect(screen.getByText('engineering')).toBeInTheDocument();
    expect(screen.getByText('research')).toBeInTheDocument();
    expect(screen.getByText('ai')).toBeInTheDocument();
    expect(screen.getByText('go')).toBeInTheDocument();
  });

  it('filters posts by tag selection', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    // Click the "engineering" tag filter button
    const engineeringBtn = screen.getByRole('button', { name: /ENGINEERING/i });
    fireEvent.click(engineeringBtn);

    // useBlogPosts should have been called with tags param
    expect(mockUseBlogPosts).toHaveBeenCalledWith(
      expect.objectContaining({ tags: 'engineering' })
    );
  });

  it('handles API error gracefully with retry', () => {
    const mockRefetch = vi.fn();
    mockUseBlogPosts.mockReturnValue({
      posts: [],
      loading: false,
      error: 'Failed to fetch blog posts',
      total: 0,
      hasMore: false,
      page: 1,
      refetch: mockRefetch,
      loadMore: vi.fn(),
    });
    mockUseBlogFeatured.mockReturnValue({
      post: null,
      loading: false,
      error: 'Failed to fetch featured post',
    });
    mockUseBlogTags.mockReturnValue({
      tags: [],
      loading: false,
      error: null,
    });

    render(<BlogPage />);

    // Should show error message
    expect(screen.getByText(/failed to fetch/i)).toBeInTheDocument();
    // Should show retry button
    const retryBtn = screen.getByRole('button', { name: /retry/i });
    expect(retryBtn).toBeInTheDocument();
    fireEvent.click(retryBtn);
    expect(mockRefetch).toHaveBeenCalled();
  });

  it('renders empty state when no posts', () => {
    mockUseBlogPosts.mockReturnValue({
      posts: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });
    mockUseBlogFeatured.mockReturnValue({
      post: null,
      loading: false,
      error: null,
    });
    mockUseBlogTags.mockReturnValue({
      tags: [],
      loading: false,
      error: null,
    });

    render(<BlogPage />);

    expect(screen.getByText(/no posts found/i)).toBeInTheDocument();
  });

  it('renders post links using slug not id', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    const postLink = screen.getByText('Test Post One').closest('a');
    expect(postLink).toHaveAttribute('href', '/blog/test-post-1');
  });

  it('renders tag links as /blog?tag={tag}', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    // Tags in the tags cloud section should link to /blog?tag={tag}
    const tagLinks = screen.getAllByRole('link').filter(
      (link) => link.getAttribute('href')?.startsWith('/blog?tag=')
    );
    expect(tagLinks.length).toBeGreaterThan(0);
  });

  it('does not render newsletter subscription form', () => {
    setupDefaultMocks();
    render(<BlogPage />);
    expect(screen.queryByPlaceholderText(/your@email.com/i)).not.toBeInTheDocument();
  });

  it('renders category filter buttons from tags', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    // "All Posts" button always present
    expect(screen.getByRole('button', { name: /ALL POSTS/i })).toBeInTheDocument();
    // Tag-based category buttons
    expect(screen.getByRole('button', { name: /ENGINEERING/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /RESEARCH/i })).toBeInTheDocument();
  });

  it('renders SOLVR BLOG header text', () => {
    setupDefaultMocks();
    render(<BlogPage />);
    expect(screen.getByText('SOLVR BLOG')).toBeInTheDocument();
  });

  it('renders AI author badge differently from human', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    // The AI author should have AI badge text
    const aiLabels = screen.getAllByText('AI');
    expect(aiLabels.length).toBeGreaterThan(0);
  });

  it('filters posts by search query matching title', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    const searchInput = screen.getByPlaceholderText(/search posts/i);
    fireEvent.change(searchInput, { target: { value: 'Post One' } });

    // Should show matching post
    expect(screen.getByText('Test Post One')).toBeInTheDocument();
    // Should not show non-matching post
    expect(screen.queryByText('Test Post Two')).not.toBeInTheDocument();
  });

  it('filters posts by search query matching excerpt', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    const searchInput = screen.getByPlaceholderText(/search posts/i);
    fireEvent.change(searchInput, { target: { value: 'second test post' } });

    expect(screen.queryByText('Test Post One')).not.toBeInTheDocument();
    expect(screen.getByText('Test Post Two')).toBeInTheDocument();
  });

  it('clears filters when clicking clear filters button', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    // Type a search query that matches nothing
    const searchInput = screen.getByPlaceholderText(/search posts/i);
    fireEvent.change(searchInput, { target: { value: 'xyznonexistent' } });

    // Should show empty state with clear filters button
    expect(screen.getByText(/no posts found/i)).toBeInTheDocument();
    const clearBtn = screen.getByRole('button', { name: /clear filters/i });
    fireEvent.click(clearBtn);

    // After clearing, posts should be visible again
    expect(screen.getByText('Test Post One')).toBeInTheDocument();
    expect(screen.getByText('Test Post Two')).toBeInTheDocument();
  });

  it('renders featured post without cover image', () => {
    setupDefaultMocks();
    mockUseBlogFeatured.mockReturnValue({
      post: { ...mockFeaturedPost, coverImageUrl: undefined, tags: [] },
      loading: false,
      error: null,
    });
    render(<BlogPage />);

    expect(screen.getByText('Featured Post Title')).toBeInTheDocument();
  });

  it('renders post without cover image showing tag placeholder', () => {
    setupDefaultMocks();
    mockUseBlogPosts.mockReturnValue({
      posts: [{ ...mockPosts[0], coverImageUrl: undefined }],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });
    render(<BlogPage />);

    expect(screen.getByText('Test Post One')).toBeInTheDocument();
  });

  it('renders WRITE POST button on blog page', () => {
    setupDefaultMocks();
    render(<BlogPage />);

    expect(screen.getAllByText('WRITE POST').length).toBeGreaterThanOrEqual(1);
  });

  it('navigates to /blog/create when authenticated user clicks WRITE POST', () => {
    setupDefaultMocks();
    mockUseAuth.mockReturnValue({ isAuthenticated: true, user: { id: '1' }, loading: false });
    render(<BlogPage />);

    const buttons = screen.getAllByText('WRITE POST');
    fireEvent.click(buttons[0]);

    expect(mockPush).toHaveBeenCalledWith('/blog/create');
  });

  it('navigates to login when unauthenticated user clicks WRITE POST', () => {
    setupDefaultMocks();
    mockUseAuth.mockReturnValue({ isAuthenticated: false, user: null, loading: false });
    render(<BlogPage />);

    const buttons = screen.getAllByText('WRITE POST');
    fireEvent.click(buttons[0]);

    expect(mockPush).toHaveBeenCalledWith('/login?next=/blog/create');
  });
});
