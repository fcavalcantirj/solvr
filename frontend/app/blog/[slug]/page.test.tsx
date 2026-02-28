import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import BlogPostPage from './page';

// Mock next/navigation
const mockUseParams = vi.fn();
vi.mock('next/navigation', () => ({
  useParams: () => mockUseParams(),
  notFound: vi.fn(),
}));

// Mock blog hooks
const mockUseBlogPost = vi.fn();
vi.mock('@/hooks/use-blog', () => ({
  useBlogPost: (...args: unknown[]) => mockUseBlogPost(...args),
}));

// Mock Header/Footer
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

// Mock useAuth
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock api
const mockVoteBlogPost = vi.fn();
const mockRecordBlogView = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    voteBlogPost: (...args: unknown[]) => mockVoteBlogPost(...args),
    recordBlogView: (...args: unknown[]) => mockRecordBlogView(...args),
  },
}));

// Mock react-markdown
vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown-content">{children}</div>,
}));

const mockPost = {
  slug: 'test-blog-post',
  title: 'Test Blog Post Title',
  excerpt: 'A test excerpt for the blog post',
  body: '# Hello World\n\nThis is the blog post body with **bold** text.',
  tags: ['golang', 'postgresql', 'testing'],
  coverImageUrl: 'https://example.com/cover.jpg',
  author: {
    name: 'Alice Developer',
    type: 'human' as const,
    avatar: 'https://example.com/avatar.jpg',
  },
  readTime: '5 min read',
  publishedAt: 'Feb 15, 2026',
  voteScore: 42,
  viewCount: 256,
  userVote: null as 'up' | 'down' | null,
};

const mockAIPost = {
  ...mockPost,
  slug: 'ai-blog-post',
  title: 'AI Agent Blog Post',
  author: {
    name: 'Claudius',
    type: 'ai' as const,
    avatar: undefined,
  },
};

function setupDefaults() {
  mockUseParams.mockReturnValue({ slug: 'test-blog-post' });
  mockUseBlogPost.mockReturnValue({
    post: mockPost,
    loading: false,
    error: null,
  });
  mockUseAuth.mockReturnValue({
    user: { id: 'user-1', username: 'alice' },
    isAuthenticated: true,
    isLoading: false,
    showAuthModal: false,
    authModalMessage: '',
    setShowAuthModal: vi.fn(),
  });
  mockRecordBlogView.mockResolvedValue(undefined);
  mockVoteBlogPost.mockResolvedValue({ data: { vote_score: 43, upvotes: 43, downvotes: 0, user_vote: 'up' } });
}

describe('BlogPostPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders blog post title and body', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByText('Test Blog Post Title')).toBeInTheDocument();
    expect(screen.getByTestId('markdown-content')).toBeInTheDocument();
  });

  it('renders author info with type badge', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByText('Alice Developer')).toBeInTheDocument();
  });

  it('renders AI author with AI badge', () => {
    setupDefaults();
    mockUseBlogPost.mockReturnValue({
      post: mockAIPost,
      loading: false,
      error: null,
    });
    render(<BlogPostPage />);

    expect(screen.getByText('Claudius')).toBeInTheDocument();
    // AI type badge
    const aiBadges = screen.getAllByText('AI');
    expect(aiBadges.length).toBeGreaterThan(0);
  });

  it('renders tags as links', () => {
    setupDefaults();
    render(<BlogPostPage />);

    const tagLinks = screen.getAllByRole('link').filter(
      (link) => link.getAttribute('href')?.startsWith('/blog?tag=')
    );
    expect(tagLinks.length).toBe(3);
    expect(tagLinks[0]).toHaveAttribute('href', '/blog?tag=golang');
    expect(tagLinks[1]).toHaveAttribute('href', '/blog?tag=postgresql');
    expect(tagLinks[2]).toHaveAttribute('href', '/blog?tag=testing');
  });

  it('renders read time and date', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByText('5 min read')).toBeInTheDocument();
    expect(screen.getByText('Feb 15, 2026')).toBeInTheDocument();
  });

  it('renders loading skeleton when loading', () => {
    mockUseParams.mockReturnValue({ slug: 'test-blog-post' });
    mockUseBlogPost.mockReturnValue({
      post: null,
      loading: true,
      error: null,
    });
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
    });

    render(<BlogPostPage />);

    expect(screen.getByTestId('blog-post-skeleton')).toBeInTheDocument();
  });

  it('renders error state when post not found', () => {
    mockUseParams.mockReturnValue({ slug: 'nonexistent-slug' });
    mockUseBlogPost.mockReturnValue({
      post: null,
      loading: false,
      error: 'Not found',
    });
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
    });

    render(<BlogPostPage />);

    expect(screen.getByText(/not found|error/i)).toBeInTheDocument();
  });

  it('renders Back to Blog link', () => {
    setupDefaults();
    render(<BlogPostPage />);

    const backLink = screen.getByRole('link', { name: /back to blog/i });
    expect(backLink).toHaveAttribute('href', '/blog');
  });

  it('renders vote buttons', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByTestId('vote-up')).toBeInTheDocument();
    expect(screen.getByTestId('vote-down')).toBeInTheDocument();
    expect(screen.getByTestId('vote-score')).toHaveTextContent('42');
  });

  it('handles vote up click', async () => {
    setupDefaults();
    render(<BlogPostPage />);

    const upButton = screen.getByTestId('vote-up');
    fireEvent.click(upButton);

    await waitFor(() => {
      expect(mockVoteBlogPost).toHaveBeenCalledWith('test-blog-post', 'up');
    });
  });

  it('records view on mount', async () => {
    setupDefaults();
    render(<BlogPostPage />);

    await waitFor(() => {
      expect(mockRecordBlogView).toHaveBeenCalledWith('test-blog-post');
    });
  });

  it('renders view count', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByText(/256/)).toBeInTheDocument();
  });

  it('renders share button', () => {
    setupDefaults();
    render(<BlogPostPage />);

    expect(screen.getByTestId('share-button')).toBeInTheDocument();
  });
});
