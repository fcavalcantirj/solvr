import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { BlogPostContent } from './blog-post-content';

// Mock next/navigation
vi.mock('next/navigation', () => ({
  useParams: () => ({ slug: 'test-blog-post' }),
  notFound: vi.fn(),
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

describe('BlogPostContent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders blog post title and body', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByText('Test Blog Post Title')).toBeInTheDocument();
    expect(screen.getByTestId('markdown-content')).toBeInTheDocument();
  });

  it('renders author info with type badge', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByText('Alice Developer')).toBeInTheDocument();
  });

  it('renders AI author with AI badge', () => {
    setupDefaults();
    render(<BlogPostContent post={mockAIPost} />);

    expect(screen.getByText('Claudius')).toBeInTheDocument();
    // AI type badge
    const aiBadges = screen.getAllByText('AI');
    expect(aiBadges.length).toBeGreaterThan(0);
  });

  it('renders tags as links', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

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
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByText('5 min read')).toBeInTheDocument();
    expect(screen.getByText('Feb 15, 2026')).toBeInTheDocument();
  });

  it('renders Back to Blog link', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    const backLink = screen.getByRole('link', { name: /back to blog/i });
    expect(backLink).toHaveAttribute('href', '/blog');
  });

  it('renders vote buttons', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByTestId('vote-up')).toBeInTheDocument();
    expect(screen.getByTestId('vote-down')).toBeInTheDocument();
    expect(screen.getByTestId('vote-score')).toHaveTextContent('42');
  });

  it('handles vote up click', async () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    const upButton = screen.getByTestId('vote-up');
    fireEvent.click(upButton);

    await waitFor(() => {
      expect(mockVoteBlogPost).toHaveBeenCalledWith('test-blog-post', 'up');
    });
  });

  it('records view on mount', async () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    await waitFor(() => {
      expect(mockRecordBlogView).toHaveBeenCalledWith('test-blog-post');
    });
  });

  it('renders view count', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByText(/256/)).toBeInTheDocument();
  });

  it('renders share button', () => {
    setupDefaults();
    render(<BlogPostContent post={mockPost} />);

    expect(screen.getByTestId('share-button')).toBeInTheDocument();
  });

  it('shows auth modal when unauthenticated user votes', async () => {
    const mockSetShowAuthModal = vi.fn();
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: mockSetShowAuthModal,
    });
    mockRecordBlogView.mockResolvedValue(undefined);

    render(<BlogPostContent post={mockPost} />);

    const upButton = screen.getByTestId('vote-up');
    fireEvent.click(upButton);

    await waitFor(() => {
      expect(mockSetShowAuthModal).toHaveBeenCalledWith(true);
    });
    expect(mockVoteBlogPost).not.toHaveBeenCalled();
  });

  it('handles share button click with clipboard', async () => {
    setupDefaults();
    const mockWriteText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: { writeText: mockWriteText },
    });

    render(<BlogPostContent post={mockPost} />);

    const shareBtn = screen.getByTestId('share-button');
    fireEvent.click(shareBtn);

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText('Copied!')).toBeInTheDocument();
    });
  });

  it('handles vote down click', async () => {
    setupDefaults();
    mockVoteBlogPost.mockResolvedValue({ data: { vote_score: 41, upvotes: 42, downvotes: 1, user_vote: 'down' } });

    render(<BlogPostContent post={mockPost} />);

    const downButton = screen.getByTestId('vote-down');
    fireEvent.click(downButton);

    await waitFor(() => {
      expect(mockVoteBlogPost).toHaveBeenCalledWith('test-blog-post', 'down');
    });
  });

  it('handles vote API failure gracefully', async () => {
    setupDefaults();
    mockVoteBlogPost.mockRejectedValue(new Error('Vote failed'));

    render(<BlogPostContent post={mockPost} />);

    const upButton = screen.getByTestId('vote-up');
    fireEvent.click(upButton);

    await waitFor(() => {
      expect(mockVoteBlogPost).toHaveBeenCalledWith('test-blog-post', 'up');
    });

    // Vote score should remain unchanged on failure
    expect(screen.getByTestId('vote-score')).toHaveTextContent('42');
  });

  it('renders post without cover image', () => {
    setupDefaults();
    const noCoverPost = { ...mockPost, coverImageUrl: undefined };
    render(<BlogPostContent post={noCoverPost} />);

    expect(screen.getByText('Test Blog Post Title')).toBeInTheDocument();
  });
});
