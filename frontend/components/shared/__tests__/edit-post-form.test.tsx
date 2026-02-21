import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { EditPostForm } from '../edit-post-form';

// Mock next/navigation
const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    back: vi.fn(),
  }),
}));

// Mock auth hook
const mockAuth = {
  user: { id: 'user-123', type: 'human' as const, displayName: 'Test User' },
  isAuthenticated: true,
  isLoading: false,
};

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockAuth,
}));

// Mock API
const mockGetPost = vi.fn();
const mockUpdatePost = vi.fn();
const mockGetComments = vi.fn();

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>();
  return {
    ...actual,
    api: {
      getPost: (...args: unknown[]) => mockGetPost(...args),
      updatePost: (...args: unknown[]) => mockUpdatePost(...args),
      getComments: (...args: unknown[]) => mockGetComments(...args),
    },
  };
});

// Mock toast
const mockToast = vi.fn();
vi.mock('@/components/ui/use-toast', () => ({
  useToast: () => ({ toast: mockToast }),
  toast: (...args: unknown[]) => mockToast(...args),
}));

const baseProblemPost = {
  id: 'post-123',
  type: 'problem' as const,
  title: 'Test Problem Title That Is Long Enough',
  description: 'This is a test description that is long enough to pass validation requirements of at least fifty characters.',
  status: 'open',
  upvotes: 5,
  downvotes: 1,
  vote_score: 4,
  view_count: 10,
  tags: ['golang', 'testing'],
  author: {
    id: 'user-123',
    type: 'human' as const,
    display_name: 'Test User',
  },
  created_at: '2026-02-01T00:00:00Z',
  updated_at: '2026-02-01T00:00:00Z',
};

const rejectedPost = {
  ...baseProblemPost,
  status: 'rejected',
};

describe('EditPostForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPost.mockResolvedValue({ data: baseProblemPost });
    mockGetComments.mockResolvedValue({ data: [], meta: { total: 0 } });
    mockUpdatePost.mockResolvedValue({ data: { ...baseProblemPost, title: 'Updated Title' } });
  });

  it('pre-fills form with current post data', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Problem Title That Is Long Enough')).toBeInTheDocument();
    });

    // Check description is pre-filled
    const descriptionTextarea = screen.getByRole('textbox', { name: /description/i });
    expect(descriptionTextarea).toHaveValue(baseProblemPost.description);

    // Check tags are displayed
    expect(screen.getByText('golang')).toBeInTheDocument();
    expect(screen.getByText('testing')).toBeInTheDocument();
  });

  it('shows rejection reason for rejected posts', async () => {
    mockGetPost.mockResolvedValue({ data: rejectedPost });
    mockGetComments.mockResolvedValue({
      data: [
        {
          id: 'comment-1',
          target_type: 'post',
          target_id: 'post-123',
          author_type: 'system',
          author_id: 'system',
          content: 'Rejected: Content violates policy - spam detected',
          created_at: '2026-02-01T01:00:00Z',
          author: {
            id: 'system',
            type: 'system',
            display_name: 'Solvr Moderation',
          },
        },
      ],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    });

    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByText(/Content violates policy/)).toBeInTheDocument();
    });
  });

  it('submit calls PATCH API with only changed fields', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    // Wait for form to load
    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Problem Title That Is Long Enough')).toBeInTheDocument();
    });

    // Change only the title
    const titleInput = screen.getByDisplayValue('Test Problem Title That Is Long Enough');
    fireEvent.change(titleInput, { target: { value: 'Updated Problem Title Long Enough' } });

    // Submit
    const submitButton = screen.getByRole('button', { name: /save/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdatePost).toHaveBeenCalledWith('post-123', {
        title: 'Updated Problem Title Long Enough',
      });
    });
  });

  it('shows status badge for open posts', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByText('OPEN')).toBeInTheDocument();
    });
  });

  it('shows rejected status badge for rejected posts', async () => {
    mockGetPost.mockResolvedValue({ data: rejectedPost });

    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByText('REJECTED')).toBeInTheDocument();
    });
  });

  it('shows "Save & Resubmit for Review" button for rejected posts', async () => {
    mockGetPost.mockResolvedValue({ data: rejectedPost });

    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save & resubmit for review/i })).toBeInTheDocument();
    });
  });

  it('shows "Save Changes" button for open posts', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    });
  });

  it('shows "Not authorized" when user is not the post author', async () => {
    mockGetPost.mockResolvedValue({
      data: {
        ...baseProblemPost,
        author: { id: 'other-user-456', type: 'human', display_name: 'Other User' },
      },
    });

    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByText(/not authorized/i)).toBeInTheDocument();
    });
  });

  it('redirects to detail page on successful submit', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Problem Title That Is Long Enough')).toBeInTheDocument();
    });

    const titleInput = screen.getByDisplayValue('Test Problem Title That Is Long Enough');
    fireEvent.change(titleInput, { target: { value: 'Updated Problem Title Long Enough' } });

    const submitButton = screen.getByRole('button', { name: /save/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/problems/post-123');
    });
  });

  it('shows loading state while fetching post', () => {
    mockGetPost.mockReturnValue(new Promise(() => {})); // Never resolves

    render(<EditPostForm postId="post-123" postType="problems" />);

    expect(screen.getByTestId('edit-post-loading')).toBeInTheDocument();
  });

  it('shows error when post fetch fails', async () => {
    mockGetPost.mockRejectedValue(new Error('Failed to fetch'));

    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /failed to load post/i })).toBeInTheDocument();
    });
  });

  it('validates title minimum length', async () => {
    render(<EditPostForm postId="post-123" postType="problems" />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Problem Title That Is Long Enough')).toBeInTheDocument();
    });

    const titleInput = screen.getByDisplayValue('Test Problem Title That Is Long Enough');
    fireEvent.change(titleInput, { target: { value: 'Short' } });

    const submitButton = screen.getByRole('button', { name: /save/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/title must be at least 10 characters/i)).toBeInTheDocument();
    });

    expect(mockUpdatePost).not.toHaveBeenCalled();
  });
});
