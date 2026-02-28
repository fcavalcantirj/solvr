import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import CreateBlogPostPage from './page';

// Mock next/navigation
const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

// Mock Header/Footer
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));
vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

// Mock useAuth
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock api
const mockCreateBlogPost = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    createBlogPost: (...args: unknown[]) => mockCreateBlogPost(...args),
  },
}));

// Mock react-markdown
vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown-preview">{children}</div>,
}));

function setupAuthenticated() {
  mockUseAuth.mockReturnValue({
    user: { id: 'user-1', username: 'alice' },
    isAuthenticated: true,
    isLoading: false,
    showAuthModal: false,
    authModalMessage: '',
    setShowAuthModal: vi.fn(),
  });
}

function setupUnauthenticated() {
  mockUseAuth.mockReturnValue({
    user: null,
    isAuthenticated: false,
    isLoading: false,
    showAuthModal: false,
    authModalMessage: '',
    setShowAuthModal: vi.fn(),
  });
}

describe('CreateBlogPostPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders all form fields when authenticated', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    // Title input
    expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
    // Body textarea
    expect(screen.getByLabelText(/body/i)).toBeInTheDocument();
    // Tags input
    expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    // Cover image URL
    expect(screen.getByLabelText(/cover image/i)).toBeInTheDocument();
    // Excerpt
    expect(screen.getByLabelText(/excerpt/i)).toBeInTheDocument();
    // Meta description
    expect(screen.getByLabelText(/meta description/i)).toBeInTheDocument();
    // Status toggle
    expect(screen.getByText(/publish/i)).toBeInTheDocument();
    // Submit button
    expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
  });

  it('validates title minimum length', async () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'Short' } });

    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: 'A'.repeat(60) } });

    const submitBtn = screen.getByRole('button', { name: /create/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText(/title must be at least 10 characters/i)).toBeInTheDocument();
    });
    expect(mockCreateBlogPost).not.toHaveBeenCalled();
  });

  it('validates body minimum length', async () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'A valid title for this blog post' } });

    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: 'Short' } });

    const submitBtn = screen.getByRole('button', { name: /create/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText(/body must be at least 50 characters/i)).toBeInTheDocument();
    });
    expect(mockCreateBlogPost).not.toHaveBeenCalled();
  });

  it('limits tags to 10', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const tagInput = screen.getByLabelText(/tags/i);

    // Add 10 tags
    for (let i = 0; i < 10; i++) {
      fireEvent.change(tagInput, { target: { value: `tag${i}` } });
      fireEvent.keyDown(tagInput, { key: 'Enter' });
    }

    // All 10 should be visible
    for (let i = 0; i < 10; i++) {
      expect(screen.getByText(`tag${i}`)).toBeInTheDocument();
    }

    // Tag input should be disabled after 10 tags
    expect(tagInput).toBeDisabled();
  });

  it('submits blog post successfully and redirects', async () => {
    setupAuthenticated();
    mockCreateBlogPost.mockResolvedValue({
      data: {
        id: 'post-1',
        slug: 'my-new-blog-post',
        title: 'My New Blog Post',
        body: 'A'.repeat(60),
        status: 'published',
      },
    });

    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'My New Blog Post' } });

    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: 'A'.repeat(60) } });

    const submitBtn = screen.getByRole('button', { name: /create/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(mockCreateBlogPost).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'My New Blog Post',
          body: 'A'.repeat(60),
        })
      );
    });

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/blog/my-new-blog-post');
    });
  });

  it('shows error on API failure', async () => {
    setupAuthenticated();
    mockCreateBlogPost.mockRejectedValue(new Error('Server error'));

    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'A valid title for blog' } });

    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: 'A'.repeat(60) } });

    const submitBtn = screen.getByRole('button', { name: /create/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText(/server error/i)).toBeInTheDocument();
    });
  });

  it('redirects unauthenticated users', () => {
    setupUnauthenticated();
    render(<CreateBlogPostPage />);

    // Should show authentication required message
    expect(screen.getByText(/authentication required/i)).toBeInTheDocument();
    // Should show sign in button
    const signInBtn = screen.getByRole('button', { name: /sign in/i });
    expect(signInBtn).toBeInTheDocument();

    // Click sign in should redirect to login
    fireEvent.click(signInBtn);
    expect(mockPush).toHaveBeenCalledWith('/login');
  });

  it('shows preview toggle for markdown body', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    // Preview button should exist
    const previewBtn = screen.getByRole('button', { name: /preview/i });
    expect(previewBtn).toBeInTheDocument();

    // Type some markdown content first
    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: '# Hello World' } });

    // Click preview
    fireEvent.click(previewBtn);

    // Should show markdown preview
    expect(screen.getByTestId('markdown-preview')).toBeInTheDocument();
  });

  it('shows character count for title', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'Hello World' } });

    expect(screen.getByText(/11\/300/)).toBeInTheDocument();
  });

  it('shows character count for meta description', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const metaInput = screen.getByLabelText(/meta description/i);
    fireEvent.change(metaInput, { target: { value: 'A brief description' } });

    expect(screen.getByText(/19\/160/)).toBeInTheDocument();
  });

  it('toggles between draft and published status', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    // Default should be published
    const draftBtn = screen.getByRole('button', { name: /draft/i });
    expect(draftBtn).toBeInTheDocument();

    fireEvent.click(draftBtn);
    // After clicking draft, draft should be active
    expect(draftBtn).toHaveClass('border-foreground');
  });

  it('sends tags in submission', async () => {
    setupAuthenticated();
    mockCreateBlogPost.mockResolvedValue({
      data: { slug: 'test-post' },
    });

    render(<CreateBlogPostPage />);

    const titleInput = screen.getByLabelText(/title/i);
    fireEvent.change(titleInput, { target: { value: 'A valid blog post title' } });

    const bodyInput = screen.getByLabelText(/body/i);
    fireEvent.change(bodyInput, { target: { value: 'A'.repeat(60) } });

    const tagInput = screen.getByLabelText(/tags/i);
    fireEvent.change(tagInput, { target: { value: 'golang' } });
    fireEvent.keyDown(tagInput, { key: 'Enter' });

    const submitBtn = screen.getByRole('button', { name: /create/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(mockCreateBlogPost).toHaveBeenCalledWith(
        expect.objectContaining({
          tags: ['golang'],
        })
      );
    });
  });

  it('removes tags when clicking X', () => {
    setupAuthenticated();
    render(<CreateBlogPostPage />);

    const tagInput = screen.getByLabelText(/tags/i);
    fireEvent.change(tagInput, { target: { value: 'golang' } });
    fireEvent.keyDown(tagInput, { key: 'Enter' });

    expect(screen.getByText('golang')).toBeInTheDocument();

    // Click remove button on the tag
    const removeBtn = screen.getByTestId('remove-tag-golang');
    fireEvent.click(removeBtn);

    expect(screen.queryByText('golang')).not.toBeInTheDocument();
  });
});
